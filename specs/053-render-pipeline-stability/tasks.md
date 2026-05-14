# Tasks: Render Pipeline Stability and CPU Optimization

**Input**: Design documents from `specs/053-render-pipeline-stability/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, quickstart.md

**Tests**: Included. Constitution requires tests for all new code.

**Organization**: Tasks grouped by user story for independent implementation and testing.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: Setup

**Purpose**: Shared infrastructure changes needed before any user story work

- [X] T001 [P] Add `ControllerPing`, `ControllerPong`, and `RenderRequest` variants to PipeAction enum and parse_pipe_message in `cc-zellij-plugin/src/pipe_handler.rs`
- [X] T002 [P] Add `disabled: bool` field to ControllerState in `cc-zellij-plugin/src/controller/state.rs`
- [X] T003 [P] Add `render_request_sent: bool` and `ticks_since_init: u8` fields to SidebarState in `cc-zellij-plugin/src/sidebar_plugin/state.rs`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Defensive guard that prevents the phantom controller from processing messages. MUST be complete before story-specific work begins.

- [X] T004 Add plugin_id guard in controller pipe() handler: skip all processing when `!state.permissions_granted` in `cc-zellij-plugin/src/controller/mod.rs`
- [X] T005 Add plugin_id guard in controller update() handler: skip Timer and other events when `state.disabled` in `cc-zellij-plugin/src/controller/mod.rs`
- [X] T006 Add plugin_id prefix to all debug_log calls in controller for disambiguation in `cc-zellij-plugin/src/debug.rs` (add a `debug_log_with_id` variant or prefix in existing calls)
- [X] T007 Add unit tests for defensive guard: controller with permissions_granted=false ignores pipe messages, disabled controller ignores timer events in `cc-zellij-plugin/src/controller/integration_tests.rs`

**Checkpoint**: Defensive guard active. Phantom controller no longer processes messages. Flickering should be reduced.

---

## Phase 3: User Story 1 - Stable Session Display (Priority: P1)

**Goal**: Eliminate flickering, blinking, and session count oscillation by implementing the startup probe protocol and ensuring only one controller is active.

**Independent Test**: Open 14 tabs with Claude Code sessions. Observe sidebar for 60 seconds in steady state. No visual changes should occur. Debug logs show a single plugin_id across all CTRL entries.

### Tests for User Story 1

- [X] T008 [P] [US1] Unit test: lower plugin_id wins startup probe (controller with higher id self-disables) in `cc-zellij-plugin/src/controller/integration_tests.rs`
- [X] T009 [P] [US1] Unit test: disabled controller does not broadcast renders or process hooks in `cc-zellij-plugin/src/controller/integration_tests.rs`

### Implementation for User Story 1

- [X] T010 [US1] Implement startup probe: on PermissionRequestResult, broadcast `cc-deck:controller-ping` with own plugin_id in `cc-zellij-plugin/src/controller/mod.rs`
- [X] T011 [US1] Handle incoming `cc-deck:controller-ping`: compare plugin_ids, respond with pong, self-disable if higher in `cc-zellij-plugin/src/controller/mod.rs`
- [X] T012 [US1] Handle incoming `cc-deck:controller-pong`: self-disable if own plugin_id is higher than responder in `cc-zellij-plugin/src/controller/mod.rs`
- [X] T013 [US1] When disabled, stop timer rescheduling and clear render_dirty to halt all broadcasts in `cc-zellij-plugin/src/controller/mod.rs`

**Checkpoint**: Only one controller active. Session flickering eliminated. SC-001 and SC-005 should pass.

---

## Phase 4: User Story 2a - Low CPU Usage at Idle (Priority: P2)

**Goal**: Reduce CPU from 100%+ to under 30% by making render triggers conditional.

**Independent Test**: Open 14 tabs with idle sessions. Measure CPU via `top` over 30 seconds. Must stay under 30%.

### Tests for User Story 2a

- [X] T014 [P] [US2] Unit test: handle_tab_update with no actual changes does not mark render dirty in `cc-zellij-plugin/src/controller/events.rs` (test module)
- [X] T015 [P] [US2] Unit test: handle_tab_update with tab count change marks dirty in `cc-zellij-plugin/src/controller/events.rs` (test module)
- [X] T016 [P] [US2] Unit test: handle_tab_update with active tab change marks dirty in `cc-zellij-plugin/src/controller/events.rs` (test module)

### Implementation for User Story 2a

- [X] T017 [US2] Replace unconditional `state.mark_render_dirty()` at end of handle_tab_update with conditional logic: mark dirty only when active tab changed, tab count changed, dead sessions removed, or stale sessions transitioned in `cc-zellij-plugin/src/controller/events.rs`

**Checkpoint**: Idle render broadcasts eliminated. CPU should drop significantly. SC-002 and SC-003 should pass.

---

## Phase 5: User Story 2b - Reliable First Render for New Sidebars (Priority: P2)

**Goal**: New sidebars receive their first render payload within 3 seconds. No permanent "No Claude sessions" display.

**Independent Test**: With 8 active sessions, open a new tab. Sidebar shows all sessions within 3 seconds.

### Tests for User Story 2b

- [X] T018 [P] [US3] Unit test: discover_sidebars_from_manifest returns newly registered plugin_ids in `cc-zellij-plugin/src/controller/sidebar_registry.rs` (test module)
- [X] T019 [P] [US3] Unit test: sidebar sends render-request after 3 ticks with no payload received in `cc-zellij-plugin/src/sidebar_plugin/integration_tests.rs`
- [X] T020 [P] [US3] Unit test: sidebar does not send render-request if payload already received in `cc-zellij-plugin/src/sidebar_plugin/integration_tests.rs`
- [X] T021 [P] [US3] Unit test: sidebar does not send render-request more than once in `cc-zellij-plugin/src/sidebar_plugin/integration_tests.rs`

### Implementation for User Story 2b

- [X] T022 [US3] Modify `discover_sidebars_from_manifest` to return `Vec<u32>` of newly registered sidebar plugin_ids in `cc-zellij-plugin/src/controller/sidebar_registry.rs`
- [X] T023 [US3] In handle_pane_update, after discover call, send targeted render to each newly discovered sidebar in `cc-zellij-plugin/src/controller/events.rs`
- [X] T024 [US3] Subscribe sidebar to Timer events and implement tick counter: increment ticks_since_init on each tick. After first render payload received (initialized=true), unsubscribe from Timer to avoid per-second overhead in `cc-zellij-plugin/src/sidebar_plugin/mod.rs`
- [X] T025 [US3] Implement one-shot render request: if ticks_since_init >= 3 and !initialized and !render_request_sent and controller_plugin_id is Some, send `cc-deck:render-request` to controller. If controller_plugin_id is None (SidebarInit not yet received), wait for next tick rather than sending to unknown target in `cc-zellij-plugin/src/sidebar_plugin/mod.rs`
- [X] T026 [US3] Handle `cc-deck:render-request` in controller: send targeted render payload to requesting sidebar in `cc-zellij-plugin/src/controller/mod.rs`
- [X] T027 [US3] Display "Connecting..." in sidebar render when cached_payload is None (instead of "No Claude sessions") in `cc-zellij-plugin/src/sidebar_plugin/render.rs`

**Checkpoint**: New sidebars always receive initial render. SC-004 should pass.

---

## Phase 6: User Story 4 - Performance Visibility (Priority: P3)

**Goal**: Profiling data covers render pipeline metrics. Debug logging is optimized.

**Independent Test**: Enable profiling, run 14 tabs for 60 seconds, verify perf.csv includes render broadcast count, serialization time, and pipe delivery count.

### Tests for User Story 4

- [X] T028 [P] [US4] Unit test: debug_log buffers lines, debug_flush writes and clears buffer in `cc-zellij-plugin/src/debug.rs` (test module)
- [X] T029 [P] [US4] Unit test: render:broadcast and render:pipe_send perf events recorded during broadcast in `cc-zellij-plugin/src/controller/render_broadcast.rs` (test module)

### Implementation for User Story 4

- [X] T030 [US4] Implement buffered debug logging using `thread_local! { static LOG_BUFFER: RefCell<Vec<String>> }` (avoids unsafe static mut): accumulate lines in buffer, flush on timer tick or when buffer exceeds 50 lines in `cc-zellij-plugin/src/debug.rs`
- [X] T031 [US4] Add `debug_flush()` call to controller timer handler in `cc-zellij-plugin/src/controller/events.rs`
- [X] T032 [US4] Add `render:broadcast` perf event with serialization timing in `broadcast_render` in `cc-zellij-plugin/src/controller/render_broadcast.rs`
- [X] T033 [US4] Add `render:pipe_send` perf event counting per-sidebar pipe deliveries in `broadcast_render` in `cc-zellij-plugin/src/controller/render_broadcast.rs`
- [X] T034 [US4] Add `render:skipped` perf event in `flush_render` when render_dirty is false in `cc-zellij-plugin/src/controller/render_broadcast.rs`

**Checkpoint**: Profiling covers render pipeline. SC-006 and SC-007 should pass.

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Integration verification and cleanup

- [X] T035 Run `make test` and verify all existing + new tests pass
- [X] T036 Run `make lint` and fix any clippy warnings
- [ ] T037 Manual verification: open 14 tabs, check for zero flickering over 60 seconds (SC-001)
- [ ] T038 Manual verification: measure CPU with 14 idle tabs, confirm under 30% (SC-002)
- [ ] T039 Manual verification: open new tab with 8 sessions, confirm sidebar populates within 3 seconds (SC-004)
- [ ] T040 Manual verification: check debug.log for single controller plugin_id (SC-005)
- [ ] T041 Run quickstart.md validation checklist

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies, can start immediately. All tasks are parallel.
- **Foundational (Phase 2)**: Depends on T001-T003 from Setup. BLOCKS all user stories.
- **US1 (Phase 3)**: Depends on Phase 2 completion. Can start after T007.
- **US2a (Phase 4)**: Depends on Phase 2 completion. Independent of US1.
- **US2b (Phase 5)**: Depends on Phase 2 (T001, T003). Independent of US1 and US2a.
- **US4 (Phase 6)**: Depends on Phase 2 completion. Independent of other stories.
- **Polish (Phase 7)**: Depends on all desired user stories being complete.

### User Story Dependencies

- **US1 (Stable Display)**: Depends on Foundational only. No cross-story dependencies.
- **US2a (Low CPU)**: Depends on Foundational only. No cross-story dependencies.
- **US2b (First Render)**: Depends on Foundational only. No cross-story dependencies.
- **US4 (Performance Visibility)**: Depends on Foundational only. No cross-story dependencies.

### Within Each User Story

- Tests written first (fail before implementation)
- Shared module changes before controller/sidebar changes
- Controller changes before sidebar changes (for bootstrapping)
- Unit tests before integration verification

### Parallel Opportunities

- T001, T002, T003 (all Setup tasks) run in parallel
- T008, T009 (US1 tests) run in parallel
- T014, T015, T016 (US2a tests) run in parallel
- T018, T019, T020, T021 (US2b tests) run in parallel
- T028, T029 (US4 tests) run in parallel
- US1, US2a, US2b, US4 can all run in parallel after Phase 2

---

## Parallel Example: Setup Phase

```text
# All setup tasks modify different files:
T001: pipe_handler.rs (new enum variants)
T002: controller/state.rs (new field)
T003: sidebar_plugin/state.rs (new fields)
```

## Parallel Example: User Stories After Foundational

```text
# All user stories are independent after Phase 2:
US1 (T008-T013): controller/mod.rs (startup probe)
US2a (T014-T017): controller/events.rs (conditional dirty)
US2b (T018-T027): sidebar_registry.rs + sidebar_plugin/mod.rs (bootstrapping)
US4 (T028-T034): debug.rs + render_broadcast.rs (profiling)
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (T001-T003)
2. Complete Phase 2: Foundational (T004-T007)
3. Complete Phase 3: User Story 1 (T008-T013)
4. **STOP and VALIDATE**: Flickering should be eliminated
5. Measure CPU impact of single-controller fix alone

### Incremental Delivery

1. Setup + Foundational -> Defensive guard active
2. Add US1 -> Single controller enforced -> Flickering gone (MVP!)
3. Add US2a -> Conditional renders -> CPU drops to target
4. Add US2b -> Push-on-discovery -> New tabs bootstrap reliably
5. Add US4 -> Profiling -> Regression detection possible
6. Polish -> Manual verification against all success criteria

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story is independently completable and testable
- Constitution requires tests: all phases include test tasks
- Build with `make test` and `make lint`, never direct cargo commands
