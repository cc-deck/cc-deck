# Voice Command Protocol Contract

## Overview

The voice command protocol defines how the Go voice relay CLI communicates with the Rust Zellij plugin through the `cc-deck:voice` named pipe. It separates control signals from dictation text using a `[[command]]` syntax.

## Message Format

All messages are sent as the payload of a `zellij pipe` call to the `cc-deck:voice` pipe name.

### Control Messages

Control messages are wrapped in double brackets: `[[command]]` or `[[namespace:command]]`.

```
[[voice:on]]       # Voice relay connected
[[voice:off]]      # Voice relay disconnecting
[[voice:mute]]     # Voice relay muted
[[voice:unmute]]   # Voice relay unmuted (listening)
[[enter]]          # Submit command (send carriage return)
```

Note: `[[voice:ping]]` is accepted for backwards compatibility but no longer sent by the CLI. Heartbeat is driven by dump-state polling instead (see Connection Lifecycle below).

### Dictation Text

Any payload that does NOT start with `[[` is treated as dictated text and injected into the attended pane via `write_chars_to_pane_id`.

## Detection Algorithm

```
fn parse_voice_payload(payload: &str) -> VoiceAction {
    if payload.starts_with("[[") && payload.ends_with("]]") {
        let command = &payload[2..payload.len()-2];
        match command {
            "voice:on"     => VoiceAction::Connect,
            "voice:off"    => VoiceAction::Disconnect,
            "voice:ping"   => VoiceAction::Ping,
            "voice:mute"   => VoiceAction::Mute,
            "voice:unmute" => VoiceAction::Unmute,
            "enter"        => VoiceAction::Enter,
            _              => VoiceAction::Unknown(command),
        }
    } else {
        VoiceAction::InjectText(payload)
    }
}
```

## Behavioral Requirements

### Connection Lifecycle

1. CLI MUST send `[[voice:on]]` before any other voice messages
2. CLI MUST send `[[voice:off]]` on graceful shutdown
3. Heartbeat is driven by `cc-deck:dump-state` polling (every 1 second). The plugin refreshes `voice_last_ping_ms` on each dump-state request when voice is enabled. No dedicated `[[voice:ping]]` messages are sent, avoiding the risk of text injection from unrecognized commands on the voice pipe.
4. Plugin MUST clear voice state after 15 seconds without a dump-state request
5. `[[voice:on]]` from a new connection MUST reset any stale state from a previous connection

### Mute State

1. `[[voice:mute]]` and `[[voice:unmute]]` MUST only be sent when voice is connected
2. On disconnect (explicit or heartbeat timeout), mute state MUST reset to false
3. The CLI is the authoritative source of mute state; the plugin MUST NOT update `voice_muted` except in response to `[[voice:mute]]` or `[[voice:unmute]]`

### Text Injection

1. Plain text MUST be injected into the attended pane (same targeting logic as current `VoiceText` handler)
2. `[[enter]]` MUST send a carriage return (`\r`) to the attended pane
3. Text and commands MUST NOT be injected when no session pane is available (discard silently)

### Backwards Compatibility

1. The existing `cc-deck:voice` pipe name is reused (no pipe name change)
2. The `PipeAction::VoiceText(String)` enum variant continues to carry the raw payload
3. Command detection happens inside the controller's match arm, not in `parse_pipe_message()`

## Dump-State Response Extension

The `cc-deck:dump-state` response includes voice state for CLI consumption:

```json
{
  "sessions": { ... },
  "attended_pane_id": 42,
  "voice_mute_requested": true
}
```

| Field | Type | Description |
|-------|------|-------------|
| `voice_mute_requested` | `bool` or absent | When present and `true`, sidebar has requested mute. When `false`, sidebar has requested unmute. When absent/null, no pending request. |

The CLI clears the pending request by sending the appropriate `[[voice:mute]]` or `[[voice:unmute]]` command.
