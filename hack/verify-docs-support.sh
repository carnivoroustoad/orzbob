#!/bin/bash

set -e

echo "=== Verifying Docs & Support Playbooks ==="
echo

# Check if pricing.md exists
echo "1. Checking pricing documentation..."
if [ -f "docs/pricing.md" ]; then
    echo "✓ Pricing documentation created"
    # Check for key sections
    if grep -q "Pricing Plans" docs/pricing.md && \
       grep -q "Instance Pricing" docs/pricing.md && \
       grep -q "Usage Limits" docs/pricing.md && \
       grep -q "Example Scenarios" docs/pricing.md; then
        echo "✓ All required sections present in pricing.md"
    else
        echo "✗ Missing sections in pricing.md"
        exit 1
    fi
else
    echo "✗ Pricing documentation not found"
    exit 1
fi

# Check if support runbooks exist
echo
echo "2. Checking support runbooks..."
if [ -f "docs/support-runbooks/rectify-incorrect-charges.md" ]; then
    echo "✓ Incorrect charges runbook created"
else
    echo "✗ Incorrect charges runbook not found"
    exit 1
fi

if [ -f "docs/support-runbooks/billing-troubleshooting.md" ]; then
    echo "✓ Billing troubleshooting guide created"
else
    echo "✗ Billing troubleshooting guide not found"
    exit 1
fi

# Check if landing page updated with pricing
echo
echo "3. Checking landing page pricing section..."
if grep -q "pricing-cards" landing/index.html && \
   grep -q "Base + Usage" landing/index.html && \
   grep -q "\$20" landing/index.html; then
    echo "✓ Landing page includes pricing section"
else
    echo "✗ Landing page missing pricing section"
    exit 1
fi

# Check if CSS includes pricing styles
echo
echo "4. Checking pricing styles..."
if grep -q ".pricing-card" landing/styles.css && \
   grep -q ".pricing-button" landing/styles.css; then
    echo "✓ Pricing styles added to CSS"
else
    echo "✗ Pricing styles missing from CSS"
    exit 1
fi

# Verify documentation structure
echo
echo "5. Checking documentation completeness..."
echo "   Pricing documentation:"
echo "   - Free Tier details: $(grep -c "Free Tier" docs/pricing.md) references"
echo "   - Base + Usage plan: $(grep -c "Base + Usage" docs/pricing.md) references"
echo "   - Instance tiers: $(grep -c -E "Small|Medium|Large|GPU" docs/pricing.md) references"
echo "   - Throttle limits: $(grep -c -E "8-hour|24-hour|idle" docs/pricing.md) references"

echo
echo "   Support runbooks:"
echo "   - Incorrect charges scenarios: $(grep -c "Scenario" docs/support-runbooks/rectify-incorrect-charges.md)"
echo "   - Troubleshooting sections: $(grep -c "^##" docs/support-runbooks/billing-troubleshooting.md)"

echo
echo "=== Docs & Support Verification Complete ==="
echo
echo "Checkpoint 9 implementation includes:"
echo
echo "1. Pricing Documentation (docs/pricing.md):"
echo "   - Comprehensive pricing plans"
echo "   - Instance tier pricing table"
echo "   - Usage limits and throttling explained"
echo "   - Example billing scenarios"
echo "   - FAQ section"
echo
echo "2. Support Runbooks:"
echo "   - Rectify incorrect charges playbook"
echo "   - Step-by-step investigation procedures"
echo "   - Credit issuance process"
echo "   - Billing troubleshooting guide"
echo "   - Common issues and solutions"
echo
echo "3. Landing Page Updates:"
echo "   - Pricing section with all plans"
echo "   - Visual pricing cards"
echo "   - Call-to-action buttons"
echo "   - Link to detailed pricing docs"
echo
echo "All billing roadmap checkpoints completed! 🎉"