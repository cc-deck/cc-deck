# Dual Controller Workarounds: Performance Impact and Revert Plan

**Date:** 2026-05-15
**Context:** Zellij's `load_plugins` + `AddClient` race creates two WASM instances of the controller plugin. Feature 055 adds a leader election protocol to work around this. Several workarounds have performance and complexity costs that should be reverted when the upstream bug is fixed.

## Performance Impact Assessment

### 1. Broadcast pipe delivery (Go CLI)

**What changed:** `localPipeChannel` and `execPipeChannel` dropped `--plugin` targeting. All `zellij pipe` commands now broadcast to every plugin instance instead of targeting the controller specifically.

**Impact:** Every CLI pipe message (hooks, voice, dump-state, refresh, etc.) is now delivered to ALL plugin instances: 2 controllers + N sidebars. With 12 tabs, that is ~14 instances receiving every message instead of 1.

- Hook events fire on every Claude Code tool use. At peak activity with 3-4 active sessions, that is 10-20 hooks/second. Each hook now hits 14 plugins instead of 1.
- Voice relay sends 2 messages/second (voice:on + dump-state). Now hits 14 plugins each.
- Sidebars ignore unknown pipe names immediately (`_ => false`), so the per-sidebar cost is just the Zellij pipe routing overhead plus WASM function entry/exit. No payload parsing.
- The dormant controller processes and drops the message in the dormant guard (a string comparison and return).

**Estimated overhead:** ~5-10% more CPU in Zellij's pipe routing layer. Not measurable in user-facing latency. The actual WASM processing cost per sidebar is negligible (one match arm, return false).

### 2. Untargeted `broadcast_render_all` (Rust plugin)

**What changed:** `broadcast_render` now sends an untargeted pipe message after the targeted per-sidebar sends. This provides a fallback for sidebars not yet in the registry.

**Impact:** One extra pipe message per render cycle. With ~12 sidebars already receiving targeted renders, this adds a 13th untargeted broadcast. Every sidebar receives the payload twice (once targeted, once broadcast), but the second delivery is a no-op since `cached_payload` is already set with the same data.

**Estimated overhead:** ~8% more render pipe messages. The duplicate payload is deserialized by each sidebar but produces no re-render (same data, no visual change). At steady-state idle (renders every 5 seconds), this is 0.2 extra messages/second.

### 3. Leader election protocol (Rust plugin)

**What changed:** New state fields, dormant guards in `pipe()` and `update()`, election timeout/heartbeat logic in `handle_timer()`.

**Impact at steady state (after election completes):**
- Dormant controller: processes only Timer events. One `is_leader` check per timer tick (1 second). Increments no election counters (leader_plugin_id is Some, so the timeout check short-circuits). Cost: ~2 boolean checks per second.
- Leader controller: one `is_multiple_of(30)` check per timer tick for heartbeat. One heartbeat ping broadcast every 30 seconds (1 pipe message). Two `is_leader` checks per pipe message (one in `update()` dormant guard, one in `pipe()` dormant guard, but only the `pipe()` guard runs since `update()` skips the guard when leader). Cost: negligible.
- Election startup cost: 2-second activation delay on fresh Zellij start. Keybindings and renders are inactive during this window.

**Estimated overhead:** Near zero at steady state. The 30-second heartbeat adds 2 pipe messages/minute.

### 4. 2-second startup delay

**What changed:** Controllers start dormant and wait 2 timer ticks before the leader activates.

**Impact:** Keybindings (Alt+s, Alt+a, Alt+w) and render broadcasts are inactive for ~2 seconds after Zellij starts. Sidebars show "Connecting..." during this window. This overlaps with Zellij's own UI initialization time.

### 5. Dormant controller resource usage

**What changed:** The dormant controller instance stays loaded but processes almost nothing.

**Impact:** ~1 MB WASM memory for the dormant instance (same as any loaded plugin). CPU usage is negligible (Timer event processing only, all other events dropped). The dormant controller does not poll git, flush renders, or process hooks.

## What to Revert When Upstream Fix Lands

When Zellij fixes the duplicate instance bug (ensuring only one controller loads), these changes can be reverted in order of impact:

### High Priority (remove immediately)

1. **Restore `--plugin` targeting in pipe channels** (`cc-deck/internal/ws/channel_pipe.go`)
   - Revert `localPipeChannel.pipeCmd` to include `--plugin` and `--plugin-configuration` flags
   - Revert `execPipeChannel.pipeCmd` similarly
   - Revert `queryPluginCtx` in `cc-deck/internal/session/save.go`
   - This eliminates broadcast overhead for CLI pipes (biggest performance win)

2. **Remove `broadcast_render_all` from `broadcast_render`** (`cc-zellij-plugin/src/controller/render_broadcast.rs`)
   - Remove the `broadcast_render_all(&json)` call at line 101
   - Remove the `broadcast_render_all` function (lines 155-166)
   - Targeted renders via sidebar registry are sufficient when only one controller exists

### Medium Priority (simplify code)

3. **Remove dormant guards** (`cc-zellij-plugin/src/controller/mod.rs`)
   - Remove the `is_leader` check in `pipe()` (lines 157-170)
   - Remove the `is_leader` check in `update()` (lines 134-141)
   - These guards serve no purpose with a single controller

4. **Remove election protocol from `handle_timer`** (`cc-zellij-plugin/src/controller/events.rs`)
   - Remove the entire dormant election block (lines 155-191)
   - Remove the heartbeat ping (lines 195-198)
   - Remove `broadcast_controller_ping` function (lines 518-530)
   - The `is_leader` guard in `handle_tab_update` keybinding registration can also be removed

5. **Remove startup election ping** (`cc-zellij-plugin/src/controller/mod.rs`)
   - Restore the immediate `broadcast_render` after `PermissionRequestResult(Granted)` (was at lines 116-119 before the election changes)
   - Remove `broadcast_controller_ping(self.state.plugin_id)` call
   - This eliminates the 2-second startup delay

### Low Priority (cleanup)

6. **Remove election state fields** (`cc-zellij-plugin/src/controller/state.rs`)
   - Remove `is_leader`, `leader_plugin_id`, `last_leader_ping_ms`, `election_ticks` fields
   - Remove `ELECTION_TIMEOUT_TICKS`, `LEADER_HEARTBEAT_TICKS`, `LEADER_FAILURE_TIMEOUT_MS` constants

7. **Remove election tests** (`cc-zellij-plugin/src/controller/integration_tests.rs`, `state.rs`)
   - Remove all `test_election_*` tests
   - Revert `setup_controller()` in `test_helpers.rs` (remove `is_leader = true` line)

8. **Remove ping/pong handler** (`cc-zellij-plugin/src/controller/mod.rs`)
   - Revert `ControllerPing | ControllerPong` match arm to no-op (or remove entirely)
   - The `ControllerPing`/`ControllerPong` pipe actions in `pipe_handler.rs` can stay for backward compatibility or be removed

9. **Remove README Known Issues section** (`README.md`)
   - Remove the "Duplicate Controller Instances (Zellij Bug)" section

## How to Detect the Upstream Fix

The fix would appear as a Zellij release note mentioning one of:
- "Fixed duplicate plugin instances on load_plugins"
- "Fixed AddClient creating extra WASM instances"
- "Plugin deduplication for background plugins"

To verify: start cc-deck, check the debug log for only one `CTRL LOAD` entry (currently shows two: CTRL[0] and CTRL[4]).

## Total Steady-State Overhead Summary

| Component | Extra messages/sec | Extra CPU | Memory |
|-----------|-------------------|-----------|---------|
| Broadcast pipes (CLI) | ~0 at idle, ~20 at peak | ~5-10% in Zellij routing | 0 |
| broadcast_render_all | ~0.2 at idle | Negligible | 0 |
| Election heartbeat | ~0.03 (2/min) | Negligible | 0 |
| Dormant controller | 0 | ~0 (timer checks only) | ~1 MB WASM |
| **Total** | **~0.2 idle, ~20 peak** | **~5-10% peak** | **~1 MB** |

The overhead is dominated by the broadcast pipe delivery change. At idle (no active Claude Code sessions), overhead is negligible. At peak (multiple sessions with rapid tool use), the extra pipe routing adds measurable but not user-visible CPU usage.
