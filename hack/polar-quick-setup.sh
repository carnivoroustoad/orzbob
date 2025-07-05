#!/bin/bash
# Quick setup to test Polar.sh connection and get project info

set -e

echo "🧊 Polar.sh Quick Setup"
echo "======================"
echo ""

# Check for access token
if [ -z "$POLAR_ACCESS_TOKEN" ]; then
    echo "Error: POLAR_ACCESS_TOKEN not set"
    echo "Export it with: export POLAR_ACCESS_TOKEN=polar_oat_..."
    exit 1
fi

echo "Testing API connection..."
echo ""

# Test API connection and list organizations
echo "Your organizations:"
curl -s -H "Authorization: Bearer $POLAR_ACCESS_TOKEN" \
    https://api.polar.sh/v1/organizations | jq -r '.items[] | "- \(.name) (ID: \(.id))"'

echo ""
echo "To continue with billing setup, we need:"
echo "1. Create or select a project/organization for Orzbob Cloud"
echo "2. Get the webhook secret (create webhook at https://polar.sh/settings/webhooks)"
echo "3. Create the products as described in docs/billing-setup.md"
echo ""
echo "Would you like me to create a test product? (y/n)"
read -p "> " CREATE_TEST

if [ "$CREATE_TEST" = "y" ]; then
    echo ""
    read -p "Enter your organization ID from above: " ORG_ID
    
    echo "Creating test product..."
    curl -X POST https://api.polar.sh/v1/products \
        -H "Authorization: Bearer $POLAR_ACCESS_TOKEN" \
        -H "Content-Type: application/json" \
        -d '{
            "organization_id": "'$ORG_ID'",
            "name": "Orzbob Cloud - Free Tier",
            "description": "Perfect for trying out Orzbob Cloud",
            "is_recurring": true,
            "price": {
                "type": "fixed",
                "amount": 0,
                "currency": "usd",
                "recurring_interval": "month"
            }
        }' | jq '.'
fi