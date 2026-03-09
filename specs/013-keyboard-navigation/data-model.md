# Data Model: 013-keyboard-navigation

## Entities

### WaitReason (New)

Distinguishes why a session is in a waiting state.

| Variant | Description | Source Hook Event |
|---------|-------------|-------------------|
| Permission | Session blocked on user permission decision | `PermissionRequest` |
| Notification | Session paused with informational notification | `Notification` |

### Activity (Modified)

Session activity state. The `Waiting` variant now carries a `WaitReason`.

| Variant | Description | Transition From | Transition To |
|---------|-------------|-----------------|---------------|
| Init | Initial state after SessionStart | (entry) | Working, ToolUse |
| Working | Active work in progress | Init, ToolUse, Waiting | ToolUse, Waiting, Done |
| ToolUse(name) | Using a specific tool | Working, Init | Working, Waiting |
| Waiting(reason) | Blocked or paused | Working, ToolUse | Working, ToolUse, Done |
| Idle | No activity for configurable timeout | Done, AgentDone | Working |
| Done | Session stopped normally | Working, ToolUse, Waiting | Idle |
| AgentDone | Subagent stopped | Working, ToolUse | Idle |

### NavigationState (Implicit in PluginState)

Not a separate struct, but a set of related fields on PluginState.

| Field | Type | Description |
|-------|------|-------------|
| navigation_mode | bool | Whether sidebar is in keyboard navigation mode |
| cursor_index | usize | Index of cursor in the filtered/sorted session list |

### FilterState (New)

Active search/filter state during `/` sub-mode.

| Field | Type | Description |
|-------|------|-------------|
| input_buffer | String | Current search query text |
| cursor_pos | usize | Text cursor position within the input buffer |

### PluginConfig (Extended)

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| navigate_key | String | "Alt s" | Global shortcut to toggle navigation mode |
| attend_key | String | "Alt a" | Global shortcut for smart attend |
| (existing fields preserved) | | | |

## Smart Attend Priority Algorithm

Sessions are evaluated in tiers. Within each tier, the sorting order differs:

| Priority | Activity State | Sort Order | Rationale |
|----------|---------------|------------|-----------|
| 1 (highest) | Waiting(Permission) | Oldest first | Blocking, needs immediate action |
| 2 | Waiting(Notification) | Oldest first | Informational pause |
| 3 | Idle / Done / AgentDone | Newest first | Older idle sessions may be intentionally parked |
| 4 (skip) | Working / ToolUse | Not selected | Actively running, no attention needed |
| 5 (skip) | Init | Not selected | Just started, no attention needed |

The currently focused session is skipped when cycling, unless it's the only session.
