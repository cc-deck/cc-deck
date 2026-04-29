# Tasks: Voice Sidebar Integration

**Input**: Design documents from `specs/045-voice-sidebar-integration/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/voice-command-protocol.md

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

---

## Phase 1: Setup

**Purpose**: No new project structure needed. This phase removes PTT infrastructure first (User Story 5 is P3 but must happen first to avoid conflicting code paths).

- [X] T001 Remove `VoiceControl` variant from `PipeAction` enum and its match arm in `cc-zellij-plugin/src/pipe_handler.rs`
- [X] T002 [P] Remove `voice_control_pipe` field from `ControllerState` in `cc-zellij-plugin/src/controller/state.rs`
- [X] T003 [P] Remove `VoiceControl` pipe holding logic and `VoiceToggle` handler from `cc-zellij-plugin/src/controller/mod.rs`
- [X] T004 [P] Remove F8 keybinding registration for `cc-deck:voice-toggle` from `register_keybindings()` in `cc-zellij-plugin/src/controller/events.rs`
- [X] T005 [P] Remove `--mode` flag and PTT-related config from `cc-deck/internal/cmd/ws_voice.go`
- [X] T006 [P] Remove PTT mode logic (long-poll goroutine, mode switching) from `cc-deck/internal/voice/relay.go`
- [X] T007 Remove `Mode` field from `RelayConfig` and PTT-related constants in `cc-deck/internal/voice/relay.go`
- [X] T008 Update tests: remove PTT-related test cases in `cc-zellij-plugin/src/pipe_handler.rs` (test_parse_voice_commands) and `cc-deck/internal/voice/relay_test.go`

**Checkpoint**: PTT fully removed. `make test` and `make lint` pass in both codebases. No references to `VoiceControl`, `voice-control`, `voice-toggle`, `--mode`, or F8 keybinding remain.

---

## Phase 2: Foundational (Command Protocol)

**Purpose**: Implement the `[[command]]` protocol that all user stories depend on. This replaces the current raw `\r` injection for command words with structured protocol messages.

- [X] T009 Add `[[command]]` parsing to the `VoiceText` handler: detect `[[` prefix in the match arm for `PipeAction::VoiceText` in `cc-zellij-plugin/src/controller/mod.rs`. Extract command string, dispatch to a new `handle_voice_command()` function. Plain text (no `[[` prefix) continues to `write_chars_to_pane_id` as before.
- [X] T010 [P] Implement `handle_voice_command()` in `cc-zellij-plugin/src/controller/mod.rs`: match on `"enter"` (send `\r` to attended pane), `"voice:on"` (set `voice_enabled = true`, record ping timestamp), `"voice:off"` (set `voice_enabled = false`, `voice_muted = false`), `"voice:ping"` (update ping timestamp), `"voice:mute"` (set `voice_muted = true`, clear `voice_mute_requested`), `"voice:unmute"` (set `voice_muted = false`, clear `voice_mute_requested`). Mark render dirty on state changes.
- [X] T011 [P] Add `voice_muted` (bool, default false), `voice_last_ping_ms` (u64, default 0), and `voice_mute_requested` (Option<bool>, default None) fields to `ControllerState` in `cc-zellij-plugin/src/controller/state.rs`
- [X] T012 [P] Update voice relay CLI to send `[[command]]` protocol messages: on startup send `[[voice:on]]`, on shutdown send `[[voice:off]]`, replace raw `\r` command word injection with `[[enter]]`, in `cc-deck/internal/voice/relay.go`
- [X] T013 [P] Add heartbeat goroutine to voice relay: send `[[voice:ping]]` every 5 seconds via `PipeSender.Send(ctx, "cc-deck:voice", "[[voice:ping]]")` in `cc-deck/internal/voice/relay.go`
- [X] T014 Add heartbeat timeout check to `handle_timer()` in `cc-zellij-plugin/src/controller/events.rs`: if `voice_enabled && now_ms - voice_last_ping_ms > 15000`, clear voice state (`voice_enabled = false`, `voice_muted = false`, `voice_mute_requested = None`) and mark render dirty
- [X] T015 Add unit tests for command protocol parsing in `cc-zellij-plugin/src/controller/mod.rs` (or a new `voice.rs` module): test `[[voice:on]]`, `[[voice:off]]`, `[[enter]]`, `[[voice:ping]]`, `[[voice:mute]]`, `[[voice:unmute]]`, plain text passthrough, and unknown commands
- [X] T016 [P] Add unit tests for `[[command]]` sending in `cc-deck/internal/voice/relay_test.go`: test that command words produce `[[enter]]`, startup sends `[[voice:on]]`, shutdown sends `[[voice:off]]`

**Checkpoint**: Command protocol works end-to-end. Voice relay sends structured commands, plugin parses and acts on them. Heartbeat detects crashed relay within 15 seconds. `make test` passes.

---

## Phase 3: User Story 1 - Voice Status Visibility (Priority: P1) MVP

**Goal**: Show ♫ indicator in sidebar header when voice relay is connected, bright when listening, dim when muted.

**Independent Test**: Start voice relay for a workspace. The sidebar header shows ♫ in bright green. Stop voice relay. The ♫ disappears. Mute via TUI. The ♫ dims.

### Implementation for User Story 1

- [X] T017 [P] [US1] Add `voice_connected` (bool, default false) and `voice_muted` (bool, default false) fields to `RenderPayload` in `cc-zellij-plugin/src/lib.rs`. Use `#[serde(default)]` for backwards compatibility.
- [X] T018 [US1] Set `voice_connected` and `voice_muted` in `RenderPayload` from `ControllerState` fields in `cc-zellij-plugin/src/controller/render_broadcast.rs`
- [X] T019 [US1] Render ♫ indicator in `render_header()` in `cc-zellij-plugin/src/sidebar_plugin/render.rs`: when `payload.voice_connected` is true, append ♫ right-aligned on the header line. Use bright green (`\x1b[38;2;80;220;120m`) when not muted, dim (`\x1b[2m`) when muted. Define a named constant `VOICE_CLICK_SENTINEL: u32 = u32::MAX - 2` (alongside the existing header sentinel `u32::MAX - 1`) and use it for the click region registration.
- [X] T020 [US1] Add unit test for `RenderPayload` serialization with voice fields in `cc-zellij-plugin/src/lib.rs` (protocol_tests module): verify roundtrip with `voice_connected: true, voice_muted: false` and backwards compat (deserialize payload without voice fields defaults to false)

**Checkpoint**: Voice relay connection state is visible in the sidebar. ♫ appears/disappears based on connection, brightens/dims based on mute state. User Story 1 acceptance scenarios pass.

---

## Phase 4: User Story 2 - Mute Toggle from Sidebar (Priority: P1)

**Goal**: Toggle voice mute via `Alt+v` global shortcut, `v` key in navigation mode, or clicking the ♫ indicator.

**Independent Test**: Start voice relay. Press `Alt+v`. The ♫ dims and voice TUI shows muted state. Press `Alt+v` again. The ♫ brightens and dictation resumes.

### Implementation for User Story 2

- [X] T021 [P] [US2] Add `voice_key` config field (default `"Alt v"`) to `PluginConfig` in `cc-zellij-plugin/src/config.rs`. Parse from KDL configuration map.
- [X] T022 [US2] Register `voice_key` as a Zellij keybinding in `register_keybindings()` in `cc-zellij-plugin/src/controller/events.rs`: bind configured key to `MessagePluginId` with name `cc-deck:voice-mute-toggle`.
- [X] T023 [US2] Add `VoiceMuteToggle` variant to `PipeAction` enum in `cc-zellij-plugin/src/pipe_handler.rs` for the `cc-deck:voice-mute-toggle` pipe name.
- [X] T024 [US2] Handle `VoiceMuteToggle` in the controller pipe handler in `cc-zellij-plugin/src/controller/mod.rs`: if `voice_enabled`, set `voice_mute_requested` to `Some(!voice_muted)` and mark render dirty. If not connected, ignore silently. Note: rapid toggles before the CLI polls are safe because each toggle overwrites the requested state (not a counter), so the final state is always correct. The CLI acknowledges by sending `[[voice:mute/unmute]]`, which clears the request.
- [X] T025 [US2] Add `VoiceMute` variant to `ActionType` enum in `cc-zellij-plugin/src/lib.rs`. Handle `VoiceMute` in `cc-zellij-plugin/src/controller/actions.rs`: same toggle logic as T024.
- [X] T026 [US2] Handle `v` key press in navigation mode in `cc-zellij-plugin/src/sidebar_plugin/input.rs`: send `ActionMessage` with `ActionType::VoiceMute` to the controller. Note: `v` is handled as a direct action key (like `p` for pause, `d` for delete), not as a search character. Navigation mode already distinguishes action keys from search input (search requires `/` prefix). No conflict with session names starting with "v".
- [X] T027 [US2] Handle ♫ click (using `VOICE_CLICK_SENTINEL` constant from T019) in click handling in `cc-zellij-plugin/src/sidebar_plugin/input.rs` or `render.rs`: send `ActionMessage` with `ActionType::VoiceMute` to the controller.
- [X] T028 [US2] Add `voice_mute_requested` field to `DumpStateResponse` in `cc-zellij-plugin/src/controller/mod.rs` (`dump_state` method). Serialize `Option<bool>` from `ControllerState`.
- [X] T029 [US2] Update voice relay CLI dump-state parsing in `cc-deck/internal/voice/relay.go`: read `voice_mute_requested` from response. When `Some(true)`, set local muted flag and send `[[voice:mute]]`. When `Some(false)`, clear local muted flag and send `[[voice:unmute]]`. Emit `RelayEvent` with type `"muted"` or `"unmuted"`.
- [X] T030 [US2] Add unit tests for mute toggle flow in `cc-deck/internal/voice/relay_test.go`: test that `voice_mute_requested: true` in dump-state response triggers mute and `[[voice:mute]]` send

**Checkpoint**: Mute toggle works from sidebar keybinding, navigation mode key, and ♫ click. State synchronizes between sidebar and voice CLI. User Story 2 acceptance scenarios pass.

---

## Phase 5: User Story 3 - Mute Toggle from Voice TUI (Priority: P2)

**Goal**: Toggle mute with `m` key in voice TUI. Mute state synchronizes with sidebar indicator.

**Independent Test**: In voice TUI, press `m`. TUI header shows "MUTED" and sidebar ♫ dims. Press `m` again. TUI resumes and ♫ brightens.

### Implementation for User Story 3

- [X] T031 [P] [US3] Add `muted` boolean field to voice TUI model in `cc-deck/internal/tui/voice/model.go`. Remove any PTT mode state.
- [X] T032 [US3] Handle `m` key in voice TUI update handler in `cc-deck/internal/tui/voice/update.go`: toggle `muted` flag, send `[[voice:mute]]` or `[[voice:unmute]]` via pipe, emit mute/unmute relay event.
- [X] T033 [US3] Update voice TUI view in `cc-deck/internal/tui/voice/view.go`: show "MUTED" in header when muted, update mode display (remove PTT references).
- [X] T034 [US3] Wire mute state from relay events to TUI model: when relay receives sidebar-initiated mute via dump-state poll, propagate the state change to the TUI via `RelayEvent`.

**Checkpoint**: Mute toggle works from voice TUI. State synchronizes bidirectionally between TUI and sidebar. User Story 3 acceptance scenarios pass.

---

## Phase 6: User Story 4 - Command Protocol (Priority: P2)

**Goal**: Verify the `[[command]]` protocol correctly separates control signals from dictation text end-to-end.

**Independent Test**: Dictate "fix the login bug" followed by saying "send". Text appears in attended pane, then newline is sent. Verify via verbose logging.

### Implementation for User Story 4

- [X] T035 [US4] Add verbose logging for command protocol in `cc-zellij-plugin/src/controller/mod.rs`: log `[[command]]` dispatch and plain text injection with payload length (no content for privacy).
- [X] T036 [US4] Verify `[[enter]]` sends carriage return to the correct attended pane: add integration-style test in `cc-zellij-plugin/src/controller/mod.rs` tests (non-wasm path) that calls `handle_voice_command("enter")` and asserts the right action.
- [X] T037 [US4] Ensure ANSI sanitization from FR-020 (042 spec) still applies to plain text payloads but NOT to `[[command]]` messages in `cc-zellij-plugin/src/controller/mod.rs`. Important: `[[command]]` detection (T009) MUST run before sanitization, so that bracket syntax is not corrupted. The flow is: receive payload -> check `[[` prefix -> if command: dispatch to `handle_voice_command()` (no sanitization); if plain text: sanitize then inject via `write_chars_to_pane_id`.

**Checkpoint**: Protocol works end-to-end. Command words produce `[[enter]]`, dictation produces plain text, sanitization applies correctly. User Story 4 acceptance scenarios pass.

---

## Phase 7: User Story 5 - PTT Removal (Priority: P3)

**Goal**: Verify PTT is fully removed (already done in Phase 1 setup).

**Independent Test**: `cc-deck ws voice mydev --mode ptt` produces an error. No `VoiceControl` pipe action exists.

### Implementation for User Story 5

- [X] T038 [US5] Verify removal: grep both codebases for any remaining references to `VoiceControl`, `voice-control`, `voice_control`, `voice-toggle`, `--mode`, `ptt`, `PTT`. Fix any stragglers.
- [X] T039 [US5] Update help text in `cc-deck/internal/cmd/ws_voice.go`: remove PTT mode documentation, update description to reflect VAD-only with mute toggle.

**Checkpoint**: PTT is fully removed. `--mode` flag does not exist. F8 keybinding is gone. User Story 5 acceptance scenarios pass.

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: Documentation, help overlay, and final validation.

- [X] T040 [P] Update help overlay in `cc-zellij-plugin/src/sidebar_plugin/render.rs` (`render_help_overlay`): add `Alt+v  Voice mute` entry and `v` key in navigation mode actions.
- [X] T041 [P] Update CLI reference documentation in `docs/modules/reference/pages/cli.adoc`: add voice mute toggle, remove PTT mode, document `voice_key` config.
- [X] T042 [P] Update configuration reference in `docs/modules/reference/pages/configuration.adoc`: add `voice_key` plugin config option.
- [X] T043 [P] Add Antora guide page for voice sidebar integration in `docs/modules/guides/pages/voice-sidebar.adoc`: explain mute toggle, ♫ indicator, command protocol overview.
- [X] T044 Update README.md with voice sidebar feature: mention ♫ indicator, mute toggle, removal of PTT.
- [X] T045 Run `make test` and `make lint` across both codebases. Fix any remaining issues.
- [ ] T046 Run quickstart.md validation: manually verify all steps in `specs/045-voice-sidebar-integration/quickstart.md`.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Setup/PTT Removal)**: No dependencies, start immediately
- **Phase 2 (Command Protocol)**: Depends on Phase 1 (PTT removal clears conflicting code)
- **Phase 3 (US1 - Visibility)**: Depends on Phase 2 (needs `voice_enabled` state from protocol)
- **Phase 4 (US2 - Sidebar Mute)**: Depends on Phase 2 + Phase 3 (needs ♫ indicator + protocol)
- **Phase 5 (US3 - TUI Mute)**: Depends on Phase 2 (needs mute protocol). Can parallel with Phase 4.
- **Phase 6 (US4 - Protocol Verify)**: Depends on Phase 2 (protocol implementation)
- **Phase 7 (US5 - PTT Verify)**: Depends on Phase 1 (just verification)
- **Phase 8 (Polish)**: Depends on all previous phases

### User Story Dependencies

- **US1 (Visibility)**: Requires Foundational (Phase 2) complete
- **US2 (Sidebar Mute)**: Requires US1 (needs ♫ indicator to click/dim)
- **US3 (TUI Mute)**: Requires Foundational only (independent of US1/US2 for core mute, but sync requires protocol)
- **US4 (Protocol)**: Requires Foundational only (verification phase)
- **US5 (PTT Removal)**: Already done in Phase 1 (verification only)

### Within Each User Story

- Protocol handlers before UI rendering
- Controller changes before sidebar changes
- Rust plugin changes before Go CLI changes (protocol is defined by plugin)
- Tests alongside implementation

### Parallel Opportunities

- T001-T008 (Phase 1): T002-T006 can all run in parallel
- T009-T016 (Phase 2): T010-T013 and T016 can run in parallel
- T017+T021 (Phase 3+4): T017 and T021 can run in parallel (different files)
- Phase 5 (US3) can run in parallel with Phase 4 (US2) after Phase 2 completes
- Phase 6 (US4) can run in parallel with Phase 4/5 after Phase 2 completes
- T040-T043 (Phase 8): All documentation tasks can run in parallel

---

## Parallel Example: Phase 1 (PTT Removal)

```
# These tasks modify different files and can run together:
Task T002: Remove voice_control_pipe from state.rs
Task T003: Remove VoiceControl handler from controller/mod.rs
Task T004: Remove F8 keybinding from events.rs
Task T005: Remove --mode flag from ws_voice.go
Task T006: Remove PTT goroutine from relay.go
```

## Parallel Example: Phase 2 (Foundational)

```
# After T009 (protocol parsing entry point):
Task T010: Implement handle_voice_command() in controller/mod.rs
Task T011: Add voice state fields to state.rs
Task T012: Update voice relay CLI for [[command]] protocol in relay.go
Task T013: Add heartbeat goroutine in relay.go
Task T016: Add CLI-side unit tests in relay_test.go
```

---

## Implementation Strategy

### MVP First (User Stories 1 + 2)

1. Complete Phase 1: PTT Removal (clears the way)
2. Complete Phase 2: Command Protocol (foundation)
3. Complete Phase 3: Voice Status Visibility (♫ in sidebar)
4. Complete Phase 4: Sidebar Mute Toggle (Alt+v, click, nav key)
5. **STOP and VALIDATE**: Test mute from sidebar, verify ♫ visual feedback
6. Deploy/demo if ready

### Incremental Delivery

1. Phase 1 + Phase 2 -> Protocol works, PTT gone
2. Add US1 (visibility) -> ♫ indicator appears in sidebar
3. Add US2 (sidebar mute) -> Full mute toggle from sidebar
4. Add US3 (TUI mute) -> Bidirectional mute sync
5. Add US4+US5 (verify) -> Protocol and PTT removal confirmed clean
6. Phase 8 (polish) -> Documentation complete

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Phase 1 removes PTT before adding mute to avoid conflicting code paths
- The command protocol (Phase 2) is the foundation that all user stories depend on
- Commit after each task or logical group
- Stop at any checkpoint to validate story independently
