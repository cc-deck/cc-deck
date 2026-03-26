# Pipe Protocol Extension: 013-keyboard-navigation

## New Pipe Messages

### cc-deck:navigate

Toggles navigation mode on the active tab's sidebar instance.

**Source**: Global keybinding (`Alt+s`) via Zellij `MessagePlugin` action.
**Target**: All sidebar plugin instances (broadcast). Only the active tab's instance responds.
**Payload**: None required.

**Behavior**:
- If navigation mode is off: enter navigation mode (set_selectable, focus sidebar, show cursor)
- If navigation mode is on: exit navigation mode (set_selectable false, focus terminal)

### cc-deck:attend (Enhanced)

Existing pipe message, enhanced with smart priority algorithm.

**Source**: Global keybinding (`Alt+a`) via Zellij `MessagePlugin` action, or `zellij pipe --name cc-deck:attend`.
**Payload**: None.

**New behavior**: Uses tiered priority (Permission waiting > Notification waiting > idle newest-first). Skips currently focused session.

## Keybinding Registration

Registered dynamically via `reconfigure()` after permissions are granted:

```kdl
keybinds {
    shared_except "locked" {
        bind "Alt s" { MessagePlugin "cc-deck" { name "navigate" } }
        bind "Alt a" { MessagePlugin "cc-deck" { name "attend" } }
    }
}
```

Written with `save_configuration_file: false` (session-only, not persisted to user config).
