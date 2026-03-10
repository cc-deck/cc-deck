# Research: 013-keyboard-navigation

## R1: Global Shortcut Registration

**Decision**: Use `reconfigure()` with KDL syntax containing `MessagePlugin` directives.
**Rationale**: The cc-deck plugin already requests `PermissionType::Reconfigure`. KDL syntax is more readable than constructing `Action::KeybindPipe` enums via `rebind_keys()`. The `shared_except "locked"` KDL block directly maps to the requirement.
**Alternatives considered**:
- `rebind_keys()`: More verbose, requires constructing `Action::KeybindPipe` structs programmatically. Works but harder to maintain.

**Implementation**:
```rust
let kdl = format!(r#"
keybinds {{
    shared_except "locked" {{
        bind "{navigate_key}" {{ MessagePlugin "cc-deck" {{ name "navigate" }} }}
        bind "{attend_key}" {{ MessagePlugin "cc-deck" {{ name "attend" }} }}
    }}
}}
"#, navigate_key = "Alt s", attend_key = "Alt a");
reconfigure(kdl, false); // false = don't write to disk
```

**Timing**: Register after `PermissionStatus::Granted`, not in `load()`.

## R2: PermissionRequest vs Notification Distinction

**Decision**: Add `WaitReason` enum to `Activity::Waiting` variant.
**Rationale**: The hook `hook_event_name` field distinguishes `"PermissionRequest"` from `"Notification"`, but the current `Activity::Waiting` collapses both into a single state. Smart attend needs the distinction.
**Alternatives considered**:
- Separate `wait_reason: Option<WaitReason>` field on Session: Works but splits related state across two fields.
- Track last hook event name on Session: Over-general, couples session state to raw hook data.

**Current mapping** (pipe_handler.rs):
| hook_event_name | Current Activity | New Activity |
|---|---|---|
| PermissionRequest | Waiting | Waiting(WaitReason::Permission) |
| Notification | None (only updates timestamp) | Waiting(WaitReason::Notification) |

**Note**: Currently `Notification` does NOT set `Waiting` state. The spec requires it to trigger soft attention, so the mapping needs to change.

## R3: Focus and Selectability Relationship

**Decision**: Navigation mode requires both `set_selectable(true)` and `focus_plugin_pane()`.
**Rationale**: These are independent systems. `set_selectable(true)` makes the pane focusable but doesn't move focus to it. `focus_plugin_pane()` requires the pane to be selectable. Since the global shortcut arrives via pipe message (delivered regardless of focus), the sidebar must explicitly grab focus.
**Alternatives considered**:
- Only `set_selectable(true)`: Won't work because the sidebar isn't focused when the pipe arrives. Key events go to the terminal pane.

**Pipe message delivery**: Always works regardless of focus or selectability. This is how the global shortcut works: `MessagePlugin` sends a pipe message, plugin's `pipe()` handler receives it.

**Exit pattern**: `set_selectable(false)` + `focus_terminal_pane()` to return focus to the terminal.

**API signatures**:
```rust
pub fn focus_plugin_pane(plugin_pane_id: u32, should_float_if_hidden: bool, should_be_in_place_if_hidden: bool)
pub fn set_selectable(selectable: bool)
```

## R4: Navigation Patterns from Other Plugins

**Decision**: Follow harpoon's pattern: `selected: usize` index + cursor wrapping with modulo.
**Rationale**: Proven pattern across multiple Zellij plugins (harpoon, room, zbuffers). Simple and predictable.

**Common patterns**:
- Key matching: `match key.bare_key { BareKey::Char('j') | BareKey::Down => ... }`
- Modifier check: `key.has_no_modifiers()` for navigation keys
- Cursor wrapping: `self.selected = (self.selected + 1) % list.len()`
- Focus action: `switch_tab_to()` + `focus_terminal_pane()` for cross-tab navigation

**Key handler nesting order** (from most specific to most general):
1. Rename state (existing)
2. Filter/search state (new)
3. Delete confirmation (new)
4. Navigation mode (new)
5. Passive mode (no-op)

## R5: Current cc-deck State Changes Needed

**New state fields**:
- `navigation_mode: bool` - whether sidebar is in navigation mode
- `cursor_index: usize` - index in sorted session list
- `filter_state: Option<FilterState>` - active search filter
- `delete_confirm: Option<u32>` - pane_id of session pending deletion

**New config fields**:
- `navigate_key: String` (default: "Alt s")
- `attend_key: String` (default: "Alt a")

**New pipe actions**:
- `cc-deck:navigate` - toggle navigation mode

**Files requiring changes**:
- `state.rs` - new fields
- `session.rs` - `Activity::Waiting(WaitReason)`
- `pipe_handler.rs` - new pipe action, updated hook mapping
- `config.rs` - new config fields
- `sidebar.rs` - cursor rendering, filter rendering
- `main.rs` - key handler expansion, keybinding registration
- `attend.rs` - smart attend algorithm with priority tiers
