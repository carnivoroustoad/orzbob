package billing

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestPolarClient_ListProducts(t *testing.T) {
	// Mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/products" {
			t.Errorf("Expected path /v1/products, got %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("Expected Bearer token")
		}

		response := struct {
			Items []PolarProductResponse `json:"items"`
		}{
			Items: []PolarProductResponse{
				{
					ID:                "prod_free_tier",
					OrganizationID:    "org-123",
					Name:              "Free Tier",
					Description:       "Perfect for trying out Orzbob Cloud",
					RecurringInterval: "month",
					IsRecurring:       true,
					Prices: []PolarPrice{
						{
							ID:                "price_free",
							RecurringInterval: "month",
							PriceAmount:       0,
							PriceCurrency:     "USD",
							ProductID:         "prod_free_tier",
							CreatedAt:         "2025-06-22T00:00:00Z",
						},
					},
					CreatedAt:  "2025-06-22T00:00:00Z",
					ModifiedAt: "2025-06-22T00:00:00Z",
				},
			},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewPolarClient("test-key", "proj-123")
	client.SetBaseURL(server.URL + "/v1")

	products, err := client.ListProducts(context.Background())
	if err != nil {
		t.Fatalf("ListProducts failed: %v", err)
	}

	if len(products) != 1 {
		t.Errorf("Expected 1 product, got %d", len(products))
	}
	if products[0].Name != "Free Tier" {
		t.Errorf("Expected product name 'Free Tier', got %s", products[0].Name)
	}
}

func TestPolarClient_RecordUsage(t *testing.T) {
	// Mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/meters/orzbob_compute_hours/usage" {
			t.Errorf("Expected path /v1/meters/orzbob_compute_hours/usage, got %s", r.URL.Path)
		}

		var usage MeterUsageRecord
		if err := json.NewDecoder(r.Body).Decode(&usage); err != nil {
			t.Errorf("Failed to decode request body: %v", err)
		}

		if usage.CustomerID != "cust-123" {
			t.Errorf("Expected customer ID cust-123, got %s", usage.CustomerID)
		}
		if usage.Usage != 2.5 {
			t.Errorf("Expected usage 2.5, got %f", usage.Usage)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewPolarClient("test-key", "proj-123")
	client.SetBaseURL(server.URL + "/v1")

	err := client.RecordUsage(context.Background(), MeterUsageRecord{
		CustomerID: "cust-123",
		Usage:      2.5,
		Timestamp:  time.Now(),
		Metadata: Metadata{
			OrgID: "org-456",
			Tier:  "small",
		},
	})
	if err != nil {
		t.Fatalf("RecordUsage failed: %v", err)
	}
}

func TestPolarClient_ErrorHandling(t *testing.T) {
	// Mock server that returns errors
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error": "Invalid request"}`))
	}))
	defer server.Close()

	client := NewPolarClient("test-key", "proj-123")
	client.SetBaseURL(server.URL + "/v1")

	_, err := client.ListProducts(context.Background())
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if err.Error() == "" {
		t.Error("Expected error message")
	}
}