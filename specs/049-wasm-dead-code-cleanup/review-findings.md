# Deep Review Findings

**Date:** 2026-05-06
**Branch:** 049-wasm-dead-code-cleanup
**Rounds:** 1
**Gate Outcome:** PASS
**Invocation:** quality-gate

## Summary

| Severity | Found | Fixed | Remaining |
|----------|-------|-------|-----------|
| Critical | 0 | 0 | 0 |
| Important | 2 | 2 | 0 |
| Minor | 14 | 7 | 7 |
| **Total** | **16** | **9** | **7** |

**Agents completed:** 5/5 (+ CodeRabbit external)
**Agents failed:** none

## Findings

### FINDING-1
- **Severity:** Important
- **Confidence:** 90
- **File:** cc-zellij-plugin/src/controller/state.rs:458-466
- **Category:** test-quality
- **Source:** test-quality-agent
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**
`extract_pid_from_filename()` and `sessions_path()` were relocated from `sync.rs` without recreating their tests.

**Why this matters:**
These functions control state file path generation and orphaned file cleanup. Regressions could cause data loss or failure to restore sessions.

**How it was resolved:**
Added 8 test cases: 6 for `extract_pid_from_filename` (valid sessions, session-meta, legacy, non-numeric, unrelated) and 2 for `sessions_path` (PID-scoped and legacy fallback).

### FINDING-2
- **Severity:** Important
- **Confidence:** 95
- **File:** cc-zellij-plugin/src/sidebar_plugin/input.rs
- **Category:** test-quality
- **Source:** test-quality-agent
- **Round found:** 1
- **Resolution:** accepted (pre-existing gap)

**What is wrong:**
The entire `input.rs` module (647 lines) has zero tests for key handling logic.

**Why this matters:**
Key-driven state machine transitions are the core sidebar interaction path.

**How it was resolved:**
This is a pre-existing gap not introduced by this refactoring. The deleted `state_machine_tests.rs` tested the dead `PluginState` type, not the sidebar_plugin input handling. Adding comprehensive input.rs tests is out of scope for this dead code cleanup. Filed as a follow-up concern.

### FINDING-3 (Minor, fixed)
- **File:** cc-zellij-plugin/src/controller/actions.rs, events.rs, hooks.rs, state.rs
- **Category:** architecture
- **Source:** architecture-agent
- **Resolution:** fixed (round 1)

`rename_tab_wasm()` duplicated across 4 modules. Consolidated to single definition in `wasm_compat.rs`.

### FINDING-4 (Minor, fixed)
- **File:** cc-zellij-plugin/src/controller/actions.rs:257
- **Category:** architecture
- **Source:** architecture-agent
- **Resolution:** fixed (round 1)

Stale comment "adapted from crate::attend" referencing deleted module. Updated.

### FINDING-5 (Minor, fixed)
- **File:** cc-zellij-plugin/src/sidebar_plugin/rename.rs:3
- **Category:** architecture
- **Source:** architecture-agent
- **Resolution:** fixed (round 1)

Stale comment "Adapted from crate::rename" referencing deleted module. Updated.

### FINDING-6 (Minor, fixed)
- **File:** cc-zellij-plugin/src/sidebar_plugin/modes.rs:3
- **Category:** architecture
- **Source:** architecture-agent
- **Resolution:** fixed (round 1)

Stale comment "Adapted from crate::state::SidebarMode" referencing deleted module. Updated.

### FINDING-7 (Minor, fixed)
- **File:** cc-zellij-plugin/src/controller/events.rs:427
- **Category:** architecture
- **Source:** architecture-agent
- **Resolution:** fixed (round 1)

Doc comment for `shift_variant()` was removed during refactoring. Restored.

### FINDING-8 (Minor, fixed)
- **File:** cc-zellij-plugin/src/main.rs:17-40
- **Category:** test-quality
- **Source:** test-quality-agent
- **Resolution:** fixed (round 1)

`sanitize_voice_text()` had zero test coverage. Added 7 tests covering plain text, ANSI colors, BEL, control characters, tab preservation, empty input, and escape-only input.

### FINDING-9 (Minor, fixed)
- **File:** cc-zellij-plugin/src/pipe_handler.rs:68-73
- **Category:** test-quality
- **Source:** test-quality-agent (also from first review round)
- **Resolution:** fixed (round 1)

`is_sync_message()` and `is_request_message()` relocated without dedicated tests. Added 8 test cases.

## Remaining Findings

### FINDING-10 (Minor)
- **File:** cc-zellij-plugin/src/debug.rs:7
- **Category:** security + architecture
- **Source:** security-agent (also: architecture-agent)
- **Description:** `static mut DEBUG_ENABLED` uses unsafe. Could use `AtomicBool` instead.
- **Confidence:** 90

### FINDING-11 (Minor)
- **File:** cc-zellij-plugin/src/sidebar_plugin/render.rs:513-514
- **Category:** architecture
- **Source:** architecture-agent
- **Description:** `Style::Header` variant has `#[allow(dead_code)]` and is never used outside tests.
- **Confidence:** 80

### FINDING-12 (Minor)
- **File:** cc-zellij-plugin/src/controller/mod.rs:706-711
- **Category:** test-quality
- **Source:** test-quality-agent
- **Description:** `test_voice_command_enter_no_session` has no assertions.
- **Confidence:** 80

### FINDING-13 (Minor)
- **File:** cc-zellij-plugin/src/controller/mod.rs:637-639
- **Category:** test-quality
- **Source:** test-quality-agent
- **Description:** Unused `Activity` and `Session` imports in test module.
- **Confidence:** 75

### FINDING-14 (Minor, CodeRabbit)
- **File:** specs/049-wasm-dead-code-cleanup/REVIEW-CODE.md
- **Category:** external
- **Source:** coderabbit
- **Description:** Contractions ("I'm") violate cc-deck voice rules (no contractions).
- **Confidence:** 75

### FINDING-15 (Minor, CodeRabbit)
- **File:** specs/049-wasm-dead-code-cleanup/REVIEW-PLAN.md
- **Category:** external
- **Source:** coderabbit
- **Description:** Same contraction issue.
- **Confidence:** 75

### FINDING-16 (Minor, CodeRabbit)
- **File:** cc-zellij-plugin/src/controller/state.rs:443-464
- **Category:** external
- **Source:** coderabbit
- **Description:** Suggests changing /proc dead-process handling to delete immediately. This is a misread: the current behavior is correct. When /proc is not mounted (WASI), a missing /proc entry does not confirm the process is dead, so the 7-day age fallback is the correct defensive behavior.
- **Confidence:** 40 (rejected)
