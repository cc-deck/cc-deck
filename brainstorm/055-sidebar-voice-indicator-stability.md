# Brainstorm: Sidebar Voice Indicator Stability

## Problem

The green voice indicator (note symbol) in the sidebar header flickers or disappears when switching sessions or tabs. The voice relay TUI correctly updates session names on switch (confirming the controller's voice state is correct), but the sidebar's visual rendering of that state is unreliable.

Observed symptoms:
- **Alt+a (attend):** Green note disappears, sometimes permanently until an unrelated event triggers a new render
- **Session click/switch:** Note disappears briefly then reappears (flicker)
- **Idle observation:** Note stays stable (no issue at rest)

## What We Know

### Controller side is correct

The controller maintains `voice_enabled` and `voice_muted` correctly. Every `broadcast_render` call builds a `RenderPayload` with `voice_connected: state.voice_enabled`. Since `voice_enabled` only goes false on explicit `voice:off` or 15-second heartbeat timeout, all broadcasts during normal voice relay operation include `voice_connected: true`.

### The broadcast timing pattern

Action handlers (attend, switch, working, navigate) follow this pattern:

```rust
broadcast_render(state);      // send to all registered sidebars
state.render_dirty = false;   // clear dirty flag
switch_tab_to_wasm(tab_idx);  // async: queues tab switch in Zellij
focus_terminal_pane_wasm(pid); // async: queues pane focus in Zellij
```

The broadcast is sent BEFORE the tab switch. The comment in `handle_switch` explains: "pipe messages are queued in Zellij ahead of the switch_tab_to command."

### Why no follow-up broadcast arrives

After the tab switch, Zellij fires TabUpdate and PaneUpdate events. But:

1. **TabUpdate:** The action handler pre-sets `state.active_tab_index`, so `active_tab_changed = false`. No dirty mark.
2. **PaneUpdate:** The `in_flight_focus` mechanism (3-second TTL) preserves the action-set `focused_pane_id`, so `focus_changed = false`. No immediate broadcast.

Result: the ONLY broadcast is the pre-switch one. If it does not reach the target sidebar before Zellij renders it, there is no recovery mechanism.

### What we tried and why it failed

Added `state.mark_render_dirty()` after the pre-switch broadcast to schedule a follow-up via the 1-second timer. This adds one extra broadcast per action. It did not reliably fix the flickering, likely because:
- The timer interval (1 second) is too long for imperceptible recovery
- Zellij may deliver the follow-up broadcast at an equally bad time
- The root cause is architectural: the sidebar depends entirely on push-based pipe messages for its display state

## Root Cause

The sidebar is a **passive receiver**. It renders from `cached_payload` which is updated only when a `cc-deck:render` pipe message arrives. Between pipe message deliveries, the sidebar has no way to detect or recover from stale state.

Zellij's pipe message delivery is asynchronous relative to its rendering cycle. When a tab switches:
1. Zellij switches the active tab
2. Zellij calls `render()` on the new tab's sidebar
3. At some later point, queued pipe messages are delivered to the sidebar

Between step 2 and step 3, the sidebar renders with whatever `cached_payload` it has. If the last payload this specific sidebar received included `voice_connected: true` (from a previous broadcast), the note shows. But if the delivery timing means a broadcast was "in flight" and not yet processed, the sidebar shows stale data.

The non-deterministic nature (sometimes works, sometimes not) confirms this is a race condition in Zellij's event loop, not a logic error in cc-deck.

## Approaches to Explore

### Approach A: Sidebar-side voice state (independent of render payload)

Instead of deriving voice indicator visibility solely from `cached_payload.voice_connected`, the sidebar could maintain its own `voice_connected` flag that is updated from MULTIPLE sources:

1. **Render payload** (existing): `voice_connected` field in RenderPayload
2. **Dedicated voice pipe message** (new): A lightweight `cc-deck:voice-state` message with just `{connected: bool, muted: bool}` that the controller sends directly to sidebars when voice state changes

The sidebar would show the note if EITHER source says voice is connected. The dedicated message is smaller and might be delivered faster than the full render payload.

**Trade-off:** Adds a second state channel that must stay consistent with the render payload. But the consequence of inconsistency is benign (momentary stale indicator, self-correcting on next render).

### Approach B: Sidebar requests render on tab activation

When the sidebar's `render()` is called, it could detect that it might be rendering on a newly active tab and proactively request a fresh render from the controller.

Detection options:
- Track the last time `render()` was called. If there is a gap (indicating the tab was inactive), request a refresh.
- Compare `cached_payload.active_tab_index` with `my_tab_index`. If they differ, the payload is from a different tab's perspective (stale).

The sidebar already has a `render-request` mechanism (used for initial load fallback after 3 timer ticks). This approach would reuse it for tab-switch recovery.

**Trade-off:** Adds a round-trip (sidebar request, controller response) which introduces its own latency. The note would still be absent for the duration of that round-trip. But the recovery would be reliable rather than depending on broadcast timing.

### Approach C: Controller broadcasts after tab switch settles

Instead of broadcasting only before the tab switch, the controller could schedule a deferred broadcast after the tab switch events (TabUpdate, PaneUpdate) have been processed.

Options:
- Use a short one-shot timer (e.g., 100ms) to trigger a follow-up broadcast
- Detect the "end of tab switch" by counting expected events (TabUpdate + PaneUpdate) and broadcasting after both arrive

**Trade-off:** Zellij's timer resolution might not support sub-second intervals reliably. And the "count events" approach is fragile (depends on knowing exactly which events Zellij fires for a tab switch).

### Approach D: Sidebar persists voice state in WASI cache

The sidebar could write voice state to the WASI `/cache/` directory and read it on every `render()` call, bypassing the pipe message delivery entirely.

The controller would write `voice_connected` and `voice_muted` to a shared cache file whenever voice state changes. All sidebar instances would read this file on render.

**Trade-off:** File I/O on every render is expensive relative to an in-memory flag. Cache file atomicity across multiple readers and one writer needs careful handling. Also, WASI filesystem access during `render()` may have its own performance implications.

### Approach E: Hybrid push + pull with generation counter

Add a `render_generation: u64` counter to the controller state, incremented on every broadcast. Include it in the render payload. On `render()`, the sidebar compares its cached generation with the controller's current generation (available via a lightweight pipe query or shared cache file). If stale, request a fresh render.

**Trade-off:** Combines the complexity of Approaches B and D. But provides a reliable staleness detection mechanism that handles all edge cases.

## Recommendation

**Start with Approach B** (sidebar requests render on tab activation). Reasons:

1. It reuses the existing `render-request` mechanism, so minimal new code
2. The detection logic is simple: check if `cached_payload.active_tab_index != my_tab_index` or if the sidebar has not received a render in the last N milliseconds
3. The round-trip latency is acceptable (one Zellij pipe round-trip, typically sub-100ms)
4. It is self-healing: even if the first request fails, the sidebar can retry on the next `render()` call
5. No changes needed on the controller side

If Approach B's round-trip latency is still perceptible, layer Approach A on top (dedicated lightweight voice-state message) to give the sidebar a fast path for voice state updates that does not depend on the full render payload delivery.

## Context for Implementation

Key files:
- `cc-zellij-plugin/src/sidebar_plugin/mod.rs` - Sidebar plugin entry point, pipe handler, render dispatch
- `cc-zellij-plugin/src/sidebar_plugin/state.rs` - `SidebarState` with `cached_payload`, `my_tab_index`, etc.
- `cc-zellij-plugin/src/sidebar_plugin/render.rs` - `render_sidebar()` and `render_header()` (voice indicator drawing)
- `cc-zellij-plugin/src/controller/render_broadcast.rs` - `broadcast_render()`, `targeted_render()`
- `cc-zellij-plugin/src/controller/actions.rs` - Action handlers with pre-switch broadcast pattern
- `cc-zellij-plugin/src/controller/events.rs` - TabUpdate/PaneUpdate handlers, sidebar discovery

The sidebar already has `render_request_sent` and `ticks_since_init` for its initial load fallback. The tab-activation detection would be a similar pattern.

The existing `RenderRequest` pipe action in the controller (mod.rs) calls `targeted_render()`, so the response path already works.

## Open Questions

1. Does Zellij call `render()` on a sidebar when its tab becomes active, or only when the plugin returns `true` from `update()`/`pipe()`? If `render()` is not called on tab activation, Approach B needs a different trigger (e.g., Timer event).
2. What is the actual round-trip latency for a `render-request` → `targeted_render` cycle in practice? If it is consistently under 50ms, Approach B alone is sufficient.
3. Could the sidebar subscribe to TabUpdate events to detect tab switches directly? This would provide a reliable trigger for requesting a fresh render. The sidebar currently subscribes to Mouse, Key, Timer, and PermissionRequestResult only.
4. Is there a Zellij API for "plugin became visible"? That would be the ideal trigger.
