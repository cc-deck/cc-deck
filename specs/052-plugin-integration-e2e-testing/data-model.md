# Data Model: Plugin Integration and E2E Testing

This feature does not introduce new persistent data entities. It operates on existing in-memory structures.

## Existing Entities Used in Tests

### SidebarState
- **cached_payload**: Optional RenderPayload from controller
- **mode**: SidebarMode enum (Passive, Navigate, Rename, etc.)
- **permissions_granted**: Boolean flag for Zellij permission status
- **tab_index**: Optional usize from SidebarInit assignment
- **filter_text**: String for local session filtering

### ControllerState
- **sessions**: BTreeMap<u32, Session> mapping pane_id to session state
- **sidebar_registry**: HashMap<u32, usize> mapping plugin_id to tab_index
- **permissions_granted**: Boolean flag for Zellij permission status
- **pending_events**: Vec<Event> for deferred event replay

### Protocol Messages (serialized as JSON through pipe)
- **RenderPayload**: Controller to sidebar, contains session list and metadata
- **ActionMessage**: Sidebar to controller, contains action type and target
- **SidebarHello**: Sidebar to controller, discovery handshake
- **SidebarInit**: Controller to sidebar, tab assignment response
- **HookPayload**: CLI to controller, Claude Code lifecycle events
