# Data Model: Voice Sidebar Integration

## Entities

### VoiceState (Plugin Side - ControllerState fields)

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `voice_enabled` | `bool` | `false` | Whether voice relay is connected (set by `[[voice:on]]`, cleared by `[[voice:off]]` or heartbeat timeout) |
| `voice_muted` | `bool` | `false` | Whether voice relay is muted (set by `[[voice:mute]]`, cleared by `[[voice:unmute]]`) |
| `voice_last_ping_ms` | `u64` | `0` | Timestamp of last `[[voice:ping]]` or `[[voice:on]]` message, in milliseconds |
| `voice_mute_requested` | `Option<bool>` | `None` | Pending mute toggle from sidebar: `Some(true)` = mute, `Some(false)` = unmute, `None` = no pending request |

**Lifecycle**:
- `voice_enabled` transitions: `false` -> `true` on `[[voice:on]]`; `true` -> `false` on `[[voice:off]]` or heartbeat timeout (15s)
- `voice_muted` transitions: toggled by `[[voice:mute]]` / `[[voice:unmute]]`; reset to `false` when `voice_enabled` becomes `false`
- `voice_mute_requested` transitions: set by sidebar mute toggle action; cleared when CLI acknowledges via `[[voice:mute/unmute]]`

### VoiceCommand (Protocol Messages)

| Command | Direction | Description |
|---------|-----------|-------------|
| `[[voice:on]]` | CLI -> Plugin | Voice relay started and connected |
| `[[voice:off]]` | CLI -> Plugin | Voice relay shutting down |
| `[[voice:ping]]` | CLI -> Plugin | Heartbeat (every 5s) |
| `[[voice:mute]]` | CLI -> Plugin | Voice relay is now muted |
| `[[voice:unmute]]` | CLI -> Plugin | Voice relay is now listening |
| `[[enter]]` | CLI -> Plugin | Submit command (translates to carriage return on attended pane) |
| Plain text (no `[[`) | CLI -> Plugin | Dictated text (injected via `write_chars_to_pane_id`) |

### RenderPayload Extensions

Two new fields added to the shared `RenderPayload` struct in `cc-zellij-plugin/src/lib.rs`:

| Field | Type | Description |
|-------|------|-------------|
| `voice_connected` | `bool` | Whether voice relay is connected (mirrors `voice_enabled`) |
| `voice_muted` | `bool` | Whether voice relay is muted |

These are set by `render_broadcast.rs` from `ControllerState` fields and consumed by sidebar rendering.

### DumpStateResponse Extensions

One new field added to the dump-state response in `controller/mod.rs`:

| Field | Type | Description |
|-------|------|-------------|
| `voice_mute_requested` | `Option<bool>` | Pending sidebar mute toggle for CLI consumption |

### PluginConfig Extensions

One new field in `config.rs`:

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `voice_key` | `String` | `"Alt m"` | Keybinding for global voice mute toggle |

## Relationships

```
VoiceRelay (Go CLI)
  --[[[voice:on/off/ping/mute/unmute/enter]] + plain text]--> cc-deck:voice pipe --> ControllerState
  <--[dump-state response with voice_mute_requested]-- cc-deck:dump-state pipe <-- ControllerState

ControllerState
  --[RenderPayload with voice_connected, voice_muted]--> Sidebar(s)

Sidebar
  --[ActionMessage with VoiceMute action]--> Controller --> sets voice_mute_requested
```

## State Consistency

The mute state has two sources of truth that synchronize:
1. **CLI side**: The Go voice relay tracks its own `muted` flag, which controls VAD processing
2. **Plugin side**: `ControllerState.voice_muted` controls sidebar indicator rendering

Synchronization rules:
- CLI-initiated mute: CLI sets local flag, sends `[[voice:mute]]`, plugin updates `voice_muted`, broadcasts to sidebars
- Sidebar-initiated mute: Plugin sets `voice_mute_requested`, CLI reads on next poll, CLI sets local flag, CLI sends `[[voice:mute]]`, plugin updates `voice_muted`, broadcasts to sidebars
- Both paths converge: the plugin's `voice_muted` is only updated by `[[voice:mute/unmute]]` messages, never directly by the sidebar toggle. This ensures the CLI is always the authoritative source of mute state.
