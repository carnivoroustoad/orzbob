# Orzbob Cloud Billing Setup

This document describes the billing integration with Polar.sh for Orzbob Cloud.

## Overview

Orzbob Cloud uses [Polar.sh](https://polar.sh) for billing and subscription management. We offer three pricing tiers:

1. **Free Tier** ($0/month)
   - 10 hours/month of small instances included
   - Perfect for trying out Orzbob Cloud
   - Community support
   - 30-minute idle timeout

2. **Base + Usage** ($20/month)
   - 200 small-tier hours/month included
   - All instance tiers available
   - Priority support
   - 60-minute idle timeout
   - Pay-as-you-go for additional usage

3. **Usage Only** (Hidden, $0/month)
   - No monthly fee
   - Pay-as-you-go pricing
   - All instance tiers
   - For special arrangements

## Pricing by Instance Tier

| Tier   | Hourly Rate | Resources        |
|--------|-------------|------------------|
| Small  | $0.083/hr   | 2 CPU, 4GB RAM   |
| Medium | $0.167/hr   | 4 CPU, 8GB RAM   |
| Large  | $0.333/hr   | 8 CPU, 16GB RAM  |
| GPU    | $2.08/hr    | 8 CPU, 32GB, GPU |

## Setup Instructions

### 1. Polar.sh Configuration

1. Create a Polar.sh account at https://polar.sh
2. Create a new project called "Orzbob Cloud"
3. Run the setup script:
   ```bash
   ./hack/setup-polar.sh
   ```

### 2. Create Products

In your Polar.sh dashboard, create these products:

1. **Free Tier** (free-tier)
   - Price: $0/month
   - Description: "Perfect for trying out Orzbob Cloud"
   - Features: "10 hours/month included"

2. **Base + Usage** (base-plus-usage)
   - Price: $20/month
   - Description: "$20/month includes 200 small-tier hours"
   - Features: "200 small-tier hours/month included"

3. **Usage Only** (usage-only) - HIDDEN
   - Price: $0/month
   - Description: "Pay only for what you use"
   - Visibility: Private/Hidden

### 3. Configure Metering

Create a usage meter with:
- Meter ID: `orzbob_compute_hours`
- Name: Compute Hours
- Aggregation: Sum
- Unit: hours

### 4. Configure Webhooks

Add these webhook endpoints:

1. `https://api.orzbob.cloud/webhooks/polar/subscription-created`
   - Events: subscription.created

2. `https://api.orzbob.cloud/webhooks/polar/subscription-updated`
   - Events: subscription.updated

3. `https://api.orzbob.cloud/webhooks/polar/subscription-canceled`
   - Events: subscription.canceled

4. `https://api.orzbob.cloud/webhooks/polar/invoice-created`
   - Events: invoice.created

5. `https://api.orzbob.cloud/webhooks/polar/invoice-paid`
   - Events: invoice.paid

### 5. Environment Variables

Required environment variables:
- `POLAR_API_KEY`: Your Polar API key
- `POLAR_WEBHOOK_SECRET`: Webhook signature verification secret
- `POLAR_PROJECT_ID`: Your Polar project ID

## Testing

Run billing tests:
```bash
make test-billing
```

Test API connection:
```bash
curl -H "Authorization: Bearer $POLAR_API_KEY" https://api.polar.sh/v1/products
```

## Implementation Details

### Product Configuration

Products are defined in `internal/billing/products.go`:

```go
var Products = map[string]PolarProduct{
    "free-tier": {
        ID:          "prod_free_tier",
        Name:        "Free Tier",
        PriceMonth:  0,
        Included:    &Usage{SmallHours: 10},
    },
    "base-plus-usage": {
        ID:          "prod_base_plus_usage",
        Name:        "Base + Usage",
        PriceMonth:  2000, // $20
        Included:    &Usage{SmallHours: 200},
    },
    "usage-only": {
        ID:          "prod_usage_only",
        Name:        "Usage Only",
        PriceMonth:  0,
        Hidden:      true,
        Included:    nil,
    },
}
```

### Usage Calculation

The `CalculateOverage` function determines billable usage:

```go
func CalculateOverage(product *PolarProduct, usage *Usage) (overageHours *Usage, totalCents int)
```

For plans with included hours, only usage exceeding the included amount is billed. For usage-only plans, all usage is billed.

## Checkpoint Verification

B-1 (Polar project & products created) is complete when:

- [x] Polar dashboard shows the three SKUs with correct display names & pricing
- [x] `usage-only` is flagged as private/hidden
- [x] Test checkout in Polar sandbox succeeds for each SKU
- [x] `make test-billing` passes all tests