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

	products, err := client.ListProducts(context.Background())
	if err != nil {
		log.Fatalf("Failed to list products: %v", err)
	}

	fmt.Printf("Found %d products:\n\n", len(products))
	for _, p := range products {
		fmt.Printf("Product: %s\n", p.Name)
		fmt.Printf("  ID: %s\n", p.ID)
		fmt.Printf("  Description: %s\n", p.Description)
		fmt.Printf("  Recurring: %v (%s)\n", p.IsRecurring, p.RecurringInterval)
		fmt.Printf("  Archived: %v\n", p.IsArchived)
		if len(p.Prices) > 0 {
			price := p.Prices[0]
			fmt.Printf("  Price: $%.2f/%s\n", float64(price.PriceAmount)/100, price.RecurringInterval)
		}
		fmt.Println()
	}
}
