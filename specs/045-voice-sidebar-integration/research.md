# Research: Voice Sidebar Integration

## R1: How does the controller-to-sidebar broadcast work?

**Decision**: Voice state is included in `RenderPayload` (broadcast from controller to all sidebars via `cc-deck:render` pipe). Two new boolean fields: `voice_connected` and `voice_muted`.

**Rationale**: The existing `render_broadcast::broadcast_render()` serializes `RenderPayload` and sends it to all registered sidebars via `pipe_message_to_plugin`. Adding two booleans has negligible serialization cost and keeps voice state delivery on the same path as session state (no separate broadcast needed). The sidebar already caches `RenderPayload` in `SidebarState::cached_payload` and re-renders from it.

**Alternatives considered**:
- Separate `cc-deck:voice-state` broadcast pipe: Rejected. Adds protocol complexity, requires sidebars to handle two incoming message types, and voice state changes already trigger `mark_render_dirty()`.

## R2: How does the existing voice text injection work?

**Decision**: Reuse the existing `PipeAction::VoiceText(String)` path in `pipe_handler.rs`. Add `[[command]]` prefix detection inside the match arm. Plain text (no `[[` prefix) continues to `write_chars_to_pane_id`. Commands are dispatched to a new `handle_voice_command()` function.

**Rationale**: The controller already handles `cc-deck:voice` pipe messages and extracts the payload string. The `[[` prefix is unambiguous: Whisper never produces text starting with `[[` (it outputs natural language transcription, not bracket-delimited control codes).

**Alternatives considered**:
- Separate pipe name per command type (e.g., `cc-deck:voice-cmd`): Rejected. Multiplies pipe names and requires the CLI to choose between two pipes based on content type.
- JSON-encoded payload with `type` field: Rejected. Adds parsing overhead for every utterance (the common case is plain text, not commands). The `[[command]]` prefix is cheaper to detect (2-byte prefix check).

## R3: How should the sidebar render the ♫ indicator?

**Decision**: Add ♫ to the right side of the header line (row 0) in `render_header()`. When `voice_connected` is true, append the indicator. When `voice_muted`, render dim (`\x1b[2m`). When listening, render bright green (`\x1b[38;2;80;220;120m`). Register a click region for the ♫ character position.

**Rationale**: The header line already has the star icon (left) and session counts (center-left). Right-aligning ♫ uses the remaining space without conflicting with session count rendering. The click region mechanism already exists for session entries and can be extended with a sentinel pane_id (similar to the header click region using `u32::MAX - 1`).

**Alternatives considered**:
- Separate status bar row below the header: Rejected. Wastes a row that could display sessions. The header has sufficient space.
- Inline with each session entry: Rejected. Voice relay is workspace-wide, not per-session. A single indicator makes conceptual sense.

## R4: How does the dump-state response carry mute toggle requests?

**Decision**: Add a `voice_mute_requested: Option<bool>` field to the `DumpStateResponse` struct in `controller/mod.rs`. When the sidebar toggles mute, the controller sets `voice_mute_requested` to `Some(true)` (to mute) or `Some(false)` (to unmute). The CLI reads this on its next 1-second poll, applies the state, sends `[[voice:mute]]` or `[[voice:unmute]]` back, and the controller clears the pending request.

**Rationale**: The CLI already polls `cc-deck:dump-state` every second for session tracking. Adding a field to the response avoids new pipes and reuses the existing synchronous `SendReceive` pattern. The `Option` type lets the CLI distinguish "no change requested" (`None`) from an active toggle request.

**Alternatives considered**:
- Boolean flag with generation counter: Rejected. Over-engineering for a toggle that fires at most once per user interaction. The clear-on-read pattern via the `[[voice:mute/unmute]]` acknowledgement is sufficient.

## R5: How should PTT removal be structured?

**Decision**: Remove in a single pass across both codebases:
- **Plugin**: Remove `VoiceControl` from `PipeAction` enum, remove `voice_control_pipe` from `ControllerState`, remove `VoiceToggle` handler and F8 keybinding registration, remove long-poll pipe holding logic.
- **CLI**: Remove `Mode` field from `RelayConfig`, remove `--mode` flag from `ws_voice.go`, remove PTT-related goroutine from `relay.go`, repurpose `m` key in voice TUI from mode-switch to mute-toggle.

**Rationale**: PTT and mute are conceptually similar (both pause audio processing) but mute is simpler (no long-poll pipe, no separate keybinding coordination). Removing PTT first (User Story 5, P3) before adding mute prevents conflicting code paths.

**Alternatives considered**:
- Keep PTT as a separate mode alongside mute: Rejected. The spec explicitly removes PTT in favor of mute. Keeping both adds complexity without user value (mute covers the same use case).

## R6: How should the heartbeat mechanism work?

**Decision**: The voice CLI sends `[[voice:ping]]` on the `cc-deck:voice` pipe every 5 seconds. The controller records `voice_last_ping_ms` on each ping (and on `[[voice:on]]`). In `handle_timer()`, if `voice_enabled` is true and `now - voice_last_ping_ms > 15000`, the controller sets `voice_enabled = false`, `voice_muted = false`, and `mark_render_dirty()`.

**Rationale**: The timer tick already runs every ~1 second (configurable via `config.timer_interval`). Adding a timestamp check is trivial. 15 seconds (3 missed pings) provides tolerance for brief pipe delays without leaving stale state for too long.

**Alternatives considered**:
- Rely on pipe error detection: Rejected. Zellij pipes are fire-and-forget from the CLI side. The plugin has no way to detect a crashed CLI process without active heartbeating.
