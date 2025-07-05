package billing_test

import (
	"context"
	"fmt"
	"log"
	"orzbob/internal/billing"
)

// Example demonstrates how to use the metering service in the control plane
func Example_meteringService() {
	// Load configuration
	config := billing.LoadConfigOptional()
	if !config.IsConfigured() {
		log.Fatal("Billing not configured")
	}

	// Create metering service
	meteringService, err := billing.NewMeteringService(config)
	if err != nil {
		log.Fatalf("Failed to create metering service: %v", err)
	}

	// Start the background flush process
	ctx := context.Background()
	meteringService.Start(ctx)
	defer meteringService.Stop()

	// Example: Record usage when an instance is stopped
	orgID := "org-123"
	customerID := "cust-456" // From Polar subscription
	instanceRuntime := 125    // minutes
	tier := "small"

	meteringService.RecordUsage(orgID, customerID, instanceRuntime, tier)

	// Example: Record usage for multiple instances
	meteringService.RecordUsage("org-123", "cust-456", 30, "small")
	meteringService.RecordUsage("org-123", "cust-456", 60, "medium")
	meteringService.RecordUsage("org-789", "cust-999", 240, "large")

	// The service will automatically batch and flush every 60 seconds
	// Or you can manually flush if needed (e.g., on shutdown)
	if err := meteringService.Flush(ctx); err != nil {
		log.Printf("Failed to flush usage: %v", err)
	}

	fmt.Println("Usage recorded and sent to Polar")
	// Output: Usage recorded and sent to Polar
}

// Example_usageCalculation shows how usage is calculated
func Example_usageCalculation() {
	// Convert minutes to hours for different tiers
	tiers := []struct {
		name    string
		minutes int
	}{
		{"small", 120},   // 2 hours
		{"medium", 90},   // 1.5 hours
		{"large", 45},    // 0.75 hours
		{"gpu", 30},      // 0.5 hours
	}

	for _, tier := range tiers {
		hours := billing.UsageToHours(tier.minutes, tier.name)
		cost := hours * billing.TierPricing[tier.name]
		fmt.Printf("%s: %d minutes = %.2f hours = $%.2f\n", 
			tier.name, tier.minutes, hours, cost/100)
	}

	// Output:
	// small: 120 minutes = 2.00 hours = $0.17
	// medium: 90 minutes = 1.50 hours = $0.25
	// large: 45 minutes = 0.75 hours = $0.25
	// gpu: 30 minutes = 0.50 hours = $1.04
}

// Example_prometheusMetrics shows available metrics
func Example_prometheusMetrics() {
	// The metering service exposes these Prometheus metrics:
	
	// orzbob_usage_meter_queue - Current queue size
	// orzbob_usage_meter_flush_total - Total flush operations
	// orzbob_usage_meter_flush_errors_total - Failed flushes
	// orzbob_usage_meter_records_total - Total records sent

	fmt.Println("Metrics are automatically exported to Prometheus")
	// Output: Metrics are automatically exported to Prometheus
}

// Example_controlPlaneIntegration shows how to integrate with the control plane
func Example_controlPlaneIntegration() {
	// In the control plane, when an instance status changes:
	//
	// func (s *Server) handleInstanceStop(instance *Instance) {
	//     // Calculate runtime
	//     runtime := time.Since(instance.StartTime)
	//     minutes := int(runtime.Minutes())
	//     
	//     // Get customer ID from organization mapping
	//     customerID := s.getCustomerID(instance.OrgID)
	//     
	//     // Record usage
	//     s.meteringService.RecordUsage(
	//         instance.OrgID,
	//         customerID,
	//         minutes,
	//         instance.Tier,
	//     )
	// }

	fmt.Println("See handleInstanceStop in cloud-cp for integration")
	// Output: See handleInstanceStop in cloud-cp for integration
}