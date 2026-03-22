# Data Model: Project-Local Environment Configuration

**Feature**: 026-project-local-config | **Date**: 2026-03-22

## Entities

### ProjectEntry (NEW)

Global registry entry stored in `$XDG_STATE_HOME/cc-deck/state.yaml` under the `projects` section.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| path | string | yes | Canonical (symlink-resolved) absolute path to the project directory |
| last_seen | time.Time | yes | Timestamp of last interaction with this project |

**Validation rules**:
- `path` must be an absolute path
- `path` must be symlink-resolved before storage (FR-021)
- Duplicate paths are rejected

**State transitions**: None (static registry entry). Staleness detected by `os.Stat(path)`.

### EnvironmentDefinition (MODIFIED)

Existing entity in `definition.go`. Extended with new fields for project-local use.

| Field | Type | Required | New? | Description |
|-------|------|----------|------|-------------|
| version | int | yes | YES | Schema version, starts at 1 (FR-029) |
| name | string | yes | no | Environment name (defaults to directory basename) |
| type | EnvironmentType | yes | no | Environment type (local, container, compose, etc.) |
| image | string | no | no | OCI image reference |
| auth | string | no | no | Auth mode (auto, none, api, vertex, bedrock) |
| storage | *StorageConfig | no | no | Storage backend configuration |
| ports | []string | no | no | Port mappings (host:container) |
| credentials | []string | no | no | Credential variable names (values resolved at runtime) |
| mounts | []string | no | no | Bind mounts as src:dst[:ro] |
| allowed-domains | []string | no | no | Domain groups for proxy sidecar |
| project-dir | string | no | no | Project directory override |
| env | map[string]string | no | YES | Arbitrary environment variables (FR-028) |

**Validation rules**:
- `name` follows existing `ValidateEnvName()` rules
- `type` must be a recognized EnvironmentType
- `version` must be >= 1

**Storage locations**:
- Project-local: `.cc-deck/environment.yaml` (single definition, no wrapper)
- Global: `$XDG_CONFIG_HOME/cc-deck/environments.yaml` (list of definitions)

**Schema difference**: Project-local files contain a bare `EnvironmentDefinition`. Global files contain a `DefinitionFile` wrapper with `version` and `environments` list.

### ProjectStatusFile (NEW)

Per-project runtime state stored at `.cc-deck/status.yaml` (gitignored).

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| variant | string | no | Variant identifier for multi-instance isolation (FR-010) |
| state | EnvironmentState | yes | Current lifecycle state (running, stopped, creating, error) |
| container_name | string | yes | Actual container name (cc-deck-\<name\>[-\<variant\>]) |
| created_at | time.Time | yes | Timestamp of environment creation |
| last_attached | *time.Time | no | Timestamp of last attach |
| overrides | map[string]string | no | CLI flag overrides not persisted to environment.yaml (FR-019) |

**Validation rules**:
- `container_name` must follow naming convention `cc-deck-<name>[-<variant>]`
- `state` must be a recognized EnvironmentState

**State transitions**:
```
(none) --[create]--> creating --[success]--> stopped
                                --[failure]--> (file removed, cleanup)
stopped --[start]--> running
running --[stop]--> stopped
running --[attach]--> running (updates last_attached)
* --[delete]--> (file removed)
```

### StateFile (MODIFIED)

Existing entity in `types.go`. Extended with projects section.

| Field | Type | Required | New? | Description |
|-------|------|----------|------|-------------|
| version | int | yes | no | File format version (remains 2) |
| environments | []EnvironmentRecord | no | no | v1 environment records (local) |
| instances | []EnvironmentInstance | no | no | v2 environment instances (container/compose) |
| projects | []ProjectEntry | no | YES | Global project registry (FR-006) |

## Relationships

```
StateFile (global, one per machine)
├── instances[]          # Runtime state for non-project environments
├── environments[]       # Legacy v1 records (local envs)
└── projects[]           # Registry of project directories
     └── path ──────────► Project Directory
                           └── .cc-deck/
                               ├── environment.yaml  # EnvironmentDefinition
                               ├── status.yaml       # ProjectStatusFile
                               ├── .gitignore        # Ignore boundary
                               ├── image/            # Build artifacts
                               └── run/              # Generated artifacts
```

**Key relationships**:
- `ProjectEntry.path` points to a directory that MAY contain `.cc-deck/environment.yaml`
- `ProjectStatusFile` is the project-local equivalent of `EnvironmentInstance` for project-scoped environments
- `EnvironmentDefinition` in `.cc-deck/environment.yaml` shadows any global definition with the same name (FR-026)
- When a project-local environment exists, its state is in `status.yaml`, NOT in the global `StateFile.instances`

## Precedence Chain (FR-003)

```
CLI flags (highest)
  └──► .cc-deck/environment.yaml (project-local)
         └──► $XDG_CONFIG_HOME/cc-deck/config.yaml defaults
                └──► Hardcoded defaults (lowest)
```

Fields resolved independently: each field uses the highest-priority source that provides a value.

## File Ownership

| File | Created by | Modified by | Committed |
|------|-----------|-------------|-----------|
| `.cc-deck/environment.yaml` | `env init` or `env create` (FR-025) | User (manual edit) | Yes |
| `.cc-deck/.gitignore` | `env init`, `env create`, or self-healing (FR-030) | Never (idempotent) | Yes |
| `.cc-deck/status.yaml` | `env create` | `env start/stop/attach/delete` | No |
| `.cc-deck/run/*` | `env create` | `env create` (regenerated) | No |
| `.cc-deck/image/*` | `image init/extract` | User + `image extract` | Yes |
| `state.yaml` (projects section) | `env create`, walk discovery (FR-007) | `env delete/prune` | No |
