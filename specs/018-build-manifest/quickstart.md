# Quickstart: cc-deck Build Pipeline

## End-to-End Workflow

### Phase 1: Initialize (CLI)

```bash
cc-deck build init my-cc-image
cd my-cc-image
```

### Phase 2: Configure (AI, in Claude Code)

```bash
# Open the build directory in Claude Code
claude

# Analyze repositories for tool dependencies
/cc-deck.extract
# → Provide repo paths, review findings, accept/modify

# Add Claude Code plugins
/cc-deck.plugin
# → Select from marketplace or provide git URLs

# Add MCP servers
/cc-deck.mcp
# → Provide image references, auto-configure from labels

# Generate the Containerfile
/cc-deck.containerfile
# → AI resolves tools to install commands, generates Containerfile
```

### Phase 3: Build & Push (CLI)

```bash
# Build the container image
cc-deck build my-cc-image

# Push to registry
cc-deck push my-cc-image

# Or both at once (from Claude Code)
/cc-deck.publish
```

## Verification

```bash
# Verify the built image works
cc-deck build verify my-cc-image

# See what changed since last build
cc-deck build diff my-cc-image
```

## Manifest Example

After running `/cc-deck.extract` on a Go and Python project:

```yaml
version: 1

image:
  name: my-team/cc-deck-dev
  tag: latest
  base: ghcr.io/rhuss/cc-deck-base:latest

tools:
  - "Go compiler >= 1.23"
  - "Python 3.12 with uv"
  - "protoc and buf CLI"

sources:
  - url: https://github.com/org/api-service
    ref: main
    path: /home/user/projects/api-service
    detected_tools: ["Go 1.23", "protoc >= 25.0", "buf CLI"]
    detected_from: [go.mod, buf.yaml, .github/workflows/ci.yml]

plugins:
  - name: sdd
    source: marketplace

github_tools:
  - repo: rhuss/cc-setup
    binary: cc-setup
  - repo: rhuss/cc-session
    binary: cc-session

settings:
  zellij_config: current
```
