#!/bin/bash

# Script to update the AWS Load Balancer Controller IAM policy
# This adds the missing elasticloadbalancing:DescribeListenerAttributes permission

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${YELLOW}AWS Load Balancer Controller IAM Policy Update Script${NC}"
echo "======================================================"

# The role ARN from the eksctl-created service account
ROLE_ARN="arn:aws:sts::011491784023:assumed-role/eksctl-orzbob-cluster-addon-iamserviceaccount-Role1-I1BTdW89nJXq"
ROLE_NAME="eksctl-orzbob-cluster-addon-iamserviceaccount-Role1-I1BTdW89nJXq"

echo -e "\n${YELLOW}Step 1: Getting the actual IAM role name from the assumed role...${NC}"
# Extract the actual IAM role name (not the assumed role)
ACTUAL_ROLE_NAME=$(echo $ROLE_NAME | sed 's/assumed-role\///')
echo "Role name: $ACTUAL_ROLE_NAME"

echo -e "\n${YELLOW}Step 2: Listing current policies attached to the role...${NC}"
aws iam list-attached-role-policies --role-name $ACTUAL_ROLE_NAME || {
    echo -e "${RED}Error: Could not list policies. Make sure you have the correct AWS credentials configured.${NC}"
    exit 1
}

echo -e "\n${YELLOW}Step 3: Finding the AWS Load Balancer Controller policy...${NC}"
# Get the policy ARN for the AWS Load Balancer Controller policy
POLICY_ARN=$(aws iam list-attached-role-policies --role-name $ACTUAL_ROLE_NAME \
    --query "AttachedPolicies[?contains(PolicyName, 'LoadBalancer') || contains(PolicyName, 'ALB')].PolicyArn" \
    --output text)

if [ -z "$POLICY_ARN" ]; then
    echo -e "${YELLOW}No existing Load Balancer Controller policy found. Creating a new one...${NC}"
    POLICY_NAME="AWSLoadBalancerControllerIAMPolicy"
    
    # Check if policy already exists
    EXISTING_POLICY=$(aws iam list-policies --scope Local \
        --query "Policies[?PolicyName=='$POLICY_NAME'].Arn" \
        --output text)
    
    if [ -n "$EXISTING_POLICY" ]; then
        echo "Policy $POLICY_NAME already exists. Creating new version..."
        POLICY_ARN=$EXISTING_POLICY
    else
        echo "Creating new policy $POLICY_NAME..."
        POLICY_ARN=$(aws iam create-policy \
            --policy-name $POLICY_NAME \
            --policy-document file://aws-load-balancer-controller-iam-policy.json \
            --query 'Policy.Arn' \
            --output text)
    fi
    
    echo "Attaching policy to role..."
    aws iam attach-role-policy \
        --role-name $ACTUAL_ROLE_NAME \
        --policy-arn $POLICY_ARN
else
    echo -e "Found policy: ${GREEN}$POLICY_ARN${NC}"
    
    echo -e "\n${YELLOW}Step 4: Updating the policy with the latest version...${NC}"
    
    # Get the current default version
    CURRENT_VERSION=$(aws iam get-policy --policy-arn $POLICY_ARN \
        --query 'Policy.DefaultVersionId' \
        --output text)
    
    echo "Current policy version: $CURRENT_VERSION"
    
    # Create a new policy version with the updated permissions
    echo "Creating new policy version with updated permissions..."
    NEW_VERSION=$(aws iam create-policy-version \
        --policy-arn $POLICY_ARN \
        --policy-document file://aws-load-balancer-controller-iam-policy.json \
        --set-as-default \
        --query 'PolicyVersion.VersionId' \
        --output text)
    
    echo -e "${GREEN}Successfully created and set new policy version: $NEW_VERSION${NC}"
    
    # Clean up old versions if there are too many (AWS limit is 5)
    echo -e "\n${YELLOW}Step 5: Cleaning up old policy versions...${NC}"
    VERSIONS=$(aws iam list-policy-versions --policy-arn $POLICY_ARN \
        --query 'Versions[?!IsDefaultVersion].VersionId' \
        --output text)
    
    VERSION_COUNT=$(echo $VERSIONS | wc -w)
    if [ $VERSION_COUNT -gt 3 ]; then
        echo "Found $VERSION_COUNT non-default versions. Cleaning up old ones..."
        OLDEST_VERSION=$(echo $VERSIONS | awk '{print $1}')
        aws iam delete-policy-version \
            --policy-arn $POLICY_ARN \
            --version-id $OLDEST_VERSION
        echo "Deleted old version: $OLDEST_VERSION"
    fi
fi

echo -e "\n${YELLOW}Step 6: Verifying the updated policy...${NC}"
# Verify the permission is present
aws iam get-policy-version \
    --policy-arn $POLICY_ARN \
    --version-id $(aws iam get-policy --policy-arn $POLICY_ARN --query 'Policy.DefaultVersionId' --output text) \
    --query 'PolicyVersion.Document' \
    --output json | jq '.Statement[].Action[]' | grep -q "elasticloadbalancing:DescribeListenerAttributes"

if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓ Successfully verified: elasticloadbalancing:DescribeListenerAttributes permission is present!${NC}"
else
    echo -e "${RED}✗ Warning: Could not verify the permission. Please check the policy manually.${NC}"
fi

echo -e "\n${GREEN}IAM policy update completed successfully!${NC}"
echo -e "\n${YELLOW}Next steps:${NC}"
echo "1. The AWS Load Balancer Controller pods may need to be restarted to pick up the new permissions."
echo "2. You can restart them with:"
echo "   kubectl rollout restart deployment aws-load-balancer-controller -n kube-system"
echo "3. Monitor the controller logs for any permission errors:"
echo "   kubectl logs -n kube-system -l app.kubernetes.io/name=aws-load-balancer-controller -f"