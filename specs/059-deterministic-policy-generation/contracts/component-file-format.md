# Contract: Policy Component File Format

**Version**: 1.0
**Consumers**: `cc-deck build refresh`, catalog repo maintainers, end users (custom components)

## File Location

Components are loaded from three tiers in precedence order:

| Tier | Path | Precedence |
|------|------|------------|
| Embedded | `cc-deck/internal/build/policies/*.yaml` | Lowest |
| Cached catalog | `.cc-deck/setup/openshell/components/*.yaml` | Middle |
| User-local | `.cc-deck/setup/openshell/policies/*.yaml` | Highest |

## Identity

A component's identity is its **filename stem** (filename without `.yaml` extension). When multiple tiers contain a file with the same stem, the highest-precedence tier wins entirely (no merging).

## Schema

```yaml
# Required: output section name in generated policy
key: claude_code

# Required: human-readable display name
name: Claude Code

# Required: match conditions (at least one field must be set)
match:
  # Include unconditionally (overrides other conditions)
  always: true
  # Include if ANY listed tool appears in manifest tools[]
  tools:
    - cargo
    - rust
  # Include if ANY listed credential type appears in manifest credentials[]
  credentials:
    - claude
    - claude-vertex
  # Include if ANY listed feature flag appears in manifest features[]
  features:
    - mcp

# Required: network endpoints to allow
endpoints:
  - host: api.anthropic.com
    port: 443
    protocol: rest
    access: full
  - host: downloads.claude.ai
    port: 443

# Optional: restrict these endpoints to specific binaries
binaries:
  - path: /usr/local/bin/claude
  - path: /sandbox/.local/share/claude/**
```

## Field Constraints

### `key` (string, required)
- Used as the map key in `network_policies` in the output policy
- Must be unique across all matched components (later precedence tier replaces earlier)
- Convention: `snake_case` (e.g., `claude_code`, `rust_crates`)

### `name` (string, required)
- Human-readable label for the policy section
- No format constraints

### `match` (object, required)
- At least one sub-field must be present
- Evaluation: OR within each field, OR across fields
- `always: true` short-circuits all other conditions

### `match.tools` ([]string, optional)
- Matched against `manifest.tools[].name` (case-insensitive substring match)
- Example: `[cargo, rust]` matches manifest tool `{name: cargo}`

### `match.credentials` ([]string, optional)
- Matched against `manifest.credentials[].type` (exact match)
- Example: `[claude, claude-vertex]` matches credential `{type: claude}`

### `match.features` ([]string, optional)
- Matched against `manifest.features[]` (exact match)
- Example: `[mcp]` matches manifest `features: [mcp]`

### `endpoints` ([]object, required)
- Each entry maps to a `PolicyEndpoint` in the output
- `host` (string, required): hostname
- `port` (int, required): port number
- `protocol` (string, optional): e.g., `rest`
- `access` (string, optional): required when `protocol: rest` (OpenShell 0.0.46)
- `rules` ([]object, optional): alternative to `access` for `rest` protocol

### `binaries` ([]object, optional)
- Each entry maps to a `PolicyBinary` in the output
- `path` (string, required): binary path or glob pattern
- If omitted, no binary restriction applies to these endpoints

## Validation Rules

1. `key` must be a non-empty string
2. `name` must be a non-empty string
3. `match` must have at least one field set
4. `endpoints` must contain at least one entry
5. Each endpoint must have `host` and `port`
6. Endpoints with `protocol: rest` must have `access` or `rules` (FR-010)
7. Deprecated fields (`tls: terminate`, `enforcement: enforce`) must not appear (FR-010)

## Output Mapping

A matched component produces one entry in `PolicyFile.network_policies`:

```yaml
# Component file: claude-code.yaml
key: claude_code
name: Claude Code
endpoints: [...]
binaries: [...]

# Becomes in openshell/policy.yaml:
network_policies:
  claude_code:
    name: Claude Code
    endpoints: [...]
    binaries: [...]
```

## Determinism Guarantee

The output `network_policies` map is ordered alphabetically by `key`. Within each component, endpoints and binaries preserve their declaration order from the component file. This produces byte-identical output for the same inputs.

## Error Handling

- Invalid YAML: skip component, log filename and parse error, continue with remaining components
- Missing required field: skip component, log validation error
- Duplicate `key` across matched components at the same tier: last file in alphabetical filename order wins
