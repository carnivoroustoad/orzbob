# Orzbob Cloud Beta Launch Checklist

## Overview

This checklist ensures Orzbob Cloud is ready for beta launch with proper security, monitoring, and operational readiness.

## Security Checklist

### Authentication & Authorization
- [x] OAuth2 integration with GitHub
- [x] JWT token validation for API access
- [x] Short-lived tokens (2 minute expiry) for attach URLs
- [x] Organization-based quota enforcement
- [ ] Rate limiting per IP/user
- [ ] API key management for CI/CD usage

### Network Security
- [x] TLS/HTTPS only for control plane
- [x] WebSocket connections use WSS
- [x] No external exposure of sidecar services
- [x] Network policies restrict pod-to-pod communication
- [ ] WAF rules configured
- [ ] DDoS protection enabled

### Secrets Management
- [x] Kubernetes secrets for sensitive data
- [x] Secrets mounted as environment variables
- [x] No secrets in container images
- [x] Audit logging for secret access
- [ ] Secret rotation policy documented
- [ ] HashiCorp Vault integration (future)

### Container Security
- [x] Non-root containers
- [x] Read-only root filesystem where possible
- [x] Security contexts enforced
- [x] Resource limits prevent DoS
- [ ] Container image scanning in CI
- [ ] Runtime security monitoring

### Compliance Pre-checks
- [ ] GDPR compliance review
  - [ ] Privacy policy updated
  - [ ] Data retention policies defined
  - [ ] Right to deletion implemented
- [ ] SOC 2 pre-assessment
  - [ ] Access controls documented
  - [ ] Change management process
  - [ ] Incident response plan
  - [ ] Business continuity plan
- [ ] Security questionnaire prepared

## Monitoring & SLO Dashboard

### Key Metrics
```yaml
# SLO Targets
availability: 99.5%      # Beta target
response_time_p99: 500ms # API responses
attach_success_rate: 99% # WebSocket connections
```

### Prometheus Metrics Implemented
- [x] `orzbob_instances_created_total` - Instance creation counter
- [x] `orzbob_instances_deleted_total` - Instance deletion counter
- [x] `orzbob_active_sessions` - Active WebSocket sessions gauge
- [x] `orzbob_quota_exceeded_total` - Quota limit hits
- [x] `orzbob_heartbeats_received_total` - Instance heartbeat counter
- [x] `orzbob_idle_instances_reaped_total` - Idle cleanup counter
- [x] `orzbob_http_request_duration_seconds` - HTTP latency histogram

### Grafana Dashboards Needed
- [ ] Service Health Dashboard
  - [ ] Request rate by endpoint
  - [ ] Error rate by endpoint
  - [ ] Response time percentiles
  - [ ] Active instances by tier
- [ ] Resource Usage Dashboard
  - [ ] CPU/Memory per instance tier
  - [ ] Storage usage trends
  - [ ] Network throughput
- [ ] Business Metrics Dashboard
  - [ ] Active users
  - [ ] Instance creation rate
  - [ ] Tier distribution
  - [ ] Feature adoption

### Alerting Rules
- [ ] Control plane down > 1 minute
- [ ] Error rate > 1% for 5 minutes
- [ ] Response time p99 > 1s for 5 minutes
- [ ] Disk usage > 80%
- [ ] Certificate expiry < 7 days

## Operational Readiness

### Runbooks
- [ ] Instance stuck in pending state
- [ ] WebSocket connection failures
- [ ] Database connection pool exhaustion
- [ ] Kubernetes node failure
- [ ] Sidecar service health check failures

### Backup & Recovery
- [ ] Database backup schedule (daily)
- [ ] Backup restoration tested
- [ ] Disaster recovery plan documented
- [ ] RTO/RPO targets defined (4hr/1hr for beta)

### Capacity Planning
- [ ] Load testing completed
  - [ ] 100 concurrent instances
  - [ ] 1000 API requests/second
  - [ ] 500 concurrent WebSocket connections
- [ ] Auto-scaling policies configured
- [ ] Resource quotas per namespace

## Free Tier Configuration

### Implemented Features
- [x] 2 concurrent instances per organization
- [x] Small tier only (2 CPU, 4GB RAM)
- [x] 30-minute idle timeout
- [x] Automatic cleanup of idle instances

### Billing Integration (Post-Beta)
- [ ] Stripe integration
- [ ] Usage tracking
- [ ] Invoice generation
- [ ] Payment failure handling

## Beta Launch Tasks

### Pre-Launch (Week -1)
- [ ] Security scan with OWASP ZAP
- [ ] Penetration testing (basic)
- [ ] Load testing scenarios
- [ ] Backup restoration drill
- [ ] Update status page

### Launch Day
- [ ] Enable feature flags progressively
- [ ] Monitor error rates closely
- [ ] Announce in Discord/Slack
- [ ] Update documentation site
- [ ] Enable support channels

### Post-Launch (Week 1)
- [ ] Daily metrics review
- [ ] User feedback collection
- [ ] Performance optimization
- [ ] Security log review
- [ ] Capacity adjustment

## Success Criteria

### Week 1 Targets
- Availability: > 99.5%
- Error rate: < 1%
- P99 latency: < 500ms
- User satisfaction: > 4/5
- Critical bugs: 0

### Beta Exit Criteria
- 500+ active users
- 10,000+ instances created
- < 0.5% error rate sustained
- All P0/P1 bugs resolved
- SOC 2 Type 1 ready

## Risk Register

| Risk | Impact | Likelihood | Mitigation |
|------|--------|------------|------------|
| DDoS attack | High | Medium | CloudFlare, rate limiting |
| Data breach | High | Low | Encryption, access controls |
| Service outage | Medium | Medium | Multi-AZ, auto-recovery |
| Quota abuse | Low | High | Strict limits, monitoring |
| Cost overrun | Medium | Medium | Resource limits, alerts |

## Communication Plan

### Internal
- Daily standup during week 1
- Slack channel: #orzbob-cloud-beta
- On-call rotation established

### External
- Status page: status.orzbob.cloud
- Support email: support@orzbob.cloud
- Discord community channel
- Weekly beta update emails

## Sign-offs

- [ ] Engineering Lead
- [ ] Security Lead
- [ ] Product Manager
- [ ] Customer Success
- [ ] Legal/Compliance

---

## Quick Reference

### Emergency Contacts
- On-call: [PagerDuty]
- Security: security@orzbob.cloud
- Executive: [CEO/CTO emails]

### Key Commands
```bash
# Check system health
kubectl get pods -n orzbob-system
kubectl top nodes
kubectl logs -n orzbob-system deployment/orzbob-cp

# Emergency shutdown
kubectl scale deployment/orzbob-cp -n orzbob-system --replicas=0

# View metrics
kubectl port-forward -n monitoring svc/prometheus 9090:9090
kubectl port-forward -n monitoring svc/grafana 3000:3000
```

### Rollback Procedure
1. Identify problematic version
2. Scale down current deployment
3. Update image tag to last known good
4. Scale up deployment
5. Verify health checks
6. Monitor for 30 minutes

---

Last Updated: [Current Date]
Next Review: [Beta + 1 week]