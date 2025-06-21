#!/bin/bash
set -e

echo "=== E2E Kind Test ==="

# Check if kind is installed
if ! command -v kind &> /dev/null; then
    echo "Error: kind is not installed"
    echo "Please install kind from https://kind.sigs.k8s.io/"
    exit 1
fi

# Check if kubectl is installed
if ! command -v kubectl &> /dev/null; then
    echo "Error: kubectl is not installed"
    exit 1
fi

# Create cluster
echo "Creating kind cluster..."
make kind-up

# Run the e2e test
echo "Running e2e tests..."
go test -v ./tests/e2e/... -tags=e2e

# Clean up
echo "Cleaning up..."
make kind-down

echo "=== E2E test completed successfully ==="