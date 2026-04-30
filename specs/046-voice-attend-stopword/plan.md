# Implementation Plan: Voice Attend Stop Word

**Branch**: `046-voice-attend-stopword` | **Date**: 2026-04-30 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `specs/046-voice-attend-stopword/spec.md`

## Summary

Add a configurable voice stop word (default: "next") that triggers attend-next session cycling, matching the Alt+a keyboard shortcut behavior. This extends the existing stop word system with a new "attend" action, adds `[[attend]]` to the voice command protocol, and handles it in the Rust plugin controller.

## Technical Context

**Language/Version**: Go 1.25 (CLI/voice relay), Rust stable edition 2021 wasm32-wasip1 (plugin)
**Primary Dependencies**: zellij-tile 0.43.1 (plugin SDK), serde/serde_json 1.x
**Storage**: N/A
**Testing**: `go test` (Go side), `cargo test` (Rust side)
**Target Platform**: WASM (plugin), macOS/Linux (CLI)
**Project Type**: CLI + WASM plugin
**Performance Goals**: N/A (voice latency dominated by transcription, not dispatch)
**Constraints**: N/A
**Scale/Scope**: 3 files modified, ~20 lines of production code, ~50 lines of tests

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Tests and documentation | PASS | Unit tests planned for Go and Rust. Documentation task included. |
| II. Interface contracts | N/A | No new interface implementations |
| III. Build and tool rules | PASS | Using `make test`, `make lint`. No direct `go build` or `cargo build`. |

## Project Structure

### Documentation (this feature)

```text
specs/046-voice-attend-stopword/
├── spec.md
├── plan.md              # This file
├── research.md
├── data-model.md
├── REVIEW-SPEC.md
└── checklists/
    └── requirements.md
```

### Source Code (files to modify)

```text
cc-deck/internal/voice/
├── stopword.go          # Add "attend" to DefaultCommands
├── stopword_test.go     # Add test cases for "attend"/"next"
├── relay.go             # Add "attend" case to dispatch switch
└── relay_test.go        # Add test for [[attend]] payload

cc-zellij-plugin/src/
├── controller/
│   └── mod.rs           # Add "attend" match arm in handle_voice_command()
└── attend.rs            # No changes (reuse existing perform_attend)
```

**Structure Decision**: No new files or directories. All changes extend existing code in the voice relay (Go) and plugin controller (Rust).

## Implementation Approach

### Phase 1: Go Side (stop word + relay)

1. **stopword.go**: Add `"attend": {"next"}` to `DefaultCommands` map (line 18)
2. **relay.go**: Add `case "attend": payload = "[[attend]]"` to the switch statement (line 431)
3. **stopword_test.go**: Add test cases for "next" triggering "attend" action (defaults and custom)
4. **relay_test.go**: Add test verifying "next" produces `[[attend]]` payload on the voice pipe

### Phase 2: Rust Side (plugin handler)

5. **controller/mod.rs**: Add `"attend"` match arm in `handle_voice_command()` that calls the attend logic (around line 492)

### Phase 3: Documentation

6. **Configuration reference**: Document the new "attend" command word in configuration docs
7. **Voice relay guide**: Update to mention the "next" command alongside "send"

## Complexity Tracking

No constitution violations to justify.
