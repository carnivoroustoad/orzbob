# Orzbob Cloud Configuration Reference

This document provides a complete reference for the `.orz/cloud.yaml` configuration file.

## Overview

The cloud configuration file allows you to customize your Orzbob Cloud instances with:
- Initialization and attachment scripts
- Sidecar services (databases, caches, etc.)
- Environment variables
- Resource requirements

## File Location

Place the configuration file at `.orz/cloud.yaml` in your project root. Orzbob will automatically detect and use it when creating cloud instances.

## Schema

### Top-level Fields

```yaml
version: "1.0"              # Required: Configuration version
setup:                      # Optional: Lifecycle scripts
  init: string             # Script that runs once on creation
  onAttach: string         # Script that runs on each attach
services:                   # Optional: Sidecar containers
  <name>: ServiceConfig    
env:                        # Optional: Environment variables
  <key>: string
resources:                  # Optional: Resource requirements
  cpu: string
  memory: string
  gpu: integer
```

### Setup Scripts

Define scripts that run at specific points in the instance lifecycle:

```yaml
setup:
  # Runs once when instance is created
  init: |
    #!/bin/bash
    set -e
    
    echo "Setting up environment..."
    npm install
    cp .env.example .env
    ./scripts/setup.sh
    
  # Runs each time someone attaches
  onAttach: |
    #!/bin/bash
    
    echo "Welcome back!"
    git fetch origin
    git status
    
    # Show test status
    npm test -- --no-coverage || echo "Tests need attention"
```

**Script Environment:**
- Working directory: `/workspace` (your project root)
- Shell: `/bin/bash`
- User: `orzbob` (non-root)
- Available tools: git, curl, wget, common build tools

**Best Practices:**
- Keep init scripts idempotent 
- Use `set -e` for error handling
- Log important steps
- Keep onAttach scripts fast (<5 seconds)

### Services

Define sidecar containers that run alongside your main development environment:

```yaml
services:
  <service-name>:
    image: string           # Required: Docker image
    env:                    # Optional: Environment variables
      <key>: string
    ports: [integer]        # Optional: Exposed ports
    health:                 # Optional: Health check
      command: [string]     # Command to run
      interval: string      # How often (default: 30s)
      timeout: string       # Timeout (default: 5s)
      retries: integer      # Retries before unhealthy (default: 3)
```

**Example Services:**

```yaml
services:
  # PostgreSQL Database
  postgres:
    image: postgres:15-alpine
    env:
      POSTGRES_USER: myapp
      POSTGRES_PASSWORD: ${SECRET_DB_PASSWORD}
      POSTGRES_DB: development
    ports: [5432]
    health:
      command: ["pg_isready", "-U", "myapp"]
      interval: "10s"
      
  # Redis Cache
  redis:
    image: redis:7-alpine
    ports: [6379]
    health:
      command: ["redis-cli", "ping"]
      
  # MongoDB
  mongo:
    image: mongo:6
    env:
      MONGO_INITDB_ROOT_USERNAME: root
      MONGO_INITDB_ROOT_PASSWORD: ${SECRET_MONGO_PASSWORD}
    ports: [27017]
    
  # Elasticsearch
  elasticsearch:
    image: elasticsearch:8.11.0
    env:
      discovery.type: single-node
      xpack.security.enabled: "false"
      ES_JAVA_OPTS: "-Xms512m -Xmx512m"
    ports: [9200, 9300]
    
  # RabbitMQ
  rabbitmq:
    image: rabbitmq:3-management-alpine
    env:
      RABBITMQ_DEFAULT_USER: admin
      RABBITMQ_DEFAULT_PASS: ${SECRET_RABBITMQ_PASSWORD}
    ports: [5672, 15672]  # AMQP + Management UI
    
  # MySQL
  mysql:
    image: mysql:8
    env:
      MYSQL_ROOT_PASSWORD: ${SECRET_MYSQL_ROOT_PASSWORD}
      MYSQL_DATABASE: myapp
      MYSQL_USER: appuser
      MYSQL_PASSWORD: ${SECRET_MYSQL_PASSWORD}
    ports: [3306]
    health:
      command: ["mysqladmin", "ping", "-h", "localhost"]
```

**Networking:**
- Services are accessible at `localhost:<port>` from the main container
- Services can communicate with each other using service names
- Ports are not exposed externally (security)

### Environment Variables

Set environment variables available to all processes:

```yaml
env:
  # Database connections
  DATABASE_URL: postgresql://user:pass@localhost:5432/myapp
  REDIS_URL: redis://localhost:6379
  
  # Application settings
  NODE_ENV: development
  APP_ENV: dev
  DEBUG: "true"
  LOG_LEVEL: debug
  
  # Feature flags
  ENABLE_CACHE: "true"
  ENABLE_METRICS: "false"
  
  # External services (use secrets for sensitive values)
  API_BASE_URL: https://api.example.com
  STRIPE_PUBLIC_KEY: pk_test_123
  STRIPE_SECRET_KEY: ${SECRET_STRIPE_KEY}
```

**Variable Expansion:**
- `${SECRET_NAME}` - References a secret created with `orz cloud secrets`
- Standard shell expansion is NOT supported (security)

### Resources

Specify resource requirements for your instance:

```yaml
resources:
  cpu: "4"        # CPU cores (decimal allowed: "0.5", "2.5")
  memory: "8Gi"   # Memory (units: Mi, Gi)
  gpu: 1          # GPU count (gpu tier only)
```

**Available Tiers:**

| Tier   | CPU | Memory | GPU | Use Case |
|--------|-----|--------|-----|----------|
| small  | 2   | 4Gi    | 0   | Light development, bug fixes |
| medium | 4   | 8Gi    | 0   | Standard development, testing |
| large  | 8   | 16Gi   | 0   | Heavy builds, multiple services |
| gpu    | 8   | 24Gi   | 1   | ML/AI development (coming soon) |

**Notes:**
- Specifying resources overrides the tier defaults
- CPU can be fractional (e.g., "0.5" for half a core)
- Memory must use Kubernetes units (Ki, Mi, Gi)

## Complete Example

```yaml
version: "1.0"

setup:
  init: |
    #!/bin/bash
    set -e
    
    echo "🚀 Initializing full-stack development environment..."
    
    # Install dependencies
    echo "📦 Installing backend dependencies..."
    cd backend && npm install && cd ..
    
    echo "📦 Installing frontend dependencies..."
    cd frontend && npm install && cd ..
    
    # Setup databases
    echo "🗄️ Setting up databases..."
    cd backend
    npm run db:migrate
    npm run db:seed
    cd ..
    
    # Build frontend
    echo "🏗️ Building frontend..."
    cd frontend && npm run build && cd ..
    
    echo "✅ Setup complete!"
    
  onAttach: |
    echo "👋 Welcome to your full-stack dev environment!"
    echo ""
    echo "📊 Services Status:"
    echo "  PostgreSQL: $(pg_isready -h localhost && echo '✅ Ready' || echo '❌ Not ready')"
    echo "  Redis: $(redis-cli ping 2>/dev/null && echo '✅ Ready' || echo '❌ Not ready')"
    echo "  Frontend: http://localhost:3000"
    echo "  Backend API: http://localhost:4000"
    echo "  Admin UI: http://localhost:15672"
    echo ""
    echo "🏃 Starting development servers..."
    cd backend && npm run dev &
    cd frontend && npm run dev &

services:
  postgres:
    image: postgres:15-alpine
    env:
      POSTGRES_USER: appuser
      POSTGRES_PASSWORD: ${SECRET_DB_PASSWORD}
      POSTGRES_DB: myapp_dev
    ports: [5432]
    health:
      command: ["pg_isready", "-U", "appuser"]
      interval: "10s"
      timeout: "5s"
      retries: 5
      
  redis:
    image: redis:7-alpine
    ports: [6379]
    health:
      command: ["redis-cli", "ping"]
      
  rabbitmq:
    image: rabbitmq:3-management-alpine
    env:
      RABBITMQ_DEFAULT_USER: admin
      RABBITMQ_DEFAULT_PASS: ${SECRET_RABBITMQ_PASSWORD}
    ports: [5672, 15672]

env:
  # Database
  DATABASE_URL: postgresql://appuser:${SECRET_DB_PASSWORD}@localhost:5432/myapp_dev
  
  # Redis
  REDIS_URL: redis://localhost:6379
  
  # RabbitMQ
  AMQP_URL: amqp://admin:${SECRET_RABBITMQ_PASSWORD}@localhost:5672
  
  # Application
  NODE_ENV: development
  APP_ENV: development
  API_PORT: "4000"
  FRONTEND_PORT: "3000"
  
  # External APIs
  STRIPE_PUBLIC_KEY: pk_test_51234567890
  STRIPE_SECRET_KEY: ${SECRET_STRIPE_KEY}
  SENDGRID_API_KEY: ${SECRET_SENDGRID_KEY}
  
  # Feature flags
  ENABLE_DEBUG_TOOLBAR: "true"
  ENABLE_HOT_RELOAD: "true"

resources:
  cpu: "4"
  memory: "8Gi"
```

## Validation

Orzbob validates your configuration before creating instances. Common validation errors:

- **Invalid YAML syntax**: Check indentation and quotes
- **Unsupported version**: Only "1.0" is currently supported
- **Missing required fields**: Services must have an `image`
- **Invalid resource values**: CPU/memory must be valid Kubernetes quantities

## Tips and Tricks

1. **Start simple**: Begin with just `setup.init` and add services as needed
2. **Use health checks**: Ensure services are ready before your app starts
3. **Log everything**: Add echo statements to debug initialization
4. **Keep secrets secret**: Use `${SECRET_NAME}` syntax, never hardcode
5. **Test locally**: Validate YAML with `yamllint .orz/cloud.yaml`

## Limitations

- Maximum 5 sidecar services per instance
- Init scripts timeout after 5 minutes
- OnAttach scripts timeout after 30 seconds
- Total storage limited to 20GB per instance
- Services must use public Docker images

## Getting Help

- Check examples in `examples/` directory
- Run `orz cloud validate` to check your config
- Join our [Discord community](https://discord.gg/orzbob)
- Open an issue on [GitHub](https://github.com/carnivoroustoad/orzbob)