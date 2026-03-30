# Data Model: Fix State Consistency and Add Refresh Command

**Date**: 2026-03-30
**Feature**: 001-fix-state-consistency

## Entities

### Session (existing, no changes)

The core entity representing a Claude Code session running in a Zellij pane.

| Field | Type | Description |
|-------|------|-------------|
| pane_id | u32 | Unique pane identifier (key) |
| session_id | String | Claude Code session identifier |
| display_name | String | User-visible session name |
| activity | Activity | Current activity state (Init, Working, Waiting, Idle, Done, AgentDone) |
| tab_index | Option<usize> | Tab position |
| tab_name | Option<String> | Tab display name |
| working_dir | Option<String> | Working directory path |
| git_branch | Option<String> | Current git branch |
| last_event_ts | u64 | Unix timestamp of last state change |
| manually_renamed | bool | Whether user renamed this session |
| paused | bool | Whether session is paused |
| meta_ts | u64 | Timestamp of last metadata change |
| done_attended | bool | Whether Done state was attended |
| pending_tab_rename | bool | Deferred tab rename flag |

### SessionMeta (existing, no changes)

Metadata override for file-based sync between instances.

| Field | Type | Description |
|-------|------|-------------|
| display_name | String | User-set display name |
| manually_renamed | bool | Whether manually renamed |
| paused | bool | Pause state |
| meta_ts | u64 | Timestamp for conflict resolution |

### PipeAction (modified)

Enum of pipe message actions. Adding one new variant.

| Variant | Description | Status |
|---------|-------------|--------|
| Refresh | Clear caches and broadcast authoritative state | **NEW** |
| (all existing variants) | Unchanged | Existing |

## State Lifecycle

### Activity State Transitions (existing, no changes)

```
Init -> Working -> Done -> Idle (via cleanup_stale_sessions after 30s)
Init -> Working -> Waiting -> Working -> Done
Init -> Working -> AgentDone -> Idle (via cleanup_stale_sessions)
```

### Cache File Lifecycle

```
Plugin Load:
  restore_sessions() reads /cache/zellij_pid
    -> PID matches: load sessions.json (reattach recovery)
    -> PID mismatch: clear sessions.json, session-meta.json, zellij_pid [CHANGED: now also clears session-meta.json]

Timer Tick (active tab only):
  cleanup_stale_sessions() -> if changed: broadcast_and_save [CHANGED: was save_sessions]
  remove_dead_sessions() -> if changed: broadcast_and_save + prune_session_meta [CHANGED: was save_sessions, added prune]

PaneClosed:
  sessions.remove() -> sync_now() + prune_session_meta [CHANGED: added prune]

Refresh (new):
  clear sessions.json, session-meta.json, last_click
  broadcast_and_save (writes current in-memory state)
```

## File-Based Storage

### /cache/sessions.json
Full session state, serialized as `BTreeMap<u32, Session>`. PID-protected.

### /cache/session-meta.json
User metadata overrides, serialized as `BTreeMap<u32, SessionMeta>`. Now PID-protected (change) and pruned on dead session removal (change).

### /cache/zellij_pid
Plain text file containing the Zellij server PID. Used for staleness detection.

### /cache/last_click
Plain text `{timestamp},{pane_id}`. Used for double-click detection. Cleared on refresh.
