# Feature Specification: Voice Sidebar Integration

**Feature Branch**: `045-voice-sidebar-integration`
**Created**: 2026-04-29
**Status**: Draft
**Input**: Brainstorm 045 - Voice relay sidebar indicator, mute toggle, command protocol, PTT removal

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Voice Status Visibility (Priority: P1)

A developer running voice relay in a second terminal wants to see at a glance whether voice relay is connected and whether it is currently listening or muted, without switching away from their current Zellij tab.

**Why this priority**: Without visual feedback in the sidebar, there is no way to know the voice relay state without switching to the voice TUI terminal. This is the foundational requirement that all other stories build on.

**Independent Test**: Start voice relay for a workspace. The sidebar status line shows ♫ in bright color. Stop voice relay. The ♫ disappears from the status line.

**Acceptance Scenarios**:

1. **Given** voice relay is not running, **When** I look at the sidebar status line, **Then** no ♫ symbol appears
2. **Given** voice relay starts for a workspace, **When** it connects, **Then** ♫ appears right-aligned on the status line in bright color
3. **Given** voice relay is connected and listening, **When** voice relay stops, **Then** ♫ disappears from the status line
4. **Given** voice relay is connected and muted, **When** I look at the status line, **Then** ♫ appears in dim color

---

### User Story 2 - Mute Toggle from Sidebar (Priority: P1)

A developer working in a Zellij session wants to mute and unmute the voice relay without switching to the voice TUI terminal. They can use a global keyboard shortcut, a navigation-mode key, or click the ♫ symbol.

**Why this priority**: Muting is the primary control action. Without it, the sidebar indicator is passive and the developer must switch terminals to pause dictation.

**Independent Test**: Start voice relay. Press `Alt+v` to mute. The ♫ dims in the sidebar and the voice TUI shows muted state. Press `Alt+v` again to unmute. The ♫ returns to bright and dictation resumes.

**Acceptance Scenarios**:

1. **Given** voice relay is connected and listening, **When** I press `Alt+v`, **Then** voice relay mutes and ♫ dims
2. **Given** voice relay is muted, **When** I press `Alt+v`, **Then** voice relay unmutes and ♫ brightens
3. **Given** I am in navigation mode with voice relay connected, **When** I press `v`, **Then** the mute state toggles
4. **Given** voice relay is connected, **When** I click the ♫ symbol, **Then** the mute state toggles
5. **Given** voice relay is not connected, **When** I press `Alt+v`, **Then** nothing happens (no error, no indicator)

---

### User Story 3 - Mute Toggle from Voice TUI (Priority: P2)

A developer using the voice TUI wants to mute and unmute dictation with a single keypress. The mute state synchronizes with the sidebar indicator.

**Why this priority**: Complements story 2 by providing mute control from the other direction. Replaces the PTT mode with a simpler interaction.

**Independent Test**: In the voice TUI, press `m` to mute. The TUI header shows "MUTED" and the sidebar ♫ dims. Press `m` again. The TUI resumes listening and the sidebar ♫ brightens.

**Acceptance Scenarios**:

1. **Given** voice relay is listening, **When** I press `m` in the voice TUI, **Then** the TUI shows muted state and sidebar ♫ dims
2. **Given** voice relay is muted via TUI, **When** I press `m` again, **Then** the TUI resumes listening and sidebar ♫ brightens
3. **Given** voice relay is muted from the sidebar, **When** I look at the voice TUI, **Then** the TUI shows muted state

---

### User Story 4 - Command Protocol (Priority: P2)

Voice relay sends control signals (connection status, mute state, submit command) through a structured protocol that separates commands from dictation text. The plugin interprets commands and translates them to the appropriate action.

**Why this priority**: The structured protocol is the transport layer that makes stories 1-3 work and enables future extensibility. It replaces the current approach of sending raw `\r` for command words.

**Independent Test**: Dictate "fix the login bug" followed by saying "send". The text appears in the attended pane, then a newline is sent. Verify via verbose logging that the pipe received plain text for dictation and `[[enter]]` for the submit command.

**Acceptance Scenarios**:

1. **Given** voice relay is running, **When** it starts up, **Then** it sends `[[voice:on]]` to the plugin
2. **Given** voice relay is running, **When** it shuts down, **Then** it sends `[[voice:off]]` to the plugin
3. **Given** a user dictates text, **When** the text is delivered, **Then** it is sent as plain text (no `[[` wrapper) and injected via `write_chars_to_pane_id`
4. **Given** a user says a command word, **When** the command is detected, **Then** `[[enter]]` is sent and the plugin translates it to a carriage return
5. **Given** the mute state changes, **When** mute is toggled, **Then** `[[voice:mute]]` or `[[voice:unmute]]` is sent to synchronize state

---

### User Story 5 - PTT Removal (Priority: P3)

The push-to-talk mode and its associated long-poll pipe are removed. The `--mode` CLI flag is removed. The `m` key in the voice TUI is repurposed for mute/unmute.

**Why this priority**: Cleanup that simplifies the codebase. PTT is replaced by the mute/unmute model which covers the same use case with less complexity.

**Independent Test**: Verify that `cc-deck ws voice mydev --mode ptt` produces an error about unknown flag. Verify the voice-control long-poll pipe is no longer created.

**Acceptance Scenarios**:

1. **Given** the updated cc-deck, **When** I run `cc-deck ws voice mydev`, **Then** voice relay starts in VAD-only mode with no mode selection
2. **Given** the voice TUI is open, **When** I press `m`, **Then** mute toggles (not mode switch)
3. **Given** the updated cc-deck, **When** I check for `VoiceControl` pipe action, **Then** it no longer exists

---

### Edge Cases

- Voice relay connects, then crashes without sending `[[voice:off]]`. The plugin tracks voice liveness via the dump-state polling loop (the CLI polls every 1 second). If no dump-state request arrives for 15 seconds, the plugin clears the voice state and removes the ♫ indicator. A subsequent `[[voice:on]]` resets the state normally.
- Two voice relay instances try to connect to the same workspace simultaneously. Only one should be active.
- Mute toggle is pressed rapidly multiple times. State should not get out of sync.
- Voice relay is running but the workspace Zellij session is restarted. The ♫ should disappear and reappear when the plugin reinitializes.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: Sidebar MUST display ♫ (beamed eighth notes, U+266B) right-aligned on the status line when voice relay is connected
- **FR-002**: ♫ MUST appear in bright color when listening and dim color when muted
- **FR-003**: ♫ MUST be absent from the status line when voice relay is not connected
- **FR-004**: Users MUST be able to toggle mute via a global shortcut (`Alt+v` by default, configurable via `voice_key` in plugin config)
- **FR-005**: Users MUST be able to toggle mute via the `v` key when in navigation mode
- **FR-006**: Users MUST be able to toggle mute by clicking the ♫ symbol
- **FR-007**: Control signals MUST use `[[command]]` syntax on the `cc-deck:voice` pipe
- **FR-008**: Plain text payloads (no `[[` prefix) MUST be injected via `write_chars_to_pane_id` as before
- **FR-009**: `[[enter]]` MUST send a carriage return to the attended pane
- **FR-010**: `[[voice:on]]` and `[[voice:off]]` MUST set the voice connection state in the plugin
- **FR-011**: `[[voice:mute]]` and `[[voice:unmute]]` MUST synchronize mute state between CLI and plugin
- **FR-012**: Mute toggle from sidebar MUST be communicated to the voice CLI by including a `voice_mute_requested` field in the `cc-deck:dump-state` response. The CLI picks up the toggle on its next poll cycle and acknowledges it by sending `[[voice:mute]]` or `[[voice:unmute]]` back to the plugin
- **FR-013**: Mute state MUST be consistent across sidebar and voice TUI at all times
- **FR-014**: Voice TUI MUST display mute state visually in the TUI header
- **FR-015**: The `m` key in voice TUI MUST toggle mute/unmute
- **FR-016**: PTT mode MUST be removed, including `--mode` CLI flag and voice-control long-poll pipe
- **FR-017**: Voice CLI MUST send `[[voice:on]]` on startup and `[[voice:off]]` on shutdown
- **FR-018**: The `Alt+v` shortcut key MUST be configurable via `voice_key` in the plugin layout config
- **FR-019**: The plugin MUST use dump-state polling as the voice heartbeat signal: each `cc-deck:dump-state` request refreshes the voice liveness timestamp when voice is enabled. The plugin MUST clear voice state if no dump-state request arrives for 15 seconds. No dedicated `[[voice:ping]]` messages are sent, avoiding the risk of text injection from unrecognized commands.

### Key Entities

- **Voice State**: Connection status (connected/disconnected) and mute status (listening/muted), tracked in the controller plugin
- **Command Message**: A `[[command]]` formatted string on the voice pipe that triggers plugin-side actions instead of text injection
- **Voice Indicator**: The ♫ symbol rendered on the sidebar status line with color reflecting current state

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Voice connection state is visible in the sidebar within 1 second of voice relay starting or stopping
- **SC-002**: Mute toggle from CLI-initiated sources (shortcut, nav key, click) reflects in the sidebar within 200ms. Sidebar-initiated mute toggles reflect in the voice TUI within 1 second (bounded by the dump-state poll interval)
- **SC-003**: Dictated text continues to arrive in the attended pane within 5 seconds end-to-end (unchanged from current behavior)
- **SC-004**: The `[[command]]` protocol correctly separates 100% of control signals from dictation text (no false positives where text starting with `[[` is misinterpreted)
- **SC-005**: PTT mode code and voice-control pipe are fully removed with no remaining references

## Clarifications

### Session 2026-04-29

- Q: How should the plugin detect a crashed voice relay that never sent `[[voice:off]]`? → A: Dump-state polling serves as heartbeat; plugin clears voice state after 15s without a dump-state request
- Q: How should the plugin communicate sidebar mute toggles back to the voice CLI? → A: Include `voice_mute_requested` in the existing `cc-deck:dump-state` response; CLI picks it up on next poll cycle
- Q: What poll interval for dump-state is acceptable for sidebar-initiated mute propagation? → A: 1 second (sidebar-to-CLI mute takes up to ~1s worst case)

## Assumptions

- The existing `cc-deck:voice` pipe carries CLI-to-plugin commands. The plugin-to-CLI backchannel uses the existing `cc-deck:dump-state` poll response (no new pipe needed)
- The sidebar plugin can register an additional click region for a single character (♫) on the status line
- The `[[` prefix does not conflict with any natural speech transcription (Whisper does not produce text starting with `[[`)
- Color values for bright and dim ♫ will follow the existing indicator palette used for session state icons
