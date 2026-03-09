# 10: Keyboard Navigation & Global Shortcuts

## Problem

The sidebar currently relies entirely on mouse clicks for session interaction. This requires moving focus away from the terminal, which breaks keyboard-centric workflows. Users need:

1. A global shortcut to activate the sidebar for keyboard navigation
2. Arrow/vim-key navigation within the session list with contextual actions
3. A global "attend" shortcut that cycles through sessions intelligently

## Design

### Interaction Model: Two Modes

The sidebar operates in two modes:

**Passive mode** (default): `set_selectable(false)`. The sidebar displays session status and handles mouse clicks. Key events pass through to the terminal pane. This is the current behavior.

**Navigation mode** (activated by global shortcut): `set_selectable(true)`. The sidebar receives key events. A "cursor" (pre-selection highlight) moves through the list. Actions are bound to single keys. Pressing Escape or Enter returns to passive mode.

### Visual Design: Pre-Selection Cursor

Navigation mode needs a distinct visual cue from the "active session" highlight (dark teal bg).

| State | Visual |
|-------|--------|
| Active session (focused pane) | Dark teal bg (`25,45,55`) + bright teal fg |
| Pre-selection cursor | Inverse/reverse video or orange border/marker |
| Active + cursor on same session | Combine both (teal bg + cursor marker) |

Options for the cursor indicator:
- **Option A**: Large right-pointing triangle `▶` prefix on the cursor line
- **Option B**: Reverse video on the name text only (not the indicator)
- **Option C**: Distinct background color (e.g., muted purple `40,30,55`)

Recommendation: **Option A** (large triangle `▶`) is simplest and doesn't conflict with the active highlight. The triangle replaces the leading spaces when the cursor is on that session.

### Key Bindings in Navigation Mode

| Key | Action |
|-----|--------|
| `j` / `↓` | Move cursor down |
| `k` / `↑` | Move cursor up |
| `Enter` | Switch to cursor session (focus pane + switch tab) |
| `r` | Start inline rename for cursor session |
| `d` | Delete/close cursor session (with confirmation) |
| `/` | Enter search/filter mode |
| `n` | Create new session (same as [+] button) |
| `Esc` | Exit navigation mode (return to passive) |
| `g` / `Home` | Jump to first session |
| `G` / `End` | Jump to last session |

### Search/Filter Mode

Pressing `/` in navigation mode enters a filter sub-mode:

1. A search input appears at the bottom of the sidebar (replacing the [+] button row)
2. As the user types, sessions are filtered by display name (case-insensitive substring match)
3. `Enter` confirms filter and moves cursor to first match
4. `Esc` clears filter and returns to unfiltered navigation mode
5. Empty filter (just pressing Enter) shows all sessions

Implementation: reuse the existing `RenameState` pattern (input buffer + cursor position) but with a new `FilterState` variant in `PluginState`.

### Delete Confirmation

Pressing `d` on a session shows an inline confirmation:

```
  Delete "cc-deck"? [y/N]
```

This replaces the session's display lines temporarily. `y` confirms (closes the command pane and optionally the tab), any other key cancels.

### Global Shortcuts

#### Activate Sidebar Navigation

A global Zellij keybinding switches focus to the sidebar plugin pane, triggering navigation mode.

**Registration approach**: Use `reconfigure()` at plugin load to inject keybindings into the `shared_except "locked"` block. The keybinding sends a pipe message that the plugin receives.

Alternative: Use `rebind_keys()` to dynamically add a keybinding that calls `focus_plugin_pane()` on the sidebar's plugin pane ID. The plugin detects it received focus (via a FocusReceived-like event or by tracking focus changes) and enters navigation mode.

**Practical approach**: Register a keybinding that sends `MessagePlugin "cc-deck" { name "navigate" }`. The plugin's `pipe()` handler receives this, calls `set_selectable(true)`, sets `navigation_mode = true`, and initializes the cursor position.

**Default key**: `Alt+s` (s for sidebar/sessions). Must not conflict with:
- Zellij defaults (Ctrl+p for pane, Ctrl+t for tab, Ctrl+n for resize, Ctrl+o for session, Ctrl+g for locked)
- Claude Code (Ctrl+C for cancel, Ctrl+D for EOF, Esc for interrupt)

`Alt+<key>` combinations are generally safe since Zellij uses Ctrl-based prefixes.

#### Smart Attend (Cycle Through Sessions)

A global shortcut that jumps directly to the next session needing attention, without entering navigation mode.

**Default key**: `Alt+a` (a for attend)

**Enhanced algorithm** (replacing the current "oldest waiting"):

Priority tiers, evaluated in order:

1. **Critical attention** (oldest first): Sessions with `Activity::Waiting` that require user input (PermissionRequest). These are blocking and need immediate action.

2. **Soft attention** (oldest first): Sessions with `Activity::Waiting` from Notification events. These are informational but the session is paused.

3. **Idle sessions** (newest first): Sessions with `Activity::Idle`, `Activity::Done`, or `Activity::AgentDone`. The newest idle session is picked first because older idle sessions may be intentionally parked.

4. **Working sessions** (skip): Sessions with `Activity::Working` or `Activity::ToolUse` are actively running and don't need attention.

When cycling, the attend action should skip the currently focused session and find the next one in priority order. If all sessions are working, show "All sessions busy".

**Implementation**: This requires distinguishing between PermissionRequest-waiting and Notification-waiting in the `Activity` enum or adding a sub-field to `Activity::Waiting`.

```rust
pub enum WaitReason {
    PermissionRequest,  // Critical: blocks progress
    Notification,       // Soft: informational pause
}

pub enum Activity {
    Waiting(WaitReason),
    // ... existing variants
}
```

### State Changes

New fields in `PluginState`:

```rust
pub struct PluginState {
    // ... existing fields

    /// Whether the sidebar is in keyboard navigation mode.
    pub navigation_mode: bool,
    /// Index of the cursor in the sorted session list (navigation mode).
    pub cursor_index: usize,
    /// Active filter string (None = no filter active).
    pub filter_state: Option<FilterState>,
    /// Delete confirmation target (pane_id of session being deleted).
    pub delete_confirm: Option<u32>,
}

pub struct FilterState {
    pub input_buffer: String,
    pub cursor_pos: usize,
}
```

### Plugin Lifecycle

1. **Plugin load**: Register global keybindings via `reconfigure()` or `rebind_keys()`
2. **Global shortcut pressed**: Pipe message received, enter navigation mode
3. **Navigation mode**: `set_selectable(true)`, process key events
4. **Action taken** (Enter/r/d/n): Execute action, optionally exit navigation mode
5. **Esc pressed**: Exit navigation mode, `set_selectable(false)`

### Configuration

New config options in the layout plugin block:

```kdl
plugin location="file:~/.config/zellij/plugins/cc_deck.wasm" {
    mode "sidebar"
    navigate_key "Alt s"     // Global shortcut to activate sidebar
    attend_key "Alt a"       // Global shortcut for smart attend
}
```

## Open Questions

1. **Keybinding registration timing**: Should keybindings be registered on first `load()` or after permissions are granted? The `Reconfigure` permission is already requested.

2. **Focus return**: After pressing Enter (switch to session), should focus return to the terminal pane automatically? Yes, since `focus_terminal_pane()` already does this.

3. **Keybinding conflicts**: Need to verify `Alt+s` and `Alt+a` don't conflict with common terminal programs (vim, tmux, etc.) running inside Zellij panes. Alt-key combinations are generally safe because most terminal programs use Ctrl-based shortcuts.

4. **Multiple sidebar instances**: Each tab has its own sidebar plugin instance. Only one should respond to the global shortcut. Use the instance on the active tab (check `self.active_tab_index` matches the tab this instance is on).

5. **Cursor persistence**: Should the cursor position persist when exiting and re-entering navigation mode? Probably yes, for quick re-navigation.

## Dependencies

- Existing: `set_selectable()`, Key event handling, pipe protocol
- New API needed: `rebind_keys()` or `reconfigure()` for global shortcut registration
- Possibly: `focus_plugin_pane()` or similar to programmatically focus the sidebar

## Risks

- **Keybinding collisions**: Different users have different Zellij configs. The chosen keys might conflict. Mitigation: make keys configurable, choose uncommon defaults.
- **set_selectable toggle latency**: Rapid toggling between selectable/non-selectable might cause visual glitches or missed events. Mitigation: debounce or ensure clean state transitions.
- **Activity differentiation**: Splitting `Waiting` into sub-reasons requires changes to the hook payload parsing. The hook JSON from Claude Code may not distinguish between PermissionRequest and Notification waiting states. Need to verify what `hook_event_name` values are sent.
