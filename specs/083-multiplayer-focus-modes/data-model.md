# Data Model: Multiplayer Focus Modes

**Date**: 2026-07-22
**Feature**: 083-multiplayer-focus-modes

## Entities

### FocusReport (new pipe message)

Sent from sidebar to controller when a client switches session focus locally.

| Field | Type | Description |
|-------|------|-------------|
| client_id | u16 | The client that switched focus (from `get_plugin_ids().client_id`) |
| pane_id | u32 | The terminal pane that was focused |
| session_name | String | Display name of the focused session (for presence indicator matching) |

**Identity**: One active report per `client_id` (latest wins).

### ClientFocusEntry (new controller state)

Stored in `ControllerState.client_focus: HashMap<u16, ClientFocusEntry>`.

| Field | Type | Description |
|-------|------|-------------|
| pane_id | u32 | The terminal pane this client is focused on |
| session_name | String | Display name for rendering presence indicators |

**Lifecycle**: Created on first focus-report from a client. Removed when `SessionUpdate` shows the client has disconnected (reduced `connected_clients` count).

### OtherClientPresence (new render payload field)

Added to `RenderPayload.other_client_focus`. Maps session display names to the list of other clients focused on that session.

| Field | Type | Description |
|-------|------|-------------|
| session_name | String | Display name of the session |
| clients | Vec<ClientIndicator> | List of other clients focused on this session |

### ClientIndicator (nested in OtherClientPresence)

| Field | Type | Description |
|-------|------|-------------|
| client_id | u16 | The client ID |
| color | (u8, u8, u8) | RGB color from `multiplayer_user_colors` palette |

### LocalActivationOrder (new sidebar state)

Stored in `SidebarState.local_activation_order: Vec<u32>`.

| Field | Type | Description |
|-------|------|-------------|
| (vec element) | u32 | Pane ID, ordered most-recently-focused first |

**Lifecycle**: Transient per sidebar instance. Initialized empty. Updated on each local focus switch (clicked/keyboard-selected pane moves to front). Lost when sidebar is recreated (client detach/reattach), which is acceptable (resets to global order).

### MultiplayerColors (new controller state)

Stored in `ControllerState.multiplayer_colors: Option<Vec<(u8, u8, u8)>>`.

| Field | Type | Description |
|-------|------|-------------|
| (vec element) | (u8, u8, u8) | RGB value for player N (index 0 = player_1, up to index 9 = player_10) |

**Lifecycle**: Set on first `ModeUpdate` event. Updated on subsequent `ModeUpdate` events (theme change). `None` until first event arrives. When `None`, presence indicators are not rendered.

## Modified Entities

### ControllerState (existing, modified)

New fields added:

| Field | Type | Default |
|-------|------|---------|
| client_focus | HashMap<u16, ClientFocusEntry> | empty |
| multiplayer_colors | Option<Vec<(u8, u8, u8)>> | None |

### RenderPayload (existing, modified)

New field added:

| Field | Type | Default |
|-------|------|---------|
| other_client_focus | Vec<OtherClientPresence> | empty vec (serde default) |

The `other_client_focus` field is filtered per render target: it excludes the target sidebar's own client_id. Single-client sessions produce an empty vec.

### SidebarState (existing, modified)

New field added:

| Field | Type | Default |
|-------|------|---------|
| local_activation_order | Vec<u32> | empty vec |

## State Transitions

### Focus Switch (sidebar click or Enter in navigate mode)

```
Sidebar:
  1. Set local_focus_override = pane_id
  2. Call focus_terminal_pane(pane_id) directly (per-client API)
  3. Call switch_tab_to(tab_index) if cross-tab (per-client API)
  4. Move pane_id to front of local_activation_order
  5. Send cc-deck:focus-report { client_id, pane_id, session_name } to controller

Controller (on receiving focus-report):
  6. Update client_focus[client_id] = { pane_id, session_name }
  7. Update session activity tracking (last-active timestamp)
  8. Trigger render broadcast (includes updated other_client_focus)
```

### Client Connect

```
1. SessionUpdate event shows increased connected_clients count
2. New sidebar instances send sidebar-hello with client_id
3. No focus-report yet (no entry in client_focus for new client)
4. Presence indicators for new client appear after first focus-report
```

### Client Disconnect

```
1. SessionUpdate event shows decreased connected_clients count
2. Controller detects reduced count, removes stale client_id from client_focus
3. Next render broadcast omits the disconnected client from other_client_focus
4. Presence indicators for disconnected client disappear
```

### ModeUpdate Event

```
1. Controller receives ModeUpdate(ModeInfo)
2. Extract multiplayer_user_colors from ModeInfo.style.colors
3. Convert PaletteColor variants to (u8, u8, u8) RGB tuples
4. Store in multiplayer_colors
5. Next render broadcast includes color data in other_client_focus
```
