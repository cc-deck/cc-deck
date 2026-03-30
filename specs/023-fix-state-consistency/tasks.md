# Tasks: Fix State Consistency and Add Refresh Command

**Input**: Design documents from `/specs/001-fix-state-consistency/`
**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3, US4)
- Include exact file paths in descriptions

---

## Phase 1: Setup

**Purpose**: Verify existing test suite passes before making changes

- [x] T001 Run `cargo test` in cc-deck/cc-zellij-plugin/ to verify all existing tests pass before changes

**Checkpoint**: Baseline test suite green, ready to begin implementation

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Add the new PipeAction variant and prune function needed by multiple user stories

**Note**: No test tasks generated (not explicitly requested in spec). Existing test suite provides baseline coverage.

- [x] T002 Add `Refresh` variant to `PipeAction` enum in cc-deck/cc-zellij-plugin/src/pipe_handler.rs
- [x] T003 Add `"cc-deck:refresh"` match arm to `parse_pipe_message()` in cc-deck/cc-zellij-plugin/src/pipe_handler.rs
- [x] T004 [P] Add `prune_session_meta()` function in cc-deck/cc-zellij-plugin/src/sync.rs that reads session-meta.json, retains only entries for pane IDs present in the provided sessions map, writes back (or deletes if empty)
- [x] T005 [P] Add unit test `test_parse_refresh_command` in cc-deck/cc-zellij-plugin/src/pipe_handler.rs verifying `cc-deck:refresh` parses to `PipeAction::Refresh`
- [x] T006 [P] Add unit test `test_prune_session_meta` in cc-deck/cc-zellij-plugin/src/sync.rs verifying prune removes entries for non-existent pane IDs and keeps entries for living sessions

**Checkpoint**: Foundation ready. New PipeAction variant exists, prune function available, tests pass.

---

## Phase 3: User Story 1 - Activity indicators consistent across tabs (Priority: P1) MVP

**Goal**: Done/AgentDone to Idle transitions propagate to ALL sidebar instances, not just disk.

**Independent Test**: Start two sessions on different tabs. Let one finish. Wait 30s. Switch tabs and verify checkmark is gone everywhere.

### Implementation for User Story 1

- [x] T007 [US1] In Timer handler in cc-deck/cc-zellij-plugin/src/main.rs, change `sync::save_sessions(&self.sessions)` to `sync::broadcast_and_save(self)` after `cleanup_stale_sessions()` returns true (~line 1259)
- [x] T008 [US1] In Timer handler in cc-deck/cc-zellij-plugin/src/main.rs, change `sync::save_sessions(&self.sessions)` to `sync::broadcast_and_save(self)` after `remove_dead_sessions()` returns true in the post-startup-grace block (~line 1255)
- [x] T009 [US1] Run `cargo test` in cc-deck/cc-zellij-plugin/ to verify no regressions

**Checkpoint**: User Story 1 complete. Activity state transitions now broadcast to all instances.

---

## Phase 4: User Story 2 - No ghost state from previous sessions (Priority: P1)

**Goal**: Clear session-meta.json alongside other cache files when PID mismatch detected.

**Independent Test**: Start a session, rename sessions, kill Zellij. Start new session. Verify no stale names appear.

### Implementation for User Story 2

- [x] T010 [US2] In `restore_sessions()` in cc-deck/cc-zellij-plugin/src/sync.rs, add `let _ = std::fs::remove_file(META_PATH);` to the PID-mismatch branch (alongside existing sessions.json and PID file removal, ~line 133)
- [x] T011 [US2] Run `cargo test` in cc-deck/cc-zellij-plugin/ to verify no regressions

**Checkpoint**: User Story 2 complete. New Zellij sessions start with a clean slate.

---

## Phase 5: User Story 3 - Dead session metadata pruned (Priority: P2)

**Goal**: Metadata entries for closed sessions are pruned to prevent stale metadata applying to reused pane IDs.

**Independent Test**: Create session, rename it, close tab. Create new tab. Verify new session doesn't inherit old name.

### Implementation for User Story 3

- [x] T012 [US3] In Timer handler in cc-deck/cc-zellij-plugin/src/main.rs, call `sync::prune_session_meta(&self.sessions)` after `remove_dead_sessions()` returns true (both post-startup-grace and regular PaneUpdate paths)
- [x] T013 [US3] In `handle_event_inner()` PaneClosed handler in cc-deck/cc-zellij-plugin/src/main.rs, call `sync::prune_session_meta(&self.sessions)` after session removal (~line 1394)
- [x] T014 [US3] Run `cargo test` in cc-deck/cc-zellij-plugin/ to verify no regressions

**Checkpoint**: User Story 3 complete. Dead session metadata no longer accumulates.

---

## Phase 6: User Story 4 - Force-refresh command (Priority: P2)

**Goal**: Users can trigger manual state refresh via "!" in navigation mode or `zellij pipe cc-deck:refresh`.

**Independent Test**: Corrupt session-meta.json. Press "!" in navigation mode. Verify sidebar refreshes with clean state and notification appears.

### Implementation for User Story 4

- [x] T015 [US4] Add `PipeAction::Refresh` handler in `pipe()` method in cc-deck/cc-zellij-plugin/src/main.rs: guard with `is_on_active_tab()`, clear sessions.json/session-meta.json/last_click via `std::fs::remove_file`, call `sync::broadcast_and_save(self)`, set notification "State refreshed"
- [x] T016 [US4] Add `BareKey::Char('!')` match arm in `handle_navigation_key()` in cc-deck/cc-zellij-plugin/src/main.rs that triggers the same refresh logic as the pipe handler
- [x] T017 [US4] Run `cargo test` in cc-deck/cc-zellij-plugin/ to verify no regressions

**Checkpoint**: User Story 4 complete. Users can force-refresh state via keyboard or CLI.

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Help text update and final validation

- [x] T018 [P] Add `" \x1b[1m!\x1b[0m      Refresh state",` line to `help_lines` array in `render_help_overlay()` in cc-deck/cc-zellij-plugin/src/sidebar.rs
- [x] T019 Run full `cargo test` in cc-deck/cc-zellij-plugin/ to confirm all tests pass
- [x] T020 Build release WASM binary with `cargo build --target wasm32-wasip1 --release` in cc-deck/cc-zellij-plugin/

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies, start immediately
- **Foundational (Phase 2)**: Depends on Phase 1
- **US1 (Phase 3)**: Depends on Phase 2
- **US2 (Phase 4)**: Depends on Phase 2 (independent of US1)
- **US3 (Phase 5)**: Depends on Phase 2 (uses prune function from T004)
- **US4 (Phase 6)**: Depends on Phase 2 (uses PipeAction::Refresh from T002/T003)
- **Polish (Phase 7)**: Depends on all user stories complete

### User Story Dependencies

- **US1 (P1)**: Independent. Only touches Timer handler save calls.
- **US2 (P1)**: Independent. Only touches restore_sessions().
- **US3 (P2)**: Independent. Only adds prune calls to removal paths.
- **US4 (P2)**: Independent. Adds new pipe action handler and key binding.

### Parallel Opportunities

- T004, T005, T006 can run in parallel (different functions/files)
- US1 and US2 can run in parallel (touch different code paths)
- US3 and US4 can run in parallel (touch different code paths)
- T018 can run in parallel with any user story task

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (baseline tests)
2. Complete Phase 2: Foundational (PipeAction variant + prune function)
3. Complete Phase 3: US1 (broadcast fix)
4. **STOP and VALIDATE**: Build WASM, test in multi-tab Zellij session

### Incremental Delivery

1. Setup + Foundational -> Foundation ready
2. US1 (broadcast fix) -> Most visible bug fixed
3. US2 (PID meta cleanup) -> Cross-session ghost state eliminated
4. US3 (prune dead metadata) -> Long-session metadata accumulation fixed
5. US4 (refresh command) -> User safety valve added
6. Polish -> Help text updated, final build

---

## Notes

- All changes are in the cc-deck/cc-zellij-plugin/src/ directory
- No new files created; all changes modify existing files
- T007 and T008 are nearly identical changes (same pattern, different call sites)
- T015 and T016 share refresh logic; consider extracting a `perform_refresh()` helper method on PluginState
