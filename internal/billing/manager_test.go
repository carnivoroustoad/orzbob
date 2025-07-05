package billing

import (
	"context"
	"testing"
	"time"
)

func TestManager_BudgetAlerts(t *testing.T) {
	// Create a test config
	config := Config{
		PolarAPIKey:    "test-key",
		PolarOrgID:     "test-org-id",
		PolarProjectID: "test-project",
		PolarWebhookSecret: "test-secret",
	}

	// Create mock client
	mockClient := NewMockPolarClient()
	mockClient.SetupDefaultProducts()
	
	// Set up subscription for test org
	mockClient.subscriptions["customer-test-org"] = &SubscriptionResponse{
		ID:         "sub-123",
		CustomerID: "customer-test-org",
		ProductID:  "prod_base_plus_usage",
		Status:     "active",
		CreatedAt:  time.Now(),
	}

	// Create manager with mock client
	manager, err := NewManagerWithClient(config, mockClient)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	// Start services
	ctx := context.Background()
	err = manager.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start manager: %v", err)
	}
	defer manager.Stop()

	// Set up test organization
	orgID := "test-org"

	// Test 50% usage alert
	t.Run("50% usage alert", func(t *testing.T) {
		// Record 100 hours of usage (50%)
		err := manager.RecordUsage(orgID, "customer-test-org", 100*60, "small") // 100 hours in minutes
		if err != nil {
			t.Fatalf("Failed to record usage: %v", err)
		}

		// Get usage info
		usage, err := manager.GetUsage(orgID)
		if err != nil {
			t.Fatalf("Failed to get usage: %v", err)
		}

		if usage.UsedHours != 100 {
			t.Errorf("Expected 100 hours used, got %f", usage.UsedHours)
		}

		percentUsed := int(usage.PercentUsed)
		if percentUsed != 50 {
			t.Errorf("Expected 50%% usage, got %d%%", percentUsed)
		}
	})

	// Test 90% usage alert
	t.Run("90% usage alert", func(t *testing.T) {
		// Record additional 80 hours (total 180 hours = 90%)
		err := manager.RecordUsage(orgID, "customer-test-org", 80*60, "small")
		if err != nil {
			t.Fatalf("Failed to record usage: %v", err)
		}

		usage, err := manager.GetUsage(orgID)
		if err != nil {
			t.Fatalf("Failed to get usage: %v", err)
		}

		if usage.UsedHours != 180 {
			t.Errorf("Expected 180 hours used, got %f", usage.UsedHours)
		}

		percentUsed := int(usage.PercentUsed)
		if percentUsed != 90 {
			t.Errorf("Expected 90%% usage, got %d%%", percentUsed)
		}
	})

	// Test overage
	t.Run("overage detection", func(t *testing.T) {
		// Record additional 30 hours (total 210 hours > 200 included)
		err := manager.RecordUsage(orgID, "customer-test-org", 30*60, "small")
		if err != nil {
			t.Fatalf("Failed to record usage: %v", err)
		}

		inOverage, err := manager.CheckQuota(orgID)
		if err != nil {
			t.Fatalf("Failed to check quota: %v", err)
		}

		if !inOverage {
			t.Error("Expected to be in overage, but wasn't")
		}

		usage, err := manager.GetUsage(orgID)
		if err != nil {
			t.Fatalf("Failed to get usage: %v", err)
		}

		if !usage.InOverage {
			t.Error("Expected InOverage flag to be true")
		}
	})
}

func TestManager_StartStop(t *testing.T) {
	config := Config{
		PolarAPIKey:    "test-key",
		PolarOrgID:     "test-org-id",
		PolarProjectID: "test-project",
		PolarWebhookSecret: "test-secret",
	}

	manager, err := NewManager(config)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	ctx := context.Background()
	
	// Start manager
	err = manager.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start manager: %v", err)
	}

	// Let it run briefly
	time.Sleep(100 * time.Millisecond)

	// Stop manager
	manager.Stop()
	
	// If Stop() doesn't work properly, the test will timeout
}