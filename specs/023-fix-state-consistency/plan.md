# Implementation Plan: Fix State Consistency and Add Refresh Command

**Branch**: `001-fix-state-consistency` | **Date**: 2026-03-30 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/001-fix-state-consistency/spec.md`

## Summary

Fix three state consistency bugs in the cc-deck Zellij plugin where session activity indicators, metadata, and cached state diverge across plugin instances. Add a manual refresh command ("!" in navigation mode + CLI pipe) as a safety valve. The fixes involve changing disk-only saves to broadcast-and-save, adding PID-based staleness detection to session-meta.json, pruning dead session metadata, and adding a new PipeAction::Refresh handler.

## Technical Context

**Language/Version**: Rust 2021 edition, compiled to WASM (wasm32-wasip1)
**Primary Dependencies**: zellij-tile 0.43, serde/serde_json 1.x
**Storage**: WASI `/cache/` filesystem (sessions.json, session-meta.json, zellij_pid, last_click)
**Testing**: cargo test (native), proptest for fuzz tests; manual testing in Zellij for WASM behavior
**Target Platform**: WASM (wasm32-wasip1), runs inside Zellij's wasmi interpreter
**Project Type**: Zellij plugin (WASM binary)
**Performance Goals**: Timer handler completes within 1ms; broadcast overhead negligible
**Constraints**: No std::time::Instant in WASI (use unix_now_ms()); all plugin instances share one `/cache/` directory; file I/O is not atomic
**Scale/Scope**: Typically 3-10 plugin instances (one per tab), 1-20 sessions

## Constitution Check

*No constitution configured for this project. Skipping gate.*

## Project Structure

### Documentation (this feature)

```text
specs/001-fix-state-consistency/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── checklists/
│   └── requirements.md  # Spec quality checklist
└── tasks.md             # Phase 2 output (from /speckit.tasks)
```

### Source Code (repository root)

```text
cc-deck/cc-zellij-plugin/
├── src/
│   ├── main.rs           # Plugin entry, event handling, key handling (FR-001, FR-002, FR-005, FR-006)
│   ├── sync.rs           # State sync, persistence, metadata (FR-003, FR-004, FR-009)
│   ├── pipe_handler.rs   # Pipe message parsing (FR-006)
│   ├── state.rs          # PluginState struct (no changes expected)
│   ├── session.rs        # Session/Activity types (no changes expected)
│   ├── sidebar.rs        # Sidebar rendering, help overlay (optional: update help text)
│   ├── state_machine_tests.rs  # State transition tests
│   └── fuzz_tests.rs     # Proptest fuzz tests
└── Cargo.toml
```

## Implementation Approach

### Bug Fix 1: Broadcast after cleanup (FR-001, FR-002)

**Problem**: Timer handler calls `sync::save_sessions()` after `cleanup_stale_sessions()` and `remove_dead_sessions()`. This only writes to disk. Other instances never receive the state change.

**Solution**: Replace `save_sessions()` with `broadcast_and_save()` in two locations in the Timer handler:
1. After `cleanup_stale_sessions()` returns true (Done/AgentDone to Idle transitions)
2. After `remove_dead_sessions()` returns true (post-startup-grace dead session cleanup)

`broadcast_and_save()` already exists and does exactly what we need: serializes once, broadcasts via pipe, and writes to disk.

**Files**: `main.rs` (Timer handler, ~2 lines changed)

### Bug Fix 2: Clear session-meta.json on PID mismatch (FR-003)

**Problem**: `restore_sessions()` clears `sessions.json` and `zellij_pid` when PID doesn't match, but leaves `session-meta.json` intact. Stale metadata from previous sessions bleeds through.

**Solution**: Add `std::fs::remove_file(META_PATH)` to the PID-mismatch branch in `restore_sessions()`.

**Files**: `sync.rs` (`restore_sessions()`, ~1 line added)

### Bug Fix 3: Prune dead session metadata (FR-004)

**Problem**: When sessions are removed, their entries in `session-meta.json` linger and can be applied to new sessions with reused pane IDs.

**Solution**: Add a `prune_session_meta()` function in `sync.rs` that:
1. Reads current session-meta.json
2. Retains only entries whose pane IDs exist in the provided live sessions map
3. Writes back the pruned file (or deletes the file if empty)

Call sites:
- After `remove_dead_sessions()` returns true in the Timer handler
- After `PaneClosed` removes a session in `handle_event_inner()`

**Files**: `sync.rs` (new function ~15 lines), `main.rs` (2 call sites)

### Feature: Refresh command (FR-005 through FR-009)

**Problem**: Users have no way to force a state refresh when things get corrupted.

**Solution**:
1. Add `PipeAction::Refresh` variant to `pipe_handler.rs`
2. Add `"cc-deck:refresh"` to `parse_pipe_message()` match
3. Add refresh handler in `main.rs` pipe() that:
   - Guards with `is_on_active_tab()` (FR-008)
   - Clears cache files: sessions.json, session-meta.json, last_click (FR-009)
   - Calls `broadcast_and_save()` to push current in-memory state (FR-009)
   - Sets notification "State refreshed" (FR-007)
4. Add `'!'` key handler in `handle_navigation_key()` that triggers the same refresh logic (FR-005)

**Files**: `pipe_handler.rs` (~3 lines), `main.rs` (~25 lines for handler + key binding)

### Optional: Update help overlay

Add "!" shortcut to the help overlay in `sidebar.rs` so users discover it via "?".

**Files**: `sidebar.rs` (~1 line added to help_lines array)

## Testing Strategy

### Unit Tests (cargo test, native)

1. **Existing tests**: All existing tests in `state.rs`, `sync.rs`, `pipe_handler.rs`, `session.rs`, `state_machine_tests.rs`, `fuzz_tests.rs` must continue to pass
2. **New test in sync.rs**: `test_prune_session_meta` - verify that prune removes entries for non-existent pane IDs and keeps entries for living sessions
3. **New test in sync.rs**: `test_restore_clears_meta_on_pid_mismatch` - verify that restore_sessions clears session-meta.json when PID doesn't match (requires file I/O, may need to be a doc-test or integration-style test)
4. **New test in pipe_handler.rs**: `test_parse_refresh_command` - verify `cc-deck:refresh` parses to `PipeAction::Refresh`

### Manual Testing (in Zellij)

1. Build with `cargo build --target wasm32-wasip1 --release`
2. Install plugin and start multi-tab Zellij session
3. Verify Done-to-Idle transitions appear on all tabs
4. Verify new Zellij sessions start clean (no ghost metadata)
5. Verify "!" in navigation mode shows "State refreshed" notification
6. Verify `zellij pipe cc-deck:refresh` works from CLI

## Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| broadcast_and_save increases message volume from timer | Low | Low | Only broadcasts when state actually changed (guarded by boolean return values) |
| Pruning session-meta.json during concurrent writes | Low | Low | File I/O errors are already handled gracefully; worst case: one timer tick misses a prune |
| "!" key conflicts with existing bindings | None | N/A | Verified: no existing "!" binding in navigation mode key handler |
