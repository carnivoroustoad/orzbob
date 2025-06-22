# Orzbob [![GitHub Release](https://img.shields.io/github/v/release/carnivoroustoad/orzbob)](https://github.com/carnivoroustoad/orzbob/releases/latest) [![Cloud CI/CD](https://github.com/carnivoroustoad/orzbob/actions/workflows/cloud-ci.yml/badge.svg)](https://github.com/carnivoroustoad/orzbob/actions/workflows/cloud-ci.yml)

Orzbob is a terminal app that helps you become a 100x engineer by managing multiple [Claude Code](https://github.com/anthropics/claude-code), [Codex](https://github.com/openai/codex) (and other local agents including [Aider](https://github.com/Aider-AI/aider)) in separate workspaces, allowing you to work on multiple tasks simultaneously.

🚀 [Visit our website](https://carnivoroustoad.github.io/orzbob/) for more information.

![Orzbob Screenshot](assets/screenshot.png)

### Highlights
- Complete tasks in the background (including yolo / auto-accept mode!)
- Manage instances and tasks in one terminal window
- Review changes before applying them, checkout changes before pushing them
- Each task gets its own isolated git workspace, so no conflicts

<br />

https://github.com/user-attachments/assets/aef18253-e58f-4525-9032-f5a3d66c975a

<br />

### Installation

The easiest way to install `orz` is by running the following command:

```bash
curl -fsSL https://raw.githubusercontent.com/carnivoroustoad/orzbob/main/install.sh | bash
```

This will install the `orz` binary to `~/.local/bin` and add it to your PATH. To install with a different name, use the `--name` flag:

```bash
curl -fsSL https://raw.githubusercontent.com/carnivoroustoad/orzbob/main/install.sh | bash -s -- --name <name>
```

Alternatively, you can also install `orz` by building from source or installing a [pre-built binary](https://github.com/carnivoroustoad/orzbob/releases).

### Prerequisites

- [tmux](https://github.com/tmux/tmux/wiki/Installing)
- [gh](https://cli.github.com/)

### Usage

```
Usage:
  orz [flags]
  orz [command]

Available Commands:
  completion  Generate the autocompletion script for the specified shell
  debug       Print debug information like config paths
  help        Help about any command
  reset       Reset all stored instances
  update      Check for and apply updates
  version     Print the version number of orz

Flags:
  -y, --autoyes          [experimental] If enabled, all instances will automatically accept prompts for claude code & aider
  -h, --help             help for orz
  -p, --program string   Program to run in new instances (e.g. 'aider --model ollama_chat/gemma3:1b')
```

Run the application with:

```bash
orz
```

<br />

<b>Using Orzbob with other AI assistants:</b>
- For [Codex](https://github.com/openai/codex): Set your API key with `export OPENAI_API_KEY=<your_key>`
- Launch with specific assistants:
   - Codex: `orz -p "codex"`
   - Aider: `orz -p "aider ..."`
- Make this the default, by modifying the config file (locate with `orz debug`)

<br />

#### Menu
The menu at the bottom of the screen shows available commands: 

##### Instance/Session Management
- `n` - Create a new session
- `N` - Create a new session with a prompt
- `D` - Kill (delete) the selected session
- `↑/j`, `↓/k` - Navigate between sessions

##### Actions
- `↵/o` - Attach to the selected session to reprompt
- `ctrl-q` - Detach from session
- `s` - Commit and push branch to github
- `c` - Checkout. Commits changes and pauses the session
- `r` - Resume a paused session
- `?` - Show help menu

##### Navigation
- `tab` - Switch between preview tab and diff tab
- `q` - Quit the application
- `shift-↓/↑` - scroll in diff view

## Orzbob Cloud (Beta) 🚀

Run your AI coding sessions in the cloud with dedicated compute resources, persistent workspaces, and seamless collaboration.

### Cloud Quick-Start

1. **Sign up for free tier** (2 concurrent instances)
   ```bash
   orz cloud auth
   ```

2. **Create your first cloud instance**
   ```bash
   orz cloud new "Implement user authentication"
   ```

3. **List and attach to instances**
   ```bash
   # List all your cloud instances
   orz cloud list
   
   # Attach to an instance
   orz cloud attach <instance-id>
   ```

4. **Clean up when done**
   ```bash
   orz cloud kill <instance-id>
   ```

📚 **[Full Cloud Quick-Start Guide](./docs/cloud-quickstart.md)** - Get up and running in under 15 minutes

### Cloud Features

- **Persistent Workspaces**: Your code, environment, and AI conversation history persist between sessions
- **Resource Tiers**: Choose from small (2 CPU, 4GB), medium (4 CPU, 8GB), or large (8 CPU, 16GB) instances
- **Sidecar Services**: Run databases and other services alongside your coding environment
- **Team Collaboration**: Share instances with team members (coming soon)
- **Automatic Idle Cleanup**: Instances auto-terminate after 30 minutes of inactivity

### Cloud Configuration

Customize your cloud instances with `.orz/cloud.yaml` in your project:

```yaml
version: "1.0"

# Setup scripts
setup:
  # Runs once when instance is created
  init: |
    echo "Setting up development environment..."
    npm install
    cp .env.example .env
    
  # Runs each time you attach
  onAttach: |
    echo "Welcome back! Here's the current status:"
    git status
    npm test -- --no-coverage

# Sidecar services
services:
  postgres:
    image: postgres:15
    env:
      POSTGRES_PASSWORD: secret
      POSTGRES_DB: myapp
    ports: [5432]
    health:
      command: ["pg_isready", "-U", "postgres"]
      
  redis:
    image: redis:7
    ports: [6379]
    health:
      command: ["redis-cli", "ping"]

# Environment variables
env:
  DATABASE_URL: postgresql://postgres:secret@localhost:5432/myapp
  REDIS_URL: redis://localhost:6379
  NODE_ENV: development

# Resource requirements (optional)
resources:
  cpu: "4"      # 4 CPU cores
  memory: "8Gi"  # 8GB RAM
```

### Cloud Commands

```bash
# Authentication
orz cloud auth              # Authenticate with Orzbob Cloud
orz cloud logout            # Log out of Orzbob Cloud

# Instance Management  
orz cloud new [prompt]      # Create a new instance
orz cloud list              # List all instances
orz cloud attach <id>       # Attach to an instance
orz cloud kill <id>         # Terminate an instance

# Advanced Options
orz cloud new --tier large  # Create a large instance
orz cloud new --secrets     # Attach secrets to instance
```

### Pricing

- **Free Tier**: 2 concurrent instances (small tier)
- **Pro**: $29/month - 5 concurrent instances, all tiers
- **Team**: $99/month - 20 concurrent instances, team features
- **Enterprise**: Contact us for custom limits and features

### Documentation

- 🚀 **[Cloud Quick-Start Guide](./docs/cloud-quickstart.md)** - Get started in 15 minutes
- 📖 **[Cloud Configuration Reference](./docs/cloud-config-reference.md)** - Complete cloud.yaml reference
- 🔐 **[Using Secrets Guide](./examples/using-secrets.md)** - Manage sensitive data securely
- 📦 **[Example Configurations](./examples/)** - Sample cloud.yaml files

### How It Works

1. **tmux** to create isolated terminal sessions for each agent
2. **git worktrees** to isolate codebases so each session works on its own branch
3. A simple TUI interface for easy navigation and management
4. **Auto-updates** to keep your installation current with the latest features

### Configuration

You can customize Orzbob's behavior by editing the config file (find its location with `orz debug`). Some notable options:

- `default_program`: Set your preferred AI assistant as default
- `enable_auto_update`: Enable or disable checking for updates on startup
- `auto_install_updates`: Automatically install updates without prompting

### License

[AGPL-3.0](LICENSE.md)
