# 09: Cross-Session Visibility

**Status**: Idea
**Depends on**: 012-sidebar-plugin (MVP complete)
**Priority**: Future enhancement

## Problem

Each Zellij session is isolated. A user running multiple Zellij sessions (e.g., one per project) can only see Claude Code activity within the current session. There's no unified view of all running Claude agents across sessions.

This matters when you have several projects running Claude agents simultaneously and want to know which ones need attention without switching between terminal windows.

## Proposed Solution

Show all Claude Code sessions across Zellij sessions in the sidebar, grouped by Zellij session name. The current session's sessions appear first, with other sessions below in a collapsible section.

### Architecture

```
Claude Code hooks (all sessions)
        |
        v
  cc-deck hook (Go CLI)
        |
        v
  Shared state file                    Zellij pipe (current session only)
  ~/.local/state/cc-deck/sessions.json    |
        |                                 v
        +-----> Plugin reads on timer --> Merge into sidebar state
```

**Two data paths for session state:**

1. **Pipe messages** (current approach): Real-time, low-latency updates for the current Zellij session via `cc-deck:hook` pipe
2. **Shared state file** (new): Periodic file reads for cross-session visibility. The `cc-deck hook` CLI writes all session state here, keyed by Zellij session name

### Shared State File Format

```json
{
  "sessions": {
    "stellar-bee": {
      "last_updated": 1772924629,
      "entries": {
        "0": {
          "pane_id": 0,
          "display_name": "cc-deck",
          "activity": "Working",
          "git_branch": "012-sidebar-plugin",
          "last_event_ts": 1772924629
        }
      }
    },
    "cosmic-fox": {
      "last_updated": 1772924500,
      "entries": {
        "0": {
          "display_name": "cc-rosa",
          "activity": "Waiting",
          "git_branch": "main",
          "last_event_ts": 1772924500
        }
      }
    }
  }
}
```

### Sidebar Rendering

```
CC-DECK
────────────────────
● cc-deck          3m
  012-sidebar-plugin
◆ cc-rosa
  main

── cosmic-fox ──────
⏳ ml-pipeline     12m
   feature/train
● data-api
  fix/auth
```

Current session's sessions at the top (no header needed). Other sessions shown below with a session name separator line. Activity indicators work the same across sessions.

### Click Behavior

- **Same session**: `focus_terminal_pane()` + `switch_tab_to()` (existing behavior)
- **Cross session**: `run_command("zellij", ["action", "switch-session", "--name", "<session>"])` to switch the terminal to that Zellij session, but this changes the entire terminal view (not just a tab switch)

### Implementation Considerations

1. **File locking**: Multiple `cc-deck hook` processes (from different sessions) write to the same file concurrently. Use atomic write (write temp file, rename) to avoid corruption.

2. **Stale session cleanup**: Include `last_updated` timestamp per Zellij session. The plugin or hook CLI removes entries older than a threshold (e.g., 5 minutes without updates).

3. **Zellij session name**: Available via the `ZELLIJ_SESSION_NAME` env var in hook processes. The hook CLI passes it along when writing to the shared state file.

4. **Poll frequency**: Read the shared state file on the existing timer tick (every 1-2 seconds). This is acceptable latency for cross-session visibility since same-session updates remain real-time via pipe.

5. **Collapsible sections**: Could allow clicking the session header to collapse/expand that section, keeping the sidebar compact when there are many sessions.

## What This Enables

- **Unified attention view**: See which agents across all projects need human input
- **Cross-session attend**: Jump to the oldest waiting session across all Zellij sessions
- **Project awareness**: Know what all your agents are doing without window-switching

## Open Questions

- Should cross-session click switch the terminal window, or just highlight the entry?
- Should the shared state file use a file watcher instead of polling?
- How to handle Zellij sessions on different machines (not in scope, but worth noting)?
- Should this integrate with the "deck" concept from the original brainstorm (08)?
