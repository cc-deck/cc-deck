# Data Model: Network Policy Generalization

## Entities

### Agent (extended)

The existing `Agent` interface gains one new method:

| Method | Returns | Description |
|--------|---------|-------------|
| `RequiredDomainGroups()` | `[]string` | Builtin domain group names this agent needs for API communication |

This method returns group name references only (e.g., `"anthropic"`, `"openai"`). The actual domain endpoints are resolved from the builtin groups map or user-defined groups.

### Manifest (extended)

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `Agents` | `[]string` | `nil` | Agent names to include in the build. When nil/empty, defaults to `["claude"]` at assembly time. |

### MatchCondition (extended)

| Field | Type | Description |
|-------|------|-------------|
| `Agents` | `[]string` | Agent names that trigger component inclusion. Matched against `manifest.Agents`. |

Evaluation: OR across all fields. A component is included if `Always` is true, OR any `Tools` entry matches, OR any `Credentials` entry matches, OR any `Agents` entry matches the manifest's agent list.

### DomainGroup (unchanged)

| Field | Type | Description |
|-------|------|-------------|
| `Name` | `string` | Group identifier (e.g., "anthropic", "github") |
| `Source` | `Source` | Origin: SourceBuiltin, SourceUser, or SourceCatalog |
| `Domains` | `[]string` | Domain patterns. Wildcard prefix "." matches all subdomains. |

New builtin group added:

| Group | Domains | Owner |
|-------|---------|-------|
| `openai` | `api.openai.com`, `.openai.com`, `.oaiusercontent.com` | OpenCodeAgent |

### PolicyComponent (unchanged struct, new YAML values)

The `claude-code.yaml` component changes its match condition:

```yaml
# Before
match:
  always: true

# After
match:
  agents:
    - claude
```

## Relationships

```text
Manifest.Agents ──references──▶ Agent.Name()
     │
     ▼
MatchCondition.Agents ──compared against──▶ Manifest.Agents
     │
     ▼
PolicyComponent (matched) ──provides──▶ Binaries for MCP entries
     │
     ▼
Agent.RequiredDomainGroups() ──resolves from──▶ builtinGroups map
```

## State Transitions

No state machines. Policy assembly is a pure function: manifest in, policy out.

## Validation Rules

- `Manifest.Agents` entries must correspond to registered agent names. Unknown agent names produce a warning but do not fail the build.
- `MatchCondition.Agents` entries are compared case-sensitively against `Manifest.Agents`.
- `Agent.RequiredDomainGroups()` must return only group names that exist in `builtinGroups` or user-defined groups. Missing groups produce a warning.
