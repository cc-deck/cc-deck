# Data Model: Plugin Bugfixes

**Feature**: 010-plugin-bugfixes

## Changes to Existing Entities

### Session (modified)

Add tab tracking field to the existing Session struct.

| Field (new) | Type | Description |
|-------------|------|-------------|
| tab_index   | Option<usize> | 0-indexed Zellij tab position. Updated on every TabUpdate event. None if tab mapping unknown. |

### PluginState (modified)

Add tab tracking and detection state.

| Field (new)          | Type                    | Description |
|----------------------|-------------------------|-------------|
| tab_pane_mapping     | HashMap<usize, Vec<u32>> | Map of tab index to pane IDs, rebuilt on each PaneUpdate. Used for correlating sessions to tabs. |
| pending_auto_start   | Option<(u32, PathBuf)>  | Session ID + cwd waiting for pane detection to trigger Claude auto-start. Set by new_session, cleared by PaneUpdate handler. |

## State Transitions

### Session Status (unchanged)

```
Unknown -> Working -> Done -> Idle
               ↑         |
               └─────────┘
          Waiting ←──→ Working
                         ↓
                       Exited (terminal)
```

### Tab Title Update Triggers

Tab title is updated (`rename_tab`) whenever any of these occur:
1. Status changes (pipe message received)
2. Display name changes (git detection completes, manual rename)
3. Tab index changes (TabUpdate event with new position)

Format: `{status_icon} {display_name}`

### Auto-Detection Flow

```
PaneUpdate fires
  ↓
Scan all panes in PaneManifest
  ↓
For each non-plugin pane with title containing "claude":
  ↓
  If pane_id not in tracked sessions:
    ↓
    Create Session (status: Unknown, tab_index from manifest key)
    Trigger git detection for cwd
    Register in sessions map
```

### Auto-Start Flow

```
User triggers new_session
  ↓
prepare_session(cwd) -> session_id
  ↓
new_tab(name, cwd) creates tab with shell
  ↓
Set pending_auto_start = Some((session_id, cwd))
  ↓
PaneUpdate fires with new pane (CommandPaneOpened or PaneUpdate)
  ↓
If pending_auto_start matches:
  ↓
  focus_terminal_pane(new_pane_id)
  open_command_pane_in_place(CommandToRun::claude(), context)
  Clear pending_auto_start
```
