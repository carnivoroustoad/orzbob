# Production Readiness Checklist

## ✅ Infrastructure
- [x] EKS Cluster created (orzbob-cluster)
- [x] 2 t3.medium nodes deployed
- [x] Essential addons installed (VPC-CNI, CoreDNS, kube-proxy)
- [x] AWS Load Balancer Controller installed
- [x] Namespace created (orzbob-system)

## ✅ CI/CD Pipeline
- [x] GitHub Actions workflow configured (.github/workflows/deploy-production.yml)
- [x] Dockerfile.cloud-cp created
- [x] Helm charts ready (charts/cp)

## ✅ GitHub Secrets Configured
- [x] KUBE_CONFIG - EKS cluster access
- [x] DEPLOY_NAMESPACE - orzbob-system
- [x] POLAR_API_KEY - Billing API access
- [x] POLAR_PROJECT_ID - Your organization ID
- [x] POLAR_WEBHOOK_SECRET - Webhook validation

## ⚠️ Optional (Not configured)
- [ ] SMTP settings for email alerts
- [ ] Custom domain setup
- [ ] SSL certificate configuration

## 🚀 Deploy to Production

### Option 1: Push to main branch
```bash
git add .
git commit -m "feat: Production deployment ready"
git push origin main
```

### Option 2: Manual trigger
```bash
gh workflow run deploy-production.yml
```

## 📊 Post-Deployment Verification

### 1. Check deployment status
```bash
kubectl get pods -n orzbob-system
kubectl get svc -n orzbob-system
```

### 2. Port-forward to test locally
```bash
kubectl port-forward -n orzbob-system svc/orzbob-cloud-cp 8080:80
curl http://localhost:8080/health
```

### 3. Check logs
```bash
kubectl logs -n orzbob-system deployment/orzbob-cloud-cp
```

## 🔧 Troubleshooting

### If deployment fails:
1. Check GitHub Actions logs
2. Verify secrets: `gh secret list`
3. Check Kubernetes events: `kubectl get events -n orzbob-system`
4. Review pod logs: `kubectl logs -n orzbob-system -l app.kubernetes.io/instance=orzbob-cloud`

### Common issues:
- **ImagePullBackOff**: Check if image was built and pushed correctly
- **CrashLoopBackOff**: Check logs for startup errors
- **Pending pods**: Check node resources with `kubectl describe nodes`

## 📝 Next Steps After Deployment

1. **Configure Domain** (if you have one):
   - Update ingress.hosts in Helm values
   - Create Route53 records pointing to ALB

2. **Set up Monitoring**:
   - Metrics available at :9090/metrics
   - Consider adding Prometheus/Grafana

3. **Configure Polar Webhooks**:
   - Set webhook URL to: https://YOUR_DOMAIN/webhooks/polar
   - Use the webhook secret you configured

4. **Test Billing**:
   - Create test subscriptions
   - Verify quota enforcement
   - Test budget alerts (if email configured)

## 🎯 You're Ready!

Everything is configured for production deployment. Just push to main or trigger the workflow manually!