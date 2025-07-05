# Quick Reference: GitHub Secrets Setup

## Required Secrets

1. **KUBE_CONFIG**
   - Your base64-encoded kubeconfig is saved at: `/tmp/kubeconfig-base64.txt`
   - Copy the entire content and add it as a secret

2. **DEPLOY_NAMESPACE**
   - Value: `orzbob-system`

## Billing Secrets (Required for billing features)

3. **POLAR_API_KEY**
   - Get from: https://polar.sh/settings/api-keys
   - Format: `polar_sk_...`

4. **POLAR_PROJECT_ID**
   - Get from your Polar project settings
   - Format: `proj_...`

5. **POLAR_WEBHOOK_SECRET**
   - Get from Polar webhook settings
   - Whatever format Polar provides

## Email Secrets (Optional)

6. **SMTP_HOST** - e.g., `smtp.sendgrid.net`
7. **SMTP_PORT** - e.g., `587`
8. **SMTP_USERNAME** - e.g., `apikey`
9. **SMTP_PASSWORD** - Your SMTP password/API key
10. **EMAIL_FROM_ADDRESS** - e.g., `noreply@orzbob.cloud`

## Setup Methods

### Option 1: Automated (Recommended)
```bash
# Run the setup script
./hack/setup-github-secrets.sh
```

### Option 2: Manual
1. Go to: https://github.com/YOUR_ORG/orzbob/settings/secrets/actions
2. Click "New repository secret"
3. Add each secret listed above

## Verify Setup
```bash
# Check if secrets are configured
gh secret list

# Or use the verification script
/tmp/verify-github-deployment.sh
```

## Trigger Deployment
```bash
# Option 1: Push to main branch
git push origin main

# Option 2: Manual trigger
gh workflow run deploy-production.yml
```