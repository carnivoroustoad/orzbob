#!/bin/bash
set -e

echo "=== GitHub Secrets Setup for Orzbob Cloud ==="
echo
echo "This script will help you configure GitHub secrets for CI/CD deployment."
echo "You'll need to manually add these secrets to your GitHub repository."
echo

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Function to check if command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Check prerequisites
echo -e "${YELLOW}Checking prerequisites...${NC}"
if ! command_exists gh; then
    echo -e "${RED}Error: GitHub CLI (gh) is not installed.${NC}"
    echo "Install it from: https://cli.github.com/"
    echo "Or use brew: brew install gh"
    exit 1
fi

if ! command_exists kubectl; then
    echo -e "${RED}Error: kubectl is not installed.${NC}"
    exit 1
fi

# Check if gh is authenticated
if ! gh auth status >/dev/null 2>&1; then
    echo -e "${RED}Error: GitHub CLI is not authenticated.${NC}"
    echo "Run: gh auth login"
    exit 1
fi

# Get repository info
REPO=$(gh repo view --json nameWithOwner -q .nameWithOwner 2>/dev/null || echo "")
if [ -z "$REPO" ]; then
    echo -e "${YELLOW}Could not detect repository. Please enter it manually.${NC}"
    read -p "GitHub repository (owner/repo): " REPO
fi

echo -e "${GREEN}Repository: $REPO${NC}"
echo

# Generate kubeconfig
echo -e "${YELLOW}Generating kubeconfig...${NC}"
kubectl config view --minify --flatten > /tmp/kubeconfig-orzbob.yaml
KUBE_CONFIG_BASE64=$(base64 -i /tmp/kubeconfig-orzbob.yaml | tr -d '\n')
echo -e "${GREEN}✓ Kubeconfig generated and encoded${NC}"

# Prepare secrets file
cat > /tmp/github-secrets.txt << EOF
=== GitHub Secrets to Add ===

Repository: $REPO
Navigate to: https://github.com/$REPO/settings/secrets/actions

Add the following secrets:

1. KUBE_CONFIG
   Value: $KUBE_CONFIG_BASE64

2. DEPLOY_NAMESPACE
   Value: orzbob-system

3. POLAR_API_KEY (required for billing)
   Value: <Get from https://polar.sh/settings/api-keys>
   Example: polar_sk_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx

4. POLAR_PROJECT_ID (required for billing)
   Value: <Get from your Polar project settings>
   Example: proj_xxxxxxxxxxxxxxxx

5. POLAR_WEBHOOK_SECRET (required for billing webhooks)
   Value: <Get from Polar webhook settings>

6. SMTP_HOST (optional for email alerts)
   Value: <Your SMTP server>
   Example: smtp.sendgrid.net

7. SMTP_PORT (optional for email alerts)
   Value: <Your SMTP port>
   Example: 587

8. SMTP_USERNAME (optional for email alerts)
   Value: <Your SMTP username>
   Example: apikey

9. SMTP_PASSWORD (optional for email alerts)
   Value: <Your SMTP password/API key>

10. EMAIL_FROM_ADDRESS (optional for email alerts)
    Value: <Sender email address>
    Example: noreply@orzbob.cloud

EOF

# Ask if user wants to set secrets via CLI
echo
echo -e "${YELLOW}Would you like to set these secrets using GitHub CLI?${NC}"
echo "Note: You'll still need to provide the actual secret values."
read -p "Set secrets via CLI? (y/n): " -n 1 -r
echo

if [[ $REPLY =~ ^[Yy]$ ]]; then
    echo
    echo -e "${GREEN}Setting up secrets via GitHub CLI...${NC}"
    
    # Set KUBE_CONFIG
    echo -e "${YELLOW}Setting KUBE_CONFIG...${NC}"
    echo "$KUBE_CONFIG_BASE64" | gh secret set KUBE_CONFIG --repo=$REPO
    echo -e "${GREEN}✓ KUBE_CONFIG set${NC}"
    
    # Set DEPLOY_NAMESPACE
    echo -e "${YELLOW}Setting DEPLOY_NAMESPACE...${NC}"
    echo "orzbob-system" | gh secret set DEPLOY_NAMESPACE --repo=$REPO
    echo -e "${GREEN}✓ DEPLOY_NAMESPACE set${NC}"
    
    # Ask for Polar credentials
    echo
    echo -e "${YELLOW}Polar.sh Configuration${NC}"
    echo "Get these values from https://polar.sh"
    
    read -p "Enter POLAR_API_KEY (starts with polar_sk_): " POLAR_API_KEY
    if [ ! -z "$POLAR_API_KEY" ]; then
        echo "$POLAR_API_KEY" | gh secret set POLAR_API_KEY --repo=$REPO
        echo -e "${GREEN}✓ POLAR_API_KEY set${NC}"
    fi
    
    read -p "Enter POLAR_PROJECT_ID (starts with proj_): " POLAR_PROJECT_ID
    if [ ! -z "$POLAR_PROJECT_ID" ]; then
        echo "$POLAR_PROJECT_ID" | gh secret set POLAR_PROJECT_ID --repo=$REPO
        echo -e "${GREEN}✓ POLAR_PROJECT_ID set${NC}"
    fi
    
    read -p "Enter POLAR_WEBHOOK_SECRET: " POLAR_WEBHOOK_SECRET
    if [ ! -z "$POLAR_WEBHOOK_SECRET" ]; then
        echo "$POLAR_WEBHOOK_SECRET" | gh secret set POLAR_WEBHOOK_SECRET --repo=$REPO
        echo -e "${GREEN}✓ POLAR_WEBHOOK_SECRET set${NC}"
    fi
    
    # Ask about email configuration
    echo
    echo -e "${YELLOW}Email Configuration (optional)${NC}"
    read -p "Configure email alerts? (y/n): " -n 1 -r
    echo
    
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        read -p "SMTP_HOST: " SMTP_HOST
        [ ! -z "$SMTP_HOST" ] && echo "$SMTP_HOST" | gh secret set SMTP_HOST --repo=$REPO
        
        read -p "SMTP_PORT: " SMTP_PORT
        [ ! -z "$SMTP_PORT" ] && echo "$SMTP_PORT" | gh secret set SMTP_PORT --repo=$REPO
        
        read -p "SMTP_USERNAME: " SMTP_USERNAME
        [ ! -z "$SMTP_USERNAME" ] && echo "$SMTP_USERNAME" | gh secret set SMTP_USERNAME --repo=$REPO
        
        read -s -p "SMTP_PASSWORD: " SMTP_PASSWORD
        echo
        [ ! -z "$SMTP_PASSWORD" ] && echo "$SMTP_PASSWORD" | gh secret set SMTP_PASSWORD --repo=$REPO
        
        read -p "EMAIL_FROM_ADDRESS: " EMAIL_FROM_ADDRESS
        [ ! -z "$EMAIL_FROM_ADDRESS" ] && echo "$EMAIL_FROM_ADDRESS" | gh secret set EMAIL_FROM_ADDRESS --repo=$REPO
        
        echo -e "${GREEN}✓ Email configuration set${NC}"
    fi
    
    echo
    echo -e "${GREEN}✅ Secrets configuration complete!${NC}"
    
    # List configured secrets
    echo
    echo -e "${YELLOW}Configured secrets:${NC}"
    gh secret list --repo=$REPO
    
else
    echo
    echo -e "${YELLOW}Manual setup required. Secret values saved to:${NC}"
    echo "/tmp/github-secrets.txt"
    echo
    echo "Follow the instructions in the file to add secrets manually."
fi

# Create verification script
cat > /tmp/verify-github-deployment.sh << 'EOF'
#!/bin/bash
# Verify GitHub deployment readiness

echo "=== GitHub Deployment Verification ==="
echo

# Check if all required secrets are set
REPO=$(gh repo view --json nameWithOwner -q .nameWithOwner)
echo "Repository: $REPO"
echo

echo "Checking required secrets..."
REQUIRED_SECRETS=("KUBE_CONFIG" "DEPLOY_NAMESPACE")
OPTIONAL_SECRETS=("POLAR_API_KEY" "POLAR_PROJECT_ID" "POLAR_WEBHOOK_SECRET" "SMTP_HOST" "SMTP_PORT" "SMTP_USERNAME" "SMTP_PASSWORD" "EMAIL_FROM_ADDRESS")

MISSING_REQUIRED=()
for secret in "${REQUIRED_SECRETS[@]}"; do
    if gh secret list --repo=$REPO | grep -q "^$secret"; then
        echo "✓ $secret"
    else
        echo "✗ $secret (REQUIRED)"
        MISSING_REQUIRED+=("$secret")
    fi
done

echo
echo "Checking optional secrets..."
for secret in "${OPTIONAL_SECRETS[@]}"; do
    if gh secret list --repo=$REPO | grep -q "^$secret"; then
        echo "✓ $secret"
    else
        echo "- $secret (optional)"
    fi
done

if [ ${#MISSING_REQUIRED[@]} -eq 0 ]; then
    echo
    echo "✅ All required secrets are configured!"
    echo "You can trigger a deployment by:"
    echo "1. Pushing to main branch"
    echo "2. Running: gh workflow run deploy-production.yml"
else
    echo
    echo "❌ Missing required secrets: ${MISSING_REQUIRED[@]}"
    echo "Please configure these before deploying."
fi
EOF

chmod +x /tmp/verify-github-deployment.sh

echo
echo -e "${GREEN}=== Next Steps ===${NC}"
echo "1. Ensure all secrets are configured"
echo "2. Run verification: /tmp/verify-github-deployment.sh"
echo "3. Trigger deployment:"
echo "   - Push to main branch"
echo "   - Or run: gh workflow run deploy-production.yml"
echo
echo -e "${YELLOW}Need help with Polar.sh setup?${NC}"
echo "See: /Users/milad/orzbob/docs/billing-setup.md"

# Clean up
rm -f /tmp/kubeconfig-orzbob.yaml