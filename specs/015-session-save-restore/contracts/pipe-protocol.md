# Pipe Protocol Extension: dump-state

## New Message: cc-deck:dump-state

**Direction**: CLI → Plugin → CLI (bidirectional)
**Purpose**: Request serialized session state from the plugin.

### Request

```bash
zellij pipe --name cc-deck:dump-state -- ""
```

- No payload required (empty string).
- Sent via CLI pipe, so `PipeSource::Cli(pipe_id)`.

### Response

Plugin responds with JSON on stdout via `cli_pipe_output()`:

```json
{
  "1": {
    "pane_id": 1,
    "session_id": "afec0bb6-...",
    "display_name": "cc-deck",
    "activity": "Idle",
    "tab_index": 0,
    "tab_name": "cc-deck",
    "working_dir": "/Users/rhuss/Development/ai/mcp/cc-deck",
    "git_branch": "main",
    "last_event_ts": 1773146811,
    "manually_renamed": false,
    "paused": false
  },
  "2": { ... }
}
```

Keys are pane IDs (as strings, from BTreeMap serialization).

### Response Rules

- Only one plugin instance responds (the one on the active tab, or the lowest plugin_id if no active tab match).
- After writing output, the plugin calls `unblock_cli_pipe_input()` to signal completion.
- Non-active-tab instances ignore the message (return false).

## CLI Command Schemas

### cc-deck session save [name]

```
Arguments:
  name    Optional snapshot name. If omitted, generates timestamp-based name.

Output:
  "Saved session snapshot: <name> (N sessions)"

Exit codes:
  0  Success
  1  Failed to query plugin state (not running inside Zellij)
  1  Failed to write state file
```

### cc-deck session restore [name]

```
Arguments:
  name    Optional snapshot name. If omitted, uses most recent snapshot.

Output (per tab):
  "Creating tab 1/5: cc-deck..."
  "Creating tab 2/5: api-server..."
  "Creating tab 3/5: frontend... (fresh start, session expired)"

Exit codes:
  0  Success (all tabs created)
  1  No snapshots found
  1  Named snapshot not found
  1  Not running inside Zellij
```

### cc-deck session list

```
Output:
  NAME              SAVED AT              SESSIONS  TYPE
  my-setup          2026-03-10 14:30:00   6         named
  auto-1            2026-03-10 14:25:00   6         auto
  auto-2            2026-03-10 14:20:00   5         auto

Exit codes:
  0  Success (even if empty list)
```

### cc-deck session remove <name>

```
Arguments:
  name    Required snapshot name to delete.

Flags:
  --all   Delete all snapshots (name argument not required).

Exit codes:
  0  Success
  1  Named snapshot not found (lists available snapshots)
```
