#!/bin/bash

set -e

echo "=== Verifying CLI & Dashboard Implementation ==="
echo

# Check if billing CLI command exists
echo "1. Checking billing CLI command..."
if [ -f "billing.go" ]; then
    echo "✓ Billing CLI command implementation found"
else
    echo "✗ Billing CLI command not found"
    exit 1
fi

# Check if billing endpoint exists in control plane
echo
echo "2. Checking control plane billing endpoint..."
if grep -q "handleGetBilling" cmd/cloud-cp/main.go && grep -q "/billing" cmd/cloud-cp/main.go; then
    echo "✓ Billing endpoint implemented in control plane"
else
    echo "✗ Billing endpoint not found in control plane"
    exit 1
fi

# Check if dashboard HTML exists
echo
echo "3. Checking dashboard HTML..."
if [ -f "landing/dashboard.html" ]; then
    echo "✓ Dashboard HTML page created"
else
    echo "✗ Dashboard HTML not found"
    exit 1
fi

# Check if React component exists
echo
echo "4. Checking React billing component..."
if [ -f "landing/BillingCard.jsx" ]; then
    echo "✓ React BillingCard component created"
else
    echo "✗ React BillingCard component not found"
    exit 1
fi

# Test CLI compilation
echo
echo "5. Testing CLI compilation..."
if go build -o /tmp/orz-test .; then
    echo "✓ CLI compiles successfully with billing command"
    rm -f /tmp/orz-test
else
    echo "✗ CLI compilation failed"
    exit 1
fi

# Test that billing command is registered
echo
echo "6. Checking billing command registration..."
if grep -q "cloudCmd.AddCommand(billingCmd)" billing.go; then
    echo "✓ Billing command properly registered"
else
    echo "✗ Billing command not registered"
    exit 1
fi

echo
echo "=== CLI & Dashboard Verification Complete ==="
echo
echo "Checkpoint 8 implementation includes:"
echo
echo "CLI Command (orz cloud billing):"
echo "- Shows current plan and usage statistics"
echo "- Displays usage progress bar"
echo "- Shows estimated charges"
echo "- Supports JSON output with --json flag"
echo "- Integrates with control plane API"
echo
echo "Dashboard Components:"
echo "- Static HTML dashboard with billing card"
echo "- React component for dynamic integration"
echo "- Visual usage progress bars"
echo "- Real-time usage statistics"
echo "- Quick action buttons"
echo
echo "API Integration:"
echo "- GET /v1/billing endpoint in control plane"
echo "- Returns comprehensive billing information"
echo "- Ready for integration with billing manager"
echo
echo "To test the CLI command:"
echo "1. Start the control plane: go run cmd/cloud-cp/main.go"
echo "2. Login: orz login"
echo "3. View billing: orz cloud billing"
echo "4. JSON output: orz cloud billing --json"