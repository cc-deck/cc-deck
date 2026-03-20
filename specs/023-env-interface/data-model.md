# Data Model: Environment Interface and CLI

**Feature**: 023-env-interface | **Date**: 2026-03-20

## Entities

### EnvironmentType

Enum defining supported environment backends.

| Value | Description |
|-------|-------------|
| `local` | Host machine, Zellij runs natively |
| `podman` | Local container via Podman |
| `k8s` | Kubernetes StatefulSet (persistent) |
| `sandbox` | Kubernetes Pod (ephemeral) |

### EnvironmentState

Lifecycle state of an environment. Transition model: permissive with guardrails (see spec clarification).

| Value | Description |
|-------|-------------|
| `running` | Environment is active and attachable |
| `stopped` | Environment is paused, resources preserved |
| `creating` | Environment is being provisioned |
| `error` | Environment is in a failed state |
| `unknown` | State cannot be determined |

**Valid user-initiated transitions**:
- `creating` -> `running` (automatic on success)
- `running` -> `stopped` (stop)
- `stopped` -> `running` (start)
- any -> deleted (delete, with `--force` if running)

**Reconciliation transitions**: any state -> `error` or `unknown` (based on observed reality).

### StorageType

Enum for storage backend types. Defined as interface types here, implemented per environment spec.

| Value | Environments | Description |
|-------|-------------|-------------|
| `host-path` | local | Host filesystem (implicit for local) |
| `named-volume` | podman | Podman named volume |
| `empty-dir` | sandbox | Kubernetes emptyDir (ephemeral) |
| `pvc` | k8s | Kubernetes PVC via StatefulSet |

### SyncStrategy

Enum for data transfer strategies. Defined as interface types here, implemented per environment spec.

| Value | Description |
|-------|-------------|
| `copy` | Tar-over-exec file transfer |
| `git-harvest` | Git via `ext::` protocol over exec |
| `remote-git` | Shared remote git repository |

### EnvironmentRecord

Persisted in `state.yaml`. One record per tracked environment.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | yes | Unique identifier, validated per FR-014 |
| `type` | EnvironmentType | yes | Environment backend type |
| `state` | EnvironmentState | yes | Current lifecycle state |
| `created_at` | RFC3339 timestamp | yes | Creation time |
| `last_attached` | RFC3339 timestamp | no | Last attachment time |
| `storage` | StorageConfig | no | Storage configuration (nil for local) |
| `sync` | SyncConfig | no | Sync configuration (nil for local) |
| `podman` | PodmanFields | no | Podman-specific fields (spec 025) |
| `k8s` | K8sFields | no | K8s-specific fields (spec 024) |
| `sandbox` | SandboxFields | no | Sandbox-specific fields (spec 026) |

### StorageConfig

Storage backend configuration, embedded in EnvironmentRecord.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `type` | StorageType | yes | Storage backend type |
| `size` | string | no | Size (e.g., "10Gi") for PVC/volume |
| `storage_class` | string | no | K8s storage class (PVC only) |
| `host_path` | string | no | Host path (bind mounts only) |

### SyncConfig

Data transfer configuration, embedded in EnvironmentRecord.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `strategy` | SyncStrategy | yes | Transfer strategy |
| `workspace` | string | no | Remote working directory (default: /workspace) |
| `excludes` | []string | no | Exclusion patterns (copy strategy) |
| `last_push` | RFC3339 timestamp | no | Last push time |
| `last_harvest` | RFC3339 timestamp | no | Last harvest time |

### StateFile

Top-level state file structure at `$XDG_STATE_HOME/cc-deck/state.yaml`.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `version` | int | yes | Schema version, starts at 1 |
| `environments` | []EnvironmentRecord | yes | All tracked environments |

### K8sFields (defined here, populated by spec 024)

| Field | Type | Description |
|-------|------|-------------|
| `namespace` | string | K8s namespace |
| `statefulset` | string | StatefulSet name |
| `profile` | string | Credential profile name |
| `kubeconfig` | string | Kubeconfig path |

### PodmanFields (defined here, populated by spec 025)

| Field | Type | Description |
|-------|------|-------------|
| `container_id` | string | Container ID |
| `container_name` | string | Container name |
| `image` | string | OCI image reference |
| `ports` | []string | Port mappings |

### SandboxFields (defined here, populated by spec 026)

| Field | Type | Description |
|-------|------|-------------|
| `namespace` | string | K8s namespace |
| `pod_name` | string | Pod name |
| `profile` | string | Credential profile name |
| `kubeconfig` | string | Kubeconfig path |
| `expires_at` | RFC3339 timestamp | Auto-deletion time |

## Relationships

```
StateFile (1) ──contains──> (0..*) EnvironmentRecord
EnvironmentRecord (1) ──has──> (0..1) StorageConfig
EnvironmentRecord (1) ──has──> (0..1) SyncConfig
EnvironmentRecord (1) ──has──> (0..1) K8sFields | PodmanFields | SandboxFields
```

## Validation Rules

| Rule | Scope | Description |
|------|-------|-------------|
| Unique name | StateFile | No two environments may share a name |
| Name format | EnvironmentRecord.name | `^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`, max 40 chars |
| Type required | EnvironmentRecord | Must be a valid EnvironmentType enum value |
| Version check | StateFile | On load, compare version to expected; run migration if mismatch |
| State validity | EnvironmentRecord | State must be a valid EnvironmentState value |

## Migration: config.yaml Sessions -> state.yaml

Existing `config.yaml` Session records map to EnvironmentRecord as follows:

| config.yaml Session field | state.yaml EnvironmentRecord field |
|--------------------------|-----------------------------------|
| `name` | `name` |
| (implicit) | `type` = `k8s` |
| `status` | `state` (map "running" -> running, else unknown) |
| `created_at` | `created_at` |
| `namespace` | `k8s.namespace` |
| `profile` | `k8s.profile` |
| `pod_name` | `k8s.statefulset` (derive from pod name) |
| `sync_dir` | (not migrated, sync config deferred to spec 024) |
