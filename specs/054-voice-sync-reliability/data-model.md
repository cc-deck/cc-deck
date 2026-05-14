# Data Model: Voice Sync Reliability

## Modified Entities

### DumpStateResponse (Rust, controller/mod.rs)

Controller-to-relay response for `cc-deck:dump-state` requests.

| Field | Type | Status | Description |
|-------|------|--------|-------------|
| sessions | BTreeMap<u32, Session> | Existing | All tracked sessions keyed by pane_id |
| attended_pane_id | Option<u32> | Existing | Last pane explicitly attended via sidebar action |
| focused_pane_id | Option<u32> | **New** | Currently focused pane from TabUpdate/PaneUpdate events |
| voice_mute_requested | Option<bool> | Existing | Pending mute toggle from sidebar (skip if None) |

### Voice heartbeat protocol message

Pipe message sent from Go relay to Rust controller every 1 second.

| Format | Status | Description |
|--------|--------|-------------|
| `[[voice:on]]` | Existing (backward compat) | Bare heartbeat, treated as unmuted |
| `[[voice:on:unmuted]]` | **New** | Heartbeat with explicit unmuted state |
| `[[voice:on:muted]]` | **New** | Heartbeat with explicit muted state |
| `[[voice:on:<other>]]` | **New** | Unrecognized suffix, treated as unmuted |

### ControllerState voice fields (Rust, controller/state.rs)

No new fields. Existing fields with clarified semantics:

| Field | Type | Initial | Description |
|-------|------|---------|-------------|
| voice_enabled | bool | false | True when relay is connected (heartbeat received) |
| voice_muted | bool | false | Relay's current mute state (updated every heartbeat) |
| voice_last_ping_ms | u64 | 0 | Timestamp of last heartbeat (for 15s timeout) |
| voice_mute_requested | Option<bool> | None | Pending mute toggle from sidebar UI |
| voice_mute_requested_ms | u64 | 0 | Timestamp of pending mute request (for 5s timeout) |

### dumpStateResult (Go, voice/relay.go)

Parsed response from dump-state in the relay.

| Field | Type | Status | Description |
|-------|------|--------|-------------|
| targetName | string | Existing | Resolved session display name |
| hasAttendedPane | bool | Existing | Whether attended_pane_id was present |
| hasFocusedPane | bool | **New** | Whether focused_pane_id was present |
| voiceMuteRequested | *bool | Existing | Pending mute state from controller |

## State Transitions

### Voice indicator lifecycle

```
[No Relay] --voice:on:unmuted--> [Connected/Unmuted] --voice:on:muted--> [Connected/Muted]
    ^                                   |                                      |
    |                                   |                                      |
    +------- 15s timeout ---------------+------- 15s timeout ------------------+
    |                                   |                                      |
    +------- voice:off -----------------+------- voice:off --------------------+
```

### Mute state through timeout/recovery

```
[Muted] --15s no heartbeat--> [Timed Out (voice_enabled=false)]
                                    |
                              voice:on:muted (relay reconnects)
                                    |
                                    v
                              [Muted] (state preserved)
```

## Session name resolution priority (Go relay)

```
1. focused_pane_id -> session lookup -> display_name (if found)
2. attended_pane_id -> session lookup -> display_name (if found)  
3. Single session fallback (if exactly one session exists)
4. Keep previous target (if none of the above resolves)
```
