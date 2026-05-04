# Implementation Plan: Voice Transcript Recording

**Branch**: `048-voice-transcript-recording` | **Date**: 2026-05-04 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `specs/048-voice-transcript-recording/spec.md`

## Summary

Add transcript recording controls to the voice relay TUI. Users can start/pause/resume/stop recording via keyboard shortcuts (`r`/`R`). Transcribed utterances are written as plain text lines to a file. When muted with recording active, the relay still transcribes but skips sending to the pane. A visual indicator in the TUI header shows recording state.

## Technical Context

**Language/Version**: Go 1.25 (CLI/voice relay TUI)
**Primary Dependencies**: charmbracelet/bubbletea (TUI), charmbracelet/bubbles v1.0.0 (textinput, viewport), charmbracelet/lipgloss (styling)
**Storage**: Plain text files in `$XDG_DATA_HOME/cc-deck/transcripts/`
**Testing**: `go test` (Go side)
**Target Platform**: macOS/Linux (CLI)
**Project Type**: CLI with TUI
**Performance Goals**: N/A (file writes are line-at-a-time, negligible overhead)
**Constraints**: N/A
**Scale/Scope**: 2 existing files modified, 2 new files created, ~200 lines of production code

## Constitution Check

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Tests and documentation | PASS | Unit tests planned. Voice documentation task included. |
| II. Interface contracts | N/A | No new interface implementations |
| III. Build and tool rules | PASS | Using `make test`, `make lint`. No direct `go build` or `cargo build`. |

## Project Structure

### Documentation (this feature)

```text
specs/048-voice-transcript-recording/
├── spec.md
├── plan.md              # This file
├── research.md
├── data-model.md
├── REVIEW-SPEC.md
└── checklists/
    └── requirements.md
```

### Source Code (files to modify/create)

```text
cc-deck/internal/voice/
├── relay.go             # Add recording field + mute-bypass logic
└── relay_test.go        # Add test for mute+recording behavior

cc-deck/internal/tui/voice/
├── model.go             # Add recording state fields + textinput
├── update.go            # Add key handlers, prompt sub-mode, transcript capture
├── view.go              # Add header indicator, footer hints, prompt rendering
├── transcript.go        # NEW: recording types, helpers, file I/O
└── transcript_test.go   # NEW: tests for state machine, file output

docs/modules/using/pages/
└── voice.adoc           # Document recording feature + keyboard controls
```

**Structure Decision**: One new file (`transcript.go`) keeps recording logic separate from existing TUI code. All relay changes are in existing `relay.go`.

## Implementation Approach

### Phase 1: Relay Recording Support

1. **relay.go**: Add `recording bool` field (protected by existing `sync.Mutex`), `SetRecording(bool)` and `IsRecording() bool` methods following the `muted`/`IsMuted()` pattern (lines 59-84)
2. **relay.go**: Modify `handleUtterance()` (line 382-388): if muted AND recording, still transcribe and emit `"transcription"` event but skip stopword processing and pipe send. If muted AND not recording, discard as before.
3. **relay_test.go**: Add test verifying muted+recording transcribes and emits event, muted+not-recording discards

### Phase 2: Transcript File I/O

4. **transcript.go** (new): Define `recStatus` type (idle/prompting/recording/paused), helper functions: `defaultTranscriptDir()`, `resolveTranscriptPath()`, `defaultTranscriptName()`, `writeTranscriptLine()`, `closeTranscript()`
5. **transcript_test.go** (new): Tests for path resolution, line writing, state machine

### Phase 3: TUI Integration

6. **model.go**: Add fields (`recState`, `recFile`, `recPath`, `recCount`, `recInput textinput.Model`), initialize textinput in `New()`
7. **update.go**: Add prompt sub-mode routing (after devicePick check), `r`/`R` key handlers, `updateFilenamePrompt()` method, transcript write in `"transcription"` event handler, `relay.SetRecording()` calls, quit-path cleanup
8. **view.go**: Add `recStyle`/`pauseStyle` styles, recording indicator in `renderHeader()`, filename prompt in `renderFooter()`, updated footer hints with recording keys, conditional `footerHeight()`

### Phase 4: Documentation

9. **voice.adoc**: Add `r`/`R` to keyboard controls table, add "Transcript Recording" subsection

## Complexity Tracking

No constitution violations to justify.
