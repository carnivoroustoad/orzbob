# Using Secrets in Orzbob Cloud

Secrets allow you to securely store sensitive configuration like database passwords, API keys, and other credentials that your cloud instances need.

## Creating a Secret

Use the control plane API to create a secret:

```bash
# Create a secret with database credentials
curl -X POST http://localhost:8080/v1/secrets \
  -H "Content-Type: application/json" \
  -d '{
    "name": "db-credentials",
    "data": {
      "DATABASE_URL": "postgres://user:password@localhost:5432/myapp",
      "DB_PASSWORD": "supersecret123"
    }
  }'

# Create a secret with API keys
curl -X POST http://localhost:8080/v1/secrets \
  -H "Content-Type: application/json" \
  -d '{
    "name": "api-keys",
    "data": {
      "GITHUB_TOKEN": "ghp_xxxxxxxxxxxx",
      "SLACK_WEBHOOK": "https://hooks.slack.com/services/xxx/yyy/zzz"
    }
  }'
```

## Creating an Instance with Secrets

When creating a cloud instance, specify which secrets to mount:

```bash
curl -X POST http://localhost:8080/v1/instances \
  -H "Content-Type: application/json" \
  -d '{
    "tier": "small",
    "secrets": ["db-credentials", "api-keys"]
  }'
```

## Accessing Secrets in Your Instance

Secrets are automatically mounted as environment variables in your instance. You can access them like any other environment variable:

```bash
# In your application code
echo $DATABASE_URL
echo $GITHUB_TOKEN

# In Python
import os
db_url = os.environ.get('DATABASE_URL')

# In Node.js
const dbUrl = process.env.DATABASE_URL;
```

## Managing Secrets

### List all secrets
```bash
curl http://localhost:8080/v1/secrets
```

### Get a specific secret (returns metadata only, not the data)
```bash
curl http://localhost:8080/v1/secrets/db-credentials
```

### Delete a secret
```bash
curl -X DELETE http://localhost:8080/v1/secrets/db-credentials
```

## Best Practices

1. **Never commit secrets to git** - Always use the secrets API instead of hardcoding values
2. **Use descriptive names** - Name secrets based on their purpose (e.g., `prod-db-credentials`)
3. **Rotate regularly** - Update secret values periodically for security
4. **Limit access** - Only mount secrets that an instance actually needs
5. **Use separate secrets** - Don't put all credentials in one secret; use multiple focused secrets

## Example: Full Application Setup

Here's a complete example setting up an application with database and external API access:

```bash
# 1. Create database secret
curl -X POST http://localhost:8080/v1/secrets \
  -H "Content-Type: application/json" \
  -d '{
    "name": "myapp-db",
    "data": {
      "PGHOST": "postgres.internal",
      "PGPORT": "5432",
      "PGUSER": "myapp",
      "PGPASSWORD": "secret123",
      "PGDATABASE": "myapp_prod"
    }
  }'

# 2. Create external API credentials
curl -X POST http://localhost:8080/v1/secrets \
  -H "Content-Type: application/json" \
  -d '{
    "name": "myapp-apis",
    "data": {
      "STRIPE_API_KEY": "sk_live_xxxxx",
      "SENDGRID_API_KEY": "SG.xxxxx",
      "AWS_ACCESS_KEY_ID": "AKIA...",
      "AWS_SECRET_ACCESS_KEY": "xxxxx"
    }
  }'

# 3. Create instance with both secrets
curl -X POST http://localhost:8080/v1/instances \
  -H "Content-Type: application/json" \
  -d '{
    "tier": "medium",
    "repo_url": "https://github.com/myorg/myapp",
    "secrets": ["myapp-db", "myapp-apis"]
  }'
```

Now your application has secure access to all required credentials without any hardcoded values!