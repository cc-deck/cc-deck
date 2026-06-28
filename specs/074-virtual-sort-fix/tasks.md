# Tasks: Virtual Sort Fix for Sidebar Session Sort

**Input**: Design documents from `/specs/074-virtual-sort-fix/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md

**Tests**: Unit tests are included as this is a bug fix that replaces existing behavior.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Add the `sort_active` flag to shared types and controller state

- [X] T001 Add `sort_active: bool` field to `ControllerState` struct in `cc-zellij-plugin/src/controller/state.rs` with default `false`, and initialize it in the `Default` impl or constructor
- [X] T002 Add `sort_active: bool` field (with `#[serde(default)]`) to `RenderPayload` struct in `cc-zellij-plugin/src/lib.rs`

---

## Phase 2: User Story 1 - Sort Sessions by Activity (Priority: P1) MVP

**Goal**: Pressing S in navigate mode displays sessions grouped by activity tier in the sidebar without moving Zellij tabs

**Independent Test**: Create sessions with mixed activity states, press S, verify sidebar shows tier-grouped order while tab positions remain unchanged

### Implementation for User Story 1

- [X] T003 [US1] Simplify `handle_sort()` in `cc-zellij-plugin/src/controller/actions.rs` to toggle `state.sort_active` and call `state.mark_render_dirty()`. Remove all `MoveTab`, `switch_tab_to_wasm`, swap sequence logic, and proactive `tab_index` updates (lines 282-398). Keep `sort_tier()` and `sort_tier_by_pane()` helper functions. Remove `move_tab_wasm()` wrapper function (lines 410-421)
- [X] T004 [US1] Modify `build_render_payload()` in `cc-zellij-plugin/src/controller/render_broadcast.rs` to sort sessions by `(sort_tier(s), tab_index)` when `state.sort_active` is true, and by `tab_index` only when false. Set `sort_active` field on the `RenderPayload` output. Import `sort_tier` from actions module (make it `pub(super)`)
- [X] T005 [US1] Update sidebar S key handler in `cc-zellij-plugin/src/sidebar_plugin/input.rs` (line 449): remove the grace period reset (lines 465-467) since no tab switching occurs during virtual sort. Keep `sort_cursor_pane_id` tracking for cursor follow

### Tests for User Story 1

- [X] T006 [P] [US1] Add unit test in `cc-zellij-plugin/src/controller/actions.rs`: verify `handle_sort()` toggles `state.sort_active` from false to true and marks render dirty
- [X] T007 [P] [US1] Add unit test in `cc-zellij-plugin/src/controller/render_broadcast.rs`: verify `build_render_payload()` sorts by tier when `sort_active` is true (Active before Inactive before Paused) and preserves relative order within tiers
- [X] T008 [P] [US1] Add unit test in `cc-zellij-plugin/src/controller/render_broadcast.rs`: verify `build_render_payload()` sorts by `tab_index` only when `sort_active` is false (existing behavior preserved)

**Checkpoint**: At this point, pressing S sorts sessions by tier in the sidebar display. No Zellij tabs are moved.

---

## Phase 3: User Story 2 - Toggle Sort On and Off (Priority: P1)

**Goal**: Pressing S again deactivates virtual sort and reverts to natural tab order

**Independent Test**: Activate sort with S, verify tier order, press S again, verify natural tab order

### Implementation for User Story 2

- [X] T009 [US2] Verify toggle behavior is already implemented by T003 (handle_sort toggles `sort_active`). No additional code needed if T003 correctly toggles the boolean. Add integration-style test only.

### Tests for User Story 2

- [X] T010 [US2] Add unit test in `cc-zellij-plugin/src/controller/actions.rs`: verify calling `handle_sort()` twice returns `sort_active` to false

**Checkpoint**: Sort toggles on/off with repeated S presses.

---

## Phase 4: User Story 3 - Cursor Follows Current Session (Priority: P1)

**Goal**: Cursor remains on the same session after sort changes display order

**Independent Test**: Position cursor on a session, press S, verify cursor still highlights the same session at its new position

### Implementation for User Story 3

- [X] T011 [US3] Verify cursor tracking is already handled by existing `sort_cursor_pane_id` mechanism in `cc-zellij-plugin/src/sidebar_plugin/mod.rs` (lines 247-251) and `track_cursor_by_pane_id()`. No changes needed since the pane_id-based tracking works regardless of physical vs virtual sort.

**Checkpoint**: Cursor follows session across sort/unsort operations.

---

## Phase 5: User Story 4 - Sort Indicator (Priority: P2)

**Goal**: Sidebar header shows a visual indicator when sort is active

**Independent Test**: Activate sort, verify indicator appears in header. Deactivate, verify indicator disappears.

### Implementation for User Story 4

- [X] T012 [US4] Modify `render_header()` in `cc-zellij-plugin/src/sidebar_plugin/render.rs` to append a sort indicator symbol (e.g., `↕`) to the header when `payload.sort_active` is true

### Tests for User Story 4

- [X] T013 [US4] Add unit test in `cc-zellij-plugin/src/sidebar_plugin/render.rs`: verify header output contains sort indicator when `payload.sort_active` is true and does not contain it when false

**Checkpoint**: Sort indicator appears/disappears with sort state.

---

## Phase 6: User Story 5 - Sort Auto-Clears on Tab Change (Priority: P2)

**Goal**: Sort deactivates when tabs are added or removed

**Independent Test**: Activate sort, open a new tab, verify sort deactivates

### Implementation for User Story 5

- [X] T014 [US5] Add `state.sort_active = false;` in `handle_tab_update()` in `cc-zellij-plugin/src/controller/events.rs` inside the `if tab_count_changed` block (around line 50)

### Tests for User Story 5

- [X] T015 [US5] Add unit test in `cc-zellij-plugin/src/controller/events.rs`: verify `handle_tab_update()` clears `sort_active` when tab count changes
- [X] T016 [P] [US5] Add unit test in `cc-zellij-plugin/src/controller/events.rs`: verify `handle_tab_update()` preserves `sort_active` when tab count is unchanged

**Checkpoint**: Sort auto-clears on tab changes.

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Cleanup, validation, documentation

- [X] T017 Run `make test` to verify all existing tests still pass with the changes
- [X] T018 Run `make lint` to verify no clippy warnings introduced
- [X] T019 Remove any dead code from the old physical sort implementation (unused imports, helper functions that are no longer called)

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Setup)**: No dependencies - start immediately
- **Phase 2 (US1 - Sort)**: Depends on Phase 1 (T001, T002)
- **Phase 3 (US2 - Toggle)**: Depends on Phase 2 (T003 implements toggle)
- **Phase 4 (US3 - Cursor)**: Depends on Phase 2 (existing mechanism, just verify)
- **Phase 5 (US4 - Indicator)**: Depends on Phase 1 (T002 adds sort_active to payload)
- **Phase 6 (US5 - Auto-clear)**: Depends on Phase 1 (T001 adds sort_active to state)
- **Phase 7 (Polish)**: Depends on all prior phases

### Parallel Opportunities

- T001 and T002 can run in parallel (different files)
- T006, T007, T008 can run in parallel (test-only tasks)
- T012 and T014 can run in parallel after Phase 1 (different files, independent stories)
- T015 and T016 can run in parallel (test-only tasks)

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (T001, T002)
2. Complete Phase 2: US1 - Sort (T003, T004, T005, T006-T008)
3. **STOP and VALIDATE**: Test sort by pressing S with mixed activity sessions
4. Sort should work and persist across sidebar refreshes

### Incremental Delivery

1. Setup + US1 (Sort) -> Core sort works (MVP)
2. Add US2 (Toggle) -> Sort toggles on/off
3. Add US3 (Cursor) -> Verify cursor tracking (no code change expected)
4. Add US4 (Indicator) -> Visual feedback in header
5. Add US5 (Auto-clear) -> Sort clears on tab changes
6. Polish -> Cleanup dead code, run full test suite

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- The core change is small (~30-40 lines modified) but spread across 7 files
- Most complexity is in T003 (removing old swap logic) and T004 (conditional sort key)
- T009 and T011 are verification tasks, not implementation tasks
