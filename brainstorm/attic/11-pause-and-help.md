# 11: Pause Mode & Keyboard Help

## Problem

When managing many Claude sessions, some are intentionally parked (e.g., waiting for a long build, or set aside for later). The attend mechanism (`Alt+a`) cycles through all non-working sessions, including these parked ones. Users need a way to exclude sessions from attend cycling without closing them.

Additionally, the growing number of keyboard shortcuts (j/k, Enter, Esc, r, d, /, n, p, Alt+s, Alt+a) needs a discoverable help system.

## Design

### Pause Mode

A per-session boolean flag `paused` that:
- Excludes the session from `Alt+a` attend cycling
- Changes the visual appearance (different icon, greyed-out name)
- Is toggled with `p` in navigation mode
- Persists across sync (included in Session serialization)
- Does NOT affect the session's actual activity state (it can still be Working, Idle, etc.)

**Visual treatment**:

| State | Icon | Name Style |
|-------|------|------------|
| Normal session | Activity-based (●, ⚙, ⚠, etc.) | Normal or bold |
| Paused session | ⏸ (pause symbol) | Dimmed grey (`\x1b[2m`) |

The pause icon replaces the activity icon. The underlying activity continues (hooks still update state), but the visual presentation shows the session is "on hold".

**Attend interaction**: The attend algorithm's candidate list filters out sessions where `paused == true`. If all non-working sessions are paused, attend shows "All sessions paused" (or "No sessions available").

**Click/Enter still works**: Paused sessions can still be navigated to via click, Enter, or cursor selection. Pause only affects automatic attend cycling.

### Implementation

**Session struct change**:
```rust
pub struct Session {
    // ... existing fields
    pub paused: bool,  // default: false
}
```

**Attend filter** (in attend.rs):
```rust
// Add .filter(|s| !s.paused) to all tier candidate lists
let mut t1: Vec<_> = sessions.iter()
    .filter(|s| matches!(s.activity, Activity::Waiting(WaitReason::Permission)))
    .filter(|s| !s.paused)
    .copied().collect();
```

**Key handler** (in navigation mode):
```rust
BareKey::Char('p') => {
    let sessions = self.filtered_sessions_by_tab_order();
    if let Some(session) = sessions.get(self.cursor_index) {
        let pane_id = session.pane_id;
        if let Some(s) = self.sessions.get_mut(&pane_id) {
            s.paused = !s.paused;
        }
        sync::broadcast_state(self);
    }
    true
}
```

**Sidebar rendering** (in sidebar.rs):
```rust
// Override indicator and name style for paused sessions
let (indicator, name_style) = if session.paused {
    ("⏸", "\x1b[2m")  // pause icon + dimmed
} else {
    (session.activity.indicator(), "")
};
```

### Keyboard Help (? key)

Pressing `?` in navigation mode shows a floating help overlay listing all keyboard shortcuts.

**Implementation options**:

1. **Notification-based**: Show help as a multi-line notification at the bottom of the sidebar. Simple but limited space.

2. **Overlay rendering**: Temporarily replace the session list with a help screen. Press any key to dismiss.

3. **Floating plugin pane**: Launch a floating pane with help text via `open_command_pane`. Overkill.

**Recommendation**: Option 2 (overlay). Set a `show_help: bool` flag in PluginState. When true, `render_sidebar` shows the help screen instead of sessions. Any key dismisses it.

**Help content**:

```
 Keyboard Shortcuts
 ──────────────────
 Alt+s  Session list
 Alt+a  Next session

 Navigation:
 j/↓    Move down
 k/↑    Move up
 Enter  Go to session
 Esc    Cancel

 Actions:
 r      Rename
 d      Delete
 p      Pause/unpause
 n      New tab
 /      Search
 ?      This help
```

**State**:
```rust
pub struct PluginState {
    // ... existing fields
    pub show_help: bool,
}
```

**Key handler**:
```rust
BareKey::Char('?') => {
    self.show_help = true;
    true
}
// And in the key handler, before navigation keys:
if self.show_help {
    self.show_help = false;
    return true; // any key dismisses
}
```

## Open Questions

1. Should paused sessions show their underlying activity icon in a dimmed form instead of ⏸? This would let users see if a paused session is still working. Decision: use ⏸ for clarity. Users can unpause to check.

2. Should the help overlay be scrollable for small sidebars? Probably not needed since the help content fits in ~15 lines.

3. Should `p` work outside navigation mode (e.g., via right-click or pipe command)? Start with navigation-mode only, extend later if needed.
