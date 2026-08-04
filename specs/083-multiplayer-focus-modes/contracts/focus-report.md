# Contract: cc-deck:focus-report Pipe Message

**Date**: 2026-07-22
**Direction**: Sidebar -> Controller
**Transport**: Zellij pipe message (`pipe_message_to_plugin`)

## Message Format

**Pipe name**: `cc-deck:focus-report`

**Payload** (JSON):

```json
{
  "client_id": 2,
  "pane_id": 42,
  "session_name": "cc-deck"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| client_id | u16 | yes | Sending sidebar's client ID (from `get_plugin_ids().client_id`) |
| pane_id | u32 | yes | Terminal pane ID that was focused |
| session_name | String | yes | Display name of the session (as shown in sidebar) |

## Sender Behavior

The sidebar sends this message **after** completing local focus operations:
1. Set `local_focus_override`
2. Call `focus_terminal_pane()`
3. Call `switch_tab_to()` (if cross-tab)
4. Update `local_activation_order`
5. **Then** send `cc-deck:focus-report`

The message is sent on every explicit focus action (click or keyboard Enter). Cursor movement (j/k in navigate mode) does NOT send a focus-report.

## Receiver Behavior

The controller:
1. Deserializes the JSON payload into a `FocusReport` struct
2. Validates `client_id` is in the current `connected_clients` set (ignores zombie reports)
3. Updates `client_focus[client_id]` with `{ pane_id, session_name }`
4. Updates session activity tracking (same as existing Switch action handling: last-active timestamp, attendance state)
5. Triggers a render broadcast to update presence indicators on all sidebars

## Backward Compatibility

This is a new message type. Older controllers that don't recognize `cc-deck:focus-report` will ignore it (the pipe handler's `parse_pipe_message` returns `Unknown` for unrecognized names). No breaking changes.

## Error Cases

| Condition | Behavior |
|-----------|----------|
| Malformed JSON payload | Controller logs warning, ignores message |
| Unknown `client_id` (zombie) | Controller ignores message |
| Missing `session_name` | Controller logs warning, ignores message |
| Controller is not leader | Dormant controller ignores all pipe messages (existing behavior) |
