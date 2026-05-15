# Data Model: Controller Leader Election

**Date**: 2026-05-14
**Feature**: 055-controller-leader-election

## Entities

### ControllerState (extended)

New fields added to the existing `ControllerState` struct:

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `is_leader` | `bool` | `false` | Whether this instance is the active leader |
| `leader_plugin_id` | `Option<u32>` | `None` | Plugin ID of the known leader (if not self) |
| `last_leader_ping_ms` | `u64` | `0` | Timestamp of last received leader ping |
| `election_ticks` | `u32` | `0` | Timer ticks since startup ping was sent |

### Election State Machine

```
                   ┌──────────┐
       startup ──> │ Dormant  │ <── ping from lower ID
                   └────┬─────┘
                        │
         2s timeout,    │ ping from higher ID
         no lower-ID    │ (respond with own ping)
         ping           │
                        v
                   ┌──────────┐
                   │  Leader  │ ──── heartbeat ping every 30s
                   └──────────┘
                        │
                        │ (crash / unload)
                        v
                   ┌──────────┐
         60s ───>  │ Re-elect │ ──── dormant instance activates
                   └──────────┘
```

### Ping Message Format

Pipe name: `cc-deck:controller-ping`
Payload: `plugin_id` as string (e.g., `"0"` or `"4"`)

The receiver parses the payload as `u32`. If parsing fails, the message is ignored.

## Constants

| Constant | Value | Description |
|----------|-------|-------------|
| `ELECTION_TIMEOUT_TICKS` | `2` | Timer ticks to wait before self-activating |
| `LEADER_HEARTBEAT_TICKS` | `30` | Ticks between leader heartbeat pings |
| `LEADER_FAILURE_TIMEOUT_MS` | `60_000` | Milliseconds without leader ping before re-activation |
