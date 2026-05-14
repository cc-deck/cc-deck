# Brainstorm: Render Pipeline Stability and CPU Optimization

**Date:** 2026-05-14
**Status:** proposed

## Problem Framing

The sidebar rendering pipeline is unstable: sessions flicker between visible and invisible states, activity indicators blink between wrong states, and Zellij CPU usage reaches 600%+ with 14 tabs open. Multiple fix attempts made the situation worse by introducing new failure modes. A systematic approach is needed.

### Symptoms observed

1. **"No Claude sessions" flicker**: The sidebar alternates between showing all sessions and showing the empty state every few seconds
2. **Activity indicator blinking**: A session oscillates between Done (checkmark) and Idle (circle), or between Working (filled circle) and Init (empty circle)
3. **Session count oscillation**: Debug logs show session count jumping (e.g., 8 to 14 and back) within the same second
4. **High CPU**: Zellij uses 600%+ CPU with 14 idle tabs (should be under 30%)
5. **Green note blinking**: Voice mute indicator flickers between states

### Root causes identified

**Root cause 1: Dual controller instances**

The most critical finding. Debug logs reveal TWO controller plugin instances (plugin_id 0 and plugin_id 4) running simultaneously:

```
CTRL PIPE name=cc-deck:sidebar-hello payload={"plugin_id":21} sessions=0    # controller A
CTRL PIPE name=cc-deck:sidebar-hello payload={"plugin_id":21} sessions=8    # controller B
SIDEBAR INIT tab_index=6 controller=0     # from controller A
SIDEBAR INIT tab_index=6 controller=4     # from controller B
```

Both controllers:
- Receive and process hook events (creating sessions independently)
- Build and broadcast render payloads (with different session lists)
- Register sidebar instances
- Run timer handlers

Sidebars receive payloads from both controllers and show whichever arrived last, causing the alternating display.

**How the second controller appears:** The layout loads sidebar instances from `file:/.../cc_deck.wasm` with `mode "sidebar"`. The `load_plugins` block in `config.kdl` loads one controller from the same WASM URL with `mode "controller"`. Zellij creates separate WASI sandbox instances for each. The controller that shows `plugin_id=0` never had its permissions granted (plugin_id is set to 0 by default, only updated on PermissionRequestResult). This suggests a phantom instance or a sidebar that incorrectly processes controller-only messages.

**Root cause 2: broadcast_render_all (untargeted broadcast)**

The `broadcast_render` function sends the render payload twice: once to each registered sidebar (targeted), and once to ALL plugin instances (untargeted). The untargeted broadcast:
- Delivers the payload to the controller itself (which ignores it)
- Delivers to sidebar instances that may not be registered yet
- When a second controller exists, its untargeted broadcasts reach sidebars with different (often empty) session data

Removing broadcast_render_all fixes the dual-delivery issue but introduces a bootstrapping gap: sidebars that load before the manifest arrives never receive their first payload.

**Root cause 3: Per-second fade re-renders**

The timer handler marks render dirty every tick when ANY session is Done/AgentDone/Idle:

```rust
if state.sessions.values().any(|s| {
    matches!(s.activity, Activity::Done | Activity::AgentDone | Activity::Idle)
}) {
    state.mark_render_dirty();
}
```

With 14 sessions in Idle/Done state, this triggers a full JSON serialize + broadcast to 14 sidebars every second. The faded_color computation changes every second, making every payload different.

**Root cause 4: voice:on heartbeat every second**

The voice relay sends `[[voice:on]]` every second as a heartbeat. The controller's handler calls `mark_render_dirty()` on every `voice:on`, even when already connected. This triggers an additional per-second broadcast.

**Root cause 5: Unconditional mark_render_dirty in handle_tab_update**

Every TabUpdate event (which Zellij sends frequently) calls `state.mark_render_dirty()` unconditionally, even when nothing changed.

## Current Architecture

See `specs/030-single-instance-arch/spec.md` for the full single-instance architecture specification.

### Controller-Sidebar communication

```
                    ┌─────────────┐
                    │  Controller │  (load_plugins, headless, plugin_id=4)
                    │  plugin_id  │
                    │     = 4     │
                    └──────┬──────┘
                           │
              ┌────────────┼────────────┐
              │            │            │
         cc-deck:render  cc-deck:render  cc-deck:render
              │            │            │
        ┌─────▼─────┐ ┌───▼───┐  ┌────▼────┐
        │ Sidebar 0  │ │ Sb 1  │  │  Sb N   │
        │ (tab 0)    │ │(tab 1)│  │ (tab N) │
        └────────────┘ └───────┘  └─────────┘
```

**Controller** (one instance, loaded via `load_plugins`):
- Subscribes to TabUpdate, PaneUpdate, Timer, PermissionRequestResult, RunCommandResult, CommandPaneOpened, PaneClosed
- Owns the authoritative session state (BTreeMap<u32, Session>)
- Processes hook events from CLI (cc-deck:hook pipes)
- Builds RenderPayload and broadcasts to registered sidebars
- Timer fires every 1 second for cleanup, fade rendering, git polling

**Sidebars** (one per tab, loaded via layout template):
- Subscribe to Mouse, Key, PermissionRequestResult
- Receive RenderPayload from controller via cc-deck:render pipe
- Handle local interaction (navigate, rename, filter, click)
- Send ActionMessage to controller via cc-deck:action pipe
- Stateless except for cached_payload and interaction mode

**Discovery protocol:**
1. Sidebar sends SidebarHello to controller (with its plugin_id)
2. Controller cross-references with PaneManifest to find sidebar's tab
3. Controller responds with SidebarInit (tab_index, controller_plugin_id)
4. Controller also auto-discovers sidebars from PaneManifest

### Pipe message routing

Zellij's `pipe_message_to_plugin`:
- With `destination_plugin_id`: delivered to that specific instance
- Without `destination_plugin_id`: broadcast to ALL loaded plugin instances
- CLI `zellij pipe`: broadcast to all instances of the matching plugin URL

This means every hook event, voice command, and dump-state request is delivered to EVERY plugin instance (controller + all sidebars). Each instance's `pipe()` handler filters relevant messages.

### Render broadcast flow

1. State changes mark `render_dirty = true`
2. Timer tick calls `flush_render()` which broadcasts if dirty
3. Some actions broadcast immediately for responsiveness (focus changes)
4. `broadcast_render` sends targeted payloads to each registered sidebar
5. `broadcast_render_all` sends untargeted payload to all plugins (the problematic part)

## Failed Fix Attempts

### Attempt 1: Stop per-second fade re-renders
Changed fade check to only re-render while fade is in progress (elapsed < duration). Correct logic, but insufficient alone because TabUpdate and voice:on still trigger per-second broadcasts.

### Attempt 2: Hash-based render dedup
Added a DJB2 hash of the serialized JSON payload. Skip broadcast if hash matches last broadcast. Failed because the faded_color changes every second (different payload each time), defeating the hash.

### Attempt 3: Move fading to sidebar
Changed RenderSession from pre-computed `color: (u8, u8, u8)` to `last_event_ts: u64`. Sidebar computes faded_color locally. Added Timer subscription to sidebar for local fade updates. This broke the rendering because:
- The protocol change invalidated all existing payload handling
- Sidebar Timer events caused unexpected re-renders
- The sidebar didn't have access to config values (done_timeout, idle_fade_secs)

### Attempt 4: Remove broadcast_render_all
Removed the untargeted broadcast to prevent dual-controller conflicts. This stopped some sidebars from ever receiving payloads because they weren't registered yet when the first render happened.

### Attempt 5: Conditional mark_render_dirty in handle_tab_update
Changed from unconditional `mark_render_dirty()` to only marking dirty on actual changes (focus, tab count, stale sessions). This broke initial sidebar population because the unconditional dirty was the only path that ensured sidebars received the first payload.

## What We Know Works

1. **Targeted render delivery** (`send_render_to_plugin`): Correct, reliable, no issues
2. **Sidebar registry via manifest**: `discover_sidebars_from_manifest` reliably registers sidebars
3. **Hook event processing**: Correct session state transitions
4. **Voice mute protocol extensions**: voice:on:muted, mute_requested timeout, local_mute_override clearing all work correctly
5. **Tick-count-based fade throttling**: `tick_count.is_multiple_of(5)` reduces fade broadcasts from 1/sec to 1/5sec without breaking anything

## What Needs to Be Fixed

### P0: Eliminate dual controller

The most critical fix. Options:
1. **Guard in pipe handler**: Controller ignores all messages if `plugin_id == 0` (permissions not granted). Sidebars never process hook/controller messages anyway.
2. **Startup validation**: Controller checks if another controller is already running (via dump-state probe) and self-disables if one responds.
3. **Zellij-level**: Use a different WASM URL or plugin configuration to prevent Zellij from creating ambiguous instances.

### P1: Remove broadcast_render_all safely

The untargeted broadcast must go because it delivers payloads from the wrong controller. But sidebars need to receive their first payload. Solution: ensure sidebars are registered before the first render, or add a "request render" message that a sidebar sends on initialization.

### P2: Throttle render broadcasts

Multiple independent sources trigger renders every second. Combine fixes:
- Fade throttle: every 5 ticks (already implemented)
- voice:on heartbeat: only mark dirty on initial connection (already implemented)
- TabUpdate: only mark dirty on actual changes
- Use render coalescing properly (mark dirty, flush on timer tick)

## Debugging and Measurement

### Enable debug logging

The WASI cache directory is shared between all instances of the same WASM URL:
```
/Users/rhuss/Library/Caches/org.Zellij-Contributors.Zellij/file:/Users/rhuss/.config/zellij/plugins/cc_deck.wasm/plugin_cache/
```

Touch `debug_enabled` in this directory BEFORE starting Zellij:
```bash
CACHE="$HOME/Library/Caches/org.Zellij-Contributors.Zellij/file:$HOME/.config/zellij/plugins/cc_deck.wasm/plugin_cache"
touch "$CACHE/debug_enabled"
```

Note: `debug_init()` runs once at plugin load. If the file is created after Zellij starts, only new plugin instances pick it up.

To read the log:
```bash
tail -f "$CACHE/debug.log"
```

### Enable perf logging

Add `perf "true"` to the controller's `load_plugins` block in `config.kdl`. Perf CSV is written to `$CACHE/perf.csv` every 30 seconds.

### Key diagnostic signals

1. **Session count in log**: `sessions=N` in every CTRL PIPE line. If N oscillates, there are multiple controllers or session state is being cleared.
2. **SIDEBAR INIT controller=N**: If N differs between entries, multiple controllers exist.
3. **CTRL TIMER: auto-restored**: If sessions are auto-restored when they shouldn't be empty, a controller is losing its state.
4. **SIDEBAR PAYLOAD frequency**: Count payload deliveries per second. Should be 0 when idle (no state changes), 1 on state change, not continuous.
5. **broadcast_render_all presence**: Any untargeted broadcast can deliver to wrong controllers.

### Measuring CPU impact

```bash
# Monitor Zellij CPU in real-time
top -pid $(pgrep -f 'zellij.*server') -stats pid,cpu,threads

# Count renders per 30 seconds (from perf.csv)
grep 'gauge:sidebars' "$CACHE/perf.csv" | tail -5
```

Target: under 30% CPU with 14 idle tabs. Currently: 100-600%.

## Open Questions

- Why does Zellij create a second controller instance? Is it an artifact of `load_plugins` with the same WASM URL as the layout, or a Zellij bug?
- Should the controller validate it's the only instance on startup? If so, how? (Zellij doesn't expose inter-plugin communication beyond pipes.)
- Is `broadcast_render_all` actually needed for ANY edge case? Can we guarantee that `discover_sidebars_from_manifest` + targeted delivery covers all scenarios?
- Should the controller use a different WASM binary or configuration mechanism to guarantee single-instance behavior?
- Would a simpler render model (sidebar polls controller for state on a timer instead of push) be more robust?
