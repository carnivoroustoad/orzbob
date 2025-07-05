# Support Runbook: Billing Troubleshooting Guide

## Quick Reference

| Issue | First Check | Common Fix |
|-------|------------|------------|
| No usage showing | Instance heartbeats | Restart metering service |
| Wrong plan displayed | Polar subscription | Sync subscription data |
| Overage not calculated | Quota engine | Reset quota cache |
| Alerts not sent | Email configuration | Check SMTP settings |
| Dashboard not updating | API endpoint | Clear browser cache |

## Common Issues and Solutions

### 1. Customer Cannot See Billing Information

**Symptoms**: 
- `orz cloud billing` returns empty/wrong data
- Dashboard shows loading indefinitely

**Troubleshooting**:
```bash
# 1. Verify customer exists in billing system
curl -X GET https://api.polar.sh/v1/customers/<CUSTOMER_ID> \
  -H "Authorization: Bearer $POLAR_API_KEY"

# 2. Check if customer has active subscription
curl -X GET https://api.polar.sh/v1/subscriptions?customer_id=<CUSTOMER_ID> \
  -H "Authorization: Bearer $POLAR_API_KEY"

# 3. Verify control plane can reach Polar
kubectl exec -n orzbob-system deployment/control-plane -- \
  curl -I https://api.polar.sh/v1/health
```

**Solutions**:
- If customer missing: Create customer in Polar
- If no subscription: Assign appropriate plan
- If connectivity issue: Check network/firewall rules

### 2. Usage Not Being Recorded

**Symptoms**:
- Customer reports using instances but usage shows 0
- Billing shows much less usage than expected

**Troubleshooting**:
```bash
# 1. Check if metering service is running
kubectl get pods -n orzbob-system -l app=control-plane

# 2. Check metering service logs
kubectl logs -n orzbob-system deployment/control-plane | grep -i meter

# 3. Verify instance is sending heartbeats
kubectl logs -n orzbob-system deployment/control-plane | grep "heartbeat.*<INSTANCE_ID>"

# 4. Check if usage is queued but not flushed
kubectl exec -n orzbob-system deployment/control-plane -- \
  curl localhost:8080/metrics | grep orzbob_usage_meter_queue
```

**Solutions**:
1. Restart metering service if crashed
2. Manually flush queued usage:
   ```go
   // Force flush in metering service
   meteringService.Flush()
   ```
3. Check Polar API rate limits

### 3. Wrong Subscription Plan

**Symptoms**:
- Customer on wrong plan (e.g., shows Free when they paid for Base+Usage)
- Included hours incorrect

**Troubleshooting**:
```bash
# 1. Check Polar subscription
curl -X GET https://api.polar.sh/v1/subscriptions/<SUBSCRIPTION_ID> \
  -H "Authorization: Bearer $POLAR_API_KEY"

# 2. Check local cache
kubectl exec -n orzbob-system deployment/control-plane -- \
  cat /var/lib/orzbob/quota/<ORG_ID>.json

# 3. Verify product mapping
grep -A5 "ProductSKUs" internal/billing/products.go
```

**Solutions**:
1. Update subscription in Polar admin panel
2. Clear local cache:
   ```bash
   kubectl exec -n orzbob-system deployment/control-plane -- \
     rm /var/lib/orzbob/quota/<ORG_ID>.json
   ```
3. Force subscription refresh:
   ```go
   quotaEngine.RefreshSubscription(orgID)
   ```

### 4. Budget Alerts Not Sending

**Symptoms**:
- Customer didn't receive 50% or 90% usage alerts
- Alerts sent multiple times

**Troubleshooting**:
```bash
# 1. Check email service configuration
kubectl get secret -n orzbob-system email-config -o yaml

# 2. Test SMTP connection
kubectl exec -n orzbob-system deployment/control-plane -- \
  nc -zv $SMTP_HOST $SMTP_PORT

# 3. Check alert history
kubectl logs -n orzbob-system deployment/control-plane | \
  grep -i "budget.*alert.*<ORG_ID>"

# 4. Verify alert thresholds
kubectl exec -n orzbob-system deployment/control-plane -- \
  curl localhost:8080/debug/alerts/<ORG_ID>
```

**Solutions**:
1. Fix SMTP configuration
2. Reset alert history for testing:
   ```go
   alertService.ResetAlerts()
   ```
3. Manually trigger alert:
   ```go
   alertService.SendTestAlert(orgID, threshold)
   ```

### 5. Throttling Not Working

**Symptoms**:
- Instances running beyond 8-hour limit
- Daily 24-hour cap not enforced
- Idle instances not paused

**Troubleshooting**:
```bash
# 1. Check throttle service status
kubectl logs -n orzbob-system deployment/control-plane | \
  grep -i throttle

# 2. Verify instance registration
kubectl exec -n orzbob-system deployment/control-plane -- \
  curl localhost:8080/debug/throttle/<INSTANCE_ID>

# 3. Check throttle limits configuration
kubectl get configmap -n orzbob-system orzbob-config -o yaml | \
  grep -A5 throttle
```

**Solutions**:
1. Register instance manually:
   ```go
   throttleService.RegisterInstance(instanceID, orgID)
   ```
2. Reset daily usage counter:
   ```go
   throttleService.ResetDailyUsage()
   ```
3. Force throttle check:
   ```go
   throttleService.CheckInstance(instanceID)
   ```

## Debugging Commands Cheatsheet

```bash
# Get customer billing status
kubectl exec -n orzbob-system deployment/control-plane -- \
  curl localhost:8080/v1/billing?org_id=<ORG_ID>

# Check instance usage for today
kubectl exec -n orzbob-system deployment/control-plane -- \
  curl localhost:8080/debug/usage/<ORG_ID>/daily

# View pending meter events
kubectl exec -n orzbob-system deployment/control-plane -- \
  curl localhost:8080/debug/meters/pending

# Force quota refresh
kubectl exec -n orzbob-system deployment/control-plane -- \
  curl -X POST localhost:8080/admin/quota/<ORG_ID>/refresh

# Test email sending
kubectl exec -n orzbob-system deployment/control-plane -- \
  curl -X POST localhost:8080/admin/test-email \
    -d '{"to": "test@example.com", "type": "budget_alert"}'
```

## Monitoring and Alerts

### Key Metrics to Watch
```prometheus
# Metering queue size (should stay low)
orzbob_usage_meter_queue > 1000

# Failed meter submissions
rate(orzbob_meter_errors_total[5m]) > 0

# Alert sending failures
rate(orzbob_alert_send_errors_total[5m]) > 0

# Throttle enforcement
rate(orzbob_instances_paused_total[1h]) == 0
```

### Log Patterns to Monitor
```bash
# Billing errors
"billing.*error|failed.*meter|quota.*exceeded"

# Alert failures  
"alert.*failed|email.*error|smtp.*timeout"

# Throttle issues
"throttle.*error|failed.*pause|limit.*not.*enforced"
```

## Preventive Maintenance

### Daily Checks
1. Review error logs for billing-related issues
2. Check meter queue size
3. Verify all scheduled jobs are running

### Weekly Tasks
1. Audit high-usage accounts for anomalies
2. Review credits issued
3. Check for stuck instances

### Monthly Tasks
1. Reconcile Polar charges with internal records
2. Review and update throttle limits
3. Test alert system with dummy account

---
Last Updated: 2025-06-23
Next Review: 2025-07-23