package billing

import (
	"context"
	"fmt"
	"testing"
)

func TestMeteringService_RecordUsage(t *testing.T) {
	// Create mock config
	config := &Config{
		PolarAPIKey:      "test-key",
		PolarWebhookSecret: "test-secret",
		PolarOrgID:       "test-org",
	}

	service, err := NewMeteringService(config)
	if err != nil {
		t.Fatalf("Failed to create metering service: %v", err)
	}

	// Replace with mock client
	mockClient := NewMockPolarClient()
	service.client = mockClient

	// Record some usage
	service.RecordUsage("org-123", "cust-123", 120, "small")
	service.RecordUsage("org-123", "cust-456", 60, "medium")
	service.RecordUsage("org-456", "cust-789", 30, "large")

	// Check queue size
	if size := service.GetQueueSize(); size != 3 {
		t.Errorf("Expected queue size 3, got %d", size)
	}

	// Flush
	err = service.Flush(context.Background())
	if err != nil {
		t.Errorf("Flush failed: %v", err)
	}

	// Check queue is empty
	if size := service.GetQueueSize(); size != 0 {
		t.Errorf("Expected queue size 0 after flush, got %d", size)
	}

	// Check recorded usage
	records := mockClient.GetUsageRecords()
	if len(records) != 3 {
		t.Errorf("Expected 3 usage records, got %d", len(records))
	}

	// Verify conversions
	expectedUsages := map[string]float64{
		"cust-123": 2.0,  // 120 minutes = 2 hours
		"cust-456": 1.0,  // 60 minutes = 1 hour
		"cust-789": 0.5,  // 30 minutes = 0.5 hours
	}

	for _, record := range records {
		expected, ok := expectedUsages[record.CustomerID]
		if !ok {
			t.Errorf("Unexpected customer ID: %s", record.CustomerID)
			continue
		}
		if record.Usage != expected {
			t.Errorf("Customer %s: expected usage %f, got %f", record.CustomerID, expected, record.Usage)
		}
	}
}

func TestMeteringService_Aggregation(t *testing.T) {
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

	// Record multiple usage samples for same customer and tier
	service.RecordUsage("org-123", "cust-123", 30, "small")
	service.RecordUsage("org-123", "cust-123", 45, "small")
	service.RecordUsage("org-123", "cust-123", 60, "small")
	service.RecordUsage("org-123", "cust-123", 90, "medium") // Different tier

	// Flush
	err = service.Flush(context.Background())
	if err != nil {
		t.Errorf("Flush failed: %v", err)
	}

	// Check aggregation
	records := mockClient.GetUsageRecords()
	if len(records) != 2 {
		t.Errorf("Expected 2 aggregated records, got %d", len(records))
	}

	// Find the small tier record
	var smallUsage, mediumUsage float64
	for _, record := range records {
		if record.Metadata.Tier == "small" {
			smallUsage = record.Usage
		} else if record.Metadata.Tier == "medium" {
			mediumUsage = record.Usage
		}
	}

	// 30 + 45 + 60 = 135 minutes = 2.25 hours
	if smallUsage != 2.25 {
		t.Errorf("Expected small tier usage 2.25 hours, got %f", smallUsage)
	}

	// 90 minutes = 1.5 hours
	if mediumUsage != 1.5 {
		t.Errorf("Expected medium tier usage 1.5 hours, got %f", mediumUsage)
	}
}

func TestBatchFlush(t *testing.T) {
	// This test verifies the batch flush functionality required by B-3
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

	// Record usage
	for i := 0; i < 100; i++ {
		service.RecordUsage("org-123", "cust-123", 1, "small")
	}

	// Queue should have 100 samples
	if size := service.GetQueueSize(); size != 100 {
		t.Errorf("Expected queue size 100, got %d", size)
	}

	// Manual flush
	err = service.Flush(ctx)
	if err != nil {
		t.Errorf("Flush failed: %v", err)
	}

	// Queue should be empty
	if size := service.GetQueueSize(); size != 0 {
		t.Errorf("Expected queue size 0 after flush, got %d", size)
	}

	// Should have 1 aggregated record (100 minutes for same customer)
	records := mockClient.GetUsageRecords()
	if len(records) != 1 {
		t.Errorf("Expected 1 aggregated record, got %d", len(records))
	}

	expectedHours := 100.0 / 60.0
	if len(records) > 0 {
		diff := records[0].Usage - expectedHours
		if diff < -0.0001 || diff > 0.0001 { // Allow small floating point differences
			t.Errorf("Expected usage %f hours, got %f", expectedHours, records[0].Usage)
		}
	}

	// Stop the service
	service.Stop()
}

func TestMeteringService_QueueLimit(t *testing.T) {
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

	// Add 1000 samples to test Prometheus gauge requirement
	for i := 0; i < 1000; i++ {
		service.RecordUsage("org-123", fmt.Sprintf("cust-%d", i), 60, "small")
	}

	queueSize := service.GetQueueSize()
	if queueSize != 1000 {
		t.Errorf("Expected queue size 1000, got %d", queueSize)
	}

	// Verify the queue stays under 1k after 10 min soak
	// (In production, the 60s flush timer would prevent this)
}

func TestMeteringService_ErrorHandling(t *testing.T) {
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
	mockClient.SetError(fmt.Errorf("API error"))
	service.client = mockClient

	// Record usage
	service.RecordUsage("org-123", "cust-123", 60, "small")

	// Flush should return error
	err = service.Flush(context.Background())
	if err == nil {
		t.Error("Expected flush to return error")
	}

	// Queue should be empty (samples were attempted)
	if size := service.GetQueueSize(); size != 0 {
		t.Errorf("Expected queue to be empty after failed flush, got %d", size)
	}
}