# Review Guide: Controller Leader Election

**Spec:** [spec.md](spec.md) | **Plan:** [plan.md](plan.md) | **Tasks:** [tasks.md](tasks.md)
**Generated:** 2026-05-14

---

## What This Spec Does

Zellij has a bug where `load_plugins` + `AddClient` race creates a duplicate WASM instance of the cc-deck controller. Two controllers independently register keybindings, process hooks, and broadcast renders. This causes navigation mode to cycle through all sessions on a single keypress (both controllers forward the navigate command) and makes the voice indicator flicker (the `broadcast_render_all` safety net was removed to work around the dual controller, leaving sidebars without a fallback).

This spec adds a ping-pong leader election so only one controller is active, then restores the untargeted render broadcast.

**In scope:** Election protocol (ping/pong with lowest-ID-wins), dormant guards on `pipe()` and `update()`, restore `broadcast_render_all`, election debug logging.

**Out of scope:** Fixing the upstream Zellij bug, changing voice relay timeouts, sidebar-side rendering optimizations, stale sidebar registry cleanup.

## Bigger Picture

This is the third attempt at fixing dual-controller symptoms. Spec 053 (render pipeline stability) addressed rendering issues. Spec 054 (voice sync reliability) fixed heartbeat protocol and sidebar init. Both treated symptoms without eliminating the root cause: two active controllers. This spec makes the system resilient to the Zellij bug by ensuring only one controller operates, regardless of how many instances Zellij loads.

The upstream Zellij fix is documented in `brainstorm/zellij-load-plugins-duplicate-instance.md` with a proposed patch (check `(plugin_id, client_id)` exists before creating). If Zellij fixes this, the election protocol becomes a no-op (single instance always wins). The protocol adds no ongoing cost beyond one ping every 30 seconds.

---

## Spec Review Guide (30 minutes)

> Focus your review on the election protocol design and the tradeoff of a 2-second startup delay.

### Understanding the approach (8 min)

Read [Purpose](spec.md#purpose) and [FR-010](spec.md#functional-requirements) for the core approach. As you read, consider:

- Does pessimistic startup (dormant by default, activate after 2s) create a noticeable gap for the user when starting Zellij?
- Is lowest-plugin-ID-wins the right tiebreaker? The Zellij bug creates plugin_id 0 first (the "real" one) and plugin_id 4 second (the duplicate). Lowest-ID-wins means the real instance always becomes leader. Does this hold in all Zellij versions?
- The spec assumes `PipeSource::Plugin(id)` correctly identifies the sender. Is this guaranteed by Zellij's pipe API?

### Key decisions that need your eyes (12 min)

**Pessimistic startup default** ([FR-010](spec.md#functional-requirements), [Clarifications](spec.md#clarifications))

Controllers start dormant and activate only after a 2-second probe window with no lower-ID ping. This eliminates the dual-active window but adds a 2-second delay on every fresh Zellij start (even without duplicates). The alternative (optimistic, default to leader) would have no delay but a brief chaos window.
- Question for reviewer: Is 2 seconds acceptable? Zellij's own UI takes several seconds to initialize, so keybindings being inactive for 2 seconds may be unnoticeable. But if sessions restore instantly, could a user try Alt+s within those 2 seconds?

**Dormant controllers skip all event processing** ([FR-008a](spec.md#functional-requirements), [Clarifications](spec.md#clarifications))

A dormant controller does not process TabUpdate, PaneUpdate, or Timer events (except for election logic). This means if the leader crashes, the dormant controller activates "cold" and must rebuild state from scratch via the next event cycle.
- Question for reviewer: Is cold re-activation acceptable? The failover scenario (leader crash) is unlikely in practice since both instances are created simultaneously by the Zellij bug. But a 60-second gap with no active controller is a long time if it ever happens.

**Restore broadcast_render_all** ([FR-007](spec.md#functional-requirements))

This was removed in commit 8ff10c0 because it caused flickering when two controllers both broadcast. With only one active controller, it is safe to restore. The untargeted broadcast acts as a safety net for sidebars not yet in the registry.
- Question for reviewer: Could restoring the untargeted broadcast cause any issues with the sidebar receiving duplicate renders (one targeted, one untargeted)? The sidebar should handle this idempotently, but worth confirming.

### Areas where I'm less certain (5 min)

- [Edge Cases](spec.md#edge-cases): The spec says "first to broadcast wins" as a timestamp tiebreaker for same-plugin-ID scenarios. But WASI `unix_now_ms()` may not have sub-millisecond precision, and both instances load nearly simultaneously. In practice this scenario should never happen (Zellij assigns unique IDs), but the tiebreaker is untested.

- [FR-006](spec.md#functional-requirements): The 60-second re-activation timeout is a guess. Too short and a slow Zellij could trigger false re-activation. Too long and a genuine leader crash leaves the system without keybindings for a full minute. The spec notes this is unlikely in practice, but the number is not derived from any measurement.

- [Plan: Design Decision 3](plan.md#design-decisions): Keybinding registration is deferred until after election. But `handle_tab_update` is where keybindings were originally registered, triggered by the first TabUpdate event. If the election finishes mid-way through the first timer tick, the next TabUpdate might not arrive for up to a second, adding to the startup delay.

### Risks and open questions (5 min)

- If Zellij's `pipe_message_to_plugin` (untargeted broadcast) delivers messages to ALL plugin types (sidebars + controllers), does the sidebar correctly ignore `cc-deck:controller-ping`? The sidebar's pipe handler has a `_ => false` fallback, so it should. But worth verifying.
- The [sidebar registry shows 40 entries](spec.md#purpose) instead of ~12. Is this caused by the dual controller (both discovering sidebars), or is there a separate registry leak? The leader election fixes the dual-discovery, but if the leak has another cause, SC-005 may not pass.
- What happens if a future Zellij update fixes the duplicate instance bug? The election protocol becomes a no-op (single instance wins after 2s). No harm, but the 2-second delay persists unnecessarily. Should there be a config option to disable election?

---
*Full context in linked [spec](spec.md) and [plan](plan.md).*
