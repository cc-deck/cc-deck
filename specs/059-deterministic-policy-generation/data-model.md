# Data Model: Deterministic Policy Generation

## Entities

### PolicyComponent

A self-contained policy fragment loaded from a YAML file.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `key` | string | yes | Output section name in generated policy (e.g., `claude_code`) |
| `name` | string | yes | Human-readable display name (e.g., `Claude Code`) |
| `match` | MatchCondition | yes | Conditions that determine if this component is included |
| `endpoints` | []PolicyEndpoint | yes | Network endpoints to allow |
| `binaries` | []PolicyBinary | no | Binary paths restricted to these endpoints |

**Identity**: Filename stem (e.g., `claude-code.yaml` identifies as `claude-code`).
**Uniqueness**: One component per filename stem per tier. Higher-precedence tier replaces lower entirely.

### MatchCondition

Conditions evaluated against the manifest to determine component inclusion.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `always` | bool | no | If true, component is always included |
| `tools` | []string | no | Include if ANY listed tool appears in the manifest |
| `credentials` | []string | no | Include if ANY listed credential type appears in the manifest |
| `features` | []string | no | Include if ANY listed feature flag appears in the manifest |

**Evaluation**: Short-circuit OR within each field. OR across fields (any field match includes the component). If `always: true`, other fields are ignored.

### PolicyEndpoint (existing, unchanged)

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `host` | string | yes | Hostname or IP |
| `port` | int | yes | Port number |
| `protocol` | string | no | Protocol type (e.g., `rest`) |
| `access` | string | no | Access level (required when protocol is `rest`) |
| `rules` | []PolicyRule | no | L7 rules (alternative to `access` for `rest` protocol) |

### PolicyBinary (existing, unchanged)

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `path` | string | yes | Binary path or glob pattern |

### CatalogIndex

The `catalog.yaml` file in the remote catalog repo.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `version` | int | yes | Catalog format version (currently 1) |
| `components` | []string | yes | List of component filenames available for download |
| `base_url` | string | no | Base URL for component downloads (defaults to repo raw URL) |

## Relationships

```text
Manifest (build.yaml)
  ├── tools[]          ──matches──▶  PolicyComponent.match.tools
  ├── credentials[]    ──matches──▶  PolicyComponent.match.credentials
  └── features[]       ──matches──▶  PolicyComponent.match.features

PolicyComponent
  ├── key             ──becomes──▶  NetworkPolicy map key in PolicyFile
  ├── name            ──becomes──▶  NetworkPolicy.Name
  ├── endpoints[]     ──becomes──▶  NetworkPolicy.Endpoints
  └── binaries[]      ──becomes──▶  NetworkPolicy.Binaries

PolicyFile (output)
  └── network_policies: map[key]NetworkPolicy
       (alphabetically ordered by key)
```

## Component Resolution Order

```text
1. Embedded (cc-deck/internal/build/policies/*.yaml)  ──lowest──▶
2. Cached catalog (.cc-deck/setup/openshell/components/*.yaml)  ──▶
3. User-local (.cc-deck/setup/openshell/policies/*.yaml)  ──highest──▶
```

Same filename stem at a higher tier replaces the lower tier's version entirely.

## State Transitions

Components are stateless. The assembly pipeline is a pure function:

```text
(manifest, components[]) ──▶ matched_components[] ──▶ sorted_components[] ──▶ PolicyFile
```

No lifecycle management needed. Components are loaded fresh on each `build refresh`.
