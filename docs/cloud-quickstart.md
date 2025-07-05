# Orzbob Cloud Quick Start Guide

Welcome to Orzbob Cloud! This guide will help you get started with running your AI coding sessions in the cloud in under 15 minutes.

## Prerequisites

- Orzbob CLI installed (`curl -fsSL https://raw.githubusercontent.com/carnivoroustoad/orzbob/main/install.sh | bash`)
- A GitHub account (for authentication)
- An active internet connection

## Step 1: Authenticate (2 minutes)

First, authenticate with Orzbob Cloud:

```bash
orz cloud auth
```

This will open your browser to authenticate with GitHub. After successful authentication, you'll see:

```
✅ Successfully authenticated with Orzbob Cloud
   Email: your-email@example.com
   Plan: Free Tier (2 concurrent instances)
```

## Step 2: Create Your First Instance (3 minutes)

Create a cloud instance with a specific task:

```bash
orz cloud new "Help me build a REST API with authentication"
```

You'll see:

```
🚀 Creating cloud instance...
   Tier: small (2 CPU, 4GB RAM)
   Instance ID: runner-1234567890
   
⏳ Waiting for instance to be ready...
✅ Instance ready! Attaching...
```

## Step 3: Working in Your Cloud Instance (5 minutes)

Once attached, you'll be in a tmux session with Claude Code ready to help. Try these commands:

1. **Ask Claude to create a project structure:**
   ```
   Create a Node.js REST API with Express and JWT authentication
   ```

2. **Review the generated code:**
   - Press `Ctrl+B` then `d` to detach from the session
   - You'll see the file changes in the preview pane

3. **Re-attach to continue working:**
   ```bash
   orz cloud attach runner-1234567890
   ```

## Step 4: Using Cloud Configuration (3 minutes)

For projects that need databases or special setup, create `.orz/cloud.yaml`:

```bash
mkdir -p .orz
cat > .orz/cloud.yaml << 'EOF'
version: "1.0"

setup:
  init: |
    echo "Installing dependencies..."
    npm install
    
  onAttach: |
    echo "Welcome back!"
    npm test || true

services:
  postgres:
    image: postgres:15-alpine
    env:
      POSTGRES_PASSWORD: secret
      POSTGRES_DB: myapp
    ports: [5432]

env:
  DATABASE_URL: postgresql://postgres:secret@localhost:5432/myapp
  NODE_ENV: development
EOF
```

Then create a new instance:

```bash
orz cloud new --tier medium "Set up the database schema"
```

## Step 5: Managing Instances (2 minutes)

### List your instances:
```bash
orz cloud list
```

Output:
```
ID                      STATUS    TIER     CREATED           LAST ACTIVE
runner-1234567890      Running   small    10 minutes ago    2 minutes ago
runner-0987654321      Running   medium   2 minutes ago     Active now

Quota: 2/2 instances (Free Tier)
```

### Terminate an instance:
```bash
orz cloud kill runner-1234567890
```

## Common Workflows

### Working with Secrets

Store API keys and sensitive data securely:

```bash
# Create a secret
orz cloud secrets create api-keys --data API_KEY=sk-1234567890

# Create instance with secrets
orz cloud new --secrets api-keys "Integrate with OpenAI API"
```

### Team Collaboration (Pro/Team plans)

Share an instance with team members:

```bash
orz cloud share runner-1234567890 --with teammate@example.com
```

### Using Different Tiers

```bash
# Small tier (default): 2 CPU, 4GB RAM
orz cloud new "Fix bug in authentication"

# Medium tier: 4 CPU, 8GB RAM  
orz cloud new --tier medium "Run integration tests"

# Large tier: 8 CPU, 16GB RAM
orz cloud new --tier large "Train ML model"
```

## Troubleshooting

### Instance won't start
- Check your quota with `orz cloud list`
- Ensure your cloud.yaml is valid YAML
- Check logs with `orz cloud logs <instance-id>`

### Can't connect to sidecar services
- Services take 10-30 seconds to start
- Check health with `orz cloud status <instance-id>`
- Ensure ports aren't conflicting

### Authentication issues
- Run `orz cloud logout` then `orz cloud auth` again
- Check your GitHub account has access to Orzbob Cloud

## Best Practices

1. **Use cloud.yaml for complex projects** - Define your environment once
2. **Clean up unused instances** - They count against your quota
3. **Use appropriate tiers** - Start small, upgrade if needed
4. **Store secrets properly** - Never commit them to git
5. **Set up init scripts** - Automate your setup process

## Next Steps

- Read the full [Cloud Configuration Reference](./cloud-config-reference.md)
- Learn about [Using Secrets](../examples/using-secrets.md)
- Explore [Advanced Cloud Features](./cloud-advanced.md)
- Join our [Discord community](https://discord.gg/orzbob) for help

## Example Projects

Check out these example configurations in the `examples/` directory:

- `cloud.yaml` - Basic configuration with common patterns
- `cloud-with-sidecars.yaml` - Full stack with PostgreSQL and Redis
- `using-secrets.md` - Guide for managing sensitive data

Happy coding in the cloud! 🚀