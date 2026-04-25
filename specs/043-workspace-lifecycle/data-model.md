# Data Model: Workspace Lifecycle Redesign

**Date**: 2026-04-25
**Branch**: 043-workspace-lifecycle

## Entities

### WorkspaceInstance (modified)

The runtime state record for a workspace. Stored in `~/.local/state/cc-deck/state.yaml`.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| name | string | yes | Unique workspace identifier |
| type | WorkspaceType | yes | local, container, compose, ssh, k8s-deploy |
| infra_state | *string | no | Infrastructure state: "running", "stopped", "error". Null for non-InfraManager types (local, ssh). |
| session_state | string | yes | Zellij session state: "none", "exists" |
| created_at | time.Time | yes | Creation timestamp |
| last_attached | *time.Time | no | Last attach timestamp |
| container | ContainerFields | no | Container-specific fields (unchanged) |
| compose | ComposeFields | no | Compose-specific fields (unchanged) |
| k8s | K8sFields | no | K8s-specific fields (unchanged) |
| ssh | SSHFields | no | SSH-specific fields (unchanged) |

**Removed field**: `state` (single WorkspaceState, replaced by infra_state + session_state)

### State File (modified)

```yaml
version: 3  # bumped from 2
instances:
  - name: mydev
    type: local
    session_state: none
    created_at: 2026-04-25T12:00:00Z

  - name: mycontainer
    type: container
    infra_state: running
    session_state: exists
    created_at: 2026-04-25T12:00:00Z
    last_attached: 2026-04-25T14:30:00Z
    container:
      container_id: abc123
      container_name: cc-deck-mycontainer
      image: quay.io/cc-deck/cc-deck-demo:latest
```

### State Constants

**InfraState values** (only for InfraManager types):
- `running` - infrastructure is up and accessible
- `stopped` - infrastructure is down (can be started)
- `error` - infrastructure is in an error state

**SessionState values** (all types):
- `none` - no Zellij session exists
- `exists` - a Zellij session exists (attached or detached, detected at query time)

**Removed constants**: `available`, `creating`, `unknown` (no longer needed with two-dimensional model)

## Interfaces

### Workspace (modified)

Universal interface for all workspace types.

```
Attach(ctx) error
KillSession(ctx) error       # NEW
Delete(ctx, force) error
Status(ctx) (*WorkspaceStatus, error)
Create(ctx, opts) error
Exec(ctx, cmd) error
ExecOutput(ctx, cmd) (string, error)
Push(ctx, opts) error
Pull(ctx, opts) error
Harvest(ctx, opts) error
PipeChannel(ctx) (PipeChannel, error)
DataChannel(ctx) (DataChannel, error)
GitChannel(ctx) (GitChannel, error)
Type() WorkspaceType
Name() string
```

**Removed**: `Start(ctx) error`, `Stop(ctx) error`

### InfraManager (new)

Optional capability interface for workspace types that manage compute infrastructure.

```
Start(ctx) error
Stop(ctx) error
```

**Implemented by**: ContainerWorkspace, ComposeWorkspace, K8sDeployWorkspace
**Not implemented by**: LocalWorkspace, SSHWorkspace

### WorkspaceStatus (modified)

```
InfraState  *string      # nil for non-InfraManager types
SessionState string      # "none" or "exists"
Since       *time.Time
Message     string
Sessions    []SessionInfo
```

## State Transitions

### Non-InfraManager types (local, ssh)

```
[created] --attach--> session_state: exists
[exists]  --detach--> session_state: exists  (session preserved)
[exists]  --kill-session--> session_state: none
[none]    --attach--> session_state: exists  (fresh session with layout)
[any]     --delete--> [removed]
```

### InfraManager types (container, compose, k8s-deploy)

```
[created] --> infra_state: running, session_state: none

infra_state transitions:
  running --stop--> stopped  (also kills session)
  stopped --start--> running
  stopped --attach--> running  (lazy start)
  running --delete--> [removed]
  stopped --delete--> [removed]

session_state transitions (requires infra_state: running):
  none --attach--> exists  (creates session with layout)
  exists --detach--> exists  (session preserved)
  exists --kill-session--> none
  exists --stop--> none  (implicit, stop kills session first)
```

## Migration

**Trigger**: StateFile.Version < 3 on Load()

**Rules**:
1. For each instance, map old `state` to new fields:
   - `state: "running"` + type is local/ssh -> `session_state: "exists"`, `infra_state: nil`
   - `state: "running"` + type is container/compose/k8s -> `infra_state: "running"`, `session_state: "none"`
   - `state: "stopped"` + type is local/ssh -> `session_state: "none"`, `infra_state: nil`
   - `state: "stopped"` + type is container/compose/k8s -> `infra_state: "stopped"`, `session_state: "none"`
   - `state: "available"` -> `infra_state: "running"`, `session_state: "none"`
   - `state: "error"` -> `infra_state: "error"`, `session_state: "none"`
   - `state: "unknown"` -> `infra_state: nil`, `session_state: "none"`
   - `state: "creating"` -> treat as `running` (creation is complete at this point)
2. Remove old `state` field
3. Set Version to 3
4. Save atomically
