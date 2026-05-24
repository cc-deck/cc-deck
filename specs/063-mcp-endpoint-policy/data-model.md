# Data Model: MCP Endpoint Policy Integration

## Modified Entities

### MCPEntry (manifest.go)

Existing struct with one new field:

| Field | Type | YAML Tag | Description |
|-------|------|----------|-------------|
| Name | string | `name` | MCP server name (existing, required) |
| Image | string | `image` | Container image (existing) |
| Transport | string | `transport,omitempty` | Transport type: http, sse, stdio (existing) |
| Port | int | `port,omitempty` | Sidecar container port (existing) |
| Auth | MCPAuth | `auth,omitempty` | Authentication config (existing) |
| Description | string | `description,omitempty` | Human-readable description (existing) |
| **Endpoint** | **string** | **`endpoint,omitempty`** | **Network endpoint in `host:port` format (NEW)** |

The `Endpoint` field is optional. When empty, no network policy entry is generated for this MCP server. The `Port` field (existing) is the container sidecar port, not the network endpoint port; they serve different purposes.

### NetworkPolicy (policy.go)

No structural changes. MCP entries produce standard `NetworkPolicy` values keyed as `mcp_<slugified_name>` in the `networkPolicies` map.

## New Functions

### slugifyMCPName (policy.go)

Converts an MCP server name to a YAML-safe key by replacing hyphens, spaces, and non-alphanumeric characters with underscores and lowercasing.

```
Input:  "google-work"    → Output: "google_work"
Input:  "jira-redhat"    → Output: "jira_redhat"
Input:  "My MCP Server"  → Output: "my_mcp_server"
```

This is distinct from the existing `slugify()` which only replaces dots with underscores (used for domain names).

## Data Flow

```
Manifest.MCP entries
  → filter: only entries with non-empty Endpoint
  → parse: split Endpoint into host and port
  → generate: NetworkPolicy with key "mcp_<slugifyMCPName(name)>"
     - Name: from MCPEntry.Description (or MCPEntry.Name as fallback)
     - Endpoints: [{Host: host, Port: port}]
     - Binaries: copied from claude_code component's explicit binaries
  → insert into networkPolicies map

If pkg_node exists in networkPolicies AND manifest has MCP entries:
  → append claude_code binaries to pkg_node's binary list
  → deduplicate by path
```
