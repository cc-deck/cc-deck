# 030: Single-Instance Architecture (Background Controller + Thin Renderers)

## Problem

The current architecture creates one WASM plugin instance per tab via `default_tab_template`.
With N tabs, every broadcast event (PaneUpdate, TabUpdate) calls interpreted WASM N times.
Under Zellij 0.44's wasmi interpreter (pure interpretation, no JIT), this creates cumulative latency that makes mouse clicks sluggish and key navigation erratic at 10+ tabs.

The root cause is architectural: expensive work (hook processing, git detection, session state management, sync broadcasting) runs in every instance, even though only the active-tab instance renders.

## Zellij Constraints

Confirmed by examining the Zellij 0.44 source:

- **No global sidebar.** Every pane belongs to exactly one Tab. There is no cross-tab rendering surface.
- **The status-bar is also N instances.** It uses the same tab_template pattern.
- **Background plugins exist.** Headless, session-global, single instance, can send/receive pipe messages. Defined in `config.kdl` via `load_plugins`.
- **Dynamic unsubscribe is not supported.** Once a plugin subscribes to an EventType, it receives every event of that type for its lifetime.

## Proposed Architecture

Split the current single plugin into two WASM binaries:

### cc-deck-controller (background plugin, single instance)

- Loaded via `load_plugins` in config.kdl (session-global, headless, no rendering)
- Subscribes to: PaneUpdate, TabUpdate, Timer, RunCommandResult, PaneClosed, CommandPaneOpened
- Receives all `cc-deck:hook` pipe messages from Claude Code hooks
- Owns the authoritative session state (BTreeMap<u32, Session>)
- Runs git detection, dead session cleanup, stale session timeout
- Handles session metadata sync (session-meta.json)
- Persists state to /cache/sessions.json for reattach recovery
- Broadcasts pre-built render payloads to sidebar renderers via `cc-deck:render` pipe

### cc-deck-sidebar (thin renderer, one per tab via tab_template)

- Loaded via `default_tab_template` in layout.kdl (one per tab)
- Subscribes to: Mouse, Key only (no PaneUpdate, TabUpdate, Timer)
- Receives `cc-deck:render` pipe messages with pre-serialized display data
- Renders ANSI output from the payload (trivial computation)
- Handles mouse clicks and keyboard navigation locally
- Forwards user actions (rename, delete, pause, navigate) to controller via pipe messages
- Does NOT maintain session state, does NOT run git detection, does NOT sync

### Data Flow

```
Claude Code hooks ──> cc-deck:hook ──> Controller
                                           │
Zellij events ─────> PaneUpdate ──────> Controller
                      TabUpdate            │
                      Timer                │
                                           v
                                   Process state changes
                                   Git detection
                                   Dead session cleanup
                                           │
                                           v
                                   Build render payload:
                                   {sessions, focused_pane, active_tab,
                                    notification, sidebar_mode_hint}
                                           │
                                           v
                              cc-deck:render pipe broadcast
                                           │
                      ┌────────────────────┼────────────────────┐
                      v                    v                    v
               Tab 1 sidebar        Tab 2 sidebar        Tab 3 sidebar
               (thin renderer)      (thin renderer)      (thin renderer)
               Deserialize ─>       Deserialize ─>       Deserialize ─>
               Render ANSI          Skip (not active)    Skip (not active)
```

### User Interaction Flow

```
User clicks session in sidebar
     │
     v
Tab N sidebar receives Mouse event
     │
     v
Determines click target (pane_id, action)
     │
     v
Sends cc-deck:action pipe to Controller
{action: "switch", pane_id: 42, tab_idx: 3}
     │
     v
Controller processes action:
  - Updates session state
  - Calls switch_tab_to / focus_terminal_pane
  - Broadcasts updated render payload
```

## Render Payload Design

The controller sends a single JSON payload that the renderer can display without computation:

```json
{
  "sessions": [
    {
      "pane_id": 1,
      "display_name": "api-server",
      "activity": "Working",
      "indicator": "●",
      "color": [180, 140, 255],
      "git_branch": "main",
      "tab_index": 0,
      "paused": false
    }
  ],
  "focused_pane_id": 1,
  "active_tab_index": 0,
  "notification": null,
  "total": 3,
  "waiting": 1,
  "working": 1,
  "idle": 1
}
```

The renderer just iterates this list and prints ANSI escape sequences.
No sorting, filtering, deduplication, or state management needed.

## Interactive State (Sidebar-Local)

Some state must remain in the sidebar renderer because it's inherently per-instance:

- **SidebarMode** (Passive/Navigate/Rename/Filter/Help): keyboard interaction state
- **Click regions**: from last render, for mouse hit testing
- **Cursor position**: navigation cursor index
- **Scroll offset**: which sessions are visible

The controller does NOT need to know about these.
When the user confirms an action (Enter to switch, 'd' to delete, rename complete), the renderer sends a pipe message to the controller.

## Layout Changes

### config.kdl (background plugin)

```kdl
load_plugins {
    "file:~/.config/zellij/plugins/cc_deck_controller.wasm" {
        mode "controller"
    }
}
```

### layout.kdl (thin sidebar)

```kdl
default_tab_template {
    pane split_direction="vertical" {
        pane size=22 borderless=true {
            plugin location="file:~/.config/zellij/plugins/cc_deck_sidebar.wasm" {
                mode "sidebar"
            }
        }
        children
    }
}
```

## Migration Path

1. Extract state management, git detection, sync, and hook processing into a `controller` module
2. Create a new `renderer` module with minimal display logic
3. Build two WASM binaries from the same crate (two `[[bin]]` targets)
4. Update the Go CLI to embed both WASM files
5. Update layout generation to include `load_plugins` for the controller
6. The renderer's `update()` becomes trivial: only handle `cc-deck:render` pipe, return false for everything else
7. The renderer's `pipe()` handles `cc-deck:render` (display) and user actions (forward to controller)

## Performance Impact

| Metric | Current (N instances) | Proposed (1 + N thin) |
|--------|----------------------|----------------------|
| PaneUpdate processing | N * full | 1 * full |
| TabUpdate processing | N * full | 1 * full |
| Timer processing | N * full | 1 * full |
| Hook processing | 1 (first match) | 1 (controller) |
| Render work per event | 1 (active tab) | 1 (active tab) |
| State serialization | N * per broadcast | 1 * per broadcast |
| WASM function calls per PaneUpdate | N | 1 + N (but N calls are trivial) |

The key win: expensive operations (rebuild_pane_map, remove_dead_sessions, git polling, JSON serialization) run exactly once, not N times.

## Open Questions

1. **Can background plugins call `switch_tab_to` / `focus_terminal_pane`?**
   Need to verify the controller has ChangeApplicationState permission.
   If not, the renderer would need to execute navigation actions locally.

2. **Pipe message ordering guarantees?**
   Does Zellij guarantee that pipe messages from controller arrive in order?
   If not, render payloads need sequence numbers.

3. **Startup sequencing?**
   Background plugins load before or after tab plugins?
   The sidebar renderer might receive events before the controller is ready.
   Solution: renderer shows "loading..." until first `cc-deck:render` arrives.

4. **Two binaries from one crate?**
   Can we use `[[bin]]` in Cargo.toml to produce both `cc_deck_controller.wasm` and `cc_deck_sidebar.wasm`?
   Or do we need feature flags to compile the same source into two variants?
