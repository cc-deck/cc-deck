# Tasks: Controller Leader Election

**Input**: Design documents from `specs/055-controller-leader-election/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: Setup

**Purpose**: Add election state fields and constants

- [X] T001 Add election fields (`is_leader`, `leader_plugin_id`, `last_leader_ping_ms`, `election_ticks`) to `ControllerState` in `cc-zellij-plugin/src/controller/state.rs`. Set `is_leader` default to `false`. Add constants `ELECTION_TIMEOUT_TICKS = 2`, `LEADER_HEARTBEAT_TICKS = 30`, `LEADER_FAILURE_TIMEOUT_MS = 60_000`.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core election protocol that MUST be complete before user story fixes work

- [X] T002 Add dormant guard at the top of `pipe()` in `cc-zellij-plugin/src/controller/mod.rs`: if `!self.state.is_leader` and the pipe name is not `cc-deck:controller-ping` or `cc-deck:controller-pong`, return `false` immediately. Also guard `cc-deck:sidebar-hello` and `cc-deck:action` the same way.
- [X] T003 Add dormant guard at the top of `update()` in `cc-zellij-plugin/src/controller/mod.rs`: if `!self.state.is_leader`, only process `PermissionRequestResult` and `Timer` events. All other events (`TabUpdate`, `PaneUpdate`, `PaneClosed`, `RunCommandResult`, `CommandPaneOpened`) return `false` immediately.
- [X] T004 Implement ping broadcast on startup: after `PermissionRequestResult(Granted)` is processed in `update()` in `cc-zellij-plugin/src/controller/mod.rs`, broadcast `cc-deck:controller-ping` with `self.state.plugin_id.to_string()` as payload. Do NOT activate as leader yet (remain dormant, start counting election ticks).
- [X] T005 Implement ping/pong handler in the `pipe()` method of `cc-zellij-plugin/src/controller/mod.rs`: when receiving `ControllerPing`, parse the payload as `u32` sender_id. If sender_id < self plugin_id, stay/go dormant (set `leader_plugin_id`, reset `last_leader_ping_ms`). If sender_id > self plugin_id, respond with own ping (lower ID wins). If sender_id == self plugin_id, ignore.
- [X] T006 Implement election timeout in `handle_timer()` in `cc-zellij-plugin/src/controller/events.rs`: when `!is_leader`, increment `election_ticks`. If `election_ticks >= ELECTION_TIMEOUT_TICKS` and `leader_plugin_id` is `None`, activate as leader: set `is_leader = true`, register keybindings, broadcast initial render, log activation. Also check `last_leader_ping_ms` for re-activation: if leader ping is older than `LEADER_FAILURE_TIMEOUT_MS`, clear `leader_plugin_id` and reset `election_ticks` to trigger a new election.
- [X] T007 Implement leader heartbeat: in `handle_timer()` in `cc-zellij-plugin/src/controller/events.rs`, when `is_leader`, every `LEADER_HEARTBEAT_TICKS` ticks broadcast a `cc-deck:controller-ping` with own plugin_id.
- [X] T008 Defer keybinding registration: in `handle_tab_update()` in `cc-zellij-plugin/src/controller/events.rs`, wrap the `register_keybindings()` call with an `if state.is_leader` guard. Keybindings are registered when the leader first processes a TabUpdate after activation.
- [X] T009 Unit tests for election protocol in `cc-zellij-plugin/src/controller/state.rs` or a new test module: test pessimistic default (`is_leader == false`), test activation after timeout, test dormant on lower-ID ping, test re-activation after leader failure timeout, test heartbeat timing.

**Checkpoint**: Election protocol complete. Only one controller should be active.

---

## Phase 3: User Story 1 - Stable Navigation Mode (Priority: P1)

**Goal**: A single Alt+s press moves the cursor by exactly one position. No racing, no duplicates.

**Independent Test**: Press Alt+s, observe one cursor move. Press again, one more move. Press Escape, returns to passive.

### Implementation for User Story 1

- [X] T010 [US1] Add integration test for navigation with dual controllers in `cc-zellij-plugin/src/controller/integration_tests.rs`: create two `ControllerState` instances with different plugin_ids. Simulate election (lower ID wins). Send a `cc-deck:navigate` pipe message to both. Assert only the leader processes it (calls `broadcast_navigate`) and the dormant one returns early. Also verify debug log contains a single `CTRL PIPE name=cc-deck:navigate` entry (not duplicated).

**Checkpoint**: Navigation mode works correctly with one cursor move per keypress.

---

## Phase 4: User Story 2 - Stable Voice Indicator (Priority: P1)

**Goal**: The green voice note stays visible during session and tab switches without flickering.

**Independent Test**: Start voice relay, click through 5 sessions rapidly, green note never disappears.

### Implementation for User Story 2

- [X] T012 [US2] Restore `broadcast_render_all` function in `cc-zellij-plugin/src/controller/render_broadcast.rs`: re-add the `broadcast_render_all` function that sends an untargeted `cc-deck:render` pipe message (no `destination_plugin_id`). Use `MessageToPlugin::new("cc-deck:render")` with the JSON payload.
- [X] T013 [US2] Call `broadcast_render_all` at the end of `broadcast_render()` in `cc-zellij-plugin/src/controller/render_broadcast.rs`, after the targeted sidebar sends. This provides the fallback for sidebars not yet in the registry.
- [X] T014 [US2] Unit test: verify `broadcast_render` calls both targeted sends (per sidebar_registry) and the untargeted broadcast_render_all.

**Checkpoint**: Voice indicator remains stable during all user actions.

---

## Phase 5: User Story 3 - Transparent Leader Election (Priority: P2)

**Goal**: Election is invisible to the user. All features work as if there is one controller.

**Independent Test**: Start cc-deck, verify via debug log that election completes within 2 seconds.

### Implementation for User Story 3

- [X] T015 [US3] Add election debug logging: log "CTRL ELECTION: starting probe (dormant)" on startup ping, "CTRL ELECTION: won (activating as leader)" on activation, "CTRL ELECTION: lost to plugin_id=N (staying dormant)" on demotion, "CTRL ELECTION: leader heartbeat" on heartbeat send.
- [X] T016 [US3] Add debug log for dormant guard: log "CTRL DORMANT: ignoring pipe name={name}" at TRACE level (only when debug enabled) so the ignored messages are visible in debug.log for diagnosis.
- [X] T017 [US3] Integration test: simulate the full election flow with two mock controllers. Assert lower plugin_id activates, higher goes dormant, heartbeat resets re-activation timer.

**Checkpoint**: Election is fully operational and observable via debug logs.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Documentation and cleanup

- [X] T018 [P] Update README.md with a note about the known Zellij duplicate instance bug and how cc-deck handles it via leader election. Use the prose plugin with the cc-deck voice profile (`.style/voice.yaml`) per constitution principle I.
- [X] T019 Run `make test` and `make lint` to verify all tests pass and no clippy warnings.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies, start immediately
- **Foundational (Phase 2)**: Depends on Phase 1 (needs election state fields)
- **US1 Navigation (Phase 3)**: Depends on Phase 2 (dormant guards must be in place)
- **US2 Voice (Phase 4)**: Depends on Phase 2 (only one controller broadcasting)
- **US3 Election (Phase 5)**: Depends on Phase 2 (election protocol must exist)
- **Polish (Phase 6)**: Depends on Phases 3-5

### User Story Dependencies

- **US1 (P1)**: Depends on Foundational only. Fix is automatic once dormant guards work.
- **US2 (P1)**: Depends on Foundational only. Independent from US1.
- **US3 (P2)**: Depends on Foundational only. Adds observability, independent from US1/US2.

### Parallel Opportunities

- US1, US2, and US3 can all proceed in parallel after Phase 2 completes
- Within Phase 2: T002 and T003 can run in parallel (different methods). T004 and T005 can run in parallel (different code paths). T006 and T007 modify the same function so must be sequential.
- T012 and T013 are sequential (same file, T013 depends on T012)

---

## Implementation Strategy

### MVP First (Phase 1 + 2 + 3)

1. Complete Phase 1: Add state fields
2. Complete Phase 2: Election protocol and dormant guards
3. Complete Phase 3: Verify navigation fix
4. **STOP and VALIDATE**: Test Alt+s produces exactly one cursor move
5. This alone fixes the most critical regression

### Incremental Delivery

1. Phase 1 + 2 -> Election works, navigation fixed
2. Add Phase 4 -> Voice indicator stable
3. Add Phase 5 -> Full observability
4. Add Phase 6 -> Documentation complete
