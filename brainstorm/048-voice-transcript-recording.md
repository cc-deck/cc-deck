# Brainstorm: Voice Transcript Recording

**Date:** 2026-05-04
**Status:** active

## Problem Framing

The voice relay TUI shows transcribed speech in a scrollable history, but there is no way to save a session transcript to a file.
Users want to capture what was dictated for later reference, review, or documentation.

## Approaches Considered

### A: Inline recording with TUI controls
- Add `r`/`R` key bindings for record/pause/stop within the existing voice TUI
- Prompt for filename via bubbletea textinput component
- Show recording indicator in the TUI header
- Pros: No new commands, lives inside the existing TUI, immediate control
- Cons: Adds complexity to the bubbletea model

### B: Separate CLI flag (--transcript)
- Start recording via CLI flag when launching voice relay
- Pros: Simple, no TUI changes
- Cons: No pause/resume, no runtime control, must restart relay to change

## Decision

**Approach A** chosen. Inline TUI controls with record/pause/resume/stop.

## Design Decisions

### Key bindings
- `r`: Start recording (prompts for filename), or toggle pause/resume when active
- `R`: Stop recording and close file
- `Enter`/`Esc` in filename prompt to confirm/cancel

### State machine
```
idle --[r]--> prompting --[Enter]--> recording --[r]--> paused --[r]--> recording
                |                       |                  |
              [Esc]                   [R]                [R]
                v                       v                  v
              idle                    idle               idle
```

### Recording behavior
- Records transcriptions regardless of Claude session connection status or mute state
- When **muted + recording**: relay still transcribes via Whisper but skips the pipe send to the pane (requires relay change to bypass mute-discard when recording is active)
- When muted without recording, audio is discarded as before (no CPU cost)
- Only **pause** stops writing to the transcript file
- On quit, flush and close file if recording is active

### File format
Plain text, one transcription per line, no timestamps or metadata:
```
Hello, this is a test transcription
Another line of speech here
Final line after resume
```
Pure recognized text, directly usable as input for other tools.

### Default file location
`$XDG_DATA_HOME/cc-deck/transcripts/`.
Relative filenames resolved against this directory, absolute paths used as-is.
Default filename suggestion: `transcript-YYYY-MM-DDTHH-MM-SS.txt`.

### TUI header indicator
Added to the device/mode line (no new header line):
- Recording: red `● REC`
- Paused: yellow `⏸ REC`

### Relay changes
- Add `recording` atomic bool to `VoiceRelay` with `SetRecording(bool)` / `IsRecording() bool`
- In `handleUtterance()`: if muted AND recording, still transcribe but skip pipe send
- If muted AND not recording, discard as before

### Files to modify
- `cc-deck/internal/voice/relay.go` (recording state, mute bypass)
- `cc-deck/internal/tui/voice/transcript.go` (new, types + helpers)
- `cc-deck/internal/tui/voice/model.go` (recording fields + textinput)
- `cc-deck/internal/tui/voice/update.go` (key handlers, prompt mode)
- `cc-deck/internal/tui/voice/view.go` (header indicator, footer hints)
- `docs/modules/using/pages/voice.adoc` (documentation)

## Open Threads
- Whether to add a CLI flag for auto-start recording without the interactive prompt
- Whether the transcript should include command words ("send", "next") or only dictation text
