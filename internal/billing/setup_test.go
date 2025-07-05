package billing

import (
	"context"
	"testing"
)

// TestPolarSetupVerification verifies B-1 checkpoint requirements
func TestPolarSetupVerification(t *testing.T) {
	// Create mock client with default products
	client := NewMockPolarClient()
	client.SetupDefaultProducts()

	// List products
	products, err := client.ListProducts(context.Background())
	if err != nil {
		t.Fatalf("Failed to list products: %v", err)
	}

	// Verify we have exactly 3 products
	if len(products) != 3 {
		t.Errorf("Expected 3 products, got %d", len(products))
	}

	// Verify each product
	productMap := make(map[string]PolarProductResponse)
	for _, p := range products {
		productMap[p.ID] = p
	}

	// Test free-tier product
	t.Run("free-tier", func(t *testing.T) {
		p, ok := productMap["prod_free_tier"]
		if !ok {
			t.Fatal("Free tier product not found")
		}
		if len(p.Prices) == 0 || p.Prices[0].PriceAmount != 0 {
			t.Errorf("Free tier price should be 0")
		}
		if features, ok := p.Metadata["features"].(string); !ok || features != "10 hours/month included" {
			t.Error("Free tier should have '10 hours/month included' feature")
		}
	})

	// Test base-plus-usage product
	t.Run("base-plus-usage", func(t *testing.T) {
		p, ok := productMap["prod_base_plus_usage"]
		if !ok {
			t.Fatal("Base plus usage product not found")
		}
		if len(p.Prices) == 0 || p.Prices[0].PriceAmount != 2000 {
			t.Errorf("Base plus usage price should be 2000 cents ($20)")
		}
		if features, ok := p.Metadata["features"].(string); !ok || features != "200 small-tier hours/month included" {
			t.Error("Base plus usage should have '200 small-tier hours/month included' feature")
		}
	})

	// Test usage-only product
	t.Run("usage-only", func(t *testing.T) {
		p, ok := productMap["prod_usage_only"]
		if !ok {
			t.Fatal("Usage only product not found")
		}
		if len(p.Prices) == 0 || p.Prices[0].PriceAmount != 0 {
			t.Errorf("Usage only price should be 0")
		}
		if visibility, ok := p.Metadata["visibility"].(string); !ok || visibility != "private" {
			t.Error("Usage only should be private/hidden")
		}
	})
}

// TestSandboxCheckout simulates a sandbox checkout flow
func TestSandboxCheckout(t *testing.T) {
	client := NewMockPolarClient()
	client.SetupDefaultProducts()

	// Simulate customer subscribing to each product
	testCases := []struct {
		customerID string
		productID  string
		name       string
	}{
		{"cust-free-001", "prod_free_tier", "Free Tier"},
		{"cust-base-001", "prod_base_plus_usage", "Base Plus Usage"},
		{"cust-usage-001", "prod_usage_only", "Usage Only"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Add subscription
			client.AddSubscription(tc.customerID, tc.productID)

			// Verify subscription exists
			sub, err := client.GetSubscription(context.Background(), tc.customerID)
			if err != nil {
				t.Fatalf("Failed to get subscription: %v", err)
			}

			if sub.ProductID != tc.productID {
				t.Errorf("Expected product ID %s, got %s", tc.productID, sub.ProductID)
			}
			if sub.Status != "active" {
				t.Errorf("Expected status 'active', got %s", sub.Status)
			}
		})
	}
}
