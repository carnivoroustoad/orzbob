#!/bin/bash

echo "=== Verifying Kind Setup ==="

# Check for kind
KIND=$(command -v kind || echo ~/go/bin/kind)
if test -x "$KIND"; then
    echo "✓ kind found at: $KIND"
    echo "  version: $($KIND version)"
else
    echo "✗ kind not found"
    exit 1
fi

# Check for kubectl
if command -v kubectl &> /dev/null; then
    echo "✓ kubectl found"
    echo "  version: $(kubectl version --client --short 2>/dev/null || kubectl version --client)"
else
    echo "✗ kubectl not found"
    echo "  Please install kubectl"
    exit 1
fi

# Check for Docker
if docker info &> /dev/null; then
    echo "✓ Docker is running"
    echo "  version: $(docker version --format '{{.Server.Version}}' 2>/dev/null)"
else
    echo "✗ Docker is not running"
    echo "  Please start Docker Desktop or Docker daemon"
    exit 1
fi

echo ""
echo "All prerequisites are met! You can run: make e2e-kind"