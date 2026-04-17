# Quickstart: K8s Deploy Environment

**Feature**: 028-k8s-deploy | **Date**: 2026-03-27

## Prerequisites

- `kubectl` configured with access to a Kubernetes cluster
- A namespace where you have permissions to create StatefulSets, Services, Secrets, and PVCs
- A cc-deck container image accessible from the cluster (e.g., `quay.io/cc-deck/cc-deck-demo:latest`)

## Create a Basic Environment

```bash
# Create a persistent environment on your K8s cluster
cc-deck env create my-project \
  --type k8s-deploy \
  --namespace cc-deck \
  --credential ANTHROPIC_API_KEY=sk-ant-...
```

This creates a StatefulSet, headless Service, ConfigMap, PVC (10Gi default), credential Secret, and NetworkPolicy in the `cc-deck` namespace.

## Attach and Work

```bash
# Connect to the environment
cc-deck env attach my-project

# You're now inside a Zellij session in the K8s Pod
# Files saved to /workspace persist across sessions
```

## Stop and Resume

```bash
# Scale down (preserves storage)
cc-deck env stop my-project

# Scale back up
cc-deck env start my-project

# Re-attach (workspace files are still there)
cc-deck env attach my-project
```

## Push Code Into the Environment

```bash
# Transfer local files
cc-deck env push my-project ./src

# Or push a git repository
cc-deck env push my-project --git
```

## Harvest Work Back

```bash
# Fetch agent commits to a local branch
cc-deck env harvest my-project -b agent-work

# Harvest and create a PR
cc-deck env harvest my-project -b agent-work --pr
```

## Delete

```bash
# Remove all K8s resources
cc-deck env delete my-project --force
```

## Advanced: Custom Storage

```bash
cc-deck env create my-project \
  --type k8s-deploy \
  --namespace cc-deck \
  --storage-size 50Gi \
  --storage-class fast-ssd \
  --credential ANTHROPIC_API_KEY=sk-ant-...
```

## Advanced: Existing Secret

```bash
# Reference a pre-existing K8s Secret (not deleted on env delete)
cc-deck env create my-project \
  --type k8s-deploy \
  --namespace cc-deck \
  --existing-secret my-team-api-keys
```

## Advanced: Network Filtering

```bash
# Allow additional domains beyond the AI backend
cc-deck env create my-project \
  --type k8s-deploy \
  --namespace cc-deck \
  --allow-domain github.com \
  --allow-group nodejs \
  --credential ANTHROPIC_API_KEY=sk-ant-...

# Or disable network policy entirely
cc-deck env create my-project \
  --type k8s-deploy \
  --namespace cc-deck \
  --no-network-policy \
  --credential ANTHROPIC_API_KEY=sk-ant-...
```

## Advanced: MCP Sidecars

```bash
# Deploy with MCP servers from build manifest
cc-deck env create my-project \
  --type k8s-deploy \
  --namespace cc-deck \
  --build-dir ./my-project \
  --credential ANTHROPIC_API_KEY=sk-ant-... \
  --credential GH_TOKEN=ghp_...
```

## Advanced: External Secrets Operator

```bash
# Sync credentials from a vault
cc-deck env create my-project \
  --type k8s-deploy \
  --namespace cc-deck \
  --secret-store vault \
  --secret-store-ref my-vault \
  --secret-path secret/data/cc-deck/anthropic
```
