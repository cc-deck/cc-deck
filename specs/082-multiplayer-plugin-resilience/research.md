# Research: Multiplayer Plugin Resilience

## Zellij Plugin ID System

**Decision**: Use `client_id: u16` from `get_plugin_ids()` as the client discriminator.

**Rationale**: Empirical testing (2026-07-21) confirmed:
- Each client connection gets a distinct `client_id` (monotonically increasing, never reused)
- `plugin_id` is shared across clients for the same WASM binary
- Zombie instances keep their original `client_id` (not 0)
- The `PluginIds` struct uses `client_id: u16` (not u32)

**Alternatives considered**:
- `plugin_id` alone: insufficient, shared across clients
- PaneManifest cross-reference: doesn't distinguish clients
- Zellij client API: no such API exists for plugin-to-plugin communication

## Sidebar Registry Data Structure

**Decision**: Change `HashMap<u32, usize>` to `HashMap<u32, SidebarEntry>` where `SidebarEntry` is `(tab_index: usize, client_id: u16)`.

**Rationale**: The registry must support:
1. Lookup by `plugin_id` (for targeted render)
2. Dedup by `(tab_index, client_id)` (on sidebar-hello)
3. Filter by `client_id` (during broadcast)

A tuple is simpler than a struct for internal use. The dedup check requires iterating to find existing entries with the same `(tab, client_id)`, which is O(N) but N is bounded (typically < 20 sidebars even with zombies).

**Alternatives considered**:
- Nested map `HashMap<u16, HashMap<usize, u32>>` (client_id -> tab -> plugin_id): more complex, harder to iterate for broadcast
- Vec of structs: loses O(1) plugin_id lookup

## Election Protocol Format

**Decision**: Change ping payload from `"{plugin_id}"` to `"{client_id}:{plugin_id}"`.

**Rationale**: The `client_id` must be the primary sort key so the primary terminal client (lowest client_id) always wins. Backward compatibility: if parsing `client_id:plugin_id` fails, fall back to `plugin_id`-only comparison.

**Alternatives considered**:
- Separate `client_id` field in a JSON payload: heavier than the current string format
- Priority = `client_id * 100000 + plugin_id`: fragile with large plugin_ids

## broadcast_render_all Fallback

**Decision**: Guard the untargeted `broadcast_render_all()` call with a check: only call it when the registry is empty (no sidebars discovered yet). Once sidebars are registered, skip the untargeted broadcast.

**Rationale**: The untargeted broadcast (line 188 in `render_broadcast.rs`) sends to ALL plugin instances regardless of client_id. This bypasses the client_id filter and defeats the purpose of the multiplayer fix. However, removing it entirely breaks initial startup (before any sidebar has registered). The guard ensures the fallback only fires during the brief window before the first sidebar-hello.

**Alternatives considered**:
- Remove `broadcast_render_all` entirely: breaks initial render before registration
- Always send untargeted: defeats client_id filtering
