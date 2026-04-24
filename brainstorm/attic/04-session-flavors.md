# Brainstorm: Session Flavors and MCP Profiles

**Date**: 2026-03-03
**Status**: active
**Feature**: cc-deck (Kubernetes CLI)
**Affects**: spec.md, plan.md, tasks.md (new user story + modifications to existing deploy workflow)

## Problem

The current spec assumes a single container image for all sessions. In practice, different work requires different environments: Go development needs compilers and module proxy access, Python work needs pip/uv and PyPI access, business tasks need MCP servers configured, and quick experiments don't need persistent storage at all.

We need a way to select pre-configured environments when deploying sessions, without building a complex image management system into cc-deck.

## Concept: Flavors

A **flavor** is a YAML definition that describes a session environment. It references a pre-built container image and bundles deployment metadata (persistence, storage, egress, environment variables). Flavors are the unit of environment selection.

### What a Flavor Controls

| Aspect | Description | Overridable? |
|--------|-------------|-------------|
| Image coordinates | Registry + repository (e.g., `ghcr.io/rhuss/cc-deck`) | Via `--image` |
| Image tag | Specific version or variant (e.g., `dev-go-v1.0`) | Via `--tag` |
| Persistence | Whether to use StatefulSet + PVC or Deployment | Via `--ephemeral` / `--persistent` |
| Storage size | Default PVC size when persistent | Via `--storage` |
| Egress hosts | Additional allowlisted hosts beyond the AI backend | Via `--allow-egress` |
| MCP profile | Reference to a named MCP server configuration | Via `--mcp-profile` |
| Environment vars | Additional env vars injected into the container | N/A (flavor only) |

### Flavor YAML Format

```yaml
# ~/.config/cc-deck/flavors/dev-go.yaml (or embedded built-in)
name: dev-go
description: "Go development environment with compiler toolchain and module proxy access"
image: ghcr.io/rhuss/cc-deck
tag: dev-go-v1.0
persistence: true
storage_size: "20Gi"
egress:
  - "proxy.golang.org"
  - "sum.golang.org"
  - "*.github.com"
mcp_profile: dev-servers
env:
  GOPATH: /workspace/go
```

### Flavor Resolution

1. User specifies `--flavor dev-go` on deploy
2. cc-deck looks up the flavor: embedded built-ins first, then `~/.config/cc-deck/flavors/`
3. User-defined flavor with same name overrides the built-in
4. Flavor values become defaults, command-line flags override any individual field

### Deployment Model

```
persistence: true  -> StatefulSet + PVC + headless Service (current behavior)
persistence: false -> Deployment (ephemeral, no PVC, no headless Service needed)
```

When persistence is false, the session runs as a Deployment instead of a StatefulSet. No PVC is created. The workspace is ephemeral; restarting the Pod loses all state.

## Concept: MCP Profiles

MCP profiles are a separate concept from flavors. They define which MCP servers are available inside a session. MCP profiles are injected at runtime (not baked into images) so that cc-setup inside the container can manage them.

### How MCP Profiles Work

1. MCP profiles use **cc-setup's `mcp.json` format** (the central server registry format)
2. Profile files live in `~/.config/cc-deck/mcp-profiles/<name>.json`
3. At deploy time, cc-deck injects the selected MCP profile as a ConfigMap mounted into the Pod
4. The ConfigMap is mounted at cc-setup's config directory so cc-setup discovers the servers
5. cc-setup runs inside the container and can reflect on available servers

### Example MCP Profile

```json
{
  "servers": {
    "filesystem": {
      "description": "Local filesystem access",
      "type": "stdio",
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-filesystem", "/workspace"]
    },
    "github": {
      "description": "GitHub integration",
      "type": "http",
      "url": "https://mcp-github.example.com/mcp"
    }
  }
}
```

### Separation of Concerns

- **Flavor** = what image to run + how to deploy it
- **MCP profile** = what tools are available inside the session
- **Credential profile** = how to authenticate with the AI backend (existing concept)

A flavor can reference an MCP profile by name, but they are independently manageable.

## Built-in Flavors

### Distribution

Built-in flavor YAML files are **embedded in the cc-deck binary** via `go:embed`. They are always available and versioned with the CLI release.

### Export for Customization

`cc-deck flavor export <name>` copies a built-in flavor to `~/.config/cc-deck/flavors/` for user modification. `cc-deck flavor list` shows all available flavors (embedded + custom) with detailed descriptions.

### Initial Built-in Set

| Flavor | Description | Persistence | Extra Egress |
|--------|-------------|-------------|--------------|
| `base` | Claude Code + Zellij, minimal tools | true | none |
| `dev-go` | Go compiler + toolchain | true | proxy.golang.org, sum.golang.org |
| `dev-python` | Python + pip/uv | true | pypi.org, files.pythonhosted.org |
| `dev-node` | Node.js + npm | true | registry.npmjs.org |
| `dev-rust` | Rust compiler + cargo | true | crates.io, static.rust-lang.org |
| `business` | MCP-focused, no compilers | false | depends on MCP profile |

### Base Image Contents

All flavor images extend a common comfortable base containing:
- Zellij (terminal multiplexer with web server)
- Claude Code (native binary)
- cc-setup (MCP server management)
- git, ripgrep, fd, jq, yq, curl
- gh (GitHub CLI), glab (GitLab CLI)
- Basic editor (vim or nano)

## CLI Changes

### New Deploy Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--flavor` | string | `base` | Select flavor by name |
| `--tag` | string | From flavor | Override flavor's image tag |
| `--ephemeral` | bool | false | Force no persistence (Deployment) |
| `--persistent` | bool | false | Force persistence (StatefulSet) |
| `--mcp-profile` | string | From flavor | Override flavor's MCP profile |

The existing `--image` flag overrides the flavor's image coordinates entirely.

### New Subcommands

| Command | Description |
|---------|-------------|
| `cc-deck flavor list` | List available flavors (embedded + custom) with descriptions |
| `cc-deck flavor show <name>` | Show flavor details (image, persistence, egress, etc.) |
| `cc-deck flavor export <name>` | Export built-in flavor to config dir for customization |

### Egress Merging

When deploying, egress hosts are merged from three sources (union):
1. Credential profile's `allowed_egress`
2. Flavor's `egress` list
3. Command-line `--allow-egress` flags

## Out of Scope (Explicitly)

- **Image building**: cc-deck does not build container images. Users build with podman. A separate future project will address image creation, potentially leveraging devfile or devcontainer prior art.
- **Composable features**: Unlike devcontainer "Features" (modular install units), flavors are monolithic image references. Composability may come in a future iteration.
- **cc-session integration**: cc-session (local session finder) is not relevant inside deployed Pods. Session management is cc-deck's responsibility externally.

## Prior Art Considered

| Tool | Approach | What We Took | What We Left |
|------|----------|-------------|--------------|
| [DevPod](https://devpod.sh/) | Client-only, devcontainer.json, multi-backend | Client-only CLI architecture | IDE-centric design, devcontainer format |
| [Red Hat Dev Spaces](https://developers.redhat.com/products/openshift-dev-spaces) | Operator-based, devfile.yaml, browser IDE | "Golden image" pattern (Dell case study) | Server-side operator, devfile spec |
| [devcontainer.json](https://containers.dev/implementors/spec/) | Features as composable OCI units | Future inspiration for composability | Too IDE-centric for terminal-native use |
| [OpenShift AI Workbenches](https://developers.redhat.com/products/red-hat-openshift-ai/overview) | Pre-built workbench images + PVC storage | Image selection + runtime injection model | ML/notebook focus |
| [devfile.io](https://devfile.io/) | CNCF standard for dev environments | Awareness of the standard | Too complex for our use case |

## Key Differentiator

cc-deck is **AI-first and terminal-native**. Every tool above (Dev Spaces, DevPod, Codespaces, Gitpod) is designed around an IDE experience. cc-deck is designed around Claude Code + Zellij in a terminal. The flavor system reflects this focus: simple image selection with deployment metadata, not a full dev environment specification.
