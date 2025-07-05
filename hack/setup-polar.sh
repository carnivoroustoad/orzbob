#!/bin/bash
# Setup script for Polar.sh billing integration
# This script helps configure Polar.sh for Orzbob Cloud

set -e

echo "🧊 Polar.sh Setup for Orzbob Cloud"
echo "=================================="
echo ""
echo "This script will guide you through setting up Polar.sh billing."
echo ""

# Color codes
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}Prerequisites:${NC}"
echo "1. Create a Polar.sh account at https://polar.sh"
echo "2. Create a new project called 'Orzbob Cloud'"
echo "3. Have your API credentials ready"
echo ""
read -p "Press Enter when ready to continue..."

echo ""
echo -e "${BLUE}Step 1: Configure Products${NC}"
echo "------------------------"
echo "In your Polar.sh dashboard, create these products:"
echo ""
echo "1. Free Tier (free-tier)"
echo "   - Price: $0/month"
echo "   - Description: 'Perfect for trying out Orzbob Cloud'"
echo "   - Features: '10 hours/month included'"
echo ""
echo "2. Base + Usage (base-plus-usage)"
echo "   - Price: $20/month"
echo "   - Description: '$20/month includes 200 small-tier hours'"
echo "   - Features: '200 small-tier hours/month included'"
echo ""
echo "3. Usage Only (usage-only) - HIDDEN"
echo "   - Price: $0/month"
echo "   - Description: 'Pay only for what you use'"
echo "   - Visibility: Private/Hidden"
echo ""
read -p "Press Enter when products are created..."

echo ""
echo -e "${BLUE}Step 2: Configure Metering${NC}"
echo "-------------------------"
echo "Create a usage meter with:"
echo "- Meter ID: orzbob_compute_hours"
echo "- Name: Compute Hours"
echo "- Aggregation: Sum"
echo "- Unit: hours"
echo ""
read -p "Press Enter when meter is created..."

echo ""
echo -e "${BLUE}Step 3: Configure Webhooks${NC}"
echo "-------------------------"
echo "Add these webhook endpoints:"
echo ""
echo "1. https://api.orzbob.cloud/webhooks/polar/subscription-created"
echo "   - Events: subscription.created"
echo ""
echo "2. https://api.orzbob.cloud/webhooks/polar/subscription-updated"
echo "   - Events: subscription.updated"
echo ""
echo "3. https://api.orzbob.cloud/webhooks/polar/subscription-canceled"
echo "   - Events: subscription.canceled"
echo ""
echo "4. https://api.orzbob.cloud/webhooks/polar/invoice-created"
echo "   - Events: invoice.created"
echo ""
echo "5. https://api.orzbob.cloud/webhooks/polar/invoice-paid"
echo "   - Events: invoice.paid"
echo ""
read -p "Press Enter when webhooks are configured..."

echo ""
echo -e "${BLUE}Step 4: Save Credentials${NC}"
echo "-----------------------"
echo "You'll need these values from Polar.sh:"
echo ""
read -p "Enter your Polar API Key (polar_sk_...): " POLAR_API_KEY
read -p "Enter your Polar Webhook Secret (whsec_...): " POLAR_WEBHOOK_SECRET
read -p "Enter your Polar Project ID (proj_...): " POLAR_PROJECT_ID

# Create .env file
echo ""
echo -e "${YELLOW}Creating .env file...${NC}"
cat > .env.polar << EOF
# Polar.sh Configuration
POLAR_API_KEY=${POLAR_API_KEY}
POLAR_WEBHOOK_SECRET=${POLAR_WEBHOOK_SECRET}
POLAR_PROJECT_ID=${POLAR_PROJECT_ID}
EOF

echo -e "${GREEN}✓ Created .env.polar${NC}"

# Create Kubernetes secret
echo ""
echo -e "${YELLOW}Creating Kubernetes secret...${NC}"
echo "Run this command to create the secret:"
echo ""
echo "kubectl create secret generic polar-credentials \\"
echo "  --namespace=orzbob-system \\"
echo "  --from-literal=api-key='${POLAR_API_KEY}' \\"
echo "  --from-literal=webhook-secret='${POLAR_WEBHOOK_SECRET}' \\"
echo "  --from-literal=project-id='${POLAR_PROJECT_ID}'"
echo ""

echo -e "${BLUE}Step 5: Test Connection${NC}"
echo "----------------------"
echo "Run these commands to test:"
echo ""
echo "# Test API connection"
echo "curl -H 'Authorization: Bearer ${POLAR_API_KEY}' https://api.polar.sh/v1/products"
echo ""
echo "# Run billing tests"
echo "go test ./internal/billing -run TestPolarClientAuth"
echo ""

echo -e "${GREEN}✓ Setup complete!${NC}"
echo ""
echo "Next steps:"
echo "1. Review internal/billing/polar_config.yaml"
echo "2. Run 'make test-billing' to verify setup"
echo "3. Deploy the updated control plane with billing support"
echo ""
echo -e "${YELLOW}⚠️  Remember to:${NC}"
echo "- Keep .env.polar secure and add to .gitignore"
echo "- Test in Polar sandbox mode first"
echo "- Set up monitoring for billing events"
echo ""