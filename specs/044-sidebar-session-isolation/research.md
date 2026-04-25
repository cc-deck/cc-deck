# Research: Sidebar Session Isolation

## R1: WASI `/cache/` Directory Listing

**Decision**: Use `std::fs::read_dir("/cache/")` for scanning state files.

**Rationale**: The WASI preview1 spec (wasi_snapshot_preview1) supports `fd_readdir`, which Rust's `std::fs::read_dir` maps to. Zellij's WASI runtime (wasmer) provides `/cache/` as a preopened directory with full read/write access. The existing code already uses `std::fs::read_to_string` and `std::fs::write` on `/cache/`, confirming WASI filesystem support.

**Alternatives**: Hardcode known file patterns and check existence individually. Rejected because it would miss files from unknown PIDs.

## R2: Process Liveness Detection in WASI

**Decision**: Use file age (mtime) as the primary cleanup criterion, not `/proc/{pid}/`.

**Rationale**: WASI sandboxes typically do not expose `/proc/`. The `std::fs::metadata` function provides `modified()` which returns the file modification time. Files older than 7 days are considered orphaned. This is conservative since Zellij sessions rarely run longer than a few days continuously.

**Alternatives**: `/proc/{pid}/stat` check. Rejected because WASI does not guarantee `/proc/` access. Zellij PID query via plugin API. Rejected because the API only returns the current PID, not arbitrary PIDs.

## R3: Pipe Message Naming Convention

**Decision**: Append PID as a colon-separated suffix: `cc-deck:sync:{pid}`, `cc-deck:request:{pid}`.

**Rationale**: Zellij's `pipe_message_to_plugin` uses the message name as a routing key. The receiver's `pipe` handler receives the full name and can extract the PID suffix. Existing message names (`cc-deck:sync`, `cc-deck:request`) gain a `:{pid}` suffix. Non-matching messages are ignored in `handle_sync`.

**Alternatives**: Embed PID in the payload JSON. Rejected because it would require parsing the payload before deciding to ignore it, adding unnecessary deserialization cost for every cross-session message.

## R4: Legacy File Migration

**Decision**: On startup, check for legacy `/cache/sessions.json` (no PID suffix). If found, rename to `/cache/sessions-{current_pid}.json`. Same for `session-meta.json` and `zellij_pid`.

**Rationale**: Users upgrading will have existing state files. Migrating them preserves sidebar state across the upgrade. The migration is a one-time rename, not a parse-and-rewrite.

**Alternatives**: Delete legacy files and start fresh. Rejected because it causes unnecessary state loss for users who only run one session at a time.

## R5: `get_plugin_ids().zellij_pid` Availability

**Decision**: Cache the PID value at plugin load time in a module-level variable.

**Rationale**: `get_plugin_ids()` is a WASM host call. While cheap, calling it on every file operation adds unnecessary overhead. The PID never changes during a plugin instance's lifetime. Caching it once in `load()` is sufficient.

**Alternatives**: Call `get_plugin_ids()` on every use. Rejected for unnecessary overhead. Use a lazy_static. Rejected because Rust's `OnceCell` in the function scope is simpler.
