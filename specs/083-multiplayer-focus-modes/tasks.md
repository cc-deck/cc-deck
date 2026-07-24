# Tasks: Multiplayer Focus Modes

**Input**: Design documents from `specs/083-multiplayer-focus-modes/`

**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md, contracts/focus-report.md

**Tests**: Unit tests included (constitution principle I requires tests for all new code).

**Organization**: Tasks grouped by user story for independent implementation and testing.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (US1, US2, US3)
- Exact file paths included in descriptions

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Add new types, pipe message support, and event subscription needed by all user stories

- [x] T001 Add `FocusReport`, `ClientFocusEntry`, `OtherClientPresence`, `ClientIndicator` structs and `other_client_focus: Vec<OtherClientPresence>` field with `#[serde(default)]` to `RenderPayload` in cc-zellij-plugin/src/lib.rs
- [x] T002 Add `FocusReport` variant to `PipeAction` enum and parse `cc-deck:focus-report` in `parse_pipe_message()` in cc-zellij-plugin/src/pipe_handler.rs
- [x] T003 [P] Add `client_focus: HashMap<u16, ClientFocusEntry>` and `multiplayer_colors: Option<Vec<(u8, u8, u8)>>` fields to `ControllerState` in cc-zellij-plugin/src/controller/state.rs
- [x] T004 [P] Add `local_activation_order: Vec<u32>` field to `SidebarState` in cc-zellij-plugin/src/sidebar_plugin/state.rs
- [x] T005 Add `EventType::ModeUpdate` to controller event subscriptions in cc-zellij-plugin/src/controller/mod.rs
- [x] T006 Add `ModeUpdate` event handler to extract `multiplayer_user_colors` from `ModeInfo.style.colors` and store in `multiplayer_colors` in cc-zellij-plugin/src/controller/events.rs

**Checkpoint**: All new types compiled, existing tests pass (`make test`)

---

## Phase 2: User Story 1 - Independent Focus Control (Priority: P1)

**Goal**: Fix the bug where client 2's sidebar clicks control client 1's terminal. Each sidebar handles focus locally.

**Independent Test**: Attach two clients. Click a session on client 2. Verify only client 2 switches. Verify client 1 is unchanged.

### Tests for User Story 1

- [x] T007 [P] [US1] Unit test: `FocusReport` serialization/deserialization round-trip in cc-zellij-plugin/src/lib.rs
- [x] T008 [P] [US1] Unit test: `parse_pipe_message("cc-deck:focus-report", ...)` returns `PipeAction::FocusReport` in cc-zellij-plugin/src/pipe_handler.rs
- [x] T009 [P] [US1] Unit test: controller `handle_focus_report` updates `client_focus` map and session activity in cc-zellij-plugin/src/controller/actions.rs

### Implementation for User Story 1

- [x] T010 [US1] Modify sidebar click handler to call `focus_terminal_pane_wasm()` and `switch_tab_to_wasm()` directly from sidebar instead of sending Switch action to controller, in cc-zellij-plugin/src/sidebar_plugin/input.rs (left click handler around line 214)
- [x] T011 [US1] Add `send_focus_report()` helper to sidebar that sends `cc-deck:focus-report` pipe message to controller (using `pipe_message_to_plugin`) in cc-zellij-plugin/src/sidebar_plugin/input.rs
- [x] T012 [US1] Call `send_focus_report()` after local focus switch in click handler and navigate-Enter handler in cc-zellij-plugin/src/sidebar_plugin/input.rs
- [x] T013 [US1] Add `handle_focus_report()` function to controller that deserializes payload, validates client_id against connected_clients, updates `client_focus` map, updates session activity tracking, and triggers render broadcast in cc-zellij-plugin/src/controller/actions.rs
- [x] T014 [US1] Route `cc-deck:focus-report` pipe messages to `handle_focus_report()` in controller pipe handling in cc-zellij-plugin/src/controller/mod.rs
- [x] T015 [US1] Add debug_log calls for focus-report send (sidebar) and receive (controller) following existing logging patterns

**Checkpoint**: Independent focus works. Client 2 clicks only affect client 2. `make test` passes.

---

## Phase 3: User Story 2 - Presence Indicators (Priority: P1)

**Goal**: Show right-aligned colored blocks on session lines indicating which other clients are focused on each session.

**Independent Test**: Two clients connected. Client 2 clicks a session. Client 1's sidebar shows a colored indicator on that session line.

### Tests for User Story 2

- [x] T016 [P] [US2] Unit test: `build_render_payload` includes `other_client_focus` data filtered by target client_id in cc-zellij-plugin/src/controller/render_broadcast.rs
- [x] T017 [P] [US2] Unit test: `ModeUpdate` handler correctly extracts multiplayer colors from `PaletteColor::Rgb` and `PaletteColor::EightBit` variants in cc-zellij-plugin/src/controller/events.rs
- [x] T018 [P] [US2] Unit test: presence indicator rendering produces correct ANSI escape sequences for 0, 1, and 2 other-client indicators in cc-zellij-plugin/src/sidebar_plugin/render.rs

### Implementation for User Story 2

- [x] T019 [US2] Modify `build_render_payload()` to accept target `client_id` parameter and populate `other_client_focus` from `client_focus` map, excluding the target client, with colors from `multiplayer_colors` (using `client_id % 10` for color index) in cc-zellij-plugin/src/controller/render_broadcast.rs
- [x] T020 [US2] Modify `broadcast_render()` to pass each sidebar's `client_id` from the sidebar registry when building per-sidebar payloads in cc-zellij-plugin/src/controller/render_broadcast.rs
- [x] T021 [US2] Add `render_presence_indicators()` function to sidebar render module that takes `other_client_focus`, session display_name, and available width, renders right-aligned inverted-space colored blocks (max 5, separated by one space) in cc-zellij-plugin/src/sidebar_plugin/render.rs
- [x] T022 [US2] Integrate `render_presence_indicators()` into `render_session_entry()`, truncating session name when indicators need space, in cc-zellij-plugin/src/sidebar_plugin/render.rs
- [x] T023 [US2] Skip presence indicator rendering when `other_client_focus` is empty (single-client or no ModeUpdate yet) in cc-zellij-plugin/src/sidebar_plugin/render.rs
- [x] T024 [US2] Clean up stale `client_focus` entries when `SessionUpdate` shows reduced `connected_clients` count in cc-zellij-plugin/src/controller/events.rs

**Checkpoint**: Presence indicators visible. Indicators move when clients switch sessions. No indicators in single-client mode. `make test` passes.

---

## Phase 4: User Story 3 - Per-Client Activation Order (Priority: P2)

**Goal**: Each client maintains its own session ordering. Client 2 switching sessions does not reorder client 1's sidebar.

**Independent Test**: Two clients connected. Client 2 clicks sessions in reverse order. Client 2's sidebar reorders. Client 1's sidebar stays unchanged.

### Tests for User Story 3

- [x] T025 [P] [US3] Unit test: `apply_local_activation_order()` reorders sessions by most-recently-focused-first, appends unknown sessions using payload order in cc-zellij-plugin/src/sidebar_plugin/state.rs

### Implementation for User Story 3

- [x] T026 [US3] Add `update_local_activation_order()` method to `SidebarState` that moves a pane_id to the front of `local_activation_order` in cc-zellij-plugin/src/sidebar_plugin/state.rs
- [x] T027 [US3] Call `update_local_activation_order(pane_id)` in sidebar click handler and navigate-Enter handler (alongside the existing `local_focus_override` set) in cc-zellij-plugin/src/sidebar_plugin/input.rs
- [x] T028 [US3] Add `apply_local_activation_order()` method to `SidebarState` that reorders `cached_payload.sessions` by local order (known pane_ids first in local order, unknown appended in payload order) in cc-zellij-plugin/src/sidebar_plugin/state.rs
- [x] T029 [US3] Call `apply_local_activation_order()` in the sidebar's payload update handler (where `cached_payload` is set) before rendering session entries, in cc-zellij-plugin/src/sidebar_plugin/mod.rs

**Checkpoint**: Per-client ordering works. Client 2's reordering doesn't affect client 1. Single-client behavior unchanged. `make test` passes.

---

## Phase 5: Polish & Cross-Cutting Concerns

**Purpose**: Documentation, cleanup, and final validation

- [x] T030 [P] Update README.md with multiplayer focus behavior, presence indicators, and keyboard shortcut limitation
- [x] T031 [P] Add or update Antora guide page for multiplayer session usage in docs/modules/ROOT/pages/ (covers independent focus and presence indicators)
- [x] T032 Run `make verify` (tests + linting) and fix any warnings or errors
- [x] T033 Run quickstart.md validation scenarios (V1-V7) manually with two terminals

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies, start immediately
- **US1 (Phase 2)**: Depends on Setup (T001-T006)
- **US2 (Phase 3)**: Depends on Setup (T001-T006) and US1 (T010-T014 for focus-report flow)
- **US3 (Phase 4)**: Depends on Setup (T004) only. Can run in parallel with US2.
- **Polish (Phase 5)**: Depends on all user stories complete

### User Story Dependencies

- **US1 (P1)**: Depends on Setup only. BLOCKS US2 (presence indicators need focus-report data).
- **US2 (P1)**: Depends on US1 (controller needs focus-report handling in place to populate `client_focus`).
- **US3 (P2)**: Depends on Setup only. Independent of US1 and US2. Can run in parallel with US2.

### Within Each User Story

- Tests before implementation (TDD)
- Types/structs before handlers
- Handlers before integration
- Debug logging alongside implementation

### Parallel Opportunities

**Setup phase**: T003 and T004 can run in parallel (different files)

**US1 tests**: T007, T008, T009 can all run in parallel

**US2 tests**: T016, T017, T018 can all run in parallel

**US2 + US3**: Once US1 is complete, US2 and US3 can proceed in parallel (US3 only needs Setup, not US1)

**Polish**: T030 and T031 can run in parallel

---

## Parallel Example: User Story 1

```text
# After Setup phase, launch tests in parallel:
T007: Unit test FocusReport serde round-trip
T008: Unit test parse_pipe_message for focus-report
T009: Unit test handle_focus_report updates client_focus

# Then implement sequentially:
T010: Modify sidebar click handler for local focus
T011: Add send_focus_report helper
T012: Wire send_focus_report into click + Enter handlers
T013: Add handle_focus_report to controller
T014: Route cc-deck:focus-report in controller pipe handler
T015: Add debug logging
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (T001-T006)
2. Complete Phase 2: US1 - Independent Focus (T007-T015)
3. **STOP and VALIDATE**: Test with two terminals, verify independent focus works
4. This alone fixes the critical bug

### Incremental Delivery

1. Setup -> Foundation ready
2. US1 (Independent Focus) -> Bug fixed, test with two clients (MVP!)
3. US2 (Presence Indicators) -> Visual awareness added, test indicators
4. US3 (Per-Client Order) -> Sidebar ordering independent, test reordering
5. Polish -> Documentation, final validation

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story
- Each user story is independently completable and testable
- Commit after each task or logical group
- Stop at any checkpoint to validate story independently
- `make test` after each phase, `make verify` at final phase
