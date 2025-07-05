package billing

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// QuotaEngine tracks usage against plan quotas
type QuotaEngine struct {
	mu          sync.RWMutex
	usage       map[string]*OrgUsage // orgID -> usage
	client      PolarClientInterface
	persistence QuotaPersistence
}

// OrgUsage tracks an organization's usage for the current billing period
type OrgUsage struct {
	OrgID            string             `json:"org_id"`
	CustomerID       string             `json:"customer_id"`
	BillingPeriodStart time.Time        `json:"billing_period_start"`
	ProductID        string             `json:"product_id"`
	IncludedHours    float64            `json:"included_hours"`
	UsedHours        float64            `json:"used_hours"`
	InOverage        bool               `json:"in_overage"`
	LastUpdated      time.Time          `json:"last_updated"`
}

// QuotaPersistence interface for storing quota data
type QuotaPersistence interface {
	Save(ctx context.Context, usage map[string]*OrgUsage) error
	Load(ctx context.Context) (map[string]*OrgUsage, error)
}

// NewQuotaEngine creates a new quota engine
func NewQuotaEngine(client PolarClientInterface, persistence QuotaPersistence) (*QuotaEngine, error) {
	qe := &QuotaEngine{
		usage:       make(map[string]*OrgUsage),
		client:      client,
		persistence: persistence,
	}
	
	// Load existing usage data
	if persistence != nil {
		usage, err := persistence.Load(context.Background())
		if err != nil {
			log.Printf("Failed to load quota data: %v", err)
		} else {
			qe.usage = usage
			log.Printf("Loaded quota data for %d organizations", len(usage))
		}
	}
	
	// Start periodic persistence
	go qe.startPersistenceLoop()
	
	return qe, nil
}

// RecordUsage updates usage and checks quotas
func (qe *QuotaEngine) RecordUsage(orgID, customerID string, hours float64) error {
	qe.mu.Lock()
	defer qe.mu.Unlock()
	
	// Get or create org usage
	usage, exists := qe.usage[orgID]
	if !exists {
		usage = &OrgUsage{
			OrgID:              orgID,
			CustomerID:         customerID,
			BillingPeriodStart: qe.getCurrentBillingPeriodStart(),
		}
		qe.usage[orgID] = usage
	}
	
	// Reset if new billing period
	if qe.isNewBillingPeriod(usage.BillingPeriodStart) {
		usage.BillingPeriodStart = qe.getCurrentBillingPeriodStart()
		usage.UsedHours = 0
		usage.InOverage = false
		log.Printf("Reset usage for org %s - new billing period", orgID)
	}
	
	// Update customer ID if changed
	if usage.CustomerID != customerID && customerID != "" {
		usage.CustomerID = customerID
	}
	
	// Get subscription and included hours if not set
	if usage.ProductID == "" && qe.client != nil {
		if err := qe.updateSubscriptionInfo(usage); err != nil {
			log.Printf("Failed to get subscription for customer %s: %v", customerID, err)
			// Default to free tier if can't get subscription
			usage.ProductID = "prod_free_tier"
			usage.IncludedHours = 10
		}
	}
	
	// Update usage
	usage.UsedHours += hours
	usage.LastUpdated = time.Now()
	
	// Check if in overage
	if usage.UsedHours > usage.IncludedHours {
		if !usage.InOverage {
			usage.InOverage = true
			log.Printf("Org %s entered overage: %.2f hours used of %.2f included", 
				orgID, usage.UsedHours, usage.IncludedHours)
		}
	}
	
	return nil
}

// GetUsageStatus returns current usage status for an org
func (qe *QuotaEngine) GetUsageStatus(orgID string) (*UsageStatus, error) {
	qe.mu.RLock()
	defer qe.mu.RUnlock()
	
	usage, exists := qe.usage[orgID]
	if !exists {
		return &UsageStatus{
			OrgID:          orgID,
			IncludedHours:  10, // Default to free tier
			UsedHours:      0,
			RemainingHours: 10,
			InOverage:      false,
			ResetDate:      qe.getNextResetDate(),
		}, nil
	}
	
	// Check if needs reset
	if qe.isNewBillingPeriod(usage.BillingPeriodStart) {
		// Return as if reset (actual reset happens on next record)
		return &UsageStatus{
			OrgID:          orgID,
			ProductID:      usage.ProductID,
			IncludedHours:  usage.IncludedHours,
			UsedHours:      0,
			RemainingHours: usage.IncludedHours,
			InOverage:      false,
			ResetDate:      qe.getNextResetDate(),
		}, nil
	}
	
	remaining := usage.IncludedHours - usage.UsedHours
	if remaining < 0 {
		remaining = 0
	}
	
	return &UsageStatus{
		OrgID:          orgID,
		CustomerID:     usage.CustomerID,
		ProductID:      usage.ProductID,
		IncludedHours:  usage.IncludedHours,
		UsedHours:      usage.UsedHours,
		RemainingHours: remaining,
		InOverage:      usage.InOverage,
		PercentUsed:    (usage.UsedHours / usage.IncludedHours) * 100,
		ResetDate:      qe.getNextResetDate(),
	}, nil
}

// UsageStatus represents current usage against quota
type UsageStatus struct {
	OrgID          string    `json:"org_id"`
	CustomerID     string    `json:"customer_id,omitempty"`
	ProductID      string    `json:"product_id,omitempty"`
	IncludedHours  float64   `json:"included_hours"`
	UsedHours      float64   `json:"used_hours"`
	RemainingHours float64   `json:"remaining_hours"`
	InOverage      bool      `json:"in_overage"`
	PercentUsed    float64   `json:"percent_used"`
	ResetDate      time.Time `json:"reset_date"`
}

// updateSubscriptionInfo fetches subscription details from Polar
func (qe *QuotaEngine) updateSubscriptionInfo(usage *OrgUsage) error {
	if usage.CustomerID == "" {
		return fmt.Errorf("no customer ID")
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	sub, err := qe.client.GetSubscription(ctx, usage.CustomerID)
	if err != nil {
		return err
	}
	
	usage.ProductID = sub.ProductID
	
	// Map product to included hours
	switch sub.ProductID {
	case "prod_free_tier":
		usage.IncludedHours = 10
	case "prod_base_plus_usage", "c7e1bf1b-6c4f-45a8-8f72-2b4bdb2147fd": // Also check existing Orzbob product
		usage.IncludedHours = 200
	case "prod_usage_only":
		usage.IncludedHours = 0
	default:
		// Check our product definitions
		if product, ok := GetProduct(sub.ProductID); ok && product.Included != nil {
			usage.IncludedHours = product.Included.SmallHours
		} else {
			usage.IncludedHours = 0
		}
	}
	
	return nil
}

// Helper functions
func (qe *QuotaEngine) getCurrentBillingPeriodStart() time.Time {
	now := time.Now().UTC()
	return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
}

func (qe *QuotaEngine) isNewBillingPeriod(lastStart time.Time) bool {
	currentStart := qe.getCurrentBillingPeriodStart()
	return lastStart.Before(currentStart)
}

func (qe *QuotaEngine) getNextResetDate() time.Time {
	now := time.Now().UTC()
	// First day of next month
	if now.Month() == 12 {
		return time.Date(now.Year()+1, 1, 1, 0, 0, 0, 0, time.UTC)
	}
	return time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, time.UTC)
}

// startPersistenceLoop saves usage data periodically
func (qe *QuotaEngine) startPersistenceLoop() {
	if qe.persistence == nil {
		return
	}
	
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	
	for range ticker.C {
		qe.mu.RLock()
		usageCopy := make(map[string]*OrgUsage)
		for k, v := range qe.usage {
			usageCopy[k] = v
		}
		qe.mu.RUnlock()
		
		if err := qe.persistence.Save(context.Background(), usageCopy); err != nil {
			log.Printf("Failed to persist quota data: %v", err)
		}
	}
}

// GetAllUsage returns usage for all organizations (for admin/monitoring)
func (qe *QuotaEngine) GetAllUsage() map[string]*UsageStatus {
	qe.mu.RLock()
	defer qe.mu.RUnlock()
	
	result := make(map[string]*UsageStatus)
	for orgID := range qe.usage {
		status, _ := qe.GetUsageStatus(orgID)
		result[orgID] = status
	}
	return result
}