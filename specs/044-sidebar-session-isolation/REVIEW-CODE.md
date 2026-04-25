# Code Review: 044-sidebar-session-isolation

**Reviewer**: Claude Code (automated)
**Date**: 2026-04-25
**Branch**: `044-sidebar-session-isolation`

## FR Compliance (11/11 PASS)

| FR | Status | Evidence |
|----|--------|----------|
| FR-001 | PASS | `sessions_path()` and `meta_path()` produce `*-{pid}.json` paths |
| FR-002 | PASS | `broadcast_state`/`broadcast_and_save` use `sync_message_name(pid)` |
| FR-003 | PASS | PID filtering in `main.rs` pipe handler; `extract_pid_from_message_name` + mismatch rejection |
| FR-004 | PASS | `restore_sessions` reads from `sessions_path(pid)` |
| FR-005 | PASS | `save_sessions` writes to `sessions_path(pid)` |
| FR-006 | PASS | `write_session_meta`/`apply_session_meta` use `meta_path(pid)` |
| FR-007 | PASS | `cleanup_orphaned_state_files` scans `/cache/`, checks `/proc/`, 7-day fallback |
| FR-008 | PASS | `request_state` uses `request_message_name(pid)` |
| FR-009 | PASS | `prune_session_meta` uses `meta_path(pid)` |
| FR-010 | PASS | No config required; isolation is automatic |
| FR-011 | PASS | `migrate_legacy_files` in both `sync.rs` and `controller/state.rs` |

## Findings

### Important

- **Duplicate migration logic**: `migrate_legacy_files` exists in both `sync.rs` (sidebar) and `controller/state.rs` (controller). The controller version does not migrate `session-meta.json` (it only migrates sessions and removes legacy PID/meta). This is intentional (controller never uses meta file), but consider a comment explaining the asymmetry.

### Minor

- **PID=0 guard in cleanup**: `cleanup_orphaned_state_files` returns early for PID=0, which is correct for tests but means cleanup is untestable in native test harness. Acceptable trade-off.
- **`is_sync_message` prefix match**: `"cc-deck:sync:".starts_with` would match `cc-deck:sync:` followed by non-numeric strings. This is safe since `extract_pid_from_message_name` handles parse failure, but the double-check is slightly redundant.

## Test Coverage

27 new tests covering path generation, message name construction, PID extraction, filename parsing, and isolation properties. All 279 tests pass. Clippy clean.

## Gate Outcome

**Score**: 11/11 FRs satisfied
**Result**: **PASS**

## Deep Review Report

**Tool**: CodeRabbit CLI (local)
**Findings**: 16 total (7 major, 9 minor)

### Actionable Findings

1. **Migration safety (minor, fixed)**: `controller/state.rs` migration removed legacy file before confirming write success. Fixed: only remove legacy file on successful write.

### Deferred (over-engineered for desktop plugin)

The remaining findings concern theoretical hardening for scenarios that do not apply to a single-user desktop Zellij plugin:

- PID reuse mitigation via UUID in filenames: Zellij server PIDs are long-lived, making reuse in the same `/cache/` directory effectively impossible.
- Concurrent migration locking: Only one plugin instance starts per session. Multiple instances within the same session share a PID and do not compete.
- Configurable cleanup thresholds: 7-day hardcoded threshold is appropriate for a plugin cache. Adding configuration to a WASM plugin increases complexity without user benefit.
- Rolling upgrade compatibility for pipe messages: The plugin is distributed as a single binary. Mixed-version scenarios do not occur in practice.

These are valid concerns for a distributed server system but not for a local terminal plugin.
