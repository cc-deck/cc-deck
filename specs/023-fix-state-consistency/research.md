# Research: Fix State Consistency and Add Refresh Command

**Date**: 2026-03-30
**Feature**: 001-fix-state-consistency

## Research Topics

### 1. broadcast_and_save vs save_sessions behavior

**Decision**: Use `broadcast_and_save()` instead of `save_sessions()` for timer-driven state changes.

**Rationale**: `broadcast_and_save()` (sync.rs:26-39) serializes sessions to JSON once, then both broadcasts via `pipe_message_to_plugin` and writes to disk. This is the same mechanism used by `sync_now()` for user-initiated actions (rename, pause, delete). Timer-driven cleanup transitions (Done-to-Idle, dead session removal) should use the same path to ensure all instances receive state changes.

**Alternatives considered**:
- `save_sessions()` only (current behavior): Insufficient. Off-screen instances never learn about transitions. They would need to re-read sessions.json on timer, but no such mechanism exists.
- Separate `broadcast_state()` + `save_sessions()`: Wasteful. Double-serializes the same data. `broadcast_and_save()` was specifically created to avoid this.

### 2. PID-based staleness detection scope

**Decision**: Clear session-meta.json alongside sessions.json and zellij_pid when PID mismatch is detected in `restore_sessions()`.

**Rationale**: The PID check in `restore_sessions()` is the single entry point for detecting a new Zellij session. All cache files should be cleaned at this point. Currently only sessions.json and zellij_pid are removed, leaving session-meta.json as a ghost data source.

**Alternatives considered**:
- Add PID checking to `apply_session_meta()`: More complex, would require reading the PID file on every timer tick. The simpler approach is to clean everything at startup.
- Ignore the issue and rely on meta_ts comparison: Unsafe. Pane IDs are reused across sessions, so a stale entry with a high meta_ts could override a fresh session's metadata.

### 3. Metadata pruning strategy

**Decision**: Read-filter-write approach in a new `prune_session_meta()` function that takes the live sessions map as reference.

**Rationale**: The simplest correct approach. Reading the file, filtering to only living pane IDs, and writing back handles all edge cases (multiple dead sessions, empty result). If no entries remain, delete the file entirely to avoid accumulating an empty JSON object.

**Alternatives considered**:
- Incremental removal (remove one entry at a time): More file I/O operations, more complex code, no benefit over read-filter-write for typical session counts (1-20).
- In-memory-only tracking (never prune file): Would still leave stale entries that `apply_session_meta()` reads on timer. The file must be pruned.

### 4. Refresh command architecture

**Decision**: Hybrid approach: clear file caches, keep in-memory state, broadcast.

**Rationale**: The active-tab instance's in-memory state is the most authoritative because it runs cleanup, receives hook events, and processes user interactions. Clearing file caches removes any corrupted persistent state, while broadcasting pushes the authoritative state to all instances.

**Alternatives considered**:
- Clear everything (including in-memory): Too destructive. Would lose all session data until hook events trickle back in. Users would see an empty sidebar temporarily.
- Re-request from all instances: If all instances have stale state, re-requesting just propagates the corruption. The active instance should be authoritative.
- File-only clear (no broadcast): Insufficient. Other instances would keep their stale in-memory state until the next sync event.

### 5. Refresh cache files to clear

**Decision**: Clear sessions.json, session-meta.json, and last_click. Do NOT clear zellij_pid.

**Rationale**:
- sessions.json: Will be rewritten by broadcast_and_save with current in-memory state
- session-meta.json: May contain stale entries; broadcast_and_save will re-persist current state
- last_click: Stale double-click state should be cleared
- zellij_pid: Must NOT be cleared. It's needed for PID-based staleness detection on next load. Clearing it would cause the next tab's plugin to think it's a new session.
