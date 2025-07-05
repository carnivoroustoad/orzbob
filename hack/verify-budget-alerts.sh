#!/bin/bash

set -e

echo "=== Verifying Budget Alerts Implementation ==="
echo

# Check if email configuration is present
echo "1. Checking email configuration in .env.example..."
if grep -q "SMTP_HOST" .env.example && grep -q "EMAIL_FROM_ADDRESS" .env.example; then
    echo "✓ Email configuration found in .env.example"
else
    echo "✗ Email configuration missing in .env.example"
    exit 1
fi

# Check if notifications package exists
echo
echo "2. Checking notifications package..."
if [ -d "internal/notifications" ] && [ -f "internal/notifications/email.go" ]; then
    echo "✓ Notifications package exists"
else
    echo "✗ Notifications package not found"
    exit 1
fi

# Check if budget alerts implementation exists
echo
echo "3. Checking budget alerts implementation..."
if [ -f "internal/billing/alerts.go" ]; then
    echo "✓ Budget alerts implementation found"
else
    echo "✗ Budget alerts implementation not found"
    exit 1
fi

# Check if manager integrates alerts
echo
echo "4. Checking billing manager integration..."
if grep -q "alertService" internal/billing/manager.go; then
    echo "✓ Budget alerts integrated in billing manager"
else
    echo "✗ Budget alerts not integrated in billing manager"
    exit 1
fi

# Run tests
echo
echo "5. Running budget alert tests..."
cd internal/billing
if go test -v -run "TestBudgetAlertService|TestManager_BudgetAlerts" ./...; then
    echo "✓ Budget alert tests passed"
else
    echo "✗ Budget alert tests failed"
    exit 1
fi

echo
echo "=== Budget Alerts Verification Complete ==="
echo
echo "Checkpoint 6 implementation includes:"
echo "- Email notification service (internal/notifications/)"
echo "- Budget alert service with 50% and 90% thresholds"
echo "- Integration with billing manager"
echo "- Prevention of duplicate alerts in same billing period"
echo "- Comprehensive test coverage"
echo
echo "To test with a real SMTP server:"
echo "1. Run a local SMTP server (e.g., MailHog: docker run -p 1025:1025 -p 8025:8025 mailhog/mailhog)"
echo "2. Configure .env with SMTP settings"
echo "3. Run the billing manager and trigger usage thresholds"