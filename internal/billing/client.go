package billing

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// PolarClient is a client for the Polar.sh API
type PolarClient struct {
	apiKey     string
	projectID  string
	baseURL    string
	httpClient *http.Client
}

// NewPolarClient creates a new Polar.sh API client
func NewPolarClient(apiKey, organizationID string) *PolarClient {
	return &PolarClient{
		apiKey:    apiKey,
		projectID: organizationID, // Using organizationID for now
		baseURL:   "https://api.polar.sh/v1",
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// SetBaseURL sets a custom base URL (for testing)
func (c *PolarClient) SetBaseURL(url string) {
	c.baseURL = url
}

// ListProducts lists all products in the project
func (c *PolarClient) ListProducts(ctx context.Context) ([]PolarProductResponse, error) {
	req, err := c.newRequest(ctx, "GET", "/products", nil)
	if err != nil {
		return nil, err
	}

	// Add organization_id query parameter
	q := req.URL.Query()
	q.Add("organization_id", c.projectID)
	req.URL.RawQuery = q.Encode()

	var response struct {
		Items []PolarProductResponse `json:"items"`
	}
	if err := c.do(req, &response); err != nil {
		return nil, err
	}

	return response.Items, nil
}

// CreateProduct creates a new product
func (c *PolarClient) CreateProduct(ctx context.Context, product PolarProductCreate) (*PolarProductResponse, error) {
	req, err := c.newRequest(ctx, "POST", "/products", product)
	if err != nil {
		return nil, err
	}

	var response PolarProductResponse
	if err := c.do(req, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

// RecordUsage records usage for metering
func (c *PolarClient) RecordUsage(ctx context.Context, usage MeterUsageRecord) error {
	req, err := c.newRequest(ctx, "POST", "/meters/orzbob_compute_hours/usage", usage)
	if err != nil {
		return err
	}

	return c.do(req, nil)
}

// GetSubscription gets a subscription by customer ID
func (c *PolarClient) GetSubscription(ctx context.Context, customerID string) (*SubscriptionResponse, error) {
	req, err := c.newRequest(ctx, "GET", fmt.Sprintf("/subscriptions?customer_id=%s", customerID), nil)
	if err != nil {
		return nil, err
	}

	var response struct {
		Items []SubscriptionResponse `json:"items"`
	}
	if err := c.do(req, &response); err != nil {
		return nil, err
	}

	if len(response.Items) == 0 {
		return nil, fmt.Errorf("no subscription found for customer %s", customerID)
	}

	return &response.Items[0], nil
}

// newRequest creates a new HTTP request
func (c *PolarClient) newRequest(ctx context.Context, method, path string, body interface{}) (*http.Request, error) {
	url := c.baseURL + path

	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal body: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	return req, nil
}

// do executes an HTTP request
func (c *PolarClient) do(req *http.Request, result interface{}) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("API error: %s (status %d): %s", req.URL.Path, resp.StatusCode, string(body))
	}

	if result != nil && len(body) > 0 {
		if err := json.Unmarshal(body, result); err != nil {
			return fmt.Errorf("failed to unmarshal response: %w", err)
		}
	}

	return nil
}

// API Types

// PolarProductCreate represents a product creation request
type PolarProductCreate struct {
	ProjectID   string   `json:"project_id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	PricingType string   `json:"pricing_type"`
	Price       Price    `json:"price"`
	Features    []string `json:"features"`
	Visibility  string   `json:"visibility,omitempty"`
}

// PolarProductResponse represents a product response
type PolarProductResponse struct {
	ID                string                 `json:"id"`
	OrganizationID    string                 `json:"organization_id"`
	Name              string                 `json:"name"`
	Description       string                 `json:"description"`
	RecurringInterval string                 `json:"recurring_interval"`
	IsRecurring       bool                   `json:"is_recurring"`
	IsArchived        bool                   `json:"is_archived"`
	Prices            []PolarPrice           `json:"prices"`
	Metadata          map[string]interface{} `json:"metadata"`
	CreatedAt         string                 `json:"created_at"`
	ModifiedAt        string                 `json:"modified_at"`
}

// PolarPrice represents a product price
type PolarPrice struct {
	ID                string `json:"id"`
	RecurringInterval string `json:"recurring_interval"`
	PriceAmount       int    `json:"price_amount"`
	PriceCurrency     string `json:"price_currency"`
	IsArchived        bool   `json:"is_archived"`
	ProductID         string `json:"product_id"`
	CreatedAt         string `json:"created_at"`
}

// Price represents a product price
type Price struct {
	Amount   int    `json:"amount"`
	Currency string `json:"currency"`
	Interval string `json:"interval"`
}

// MeterUsageRecord represents a usage record
type MeterUsageRecord struct {
	CustomerID string    `json:"customer_id"`
	Usage      float64   `json:"usage"`
	Timestamp  time.Time `json:"timestamp"`
	Metadata   Metadata  `json:"metadata"`
}

// Metadata for usage records
type Metadata struct {
	OrgID string `json:"org_id"`
	Tier  string `json:"tier"`
}

// SubscriptionResponse represents a subscription
type SubscriptionResponse struct {
	ID         string    `json:"id"`
	CustomerID string    `json:"customer_id"`
	ProductID  string    `json:"product_id"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
}
