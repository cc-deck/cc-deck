---
description: Add MCP servers to the cc-deck build manifest
---

## User Input

$ARGUMENTS

## Outline

Add MCP server sidecar configurations to `cc-deck-build.yaml`. Auto-configure from container image labels when available.

### Step 1: Get image reference

If the user provided an image reference in the input, use it. Otherwise ask:

"Provide a container image reference for the MCP server (e.g., 'ghcr.io/modelcontextprotocol/github-mcp:latest')."

### Step 2: Try auto-configuration from labels

Run the container runtime to inspect the image for `cc-deck.mcp/*` labels:

```bash
podman inspect <image> --format '{{json .Config.Labels}}' 2>/dev/null || \
docker inspect <image> --format '{{json .Config.Labels}}' 2>/dev/null
```

Look for these labels:
- `cc-deck.mcp/name`: Server name
- `cc-deck.mcp/transport`: `sse` or `stdio`
- `cc-deck.mcp/port`: Service port
- `cc-deck.mcp/auth-type`: `token`, `basic`, `oauth`, `none`
- `cc-deck.mcp/auth-env-vars`: Comma-separated env var names
- `cc-deck.mcp/description`: Human-readable description

### Step 3: Fill gaps interactively

If any required fields are missing from labels, ask the user:

- "What transport does this MCP server use? (sse/stdio)"
- "What port does it listen on?"
- "What authentication type does it require? (token/basic/oauth/none)"
- "What environment variables are needed for auth? (comma-separated)"

### Step 4: Present and confirm

Show the complete MCP entry and ask for confirmation before adding to the manifest.

### Step 5: Update the manifest

Add the MCP entry to the `mcp` section of `cc-deck-build.yaml`.

### Key Rules

- Always try to pull the image and read labels first
- Never include actual credential values, only env var names
- Check for duplicate names before adding
- Show the complete entry for review before writing
