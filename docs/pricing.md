# Orzbob Cloud Pricing

## Overview

Orzbob Cloud offers flexible pricing plans designed to scale with your needs. Whether you're a solo developer or a growing team, we have a plan that fits your usage patterns.

## Pricing Plans

### Free Tier
**$0/month**
- 10 hours of compute time included per month
- Access to small instances only
- Community support
- Perfect for trying out Orzbob Cloud

### Base + Usage Plan
**Starts at $20/month**
- 200 hours of small-tier compute time included
- Access to all instance tiers
- Pay-as-you-go for additional usage
- Email support
- Budget alerts at 50% and 90% usage
- Most popular plan for individuals and small teams

### Usage Only Plan
**$0/month base + usage**
- No monthly fee
- Pay only for what you use
- Access to all instance tiers
- Email support
- Best for sporadic or unpredictable usage

## Instance Pricing

Usage beyond your included hours is charged at the following rates:

| Instance Tier | Hourly Rate | Monthly Equivalent* |
|--------------|-------------|-------------------|
| Small | $0.083/hour | ~$60/month |
| Medium | $0.167/hour | ~$120/month |
| Large | $0.333/hour | ~$240/month |
| GPU | $2.08/hour | ~$1,500/month |

*Monthly equivalent based on 720 hours (30 days)

## Usage Limits & Throttling

To ensure fair usage and prevent runaway costs, we implement the following limits:

### Daily Limits
- **24-hour cap**: Maximum 24 hours of usage per organization per day
- Instances are automatically paused when daily limit is reached
- Resets at midnight UTC

### Continuous Run Limits
- **8-hour cap**: Instances automatically pause after 8 hours of continuous running
- Helps prevent forgotten instances from running indefinitely
- Can be resumed after a short break

### Idle Timeout
- Instances are paused after 30 minutes of inactivity
- Prevents waste from idle resources
- Can be resumed anytime

## Billing Details

### Billing Cycle
- Monthly billing cycle
- Usage resets on the same day each month
- Charges are processed at the end of each billing cycle

### What Counts as Usage?
- Only running time counts toward your usage
- Paused or stopped instances do not consume hours
- Usage is tracked per minute and rounded up

### Overage Charges
- When you exceed your included hours, additional usage is charged at the standard rates
- You'll receive email alerts at 50% and 90% of your included hours
- Overage charges appear on your next invoice

## Example Scenarios

### Scenario 1: Solo Developer
- **Plan**: Free Tier ($0/month)
- **Usage**: 8 hours/month of small instances
- **Total Cost**: $0 (within 10-hour limit)

### Scenario 2: Small Team
- **Plan**: Base + Usage ($20/month)
- **Usage**: 250 hours/month (mixed small/medium)
  - 200 hours included (small-tier equivalent)
  - 50 hours overage: 30 small + 20 medium
- **Overage Cost**: (30 × $0.083) + (20 × $0.167) = $5.83
- **Total Cost**: $25.83

### Scenario 3: ML Researcher
- **Plan**: Usage Only ($0/month base)
- **Usage**: 40 hours/month of GPU instances
- **Total Cost**: 40 × $2.08 = $83.20

## FAQ

### Can I change plans anytime?
Yes, you can upgrade or downgrade your plan at any time. Changes take effect immediately, and usage is prorated.

### What happens if I exceed my limits?
- **Included hours**: You'll be charged for overage at standard rates
- **Daily cap**: Instances pause until the next day
- **Continuous run**: Instance pauses but can be resumed immediately

### Do paused instances count toward usage?
No, only running instances count toward your usage hours.

### How do I monitor my usage?
- Use `orz cloud billing` command to check current usage
- View detailed usage in the web dashboard
- Receive email alerts at 50% and 90% thresholds

### Can I set a spending limit?
Contact support to set a hard spending limit on your account to prevent unexpected charges.

## Getting Started

1. Sign up for an account at [orzbob.cloud](https://orzbob.cloud)
2. Choose your pricing plan
3. Start creating instances with `orz cloud new`
4. Monitor usage with `orz cloud billing`

For questions about pricing or to discuss enterprise plans, contact sales@orzbob.cloud.