#!/bin/bash

set -e

echo "=== Verifying Throttle & Daily Cap Implementation ==="
echo

# Check if throttle service exists
echo "1. Checking throttle service implementation..."
if [ -f "internal/billing/throttle.go" ]; then
    echo "✓ Throttle service implementation found"
else
    echo "✗ Throttle service implementation not found"
    exit 1
fi

# Check if throttle integration exists
echo
echo "2. Checking throttle integration..."
if [ -f "internal/billing/throttle_integration.go" ]; then
    echo "✓ Throttle integration found"
else
    echo "✗ Throttle integration not found"
    exit 1
fi

# Check if manager integrates throttle
echo
echo "3. Checking billing manager integration..."
if grep -q "throttleService" internal/billing/manager.go; then
    echo "✓ Throttle service integrated in billing manager"
else
    echo "✗ Throttle service not integrated in billing manager"
    exit 1
fi

# Check if metrics are defined
echo
echo "4. Checking throttle metrics..."
if grep -q "InstancesPaused" internal/metrics/metrics.go; then
    echo "✓ Throttle metrics defined"
else
    echo "✗ Throttle metrics not defined"
    exit 1
fi

# Run tests
echo
echo "5. Running throttle tests..."
cd internal/billing
if go test -v -run "TestThrottleService|TestControlPlaneIntegration" ./...; then
    echo "✓ Throttle tests passed"
else
    echo "✗ Throttle tests failed"
    exit 1
fi

echo
echo "=== Throttle & Daily Cap Verification Complete ==="
echo
echo "Checkpoint 7 implementation includes:"
echo "- Throttle service with 8h continuous and 24h daily limits"
echo "- Idle timeout tracking (30 minutes configurable)"
echo "- Per-organization daily usage tracking"
echo "- Instance pause/resume functionality"
echo "- Integration hooks for control plane"
echo "- Prometheus metrics for monitoring"
echo "- Comprehensive test coverage"
echo
echo "Integration steps for control plane:"
echo "1. Create billing manager instance"
echo "2. Use ControlPlaneIntegration wrapper"
echo "3. Call OnInstanceCreate when creating instances"
echo "4. Call OnHeartbeat when receiving heartbeats"
echo "5. Call OnInstanceDelete when deleting instances"
echo "6. Implement pause callback to actually pause instances"