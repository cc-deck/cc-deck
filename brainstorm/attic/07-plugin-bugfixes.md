# Brainstorm 07: Plugin Bugfixes and Improvements

Date: 2026-03-05
Status: complete

## Problem

The cc-deck Zellij plugin has several issues that prevent it from working as intended:

1. The session picker can't render because the plugin pane is `size=1` (one row)
2. Claude doesn't auto-start in new session tabs
3. Manually started Claude sessions aren't tracked by the plugin
4. Tab titles are static (`cc-0`, `cc-1`) and don't reflect session status or project name
5. The smoke test is manual and doesn't cover all plugin features

## Issue 1: Picker Shows as Split Pane (Not Popup)

### Root cause

The plugin renders the picker inside its own pane via ANSI text. The layout gives the plugin `size=1 borderless=true`, so `render()` checks `rows > 1` and skips the picker entirely. The picker was designed for a multi-row pane but the production layout uses a single-row status bar.

### Solution: Floating plugin pane

When the picker opens, spawn a temporary floating plugin pane with enough rows to display the session list. Close the floating pane when a selection is made or the picker is dismissed.

Implementation approach:
- On `open_picker`: use `open_plugin_pane_floating` (if available in zellij-tile 0.43) to spawn a second instance of the plugin in floating mode, or use `toggle_pane_embed_or_floating` to temporarily float the status bar pane
- Alternative: if the Zellij API doesn't support floating plugin panes well, use `request_plugin_pane_size` to temporarily expand the status bar to 10+ rows while the picker is active, then shrink back on close
- The floating pane approach is preferred because it doesn't displace the terminal content

### Open questions
- Does `open_plugin_pane_floating` exist in zellij-tile 0.43?
- Can a second instance of the same plugin communicate with the first?
- Would `toggle_pane_embed_or_floating` work on the status bar pane itself?

## Issue 2: Claude Doesn't Auto-Start

### Root cause

`new_tab(name, cwd)` creates a tab with a plain shell. The original code used `new_tabs_with_layout` with `command="claude"` but that API silently fails in Zellij 0.43 WASM plugins.

### Solution: open_command_pane in new tab

After creating the tab with `new_tab()`, use `open_command_pane_in_place` to replace the shell pane with a Claude command pane.

Implementation approach:
- Call `new_tab(Some(&name), Some(&cwd))`
- Listen for `TabUpdate` or `PaneUpdate` to detect the new tab's pane ID
- Call `open_command_pane_in_place` with `command="claude"` targeting that pane
- Handle the case where `claude` is not on PATH (show error in status bar)

Alternative: Use `open_command_pane` directly instead of `new_tab()`, but this creates a pane in the current tab rather than a new tab. Would need to figure out how to move it to a new tab, or accept the split-pane layout.

## Issue 3: Manual Sessions Not Tracked

### Root cause

The plugin only tracks sessions created via `CommandPaneOpened`, which fires when the plugin calls `new_tab()` with session context. Panes opened manually by the user bypass this registration.

### Solution: Track via PaneUpdate

Listen to `PaneUpdate` events and detect when a pane's title or command contains "claude". Auto-register it as a tracked session.

Implementation approach:
- In the `PaneUpdate` handler, iterate over pane info
- For each terminal pane not already tracked, check if its title contains "claude" (Claude Code sets the terminal title)
- If detected, create a new `Session` and register it
- Use the pane's cwd for git repo detection and auto-naming
- Mark it as an "adopted" session (optional: different visual indicator)

Considerations:
- Need to handle pane title changes (Claude might not set the title immediately)
- Debounce detection to avoid registering the same pane multiple times
- What happens when a tracked pane closes? Already handled by `PaneClosed` event

## Issue 4: Tab Titles Should Reflect Status

### Root cause

Tab titles are set once at creation (`cc-0`, `cc-1`) and never updated. The plugin's session display names and status indicators only appear in the status bar, not the Zellij tab bar.

### Solution: Status + project name in tab title

Update tab titles dynamically using `rename_tab()` whenever session status or name changes.

Format: `<status_icon> <project_name>`
- `⚡ my-project` (working)
- `⏳ my-project` (waiting for input)
- `✓ my-project` (done)
- `💤 my-project` (idle)
- `? my-project` (unknown)

Implementation approach:
- Add a `tab_index` field to the `Session` struct (detected from `TabUpdate` events)
- On every status change (pipe message received), call `rename_tab(tab_index, &new_title)`
- On session rename (manual rename via plugin), also update the tab title
- On git detection complete (async `RunCommandResult`), update both display name and tab title

Considerations:
- `rename_tab` takes a tab position (0-indexed), not a tab name. Need to track which tab index each session lives in
- Tab indices can shift when tabs are closed/reordered. Listen to `TabUpdate` to keep mapping current
- Status emoji might not render in all terminals. Consider using ASCII fallbacks: `[W]`, `[?]`, `[I]`, `[D]`

## Issue 5: Automated Smoke Tests

### Root cause

The current `smoke_test.sh` is interactive (requires manual key presses and visual inspection). It can't run in CI or verify behavior automatically.

### Solution: Fully automated test script

Create a test script that launches Zellij in headless mode, sends pipe commands, queries state via `zellij action` commands, and reports pass/fail.

Implementation approach:
- Use `zellij --layout cc-deck --session test-session` to start a session
- Send commands via `zellij pipe --name <command> --session test-session`
- Query state via `zellij action query-tab-names` (if available) or check `zellij list-sessions`
- Verify:
  - Plugin loads (status bar renders)
  - `new_session` creates a tab (tab count increases)
  - `switch_session_N` changes focused tab
  - `close_session` reduces tab count
  - `plugin status` reports correct state
  - `plugin remove` cleans up files
  - `plugin install` restores files
- Use `zellij kill-session test-session` to clean up

Considerations:
- Zellij might not have a headless mode or programmatic state query API
- May need to use `zellij action dump-layout` or `zellij action list-clients` for state verification
- Timing: need to wait for async operations (tab creation, plugin load) before asserting
- CI environment needs Zellij installed

## Summary of Decisions

| Issue | Solution | Complexity |
|-------|----------|------------|
| Picker rendering | Floating plugin pane | Medium (API research needed) |
| Claude auto-start | open_command_pane in new tab | Medium (async pane detection) |
| Manual session tracking | Detect via PaneUpdate title | Low-Medium |
| Tab title updates | rename_tab on status change | Low |
| Automated tests | Headless Zellij test script | Medium-High |

## Recommended Implementation Order

1. **Tab titles** (lowest risk, immediate visual improvement)
2. **Manual session tracking** (enables testing other features)
3. **Claude auto-start** (requires pane detection from #2)
4. **Picker as floating pane** (needs API research)
5. **Automated tests** (validates all above fixes)
