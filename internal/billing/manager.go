package billing

import (
	"context"
	"fmt"
	"log"

	"orzbob/internal/notifications"
)

// Manager coordinates all billing services
type Manager struct {
	client          PolarClientInterface
	quotaEngine     *QuotaEngine
	meteringService *MeteringService
	alertService    *BudgetAlertService
	throttleService *ThrottleService
}

// NewManager creates a new billing manager
func NewManager(config Config) (*Manager, error) {
	// Create Polar client
	client := NewPolarClient(config.PolarAPIKey, config.PolarOrgID)
	return NewManagerWithClient(config, client)
}

// NewManagerWithClient creates a new billing manager with a specific client (for testing)
func NewManagerWithClient(config Config, client PolarClientInterface) (*Manager, error) {
	// Create quota engine
	quotaEngine, err := NewQuotaEngine(client, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create quota engine: %w", err)
	}

	// Create metering service
	meteringService, err := NewMeteringServiceWithClient(&config, client)
	if err != nil {
		return nil, fmt.Errorf("failed to create metering service: %w", err)
	}

	// Create email service
	emailService, err := notifications.NewEmailServiceFromEnv()
	if err != nil {
		log.Printf("Warning: Failed to create email service: %v. Budget alerts will be disabled.", err)
		// Continue without email alerts
	}

	// Create alert service (only if email is available)
	var alertService *BudgetAlertService
	if emailService != nil {
		alertService = NewBudgetAlertService(quotaEngine, emailService)
	}

	// Create throttle service
	throttleService := NewThrottleService(quotaEngine)

	return &Manager{
		client:          client,
		quotaEngine:     quotaEngine,
		meteringService: meteringService,
		alertService:    alertService,
		throttleService: throttleService,
	}, nil
}

// Start starts all billing services
func (m *Manager) Start(ctx context.Context) error {
	// Start metering service
	m.meteringService.Start(ctx)
	log.Println("Billing: Metering service started")

	// Start alert service if available
	if m.alertService != nil {
		m.alertService.Start(ctx)
		log.Println("Billing: Budget alert service started")
	} else {
		log.Println("Billing: Budget alerts disabled (no email service)")
	}

	// Start throttle service
	m.throttleService.Start(ctx)
	log.Println("Billing: Throttle service started")

	return nil
}

// Stop stops all billing services
func (m *Manager) Stop() {
	// Stop metering service
	m.meteringService.Stop()
	log.Println("Billing: Metering service stopped")

	// Stop alert service if running
	if m.alertService != nil {
		m.alertService.Stop()
		log.Println("Billing: Budget alert service stopped")
	}

	// Stop throttle service
	m.throttleService.Stop()
	log.Println("Billing: Throttle service stopped")
}

// RecordUsage records usage for an organization
func (m *Manager) RecordUsage(orgID, customerID string, minutes float64, tier string) error {
	// Record in quota engine
	if err := m.quotaEngine.RecordUsage(orgID, customerID, minutes/60.0); err != nil {
		return fmt.Errorf("failed to record quota usage: %w", err)
	}

	// Send to metering service
	m.meteringService.RecordUsage(orgID, customerID, int(minutes), tier)

	return nil
}

// GetUsage returns usage information for an organization
func (m *Manager) GetUsage(orgID string) (*UsageStatus, error) {
	return m.quotaEngine.GetUsageStatus(orgID)
}

// SetSubscription sets the subscription for an organization
func (m *Manager) SetSubscription(orgID, customerID string) error {
	// Update the organization's customer ID in the quota engine
	// This will trigger a subscription lookup on next usage record
	return m.quotaEngine.RecordUsage(orgID, customerID, 0)
}

// CheckQuota checks if an organization has exceeded their quota
func (m *Manager) CheckQuota(orgID string) (bool, error) {
	status, err := m.quotaEngine.GetUsageStatus(orgID)
	if err != nil {
		return false, err
	}
	return status.InOverage, nil
}

// GetClient returns the billing client
func (m *Manager) GetClient() PolarClientInterface {
	return m.client
}

// GetQuotaEngine returns the quota engine
func (m *Manager) GetQuotaEngine() *QuotaEngine {
	return m.quotaEngine
}

// GetMeteringService returns the metering service
func (m *Manager) GetMeteringService() *MeteringService {
	return m.meteringService
}

// GetAlertService returns the alert service (may be nil)
func (m *Manager) GetAlertService() *BudgetAlertService {
	return m.alertService
}

// GetThrottleService returns the throttle service
func (m *Manager) GetThrottleService() *ThrottleService {
	return m.throttleService
}

// RegisterInstance registers an instance for throttle tracking
func (m *Manager) RegisterInstance(instanceID, orgID string) {
	m.throttleService.RegisterInstance(instanceID, orgID)
}

// UnregisterInstance removes an instance from throttle tracking
func (m *Manager) UnregisterInstance(instanceID string) {
	m.throttleService.UnregisterInstance(instanceID)
}

// RecordInstanceActivity records activity for an instance (heartbeat)
func (m *Manager) RecordInstanceActivity(instanceID string) {
	m.throttleService.RecordActivity(instanceID)
}

// SetThrottlePauseCallback sets the callback for pausing instances
func (m *Manager) SetThrottlePauseCallback(callback func(instanceID string, reason ThrottleReason) error) {
	m.throttleService.SetPauseCallback(callback)
}