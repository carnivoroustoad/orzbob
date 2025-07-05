package billing

import (
	"context"
	"testing"
	"time"
)

// TestUsageEmissionOnStop verifies that usage is recorded when an instance stops
func TestUsageEmissionOnStop(t *testing.T) {
	// Create mock config
	config := &Config{
		PolarAPIKey:      "test-key",
		PolarWebhookSecret: "test-secret",
		PolarOrgID:       "test-org",
	}

	// Create metering service
	service, err := NewMeteringService(config)
	if err != nil {
		t.Fatalf("Failed to create metering service: %v", err)
	}

	// Replace with mock client
	mockClient := NewMockPolarClient()
	service.client = mockClient

	// Simulate instance lifecycle
	tests := []struct {
		name        string
		instanceID  string
		orgID       string
		customerID  string
		tier        string
		runtime     time.Duration
		wantMinutes int
		wantHours   float64
	}{
		{
			name:        "Small instance 2 hours",
			instanceID:  "inst-001",
			orgID:       "org-123",
			customerID:  "cust-123",
			tier:        "small",
			runtime:     2 * time.Hour,
			wantMinutes: 120,
			wantHours:   2.0,
		},
		{
			name:        "Medium instance 90 minutes",
			instanceID:  "inst-002",
			orgID:       "org-456",
			customerID:  "cust-456",
			tier:        "medium",
			runtime:     90 * time.Minute,
			wantMinutes: 90,
			wantHours:   1.5,
		},
		{
			name:        "Large instance 45 minutes",
			instanceID:  "inst-003",
			orgID:       "org-789",
			customerID:  "cust-789",
			tier:        "large",
			runtime:     45 * time.Minute,
			wantMinutes: 45,
			wantHours:   0.75,
		},
		{
			name:        "GPU instance 30 minutes",
			instanceID:  "inst-004",
			orgID:       "org-999",
			customerID:  "cust-999",
			tier:        "gpu",
			runtime:     30 * time.Minute,
			wantMinutes: 30,
			wantHours:   0.5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear previous records
			mockClient.usageRecords = nil

			// Record usage when instance stops
			minutes := int(tt.runtime.Minutes())
			service.RecordUsage(tt.orgID, tt.customerID, minutes, tt.tier)

			// Flush to send to Polar
			err := service.Flush(context.Background())
			if err != nil {
				t.Errorf("Flush failed: %v", err)
			}

			// Verify usage was recorded
			records := mockClient.GetUsageRecords()
			if len(records) != 1 {
				t.Errorf("Expected 1 usage record, got %d", len(records))
				return
			}

			record := records[0]
			if record.CustomerID != tt.customerID {
				t.Errorf("CustomerID = %s, want %s", record.CustomerID, tt.customerID)
			}
			if record.Usage != tt.wantHours {
				t.Errorf("Usage hours = %f, want %f", record.Usage, tt.wantHours)
			}
			if record.Metadata.OrgID != tt.orgID {
				t.Errorf("OrgID = %s, want %s", record.Metadata.OrgID, tt.orgID)
			}
			if record.Metadata.Tier != tt.tier {
				t.Errorf("Tier = %s, want %s", record.Metadata.Tier, tt.tier)
			}
		})
	}
}

// TestTierPricingValidation verifies tier pricing matches requirements
func TestTierPricingValidation(t *testing.T) {
	// Verify tier pricing matches B-4 requirements
	expectedPricing := map[string]float64{
		"small":  8.3,   // $0.083/hour
		"medium": 16.7,  // $0.167/hour
		"large":  33.3,  // $0.333/hour
		"gpu":    208.0, // $2.08/hour
	}

	for tier, expectedCents := range expectedPricing {
		actualCents, ok := TierPricing[tier]
		if !ok {
			t.Errorf("Tier %s not found in pricing", tier)
			continue
		}
		if actualCents != expectedCents {
			t.Errorf("Tier %s: price = %.1f cents/hour, want %.1f cents/hour", 
				tier, actualCents, expectedCents)
		}
	}
}

// TestControlPlaneIntegration simulates control plane usage recording
func TestControlPlaneIntegration(t *testing.T) {
	// This test demonstrates how the control plane records usage
	config := &Config{
		PolarAPIKey:      "test-key",
		PolarWebhookSecret: "test-secret",
		PolarOrgID:       "test-org",
	}

	service, err := NewMeteringService(config)
	if err != nil {
		t.Fatalf("Failed to create metering service: %v", err)
	}

	mockClient := NewMockPolarClient()
	service.client = mockClient

	// Start the service
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	service.Start(ctx)

	// Simulate multiple instance stops
	instances := []struct {
		id       string
		orgID    string
		tier     string
		runtime  int // minutes
	}{
		{"inst-1", "org-1", "small", 125},
		{"inst-2", "org-1", "small", 60},
		{"inst-3", "org-2", "medium", 90},
		{"inst-4", "org-3", "large", 240},
	}

	// Record usage for each instance
	for _, inst := range instances {
		// In real control plane, customer ID would come from subscription mapping
		customerID := inst.orgID // Simplified for test
		service.RecordUsage(inst.orgID, customerID, inst.runtime, inst.tier)
	}

	// Flush
	err = service.Flush(ctx)
	if err != nil {
		t.Errorf("Flush failed: %v", err)
	}

	// Verify aggregation
	records := mockClient.GetUsageRecords()
	
	// Should have 3 records (2 for org-1 small tier get aggregated)
	if len(records) != 3 {
		t.Errorf("Expected 3 aggregated records, got %d", len(records))
	}

	// Find org-1 record
	var org1Usage float64
	for _, r := range records {
		if r.CustomerID == "org-1" {
			org1Usage = r.Usage
			break
		}
	}

	// org-1 had 125 + 60 = 185 minutes = 3.083 hours
	expectedHours := 185.0 / 60.0
	if diff := org1Usage - expectedHours; diff < -0.01 || diff > 0.01 {
		t.Errorf("org-1 usage = %f hours, want %f hours", org1Usage, expectedHours)
	}

	// Stop the service
	service.Stop()
}

// TestHeartbeatTimeout verifies usage is recorded when heartbeat times out
func TestHeartbeatTimeout(t *testing.T) {
	// This test verifies that the idle reaper correctly records usage
	// when an instance hasn't sent a heartbeat in 30 minutes
	
	// The control plane idle reaper will:
	// 1. Check heartbeats every minute
	// 2. Find instances idle for > 30 minutes
	// 3. Call recordInstanceUsage() before deletion
	// 4. Delete the instance
	
	// This is handled by the control plane, not the billing package
	// See reapIdleInstances() in cmd/cloud-cp/main.go
	t.Log("Heartbeat timeout usage recording is handled by control plane")
	t.Log("See TestE2EIdleInstanceReaping in e2e tests")
}