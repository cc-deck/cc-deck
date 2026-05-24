# MCP Endpoint Policy Integration

## Problem

MCP server endpoints baked into the sandbox image via `cc-setup-mcp.json` are blocked by the OpenShell supervisor because they are not in the network policy. Claude Code tries to connect to all configured MCP servers at startup, and when the supervisor blocks them, it hangs waiting for connections to time out.

For example, `mcp-google-work.int-tichny.org:8443` and `mcp.atlassian.com:443` produce repeated DENIED log entries and prevent Claude Code from starting.

## Solution

Add MCP server endpoints to the OpenShell network policy automatically during policy assembly.

### Manifest changes

Add an `endpoint` field to `MCPEntry` in the manifest schema:

```yaml
mcp:
  - name: google-work
    transport: http
    endpoint: mcp-google-work.int-tichny.org:8443
    description: "Google Workspace (work)"
  - name: jira-redhat
    transport: stdio
    endpoint: mcp.atlassian.com:443
    description: "Red Hat Jira (npx mcp-remote)"
  - name: playwright
    transport: stdio
    description: "Browser automation via Playwright (npx)"
```

The `endpoint` field uses `host:port` format. MCP servers without an endpoint (purely local stdio servers like `playwright`) produce no policy entry.

### Capture changes

The `/cc-deck.capture` command already discovers MCP servers from Claude Code settings. Extend it to also extract the endpoint URL:

- HTTP/SSE servers: parse the `url` field, extract host and port.
- Stdio servers with `mcp-remote`: scan `args` for URLs matching `https://...`, extract host and port.
- Local stdio servers (no URL in args): no endpoint extracted.

Present the extracted endpoint to the user for confirmation during capture.

### Policy assembly

In `assemblePolicyCore()`, after processing credentials, iterate `manifest.MCP` entries. For each entry with a non-empty `endpoint`, create a network policy entry keyed as `mcp_<slugified-name>`:

- Endpoints: single entry with the parsed host and port.
- Binaries: Claude Code's explicit binary paths (from the embedded `claude-code.yaml` component).
- Name: the MCP server's `description` field.

### pkg_node binary augmentation

Claude Code spawns `npx` to run stdio MCP servers, which downloads scoped npm packages (e.g., `@anthropic-ai/mcp-remote`). The `pkg_node` component must allow Claude Code's binaries to reach npm registries. During policy assembly, if `pkg_node` is a matched component, append the `claude_code` component's binaries to its binary list.

### Example output

For a manifest with two MCP servers, the generated policy includes:

```yaml
mcp_google_work:
  name: Google Workspace (work)
  endpoints:
    - host: mcp-google-work.int-tichny.org
      port: 8443
  binaries:
    - path: /usr/local/bin/claude
    - path: /sandbox/.local/bin/claude
    - path: /sandbox/.local/share/claude/**
    - path: /usr/bin/node

mcp_jira_redhat:
  name: Red Hat Jira (npx mcp-remote)
  endpoints:
    - host: mcp.atlassian.com
      port: 443
  binaries:
    - path: /usr/local/bin/claude
    - path: /sandbox/.local/bin/claude
    - path: /sandbox/.local/share/claude/**
    - path: /usr/bin/node
```

### Files to modify

1. `cc-deck/internal/build/manifest.go` - Add `Endpoint` field to `MCPEntry`
2. `cc-deck/internal/build/policy.go` - Add MCP endpoint processing in `assemblePolicyCore()`, add `pkg_node` binary augmentation
3. `cc-deck/internal/build/policy_test.go` - Tests for MCP policy entries and pkg_node augmentation
4. `cc-deck/internal/build/commands/cc-deck.capture.md` - Update capture command to extract MCP endpoints
5. `docs/modules/reference/pages/configuration.adoc` - Document the `endpoint` field
6. `docs/modules/reference/pages/manifest-schema.adoc` - Document the `endpoint` field in schema reference

### Scope boundaries

- This feature only affects OpenShell targets. Container and SSH targets are unaffected.
- The capture command presents extracted endpoints for user confirmation. No silent additions.
- MCP servers without endpoints produce no policy entry. Local-only servers are not affected.
- The `endpoint` field is optional. Existing manifests without it continue to work (MCP servers just have no policy entry, same as today).
