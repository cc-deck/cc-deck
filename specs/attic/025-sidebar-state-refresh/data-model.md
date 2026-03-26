# Data Model: 025-sidebar-state-refresh

**Date**: 2026-03-21

## Modified Entities

### PluginState (existing, `state.rs`)

One new field added:

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `startup_grace_until` | `Option<u64>` | `None` | Millisecond timestamp until which `remove_dead_sessions()` is skipped. Set at permission grant, cleared implicitly when current time exceeds the value. |

### Session / Session Cache (unchanged)

No changes to `Session` struct or `/cache/sessions.json` format. The cache is read and written exactly as before.

## State Transitions

```text
Plugin Load
  │
  ▼
Permission Grant
  ├─ restore cached sessions
  ├─ set startup_grace_until = now_ms + 3000
  └─ request sync from other instances
  │
  ▼
PaneUpdate events (grace period active)
  ├─ rebuild_pane_map() ← runs normally
  ├─ remove_dead_sessions() ← SKIPPED
  └─ preserve_cursor() ← runs normally
  │
  ▼
Grace period expires (now_ms >= startup_grace_until)
  │
  ▼
Next PaneUpdate event
  ├─ rebuild_pane_map() ← runs normally
  ├─ remove_dead_sessions() ← runs, reconciles cache vs manifest
  └─ save_sessions() ← persists reconciled state
  │
  ▼
Normal operation (identical to existing behavior)
```

## No New Entities

This feature does not introduce new data structures, new cache files, new pipe message types, or new configuration options. It adds a single timestamp field to the existing `PluginState` to gate an existing cleanup function.
