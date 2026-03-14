# Pipe Command Contract: Demo Control

## Overview

New pipe commands for programmatic plugin control. Extends the existing `cc-deck:*` pipe namespace.

## Commands

### Navigation Control

| Command | Payload | Response | Re-render |
|---------|---------|----------|-----------|
| `cc-deck:nav-toggle` | None | None | Yes |
| `cc-deck:nav-up` | None | None | Yes |
| `cc-deck:nav-down` | None | None | Yes |
| `cc-deck:nav-select` | None | None | Yes |

### Session Control

| Command | Payload | Response | Re-render |
|---------|---------|----------|-----------|
| `cc-deck:attend` | None | None (existing) | Yes |
| `cc-deck:pause` | None | None | Yes |
| `cc-deck:help` | None | None | Yes |

## CLI Usage

```bash
# Toggle navigation mode
zellij pipe --name "cc-deck:nav-toggle"

# Move cursor down, then select
zellij pipe --name "cc-deck:nav-down"
zellij pipe --name "cc-deck:nav-select"

# Smart attend (already exists)
zellij pipe --name "cc-deck:attend"

# Toggle pause on selected session
zellij pipe --name "cc-deck:pause"

# Show help overlay
zellij pipe --name "cc-deck:help"
```

## Behavior

- All commands are handled by the active-tab plugin instance only (`is_on_active_tab()` check)
- Navigation commands are no-ops when not in navigation mode (except `nav-toggle`)
- `nav-select` exits navigation mode after selecting
- `pause` operates on the session at the current cursor position (requires navigation mode)
- All commands return `true` from `pipe()` to trigger re-render

## Demo Script Helper

```bash
# Helper function for demo scripts
cc_pipe() {
    zellij pipe --name "cc-deck:$1"
}

# Usage
cc_pipe nav-toggle
cc_pipe nav-down
cc_pipe nav-select
cc_pipe attend
```
