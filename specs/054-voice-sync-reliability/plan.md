# Implementation Plan: Voice Sync Reliability

**Branch**: `054-voice-sync-reliability` | **Date**: 2026-05-14 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/054-voice-sync-reliability/spec.md`

## Summary

Fix four voice sync reliability issues: indicator flickering, disappearance on session switch, wrong session name in voice relay TUI, and mute state loss after recovery. The changes span the Rust plugin (controller voice handler, dump-state response, sidebar registration) and the Go CLI (relay heartbeat protocol, session name resolution).

## Technical Context

**Language/Version**: Rust stable (edition 2021, wasm32-wasip1 target) for plugin; Go 1.25 for CLI
**Primary Dependencies**: zellij-tile 0.43.1 (plugin SDK), serde/serde_json 1.x; cobra (CLI), encoding/json (Go stdlib)
**Storage**: WASI `/cache/` directory for plugin state
**Testing**: `cargo test` (Rust unit tests), `go test` (Go unit tests), manual visual verification
**Target Platform**: WASM (wasm32-wasip1) plugin running inside Zellij terminal multiplexer
**Project Type**: Terminal plugin + CLI tool
**Performance Goals**: Zero render broadcasts at steady state (no state changes), voice indicator stable for 60+ seconds
**Constraints**: 1-second heartbeat interval, 15-second timeout, backward compatibility with older relays
**Scale/Scope**: 14 concurrent tabs/sessions, single user

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Tests and documentation | PASS | Unit tests required for changed handlers. No new CLI commands or config options, so CLI reference and config reference unchanged. README update for voice sync behavior. |
| II. Interface behavioral contracts | PASS | dump-state response is an internal protocol (not a user-facing interface contract). Backward compatibility maintained via fallback handling. |
| III. Build and tool rules | PASS | Using `make test`, `make lint`. No direct `go build` or `cargo build`. |

## Project Structure

### Documentation (this feature)

```text
specs/054-voice-sync-reliability/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
└── tasks.md             # Phase 2 output (created by /speckit.tasks)
```

### Source Code (repository root)

```text
cc-zellij-plugin/src/
├── controller/
│   ├── mod.rs               # handle_voice_command(), dump_state()
│   ├── sidebar_registry.rs  # handle_sidebar_hello() + targeted render
│   └── render_broadcast.rs  # broadcast_render(), send_render_to_plugin()
└── lib.rs                   # RenderPayload struct (voice_connected, voice_muted)

cc-deck/internal/voice/
└── relay.go                 # statePoll(), parseDumpStateResponse()
```

**Structure Decision**: Existing files only. No new files needed. All changes are modifications to existing handlers and structs.

## Design

### Change 1: Heartbeat carries mute state (FR-001, FR-006)

**Current behavior**: The Go relay sends bare `[[voice:on]]` on every 1-second tick (relay.go:278). The controller's `handle_voice_command()` only processes mute state on the first enable (`if !was_enabled`, mod.rs:490-496).

**New behavior**: The relay sends `[[voice:on:muted]]` or `[[voice:on:unmuted]]` on every tick, reflecting its current `r.muted` state. The controller extracts the mute suffix on every heartbeat and updates `voice_muted`. `mark_render_dirty()` is called only when `voice_enabled` or `voice_muted` actually changes.

**Files changed**:
- `cc-deck/internal/voice/relay.go` (line 278): Change `"[[voice:on]]"` to `fmt.Sprintf("[[voice:on:%s]]", muteState)` where muteState is `"muted"` or `"unmuted"` based on `r.muted`.
- `cc-zellij-plugin/src/controller/mod.rs` (lines 486-496): Restructure the `voice:on` handler to:
  1. Always set `voice_enabled = true` and refresh `voice_last_ping_ms`
  2. Always parse mute suffix: `"muted"` -> true, anything else (including bare `voice:on`) -> false
  3. Only call `mark_render_dirty()` when `voice_enabled` changed (was false, now true) OR `voice_muted` changed
  4. Always clear `voice_mute_requested` on first enable

### Change 2: dump-state includes focused_pane_id (FR-003)

**Current behavior**: `DumpStateResponse` includes `attended_pane_id` (from `state.last_attended_pane_id`) but not `focused_pane_id`.

**New behavior**: Add `focused_pane_id: Option<u32>` to `DumpStateResponse`, populated from `state.focused_pane_id`.

**Files changed**:
- `cc-zellij-plugin/src/controller/mod.rs` (lines 560-571): Add `focused_pane_id` field to `DumpStateResponse` struct and populate it.

### Change 3: Relay prefers focused_pane_id for session name (FR-004)

**Current behavior**: `parseDumpStateResponse()` resolves session name from `attended_pane_id` only, with single-session fallback.

**New behavior**: Resolution priority becomes:
1. `focused_pane_id` (if present and maps to a session with a display_name)
2. `attended_pane_id` (if present and maps to a session with a display_name)
3. Single-session fallback (if exactly one session exists)

**Files changed**:
- `cc-deck/internal/voice/relay.go` (lines 332-383): Add `FocusedPaneID *int` to envelope struct, try it first in resolution logic.

### Change 4: Targeted render on sidebar registration (FR-005)

**Current behavior**: `handle_sidebar_hello()` registers the sidebar and sends `cc-deck:sidebar-init` (tab assignment + controller ID) but does not send a render payload.

**New behavior**: After sending `sidebar-init`, also send the current render payload to the newly registered sidebar. This ensures new sidebars immediately show voice state (and all session data) without waiting for the next dirty render cycle.

**Files changed**:
- `cc-zellij-plugin/src/controller/sidebar_registry.rs` (lines 15-36): After `send_sidebar_init()`, call a new `send_render_to_sidebar()` helper that builds and sends a targeted render payload to the single sidebar.
- `cc-zellij-plugin/src/controller/render_broadcast.rs`: Expose `send_render_to_plugin()` (currently private/wasm-gated) for use from sidebar_registry, or add a `targeted_render()` public function that builds payload + sends to one plugin_id.

### Change 5: Backward compatibility (Assumptions)

**Current behavior**: Bare `[[voice:on]]` is the only heartbeat format.

**New behavior**: The controller already handles `cmd.starts_with("voice:on:")` (mod.rs:486). Bare `[[voice:on]]` (no suffix) continues to work and is treated as unmuted. Unrecognized suffixes are also treated as unmuted. No additional code changes needed for backward compatibility, just the suffix parsing logic in Change 1.

## Testing Strategy

### Rust Unit Tests

1. **voice:on handler change-gating**: Test that `mark_render_dirty()` is NOT called when `voice_enabled` is already true and `voice_muted` has not changed. Test that it IS called when mute state transitions.
2. **Mute suffix parsing**: Test `voice:on:muted`, `voice:on:unmuted`, bare `voice:on`, and `voice:on:garbage` all produce correct `voice_muted` values.
3. **dump-state response**: Test that serialized response includes both `attended_pane_id` and `focused_pane_id`.
4. **Sidebar hello targeted render**: Test that `handle_sidebar_hello()` triggers a render payload send to the registered sidebar.

### Go Unit Tests

5. **Heartbeat format**: Test that the relay sends `[[voice:on:muted]]` when muted and `[[voice:on:unmuted]]` when unmuted.
6. **parseDumpStateResponse focused_pane_id**: Test that `focused_pane_id` is preferred over `attended_pane_id` for session name resolution. Test fallback when `focused_pane_id` is absent or doesn't map to a session.

### Manual Verification

7. Start voice relay, observe sidebar for 60 seconds (SC-001)
8. Switch between 5 sessions rapidly (SC-002)
9. Switch focus between sessions, verify TUI name updates within 2 seconds (SC-003)
10. Mute, wait for timeout + recovery, verify mute state preserved (SC-004)

## Complexity Tracking

No constitution violations. All changes are modifications to existing handlers within existing files.
