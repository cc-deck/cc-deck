# Data Model: Sidebar Session Isolation

## State File Schema

No schema change. The JSON structure inside `sessions-{pid}.json` and `session-meta-{pid}.json` is identical to the current `sessions.json` and `session-meta.json`. Only the file naming convention changes.

### Current File Layout

```text
/cache/
├── sessions.json          # BTreeMap<u32, Session> (all sessions, shared)
├── session-meta.json      # BTreeMap<u32, SessionMeta> (metadata overrides)
└── zellij_pid             # Current PID for stale detection
```

### New File Layout

```text
/cache/
├── sessions-{pid}.json         # BTreeMap<u32, Session> (scoped to one Zellij session)
├── session-meta-{pid}.json     # BTreeMap<u32, SessionMeta> (scoped to one Zellij session)
└── (no more zellij_pid file)   # PID is embedded in the filename
```

The `zellij_pid` file becomes unnecessary because the PID is encoded in the state file name itself. Stale detection is replaced by orphan cleanup (file age or process liveness).

## Pipe Message Names

### Current

| Message | Direction | Purpose |
|---|---|---|
| `cc-deck:sync` | broadcast | Session state sync |
| `cc-deck:request` | broadcast | Request state from peers |

### New

| Message | Direction | Purpose |
|---|---|---|
| `cc-deck:sync:{pid}` | broadcast | Session state sync (receivers filter by PID) |
| `cc-deck:request:{pid}` | broadcast | Request state (only same-PID responds) |

## Entities

### Session (unchanged)

```rust
struct Session {
    pane_id: u32,
    display_name: String,
    activity: Activity,
    working_dir: Option<String>,
    git_branch: Option<String>,
    tab_index: Option<usize>,
    tab_name: Option<String>,
    last_event_ts: u64,
    manually_renamed: bool,
    paused: bool,
    meta_ts: u64,
    done_attended: bool,
    pending_tab_rename: bool,
}
```

### SessionMeta (unchanged)

```rust
struct SessionMeta {
    display_name: String,
    manually_renamed: bool,
    paused: bool,
    meta_ts: u64,
}
```

No fields are added or removed. The isolation is purely at the storage and transport layer.
