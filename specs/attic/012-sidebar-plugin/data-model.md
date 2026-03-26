# Data Model: cc-deck Sidebar Plugin

**Date**: 2026-03-07
**Feature**: 012-sidebar-plugin

## Entities

### Session

Represents a single Claude Code instance running in a Zellij tab.

| Field | Type | Description |
|-------|------|-------------|
| pane_id | u32 | Zellij pane identifier (primary key, from hook events) |
| session_id | String | Claude Code session ID (from hook payload) |
| display_name | String | User-visible name (auto-detected or manually set) |
| activity | Activity | Current activity state |
| tab_index | Option<usize> | Position in the tab bar (from TabUpdate events) |
| tab_name | Option<String> | Zellij tab name (from TabUpdate events) |
| working_dir | Option<String> | Working directory (from hook payload cwd field) |
| git_branch | Option<String> | Git branch name (from async git detection) |
| last_event_ts | u64 | Unix timestamp of the last hook event |
| manually_renamed | bool | Whether the user has manually renamed this session |

### Activity

The current state of a session. Determines the sidebar indicator.

| State | Indicator | Triggered By | Transitions To |
|-------|-----------|-------------|----------------|
| Init | ○ (idle dot) | SessionStart hook | Working, Idle |
| Working | ● (active dot) | PreToolUse, PostToolUse, UserPromptSubmit hooks | Waiting, Done, Idle |
| Waiting | ⚡ (attention marker) | PermissionRequest hook | Working (on PostToolUse), Done |
| Idle | ○ (idle dot) | Timer (no activity for configurable threshold) | Working, Done |
| Done | ✓ (checkmark) | Stop hook | Idle (after timeout) |
| AgentDone | ✓ (checkmark, dim) | SubagentStop hook | Idle (after timeout) |

Note: Init displays identically to Idle (same indicator ○, same color). Sessions are removed entirely on SessionEnd; there is no intermediate Exited state.

State transition rules:
- Waiting can only transition to Working or Done (never back to Idle directly)
- Done transitions to Idle after a configurable timeout (default 30s)
- SessionEnd removes the session entirely (no state transition)
- Notification hook updates last_event_ts but does not change activity state

### HookEvent

A notification from Claude Code, received via `cc-deck hook` CLI command.

| Field | Type | Source | Description |
|-------|------|--------|-------------|
| session_id | Option<String> | Hook JSON body | Claude session identifier |
| pane_id | u32 | ZELLIJ_PANE_ID env var | Which pane this Claude runs in |
| hook_event | String | Hook JSON body | Event type name |
| tool_name | Option<String> | Hook JSON body | Tool name (for PreToolUse) |
| cwd | Option<String> | Hook JSON body | Working directory |

### PluginState

The aggregate state held by each sidebar plugin instance.

| Field | Type | Description |
|-------|------|-------------|
| sessions | BTreeMap<u32, Session> | All known sessions keyed by pane_id |
| tabs | Vec<TabInfo> | Current tab list from TabUpdate events |
| pane_manifest | Option<PaneManifest> | Current pane info from PaneUpdate events |
| active_tab_index | Option<usize> | Currently focused tab position |
| mode | PluginMode | Sidebar or Picker (from config) |
| sidebar_width | usize | Configured sidebar width in characters |
| permissions_granted | bool | Whether plugin permissions have been granted |
| input_mode | InputMode | Current Zellij input mode |
| rename_state | Option<RenameState> | Active rename operation (if any) |
| notification | Option<Notification> | Brief message to display (e.g., attend result) |

### RenameState

Transient state for an active inline rename operation.

| Field | Type | Description |
|-------|------|-------------|
| pane_id | u32 | Which session is being renamed |
| input_buffer | String | Characters typed so far |
| cursor_pos | usize | Cursor position in the input buffer |

### Notification

A brief inline message displayed in the sidebar.

| Field | Type | Description |
|-------|------|-------------|
| message | String | Text to display |
| expires_at | u64 | Unix timestamp (ms) when to auto-dismiss |

## Pipe Message Protocol

Messages exchanged between plugin instances and from the CLI hook command.

| Pipe Name | Direction | Payload | Purpose |
|-----------|-----------|---------|---------|
| cc-deck:hook | CLI -> Plugin | JSON (HookEvent fields) | Forward Claude Code hook events |
| cc-deck:sync | Plugin -> All | JSON (sessions map) | Broadcast current state |
| cc-deck:request | Plugin -> All | None | Ask other instances for their state |
| cc-deck:attend | Keybinding -> Plugin | None | Trigger attend action |
| cc-deck:rename | Keybinding -> Plugin | JSON (pane_id, new_name) | Rename a session |
| cc-deck:new | Keybinding -> Plugin | None | Create a new Claude session |

## Relationships

```
Deck (zellij session)
  └── has many: Session (1 per Claude tab)
       ├── has one: Activity (current state)
       └── receives many: HookEvent (from CLI)

PluginState (per sidebar instance)
  ├── holds many: Session (synced across instances)
  ├── optionally holds: RenameState
  └── optionally holds: Notification
```
