#!/bin/bash
set -euo pipefail

echo "🚀 Starting Orzbob Cloud E2E Tests"

# Configuration
CLUSTER_NAME="${CLUSTER_NAME:-orzbob-e2e}"
NAMESPACE="${NAMESPACE:-default}"
TIMEOUT="${TIMEOUT:-300s}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Helper functions
info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

error() {
    echo -e "${RED}[ERROR]${NC} $1"
    exit 1
}

cleanup() {
    info "Cleaning up..."
    if [ "${KEEP_CLUSTER:-false}" != "true" ]; then
        kind delete cluster --name "$CLUSTER_NAME" 2>/dev/null || true
    else
        info "Keeping cluster $CLUSTER_NAME (KEEP_CLUSTER=true)"
    fi
}

trap cleanup EXIT

# Check dependencies
info "Checking dependencies..."
command -v kind >/dev/null 2>&1 || error "kind is not installed"
command -v kubectl >/dev/null 2>&1 || error "kubectl is not installed"
command -v helm >/dev/null 2>&1 || error "helm is not installed"
command -v docker >/dev/null 2>&1 || error "docker is not installed"

# Create kind cluster
info "Creating kind cluster: $CLUSTER_NAME"
if kind get clusters | grep -q "^$CLUSTER_NAME$"; then
    warn "Cluster $CLUSTER_NAME already exists, deleting..."
    kind delete cluster --name "$CLUSTER_NAME"
fi

# Create cluster with custom config
cat <<EOF | kind create cluster --name "$CLUSTER_NAME" --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  image: kindest/node:v1.28.0
  extraPortMappings:
  - containerPort: 30080
    hostPort: 8080
    protocol: TCP
EOF

# Build Docker images
info "Building Docker images..."
docker build -f docker/control-plane.Dockerfile -t orzbob/cloud-cp:e2e .
docker build -f docker/runner.Dockerfile -t orzbob/cloud-agent:e2e .

# Load images into kind
info "Loading images into kind cluster..."
kind load docker-image orzbob/cloud-cp:e2e --name "$CLUSTER_NAME"
kind load docker-image orzbob/cloud-agent:e2e --name "$CLUSTER_NAME"

# Install control plane
info "Installing control plane..."
helm upgrade --install orzbob-cp charts/cp \
    --set image.repository=orzbob/cloud-cp \
    --set image.tag=e2e \
    --set image.pullPolicy=Never \
    --set service.type=NodePort \
    --set service.nodePort=30080 \
    --wait --timeout "$TIMEOUT"

# Wait for control plane to be ready
info "Waiting for control plane to be ready..."
kubectl wait --for=condition=ready pod -l app=orzbob-cp --timeout="$TIMEOUT"

# Show pod status
kubectl get pods -l app=orzbob-cp

# Run smoke test
info "Running smoke test..."
MAX_RETRIES=30
RETRY_COUNT=0
while [ $RETRY_COUNT -lt $MAX_RETRIES ]; do
    if curl -s http://localhost:8080/health | grep -q "healthy"; then
        info "Control plane is healthy!"
        break
    fi
    RETRY_COUNT=$((RETRY_COUNT + 1))
    warn "Waiting for control plane to be accessible... ($RETRY_COUNT/$MAX_RETRIES)"
    sleep 2
done

if [ $RETRY_COUNT -eq $MAX_RETRIES ]; then
    error "Control plane failed to become accessible"
fi

# Run Go e2e tests
info "Running Go E2E tests..."
export RUN_E2E=true
go test -v -tags=e2e ./test/e2e/...

# Run additional smoke tests
info "Running additional smoke tests..."
go run hack/smoke.go

# Show metrics
info "Checking metrics endpoint..."
curl -s http://localhost:8080/metrics | grep "orzbob_" | head -10

# Collect logs for debugging
if [ "${COLLECT_LOGS:-false}" = "true" ]; then
    info "Collecting logs..."
    kubectl logs -l app=orzbob-cp --tail=100 > cp-logs.txt
    info "Logs saved to cp-logs.txt"
fi

info "✅ E2E tests completed successfully!"