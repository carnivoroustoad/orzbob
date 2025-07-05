//go:build tools
// +build tools

package main

import (
	"context"
	"fmt"
	"log"
	"orzbob/internal/billing"
)

func main() {
	config := billing.LoadConfigOptional()
	if !config.IsConfigured() {
		log.Fatal("Polar credentials not configured")
	}

	client := billing.NewPolarClient(config.PolarAPIKey, config.PolarOrgID)

	// Products to create
	products := []billing.PolarProductCreate{
		{
			ProjectID:   config.PolarOrgID,
			Name:        "Orzbob Cloud - Free Tier",
			Description: "Perfect for trying out Orzbob Cloud",
			PricingType: "recurring",
			Price: billing.Price{
				Amount:   0,
				Currency: "USD",
				Interval: "month",
			},
			Features: []string{
				"10 hours/month included",
				"Small instances only (2 CPU, 4GB RAM)",
				"Community support",
				"30-minute idle timeout",
			},
			Visibility: "public",
		},
		{
			ProjectID:   config.PolarOrgID,
			Name:        "Orzbob Cloud - Base + Usage",
			Description: "$20/month includes 200 small-tier hours",
			PricingType: "recurring",
			Price: billing.Price{
				Amount:   2000, // $20.00
				Currency: "USD",
				Interval: "month",
			},
			Features: []string{
				"200 small-tier hours/month included",
				"All instance tiers available",
				"Priority support",
				"60-minute idle timeout",
				"Pay-as-you-go for additional usage",
			},
			Visibility: "public",
		},
		{
			ProjectID:   config.PolarOrgID,
			Name:        "Orzbob Cloud - Usage Only",
			Description: "Pay only for what you use",
			PricingType: "recurring",
			Price: billing.Price{
				Amount:   0,
				Currency: "USD",
				Interval: "month",
			},
			Features: []string{
				"No monthly fee",
				"Pay-as-you-go pricing",
				"All instance tiers",
			},
			Visibility: "private", // Hidden product
		},
	}

	// First, list existing products
	existing, err := client.ListProducts(context.Background())
	if err != nil {
		log.Fatalf("Failed to list existing products: %v", err)
	}

	fmt.Printf("Found %d existing products\n", len(existing))
	
	// Check if we already have the products
	existingNames := make(map[string]bool)
	for _, p := range existing {
		existingNames[p.Name] = true
		fmt.Printf("  - %s (ID: %s)\n", p.Name, p.ID)
	}

	// Create missing products
	for _, product := range products {
		if existingNames[product.Name] {
			fmt.Printf("\nProduct '%s' already exists, skipping...\n", product.Name)
			continue
		}

		fmt.Printf("\nCreating product: %s\n", product.Name)
		created, err := client.CreateProduct(context.Background(), product)
		if err != nil {
			log.Printf("Failed to create product %s: %v", product.Name, err)
			continue
		}
		fmt.Printf("  Created with ID: %s\n", created.ID)
	}

	// List final products
	fmt.Println("\nFinal product list:")
	final, err := client.ListProducts(context.Background())
	if err != nil {
		log.Fatalf("Failed to list final products: %v", err)
	}

	for _, p := range final {
		fmt.Printf("  - %s ($%.2f/%s) - %s\n", 
			p.Name, 
			float64(p.Price.Amount)/100, 
			p.Price.Interval,
			p.Visibility)
	}
}