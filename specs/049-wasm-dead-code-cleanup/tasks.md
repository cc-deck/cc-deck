# Tasks: WASM Plugin Dead Code Removal and Code Health

**Input**: Design documents from `/specs/049-wasm-dead-code-cleanup/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md

**Organization**: Tasks are grouped by user story. US1 (dead code removal) must complete before US2 (reorganization), and US2 before US3 (suppression removal). This is a sequential dependency chain since each story builds on the cleanup from the previous one.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: Setup (Baseline Measurement)

**Purpose**: Establish baseline metrics before any changes

- [ ] T001 Record baseline line counts: total lines in `src/` via `wc -l`, `src/main.rs` line count, and total file count. Save to `specs/049-wasm-dead-code-cleanup/baseline.txt`
- [ ] T002 Build WASM binary via `make build-wasm` and record baseline binary size. Append to `specs/049-wasm-dead-code-cleanup/baseline.txt`
- [ ] T003 Run `make test` and `make lint` to confirm baseline passes before changes

---

## Phase 2: Foundational (Audit and Relocate)

**Purpose**: Move active code out of otherwise-dead modules before any deletions. This phase MUST complete before US1 can begin.

**CRITICAL**: No deletion can begin until relocations are verified.

- [ ] T004 Audit WASM shim functions in `src/main.rs` (lines 283-361): grep each function name (`broadcast_action`, `focus_plugin`, `focus_terminal`, `switch_to_tab`, `create_new_session_tab`, `create_new_session_pane`, `close_session_pane`, `auto_rename_tab`, `new_tab_wasm`, `new_session_pane_wasm`) in `src/controller/` and `src/sidebar_plugin/` to classify as active or dead. Update the WASM Function Pair Audit table in `specs/049-wasm-dead-code-cleanup/plan.md` with the results (replace "Verify" entries with "ACTIVE - relocate" or "DEAD - delete").
- [ ] T005 [P] Relocate `HELP_LINES` constant from `src/sidebar.rs` to `src/sidebar_plugin/render.rs`. Update the `crate::sidebar::HELP_LINES` reference in `src/sidebar_plugin/render.rs` to use the local constant.
- [ ] T006 [P] Relocate sync message utilities (`is_sync_message()`, `is_request_message()`, `extract_pid_from_message_name()`) from `src/sync.rs` to `src/pipe_handler.rs`. Update the two call sites in `pipe_handler.rs` from `crate::sync::is_sync_message` / `crate::sync::is_request_message` to local function calls.
- [ ] T007 Relocate `cleanup_orphaned_state_files()` and its helpers (`extract_pid_from_filename()`, process-alive checking logic) from `src/sync.rs` to `src/controller/state.rs`. Update call sites in `src/controller/mod.rs` and `src/controller/events.rs` from `crate::sync::cleanup_orphaned_state_files()` to `self.state.cleanup_orphaned_state_files()` or `super::state::cleanup_orphaned_state_files()` as appropriate.
- [ ] T008 Relocate any WASM shim functions identified as active in T004 from `src/main.rs` to `src/wasm_compat.rs`. Update call sites in `src/controller/` and `src/sidebar_plugin/` to use `crate::wasm_compat::` prefix.
- [ ] T009 Audit legacy test files: compare test scenarios in `src/state_machine_tests.rs` (1,080 lines) against `src/sidebar_plugin/modes.rs` tests, and `src/fuzz_tests.rs` (324 lines) against controller test coverage. Document any unique scenarios that need porting. Port unique test scenarios to `src/sidebar_plugin/` or `src/controller/` test modules before proceeding.
- [ ] T010 Run `make test` and `make lint` to verify all relocations compile and pass

**Checkpoint**: All active code has been relocated out of legacy modules. Deletions can now proceed safely.

---

## Phase 3: User Story 1 - Remove Legacy Dead Code (Priority: P1) MVP

**Goal**: Delete all legacy monolithic `PluginState` code so that dead code paths can never be accidentally modified instead of the live controller/sidebar architecture.

**Independent Test**: `make test` and `make lint` pass. `rg "PluginState" src/` returns zero matches. None of the deleted files exist.

### Implementation for User Story 1

- [ ] T011 [P] [US1] Delete legacy module file `src/state.rs`
- [ ] T012 [P] [US1] Delete legacy module file `src/sidebar.rs`
- [ ] T013 [P] [US1] Delete legacy module file `src/rename.rs`
- [ ] T014 [P] [US1] Delete legacy module file `src/attend.rs`
- [ ] T015 [P] [US1] Delete legacy module file `src/sync.rs`
- [ ] T016 [P] [US1] Delete legacy module file `src/notification.rs`
- [ ] T017 [P] [US1] Delete legacy test file `src/state_machine_tests.rs`
- [ ] T018 [P] [US1] Delete legacy test file `src/fuzz_tests.rs`
- [ ] T019 [US1] Remove dead `mod` declarations from `src/main.rs`: `mod state`, `mod sidebar`, `mod rename`, `mod attend`, `mod sync`, `mod notification`, `mod state_machine_tests`, `mod fuzz_tests`
- [ ] T020 [US1] Remove dead `use` imports from `src/main.rs`: `use state::{NavigateContext, PluginMode, PluginState, SidebarMode}` and any other imports referencing deleted modules
- [ ] T021 [US1] Delete all `PluginState` impl blocks from `src/main.rs` (lines ~512-2061 including `ZellijPlugin for PluginState` impl, `handle_event`, `handle_event_inner`, all key handlers, utility methods)
- [ ] T022 [US1] Delete dead WASM shim functions from `src/main.rs`: `register_keybindings()`, `shift_variant()`, and any WASM functions identified as dead in T004. Delete associated tests (shift_variant tests around line 484). Note: FR-009 (extract keybindings to keybindings.rs) is satisfied by the existing `register_keybindings()` and `shift_variant()` in `src/controller/events.rs`. The main.rs copies are dead duplicates being deleted here. No shared `keybindings.rs` module is needed since only the controller uses keybinding registration.
- [ ] T023 [US1] Delete dead `PerfTimer` and `PerfTimerPipe` structs from `src/main.rs` if they are only used by the deleted `PluginState` code (verify with grep first; if used by controller via `crate::PerfTimer`, keep them)
- [ ] T024 [US1] Run `make test` and `make lint` to verify all deletions compile and pass
- [ ] T025 [US1] Verify zero references to `PluginState`, `PluginMode`, or legacy `SidebarMode` remain: `rg "PluginState|PluginMode" src/` should return no results (or only comments)
- [ ] T026 [US1] Verify no deleted files remain: confirm `src/state.rs`, `src/sidebar.rs`, `src/rename.rs`, `src/attend.rs`, `src/sync.rs`, `src/notification.rs`, `src/state_machine_tests.rs`, `src/fuzz_tests.rs` do not exist

**Checkpoint**: All legacy dead code removed. `make test` and `make lint` pass. No references to PluginState remain.

---

## Phase 4: User Story 2 - Reorganize main.rs Into Focused Modules (Priority: P2)

**Goal**: Reduce `main.rs` to under 500 lines containing only the `UnifiedPlugin` dispatcher, module declarations, and the `register_plugin!` macro.

**Independent Test**: `main.rs` is under 500 lines. `make test` and `make lint` pass. Each extracted module compiles independently.

### Implementation for User Story 2

- [ ] T027 [US2] Extract debug code from `src/main.rs` into new `src/debug.rs`: move `DEBUG_ENABLED` static, `debug_init()` (both WASM and native variants), `debug_log()` (both variants), and `install_panic_hook()` (both variants). Make them `pub` or `pub(crate)`. Update all call sites across `src/controller/` and `src/sidebar_plugin/` from `crate::debug_log` to `crate::debug::debug_log` (or re-export from crate root with `pub use debug::debug_log;` to minimize churn).
- [ ] T028 [US2] Move remaining WASM/native conditional function pairs from `src/main.rs` to `src/wasm_compat.rs` (any that were not already moved in T008). This may include `sanitize_voice_text()` if it belongs better in a utility module.
- [ ] T029 [US2] Clean up `src/main.rs` module declarations: ensure only active modules are declared, declarations are organized logically (shared modules, then controller/sidebar, then wasm_compat/debug)
- [ ] T030 [US2] Run `make test` and `make lint` to verify reorganization compiles and passes
- [ ] T031 [US2] Measure `src/main.rs` line count and verify it is under 500 lines (SC-001)

**Checkpoint**: main.rs is under 500 lines. All extracted modules compile and tests pass.

---

## Phase 5: User Story 3 - Consolidate Types and Remove Warning Suppressions (Priority: P3)

**Goal**: Remove the global `#![allow(dead_code, unused_imports)]` from `main.rs` and fix any resulting warnings with targeted allows, so the compiler catches real dead code going forward.

**Independent Test**: `make lint` passes with zero warnings. No global `#![allow(dead_code, unused_imports)]` exists in `src/main.rs`.

### Implementation for User Story 3

- [ ] T032 [US3] Remove `#![allow(dead_code, unused_imports)]` from line 1 of `src/main.rs`
- [ ] T033 [US3] Run `make lint` and catalog all new warnings. For each warning, determine if it is: (a) genuinely dead code to delete, (b) a WASM/native conditional item that legitimately needs `#[allow(dead_code)]`, or (c) an unused import to remove
- [ ] T034 [US3] Delete genuinely dead code identified in T033 (functions, imports, types that are truly unused)
- [ ] T035 [US3] Add targeted `#[allow(dead_code)]` attributes to WASM/native conditional code that legitimately triggers the lint (the `#[cfg(not(target_family = "wasm"))]` no-op stubs in `src/wasm_compat.rs` and `src/debug.rs`)
- [ ] T036 [US3] Remove unused imports identified in T033
- [ ] T037 [US3] Check for redundant type definitions between `src/lib.rs` and the module tree. Consolidate any duplicates to a single canonical location. (Research found zero duplicates, but verify after all the code movement.)
- [ ] T037b [US3] Check `Cargo.toml` for dev-dependencies only used by deleted test files (e.g., `proptest` used only by `fuzz_tests.rs`). Remove any orphaned dev-dependencies.
- [ ] T038 [US3] Run `make test` and `make lint` to verify zero warnings and all tests pass (SC-005)

**Checkpoint**: Zero warnings. Global suppression removed. Compiler will catch real dead code going forward.

---

## Phase 6: Polish and Verification

**Purpose**: Final measurements, validation, and documentation of results

- [ ] T039 Record final line counts: total lines in `src/`, `src/main.rs` line count, and total file count. Compare against baseline from T001.
- [ ] T040 Build WASM binary via `make build-wasm` and record final binary size. Compare against baseline from T002. Document any reduction (SC-006).
- [ ] T041 Verify success criteria: SC-001 (main.rs < 500 lines), SC-002 (total reduction >= 4,000 lines), SC-003 (zero PluginState/PluginMode references), SC-004 (make test passes), SC-005 (make lint zero warnings), SC-007 (no legacy files exist)
- [ ] T042 Run full `make test` and `make lint` one final time to confirm everything passes

---

## Dependencies and Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies, start immediately
- **Foundational (Phase 2)**: Depends on Phase 1 (baseline needed for comparison)
- **US1 (Phase 3)**: Depends on Phase 2 (relocations must complete before deletions)
- **US2 (Phase 4)**: Depends on US1 (dead code must be removed before reorganizing what remains)
- **US3 (Phase 5)**: Depends on US2 (reorganization must complete before removing the global allow, otherwise warnings would be overwhelming)
- **Polish (Phase 6)**: Depends on all user stories complete

### User Story Dependencies

- **User Story 1 (P1)**: Depends on Foundational phase. BLOCKS US2 and US3.
- **User Story 2 (P2)**: Depends on US1. BLOCKS US3.
- **User Story 3 (P3)**: Depends on US2.

This is a strictly sequential dependency chain. Each story builds on the cleanup from the previous one.

### Within Each Phase

- Tasks marked [P] can run in parallel within their phase
- T011-T018 (file deletions) can all run in parallel
- T019-T023 (main.rs cleanup) should run sequentially after file deletions
- T027-T028 (extractions) could run in parallel if they touch different code sections

### Parallel Opportunities

- Phase 2: T005, T006 can run in parallel (different target files)
- Phase 3: T011-T018 can all run in parallel (independent file deletions)
- Phase 5: T034, T035, T036 can run in parallel after T033 cataloging

---

## Parallel Example: User Story 1

```bash
# Delete all legacy files in parallel (T011-T018):
Task: "Delete src/state.rs"
Task: "Delete src/sidebar.rs"
Task: "Delete src/rename.rs"
Task: "Delete src/attend.rs"
Task: "Delete src/sync.rs"
Task: "Delete src/notification.rs"
Task: "Delete src/state_machine_tests.rs"
Task: "Delete src/fuzz_tests.rs"

# Then sequentially clean up main.rs (T019-T023):
Task: "Remove dead mod declarations from src/main.rs"
Task: "Remove dead use imports from src/main.rs"
Task: "Delete PluginState impl blocks from src/main.rs"
Task: "Delete dead WASM shim functions from src/main.rs"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Baseline measurement
2. Complete Phase 2: Audit and relocate active code
3. Complete Phase 3: Delete all dead code (US1)
4. **STOP and VALIDATE**: `make test`, `make lint`, verify no PluginState references
5. This alone delivers the primary safety benefit (no more accidentally editing dead code)

### Incremental Delivery

1. Setup + Foundational -> Relocations verified
2. US1 (Dead code removal) -> Primary safety benefit delivered (MVP)
3. US2 (Reorganize main.rs) -> Developer navigation improved
4. US3 (Remove suppression) -> Compiler catches future dead code
5. Polish -> Metrics documented, final verification

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- This is a strictly sequential feature: US1 -> US2 -> US3
- Each phase has a verification step (`make test` + `make lint`) before proceeding
- Commit after each phase for easy rollback
- The global `#![allow(dead_code)]` removal (US3) should be the LAST change, after all dead code is actually removed
