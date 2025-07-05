package billing

import "time"

// PolarProduct represents a product/SKU in Polar.sh
type PolarProduct struct {
	ID          string
	Name        string
	Description string
	PriceMonth  int    // in cents
	Hidden      bool   // for usage-only plan
	Included    *Usage // included usage per month
}

// Usage represents resource usage
type Usage struct {
	SmallHours  float64
	MediumHours float64
	LargeHours  float64
	GPUHours    float64
}

// TierPricing defines the hourly pricing for each tier in cents
var TierPricing = map[string]float64{
	"small":  8.3,   // $0.083/hour
	"medium": 16.7,  // $0.167/hour
	"large":  33.3,  // $0.333/hour
	"gpu":    208.0, // $2.08/hour
}

// Products defines the available Polar.sh products
var Products = map[string]PolarProduct{
	"free-tier": {
		ID:          "prod_free_tier",
		Name:        "Free Tier",
		Description: "Perfect for trying out Orzbob Cloud",
		PriceMonth:  0,
		Hidden:      false,
		Included: &Usage{
			SmallHours: 10, // 10 hours of small tier included
		},
	},
	"base-plus-usage": {
		ID:          "prod_base_plus_usage",
		Name:        "Base + Usage",
		Description: "$20/month includes 200 small-tier hours, then pay-as-you-go",
		PriceMonth:  2000, // $20
		Hidden:      false,
		Included: &Usage{
			SmallHours: 200, // 200 hours of small tier included
		},
	},
	"usage-only": {
		ID:          "prod_usage_only",
		Name:        "Usage Only",
		Description: "Pay only for what you use, no monthly fee",
		PriceMonth:  0,
		Hidden:      true, // Hidden product
		Included:    nil,  // No included hours
	},
}

// GetProduct returns a product by ID
func GetProduct(productID string) (*PolarProduct, bool) {
	for _, product := range Products {
		if product.ID == productID {
			return &product, true
		}
	}
	return nil, false
}

// CalculateOverage calculates overage charges for usage beyond included hours
func CalculateOverage(product *PolarProduct, usage *Usage) (overageHours *Usage, totalCents int) {
	if product.Included == nil {
		// Usage-only plan - all usage is charged
		overageHours = usage
	} else {
		// Calculate overage by subtracting included from actual
		overageHours = &Usage{
			SmallHours:  max(0, usage.SmallHours-product.Included.SmallHours),
			MediumHours: max(0, usage.MediumHours-product.Included.MediumHours),
			LargeHours:  max(0, usage.LargeHours-product.Included.LargeHours),
			GPUHours:    max(0, usage.GPUHours-product.Included.GPUHours),
		}
	}

	// Calculate total cost
	totalCents = int(
		overageHours.SmallHours*TierPricing["small"] +
			overageHours.MediumHours*TierPricing["medium"] +
			overageHours.LargeHours*TierPricing["large"] +
			overageHours.GPUHours*TierPricing["gpu"])

	return overageHours, totalCents
}

// UsageToHours converts usage minutes to hours for a specific tier
func UsageToHours(minutes int, tier string) float64 {
	return float64(minutes) / 60.0
}

// MonthlyReset returns the next monthly reset time
func MonthlyReset() time.Time {
	now := time.Now()
	// Reset on the 1st of next month
	return time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, time.UTC)
}

func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}