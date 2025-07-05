package tiers

import "time"

// TierConfig represents the configuration for a specific tier
type TierConfig struct {
	Name               string
	MaxInstances       int
	AllowedInstanceTypes []string
	MaxCPU             string
	MaxMemory          string
	MaxStorage         string
	IdleTimeout        time.Duration
	MaxSessionDuration time.Duration
	Features           []string
}

// PlanConfig represents a subscription plan
type PlanConfig struct {
	Name        string
	MonthlyPrice int // in cents
	Tiers       []TierConfig
}

var (
	// FreeTier configuration
	FreeTier = PlanConfig{
		Name:         "free",
		MonthlyPrice: 0,
		Tiers: []TierConfig{
			{
				Name:                 "free",
				MaxInstances:         2,
				AllowedInstanceTypes: []string{"small"},
				MaxCPU:               "2",
				MaxMemory:            "4Gi",
				MaxStorage:           "10Gi",
				IdleTimeout:          30 * time.Minute,
				MaxSessionDuration:   4 * time.Hour,
				Features: []string{
					"basic_support",
					"community_forum",
				},
			},
		},
	}

	// ProTier configuration
	ProTier = PlanConfig{
		Name:         "pro",
		MonthlyPrice: 2900, // $29
		Tiers: []TierConfig{
			{
				Name:                 "pro",
				MaxInstances:         5,
				AllowedInstanceTypes: []string{"small", "medium", "large"},
				MaxCPU:               "8",
				MaxMemory:            "16Gi",
				MaxStorage:           "50Gi",
				IdleTimeout:          60 * time.Minute,
				MaxSessionDuration:   8 * time.Hour,
				Features: []string{
					"priority_support",
					"custom_domains",
					"advanced_metrics",
					"sla_99_5",
				},
			},
		},
	}

	// TeamTier configuration
	TeamTier = PlanConfig{
		Name:         "team",
		MonthlyPrice: 9900, // $99
		Tiers: []TierConfig{
			{
				Name:                 "team",
				MaxInstances:         20,
				AllowedInstanceTypes: []string{"small", "medium", "large"},
				MaxCPU:               "16",
				MaxMemory:            "32Gi",
				MaxStorage:           "200Gi",
				IdleTimeout:          120 * time.Minute,
				MaxSessionDuration:   24 * time.Hour,
				Features: []string{
					"dedicated_support",
					"team_management",
					"audit_logs",
					"custom_integrations",
					"sla_99_9",
					"instance_sharing",
				},
			},
		},
	}

	// EnterpriseTier configuration
	EnterpriseTier = PlanConfig{
		Name:         "enterprise",
		MonthlyPrice: -1, // Custom pricing
		Tiers: []TierConfig{
			{
				Name:                 "enterprise",
				MaxInstances:         -1, // Unlimited
				AllowedInstanceTypes: []string{"small", "medium", "large", "gpu"},
				MaxCPU:               "64",
				MaxMemory:            "256Gi",
				MaxStorage:           "1Ti",
				IdleTimeout:          -1, // No timeout
				MaxSessionDuration:   -1, // No limit
				Features: []string{
					"24x7_support",
					"dedicated_account_manager",
					"custom_sla",
					"on_premise_option",
					"compliance_reports",
					"custom_features",
					"white_label",
				},
			},
		},
	}
)

// GetPlan returns the plan configuration for a given plan name
func GetPlan(planName string) *PlanConfig {
	switch planName {
	case "free":
		return &FreeTier
	case "pro":
		return &ProTier
	case "team":
		return &TeamTier
	case "enterprise":
		return &EnterpriseTier
	default:
		return &FreeTier
	}
}

// IsInstanceTypeAllowed checks if an instance type is allowed for a plan
func IsInstanceTypeAllowed(plan *PlanConfig, instanceType string) bool {
	for _, tier := range plan.Tiers {
		for _, allowed := range tier.AllowedInstanceTypes {
			if allowed == instanceType {
				return true
			}
		}
	}
	return false
}

// GetMaxInstances returns the maximum number of instances for a plan
func GetMaxInstances(plan *PlanConfig) int {
	if len(plan.Tiers) > 0 {
		return plan.Tiers[0].MaxInstances
	}
	return 0
}