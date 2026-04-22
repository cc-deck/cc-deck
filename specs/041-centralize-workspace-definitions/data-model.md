# Data Model: Centralize Workspace Definitions

**Feature**: 041-centralize-workspace-definitions
**Date**: 2026-04-22

## Entities

### WorkspaceDefinition (modified)

**Location**: `internal/ws/definition.go`
**Storage**: `~/.config/cc-deck/workspaces.yaml` (central store)

The existing struct is reused. The `project-dir` field's semantics change from "compose project directory" to "project association for all workspace types." No new fields are added.

| Field | Type | YAML Key | Change |
|-------|------|----------|--------|
| Name | string | `name` | Unchanged |
| Type | WorkspaceType | `type` | Unchanged |
| Image | string | `image` | Unchanged |
| Auth | string | `auth` | Unchanged |
| Storage | *StorageConfig | `storage` | Unchanged |
| Ports | []string | `ports` | Unchanged |
| Credentials | []string | `credentials` | Unchanged |
| Mounts | []string | `mounts` | Unchanged |
| AllowedDomains | []string | `allowed-domains` | Unchanged |
| **ProjectDir** | string | `project-dir` | **Semantic change**: now used for all types, not just compose |
| Env | map[string]string | `env` | Unchanged |
| Host | string | `host` | Unchanged |
| Port | int | `port` | Unchanged |
| IdentityFile | string | `identity-file` | Unchanged |
| JumpHost | string | `jump-host` | Unchanged |
| SSHConfig | string | `ssh-config` | Unchanged |
| Workspace | string | `workspace` | Unchanged |
| Repos | []RepoEntry | `repos` | Unchanged |
| RemoteBG | string | `remote-bg` | Unchanged |
| Namespace | string | `namespace` | Unchanged |
| Kubeconfig | string | `kubeconfig` | Unchanged |
| K8sContext | string | `context` | Unchanged |
| StorageSize | string | `storage-size` | Unchanged |
| StorageClass | string | `storage-class` | Unchanged |

**Uniqueness**: Name must be unique within the central store for a given type. Same name + different type triggers auto-suffix (e.g., `foo` + ssh = `foo-ssh`).

### WorkspaceTemplate (new)

**Location**: `internal/ws/template.go` (new file)
**Storage**: `.cc-deck/workspace-template.yaml` (project-local, git-committed)
**Lifecycle**: Read-only input to `ws new`. Never persisted in central store.

| Field | Type | YAML Key | Description |
|-------|------|----------|-------------|
| Name | string | `name` | Required. Default workspace name. |
| Variants | map[string]TemplateVariant | `variants` | Type-keyed variant definitions |

### TemplateVariant (new)

**Location**: `internal/ws/template.go`

A variant body uses the same YAML fields as `WorkspaceDefinition` (minus `name` and `type`, which come from the template structure). Fields may contain `{{placeholder}}` or `{{placeholder:default}}` strings.

| Field | Type | Description |
|-------|------|-------------|
| (all WorkspaceDefinition fields except Name, Type) | various | Same schema as WorkspaceDefinition |

**Validation rules**:
- `name` is required; error if missing.
- Each variant key must be a valid `WorkspaceType` (`ssh`, `container`, `compose`, `k8s-deploy`).
- Placeholders are resolved before storing the definition centrally.

### Placeholder (new, internal)

**Location**: `internal/ws/template.go`

| Field | Type | Description |
|-------|------|-------------|
| Name | string | Placeholder identifier (e.g., `ssh_user`) |
| Default | string | Optional default value (empty if none) |

**Extraction**: Regex `\{\{(\w+)(?::([^}]*))?\}\}` applied to all string fields in a variant.

### DefinitionStore (modified)

**Location**: `internal/ws/definition.go`

Two new methods added:

| Method | Signature | Description |
|--------|-----------|-------------|
| FindByProjectDir | `(path string) ([]*WorkspaceDefinition, error)` | Returns all definitions whose `ProjectDir` is an ancestor of (or equal to) the given path |
| AddWithCollisionHandling | `(def *WorkspaceDefinition) (string, error)` | Adds definition; on same-name + different-type, auto-suffixes name. Returns final name used. Errors on same-name + same-type. |

### DefinitionFile (unchanged)

| Field | Type | YAML Key |
|-------|------|----------|
| Version | int | `version` |
| Workspaces | []WorkspaceDefinition | `workspaces` |

### WorkspaceInstance (unchanged)

Runtime state in `state.yaml`. No changes. Already tracks `LastAttached` which is used for default resolution recency.

### StateFile (modified)

| Field | Type | YAML Key | Change |
|-------|------|----------|--------|
| Version | int | `version` | Unchanged |
| Instances | []WorkspaceInstance | `instances` | Unchanged |
| ~~Projects~~ | ~~[]ProjectEntry~~ | ~~`projects`~~ | **Removed** (FR-012) |

## Removed Entities

| Entity | File | Reason |
|--------|------|--------|
| ProjectEntry | `internal/ws/types.go` | FR-016: project association moves to `WorkspaceDefinition.ProjectDir` |
| ProjectStatusFile | `internal/ws/types.go` | FR-017: runtime state tracked in WorkspaceInstance; overrides unnecessary |
| ProjectStatusStore | `internal/ws/project_status.go` | FR-017: entire file deleted |

## Removed Functions

| Function | File | Reason |
|----------|------|--------|
| LoadProjectDefinition | `internal/ws/definition.go` | FR-015: no project-local definitions |
| SaveProjectDefinition | `internal/ws/definition.go` | FR-015: no project-local definitions |
| AllProjectWorkspaceNames | `internal/ws/state.go` | FR-015: collision handled by DefinitionStore |
| ListProjects | `internal/ws/state.go` | FR-015: no project registry |
| RegisterProject | `internal/ws/state.go` | FR-015: no project registry |
| UnregisterProject | `internal/ws/state.go` | Depends on ProjectEntry |
| PruneStaleProjects | `internal/ws/state.go` | Depends on ProjectEntry |
| PruneStaleProjectsVerbose | `internal/ws/state.go` | Depends on ProjectEntry |

## Relationships

```
WorkspaceTemplate (project-local, read-only)
    │
    │ ws new (resolves placeholders, selects variant)
    ▼
WorkspaceDefinition (central store)
    │ project-dir ──► project directory (informational)
    │
    │ ws new (creates)
    ▼
WorkspaceInstance (state.yaml, runtime)
    │ LastAttached ──► used for default resolution
```

## State Transitions

### Template → Definition (during `ws new`)

1. Load template from `.cc-deck/workspace-template.yaml`
2. Select variant by `--type` flag (or auto-select if single variant)
3. Extract placeholders from all string fields
4. Prompt user for each placeholder value (show defaults)
5. Substitute placeholders in variant fields
6. Set `ProjectDir` to canonical cwd
7. Apply CLI flag overrides (flags take precedence over template values)
8. Call `DefinitionStore.AddWithCollisionHandling()` to store

### Default Workspace Resolution (during `ws attach` etc.)

1. Get cwd
2. Call `DefinitionStore.FindByProjectDir(cwd)` for ancestor-matching definitions
3. If exactly one match: use it
4. If multiple matches: select by most recent `WorkspaceInstance.LastAttached`
5. If no matches: fall back to global pool (all definitions)
6. Apply same single/recency/error logic on global pool
7. Print `Using workspace "X"` to stderr
