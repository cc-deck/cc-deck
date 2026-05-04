# Data Model: Voice Transcript Recording

**Date:** 2026-05-04

## Entities

### Recording State (TUI Model)

| Field | Description |
|-------|-------------|
| recState | Current state: idle, prompting, recording, paused |
| recFile | Open file handle for the transcript (nil when idle) |
| recPath | Resolved filesystem path of the transcript file |
| recCount | Number of lines written to the transcript |
| recInput | Text input component for filename prompt |

### Relay Recording Flag

| Field | Description |
|-------|-------------|
| recording | Whether transcript recording is active (controls mute-bypass behavior) |

## State Transitions

```
idle --[r pressed]--> prompting
prompting --[Enter]--> recording (file opened, relay.SetRecording(true))
prompting --[Esc]--> idle
recording --[r pressed]--> paused
paused --[r pressed]--> recording
recording --[R pressed]--> idle (file closed, relay.SetRecording(false))
paused --[R pressed]--> idle (file closed, relay.SetRecording(false))
recording --[q/ctrl+c]--> quit (file closed, relay.SetRecording(false))
paused --[q/ctrl+c]--> quit (file closed, relay.SetRecording(false))
```

## Data Flow

### Normal recording (not muted)
```
Speech -> VAD -> Whisper transcription -> "text"
  -> ProcessStopwords -> relay dispatch -> pipe send (text to pane)
  -> RelayEvent{Type: "transcription", Text: "text"}
  -> TUI: append to history + write to transcript file
```

### Recording while muted
```
Speech -> VAD -> Whisper transcription -> "text"
  -> RelayEvent{Type: "transcription", Text: "text"}
  -> (skip stopword processing and pipe send)
  -> TUI: append to history + write to transcript file
```

### Recording while paused
```
Speech -> VAD -> Whisper transcription -> "text"
  -> (normal relay flow, pipe send if not muted)
  -> RelayEvent{Type: "transcription", Text: "text"}
  -> TUI: append to history (but do NOT write to transcript file)
```
