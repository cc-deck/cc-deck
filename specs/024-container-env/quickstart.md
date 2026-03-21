# Quickstart: Container Environment

**Feature**: 024-container-env | **Date**: 2026-03-20

## Setup

Ensure podman is installed:

```bash
# macOS
brew install podman
podman machine init && podman machine start

# Fedora/RHEL
sudo dnf install podman
```

## Create a Container Environment

```bash
# Create with default demo image
cc-deck env create mydev --type container

# Create with a specific image
cc-deck env create mydev --type container --image quay.io/cc-deck/cc-deck-demo:latest

# Create with port forwarding
cc-deck env create webdev --type container --image myimage:latest --port 8080:8080

# Create with host bind mount
cc-deck env create mydev --type container --storage host-path --path ~/projects/myapp

# Create with credentials
cc-deck env create mydev --type container --credential ANTHROPIC_API_KEY=$ANTHROPIC_API_KEY
```

## Attach to the Environment

```bash
cc-deck env attach mydev
# Opens interactive Zellij session inside the container
# Detach with: Ctrl+o d
```

## Lifecycle Operations

```bash
# Stop to free resources
cc-deck env stop mydev

# Restart (workspace data preserved)
cc-deck env start mydev

# Attach auto-starts if stopped
cc-deck env attach mydev

# Delete (removes container + volume)
cc-deck env delete mydev

# Delete but keep workspace data
cc-deck env delete mydev --keep-volumes
```

## File Transfer

```bash
# Push local files into the container
cc-deck env push mydev ./my-project

# Pull files out
cc-deck env pull mydev /workspace/results ./local-results
```

## Listing and Status

```bash
# List all environments
cc-deck env list

# Filter by type
cc-deck env list --type container

# Detailed status
cc-deck env status mydev
```

## Run Commands

```bash
cc-deck env exec mydev -- git status
cc-deck env exec mydev -- ls /workspace
```

## Configuration Defaults

Set defaults in `~/.config/cc-deck/config.yaml`:

```yaml
defaults:
  container:
    image: quay.io/my-org/my-image:latest
    storage: named-volume
```

## Hand-Edit Definitions

Environment definitions are human-editable at `~/.config/cc-deck/environments.yaml`:

```yaml
version: 1
environments:
  - name: mydev
    type: container
    image: quay.io/cc-deck/cc-deck-demo:latest
    storage:
      type: named-volume
    ports:
      - "8082:8082"
    credentials:
      - ANTHROPIC_API_KEY
```

Edit the file, then recreate the environment to pick up changes:

```bash
cc-deck env delete mydev
cc-deck env create mydev
```
