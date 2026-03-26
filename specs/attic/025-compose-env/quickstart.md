# Quickstart: Compose Environment

## Prerequisites

- `cc-deck` CLI installed
- `podman` and `podman-compose` installed and in PATH
- A project directory to work in

## Basic Usage

### Create a compose environment

Navigate to your project directory and create a compose environment:

```bash
cd ~/projects/my-api
cc-deck env create mydev --type compose
```

This generates orchestration files in `.cc-deck/`, starts a container with your project mounted at `/workspace`, and records the environment.

### Attach to the environment

```bash
cc-deck env attach mydev
```

Opens a Zellij session with the cc-deck sidebar plugin. Your project files are available at `/workspace` with bidirectional sync.

### Stop and restart

```bash
cc-deck env stop mydev    # Free resources
cc-deck env start mydev   # Resume where you left off
```

### Delete the environment

```bash
cc-deck env delete mydev  # Removes containers + .cc-deck/ directory
```

## Network Filtering

Create an environment that restricts outbound network access to specific domains:

```bash
cc-deck env create filtered-dev --type compose \
  --allowed-domains anthropic,github,npm
```

This adds a proxy sidecar. The session container can only reach allowed domains.

Test it from inside:
```bash
curl https://api.anthropic.com  # Works
curl https://example.com        # Blocked
```

## Common Options

```bash
# Custom image
cc-deck env create mydev --type compose --image quay.io/cc-deck/cc-deck-demo:v1.0.0

# Explicit project path
cc-deck env create mydev --type compose --path /home/user/projects/my-api

# Named volume instead of bind mount
cc-deck env create mydev --type compose --storage named-volume

# Port forwarding
cc-deck env create mydev --type compose --port 8080:8080

# Auto-add .cc-deck/ to .gitignore
cc-deck env create mydev --type compose --gitignore

# Explicit credentials
cc-deck env create mydev --type compose --credential ANTHROPIC_API_KEY=sk-ant-...
```

## Lifecycle Commands

All standard environment commands work with compose environments:

| Command | Description |
|---------|-------------|
| `cc-deck env create <name> --type compose` | Create environment |
| `cc-deck env attach <name>` | Attach to session |
| `cc-deck env start <name>` | Start stopped environment |
| `cc-deck env stop <name>` | Stop running environment |
| `cc-deck env delete <name>` | Delete environment and artifacts |
| `cc-deck env status <name>` | Show environment status |
| `cc-deck env list` | List all environments |
| `cc-deck env exec <name> -- <cmd>` | Run command inside |
| `cc-deck env push <name> <path>` | Copy files into environment |
| `cc-deck env pull <name> <path>` | Copy files from environment |
