package billing

import (
	"context"
	"testing"
	"time"

	"orzbob/internal/notifications"
)

// mockEmailService captures emails for testing
type mockEmailService struct {
	sentEmails []mockEmail
}

type mockEmail struct {
	to   []string
	data notifications.BudgetAlertData
}

func (m *mockEmailService) SendBudgetAlert(ctx context.Context, to []string, data notifications.BudgetAlertData) error {
	m.sentEmails = append(m.sentEmails, mockEmail{to: to, data: data})
	return nil
}

func TestBudgetAlertService_SendAlerts(t *testing.T) {
	// Create quota engine with test data
	mockClient := NewMockPolarClient()
	mockClient.SetupDefaultProducts()
	quotaEngine, _ := NewQuotaEngine(mockClient, NewMemoryQuotaPersistence())
	
	// Create mock email service
	mockEmail := &mockEmailService{}
	
	// Create budget alert service with mock email
	alertService := NewBudgetAlertService(quotaEngine, mockEmail)

	tests := []struct {
		name               string
		orgID              string
		customerID         string
		subscription       string
		hoursUsed          float64
		expectedAlerts     int
		expectedThresholds []int
	}{
		{
			name:               "No alert under 50%",
			orgID:              "org1",
			customerID:         "customer-org1",
			subscription:       "prod_free_tier",
			hoursUsed:          4, // 40% of 10 hours
			expectedAlerts:     0,
			expectedThresholds: []int{},
		},
		{
			name:               "Alert at 50%",
			orgID:              "org2",
			customerID:         "customer-org2",
			subscription:       "prod_free_tier",
			hoursUsed:          5, // 50% of 10 hours
			expectedAlerts:     1,
			expectedThresholds: []int{50},
		},
		{
			name:               "Alert at 90%",
			orgID:              "org3",
			customerID:         "customer-org3",
			subscription:       "prod_free_tier",
			hoursUsed:          9, // 90% of 10 hours
			expectedAlerts:     2, // Should get both 50% and 90% alerts
			expectedThresholds: []int{50, 90},
		},
		{
			name:               "Alert for overage",
			orgID:              "org4",
			customerID:         "customer-org4",
			subscription:       "prod_free_tier",
			hoursUsed:          12, // 120% of 10 hours
			expectedAlerts:     2,  // Both alerts
			expectedThresholds: []int{50, 90},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset mock emails
			mockEmail.sentEmails = nil
			alertService.ResetAlerts()

			// Set subscription and usage
			mockClient.AddSubscription(tt.customerID, tt.subscription)
			quotaEngine.RecordUsage(tt.orgID, tt.customerID, tt.hoursUsed)

			// Check organization
			ctx := context.Background()
			err := alertService.checkOrganization(ctx, tt.orgID)
			if err != nil {
				t.Fatalf("checkOrganization() error = %v", err)
			}

			// Verify alerts sent
			if len(mockEmail.sentEmails) != tt.expectedAlerts {
				t.Errorf("Expected %d alerts, got %d", tt.expectedAlerts, len(mockEmail.sentEmails))
			}

			// Verify thresholds
			alertStatus := alertService.GetAlertStatus(tt.orgID)
			for _, threshold := range tt.expectedThresholds {
				if _, exists := alertStatus[threshold]; !exists {
					t.Errorf("Expected alert for %d%% threshold, but not found", threshold)
				}
			}
		})
	}
}

func TestBudgetAlertService_NoRepeatAlerts(t *testing.T) {
	// Create services
	mockClient := NewMockPolarClient()
	mockClient.SetupDefaultProducts()
	quotaEngine, _ := NewQuotaEngine(mockClient, NewMemoryQuotaPersistence())
	mockEmail := &mockEmailService{}
	
	alertService := NewBudgetAlertService(quotaEngine, mockEmail)

	orgID := "test-org"
	customerID := "customer-test-org"

	// Set subscription
	mockClient.AddSubscription(customerID, "prod_free_tier")

	// First check - should send alert
	quotaEngine.RecordUsage(orgID, customerID, 9) // 90% usage
	ctx := context.Background()
	
	err := alertService.checkOrganization(ctx, orgID)
	if err != nil {
		t.Fatalf("First check failed: %v", err)
	}

	if len(mockEmail.sentEmails) != 2 { // 50% and 90% alerts
		t.Errorf("Expected 2 alerts on first check, got %d", len(mockEmail.sentEmails))
	}

	// Second check - should not send alert again
	mockEmail.sentEmails = nil
	err = alertService.checkOrganization(ctx, orgID)
	if err != nil {
		t.Fatalf("Second check failed: %v", err)
	}

	if len(mockEmail.sentEmails) != 0 {
		t.Errorf("Expected 0 alerts on second check (already sent), got %d", len(mockEmail.sentEmails))
	}
}

func TestBudgetAlertService_ResetAfterBillingPeriod(t *testing.T) {
	// Create services
	mockClient := NewMockPolarClient()
	mockClient.SetupDefaultProducts()
	quotaEngine, _ := NewQuotaEngine(mockClient, NewMemoryQuotaPersistence())
	mockEmail := &mockEmailService{}
	
	alertService := NewBudgetAlertService(quotaEngine, mockEmail)

	orgID := "test-org"
	customerID := "customer-test-org"
	ctx := context.Background()

	// Set subscription
	mockClient.AddSubscription(customerID, "prod_free_tier")

	// Send alerts
	quotaEngine.RecordUsage(orgID, customerID, 9) // 90% usage
	alertService.checkOrganization(ctx, orgID)

	// Manually set alert sent time to last month
	alertService.alertsMu.Lock()
	if alertService.alertsSent[orgID] == nil {
		alertService.alertsSent[orgID] = make(map[int]time.Time)
	}
	lastMonth := time.Now().AddDate(0, -1, 0)
	alertService.alertsSent[orgID][50] = lastMonth
	alertService.alertsSent[orgID][90] = lastMonth
	alertService.alertsMu.Unlock()

	// Check again - should send alerts since it's a new billing period
	mockEmail.sentEmails = nil
	err := alertService.checkOrganization(ctx, orgID)
	if err != nil {
		t.Fatalf("Check after billing period reset failed: %v", err)
	}

	// Should send alerts again since it's a new billing period
	if len(mockEmail.sentEmails) != 2 {
		t.Errorf("Expected 2 alerts after billing period reset, got %d", len(mockEmail.sentEmails))
	}
}