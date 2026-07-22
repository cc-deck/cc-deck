# Brainstorm: Multiplayer Plugin Resilience

**Date:** 2026-07-22
**Status:** active

## Problem Framing

When multiple clients connect to a Zellij session (via web sharing or terminal multiplayer), Zellij instantiates the CC Deck plugin (cc_deck.wasm) for each connected client. The current controller/sidebar architecture assumes a small, stable number of plugin instances (1 controller + 1 sidebar per tab). Web clients violate this assumption, causing:

- **44+ sidebar instances** registered instead of the expected ~12 (one per tab)
- Repeated `sidebar-hello` re-registrations (`plugin_id=0` on `tab=0` fires dozens of times)
- Render broadcast storms (`CTRL RENDER broadcast: sidebars=44`)
- Visible sidebar reshuffling and "Loading status-bar..." flickering
- **Orphaned instances persist after client disconnect** ([Zellij Issue #4064](https://github.com/zellij-org/zellij/issues/4064))
- Stopping the web server does NOT clean up orphaned instances
- Session detach+reattach does NOT clean up instances
- Only `zellij kill-session` + recreate clears the zombie instances

This is a prerequisite blocker for session sharing (082-086) and an amplification of the existing dual controller bug (brainstorm 055).

## Evidence from Spike Testing

Debug log excerpt showing the problem:

```
CTRL SIDEBAR registered plugin_id=23 on tab=7
CTRL SIDEBAR registered plugin_id=0 on tab=0    # repeated dozens of times
CTRL SIDEBAR registered plugin_id=0 on tab=0
CTRL SIDEBAR registered plugin_id=0 on tab=0
CTRL RENDER broadcast: sidebars=44 serialization_us=1000
CTRL RENDER broadcast: sidebars=44 serialization_us=2000
```

Each web client connection triggers:
1. Zellij loads cc_deck.wasm for the new client
2. The new instance sends `sidebar-hello` to the controller
3. The controller registers it, incrementing the sidebar count
4. All 44 sidebars receive every render broadcast
5. When the web client disconnects, the plugin instance stays alive (Zellij #4064)

## Approaches Considered

### A: Deduplicate sidebar registrations in the controller

- Pros: Directly addresses the symptom. Controller ignores re-registrations from the same (tab, plugin_id) pair. Existing instances keep working normally.
- Cons: Doesn't reduce the number of active plugin instances (they still consume memory/CPU), just prevents the render storm.

### B: Client-aware instance detection

- Pros: Use `get_plugin_ids()` to get the `client_id`, then have the controller track which client each sidebar belongs to. Only accept one sidebar per (tab, client) pair. Detect orphaned instances (client_id=0) and ignore them.
- Cons: More complex, but addresses both the duplication and the zombie problem.

### C: Single-instance architecture (controller gates plugin loading)

- Pros: Prevent the problem entirely by having the controller refuse additional instances. Only one sidebar per tab, regardless of client count.
- Cons: May conflict with Zellij's multiplayer model where each client gets its own plugin instances. Might not be possible with the current plugin API.

## Decision

**Chosen: B (client-aware instance detection) with A as the immediate fix.**

Phase 1: Deduplicate sidebar registrations by (tab, plugin_id) pair. This is a small code change that immediately stops the render storms.

Phase 2: Add client_id tracking. Each sidebar reports its client_id on hello. The controller tracks active clients and ignores orphaned instances (client_id=0). When a sidebar registers with a client_id that already has a sidebar on that tab, the new registration replaces the old one.

## Key Requirements

1. **Sidebar registration dedup**: Controller must reject or replace duplicate registrations from the same (tab, plugin_id) pair
2. **Client_id tracking**: Sidebars include their `client_id` (from `get_plugin_ids()`) in the `sidebar-hello` payload
3. **Orphan detection**: Controller ignores or cleans up sidebar registrations with `client_id=0`
4. **Render broadcast cap**: Controller limits the sidebar set to one per (tab, primary_client) to prevent N-fold render amplification
5. **Graceful degradation**: If 44 sidebars are registered, rendering should still be stable (no visual glitches even if slow)
6. **No session kill required**: Recovery from multiplayer mode should work via detach+reattach, not only via session destruction

## Open Questions

- Can `get_plugin_ids()` reliably distinguish the "primary" terminal client from web client instances?
- Does the controller election (leader heartbeat) need to account for multiple controller instances from web clients?
- Should the sidebar plugin detect it's running in a web client context and skip initialization entirely (let the web client render the host's sidebar)?
- Is `client_id=0` a reliable signal for orphaned instances, or can it appear in other scenarios?
- Should we file upstream on Zellij for plugin instance cleanup on client disconnect?

## Related

- Brainstorm 055: Dual controller workarounds (same root cause, different trigger)
- Brainstorm 082: Session sharing spike (discovered this bug during manual testing)
- Brainstorm 085: Sidebar presence panel (depends on this fix)
- [Zellij Issue #4064](https://github.com/zellij-org/zellij/issues/4064): Plugin instances persist after client disconnect
