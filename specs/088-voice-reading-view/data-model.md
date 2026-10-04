# Data Model: Voice Reading View with Turn Detection

## Utterance (passage), `internal/voice/audio.go`

| Field | Type | Notes |
|-------|------|-------|
| Audio | `[]int16` | existing |
| SampleRate | `int` | existing |
| Start | `time.Duration` | NEW. Offset of the first emitted sample (including pre-roll) from audio stream start |
| End | `time.Duration` | NEW. `Start` plus the emitted audio length (after hangover trim) |
| SpeechStart | `time.Duration` | NEW. Offset of the first loud frame (onset), without pre-roll |
| SpeechEnd | `time.Duration` | NEW. Offset just after the last loud frame, without hangover |

Invariant: `End >= Start`; for consecutive utterances `next.Start >= prev.End`.

## Segment, `internal/voice/turns.go`

| Field | Type | Notes |
|-------|------|-------|
| Text | `string` | Sanitized text (no dash marker, no bracket annotations, no control bytes) |
| Start | `time.Duration` | Offset from recording start |
| End | `time.Duration` | Offset from recording start |
| At | `time.Time` | Wall-clock time of `Start` (`recordingStartedAt + Start`) |
| TurnStart | `bool` | True when this segment begins a new turn |
| Speaker | `string` | Always empty in this feature; reserved for future labels |

Rules:
- The first segment of a recording has `TurnStart = true`.
- In `basic` mode, segments come from `SplitDashTurns`; a piece after a dash marker has `TurnStart = true`.
- In `tdrz` mode, segment `i+1` has `TurnStart = true` when segment `i` reported `speaker_turn_next`.
- In both modes, `segs[0].TurnStart` is forced true when the gap before the passage exceeds `PauseBreak`.
- Segments with empty text after sanitizing are dropped; if a dropped segment carried `TurnStart`, the flag moves to the next kept segment.
- Without timing information (basic mode), all segments of a passage share the passage `Start`/`End`.

## RelayEvent, `internal/voice/relay.go`

| Field | Change |
|-------|--------|
| Segments | NEW `[]Segment`. Set only for `transcription` events produced while recording; nil for dictation |
| Text | unchanged; for recording events it is the space-joined segment text |

## TurnMode, `internal/voice/turns.go`

`type TurnMode string` with constants `TurnModeBasic = "basic"` and `TurnModeTdrz = "tdrz"`. `ParseTurnMode(string) (TurnMode, error)` accepts exactly these two values (case-insensitive).

## RecordingConfig, `internal/voice/audio.go`

| Field | Type | Default | Config key |
|-------|------|---------|------------|
| Threshold | `float64` (RMS) | `PercentToThreshold(20)` | `defaults.voice.recording.threshold` (percent) |
| SilenceDuration | `float64` seconds | `1.0` | `defaults.voice.recording.silence` |
| MaxUtteranceDuration | `float64` seconds | `12` | `defaults.voice.recording.max_chunk` |
| PauseBreak | `time.Duration` | `2s` | `defaults.voice.recording.pause_break` (seconds); compared with `next.SpeechStart - prev.SpeechEnd` |

Validation: threshold 0 to 100; silence greater than 0 and at most 10; max_chunk at least 2 and at most 30; pause_break greater than silence. Invalid values produce config findings and fall back to defaults.

## Relay recording state (unexported)

| Field | Purpose |
|-------|---------|
| recording | existing |
| recordMode | effective `TurnMode` for the running recording |
| saved (threshold, silence, maxUtterance, muted) | dictation values restored by `StopRecording` |
| recStartWall, recStartOffset | wall clock and stream offset at recording start |
| lastRecEnd | `End` of the last recording passage, for the pause gap |

State transitions: `idle -> recording` via `StartRecording(mode)`; `recording -> idle` via `StopRecording()`. Pause and resume are TUI-only states (the relay keeps recording settings while paused).

## TUI state, `internal/tui/voice`

| Entity | Fields | Notes |
|--------|--------|-------|
| turnMode | `voice.TurnMode` | Session value, initialized from `defaults.voice.speaker_split`; `s` toggles while `recState == recIdle` |
| reading | `bool` | Reading view open; forced false by `closeTranscript` |
| turnBlock | `at time.Time`, `parts []string`, `speaker string` | One rendered block; text is `strings.Join(parts, " ")` |
| recBuffer | `[]turnBlock` | Reset at recording start; appended only while `recState == recRecording` |
| readView | `viewport.Model`, `follow bool`, `newBlocks int` | Follow state and unseen-block counter |
| download | `state (idle, prompting, running)`, `done, total int64`, `cancel context.CancelFunc`, `ch <-chan downloadMsg` | Background model download |
| fileTurnOpen | `bool` | Whether the transcript file already has text (controls blank-line insertion) |
