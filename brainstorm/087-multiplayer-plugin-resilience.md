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

---

## Revisit: 2026-07-21

### Empirical Testing of `client_id` Behavior

Tested `client_id` stability by attaching/detaching a second terminal to a live Zellij session (`cc-deck-local`). All plugin instances already log `client_id` from `get_plugin_ids()` during permission grant.

**Test protocol:**
1. Start Zellij session (terminal 1)
2. Attach second terminal (`cc-deck attach local`)
3. Detach second terminal
4. Reattach second terminal

**Results:**

| Event | client_id | plugin_id | Notes |
|-------|-----------|-----------|-------|
| Original terminal | 1 | 0 | Primary client |
| Attach 2nd terminal | 2 | 0 | New plugin instances created, same plugin_id |
| Detach 2nd terminal | - | - | Zombie instances persist (Zellij #4064 confirmed) |
| Reattach 2nd terminal | **3** (not 2) | 0 | New ID, zombies from client_id=2 still alive |
| Sidebar count after | 45-46 | - | Should be ~15 (3x amplification from zombies) |

**Key findings:**

1. **`client_id` is distinct and reliable**: each client connection gets a unique ID
2. **`client_id` is monotonically increasing**: Zellij never reuses disconnected client IDs
3. **`client_id=0` is NOT an orphan signal**: zombies keep their original client_id (2, not 0)
4. **`plugin_id` is shared across clients**: the same WASM binary gets plugin_id=0 for all client instances, so plugin_id alone cannot distinguish clients
5. **Zombie instances are fully alive**: they receive pipe messages, respond to events, and participate in sidebar registration. Only killing the session removes them.

### Updated Problem Understanding

The original brainstorm assumed `client_id=0` would mark orphans. Testing disproves this. Instead, orphan detection must be based on the controller tracking which `client_id` values are currently active. The simplest strategy: the controller only broadcasts render payloads to sidebars whose `client_id` matches its own.

### Refined Approach: Client-Aware Plugin Architecture

**Unchanged from original decision** (Approach B), but with updated implementation details based on testing:

**1. Protocol change**: Add `client_id: u32` to `SidebarHello`. Each sidebar reads `get_plugin_ids().client_id` and sends it during registration.

**2. Registry change**: `sidebar_registry` becomes `HashMap<u32, (usize, u32)>` mapping `plugin_id -> (tab_index, client_id)`. On `sidebar-hello`, if a sidebar for the same (tab, client_id) already exists, the new one replaces the old.

**3. Render broadcast filtering**: Only send render payloads to sidebars whose `client_id` matches the controller's own `client_id`. Zombies from disconnected clients silently stop receiving updates.

**4. Controller election**: Include `client_id` in the ping payload. Election priority becomes `(client_id, plugin_id)` tuple (lowest wins). The primary terminal client's controller always wins over web/attach client instances.

### Updated Scope

**In scope:**
- Add `client_id` to `SidebarHello` protocol
- Change sidebar registry to track `(tab_index, client_id)` per plugin_id
- Deduplicate registrations: same (tab, client_id) replaces old entry
- Skip orphaned sidebars (mismatched client_id) during render broadcast
- Include `client_id` in controller election ping, priority `(client_id, plugin_id)` lowest wins
- Graceful degradation: rendering stable even with 40+ zombie sidebars

**Out of scope:**
- Killing zombie plugin instances (Zellij's responsibility, #4064)
- Active periodic cleanup of orphaned registry entries (skip is sufficient)
- Multi-user presence display (brainstorm 085, depends on this fix)
- Session sharing protocol (brainstorms 082-084)

### Resolved Open Questions

- ~~Can `get_plugin_ids()` reliably distinguish clients?~~ **Yes.** Each client gets a distinct, stable, monotonically increasing `client_id`.
- ~~Is `client_id=0` a reliable orphan signal?~~ **No.** Orphans keep their real `client_id`. Use controller's own `client_id` as the filter instead.
- ~~Does `client_id` remain stable across the lifetime of a connection?~~ **Yes.** Same `client_id` observed throughout a client session. Reconnection gets a new ID.

### Remaining Open Questions

- Does the controller election need to handle the case where the lowest-client_id controller is a zombie? (The leader heartbeat timeout should cover this, but needs testing.)
- Should dormant controllers also track sidebar registrations for faster failover?
