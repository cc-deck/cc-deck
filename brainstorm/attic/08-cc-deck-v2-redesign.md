# Brainstorm: cc-deck v2 Redesign

**Date:** 2026-03-07
**Status:** active
**Participants:** Roland, Claude

## Problem Framing

The original cc-deck plugin tried to be everything: status bar, picker, session manager, and keybinding controller in a single dual-mode WASM binary. This led to circular development issues, especially around floating pane permissions, dual-mode complexity, and duplicated functionality with the existing zellaude status bar plugin.

After studying the zellij plugin architecture, zellaude, zellij-attention, zellij-vertical-tabs, and other plugins (room, harpoon, zbuffers), we're redesigning cc-deck from scratch with clear architectural boundaries and a focused scope.

## Prior Art Analysis

### zellaude (status bar plugin)
**Strengths to absorb:**
- Rich per-tool activity indicators (thinking, bash, edit, waiting, done)
- Auto-installation of Claude Code hooks into settings.json
- Multi-instance state sync via pipe messages (`zellaude:sync`, `zellaude:request`)
- Desktop notifications on permission requests
- Flash/blink animation on waiting tabs
- Click-to-focus on waiting panes
- Settings persistence to JSON file
- Powerline rendering with true-color support

**Weaknesses to avoid:**
- Destroyed `~/.claude/settings.json` (wrote 0 bytes) during uninstall
- No safe backup mechanism for settings.json modifications
- Shell script hook is fragile (depends on jq, complex sed/jq pipelines)
- No session management or workspace organization
- No keyboard-driven navigation beyond clicking

### zellij-attention (tab notification plugin)
**Strengths:** Lightweight, non-intrusive (icons in tab names), global instance via `load_plugins`
**Weaknesses:** Very limited (only two states), modifies tab names (fragile)

### zellij-vertical-tabs (sidebar navigation)
**Strengths:** Vertical tab list with tmux-style format strings, configurable styling, click navigation, scroll support, border customization, overflow indicators
**Weaknesses:** Generic tab display (no Claude-specific awareness)

### room.wasm / harpoon.wasm / zbuffers.wasm
**Pattern:** Registered as plugin aliases, launched via `LaunchOrFocusPlugin` with floating=true from keybindings in `shared_except "locked"` block.

## Key Decisions

### Terminology

| Term | Meaning | Zellij Mapping |
|------|---------|---------------|
| **Deck** | A project workspace containing related Claude sessions | Zellij session |
| **Session** | An individual Claude Code instance | Tab with command pane running `claude` |
| **Attend** | Jump to the next session that needs human input | Focus switch (within or across decks) |

We keep "session" despite the zellij collision because:
- It's the natural term for "a running Claude Code instance"
- Within cc-deck UI/docs, context makes it unambiguous
- When disambiguation is needed: "Claude session" vs "zellij session"
- The CLI uses "deck" for the zellij-session-level concept

### Architecture: Absorb, Don't Complement

cc-deck absorbs the best ideas from zellaude and replaces it entirely. Reasons:
- Avoid maintaining two separate hook systems
- Single installation, single plugin
- zellaude had quality issues (settings.json destruction)
- Unified state model instead of two plugins trying to track the same thing

### Plugin Architecture: Single WASM, Multiple Instances

One WASM binary (`cc_deck.wasm`) used in two contexts:

1. **Sidebar instance** - loaded via `tab_template` in the layout, appears on every tab as a vertical session list. Each tab gets its own instance; they sync state via pipe messages. Handles: session list rendering, activity indicators, click navigation, rename, attend.

2. **Picker instance** - launched on demand via `LaunchOrFocusPlugin` as a floating pane. Provides fuzzy search across sessions. Reads state from sidebar instances via pipe sync.

State sync protocol (proven by zellaude):
```
cc-deck:sync     - broadcast current state to all instances
cc-deck:request  - ask other instances for their state
cc-deck:hook     - receive hook events from CLI
cc-deck:attend   - trigger attend action
cc-deck:rename   - trigger rename on a session
```

### Sidebar on All Tabs

The sidebar appears on every tab via the layout's `tab_template`:

```kdl
layout {
    tab_template name="default" {
        pane split_direction="vertical" {
            pane size=22 borderless=true {
                plugin location="file:~/.config/zellij/plugins/cc_deck.wasm" {
                    mode "sidebar"
                }
            }
            children
        }
        pane size=1 borderless=true {
            plugin location="compact-bar"
        }
    }
}
```

Each instance syncs via pipes. The sidebar highlights the session corresponding to the currently active tab.

### Hook Integration: Go Binary

Claude Code hooks call `cc-deck hook` directly (registered in settings.json as the hook command). The Go binary:
- Reads hook JSON from stdin
- Extracts session_id, hook_event, tool_name, cwd, ZELLIJ_PANE_ID
- Forwards to zellij via `zellij pipe --name "cc-deck:hook" --payload '<json>'`
- Gracefully no-ops if zellij is not running (no error, no crash)
- Handles all Claude Code hook events: SessionStart, SessionEnd, PreToolUse, PostToolUse, PostToolUseFailure, UserPromptSubmit, PermissionRequest, Notification, Stop, SubagentStop

### Installation: Layout-Only, No Config Surgery

`cc-deck install` does not modify config.kdl. Instead:
- Copies `cc_deck.wasm` to `~/.config/zellij/plugins/`
- Installs layout files to `~/.config/zellij/layouts/`
- Backs up `~/.claude/settings.json` before modifying (timestamped backup)
- Registers hooks in settings.json using safe JSON manipulation (no jq dependency, Go handles it)
- Prints instructions for the user to set `default_layout "cc-deck"` if desired

### Sidebar Design

```
+--------------------+
| CC-DECK            |  <- deck name (clickable for settings)
|--------------------|
|                    |
| * refactor-auth    |  <- active session (highlighted, bold)
|   main             |  <- git branch (dimmed)
|                    |
| ! add-tests        |  <- waiting (attention indicator, flash)
|   feature/tests    |
|                    |
| . fix-bug-123      |  <- idle
|   main             |
|                    |
| + fix-bug-456   2m |  <- done (with elapsed time)
|   hotfix/456       |
|                    |
+--------------------+
|  [+] New  [?] Help |  <- action bar (clickable)
+--------------------+
```

Activity indicators (from zellaude, refined):
- `*` or spinning: thinking/working
- `!` or flashing: waiting for input (permission request)
- `>` prompting (user just submitted)
- `.` idle
- `+` done
- Tool-specific: show tool name briefly during PreToolUse

Sidebar features:
- **Highlight**: Active session (matching current tab) shown with distinct background/bold
- **Click**: Click a session to switch to its tab
- **Rename**: Keybinding or double-click triggers inline rename; updates both cc-deck display name and zellij tab/pane name
- **Collapse**: Toggle between full sidebar (22 chars) and icon-only mode (3 chars: just activity indicator)
- **Configurable width**: Default 22, adjustable in plugin config

### Attend Key

Single keystroke to jump to the next session needing attention:

1. Scan current deck for sessions with `Waiting` status
2. If found: focus that tab
3. If not found: show notification with deck names that have waiting sessions, let user decide
4. Priority order: PermissionRequest > oldest waiting first

### Floating Picker

Triggered by keybinding (e.g., `Ctrl+y` or configurable):
- Shows all sessions in current deck with fuzzy search
- Type to filter by session name, branch, or directory
- Arrow keys to navigate, Enter to select, Esc to close
- Toggle key to expand to cross-deck view (all zellij sessions)
- Shows activity indicator, elapsed time, branch for each entry

### Session Creation

New session keybinding or click on [+] in sidebar:
- Opens a new tab with `claude` command in the current deck's working directory
- Auto-names based on git repo detection (with numeric suffix for duplicates)
- User can rename immediately after creation

## Priority Scope

### P0 - Must Have (v1.0)

| Feature | Description |
|---------|-------------|
| Sidebar plugin | Vertical session list with activity indicators, highlight active, click-to-focus |
| Hook integration | `cc-deck hook` Go binary, pipe to plugin, graceful degradation |
| Install command | `cc-deck install`: WASM + layout + hooks with settings.json backup |
| State sync | Multi-instance pipe sync (sidebar on every tab) |
| Permission handling | Permissions dialog visible in sidebar pane |

### P1 - Important (v1.1)

| Feature | Description |
|---------|-------------|
| Floating picker | Fuzzy search, keyboard navigation, current-deck scope |
| Attend key | Jump to next waiting session in current deck |
| Session creation | New tab with Claude from sidebar or keybinding |
| Session rename | Inline rename in sidebar, updates pane name |

### P2 - Nice to Have (v1.2)

| Feature | Description |
|---------|-------------|
| Sidebar collapse | Icon-only mode (3 chars) to save screen space |
| Bar mode | Alternative 1-row horizontal status bar mode (config option) |
| Opinionated layouts | Multi-session layouts (e.g., 2-up, 3-up with shared sidebar) |
| Desktop notifications | macOS/Linux notifications on permission requests |
| Settings menu | Click sidebar header to toggle settings (notifications, flash, elapsed time) |

### P3 - Future (v2.0+)

| Feature | Description |
|---------|-------------|
| Cross-deck attend | Auto-switch zellij session when no local waiting sessions |
| Deck management CLI | `cc-deck new <project>`, `cc-deck list`, `cc-deck switch` |
| Picker cross-deck | Expand picker to show sessions across all decks |
| Remote sessions | Container/K8s-based Claude execution backends |
| Session persistence | Remember and restore sessions across zellij restarts |
| Team mode | Multiple sessions in worktrees, auto-merge coordination |

## Open Threads

- Exact keybinding assignments (avoid conflicts with zellij defaults and Claude Code)
- Sidebar width default and minimum viable collapsed width
- Flash animation implementation (timer-based blink vs steady highlight)
- Whether to show non-Claude tabs in sidebar or only Claude sessions
- Git branch detection: async via `run_command` or sync
- Pipe message format: JSON payload vs simple delimited strings
- How sidebar handles permission dialog (inline or delegates to floating pane)

## Rejected Alternatives

### Complement zellaude instead of absorbing
Rejected because: two hook systems, two state models, two plugins to install and coordinate. The quality issues in zellaude (settings.json destruction) mean we'd inherit fragility.

### Pure CLI orchestrator (no custom plugin)
Rejected because: can't provide floating picker UI or real-time sidebar from outside zellij. The pipe API is powerful but render-heavy interactions need a plugin.

### Single global instance via load_plugins
Rejected because: `load_plugins` creates background instances without visible panes. The sidebar needs a visible pane on every tab, which requires `tab_template` in the layout.

### 1-row status bar as default
Rejected because: permission dialogs were invisible, limited space for session info, click targets too small, doesn't scale beyond 4-5 sessions.
