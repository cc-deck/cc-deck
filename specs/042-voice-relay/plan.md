# Implementation Plan: Voice Relay

**Branch**: `042-voice-relay` | **Date**: 2026-04-24 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/042-voice-relay/spec.md`

## Summary

Voice relay enables hands-free dictation to remote agent sessions by capturing audio locally, transcribing via a local Whisper model, and relaying text through PipeChannel to the attended pane in any workspace type. The feature spans both the Go CLI (audio pipeline, TUI, transcription, model management) and the Rust WASM plugin (text injection, PTT keybinding, permission-state pausing). PipeChannel.SendReceive must be implemented to enable bidirectional communication for the PTT long-poll pattern.

## Technical Context

**Language/Version**: Go 1.25 (CLI), Rust stable edition 2021 wasm32-wasip1 (plugin)
**Primary Dependencies**: cobra (CLI), charmbracelet/bubbletea + lipgloss + bubbles (TUI), gen2brain/malgo (audio, CGo), zellij-tile 0.43.1 (plugin SDK), serde/serde_json (plugin serialization)
**Storage**: `~/.cache/cc-deck/models/` (whisper models, XDG cache), WASI `/cache/` (plugin state)
**Testing**: `go test` (Go), `cargo test` (Rust, native target with WASM stubs)
**Target Platform**: macOS (CoreAudio), Linux (PulseAudio/ALSA)
**Project Type**: CLI tool + Zellij WASM plugin (two-component architecture)
**Performance Goals**: <5s end-to-end latency (speech-to-text-to-pane), <500ms PTT toggle latency
**Constraints**: <200 MB RSS for voice process, <100KB binary size increase, CGo-free fallback via build tags
**Scale/Scope**: Single developer, single workspace, English-only default

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Two-Component Architecture | PASS | Feature spans Go CLI (voice pipeline) and Rust plugin (text injection, PTT, permission pausing) |
| II. Plugin Installation | PASS | Will use `make install` for all builds |
| III. WASM Filename Convention | PASS | No changes to WASM naming |
| IV. WASM Host Function Gating | PASS | New plugin functions (`write_chars_to_pane_id`, `cli_pipe_output`) will be `#[cfg(target_family = "wasm")]` gated |
| V. Zellij API Research Order | PASS | Will verify `write_chars_to_pane_id` and keybinding APIs against docs/source |
| VI. Build via Makefile Only | PASS | All builds via `make install`, `make test`, `make lint` |
| VII. Interface Behavioral Contracts | PASS | PipeChannel.SendReceive extends existing Channel interface; behavioral contract already documented in spec 041 |
| VIII. Simplicity | PASS | Minimal abstractions: AudioSource, Transcriber, VoiceRelay interfaces. No unnecessary indirection |
| IX. Documentation Freshness | REQUIRED | Must update README, CLI reference, and potentially landing page |
| X. Spec Tracking in README | REQUIRED | Add 042-voice-relay to spec table |
| XI. Release Process | N/A | No release changes |
| XII. Prose Plugin | REQUIRED | All documentation must use prose plugin with cc-deck voice profile |
| XIII. XDG Paths | PASS | Models cached at `~/.cache/cc-deck/models/` using internal xdg helper |
| XIV. No Dotfile Nesting | N/A | No dotfile directories introduced |

**Gate Status**: PASS (all NON-NEGOTIABLE principles satisfied, REQUIRED items tracked for implementation)

## Project Structure

### Documentation (this feature)

```text
specs/042-voice-relay/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output
└── tasks.md             # Phase 2 output
```

### Source Code (repository root)

```text
cc-deck/                        # Go CLI
├── cmd/cc-deck/
│   └── ws/
│       ├── voice.go            # cc-deck ws voice command
│       └── pipe.go             # cc-deck ws pipe command
├── internal/
│   ├── voice/
│   │   ├── audio.go            # AudioSource interface
│   │   ├── audio_malgo.go      # malgo backend (CGo, //go:build cgo)
│   │   ├── audio_ffmpeg.go     # ffmpeg backend (//go:build !cgo)
│   │   ├── vad.go              # Energy-based VAD
│   │   ├── transcriber.go      # Transcriber interface
│   │   ├── transcriber_http.go # whisper-server HTTP client
│   │   ├── transcriber_cli.go  # whisper-cli subprocess
│   │   ├── stopword.go         # Stopword detection engine
│   │   ├── relay.go            # VoiceRelay orchestrator
│   │   ├── setup.go            # Model download + dependency check
│   │   └── server.go           # whisper-server lifecycle management
│   ├── ws/
│   │   └── channel_pipe.go     # SendReceive implementation (extend existing)
│   └── tui/
│       └── voice/
│           ├── model.go        # Bubbletea model
│           ├── view.go         # TUI rendering
│           └── update.go       # Event handling
└── test/
    └── voice/                  # Integration tests

cc-zellij-plugin/               # Rust WASM plugin
└── src/
    ├── pipe_handler.rs         # Extend PipeAction enum with voice variants
    ├── controller/
    │   ├── mod.rs              # Handle voice pipes
    │   ├── events.rs           # F8 keybinding registration
    │   └── state.rs            # voice_control_pipe, voice_buffer fields
    └── session.rs              # Activity::Waiting(Permission) already exists
```

## Complexity Tracking

No constitution violations requiring justification.
