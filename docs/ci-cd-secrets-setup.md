# CI/CD Secrets Setup Guide for Orzbob Cloud

This guide explains how to configure secrets for automatic deployment through GitHub Actions.

## Overview

The Orzbob Cloud deployment requires several secrets to be configured at different levels:
1. **GitHub Secrets** - For CI/CD pipeline
2. **Kubernetes Secrets** - For runtime services
3. **Environment Variables** - For local development

## Required Secrets

### 1. Billing Secrets (Polar.sh)
- `POLAR_API_KEY` - API key from Polar.sh dashboard
- `POLAR_PROJECT_ID` - Your Polar project ID
- `POLAR_WEBHOOK_SECRET` - Webhook signing secret

### 2. Email Configuration (Optional but recommended)
- `SMTP_HOST` - SMTP server hostname
- `SMTP_PORT` - SMTP server port
- `SMTP_USERNAME` - SMTP authentication username
- `SMTP_PASSWORD` - SMTP authentication password
- `EMAIL_FROM_ADDRESS` - Sender email address

### 3. Deployment Configuration
- `KUBE_CONFIG` - Base64-encoded kubeconfig for target cluster
- `DEPLOY_NAMESPACE` - Kubernetes namespace (default: orzbob-system)

## Step-by-Step Setup

### Step 1: Get Polar.sh Credentials

1. Sign up at [polar.sh](https://polar.sh)
2. Create a new project for Orzbob Cloud
3. Navigate to Settings → API Keys
4. Create a new API key with full permissions
5. Copy the API key (starts with `polar_sk_`)
6. Get your project ID from the URL or project settings
7. Set up webhooks and copy the webhook secret

### Step 2: Configure GitHub Repository Secrets

1. Go to your GitHub repository
2. Navigate to Settings → Secrets and variables → Actions
3. Add the following repository secrets:

```yaml
# Required for billing
POLAR_API_KEY: polar_sk_your_actual_key_here
POLAR_PROJECT_ID: proj_your_project_id
POLAR_WEBHOOK_SECRET: whsec_your_webhook_secret

# Optional for email alerts
SMTP_HOST: smtp.sendgrid.net
SMTP_PORT: 587
SMTP_USERNAME: apikey
SMTP_PASSWORD: your_sendgrid_api_key
EMAIL_FROM_ADDRESS: noreply@yourdomain.com

# Required for deployment
KUBE_CONFIG: <base64-encoded-kubeconfig>
DEPLOY_NAMESPACE: orzbob-system
```

### Step 3: Update GitHub Actions Workflow

Create or update `.github/workflows/deploy.yml`:

```yaml
name: Deploy to Production

on:
  push:
    branches:
      - main
  workflow_dispatch:

env:
  REGISTRY: ghcr.io
  IMAGE_NAME: ${{ github.repository }}

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - name: Log in to Container Registry
        uses: docker/login-action@v3
        with:
          registry: ${{ env.REGISTRY }}
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}
      
      - name: Setup Kubernetes
        run: |
          echo "${{ secrets.KUBE_CONFIG }}" | base64 -d > /tmp/kubeconfig
          echo "KUBECONFIG=/tmp/kubeconfig" >> $GITHUB_ENV
      
      - name: Create namespace if not exists
        run: |
          kubectl create namespace ${{ secrets.DEPLOY_NAMESPACE }} --dry-run=client -o yaml | kubectl apply -f -
      
      - name: Create Polar credentials secret
        run: |
          kubectl create secret generic polar-credentials \
            --from-literal=api-key="${{ secrets.POLAR_API_KEY }}" \
            --from-literal=project-id="${{ secrets.POLAR_PROJECT_ID }}" \
            --from-literal=webhook-secret="${{ secrets.POLAR_WEBHOOK_SECRET }}" \
            --namespace=${{ secrets.DEPLOY_NAMESPACE }} \
            --dry-run=client -o yaml | kubectl apply -f -
      
      - name: Create email configuration secret
        if: ${{ secrets.SMTP_HOST != '' }}
        run: |
          kubectl create secret generic email-config \
            --from-literal=smtp-host="${{ secrets.SMTP_HOST }}" \
            --from-literal=smtp-port="${{ secrets.SMTP_PORT }}" \
            --from-literal=smtp-username="${{ secrets.SMTP_USERNAME }}" \
            --from-literal=smtp-password="${{ secrets.SMTP_PASSWORD }}" \
            --from-literal=from-address="${{ secrets.EMAIL_FROM_ADDRESS }}" \
            --namespace=${{ secrets.DEPLOY_NAMESPACE }} \
            --dry-run=client -o yaml | kubectl apply -f -
      
      - name: Deploy with Helm
        run: |
          helm upgrade --install orzbob-cloud ./charts/cp \
            --namespace=${{ secrets.DEPLOY_NAMESPACE }} \
            --set image.repository=${{ env.REGISTRY }}/${{ github.repository }}/cloud-cp \
            --set image.tag=${{ github.sha }} \
            --set billing.enabled=true \
            --set billing.existingSecret=polar-credentials \
            --set email.existingSecret=email-config \
            --wait
```

### Step 4: Update Helm Values

Update `charts/cp/values.yaml` to reference secrets:

```yaml
billing:
  enabled: true
  existingSecret: polar-credentials
  # Or specify directly (not recommended for production)
  # polarApiKey: ""
  # polarProjectId: ""
  # polarWebhookSecret: ""

email:
  enabled: true
  existingSecret: email-config
  # Or specify directly (not recommended for production)
  # smtpHost: ""
  # smtpPort: ""
  # smtpUsername: ""
  # smtpPassword: ""
  # fromAddress: ""

# Other configuration...
```

### Step 5: Update Kubernetes Deployment

Ensure your deployment references the secrets. Update `charts/cp/templates/deployment.yaml`:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ include "cp.fullname" . }}
spec:
  template:
    spec:
      containers:
      - name: control-plane
        env:
        {{- if .Values.billing.enabled }}
        - name: BILLING_ENABLED
          value: "true"
        {{- if .Values.billing.existingSecret }}
        - name: POLAR_API_KEY
          valueFrom:
            secretKeyRef:
              name: {{ .Values.billing.existingSecret }}
              key: api-key
        - name: POLAR_PROJECT_ID
          valueFrom:
            secretKeyRef:
              name: {{ .Values.billing.existingSecret }}
              key: project-id
        - name: POLAR_WEBHOOK_SECRET
          valueFrom:
            secretKeyRef:
              name: {{ .Values.billing.existingSecret }}
              key: webhook-secret
        {{- end }}
        {{- end }}
        
        {{- if .Values.email.enabled }}
        {{- if .Values.email.existingSecret }}
        - name: SMTP_HOST
          valueFrom:
            secretKeyRef:
              name: {{ .Values.email.existingSecret }}
              key: smtp-host
        - name: SMTP_PORT
          valueFrom:
            secretKeyRef:
              name: {{ .Values.email.existingSecret }}
              key: smtp-port
        - name: SMTP_USERNAME
          valueFrom:
            secretKeyRef:
              name: {{ .Values.email.existingSecret }}
              key: smtp-username
        - name: SMTP_PASSWORD
          valueFrom:
            secretKeyRef:
              name: {{ .Values.email.existingSecret }}
              key: smtp-password
        - name: EMAIL_FROM_ADDRESS
          valueFrom:
            secretKeyRef:
              name: {{ .Values.email.existingSecret }}
              key: from-address
        {{- end }}
        {{- end }}
```

## Local Development Setup

For local development, create a `.env` file:

```bash
# Copy the example
cp .env.example .env

# Edit with your credentials
vim .env
```

Add to `.gitignore`:
```
.env
.env.*
!.env.example
```

## Verification

After deployment, verify secrets are properly configured:

1. **Check Kubernetes secrets exist:**
   ```bash
   kubectl get secrets -n orzbob-system
   ```

2. **Verify billing configuration:**
   ```bash
   kubectl exec -n orzbob-system deployment/orzbob-cloud-cp -- env | grep POLAR
   ```

3. **Test billing endpoint:**
   ```bash
   kubectl port-forward -n orzbob-system svc/orzbob-cloud-cp 8080:80
   curl http://localhost:8080/v1/billing
   ```

4. **Check logs for errors:**
   ```bash
   kubectl logs -n orzbob-system deployment/orzbob-cloud-cp | grep -i "billing\|polar\|smtp"
   ```

## Security Best Practices

1. **Never commit secrets to git**
   - Use `.gitignore` for local files
   - Use GitHub Secrets for CI/CD
   - Use Kubernetes Secrets for runtime

2. **Rotate secrets regularly**
   - Update Polar API keys quarterly
   - Rotate SMTP passwords if compromised
   - Use short-lived kubeconfig tokens

3. **Limit secret access**
   - Use GitHub environments for production secrets
   - Implement RBAC in Kubernetes
   - Audit secret usage regularly

4. **Use secret scanning**
   - Enable GitHub secret scanning
   - Use pre-commit hooks to prevent leaks
   - Regular security audits

## Troubleshooting

### Common Issues

1. **"Invalid API key" errors**
   - Verify the secret is base64-encoded correctly
   - Check for extra whitespace or newlines
   - Ensure the key starts with `polar_sk_`

2. **Email not sending**
   - Verify SMTP credentials
   - Check firewall rules for SMTP port
   - Test with telnet: `telnet smtp.host 587`

3. **Secrets not found in pod**
   - Check secret exists in correct namespace
   - Verify deployment references correct secret name
   - Check RBAC permissions

### Debug Commands

```bash
# Decode a secret
kubectl get secret polar-credentials -n orzbob-system -o jsonpath='{.data.api-key}' | base64 -d

# Test SMTP connection
kubectl run smtp-test --rm -it --image=busybox -- sh
# Then: telnet smtp.sendgrid.net 587

# Check environment variables
kubectl exec -n orzbob-system deployment/orzbob-cloud-cp -- env | sort
```

## Monitoring

Set up alerts for:
- Failed billing API calls
- SMTP connection failures  
- Secret expiration (if using time-limited tokens)
- Deployment failures due to missing secrets

## Next Steps

1. Set up all required secrets in GitHub
2. Update your workflow files as shown above
3. Test deployment to a staging environment
4. Enable branch protection requiring successful deployment
5. Set up monitoring and alerts
6. Document any custom secret requirements

Remember: The security of your billing system depends on proper secret management. Take time to set this up correctly!