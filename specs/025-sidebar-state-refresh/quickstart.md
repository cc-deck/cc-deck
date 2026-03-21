# Quickstart: 025-sidebar-state-refresh

## What Changed

After reattaching to a Zellij session, the cc-deck sidebar now preserves and displays previously known Claude Code sessions. Stale entries for panes that no longer exist are cleaned up within a few seconds.

## How to Test

### Test 1: Reattach preserves sessions

1. Start a Zellij session with the cc-deck layout:
   ```bash
   zellij --layout cc-deck
   ```
2. Open two or more Claude Code panes and wait for the sidebar to show them.
3. Detach from the session: `Ctrl+o d`
4. Reattach: `zellij attach`
5. Verify: The sidebar shows the previously known sessions within 1 second.

### Test 2: Stale session cleanup

1. Start a session with one Claude Code pane visible in the sidebar.
2. Detach: `Ctrl+o d`
3. Kill the Claude Code pane externally (e.g., close the terminal that was running it, or use `zellij action close-pane`).
4. Reattach: `zellij attach`
5. Verify: The stale entry appears briefly, then is removed within a few seconds.

### Test 3: Fresh session (regression check)

1. Clear any cached state:
   ```bash
   zellij kill-all-sessions -y
   ```
2. Start a fresh session: `zellij --layout cc-deck`
3. Verify: Sidebar shows "No Claude sessions" (unchanged behavior).
4. Open a Claude Code pane and trigger a hook event.
5. Verify: Session appears in sidebar and persists across future reattaches.
