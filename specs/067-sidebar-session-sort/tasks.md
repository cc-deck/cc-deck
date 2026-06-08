# Tasks: Sidebar Session Sort by Activity

**Input**: Design documents from `/specs/067-sidebar-session-sort/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: Setup

**Purpose**: Add the Sort action type and WASM wrapper needed by all subsequent tasks

- [ ] T001 Add `Sort` variant to `ActionType` enum in cc-zellij-plugin/src/lib.rs
- [ ] T002 [P] Add `move_focus_or_tab_wasm` wrapper function in cc-zellij-plugin/src/wasm_compat.rs (WASM calls `move_focus_or_tab(direction)`, native stub is no-op)

**Checkpoint**: New action type compiles, WASM wrapper available

---

## Phase 2: Foundational (Sort Algorithm)

**Purpose**: Implement the core sort logic in the controller that all user stories depend on

- [ ] T003 Add `sort_tier(session: &Session) -> u8` helper function in cc-zellij-plugin/src/controller/actions.rs that returns 0 for Active (Working/Waiting and not paused), 1 for Inactive (Idle/Done/AgentDone/Init and not paused), 2 for Paused (session.paused == true)
- [ ] T004 Add `handle_sort(state: &mut ControllerState)` function in cc-zellij-plugin/src/controller/actions.rs that: (1) snapshots sessions with tab indices and tiers, (2) computes target order via stable partition by tier, (3) executes swap sequence using switch_tab_to_wasm + move_focus_or_tab_wasm, (4) broadcasts render update
- [ ] T005 Wire `ActionType::Sort` to `handle_sort` in the action dispatch match in cc-zellij-plugin/src/controller/actions.rs
- [ ] T006 Add unit tests for `sort_tier` covering all Activity variants and paused combinations in cc-zellij-plugin/src/controller/actions.rs

**Checkpoint**: Controller can receive Sort action and reorder tabs. Core sort logic tested.

---

## Phase 3: User Story 1 - Sort Sessions by Activity (Priority: P1) MVP

**Goal**: User presses S in navigate mode and sessions reorder by activity tier

**Independent Test**: Create sessions with mixed states, press S, verify tab order matches tier grouping

### Implementation for User Story 1

- [ ] T007 [US1] Add `BareKey::Char('S')` handler in `handle_navigate_key()` in cc-zellij-plugin/src/sidebar_plugin/input.rs that sends `ActionType::Sort` via `send_action`
- [ ] T008 [US1] Add unit test for S key in navigate mode dispatching Sort action in cc-zellij-plugin/src/sidebar_plugin/input.rs
- [ ] T009 [US1] Add unit test verifying S key is ignored in Passive mode (no action sent) in cc-zellij-plugin/src/sidebar_plugin/input.rs

**Checkpoint**: S key in navigate mode triggers sort, tabs reorder by activity tier

---

## Phase 4: User Story 2 - Cursor Follows Current Session (Priority: P1)

**Goal**: After sort, the navigate cursor still highlights the same session

**Independent Test**: Position cursor on a session, press S, verify cursor tracks the same pane_id

### Implementation for User Story 2

- [ ] T010 [US2] After Sort action returns, update `cursor_index` in NavigateContext to match the new position of the pane_id the cursor was on before sort. Implement in the Sort response handling path in cc-zellij-plugin/src/sidebar_plugin/input.rs (the cursor recalculation happens when the next render broadcast arrives with updated session order; the sidebar already recalculates via `preserve_cursor()` in cc-zellij-plugin/src/sidebar_plugin/mod.rs)
- [ ] T011 [US2] Add unit test verifying cursor follows session after sort reorders sessions in cc-zellij-plugin/src/sidebar_plugin/input.rs

**Checkpoint**: Cursor tracks the same session across sort-induced position changes

---

## Phase 5: User Story 3 - Help Overlay Documents Sort (Priority: P2)

**Goal**: Help overlay (? key) lists the S keybinding

**Independent Test**: Press ? in navigate mode, verify S appears in the help text

### Implementation for User Story 3

- [ ] T012 [US3] Add `" \x1b[1mS\x1b[0m      Sort by activity"` entry to `HELP_LINES` in the Actions section in cc-zellij-plugin/src/sidebar_plugin/render.rs

**Checkpoint**: Help overlay includes S keybinding

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Documentation, build verification, final cleanup

- [ ] T013 Run `make test` and `make lint` to verify all tests pass and no clippy warnings
- [ ] T014 Run `make install` to verify WASM build succeeds

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies, start immediately
- **Foundational (Phase 2)**: Depends on Phase 1 (needs Sort action type and WASM wrapper)
- **US1 (Phase 3)**: Depends on Phase 2 (needs handle_sort in controller)
- **US2 (Phase 4)**: Depends on Phase 3 (cursor follow only matters after sort works)
- **US3 (Phase 5)**: Independent of Phases 3-4 (help text is static)
- **Polish (Phase 6)**: Depends on all implementation phases

### Parallel Opportunities

- T001 and T002 can run in parallel (different files)
- T007, T008, T009 can run after T005 completes
- T012 (help text) can run in parallel with any Phase 3/4 task (different file section)

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (T001, T002)
2. Complete Phase 2: Foundational (T003-T006)
3. Complete Phase 3: User Story 1 (T007-T009)
4. **STOP and VALIDATE**: Test S key triggers sort, tabs reorder correctly
5. Continue with remaining stories

### Incremental Delivery

1. Setup + Foundational -> Sort logic ready
2. Add US1 (S keybinding) -> Sort works end-to-end (MVP!)
3. Add US2 (cursor follow) -> Polish the UX
4. Add US3 (help text) -> Discoverability
5. Polish -> Build verification

---

## Notes

- All changes are within the existing cc-zellij-plugin crate, no new files
- The `preserve_cursor()` method already exists in sidebar state and recalculates cursor position from pane_id after payload updates, so US2 may require minimal new code
- Use `make test` and `make lint` (not direct cargo commands) per constitution
