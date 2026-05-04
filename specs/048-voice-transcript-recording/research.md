# Research: Voice Transcript Recording

**Date:** 2026-05-04

## Decision: Recording State in Relay

- **Decision**: Add a `recording` bool field to `VoiceRelay` (protected by existing `sync.Mutex`) with `SetRecording(bool)` / `IsRecording() bool` methods
- **Rationale**: Follows the exact same pattern as the existing `muted` field (lines 61, 80-84 of relay.go). Using the existing mutex avoids introducing atomic types.
- **Alternatives considered**: `sync/atomic.Bool` for lock-free access. Rejected because the existing mute pattern uses `sync.Mutex` and the two fields are read together in `handleUtterance`.

## Decision: Mute Bypass When Recording

- **Decision**: In `handleUtterance()`, change the mute check (line 383) from early-return to conditional: if muted AND recording, still transcribe but skip stopword processing and pipe send. Emit `"transcription"` event so TUI and transcript file receive the text.
- **Rationale**: The user wants to capture speech while muted. Transcription must happen for that. But sending to the pane must be skipped, matching existing mute semantics.
- **Alternatives considered**: Separate codepath for muted+recording. Rejected as unnecessarily complex; a simple branch in the existing flow suffices.

## Decision: Textinput for Filename Prompt

- **Decision**: Use `charmbracelet/bubbles/textinput` for the filename prompt, following the device picker sub-mode pattern
- **Rationale**: Already an indirect dependency (v1.0.0 via viewport). The device picker pattern (`m.devicePick bool` + `updateDevicePicker` method) provides the exact template for the prompt sub-mode.
- **Alternatives considered**: Custom key-by-key input handling. Rejected as reinventing what textinput already provides.

## Decision: File Format

- **Decision**: Plain text, one utterance per line, no timestamps or metadata
- **Rationale**: User requirement. Makes files directly usable as input for other tools without parsing.

## Decision: Default Transcript Directory

- **Decision**: `xdg.DataHome + "/cc-deck/transcripts/"` (typically `~/.local/share/cc-deck/transcripts/`)
- **Rationale**: XDG Data Home is the correct location for user-created data files. Follows project convention from `xdg.go`.

## Code Locations

| Change | File | Lines |
|--------|------|-------|
| Add `recording` field + methods | `cc-deck/internal/voice/relay.go` | 59-84 |
| Modify mute check in handleUtterance | `cc-deck/internal/voice/relay.go` | 382-388 |
| New transcript types + helpers | `cc-deck/internal/tui/voice/transcript.go` | new file |
| Add recording fields to Model | `cc-deck/internal/tui/voice/model.go` | 14-33 |
| Add key handlers + prompt sub-mode | `cc-deck/internal/tui/voice/update.go` | 42-83 |
| Add header indicator + footer hints | `cc-deck/internal/tui/voice/view.go` | 79-141 |
| Add relay test for mute+recording | `cc-deck/internal/voice/relay_test.go` | append |
| Add transcript tests | `cc-deck/internal/tui/voice/transcript_test.go` | new file |
