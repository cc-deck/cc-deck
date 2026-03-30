# Data Model: Session TUI

**Feature**: 031-session-tui
**Date**: 2026-03-30

## Entities

### envRow (TUI display model)

Flattened representation of an environment for table rendering. Built from existing `EnvironmentRecord`, `EnvironmentInstance`, and `EnvironmentDefinition` types.

| Field | Type | Source | Notes |
|-------|------|--------|-------|
| name | string | record.Name / instance.Name | Display name, also used for attach/delete |
| envType | EnvironmentType | record.Type / instance.Type | local, container, compose |
| state | EnvironmentState | record.State / instance.State | running, stopped, creating, error, unknown |
| sessionCount | int | len(sessions) | Total sessions in this environment |
| attentionCount | int | computed | Sessions with Activity == Waiting |
| storageName | string | computed | "host", "named-volume", "host-path", etc. |
| lastAttached | *time.Time | record.LastAttached | nil if never attached |
| tags | []string | definition.Tags | User-defined labels (future: from definition) |
| sessions | []sessionRow | from plugin cache / Status() | Only populated for detail view |

### sessionRow (TUI display model for detail view)

Flattened session data for the detail view table. Built from Zellij plugin `sessions.json` data.

| Field | Type | Source | Notes |
|-------|------|--------|-------|
| name | string | Session.display_name | User-visible name |
| activity | string | Session.activity | "Working", "Idle", "Permission", "Notification", "Done", "AgentDone", "Init" |
| branch | string | Session.git_branch | Git branch, may be empty |
| lastEvent | time.Time | Session.last_event_ts | Unix timestamp, converted |
| paneID | uint32 | Session.pane_id | For Zellij tab focusing |
| tabIndex | *int | Session.tab_index | For attach-to-specific-session |
| paused | bool | Session.paused | Whether updates are paused |

### pluginSessionData (Go representation of Zellij plugin JSON)

Deserialized from `~/.config/zellij/plugins/cc_deck.wasm/cache/sessions.json`.

```go
type PluginSession struct {
    PaneID           uint32  `json:"pane_id"`
    SessionID        string  `json:"session_id"`
    DisplayName      string  `json:"display_name"`
    Activity         any     `json:"activity"` // string or {"Waiting":"Permission"}
    TabIndex         *int    `json:"tab_index"`
    TabName          *string `json:"tab_name"`
    WorkingDir       *string `json:"working_dir"`
    GitBranch        *string `json:"git_branch"`
    LastEventTS      uint64  `json:"last_event_ts"`
    ManuallyRenamed  bool    `json:"manually_renamed"`
    Paused           bool    `json:"paused"`
    MetaTS           uint64  `json:"meta_ts"`
    DoneAttended     bool    `json:"done_attended"`
    PendingTabRename bool    `json:"pending_tab_rename"`
}
```

**Activity field deserialization**: The Rust `Activity` enum serializes as:
- Simple variants: `"Init"`, `"Working"`, `"Idle"`, `"Done"`, `"AgentDone"`
- Enum with data: `{"Waiting":"Permission"}`, `{"Waiting":"Notification"}`

The Go parser must handle both string and object forms.

### viewType (view state machine)

```
viewList ──Enter──> (suspend/attach)
   │
   ├──s/Tab──> viewDetail ──Esc──> viewList
   │               │
   │               └──Σ──> viewReport ──Esc──> viewDetail
   │
   ├──n──> viewCreate ──Esc──> viewList
   │                   └──Enter──> (create + optionally attach)
   │
   └──?──> viewHelp (overlay) ──Esc/q/?──> (previous view)
```

States: `viewList`, `viewDetail`, `viewCreate`, `viewReport`, `viewHelp`

Confirmation dialogs are overlays on any view, not separate view states.

## Existing Types Reused (no changes needed)

From `internal/env`:
- `EnvironmentType` (local, container, compose, k8s-deploy, k8s-sandbox)
- `EnvironmentState` (running, stopped, creating, error, unknown)
- `EnvironmentRecord` (v1 state for local envs)
- `EnvironmentInstance` (v2 state for container/compose envs)
- `EnvironmentDefinition` (from definition store)
- `Environment` interface (for lifecycle operations)
- `FileStateStore`, `DefinitionStore` (persistence)
- `CreateOpts`, `SyncOpts`, `HarvestOpts` (operation parameters)
- `SessionInfo` (from Status() response)

## State Transitions

### Environment State (existing, no changes)

```
(not created) ──Create──> running
running ──Stop──> stopped
stopped ──Start──> running
running/stopped ──Delete──> (removed)
* ──error──> error
```

### TUI View State

```
startup ──> viewList (always starts here)
viewList ──s/Tab──> viewDetail
viewDetail ──Esc──> viewList
viewList/viewDetail ──n──> viewCreate
viewCreate ──Esc──> viewList
viewCreate ──confirm──> viewList (or suspend/attach)
viewDetail ──Σ──> viewReport
viewReport ──Esc──> viewDetail
any ──?──> viewHelp (overlay, returns to previous)
any ──Enter on env──> (suspend) ──(resume)──> viewList
```

## Data Flow

### Polling Cycle (P1 direct polling)

```
Timer tick (2s/5s)
  │
  ├── ReconcileLocalEnvs(store)     // updates state.yaml from zellij list-sessions
  ├── ReconcileContainerEnvs(store) // updates state.yaml from podman inspect
  ├── ReconcileComposeEnvs(store)   // updates state.yaml from podman-compose
  │
  ├── store.List() + store.ListInstances() + defs.List()
  │     └── merge into []envRow
  │
  └── For selected local env (detail view only):
        read sessions.json from host filesystem
        └── parse into []sessionRow
```
