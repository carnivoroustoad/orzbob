package billing

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// PolarClientInterface defines the interface for Polar API operations
type PolarClientInterface interface {
	ListProducts(ctx context.Context) ([]PolarProductResponse, error)
	CreateProduct(ctx context.Context, product PolarProductCreate) (*PolarProductResponse, error)
	RecordUsage(ctx context.Context, usage MeterUsageRecord) error
	GetSubscription(ctx context.Context, customerID string) (*SubscriptionResponse, error)
}

// MockPolarClient is a mock implementation for testing
type MockPolarClient struct {
	mu            sync.Mutex
	products      []PolarProductResponse
	usageRecords  []MeterUsageRecord
	subscriptions map[string]*SubscriptionResponse
	recordError   error // Simulate errors
}

// NewMockPolarClient creates a new mock client
func NewMockPolarClient() *MockPolarClient {
	return &MockPolarClient{
		products:      make([]PolarProductResponse, 0),
		usageRecords:  make([]MeterUsageRecord, 0),
		subscriptions: make(map[string]*SubscriptionResponse),
	}
}

// SetupDefaultProducts sets up the default products
func (m *MockPolarClient) SetupDefaultProducts() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now().Format(time.RFC3339)
	m.products = []PolarProductResponse{
		{
			ID:                "prod_free_tier",
			OrganizationID:    "org_test",
			Name:              "Free Tier",
			Description:       "Perfect for trying out Orzbob Cloud",
			RecurringInterval: "month",
			IsRecurring:       true,
			IsArchived:        false,
			Prices: []PolarPrice{
				{
					ID:                "price_free",
					RecurringInterval: "month",
					PriceAmount:       0,
					PriceCurrency:     "USD",
					ProductID:         "prod_free_tier",
					CreatedAt:         now,
				},
			},
			Metadata:   map[string]interface{}{"features": "10 hours/month included"},
			CreatedAt:  now,
			ModifiedAt: now,
		},
		{
			ID:                "prod_base_plus_usage",
			OrganizationID:    "org_test",
			Name:              "Base + Usage",
			Description:       "$20/month includes 200 small-tier hours",
			RecurringInterval: "month",
			IsRecurring:       true,
			IsArchived:        false,
			Prices: []PolarPrice{
				{
					ID:                "price_base",
					RecurringInterval: "month",
					PriceAmount:       2000,
					PriceCurrency:     "USD",
					ProductID:         "prod_base_plus_usage",
					CreatedAt:         now,
				},
			},
			Metadata:   map[string]interface{}{"features": "200 small-tier hours/month included"},
			CreatedAt:  now,
			ModifiedAt: now,
		},
		{
			ID:                "prod_usage_only",
			OrganizationID:    "org_test",
			Name:              "Usage Only",
			Description:       "Pay only for what you use",
			RecurringInterval: "month",
			IsRecurring:       true,
			IsArchived:        false,
			Prices: []PolarPrice{
				{
					ID:                "price_usage",
					RecurringInterval: "month",
					PriceAmount:       0,
					PriceCurrency:     "USD",
					ProductID:         "prod_usage_only",
					CreatedAt:         now,
				},
			},
			Metadata:   map[string]interface{}{"features": "Pay-as-you-go pricing", "visibility": "private"},
			CreatedAt:  now,
			ModifiedAt: now,
		},
	}
}

// ListProducts returns mock products
func (m *MockPolarClient) ListProducts(ctx context.Context) ([]PolarProductResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.recordError != nil {
		return nil, m.recordError
	}

	return append([]PolarProductResponse{}, m.products...), nil
}

// CreateProduct creates a mock product
func (m *MockPolarClient) CreateProduct(ctx context.Context, product PolarProductCreate) (*PolarProductResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.recordError != nil {
		return nil, m.recordError
	}

	now := time.Now().Format(time.RFC3339)
	response := &PolarProductResponse{
		ID:                fmt.Sprintf("prod_%d", len(m.products)+1),
		OrganizationID:    product.ProjectID,
		Name:              product.Name,
		Description:       product.Description,
		RecurringInterval: product.Price.Interval,
		IsRecurring:       true,
		IsArchived:        false,
		Prices: []PolarPrice{
			{
				ID:                fmt.Sprintf("price_%d", len(m.products)+1),
				RecurringInterval: product.Price.Interval,
				PriceAmount:       product.Price.Amount,
				PriceCurrency:     product.Price.Currency,
				ProductID:         fmt.Sprintf("prod_%d", len(m.products)+1),
				CreatedAt:         now,
			},
		},
		Metadata:   map[string]interface{}{"features": product.Features, "visibility": product.Visibility},
		CreatedAt:  now,
		ModifiedAt: now,
	}

	m.products = append(m.products, *response)
	return response, nil
}

// RecordUsage records mock usage
func (m *MockPolarClient) RecordUsage(ctx context.Context, usage MeterUsageRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.recordError != nil {
		return m.recordError
	}

	m.usageRecords = append(m.usageRecords, usage)
	return nil
}

// GetSubscription gets a mock subscription
func (m *MockPolarClient) GetSubscription(ctx context.Context, customerID string) (*SubscriptionResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.recordError != nil {
		return nil, m.recordError
	}

	sub, ok := m.subscriptions[customerID]
	if !ok {
		return nil, fmt.Errorf("no subscription found for customer %s", customerID)
	}

	return sub, nil
}

// AddSubscription adds a mock subscription
func (m *MockPolarClient) AddSubscription(customerID, productID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.subscriptions[customerID] = &SubscriptionResponse{
		ID:         fmt.Sprintf("sub_%s", customerID),
		CustomerID: customerID,
		ProductID:  productID,
		Status:     "active",
		CreatedAt:  time.Now(),
	}
}

// GetUsageRecords returns all recorded usage (for testing)
func (m *MockPolarClient) GetUsageRecords() []MeterUsageRecord {
	m.mu.Lock()
	defer m.mu.Unlock()

	return append([]MeterUsageRecord{}, m.usageRecords...)
}

// SetError sets an error to be returned by all methods
func (m *MockPolarClient) SetError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.recordError = err
}

// ClearError clears the error
func (m *MockPolarClient) ClearError() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.recordError = nil
}
