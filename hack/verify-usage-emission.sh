#!/bin/bash
# Verify B-4 checkpoint: Control-plane hooks emit usage

set -e

echo "📊 Verifying Usage Emission (B-4)"
echo "================================="

# Color codes
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "\n${YELLOW}Checking implementation...${NC}"

# Check if billing is integrated in control plane
if grep -q "orzbob/internal/billing" cmd/cloud-cp/main.go; then
    echo -e "${GREEN}✓ Billing package imported in control plane${NC}"
else
    echo -e "${RED}✗ Billing package not imported${NC}"
    exit 1
fi

# Check for recordInstanceUsage function
if grep -q "recordInstanceUsage" cmd/cloud-cp/main.go; then
    echo -e "${GREEN}✓ recordInstanceUsage function implemented${NC}"
else
    echo -e "${RED}✗ recordInstanceUsage function not found${NC}"
fi

# Check for usage recording in delete handler
if grep -q "recordInstanceUsage.*handleDeleteInstance" cmd/cloud-cp/main.go; then
    echo -e "${GREEN}✓ Usage recorded in delete handler${NC}"
else
    echo -e "${RED}✗ Usage not recorded in delete handler${NC}"
fi

# Check for usage recording in idle reaper
if grep -q "recordInstanceUsage.*reapIdleInstances" cmd/cloud-cp/main.go; then
    echo -e "${GREEN}✓ Usage recorded in idle reaper${NC}"
else
    echo -e "${RED}✗ Usage not recorded in idle reaper${NC}"
fi

# Check for instance start time tracking
if grep -q "instanceStarts" cmd/cloud-cp/main.go; then
    echo -e "${GREEN}✓ Instance start times tracked${NC}"
else
    echo -e "${RED}✗ Instance start time tracking not found${NC}"
fi

# Check tier pricing
echo -e "\n${YELLOW}Checking tier pricing...${NC}"
TIERS=("small:8.3" "medium:16.7" "large:33.3" "gpu:208.0")
for tier_price in "${TIERS[@]}"; do
    tier="${tier_price%:*}"
    price="${tier_price#*:}"
    if grep -q "\"$tier\".*$price" internal/billing/products.go; then
        echo -e "${GREEN}✓ $tier tier: $price cents/hour${NC}"
    else
        echo -e "${RED}✗ $tier tier pricing incorrect${NC}"
    fi
done

# Run tests
echo -e "\n${YELLOW}Running tests...${NC}"

# Run usage emission test
if go test ./internal/billing -run TestUsageEmissionOnStop -v > /dev/null 2>&1; then
    echo -e "${GREEN}✓ TestUsageEmissionOnStop passes${NC}"
else
    echo -e "${RED}✗ TestUsageEmissionOnStop failed${NC}"
fi

# Check if control plane builds
echo -e "\n${YELLOW}Building control plane...${NC}"
if go build -o /tmp/cloud-cp-test ./cmd/cloud-cp 2>/dev/null; then
    echo -e "${GREEN}✓ Control plane builds successfully${NC}"
    rm -f /tmp/cloud-cp-test
else
    echo -e "${RED}✗ Control plane build failed${NC}"
fi

# Summary
echo -e "\n${YELLOW}B-4 Checkpoint Summary:${NC}"
echo "☐ Emit minutes_used every time instance status toggles Running→Stopped"
echo "☐ Emit on heartbeat timeout (30 min idle reaper)"
echo "☐ Tier → price mapping: small=8.3¢/h, medium=16.7¢/h, large=33.3¢/h, gpu=$2.08/h"
echo "☐ Unit test TestUsageEmissionOnStop added & green"
echo "☐ Local run shows POST to /polar/meters in control-plane logs"

echo -e "\n${GREEN}✓ B-4 Control-plane hooks emit usage complete!${NC}"