package billing

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"orzbob/internal/notifications"
)

// AlertThreshold represents a usage threshold for sending alerts
type AlertThreshold struct {
	Percentage int
	SentAt     time.Time
}

// EmailSender interface for sending budget alerts
type EmailSender interface {
	SendBudgetAlert(ctx context.Context, to []string, data notifications.BudgetAlertData) error
}

// BudgetAlertService manages budget alerts for organizations
type BudgetAlertService struct {
	quotaEngine   *QuotaEngine
	emailService  EmailSender
	alertsSent    map[string]map[int]time.Time // orgID -> percentage -> sentAt
	alertsMu      sync.RWMutex
	checkInterval time.Duration
	stopCh        chan struct{}
	wg            sync.WaitGroup
}

// NewBudgetAlertService creates a new budget alert service
func NewBudgetAlertService(quotaEngine *QuotaEngine, emailService EmailSender) *BudgetAlertService {
	return &BudgetAlertService{
		quotaEngine:   quotaEngine,
		emailService:  emailService,
		alertsSent:    make(map[string]map[int]time.Time),
		checkInterval: 1 * time.Hour, // Check hourly by default
		stopCh:        make(chan struct{}),
	}
}

// Start begins the budget alert checking routine
func (b *BudgetAlertService) Start(ctx context.Context) {
	b.wg.Add(1)
	go b.checkRoutine(ctx)
}

// Stop stops the budget alert service
func (b *BudgetAlertService) Stop() {
	close(b.stopCh)
	b.wg.Wait()
}

// checkRoutine runs periodically to check usage and send alerts
func (b *BudgetAlertService) checkRoutine(ctx context.Context) {
	defer b.wg.Done()

	// Check immediately on start
	b.checkAllOrganizations(ctx)

	ticker := time.NewTicker(b.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-b.stopCh:
			return
		case <-ticker.C:
			b.checkAllOrganizations(ctx)
		}
	}
}

// checkAllOrganizations checks usage for all organizations
func (b *BudgetAlertService) checkAllOrganizations(ctx context.Context) {
	// Get all organizations from quota engine
	usageMap := b.quotaEngine.GetAllUsage()

	for orgID := range usageMap {
		if err := b.checkOrganization(ctx, orgID); err != nil {
			log.Printf("Error checking budget alerts for org %s: %v", orgID, err)
		}
	}
}

// checkOrganization checks a single organization's usage and sends alerts if needed
func (b *BudgetAlertService) checkOrganization(ctx context.Context, orgID string) error {
	// Get current usage
	usage, err := b.quotaEngine.GetUsageStatus(orgID)
	if err != nil {
		return fmt.Errorf("failed to get usage: %w", err)
	}

	// Calculate percentage used
	percentageUsed := int(usage.PercentUsed)
	if percentageUsed < 50 {
		return nil // No alerts needed yet
	}

	// Check which alerts to send
	thresholds := []int{50, 90}
	for _, threshold := range thresholds {
		if percentageUsed >= threshold {
			if err := b.sendAlertIfNeeded(ctx, orgID, usage, threshold); err != nil {
				log.Printf("Failed to send %d%% alert for org %s: %v", threshold, orgID, err)
			}
		}
	}

	return nil
}

// sendAlertIfNeeded sends an alert if it hasn't been sent this billing period
func (b *BudgetAlertService) sendAlertIfNeeded(ctx context.Context, orgID string, usage *UsageStatus, threshold int) error {
	// Check if we've already sent this alert this billing period
	b.alertsMu.RLock()
	orgAlerts, exists := b.alertsSent[orgID]
	if exists {
		sentAt, alertSent := orgAlerts[threshold]
		if alertSent {
			// Check if it was sent in the current billing period
			// Use the reset date to determine the billing period start
			billingPeriodStart := usage.ResetDate.AddDate(0, -1, 0)
			if sentAt.After(billingPeriodStart) {
				b.alertsMu.RUnlock()
				return nil // Already sent this period
			}
		}
	}
	b.alertsMu.RUnlock()

	// Send the alert
	if err := b.sendAlert(ctx, orgID, usage, threshold); err != nil {
		return err
	}

	// Record that we sent the alert
	b.alertsMu.Lock()
	if b.alertsSent[orgID] == nil {
		b.alertsSent[orgID] = make(map[int]time.Time)
	}
	b.alertsSent[orgID][threshold] = time.Now()
	b.alertsMu.Unlock()

	return nil
}

// sendAlert sends a budget alert email
func (b *BudgetAlertService) sendAlert(ctx context.Context, orgID string, usage *UsageStatus, threshold int) error {
	// Get organization details (in a real system, this would come from a database)
	orgEmail := b.getOrganizationEmail(orgID)
	if orgEmail == "" {
		return fmt.Errorf("no email found for organization %s", orgID)
	}

	// Calculate actual percentage
	percentageUsed := int(usage.PercentUsed)

	// Send email
	data := notifications.BudgetAlertData{
		OrgName:        orgID, // In a real system, we'd have the org name
		HoursUsed:      usage.UsedHours,
		HoursIncluded:  usage.IncludedHours,
		PercentageUsed: percentageUsed,
		ResetDate:      usage.ResetDate,
		ManagePlanURL:  fmt.Sprintf("https://orzbob.cloud/organizations/%s/billing", orgID),
	}

	return b.emailService.SendBudgetAlert(ctx, []string{orgEmail}, data)
}

// getOrganizationEmail retrieves the email for an organization
// In a real implementation, this would query a database
func (b *BudgetAlertService) getOrganizationEmail(orgID string) string {
	// This is a placeholder - in reality, we'd fetch from database
	// For testing, we'll use environment variables or a config map
	return fmt.Sprintf("admin@%s.example.com", orgID)
}

// SetCheckInterval sets how often to check for budget alerts
func (b *BudgetAlertService) SetCheckInterval(interval time.Duration) {
	b.checkInterval = interval
}

// ResetAlerts clears all sent alerts (useful for testing)
func (b *BudgetAlertService) ResetAlerts() {
	b.alertsMu.Lock()
	b.alertsSent = make(map[string]map[int]time.Time)
	b.alertsMu.Unlock()
}

// GetAlertStatus returns the alert status for an organization
func (b *BudgetAlertService) GetAlertStatus(orgID string) map[int]time.Time {
	b.alertsMu.RLock()
	defer b.alertsMu.RUnlock()

	if alerts, exists := b.alertsSent[orgID]; exists {
		// Return a copy to avoid race conditions
		result := make(map[int]time.Time)
		for k, v := range alerts {
			result[k] = v
		}
		return result
	}

	return nil
}
