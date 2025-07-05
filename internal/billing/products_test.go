package billing

import (
	"testing"
)

func TestPolarProducts(t *testing.T) {
	tests := []struct {
		name         string
		productKey   string
		wantPrice    int
		wantHidden   bool
		wantIncluded float64 // small hours included
	}{
		{
			name:         "Free tier",
			productKey:   "free-tier",
			wantPrice:    0,
			wantHidden:   false,
			wantIncluded: 10,
		},
		{
			name:         "Base plus usage",
			productKey:   "base-plus-usage",
			wantPrice:    2000,
			wantHidden:   false,
			wantIncluded: 200,
		},
		{
			name:         "Usage only",
			productKey:   "usage-only",
			wantPrice:    0,
			wantHidden:   true,
			wantIncluded: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			product, ok := Products[tt.productKey]
			if !ok {
				t.Fatalf("Product %s not found", tt.productKey)
			}

			if product.PriceMonth != tt.wantPrice {
				t.Errorf("Price = %d, want %d", product.PriceMonth, tt.wantPrice)
			}

			if product.Hidden != tt.wantHidden {
				t.Errorf("Hidden = %v, want %v", product.Hidden, tt.wantHidden)
			}

			included := float64(0)
			if product.Included != nil {
				included = product.Included.SmallHours
			}
			if included != tt.wantIncluded {
				t.Errorf("Included small hours = %f, want %f", included, tt.wantIncluded)
			}
		})
	}
}

func TestCalculateOverage(t *testing.T) {
	tests := []struct {
		name       string
		product    string
		usage      *Usage
		wantCents  int
		wantSmall  float64
		wantMedium float64
	}{
		{
			name:    "Free tier under limit",
			product: "free-tier",
			usage: &Usage{
				SmallHours: 5,
			},
			wantCents: 0,
			wantSmall: 0,
		},
		{
			name:    "Free tier overage",
			product: "free-tier",
			usage: &Usage{
				SmallHours: 15, // 5 hours overage
			},
			wantCents: 41, // 5 * 8.3 = 41.5 cents
			wantSmall: 5,
		},
		{
			name:    "Base plan with multiple tiers",
			product: "base-plus-usage",
			usage: &Usage{
				SmallHours:  250, // 50 hours overage
				MediumHours: 20,  // 20 hours overage
			},
			wantCents:  415 + 334, // (50 * 8.3) + (20 * 16.7)
			wantSmall:  50,
			wantMedium: 20,
		},
		{
			name:    "Usage only all charged",
			product: "usage-only",
			usage: &Usage{
				SmallHours:  100,
				MediumHours: 50,
				LargeHours:  10,
				GPUHours:    1,
			},
			wantCents: 830 + 835 + 333 + 208, // All usage charged
			wantSmall: 100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			product := Products[tt.product]
			overage, cents := CalculateOverage(&product, tt.usage)

			if cents != tt.wantCents {
				t.Errorf("Total cents = %d, want %d", cents, tt.wantCents)
			}

			if overage.SmallHours != tt.wantSmall {
				t.Errorf("Overage small hours = %f, want %f", overage.SmallHours, tt.wantSmall)
			}

			if tt.wantMedium > 0 && overage.MediumHours != tt.wantMedium {
				t.Errorf("Overage medium hours = %f, want %f", overage.MediumHours, tt.wantMedium)
			}
		})
	}
}

func TestTierPricing(t *testing.T) {
	tests := []struct {
		tier      string
		wantCents float64
	}{
		{"small", 8.3},
		{"medium", 16.7},
		{"large", 33.3},
		{"gpu", 208.0},
	}

	for _, tt := range tests {
		t.Run(tt.tier, func(t *testing.T) {
			price, ok := TierPricing[tt.tier]
			if !ok {
				t.Fatalf("Tier %s not found in pricing", tt.tier)
			}
			if price != tt.wantCents {
				t.Errorf("Price for %s = %f, want %f", tt.tier, price, tt.wantCents)
			}
		})
	}
}

func TestUsageToHours(t *testing.T) {
	tests := []struct {
		minutes int
		want    float64
	}{
		{60, 1.0},
		{90, 1.5},
		{120, 2.0},
		{30, 0.5},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := UsageToHours(tt.minutes, "small")
			if got != tt.want {
				t.Errorf("UsageToHours(%d) = %f, want %f", tt.minutes, got, tt.want)
			}
		})
	}
}
