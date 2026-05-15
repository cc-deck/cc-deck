# Research: Controller Leader Election

**Date**: 2026-05-14
**Feature**: 055-controller-leader-election

## Root Cause Analysis

### Decision: Dual controller is caused by Zellij load_plugins + AddClient race
- **Rationale**: Zellij loads background plugins before entering its event loop, then `AddClient` creates a second WASM instance because `connected_clients` is still empty. Both instances share the same `plugin_id` internally but have separate WASM stores.
- **Evidence**: Debug logs show `CTRL[0]` and `CTRL[4]` both processing every pipe message. Both register keybindings. Render broadcasts go to 40 sidebars instead of ~12.
- **Alternatives considered**: Hook CLI targeting mismatch (fixed in 8ded8d1 but did not eliminate the duplicate instance)

## Protocol Design

### Decision: Ping-pong leader election with pessimistic startup
- **Rationale**: Existing `ControllerPing`/`ControllerPong` pipe actions are already defined in `pipe_handler.rs`. Pessimistic startup (dormant by default) eliminates the dual-active window that causes navigation chaos.
- **Protocol flow**:
  1. Controller starts dormant (`is_leader = false`)
  2. On `PermissionRequestResult(Granted)`, broadcast `cc-deck:controller-ping` with own `plugin_id` as payload
  3. Start timer (existing 1s tick). Count ticks.
  4. If ping received from lower-ID instance: stay dormant, record `leader_plugin_id` and `last_leader_ping_ms`
  5. If ping received from higher-ID instance: respond with own ping (lower ID wins), the higher-ID instance goes dormant
  6. If 2 timer ticks (2 seconds) pass with no ping from a lower-ID: activate as leader
  7. Leader re-pings every 30 ticks (30 seconds) as heartbeat
  8. Dormant re-activates if 60 ticks (60 seconds) pass without a leader ping
- **Alternatives considered**: File-based lock (adds I/O per message), deferred activation (2s startup delay even without duplicates, but we chose pessimistic which has same delay)

### Decision: Restore broadcast_render_all as untargeted fallback
- **Rationale**: Removed in 8ff10c0 to prevent dual-controller interference. With leader election ensuring only one active controller, the safety net is needed again for sidebars not yet in the registry.
- **Alternatives considered**: TabUpdate subscription for sidebar self-healing (too complex, adds N pipe messages per action with 10-15 tabs)

## Key Files

| File | Role | Changes needed |
|------|------|----------------|
| `cc-zellij-plugin/src/controller/state.rs` | Controller state | Add `is_leader`, `leader_plugin_id`, `last_leader_ping_ms`, `election_ticks` fields |
| `cc-zellij-plugin/src/controller/mod.rs` | Main controller logic | Add dormant guard in `pipe()` and `update()`, handle ping/pong, defer keybinding registration |
| `cc-zellij-plugin/src/controller/events.rs` | Event handlers | Add dormant guard in `handle_timer`, leader heartbeat ping, re-activation check |
| `cc-zellij-plugin/src/controller/render_broadcast.rs` | Render broadcasting | Restore `broadcast_render_all` function and call it from `broadcast_render` |
| `cc-zellij-plugin/src/pipe_handler.rs` | Pipe message parsing | No changes needed (ControllerPing/Pong already defined) |

## Risks

- **2-second startup delay**: The pessimistic default means keybindings and renders are inactive for 2 seconds on every fresh Zellij start (even without duplicates). This is acceptable because Zellij itself takes several seconds to initialize its UI.
- **60-second failover gap**: If the leader crashes, the dormant instance takes up to 60 seconds to re-activate. During this window, no keybindings or renders are active. This is acceptable because the Zellij bug creates instances simultaneously (not sequentially), so the failover scenario is unlikely in practice.
