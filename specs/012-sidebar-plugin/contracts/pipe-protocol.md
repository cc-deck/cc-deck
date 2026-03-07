# Pipe Protocol Contract: cc-deck

**Date**: 2026-03-07
**Feature**: 012-sidebar-plugin

## Overview

cc-deck uses Zellij's pipe message system for two purposes:
1. CLI-to-plugin communication (hook events from `cc-deck hook`)
2. Plugin-to-plugin synchronization (state sync between sidebar instances)

All pipe messages use the broadcast form (`pipe_message_to_plugin(MessageToPlugin::new(...))`) to reach all instances. Targeted messages are avoided because they can create new plugin instances.

## Message: cc-deck:hook

**Direction**: CLI -> all plugin instances
**Trigger**: `cc-deck hook` command invoked by Claude Code hook
**Transport**: `zellij pipe --name "cc-deck:hook" --payload '<json>'`

**Payload schema:**
```json
{
  "session_id": "string (optional)",
  "pane_id": 42,
  "hook_event": "PreToolUse",
  "tool_name": "Bash (optional)",
  "cwd": "/path/to/dir (optional)"
}
```

**Handling**: Each sidebar instance updates its local session state based on the hook_event, then broadcasts via cc-deck:sync.

## Message: cc-deck:sync

**Direction**: Plugin instance -> all other instances
**Trigger**: After any state change (hook event received, session added/removed)

**Payload schema:**
```json
{
  "42": {
    "pane_id": 42,
    "session_id": "abc123",
    "display_name": "api-server",
    "activity": "Working",
    "tab_index": 2,
    "working_dir": "/home/user/api-server",
    "git_branch": "main",
    "last_event_ts": 1741363200,
    "manually_renamed": false
  }
}
```

**Handling**: Receiving instances merge incoming state. For each pane_id, the instance with the newer `last_event_ts` wins. Tab-local info (tab_index, tab_name) is refreshed from local pane_to_tab mapping.

## Message: cc-deck:request

**Direction**: Plugin instance -> all other instances
**Trigger**: On plugin load (new tab created), after permissions granted
**Payload**: None

**Handling**: All existing instances respond by broadcasting cc-deck:sync with their current state.

## Message: cc-deck:attend

**Direction**: Keybinding -> plugin instance
**Trigger**: User presses the attend keyboard shortcut
**Payload**: None

**Handling**: The receiving instance scans sessions for Waiting state, finds the oldest waiting session, and calls `switch_tab_to()` to focus its tab. If no session is waiting, displays an inline notification.

## Message: cc-deck:rename

**Direction**: Keybinding -> plugin instance
**Trigger**: User presses the rename keyboard shortcut
**Payload**: None (initiates rename mode on the currently active session)

**Handling**: The receiving instance enters rename mode for the session matching the active tab. After the user confirms, it updates the display_name and calls `rename_tab()`.

## Message: cc-deck:new

**Direction**: Keybinding -> plugin instance
**Trigger**: User presses the new session keyboard shortcut or clicks [+] in sidebar
**Payload**: None

**Handling**: The receiving instance calls `open_command_pane()` to create a new tab running the `claude` command in the current working directory.
