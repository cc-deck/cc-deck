# Quickstart: Multiplayer Focus Modes Validation

**Date**: 2026-07-22
**Feature**: 083-multiplayer-focus-modes

## Prerequisites

- Zellij installed (0.43+)
- cc-deck plugin built and installed: `make install`
- Two terminal windows (or tmux panes) for simulating multiplayer

## Setup

1. Start a Zellij session with cc-deck:
   ```bash
   zellij --session test-multi
   ```

2. Start at least 2 Claude Code sessions (or any terminal sessions) so the sidebar has entries.

3. Open a second terminal and attach to the same session:
   ```bash
   zellij attach test-multi
   ```

4. Both terminals should show the cc-deck sidebar with session entries.

## Validation Scenarios

### V1: Independent Focus (Bug Fix)

**Steps**:
1. In the second terminal (client 2), click a session in the sidebar
2. Observe client 2's terminal switches to the clicked session
3. Observe client 1's terminal is **unchanged**

**Expected**: Each client controls its own focus independently. No cross-client focus interference.

**Previous behavior (bug)**: Clicking in client 2's sidebar would switch client 1's terminal.

### V2: Presence Indicators

**Steps**:
1. With both clients connected, have client 2 click on a specific session
2. Look at client 1's sidebar

**Expected**: Client 1's sidebar shows a colored block (inverted space) right-aligned on the session line that client 2 is focused on. The color matches Zellij's multiplayer user colors (same as the tab bar indicators).

### V3: Indicator Movement

**Steps**:
1. Have client 2 switch to a different session
2. Observe client 1's sidebar

**Expected**: The colored indicator moves from the old session line to the new one within one render cycle.

### V4: Stable Auto-Sort Zones

**Steps**:
1. Note the active-zone order on both clients
2. Click sessions in reverse order on either client
3. Change their activity states while keeping them unpaused
4. Pause one session, then reactivate it by clicking it

**Expected**: Clicks and activity changes never reorder the active zone. The paused session leaves that zone and is appended when reactivated.

### V5: Single-Client Regression

**Steps**:
1. Detach the second terminal: `Ctrl+o, d` (or close it)
2. In the remaining single-client session, click sessions, use keyboard navigation (Alt-s, j/k, Enter)

**Expected**: All functionality works identically to before. No presence indicators shown. No behavioral differences.

### V6: Keyboard Shortcuts (Primary Client)

**Steps**:
1. With both clients connected, press Alt-s on client 1 (primary)
2. Navigate with j/k, press Enter to select

**Expected**: Alt-s activates navigate mode on client 1. Selection switches client 1's focus only.

**Known limitation**: Alt-s pressed on client 2 activates navigate mode on client 1, not client 2. This is documented.

### V7: Client Disconnect Cleanup

**Steps**:
1. With both clients connected and client 2 focused on a session (indicator visible on client 1)
2. Detach client 2
3. Observe client 1's sidebar

**Expected**: The presence indicator for client 2 disappears after the next render cycle.

## Debug Logging

Enable debug logging to trace focus-report flow:

```bash
touch ~/Library/Caches/org.Zellij-Contributors.Zellij/file:~/.config/zellij/plugins/cc_deck.wasm/plugin_cache/debug_enabled
```

Then restart Zellij. Truncate log before reproducing:

```bash
: > ~/Library/Caches/org.Zellij-Contributors.Zellij/file:~/.config/zellij/plugins/cc_deck.wasm/plugin_cache/debug.log
```

Watch for:
- `SIDEBAR FOCUS-REPORT:` lines showing local focus operations
- `CTRL FOCUS-REPORT:` lines showing controller receiving focus data
- `CTRL RENDER:` lines showing other_client_focus data in payload
