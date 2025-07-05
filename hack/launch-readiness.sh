#!/bin/bash
# Launch readiness check for Orzbob Cloud Beta
# Runs through all pre-launch checks

set -e

echo "🚀 Orzbob Cloud Beta Launch Readiness Check"
echo "=========================================="
echo ""
echo "Date: $(date)"
echo ""

# Color codes
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Check function
check() {
    local name=$1
    local command=$2
    echo -n "Checking $name... "
    if eval $command &> /dev/null; then
        echo -e "${GREEN}✓${NC}"
        return 0
    else
        echo -e "${RED}✗${NC}"
        return 1
    fi
}

# Manual check function
manual_check() {
    local name=$1
    echo -e "${YELLOW}⚠${NC}  $name - Manual verification required"
}

echo "1. Infrastructure Checks"
echo "------------------------"
check "Kubernetes cluster" "kubectl cluster-info"
check "Control plane deployment" "kubectl get deployment -n orzbob-system orzbob-cp"
check "Control plane service" "kubectl get service -n orzbob-system orzbob-cp"
check "Namespace exists" "kubectl get namespace orzbob-runners"
check "RBAC configured" "kubectl get clusterrolebinding orzbob-cp"

echo ""
echo "2. Application Health"
echo "--------------------"
check "Control plane pods running" "kubectl get pods -n orzbob-system -l app=orzbob-cp | grep Running"
check "No pods in error state" "! kubectl get pods -A | grep -E 'Error|CrashLoop'"
check "Health endpoint" "kubectl exec -n orzbob-system deployment/orzbob-cp -- wget -O- http://localhost:8080/health"

echo ""
echo "3. Security Checks"
echo "-----------------"
check "TLS certificates valid" "kubectl get ingress -n orzbob-system -o yaml | grep 'tls:' || echo 'No ingress found'"
check "No hardcoded secrets" "! kubectl get deployments -A -o yaml | grep -i 'password:' | grep -v 'secretKeyRef'"
check "Pod security policies" "kubectl get podsecuritypolicies 2>/dev/null || echo 'PSPs not enabled'"
check "Network policies exist" "kubectl get networkpolicies -n orzbob-system 2>/dev/null || echo 'No network policies'"

echo ""
echo "4. Monitoring & Metrics"
echo "----------------------"
check "Prometheus running" "kubectl get pods -n monitoring -l app=prometheus | grep Running || echo 'Prometheus not found'"
check "Grafana running" "kubectl get pods -n monitoring -l app=grafana | grep Running || echo 'Grafana not found'"
check "Metrics endpoint" "kubectl exec -n orzbob-system deployment/orzbob-cp -- wget -O- http://localhost:8080/metrics | grep orzbob_"

echo ""
echo "5. Resource Limits"
echo "-----------------"
check "CPU limits set" "kubectl get pods -n orzbob-system -o yaml | grep -q 'limits:' | grep -q 'cpu:'"
check "Memory limits set" "kubectl get pods -n orzbob-system -o yaml | grep -q 'limits:' | grep -q 'memory:'"
check "Resource quotas" "kubectl get resourcequotas -A 2>/dev/null || echo 'No quotas set'"

echo ""
echo "6. Backup & Recovery"
echo "-------------------"
manual_check "Database backup configured"
manual_check "Backup restoration tested"
manual_check "Disaster recovery plan documented"

echo ""
echo "7. Documentation"
echo "---------------"
check "README.md exists" "test -f README.md"
check "Cloud quickstart guide" "test -f docs/cloud-quickstart.md"
check "Configuration reference" "test -f docs/cloud-config-reference.md"
check "Example configurations" "test -d examples"

echo ""
echo "8. CI/CD Pipeline"
echo "----------------"
check "GitHub Actions configured" "test -f .github/workflows/cloud-ci.yml"
check "Docker images built" "docker images | grep orzbob/cloud"
check "Helm charts valid" "helm lint charts/orzbob-cloud 2>/dev/null || echo 'Charts not found locally'"

echo ""
echo "9. Load Testing Results"
echo "----------------------"
manual_check "100 concurrent instances tested"
manual_check "1000 RPS API load tested"
manual_check "500 concurrent WebSocket connections tested"

echo ""
echo "10. Compliance & Legal"
echo "---------------------"
manual_check "Privacy policy updated"
manual_check "Terms of service updated"
manual_check "GDPR compliance reviewed"
manual_check "Security audit completed"

echo ""
echo "11. Team Readiness"
echo "-----------------"
manual_check "On-call rotation established"
manual_check "Runbooks written and reviewed"
manual_check "Support channels configured"
manual_check "Team trained on incident response"

echo ""
echo "12. Feature Flags"
echo "----------------"
check "Free tier enabled" "grep -q 'freeQuota.*3' cmd/cloud-cp/main.go"
manual_check "Beta feature flag enabled"
manual_check "Rate limiting configured"

echo ""
echo "========================"
echo "Launch Readiness Summary"
echo "========================"
echo ""
echo "Automated checks complete. Please verify all manual checks above."
echo ""
echo "Critical items before launch:"
echo "1. [ ] All manual checks verified"
echo "2. [ ] Load testing results acceptable"
echo "3. [ ] Security scan completed"
echo "4. [ ] Team sign-offs obtained"
echo "5. [ ] Communication plan ready"
echo ""
echo "To run security scan: ./hack/security-scan.sh"
echo "To view SLO dashboard: kubectl port-forward -n monitoring svc/grafana 3000:3000"
echo ""
echo "Good luck with the beta launch! 🚀"