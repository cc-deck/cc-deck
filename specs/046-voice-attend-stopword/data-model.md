# Data Model: Voice Attend Stop Word

**Date:** 2026-04-30

## Entities

### Command Action (Go: DefaultCommands)

Extends the existing `DefaultCommands` map in `stopword.go`:

| Action Name | Default Words | Payload | Description |
|-------------|---------------|---------|-------------|
| submit | send | `[[enter]]` | Sends carriage return to attended pane |
| attend | next | `[[attend]]` | Cycles to next attended session (tiered) |

### TranscriptionResult (Go: existing struct)

No changes needed. The existing struct already carries `CommandAction` as a string field.

### PipeAction::VoiceText (Rust: existing enum variant)

No changes needed. The existing `VoiceText(String)` variant already carries the payload string which is parsed for `[[command]]` syntax.

## State Transitions

No new state transitions. The `[[attend]]` command reuses the existing attend logic which manages its own state (last_attended_pane_id, done_attended flags).

## Data Flow

```
Speech → Whisper transcription → "next"
  → ProcessStopwords("next", commands) → TranscriptionResult{IsCommand: true, Action: "attend"}
  → relay switch: "attend" → payload = "[[attend]]"
  → pipe.Send("cc-deck:voice", "[[attend]]")
  → Rust: parse_pipe_message → PipeAction::VoiceText("[[attend]]")
  → handle_voice_command("attend")
  → perform_attend() → tiered session cycling
```
