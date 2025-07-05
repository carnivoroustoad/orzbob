#!/bin/bash
# Security scan script for Orzbob Cloud
# Runs basic security checks before beta launch

set -e

echo "🔒 Orzbob Cloud Security Scan"
echo "============================="
echo ""

# Check if required tools are installed
check_tool() {
    if ! command -v $1 &> /dev/null; then
        echo "❌ $1 is not installed. Please install it first."
        exit 1
    fi
}

# Container Image Scanning
echo "📦 Container Image Security Scan"
echo "--------------------------------"

# Scan cloud-agent image
echo "Scanning cloud-agent image..."
if command -v trivy &> /dev/null; then
    trivy image --severity HIGH,CRITICAL orzbob/cloud-agent:latest || true
else
    echo "⚠️  Trivy not installed. Install with: brew install aquasecurity/trivy/trivy"
fi

echo ""

# Scan control-plane image  
echo "Scanning control-plane image..."
if command -v trivy &> /dev/null; then
    trivy image --severity HIGH,CRITICAL orzbob/cloud-cp:latest || true
fi

echo ""

# Kubernetes Security Scan
echo "☸️  Kubernetes Security Scan"
echo "---------------------------"

# Check for security policies
echo "Checking for NetworkPolicies..."
kubectl get networkpolicies -n orzbob-system 2>/dev/null || echo "⚠️  No NetworkPolicies found"

echo ""
echo "Checking for PodSecurityPolicies..."
kubectl get podsecuritypolicies 2>/dev/null | grep orzbob || echo "⚠️  No PodSecurityPolicies found"

echo ""

# Secret Management Check
echo "🔐 Secret Management Check"
echo "-------------------------"

# Check for secrets in environment variables
echo "Checking for hardcoded secrets in deployments..."
kubectl get deployments -n orzbob-system -o yaml | grep -i "password\|secret\|key" | grep -v "secretKeyRef" | grep -v "name:" || echo "✅ No hardcoded secrets found"

echo ""

# TLS Certificate Check
echo "🔒 TLS Certificate Check"
echo "-----------------------"

# Check ingress TLS configuration
echo "Checking Ingress TLS configuration..."
kubectl get ingress -n orzbob-system -o yaml | grep -A5 "tls:" || echo "⚠️  No TLS configuration found in Ingress"

echo ""

# RBAC Check
echo "👥 RBAC Security Check"
echo "---------------------"

# Check service account permissions
echo "Checking ServiceAccount permissions..."
kubectl get clusterrolebindings -o json | jq '.items[] | select(.subjects[]?.name == "orzbob-cp")' | jq '.roleRef.name' || echo "⚠️  Could not check RBAC"

echo ""

# Resource Limits Check
echo "📊 Resource Limits Check"
echo "-----------------------"

# Check if pods have resource limits
echo "Checking pod resource limits..."
kubectl get pods -n orzbob-system -o json | jq '.items[] | {name: .metadata.name, limits: .spec.containers[].resources.limits}' || echo "⚠️  Could not check resource limits"

echo ""

# Security Headers Check (if service is running)
echo "🌐 HTTP Security Headers Check"
echo "-----------------------------"

if kubectl get svc -n orzbob-system orzbob-cp &> /dev/null; then
    # Port forward to test
    kubectl port-forward -n orzbob-system svc/orzbob-cp 8080:80 &
    PF_PID=$!
    sleep 3
    
    # Check security headers
    echo "Checking security headers..."
    curl -s -I http://localhost:8080/health | grep -i "strict-transport-security\|x-content-type-options\|x-frame-options\|content-security-policy" || echo "⚠️  Missing security headers"
    
    kill $PF_PID 2>/dev/null || true
else
    echo "⚠️  Control plane service not found"
fi

echo ""

# Vulnerability Database Check
echo "🔍 Go Vulnerability Check"
echo "------------------------"

# Check for known vulnerabilities in Go modules
if command -v govulncheck &> /dev/null; then
    echo "Running Go vulnerability check..."
    cd /tmp && govulncheck github.com/carnivoroustoad/orzbob/... || true
else
    echo "⚠️  govulncheck not installed. Install with: go install golang.org/x/vuln/cmd/govulncheck@latest"
fi

echo ""

# Summary
echo "📋 Security Scan Summary"
echo "======================="
echo ""
echo "✅ Completed basic security scan"
echo "⚠️  Review any warnings above before beta launch"
echo ""
echo "Additional recommended scans:"
echo "- [ ] OWASP ZAP for web application scanning"
echo "- [ ] Nessus or OpenVAS for infrastructure scanning"
echo "- [ ] Manual penetration testing"
echo "- [ ] AWS Inspector (if on AWS)"
echo ""
echo "For comprehensive results, consider using:"
echo "- Snyk for dependency scanning"
echo "- SonarQube for code analysis"
echo "- Falco for runtime security"
echo ""

# Exit with success
exit 0