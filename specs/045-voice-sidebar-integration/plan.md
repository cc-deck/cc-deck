# Implementation Plan: Voice Sidebar Integration

**Branch**: `045-voice-sidebar-integration` | **Date**: 2026-04-29 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `specs/045-voice-sidebar-integration/spec.md`

## Summary

Add voice relay state visibility to the cc-deck sidebar (♫ indicator with bright/dim for listening/muted), bidirectional mute toggling (sidebar keybinding/click and voice TUI), a structured `[[command]]` protocol replacing raw text for control signals, and removal of PTT mode. The implementation spans both the Rust plugin (controller + sidebar) and the Go CLI (voice relay + TUI).

## Technical Context

**Language/Version**: Rust stable (edition 2021, wasm32-wasip1 target) for plugin; Go 1.25 for CLI
**Primary Dependencies**: zellij-tile 0.43.1 (plugin SDK), serde/serde_json 1.x; cobra (CLI), encoding/json (Go stdlib)
**Storage**: WASI `/cache/` for plugin state (sessions.json); no new persistent storage
**Testing**: `cargo test` (plugin unit tests), `go test ./...` (CLI unit tests), `make test` / `make lint`
**Target Platform**: WASM (plugin), macOS/Linux (CLI)
**Project Type**: CLI + Zellij WASM plugin
**Performance Goals**: Mute toggle reflects in sidebar within 200ms (CLI-initiated) or 1s (sidebar-initiated via dump-state poll)
**Constraints**: No new pipes; reuse existing `cc-deck:voice` (CLI-to-plugin) and `cc-deck:dump-state` (plugin-to-CLI poll)
**Scale/Scope**: ~10 files modified across 2 codebases (Rust plugin, Go CLI)

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| Tests + documentation | PASS | Unit tests required for command protocol parsing, voice state management, mute toggle. CLI reference and Antora docs for `Alt+v` / `voice_key` config. |
| Interface contracts | PASS | Voice command protocol is a new interface; contract defined in spec (FR-007 through FR-012). Will document in `contracts/`. |
| Build/tool rules | PASS | Use `make install`, `make test`, `make lint`. No direct cargo/go build. Use internal/xdg, podman. |

## Project Structure

### Documentation (this feature)

```text
specs/045-voice-sidebar-integration/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── contracts/           # Phase 1 output
│   └── voice-command-protocol.md
└── tasks.md             # Phase 2 output (from /speckit.tasks)
```

### Source Code (repository root)

```text
cc-zellij-plugin/
├── src/
│   ├── lib.rs                          # Shared types: add voice_connected + voice_muted to RenderPayload
│   ├── pipe_handler.rs                 # Parse [[command]] protocol in VoiceText variant
│   ├── controller/
│   │   ├── state.rs                    # Add voice_muted, voice_last_ping_ms fields
│   │   ├── mod.rs                      # Handle voice commands, heartbeat timeout, dump-state voice fields
│   │   ├── events.rs                   # Add voice heartbeat check in handle_timer
│   │   └── render_broadcast.rs         # Include voice state in RenderPayload
│   ├── sidebar_plugin/
│   │   ├── render.rs                   # Render ♫ indicator in header, click region
│   │   ├── input.rs                    # Handle `v` key in nav mode, ♫ click
│   │   └── state.rs                    # Track voice state from RenderPayload
│   └── config.rs                       # Add voice_key config field

cc-deck/
├── internal/
│   ├── voice/
│   │   ├── relay.go                    # Add command protocol, heartbeat, mute state, remove PTT
│   │   └── relay_test.go              # Test command protocol parsing
│   ├── tui/voice/
│   │   ├── model.go                   # Add muted state, remove mode
│   │   ├── update.go                  # Handle 'm' key for mute, remove PTT logic
│   │   └── view.go                    # Show MUTED in header
│   └── cmd/
│       └── ws_voice.go                # Remove --mode flag, add mute keybinding info
```

**Structure Decision**: No new directories or modules. All changes fit within the existing voice relay (Go) and plugin (Rust) codebases. The command protocol is an extension of the existing `cc-deck:voice` pipe message handling.

## Complexity Tracking

No constitution violations to justify.
