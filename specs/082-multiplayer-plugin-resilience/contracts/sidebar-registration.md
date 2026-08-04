# Contract: Sidebar Registration Protocol

## Participants

- **Sidebar instance**: sends `sidebar-hello` on pipe `cc-deck:sidebar-hello`
- **Controller (leader)**: receives, registers, responds with `sidebar-init`

## Message Flow

1. Sidebar sends `SidebarHello { plugin_id, client_id }` to controller
2. Controller looks up `plugin_id` in PaneManifest to find `tab_index`
3. Controller checks for existing entry with same `(tab_index, client_id)`
   - If found: replaces old entry (dedup)
   - If not found: inserts new entry
4. Controller responds with `SidebarInit { tab_index, controller_plugin_id }`
5. Controller sends targeted render payload to the new sidebar

## Behavioral Requirements

- B1: `SidebarHello` without `client_id` field deserializes with `client_id = 0`
- B2: Duplicate `(tab_index, client_id)` replaces existing entry (removes old plugin_id)
- B3: Different `client_id` values on same `tab_index` coexist (multi-client)
- B4: `plugin_id` not found in PaneManifest is silently ignored (logged)
- B5: Registration from a non-leader controller instance is ignored (dormant)

## Render Broadcast Filtering

- B6: `broadcast_render` iterates only entries where `client_id == state.client_id`
- B7: `broadcast_render_all` untargeted fallback only fires when registry is empty
- B8: `targeted_render` (individual sidebar) does not filter by client_id (used for init)
