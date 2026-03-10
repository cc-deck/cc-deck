# Data Model: Session Save and Restore

## Entities

### Snapshot

A point-in-time capture of the cc-deck workspace state.

| Field       | Type     | Description                                              |
|-------------|----------|----------------------------------------------------------|
| version     | integer  | Schema version (currently 1)                             |
| name        | string   | Snapshot identifier (user-provided or auto-generated)    |
| saved_at    | datetime | ISO 8601 timestamp of when the snapshot was created      |
| auto_save   | boolean  | Whether this is an auto-save (subject to rotation)       |
| sessions    | list     | Ordered list of Session Entry objects (by tab position)  |

**Identity**: Unique by `name` within the sessions directory.

**Lifecycle**: Named snapshots persist indefinitely. Auto-saves rotate (keep latest 5).

### Session Entry

One Claude Code session within a snapshot.

| Field          | Type    | Description                                         |
|----------------|---------|-----------------------------------------------------|
| tab_name       | string  | Zellij tab name at save time                        |
| working_dir    | string  | Absolute path to the session's working directory    |
| session_id     | string  | Claude Code session ID (for `--resume`)             |
| display_name   | string  | User-visible name in the sidebar                    |
| paused         | boolean | Whether the session was paused                      |
| git_branch     | string  | Git branch at save time (informational, not restored) |

**Ordering**: Entries are stored in tab index order (ascending).

## File Layout

```
~/.config/cc-deck/sessions/
  auto-1.json           # Rolling auto-save (newest)
  auto-2.json           # Rolling auto-save
  ...
  auto-5.json           # Rolling auto-save (oldest)
  my-setup.json         # Named explicit save
  pre-upgrade.json      # Named explicit save
```

## State Transitions

### Auto-save Rotation

```
Hook event fires
  → Check cooldown (5 min since last auto-save?)
    → No: skip
    → Yes: dump state from plugin
      → Shift auto-{N} → auto-{N+1} (oldest deleted if > 5)
      → Write new state as auto-1.json
```

### Snapshot Selection (restore without name)

```
Scan all .json files in sessions directory
  → Parse saved_at timestamp from each
  → Select the one with the most recent timestamp
  → Use for restore
```
