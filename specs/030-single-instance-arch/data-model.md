# Data Model: Single-Instance Architecture

## Shared Types (lib.rs)

### Session (existing, moved to shared)

```
Session
├── pane_id: u32                    # Zellij terminal pane ID (key)
├── session_id: String              # Claude Code session UUID
├── display_name: String            # User-visible name (deduplicated)
├── activity: Activity              # Current state enum
├── tab_index: Option<usize>        # Tab position (assigned by controller)
├── tab_name: Option<String>        # Tab name string
├── working_dir: Option<String>     # CWD for git detection
├── git_branch: Option<String>      # Current git branch
├── last_event_ts: u64              # Last hook event timestamp
├── manually_renamed: bool          # User explicitly renamed
├── paused: bool                    # User paused session
├── meta_ts: u64                    # User metadata change timestamp
├── done_attended: bool             # Already attended once
└── pending_tab_rename: bool        # Deferred rename (waiting for tab_index)
```

### Activity (existing, moved to shared)

```
Activity
├── Init                            # Session just started
├── Working                         # Actively processing
├── Waiting(WaitReason)             # Needs attention
│   ├── Permission                  # Permission request
│   └── Notification                # User notification
├── Idle                            # Between tasks
├── Done                            # Session completed
└── AgentDone                       # Sub-agent completed
```

### RenderPayload (new)

Sent from controller to all sidebars via `cc-deck:render` pipe message.

```
RenderPayload
├── sessions: Vec<RenderSession>    # Pre-sorted session list
├── focused_pane_id: Option<u32>    # Currently focused terminal pane
├── active_tab_index: usize         # Currently active tab
├── notification: Option<String>    # Controller-level notification text
├── notification_expiry: Option<u64> # When notification expires
├── total: usize                    # Total session count
├── waiting: usize                  # Sessions in Waiting state
├── working: usize                  # Sessions in Working state
├── idle: usize                     # Sessions in Idle/Done state
└── controller_plugin_id: u32      # Controller's plugin_id for targeted responses
```

### RenderSession (new)

Pre-computed display data for a single session. The sidebar renders this directly without further computation.

```
RenderSession
├── pane_id: u32                    # For action messages back to controller
├── display_name: String            # Already deduplicated
├── activity_label: String          # "Working", "Waiting", "Idle", etc.
├── indicator: char                 # ●, ⚠, ○, ✓
├── color: (u8, u8, u8)            # RGB tuple for indicator/text
├── git_branch: Option<String>      # Branch name for second line
├── tab_index: usize                # Tab containing this session
├── paused: bool                    # Pause indicator
└── done_attended: bool             # Already attended
```

### ActionMessage (new)

Sent from sidebar to controller via `cc-deck:action` pipe message.

```
ActionMessage
├── action: ActionType              # What to do
├── pane_id: Option<u32>            # Target session (if applicable)
├── tab_index: Option<usize>        # Source tab
├── value: Option<String>           # Action parameter (e.g., new name)
└── sidebar_plugin_id: u32         # Sender's plugin_id
```

### ActionType (new)

```
ActionType
├── Switch                          # Switch to session (pane_id required)
├── Rename                          # Rename session (pane_id + value required)
├── Delete                          # Delete/close session (pane_id required)
├── Pause                           # Toggle pause (pane_id required)
├── Attend                          # Smart attend (no pane_id, controller picks)
├── Navigate                        # Enter/exit navigate mode on active sidebar
└── NewSession                      # Request new Claude Code session
```

### SidebarHello (new)

Sent from sidebar to controller via `cc-deck:sidebar-hello` pipe message during registration.

```
SidebarHello
└── plugin_id: u32                  # Sidebar's own plugin_id
```

### SidebarInit (new)

Sent from controller to specific sidebar via `cc-deck:sidebar-init` pipe message.

```
SidebarInit
├── tab_index: usize                # Assigned tab index
└── controller_plugin_id: u32      # Controller's plugin_id for future messages
```

### HookPayload (existing, moved to shared)

```
HookPayload
├── session_id: String              # Claude Code session UUID
├── pane_id: u32                    # Zellij pane ID (added by CLI)
├── hook_event_name: String         # Event type
├── tool_name: Option<String>       # Tool being used
└── cwd: Option<String>             # Working directory
```

## Controller State (controller only)

```
ControllerState
├── sessions: BTreeMap<u32, Session> # Authoritative session state
├── pane_manifest: Option<PaneManifest> # Latest pane layout
├── pane_to_tab: HashMap<u32, (usize, String)> # Pane → (tab_index, tab_name)
├── tabs: Vec<TabInfo>              # Tab list from TabUpdate
├── active_tab_index: Option<usize> # Currently active tab
├── focused_pane_id: Option<u32>    # Currently focused pane
├── sidebar_registry: HashMap<u32, usize> # sidebar_plugin_id → tab_index
├── plugin_id: u32                  # Controller's own plugin_id
├── permissions_granted: bool       # Permission state
├── render_dirty: bool              # Coalesce flag for render broadcasts
├── startup_grace_until: Option<u64> # Startup grace deadline
├── pending_overrides: HashMap<String, Vec<PendingOverride>> # Snapshot restore
├── config: PluginConfig            # Parsed KDL configuration
└── zellij_pid: u32                 # Current Zellij server PID
```

## Sidebar State (sidebar only)

```
SidebarState
├── mode: SidebarMode               # Current interaction mode
├── cached_payload: Option<RenderPayload> # Last received render data
├── click_regions: Vec<(usize, u32, usize)> # (row, pane_id, tab_index)
├── my_tab_index: Option<usize>     # Assigned by controller
├── my_plugin_id: u32               # Own plugin_id
├── controller_plugin_id: Option<u32> # Learned from init/render
├── scroll_offset: usize            # Vertical scroll position
├── filter_text: String             # Active filter (empty = no filter)
├── notification: Option<Notification> # Local notification
├── config: PluginConfig            # Parsed KDL configuration
└── initialized: bool              # Has received first render payload
```

## State Transitions

### Session Lifecycle (controller)

```
Hook: SessionStart → Session created (Activity::Init)
Hook: PreToolUse/PostToolUse/UserPromptSubmit → Activity::Working
Hook: PermissionRequest → Activity::Waiting(Permission)
Hook: Notification → Activity::Waiting(Notification)
Hook: Stop → Activity::Done
Hook: SubagentStop → Activity::AgentDone
Hook: SessionEnd → Session removed
Timer: Stale timeout → Session removed (if no hooks for configurable period)
PaneClosed: → Session removed
```

### Sidebar Mode Transitions (sidebar)

```
Passive → Navigate (Alt+s keybinding or cc-deck:navigate pipe)
Navigate → NavigateFilter ('/' key)
Navigate → NavigateDeleteConfirm ('d' key)
Navigate → NavigateRename ('r' key)
Navigate → Passive (Esc, focus lost)
NavigateFilter → Navigate (Esc, Enter)
NavigateDeleteConfirm → Navigate ('n' or Esc)
NavigateDeleteConfirm → Passive ('y' confirms, action sent to controller)
NavigateRename → Navigate (Esc cancels)
NavigateRename → Passive (Enter confirms, action sent to controller)
Any → Help (F1 or '?' key)
Help → Previous mode (any key)
Passive → RenamePassive (double-click or right-click)
RenamePassive → Passive (Enter confirms, Esc cancels)
```

## Persistence

### /cache/sessions.json (controller writes, controller reads)

Single writer. No merge conflicts. Written on state changes (debounced). Read on controller startup for reattach recovery.

### /cache/zellij_pid (controller writes, controller reads)

Tracks Zellij server PID for stale cache detection across session boundaries.

### Eliminated Files

- `/cache/session-meta.json` - No longer needed (single writer)
- `/cache/last_click` - Double-click detection moves to sidebar-local state
- `/cache/debug.log` - Remains (debugging aid, not sync-related)
