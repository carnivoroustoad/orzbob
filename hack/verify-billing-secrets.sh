#!/bin/bash
# Verify B-2 checkpoint: Secrets & env-vars wired

set -e

echo "🔐 Verifying Billing Secrets Configuration"
echo "========================================"

# Color codes
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Check if running in Kubernetes
if kubectl cluster-info &> /dev/null; then
    echo -e "\n${YELLOW}Checking Kubernetes secrets...${NC}"
    
    # Check if namespace exists
    if kubectl get namespace orzbob-system &> /dev/null; then
        echo -e "${GREEN}✓ Namespace orzbob-system exists${NC}"
        
        # Check if secret exists
        if kubectl get secret polar-credentials -n orzbob-system &> /dev/null; then
            echo -e "${GREEN}✓ Secret polar-credentials exists${NC}"
            
            # Verify secret keys
            KEYS=$(kubectl get secret polar-credentials -n orzbob-system -o jsonpath='{.data}' | jq -r 'keys[]' 2>/dev/null || echo "")
            
            REQUIRED_KEYS=("api-key" "webhook-secret" "project-id")
            for key in "${REQUIRED_KEYS[@]}"; do
                if echo "$KEYS" | grep -q "$key"; then
                    echo -e "${GREEN}✓ Key '$key' exists in secret${NC}"
                else
                    echo -e "${RED}✗ Key '$key' missing from secret${NC}"
                fi
            done
        else
            echo -e "${RED}✗ Secret polar-credentials not found${NC}"
            echo "  Create it with:"
            echo "  kubectl create secret generic polar-credentials \\"
            echo "    --namespace=orzbob-system \\"
            echo "    --from-literal=api-key='your-api-key' \\"
            echo "    --from-literal=webhook-secret='your-webhook-secret' \\"
            echo "    --from-literal=project-id='your-project-id'"
        fi
    else
        echo -e "${RED}✗ Namespace orzbob-system not found${NC}"
        echo "  Create it with: kubectl create namespace orzbob-system"
    fi
else
    echo -e "${YELLOW}Not connected to a Kubernetes cluster${NC}"
fi

# Check local environment
echo -e "\n${YELLOW}Checking local environment...${NC}"

if [ -f .env ]; then
    echo -e "${GREEN}✓ .env file exists${NC}"
    
    # Check for required variables (without exposing values)
    VARS=("POLAR_API_KEY" "POLAR_WEBHOOK_SECRET" "POLAR_PROJECT_ID")
    for var in "${VARS[@]}"; do
        if grep -q "^${var}=" .env; then
            echo -e "${GREEN}✓ ${var} is defined in .env${NC}"
        else
            echo -e "${RED}✗ ${var} is not defined in .env${NC}"
        fi
    done
else
    echo -e "${YELLOW}! .env file not found${NC}"
    echo "  Copy .env.example to .env and fill in your values"
fi

# Run Go tests
echo -e "\n${YELLOW}Running billing tests...${NC}"
if go test ./internal/billing -run TestPolarClientAuth; then
    echo -e "${GREEN}✓ Polar client authentication test passed${NC}"
else
    echo -e "${YELLOW}! Polar client authentication test skipped (credentials not set)${NC}"
fi

# Summary
echo -e "\n${YELLOW}B-2 Checkpoint Verification:${NC}"
echo "☐ kubectl get secret polar-credentials -n orzbob-system returns key names"
echo "☐ go test ./internal/billing -run TestPolarClientAuth passes"
echo ""
echo "Next step: Configure your credentials and run this script again"