# Support Runbook: Rectifying Incorrect Charges

## Overview
This runbook provides step-by-step procedures for support staff to investigate and rectify incorrect billing charges reported by customers.

## Common Scenarios

### 1. Instance Stuck in Running State
**Symptoms**: Customer reports being charged for an instance they believe was stopped

**Investigation Steps**:
1. Check instance status in control plane:
   ```bash
   kubectl get pods -n orzbob-instances -l instance-id=<INSTANCE_ID>
   ```

2. Review instance lifecycle logs:
   ```bash
   kubectl logs -n orzbob-system deployment/control-plane | grep <INSTANCE_ID>
   ```

3. Check heartbeat timestamps:
   ```sql
   SELECT instance_id, last_heartbeat, status 
   FROM instances 
   WHERE instance_id = '<INSTANCE_ID>';
   ```

**Resolution**:
- If instance is actually stopped but still being charged:
  1. Manually update instance status
  2. Calculate correct usage hours
  3. Issue credit for overcharge

### 2. Incorrect Tier Pricing
**Symptoms**: Customer charged at wrong tier rate (e.g., GPU rates for small instance)

**Investigation Steps**:
1. Verify instance tier from creation request:
   ```bash
   kubectl describe pod -n orzbob-instances <POD_NAME> | grep tier
   ```

2. Check billing records:
   ```sql
   SELECT instance_id, tier, rate_per_hour, hours_used, total_charge
   FROM billing_records
   WHERE customer_id = '<CUSTOMER_ID>'
   AND billing_period = '<PERIOD>';
   ```

**Resolution**:
1. Identify the pricing discrepancy
2. Calculate difference: `(incorrect_rate - correct_rate) * hours_used`
3. Issue credit for the difference

### 3. Throttle Limits Not Applied
**Symptoms**: Customer charged beyond daily 24-hour cap or wasn't paused at 8-hour limit

**Investigation Steps**:
1. Check throttle service logs:
   ```bash
   kubectl logs -n orzbob-system deployment/control-plane | grep -i throttle | grep <ORG_ID>
   ```

2. Verify throttle configuration:
   ```sql
   SELECT org_id, continuous_limit, daily_limit, idle_timeout
   FROM throttle_settings
   WHERE org_id = '<ORG_ID>';
   ```

3. Review daily usage:
   ```sql
   SELECT date, SUM(hours_used) as daily_hours
   FROM instance_usage
   WHERE org_id = '<ORG_ID>'
   AND date BETWEEN '<START>' AND '<END>'
   GROUP BY date;
   ```

**Resolution**:
1. If limits weren't enforced:
   - Credit any usage beyond the cap
   - File bug report for throttle service
   - Add customer to manual monitoring list

### 4. Duplicate Charges
**Symptoms**: Customer charged multiple times for same usage period

**Investigation Steps**:
1. Query for duplicate billing records:
   ```sql
   SELECT instance_id, start_time, end_time, COUNT(*) as count
   FROM billing_records
   WHERE customer_id = '<CUSTOMER_ID>'
   GROUP BY instance_id, start_time, end_time
   HAVING COUNT(*) > 1;
   ```

2. Check for duplicate meter submissions:
   ```bash
   curl -X GET https://api.polar.sh/v1/meters/usage \
     -H "Authorization: Bearer $POLAR_API_KEY" \
     -d "customer_id=<CUSTOMER_ID>&period=<PERIOD>"
   ```

**Resolution**:
1. Identify all duplicate charges
2. Calculate total overcharge
3. Issue full credit for duplicates
4. Investigate root cause in metering service

## Credit Issuance Process

### Step 1: Calculate Credit Amount
```python
# Example calculation
original_charge = hours_used * incorrect_rate
correct_charge = hours_used * correct_rate
credit_amount = original_charge - correct_charge
```

### Step 2: Create Credit in Polar
```bash
curl -X POST https://api.polar.sh/v1/credits \
  -H "Authorization: Bearer $POLAR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "customer_id": "<CUSTOMER_ID>",
    "amount": <CREDIT_AMOUNT>,
    "description": "Credit for incorrect <TIER> instance charges",
    "metadata": {
      "ticket_id": "<SUPPORT_TICKET_ID>",
      "instance_id": "<INSTANCE_ID>",
      "period": "<BILLING_PERIOD>"
    }
  }'
```

### Step 3: Notify Customer
Use email template:
```
Subject: Credit Applied - Billing Issue Resolved

Hello [Customer Name],

We've investigated the billing issue you reported (Ticket #[TICKET_ID]) and have applied a credit of $[AMOUNT] to your account.

Issue: [Brief description of the issue]
Resolution: [What was corrected]
Credit Amount: $[AMOUNT]

This credit will appear on your next invoice. The underlying issue has been addressed to prevent recurrence.

If you have any questions, please don't hesitate to reach out.

Best regards,
Orzbob Cloud Support Team
```

### Step 4: Document Resolution
1. Update support ticket with:
   - Root cause analysis
   - Credit amount and reference
   - Steps taken to prevent recurrence

2. Log in internal tracking:
   ```sql
   INSERT INTO billing_corrections (
     customer_id, ticket_id, issue_type, credit_amount, 
     resolved_by, resolved_at, notes
   ) VALUES (
     '<CUSTOMER_ID>', '<TICKET_ID>', '<ISSUE_TYPE>', <AMOUNT>,
     '<SUPPORT_AGENT>', NOW(), '<RESOLUTION_NOTES>'
   );
   ```

## Prevention Measures

### Automated Checks
1. Daily reconciliation job comparing:
   - Instance runtime vs billed hours
   - Expected charges vs actual charges
   - Throttle limits vs actual usage

2. Alerts for anomalies:
   - Any org exceeding 24 hours in a day
   - Instances running >8 hours continuously
   - Billing rate mismatches

### Manual Reviews
- Weekly review of credits issued
- Monthly audit of high-usage accounts
- Quarterly review of this runbook

## Escalation Path

1. **Level 1**: Support agent can issue credits up to $50
2. **Level 2**: Support lead can issue credits up to $500
3. **Level 3**: Engineering team for systematic issues
4. **Level 4**: VP Engineering for credits over $500

## Related Documentation
- [Billing System Architecture](./billing-architecture.md)
- [Metering Service Guide](./metering-service.md)
- [Customer Communication Templates](./email-templates.md)

---
Last Updated: 2025-06-23
Next Review: 2025-07-23