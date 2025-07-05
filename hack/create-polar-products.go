//go:build tools
// +build tools

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"orzbob/internal/billing"
)

type CreateProductRequest struct {
	Name              string                 `json:"name"`
	Description       string                 `json:"description,omitempty"`
	IsRecurring       bool                   `json:"is_recurring"`
	RecurringInterval string                 `json:"recurring_interval"`
	IsArchived        bool                   `json:"is_archived"`
	Prices            []interface{}          `json:"prices"`
	Metadata          map[string]interface{} `json:"metadata,omitempty"`
}

type CreatePriceFixed struct {
	AmountType        string `json:"amount_type"`
	RecurringInterval string `json:"recurring_interval"`
	PriceAmount       int    `json:"price_amount"`
	PriceCurrency     string `json:"price_currency"`
}

type CreatePriceFree struct {
	AmountType        string `json:"amount_type"`
	RecurringInterval string `json:"recurring_interval"`
}

func main() {
	config := billing.LoadConfigOptional()
	if !config.IsConfigured() {
		log.Fatal("Polar credentials not configured")
	}

	// Products to create
	products := []CreateProductRequest{
		{
			Name:              "Orzbob Cloud - Free Tier",
			Description:       "Perfect for trying out Orzbob Cloud",
			IsRecurring:       true,
			RecurringInterval: "month",
			Prices: []interface{}{
				CreatePriceFree{
					AmountType:        "free",
					RecurringInterval: "month",
				},
			},
			Metadata: map[string]interface{}{
				"tier":     "free",
				"included": "10 hours/month",
				"features": "10 hours/month included, Small instances only (2 CPU, 4GB RAM), Community support, 30-minute idle timeout",
			},
		},
		{
			Name:              "Orzbob Cloud - Usage Only",
			Description:       "Pay only for what you use (no monthly fee)",
			IsRecurring:       true,
			RecurringInterval: "month",
			IsArchived:        true, // Archive immediately to hide it
			Prices: []interface{}{
				CreatePriceFree{
					AmountType:        "free",
					RecurringInterval: "month",
				},
			},
			Metadata: map[string]interface{}{
				"tier":       "usage-only",
				"visibility": "private",
				"features":   "No monthly fee, Pay-as-you-go pricing, All instance tiers",
			},
		},
	}

	client := &http.Client{}
	baseURL := "https://api.polar.sh/v1"

	// First, list existing products
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/products?organization_id=%s", baseURL, config.PolarOrgID), nil)
	if err != nil {
		log.Fatal(err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", config.PolarAPIKey))

	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	var existingResp struct {
		Items []struct {
			Name string `json:"name"`
		} `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&existingResp); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Found %d existing products:\n", len(existingResp.Items))
	existingNames := make(map[string]bool)
	for _, p := range existingResp.Items {
		fmt.Printf("  - %s\n", p.Name)
		existingNames[p.Name] = true
	}

	// Create missing products
	for _, product := range products {
		if existingNames[product.Name] {
			fmt.Printf("\nProduct '%s' already exists, skipping...\n", product.Name)
			continue
		}

		fmt.Printf("\nCreating product: %s\n", product.Name)
		
		body, err := json.Marshal(product)
		if err != nil {
			log.Printf("Failed to marshal product: %v", err)
			continue
		}

		req, err := http.NewRequest("POST", fmt.Sprintf("%s/products", baseURL), bytes.NewReader(body))
		if err != nil {
			log.Printf("Failed to create request: %v", err)
			continue
		}
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", config.PolarAPIKey))
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			log.Printf("Failed to create product: %v", err)
			continue
		}
		defer resp.Body.Close()

		respBody, _ := io.ReadAll(resp.Body)
		if resp.StatusCode >= 400 {
			log.Printf("Failed to create product (status %d): %s", resp.StatusCode, string(respBody))
			continue
		}

		var created struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		}
		if err := json.Unmarshal(respBody, &created); err != nil {
			log.Printf("Failed to parse response: %v", err)
			continue
		}

		fmt.Printf("  Created with ID: %s\n", created.ID)
	}

	fmt.Println("\nNote: The existing 'Orzbob' product at $20/month will serve as the 'Base + Usage' plan.")
}