# Quickstart: Pause Mode & Keyboard Help

**Feature**: 014-pause-and-help

## Test Pause Mode

1. Start Zellij with cc-deck layout, create 3+ Claude sessions
2. Press `Alt+s` to enter navigation mode
3. Move cursor to a session, press `p` to pause it
4. Verify: ⏸ icon, dimmed grey name
5. Press `Alt+a` repeatedly, verify paused session is skipped
6. Move cursor back to paused session, press `p` to unpause
7. Verify: normal icon and name return

## Test Keyboard Help

1. Press `Alt+s` to enter navigation mode
2. Press `?` to show help overlay
3. Verify: all shortcuts listed (j/k, Enter, Esc, r, d, p, n, /, ?)
4. Press any key to dismiss
5. Verify: session list returns
