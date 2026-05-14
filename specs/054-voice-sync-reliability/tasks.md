# Tasks: Voice Sync Reliability

**Input**: Design documents from `/specs/054-voice-sync-reliability/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md

**Tests**: Included (constitution requires tests for all features).

**Organization**: Tasks grouped by user story. US1 and US4 share the same implementation (heartbeat protocol change) and are combined into one phase.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

---

## Phase 1: Foundational (Blocking Prerequisites)

**Purpose**: Expose targeted render API needed by US2, and establish test helpers shared across stories

**CRITICAL**: No user story work can begin until this phase is complete

- [X] T001 Add `targeted_render()` public function to `cc-zellij-plugin/src/controller/render_broadcast.rs` that builds a RenderPayload and sends it to a single plugin_id (combines `build_render_payload()` + `send_render_to_plugin()`)
- [X] T002 Add unit test for `targeted_render()` in `cc-zellij-plugin/src/controller/render_broadcast.rs` verifying payload is built and send is called for the specified plugin_id

**Checkpoint**: Foundation ready, user story implementation can begin

---

## Phase 2: User Story 1 + User Story 4 - Stable Voice Indicator & Mute State Recovery (Priority: P1 + P2) MVP

**Goal**: Eliminate voice indicator flickering by making every heartbeat carry mute state, and ensure mute state survives timeout/recovery by updating `voice_muted` on every heartbeat (not just first enable).

**Independent Test (US1)**: Start voice relay. Observe sidebar for 60 seconds. Green note must remain stable with no visual changes.

**Independent Test (US4)**: Mute voice relay. Wait 15+ seconds (heartbeat timeout). Resume heartbeat. Sidebar must show dim note (muted state preserved).

### Tests for US1 + US4

- [X] T003 [P] [US1] Add Rust unit test in `cc-zellij-plugin/src/controller/mod.rs`: verify `handle_voice_command("voice:on:unmuted")` when `voice_enabled` is already true and `voice_muted` is false does NOT call `mark_render_dirty()`
- [X] T004 [P] [US1] Add Rust unit test in `cc-zellij-plugin/src/controller/mod.rs`: verify `handle_voice_command("voice:on:muted")` when `voice_enabled` is true and `voice_muted` is false DOES call `mark_render_dirty()` and sets `voice_muted = true`
- [X] T005 [P] [US1] Add Rust unit test in `cc-zellij-plugin/src/controller/mod.rs`: verify bare `handle_voice_command("voice:on")` (no suffix) sets `voice_muted = false` (backward compat), and `handle_voice_command("voice:on:garbage")` also sets `voice_muted = false`
- [X] T006 [P] [US4] Add Go unit test in `cc-deck/internal/voice/relay_test.go`: verify heartbeat sends `[[voice:on:muted]]` when relay is muted and `[[voice:on:unmuted]]` when relay is unmuted

### Implementation for US1 + US4

- [X] T007 [US1][US4] Restructure `handle_voice_command()` in `cc-zellij-plugin/src/controller/mod.rs` (lines 486-496): always parse mute suffix from `voice:on:*` heartbeats, always update `voice_muted`, only call `mark_render_dirty()` when `voice_enabled` or `voice_muted` actually changed. Bare `voice:on` and unrecognized suffixes treated as unmuted.
- [X] T008 [US4] Update `statePoll()` in `cc-deck/internal/voice/relay.go` (line 278): change `"[[voice:on]]"` to send `fmt.Sprintf("[[voice:on:%s]]", muteState)` where muteState is `"muted"` or `"unmuted"` based on `r.muted` (read under `r.mu` lock)

**Checkpoint**: Voice indicator stable at idle. Mute state preserved through timeout/recovery. Verify: `make test` passes for both Rust and Go.

---

## Phase 3: User Story 2 - Stable Indicator During Session Switch (Priority: P1)

**Goal**: Prevent green note from disappearing when switching sessions or tabs by sending the current render payload (including voice state) to newly registered sidebars.

**Independent Test**: Start voice relay. Click through 5 sessions rapidly. Green note must never disappear.

### Tests for US2

- [X] T009 [US2] Add Rust unit test in `cc-zellij-plugin/src/controller/sidebar_registry.rs`: verify `handle_sidebar_hello()` calls `targeted_render()` for the registered sidebar plugin_id after sending sidebar-init

### Implementation for US2

- [X] T010 [US2] Update `handle_sidebar_hello()` in `cc-zellij-plugin/src/controller/sidebar_registry.rs` (lines 18-29): after `send_sidebar_init()`, call `targeted_render()` from `render_broadcast` module to send the current render payload (including voice state) to the newly registered sidebar

**Checkpoint**: New sidebars show voice indicator immediately on registration. Verify: switching tabs keeps green note visible.

---

## Phase 4: User Story 3 - Accurate Session Name in Voice Relay (Priority: P2)

**Goal**: Voice relay TUI displays the correct session name by using `focused_pane_id` for resolution instead of relying solely on `attended_pane_id`.

**Independent Test**: Focus session "api-server", verify relay shows "api-server". Switch to "frontend", verify relay shows "frontend" within 2 seconds.

### Tests for US3

- [X] T011 [P] [US3] Add Rust unit test in `cc-zellij-plugin/src/controller/mod.rs`: verify `dump_state()` serialized response includes both `focused_pane_id` and `attended_pane_id` fields
- [X] T012 [P] [US3] Add Go unit test in `cc-deck/internal/voice/relay_test.go`: verify `parseDumpStateResponse()` prefers `focused_pane_id` over `attended_pane_id` when both are present and both map to sessions
- [X] T013 [P] [US3] Add Go unit test in `cc-deck/internal/voice/relay_test.go`: verify `parseDumpStateResponse()` falls back to `attended_pane_id` when `focused_pane_id` is absent or does not map to a session

### Implementation for US3

- [X] T014 [US3] Add `focused_pane_id: Option<u32>` field to `DumpStateResponse` struct in `cc-zellij-plugin/src/controller/mod.rs` (lines 560-566) and populate from `state.focused_pane_id`
- [X] T015 [US3] Update `parseDumpStateResponse()` in `cc-deck/internal/voice/relay.go` (lines 332-383): add `FocusedPaneID *int` to envelope struct, resolve session name with priority: focused_pane_id -> attended_pane_id -> single-session fallback

**Checkpoint**: Voice relay TUI shows correct session name when switching focus. Verify: `make test` passes for both Rust and Go.

---

## Phase 5: Polish & Cross-Cutting Concerns

**Purpose**: Documentation and final validation

- [X] T016 Update README.md with voice sync reliability improvements (heartbeat protocol change, session name resolution change)
- [X] T017 Run `make test` and `make lint` for both `cc-zellij-plugin/` and `cc-deck/` to verify no regressions

---

## Dependencies & Execution Order

### Phase Dependencies

- **Foundational (Phase 1)**: No dependencies, start immediately
- **US1+US4 (Phase 2)**: No dependency on Phase 1 (does not use `targeted_render()`). Can start immediately.
- **US2 (Phase 3)**: Depends on Phase 1 completion (uses `targeted_render()` from T001)
- **US3 (Phase 4)**: No dependency on Phase 1. Can start immediately.
- **Polish (Phase 5)**: Depends on all user stories being complete

### User Story Dependencies

- **US1+US4 (Phase 2)**: Independent of US2 and US3. Can start after Phase 1.
- **US2 (Phase 3)**: Depends on T001 (`targeted_render()` API). Independent of US1/US4 and US3.
- **US3 (Phase 4)**: Independent of US1/US4 and US2. Can start after Phase 1.

### Within Each User Story

- Tests written first, expected to fail
- Implementation follows to make tests pass
- Rust and Go changes within a story are parallelizable when marked [P]

### Parallel Opportunities

- T001 and T002 are sequential (API then test)
- US1+US4 (Phase 2) and US3 (Phase 4) can start immediately (no Phase 1 dependency). US2 (Phase 3) must wait for T001.
- Within US1+US4: T003/T004/T005/T006 are all [P] (parallel tests), then T007/T008 are sequential
- Within US3: T011/T012/T013 are all [P] (parallel tests), then T014/T015 are sequential (Rust before Go since Go depends on new field)

---

## Parallel Example: All User Stories After Phase 1

```
# After Phase 1 completes, launch all user stories in parallel:

Agent 1 (US1+US4): T003-T008 (Rust controller handler + Go relay heartbeat)
Agent 2 (US2):     T009-T010 (Rust sidebar registry + targeted render)
Agent 3 (US3):     T011-T015 (Rust dump-state + Go session resolution)
```

---

## Implementation Strategy

### MVP First (US1 + US4)

1. Complete Phase 1: Foundational (`targeted_render()` API)
2. Complete Phase 2: US1+US4 (heartbeat protocol + handler change-gating)
3. **STOP and VALIDATE**: Test voice indicator stability and mute recovery
4. This alone fixes the two most impactful issues (flickering + mute state loss)

### Incremental Delivery

1. Phase 1: Foundation -> `targeted_render()` ready
2. Phase 2: US1+US4 -> Stable indicator + mute recovery (MVP)
3. Phase 3: US2 -> Stable during session switch
4. Phase 4: US3 -> Accurate session name
5. Phase 5: Polish -> Documentation + final validation

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- US1 and US4 are combined because they share the same code changes (heartbeat protocol + handler restructure)
- Commit after each phase checkpoint
- All changes are modifications to existing files (no new files created)
