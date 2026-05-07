# Tasks: Plugin Integration and E2E Testing

**Input**: Design documents from `/specs/052-plugin-integration-e2e-testing/`
**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, quickstart.md

**Tests**: This feature IS an integration test suite, so "test tasks" are the implementation itself.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Add test state accessors and module declarations needed by all integration tests

- [ ] T001 Add `#[cfg(test)]` test_state() accessor to SidebarRendererPlugin in cc-zellij-plugin/src/sidebar_plugin/mod.rs
- [ ] T002 Add `#[cfg(test)]` test_state() accessor to ControllerPlugin in cc-zellij-plugin/src/controller/mod.rs
- [ ] T003 [P] Add `#[cfg(test)] mod integration_tests;` declaration to cc-zellij-plugin/src/sidebar_plugin/mod.rs
- [ ] T004 [P] Add `#[cfg(test)] mod integration_tests;` declaration to cc-zellij-plugin/src/controller/mod.rs

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: PipeMessage construction helpers and plugin setup helpers shared across all user stories

**WARNING**: No user story work can begin until this phase is complete

- [ ] T005 Add make_pipe() helper that constructs a PipeMessage with plugin source in cc-zellij-plugin/src/sidebar_plugin/test_helpers.rs
- [ ] T006 Add make_hook_pipe() and make_hook_pipe_with_cwd() helpers for CLI-sourced hook events in cc-zellij-plugin/src/sidebar_plugin/test_helpers.rs
- [ ] T007 Add make_action_pipe() helper for sidebar action messages in cc-zellij-plugin/src/sidebar_plugin/test_helpers.rs
- [ ] T008 Add make_hello_pipe() and make_init_pipe() helpers for discovery protocol messages in cc-zellij-plugin/src/sidebar_plugin/test_helpers.rs
- [ ] T009 Add setup_sidebar() and setup_controller() convenience functions that create, load, and grant permissions in cc-zellij-plugin/src/sidebar_plugin/test_helpers.rs
- [ ] T010 Add setup_sidebar_with_tab() helper that also sends SidebarInit in cc-zellij-plugin/src/sidebar_plugin/test_helpers.rs

**Checkpoint**: Foundation ready - all integration test helper functions available

---

## Phase 3: User Story 1 - Sidebar Receives Render Payload (Priority: P1)

**Goal**: Verify the full pipe-to-state chain for render payloads sent to the sidebar plugin

**Independent Test**: Run `cargo test sidebar_plugin::integration_tests` and verify all sidebar render tests pass

### Implementation for User Story 1

- [ ] T011 [P] [US1] Create cc-zellij-plugin/src/sidebar_plugin/integration_tests.rs with test_sidebar_load_and_permission_grant
- [ ] T012 [P] [US1] Add test_sidebar_receives_render_payload verifying 3 sessions appear in state in cc-zellij-plugin/src/sidebar_plugin/integration_tests.rs
- [ ] T013 [P] [US1] Add test_sidebar_payload_replacement verifying second payload replaces first in cc-zellij-plugin/src/sidebar_plugin/integration_tests.rs
- [ ] T014 [P] [US1] Add test_sidebar_render_before_permissions verifying payload is not processed before grant in cc-zellij-plugin/src/sidebar_plugin/integration_tests.rs

**Checkpoint**: Sidebar render payload pipeline fully tested

---

## Phase 4: User Story 2 - Controller Processes Hook Events (Priority: P1)

**Goal**: Verify hook events from CLI create and update session state in the controller

**Independent Test**: Run `cargo test controller::integration_tests` and verify all controller hook tests pass

### Implementation for User Story 2

- [ ] T015 [P] [US2] Create cc-zellij-plugin/src/controller/integration_tests.rs with test_controller_load_and_permission_grant
- [ ] T016 [P] [US2] Add test_controller_hook_session_start verifying new session creation with Init activity in cc-zellij-plugin/src/controller/integration_tests.rs
- [ ] T017 [P] [US2] Add test_controller_hook_pre_tool_use verifying activity transitions to Working in cc-zellij-plugin/src/controller/integration_tests.rs
- [ ] T018 [P] [US2] Add test_controller_hook_stop verifying activity transitions to Done in cc-zellij-plugin/src/controller/integration_tests.rs

**Checkpoint**: Controller hook event pipeline fully tested

---

## Phase 5: User Story 3 - Sidebar-Controller Discovery Protocol (Priority: P2)

**Goal**: Verify SidebarHello/SidebarInit handshake works through the pipe interface

**Independent Test**: Run `cargo test integration_tests::test_controller_sidebar_hello` and `cargo test integration_tests::test_sidebar_init`

### Implementation for User Story 3

- [ ] T019 [P] [US3] Add test_controller_sidebar_hello_registration verifying sidebar registered in registry in cc-zellij-plugin/src/controller/integration_tests.rs
- [ ] T020 [P] [US3] Add test_sidebar_init_assigns_tab verifying tab_index stored after SidebarInit in cc-zellij-plugin/src/sidebar_plugin/integration_tests.rs

**Checkpoint**: Discovery protocol verified end-to-end

---

## Phase 6: User Story 4 - Sidebar Action Message Dispatch (Priority: P2)

**Goal**: Verify action messages from sidebar are processed correctly by controller

**Independent Test**: Run action-related integration tests and verify controller state changes

### Implementation for User Story 4

- [ ] T021 [P] [US4] Add test_controller_action_pause verifying paused state toggles in cc-zellij-plugin/src/controller/integration_tests.rs
- [ ] T022 [P] [US4] Add test_controller_action_attend verifying done_attended flag set in cc-zellij-plugin/src/controller/integration_tests.rs

**Checkpoint**: Action dispatch verified for pause and attend operations

---

## Phase 7: User Story 5 - Permission Grant and Deferred Event Replay (Priority: P3)

**Goal**: Verify events received before permissions are granted are queued and replayed

**Independent Test**: Send events before permissions, grant permissions, verify replayed state

### Implementation for User Story 5

- [ ] T023 [US5] Add test_controller_deferred_events verifying queued events are replayed on permission grant in cc-zellij-plugin/src/controller/integration_tests.rs

**Checkpoint**: Permission deferral and replay verified

---

## Phase 8: Error Handling and Protocol Roundtrips

**Purpose**: Verify robustness for malformed input and protocol serialization fidelity

- [ ] T024 [P] [US1] Add test_sidebar_malformed_pipe_message verifying no panic on invalid JSON in cc-zellij-plugin/src/sidebar_plugin/integration_tests.rs
- [ ] T025 [P] [US2] Add test_controller_malformed_hook_payload verifying no panic on invalid JSON in cc-zellij-plugin/src/controller/integration_tests.rs
- [ ] T026 [P] [US1] Add test_sidebar_empty_payload verifying empty session list handled in cc-zellij-plugin/src/sidebar_plugin/integration_tests.rs
- [ ] T027 [P] [US3] Add test_render_payload_roundtrip_through_pipe verifying serialization fidelity in cc-zellij-plugin/src/sidebar_plugin/integration_tests.rs
- [ ] T028 [P] [US4] Add test_action_message_roundtrip_through_pipe verifying serialization fidelity in cc-zellij-plugin/src/controller/integration_tests.rs

---

## Phase 9: User Story 6 - Sidebar Mode Transitions (Priority: P2)

**Purpose**: Verify sidebar mode changes triggered through the pipe interface

- [ ] T029 [US1] Add test_sidebar_navigate_mode_via_pipe verifying mode transitions to Navigate in cc-zellij-plugin/src/sidebar_plugin/integration_tests.rs

---

## Phase 10: Polish & Cross-Cutting Concerns

**Purpose**: Documentation and validation

- [ ] T030 Update README.md to document the integration test suite
- [ ] T031 Run `make test` to verify all integration tests pass alongside existing tests
- [ ] T032 Run `make lint` to verify no clippy warnings introduced

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies, can start immediately
- **Foundational (Phase 2)**: Depends on Setup (T001-T004) completion, BLOCKS all user stories
- **User Stories (Phases 3-7, 9)**: All depend on Foundational phase completion
  - US1 (sidebar render) and US2 (controller hooks) can proceed in parallel
  - US3 (discovery) can proceed in parallel with US1/US2
  - US4 (actions) can proceed in parallel with US1/US2/US3
  - US5 (deferred events) can proceed in parallel
- **Error Handling (Phase 8)**: Can proceed in parallel with user stories (different test functions)
- **Polish (Phase 10)**: Depends on all implementation phases being complete

### Within Each User Story

- All test tasks within a story marked [P] can run in parallel (different test functions in same file, no state shared)
- Story complete when all its tests pass

### Parallel Opportunities

- T001 and T002 can run in parallel (different plugin files)
- T003 and T004 can run in parallel (different plugin files)
- T005-T010 are sequential (same file: test_helpers.rs)
- T011-T014 are parallel (same file but independent test functions)
- T015-T018 are parallel (same file but independent test functions)
- Phases 3-9 can all proceed in parallel after Phase 2 completes

---

## Parallel Example: User Story 1

```bash
# Launch all sidebar integration tests together (independent functions):
Task: "test_sidebar_load_and_permission_grant in integration_tests.rs"
Task: "test_sidebar_receives_render_payload in integration_tests.rs"
Task: "test_sidebar_payload_replacement in integration_tests.rs"
Task: "test_sidebar_render_before_permissions in integration_tests.rs"
```

---

## Implementation Strategy

### MVP First (User Stories 1 + 2)

1. Complete Phase 1: Setup (accessors + module declarations)
2. Complete Phase 2: Foundational (helper functions)
3. Complete Phase 3: US1 - Sidebar render payload tests
4. Complete Phase 4: US2 - Controller hook event tests
5. **STOP and VALIDATE**: Run `make test` to verify 8+ integration tests pass
6. This delivers the highest-value coverage (the two P1 stories)

### Incremental Delivery

1. Complete Setup + Foundational -> Helper infrastructure ready
2. Add US1 + US2 -> Core pipeline coverage (MVP!)
3. Add US3 + US4 -> Protocol and action coverage
4. Add US5 + Error handling -> Robustness coverage
5. Polish -> Documentation and final validation

---

## Notes

- [P] tasks = different files or independent test functions, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story is independently completable and testable
- Commit after each phase completion
- Stop at any checkpoint to validate story independently
