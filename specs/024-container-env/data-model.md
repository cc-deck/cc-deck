# Data Model: Container Environment

**Feature**: 024-container-env | **Date**: 2026-03-20

## Entities

### EnvironmentDefinition

Declarative, user-editable description of an environment. Stored in `environments.yaml`.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| name | string | yes | Unique environment name (validated: lowercase, digits, hyphens, max 40 chars) |
| type | EnvironmentType | yes | One of: `local`, `container`, `compose`, `k8s-deploy`, `k8s-sandbox` |
| image | string | no | OCI image reference (container/compose types only) |
| storage | StorageConfig | no | Storage backend configuration |
| ports | []string | no | Port mappings in `host:container` format |
| credentials | []string | no | Credential key names to inject (resolved from host env or explicit values) |

### EnvironmentInstance (state.yaml v2)

Machine-managed runtime state for a created environment.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| name | string | yes | Matches definition name (join key) |
| state | EnvironmentState | yes | Current state: running, stopped, creating, error, unknown |
| created_at | time.Time | yes | When the instance was created |
| last_attached | *time.Time | no | When the user last attached |
| container | *ContainerFields | no | Container-specific runtime state (container/compose types) |

### ContainerFields (replaces PodmanFields)

Container-specific runtime state. Used by both `container` and future `compose` types.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| container_id | string | no | Full podman container ID |
| container_name | string | no | Container name (`cc-deck-<env-name>`) |
| image | string | no | Image used at creation time |
| ports | []string | no | Active port mappings |

### StorageConfig (existing, unchanged)

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| type | StorageType | yes | `named-volume` (default) or `host-path` |
| host_path | string | no | Absolute path for bind mount (required when type is host-path) |
| size | string | no | Not used for container type (K8s PVC only) |
| storage_class | string | no | Not used for container type (K8s PVC only) |

## File Schemas

### environments.yaml

```yaml
version: 1
environments:
  - name: my-project
    type: container
    image: quay.io/cc-deck/cc-deck-demo:latest
    storage:
      type: named-volume
    ports: []
    credentials:
      - ANTHROPIC_API_KEY

  - name: local-dev
    type: local
```

### state.yaml (v2)

```yaml
version: 2
instances:
  - name: my-project
    state: running
    created_at: 2026-03-20T10:00:00Z
    last_attached: 2026-03-20T15:30:00Z
    container:
      container_id: abc123def456
      container_name: cc-deck-my-project
      image: quay.io/cc-deck/cc-deck-demo:latest
      ports:
        - "8082:8082"

  - name: local-dev
    state: running
    created_at: 2026-03-20T09:00:00Z
```

## State Transitions

```
                 create
  (none) ──────────────────> running
                                │
                     stop       │       start
              ┌─────────────── │ ◄──────────────┐
              │                │                │
              v                │                │
           stopped ────────────┘           (auto-start
              │                             on attach)
              │  delete
              └──────────────> (none)
                                 ▲
           running ──────────────┘
                     delete --force

           running/stopped ───> error
                     (container deleted externally,
                      detected by reconciliation)

           error ──────────────> (none)
                     delete
```

## Naming Conventions

| Resource | Pattern | Example |
|----------|---------|---------|
| Container | `cc-deck-<env-name>` | `cc-deck-my-project` |
| Volume | `cc-deck-<env-name>-data` | `cc-deck-my-project-data` |
| Secret | `cc-deck-<env-name>-<key>` | `cc-deck-my-project-anthropic-api-key` |
| Zellij session (inside container) | `cc-deck` | `cc-deck` (always, per container isolation) |

## Relationships

```
EnvironmentDefinition (environments.yaml)
    │
    │  joined by name
    │
    ▼
EnvironmentInstance (state.yaml)
    │
    │  has-one (optional)
    │
    ▼
ContainerFields
    │
    │  references
    │
    ├──> podman container (cc-deck-<name>)
    ├──> podman volume (cc-deck-<name>-data)
    └──> podman secrets (cc-deck-<name>-<key>)
```

## Type Changes from Spec 023

| Old (spec 023) | New (spec 024) | Reason |
|----------------|----------------|--------|
| `EnvironmentTypePodman` | `EnvironmentTypeContainer` | Focuses on what it is, not the tool |
| `PodmanFields` | `ContainerFields` | Shared by container and compose types |
| `EnvironmentRecord` (monolithic) | `EnvironmentDefinition` + `EnvironmentInstance` | Definition/state separation |
| `StateFile.Version = 1` | `StateFile.Version = 2` | Slimmed-down instance records |
| `StateFile.Environments` | `StateFile.Instances` | Renamed to reflect role |
