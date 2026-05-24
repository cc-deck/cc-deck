# Quickstart: MCP Endpoint Policy Integration

## What this feature does

Adds MCP server network endpoints to the OpenShell network policy so Claude Code can connect to MCP servers at startup without being blocked by the supervisor.

## Key files

| File | What to change |
|------|---------------|
| `cc-deck/internal/build/manifest.go` | Add `Endpoint` field to `MCPEntry` |
| `cc-deck/internal/build/policy.go` | Add `slugifyMCPName()`, `parseMCPEndpoint()`, MCP processing in `assemblePolicyCore()`, `pkg_node` augmentation |
| `cc-deck/internal/build/policy_test.go` | Tests for all new functionality |
| `cc-deck/internal/build/commands/cc-deck.capture.md` | Extend Step 9 with endpoint extraction |
| `docs/modules/reference/pages/configuration.adoc` | Document `endpoint` field |

## How to verify

```bash
make test    # All existing + new tests pass
make lint    # No linting errors
```

## Example manifest input

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

## Expected policy output (for the two entries with endpoints)

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

The `playwright` entry (no endpoint) produces no policy entry.
