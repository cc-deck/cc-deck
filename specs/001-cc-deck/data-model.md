# Data Model: cc-deck

## Entities

### Session

Represents a running Claude Code instance managed by cc-deck.

| Field | Type | Description |
|-------|------|-------------|
| id | u32 | Unique session identifier (sequential) |
| pane_id | u32 | Zellij terminal pane ID (from `CommandPaneOpened` event) |
| display_name | String | Current display name (auto-detected or manually set) |
| auto_name | String | Auto-detected name (git repo or directory basename) |
| is_name_manual | bool | Whether the user manually renamed this session |
| working_dir | PathBuf | Absolute path to session's working directory |
| status | SessionStatus | Current activity status |
| group_id | String | Project group identifier (normalized repo/dir name) |
| created_at | Instant | When the session was created |
| last_activity_at | Instant | Last time any activity was detected |
| exit_code | Option\<i32\> | Exit code if Claude process has terminated |

### SessionStatus (Enum)

| Variant | Description | Trigger |
|---------|-------------|---------|
| Working | Claude is generating output or using tools | `cc-deck::working` pipe message |
| Waiting | Claude needs user input (permission, question) | `cc-deck::waiting` pipe message |
| Idle(Duration) | No activity for the configured timeout | Timer expiry since last `done` event |
| Done | Claude finished, awaiting next prompt | `cc-deck::done` pipe message |
| Exited(i32) | Claude process terminated | `CommandPaneExited` event |
| Unknown | No hook data received (hooks not configured) | Default state, fallback mode |

### State Transitions

```
                    UserPromptSubmit / PreToolUse
               ┌──────────────────────────────────────┐
               v                                      │
  ┌─────────┐     ┌─────────┐     ┌──────┐     ┌──────────┐
  │ Unknown │────>│ Working │────>│ Done │────>│   Idle   │
  └─────────┘     └─────────┘     └──────┘     └──────────┘
                    │     ^          │              │
                    v     │          v              │
                  ┌─────────┐   (timeout)          │
                  │ Waiting │                      │
                  └─────────┘                      │
                    │                              │
                    v                              v
                  ┌──────────────────────────────────┐
                  │           Exited(code)            │
                  └──────────────────────────────────┘
```

### ProjectGroup

Logical grouping of sessions sharing the same project.

| Field | Type | Description |
|-------|------|-------------|
| id | String | Normalized project name (lowercase git repo name or dir basename) |
| display_name | String | Original-case project name |
| color | Color | Assigned color from palette |
| session_count | usize | Number of active sessions in this group |

### Color Assignment

Colors are assigned from a fixed palette in order of group creation:

```
Palette: [Blue, Green, Yellow, Magenta, Cyan, Red, White, Orange]
```

When all colors are used, they wrap around. Groups keep their assigned color for the lifetime of the cc-deck session.

### RecentEntry

A previously used session configuration for quick re-launch.

| Field | Type | Description |
|-------|------|-------------|
| directory | PathBuf | Absolute path to project directory |
| name | String | Last-used session name |
| last_used | DateTime | When this entry was last used |

### RecentEntries

| Field | Type | Description |
|-------|------|-------------|
| entries | Vec\<RecentEntry\> | Ordered list, most recent first |
| max_entries | usize | Maximum entries to keep (default: 20) |

LRU eviction: when adding a new entry beyond `max_entries`, the oldest entry is removed. If the directory already exists in the list, it is moved to the front and its name/timestamp updated.

## Plugin State

The main plugin struct holds all runtime state:

| Field | Type | Description |
|-------|------|-------------|
| sessions | BTreeMap\<u32, Session\> | Active sessions keyed by session ID |
| groups | HashMap\<String, ProjectGroup\> | Project groups keyed by group ID |
| recent | RecentEntries | Persisted recent sessions |
| focused_pane_id | Option\<u32\> | Currently focused Zellij pane ID |
| picker_active | bool | Whether the fuzzy picker is currently showing |
| picker_query | String | Current search text in the picker |
| picker_selected | usize | Currently highlighted item index in picker |
| idle_timeout_secs | u64 | Configurable idle timeout (default: 300) |
| config | PluginConfig | User configuration from KDL |
| next_session_id | u32 | Counter for session ID generation |
| next_color_index | usize | Counter for color palette assignment |

### PluginConfig

Configuration from Zellij KDL plugin config:

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| idle_timeout | u64 | 300 | Seconds before idle status |
| picker_key | String | "Ctrl Shift t" | Fuzzy picker keybinding |
| new_session_key | String | "Ctrl Shift n" | New session keybinding |
| rename_key | String | "Ctrl Shift r" | Rename session keybinding |
| close_key | String | "Ctrl Shift x" | Close session keybinding |
| max_recent | usize | 20 | Max recent entries |

## Persistence Format

`/cache/recent.json`:

```json
{
  "version": 1,
  "entries": [
    {
      "directory": "/home/user/projects/api-server",
      "name": "api-server",
      "last_used": "2026-03-02T15:30:00Z"
    },
    {
      "directory": "/home/user/projects/frontend",
      "name": "frontend",
      "last_used": "2026-03-02T14:00:00Z"
    }
  ]
}
```

Version field allows future schema migrations without breaking existing data.
