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

- [x] T001 Record baseline line counts: total lines in `src/` via `wc -l`, `src/main.rs` line count, and total file count. Save to `specs/049-wasm-dead-code-cleanup/baseline.txt`
- [x] T002 Build WASM binary via `make build-wasm` and record baseline binary size. Append to `specs/049-wasm-dead-code-cleanup/baseline.txt`
- [x] T003 Run `make test` and `make lint` to confirm baseline passes before changes

---

## Phase 2: Foundational (Audit and Relocate)

**Purpose**: Move active code out of otherwise-dead modules before any deletions. This phase MUST complete before US1 can begin.

**CRITICAL**: No deletion can begin until relocations are verified.

- [x] T004 Audit WASM shim functions (ALL DEAD - controller/sidebar have own copies) in `src/main.rs` (lines 283-361): grep each function name (`broadcast_action`, `focus_plugin`, `focus_terminal`, `switch_to_tab`, `create_new_session_tab`, `create_new_session_pane`, `close_session_pane`, `auto_rename_tab`, `new_tab_wasm`, `new_session_pane_wasm`) in `src/controller/` and `src/sidebar_plugin/` to classify as active or dead. Update the WASM Function Pair Audit table in `specs/049-wasm-dead-code-cleanup/plan.md` with the results (replace "Verify" entries with "ACTIVE - relocate" or "DEAD - delete").
- [x] T005 [P] Relocate `HELP_LINES` constant from `src/sidebar.rs` to `src/sidebar_plugin/render.rs`. Update the `crate::sidebar::HELP_LINES` reference in `src/sidebar_plugin/render.rs` to use the local constant.
- [x] T006 [P] Relocate sync message utilities (`is_sync_message()`, `is_request_message()`, `extract_pid_from_message_name()`) from `src/sync.rs` to `src/pipe_handler.rs`. Update the two call sites in `pipe_handler.rs` from `crate::sync::is_sync_message` / `crate::sync::is_request_message` to local function calls.
- [x] T007 Relocate `cleanup_orphaned_state_files()` and its helpers (`extract_pid_from_filename()`, process-alive checking logic) from `src/sync.rs` to `src/controller/state.rs`. Update call sites in `src/controller/mod.rs` and `src/controller/events.rs` from `crate::sync::cleanup_orphaned_state_files()` to `self.state.cleanup_orphaned_state_files()` or `super::state::cleanup_orphaned_state_files()` as appropriate.
- [x] T008 No-op: all WASM shims are dead (T004 audit found none active) from `src/main.rs` to `src/wasm_compat.rs`. Update call sites in `src/controller/` and `src/sidebar_plugin/` to use `crate::wasm_compat::` prefix.
- [x] T009 Audit legacy test files (no unique scenarios to port): compare test scenarios in `src/state_machine_tests.rs` (1,080 lines) against `src/sidebar_plugin/modes.rs` tests, and `src/fuzz_tests.rs` (324 lines) against controller test coverage. Document any unique scenarios that need porting. Port unique test scenarios to `src/sidebar_plugin/` or `src/controller/` test modules before proceeding.
- [x] T010 Run `make test` and `make lint` to verify all relocations compile and pass (296/296 pass)

**Checkpoint**: All active code has been relocated out of legacy modules. Deletions can now proceed safely.

---

## Phase 3: User Story 1 - Remove Legacy Dead Code (Priority: P1) MVP

**Goal**: Delete all legacy monolithic `PluginState` code so that dead code paths can never be accidentally modified instead of the live controller/sidebar architecture.

**Independent Test**: `make test` and `make lint` pass. `rg "PluginState" src/` returns zero matches. None of the deleted files exist.

### Implementation for User Story 1

- [x] T011 [P] [US1] Delete legacy module file `src/state.rs`
- [x] T012 [P] [US1] Delete legacy module file `src/sidebar.rs`
- [x] T013 [P] [US1] Delete legacy module file `src/rename.rs`
- [x] T014 [P] [US1] Delete legacy module file `src/attend.rs`
- [x] T015 [P] [US1] Delete legacy module file `src/sync.rs`
- [x] T016 [P] [US1] Delete legacy module file `src/notification.rs`
- [x] T017 [P] [US1] Delete legacy test file `src/state_machine_tests.rs`
- [x] T018 [P] [US1] Delete legacy test file `src/fuzz_tests.rs`
- [x] T019 [US1] Remove dead `mod` declarations from `src/main.rs`
- [x] T020 [US1] Remove dead `use` imports from `src/main.rs`
- [x] T021 [US1] Delete all `PluginState` impl blocks from `src/main.rs`
- [x] T022 [US1] Delete dead WASM shim functions from `src/main.rs` (register_keybindings, shift_variant, broadcast_action, focus_plugin, focus_terminal, switch_to_tab, create_new_session_tab, create_new_session_pane, close_session_pane, auto_rename_tab, PerfTimer, PerfTimerPipe, shared_last_click functions, shift_variant_tests)
- [x] T023 [US1] Delete dead PerfTimer and PerfTimerPipe (confirmed not used by active code)
- [x] T024 [US1] Tests pass (147/147), lint has same 3 pre-existing warnings
- [x] T025 [US1] Zero PluginState/PluginMode references confirmed
- [x] T026 [US1] All 8 legacy files confirmed deleted

**Checkpoint**: All legacy dead code removed. `make test` and `make lint` pass. No references to PluginState remain.

---

## Phase 4: User Story 2 - Reorganize main.rs Into Focused Modules (Priority: P2)

**Goal**: Reduce `main.rs` to under 500 lines containing only the `UnifiedPlugin` dispatcher, module declarations, and the `register_plugin!` macro.

**Independent Test**: `main.rs` is under 500 lines. `make test` and `make lint` pass. Each extracted module compiles independently.

### Implementation for User Story 2

- [x] T027 [US2] Extracted debug code to `src/debug.rs`, re-exported via `pub use` to avoid call site churn
- [x] T028 [US2] No remaining WASM pairs to move (all dead shims deleted in US1). sanitize_voice_text kept in main.rs (single user)
- [x] T029 [US2] main.rs module declarations organized: shared modules, controller/sidebar, wasm_compat/debug
- [x] T030 [US2] Tests pass (147/147), lint has same 3 pre-existing warnings
- [x] T031 [US2] main.rs is 147 lines (target was <500)

**Checkpoint**: main.rs is under 500 lines. All extracted modules compile and tests pass.

---

## Phase 5: User Story 3 - Consolidate Types and Remove Warning Suppressions (Priority: P3)

**Goal**: Remove the global `#![allow(dead_code, unused_imports)]` from `main.rs` and fix any resulting warnings with targeted allows, so the compiler catches real dead code going forward.

**Independent Test**: `make lint` passes with zero warnings. No global `#![allow(dead_code, unused_imports)]` exists in `src/main.rs`.

### Implementation for User Story 3

- [x] T032 [US3] Removed `#![allow(dead_code, unused_imports)]` from `src/main.rs`
- [x] T033 [US3] Cataloged 17 warnings: unused imports, dead code, WASM stubs, clippy issues
- [x] T034 [US3] Deleted: extract_pid_from_message_name, render_unavailable, PerfTracker::record/count, scroll_offset field, filtered_session_count, active_tab_index, set_notification, is_capturing_input, rename_state_mut
- [x] T035 [US3] Added targeted `#[allow(dead_code)]` to: DEBUG_ENABLED, cli_pipe_output_wasm, unblock_cli_pipe_input_wasm, shift_variant (all WASM/native conditional)
- [x] T036 [US3] Removed: `use super::modes::SidebarMode` in render.rs, added `#[allow(unused_imports)]` for zellij_tile::prelude in sidebar_registry.rs (needed for WASM)
- [x] T037 [US3] No redundant types found (confirmed)
- [x] T037b [US3] Removed orphaned `proptest` dev-dependency from Cargo.toml
- [x] T038 [US3] 143 tests pass, zero lint warnings. Also fixed 2x map_or->is_some_and and large_enum_variant clippy issues

**Checkpoint**: Zero warnings. Global suppression removed. Compiler will catch real dead code going forward.

---

## Phase 6: Polish and Verification

**Purpose**: Final measurements, validation, and documentation of results

- [x] T039 Final: 8,019 lines (was 14,436), main.rs 146 lines (was 2,083), 22 files (was 29)
- [x] T040 WASM binary: 898 KB (unchanged, LTO already eliminated dead code)
- [x] T041 All success criteria verified: SC-001 PASS (146 < 500), SC-002 PASS (6,417 >= 4,000), SC-003 PASS (0 refs), SC-004 PASS (143 tests), SC-005 PASS (0 warnings), SC-007 PASS (all deleted)
- [x] T042 Final make test-rust and make lint-rust both pass

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
