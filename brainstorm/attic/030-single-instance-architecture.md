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
- Persists state to /cache/sessions.json for reattach recovery (single writer, no sync needed)
- Manages sidebar discovery via hello/init handshake (tracks sidebar plugin_ids per tab)
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
- **my_tab_index**: assigned by the controller during hello/init handshake
- **Filter text**: local search buffer applied against the cached session list
- **Local notifications**: mode transition feedback ("Navigate mode", "Rename: ...")

The controller does NOT need to know about these.
When the user confirms an action (Enter to switch, 'd' to delete, rename complete), the renderer sends a pipe message to the controller.

## Sidebar Discovery Protocol

Sidebars cannot independently determine their tab_index (that requires PaneUpdate, which they don't subscribe to). Instead, the controller assigns it.

### Hello/Init Handshake

```
Sidebar loads → receives first cc-deck:render payload
     │
     v
Sends cc-deck:sidebar-hello { plugin_id } to controller
     │
     v
Controller cross-references PaneManifest to find
which tab contains a plugin pane matching this plugin_id
     │
     v
Controller responds cc-deck:sidebar-init { tab_index: N }
targeted via destination_plugin_id to this specific sidebar
     │
     v
Sidebar stores my_tab_index = N, ready for self-filtering
```

### Tab Reindexing

When TabUpdate shows a changed tab count (closure or addition), tab indices may shift. The controller broadcasts `cc-deck:sidebar-reindex` to all sidebars. Each surviving sidebar re-sends `cc-deck:sidebar-hello`, and the controller rebuilds the mapping.

Dead sidebars (from closed tabs) are silently cleaned up: targeted pipe messages to non-existent plugin_ids are dropped by Zellij without error.

### Navigation Mode Flow (Updated)

```
User presses Alt+s
     │
     v
Controller receives keybinding pipe
     │
     v
Controller broadcasts cc-deck:navigate { active_tab_index: 2 }
to all sidebars (via plugin_url targeting)
     │
     v
Each sidebar checks: my_tab_index == active_tab_index?
  - No  → ignore
  - Yes → enter navigate mode:
           set_selectable(true), focus_plugin()
           Capture Key events locally
           j/k moves cursor, re-renders locally
           Enter/d/r/p → send cc-deck:action to controller
           Esc → exit navigate mode locally
```

### Assumption to Verify

PaneInfo.id from PaneManifest must be usable to correlate with the plugin_id from `get_plugin_ids()`. If Zellij uses a unified ID space for plugin panes, this works directly. If not, the controller falls back to matching by pane title (sidebar sets a recognizable title).

## Notification Architecture

Two-level notification system to minimize pipe round-trips:

### Controller Notifications (in render payload)

Included in the `cc-deck:render` payload. All sidebars receive them, only the active one displays.

- Session attended/switched
- Session deleted
- Session state change errors
- Persist across tab switches (same payload goes to all sidebars)

### Sidebar-Local Notifications (never leave the sidebar)

Generated and consumed entirely within the sidebar instance. No pipe message needed.

- Mode transitions: "Navigate mode", "Filter: /api"
- Rename feedback: current text, cursor position
- Delete confirmation: "[y/N]" prompt
- Help overlay toggle

This keeps interactive feedback snappy (zero latency) while still allowing the controller to push state-change notifications.

## Local Filtering

Session filtering runs entirely in the sidebar, no controller involvement.

When the user types `/` in navigate mode:
1. Sidebar enters NavigateFilter mode
2. Key events build a local filter string
3. Sidebar filters the cached session list from the last render payload
4. Sidebar rebuilds click regions from the filtered list and re-renders
5. No communication with the controller until the user acts on a filtered result

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

## Sync Protocol Elimination

The current codebase has a complex 3-level sync system that exists solely because N instances each maintain independent state:

1. **Pipe broadcasts** (`cc-deck:sync` / `cc-deck:request`): instances broadcast their full session BTreeMap to peers, with timestamp-based merge resolution
2. **File-based metadata merging** (`/cache/session-meta.json`): rename/pause changes written to disk with `meta_ts` timestamps, polled by all instances on a 10s timer, hash-based change detection
3. **Full state restoration** (`/cache/sessions.json`): PID-tagged cache for reattach recovery, peer request protocol, 3s grace period for manifest stabilization

**All three levels are eliminated.** The controller is the single source of truth. No merge conflicts, no timestamp dominance, no stale metadata polling. Persistence reduces to the controller writing `/cache/sessions.json` on state changes.

Removed code: `sync.rs` module, `merge_sessions()`, `broadcast_and_save()`, `sync_dirty` flag, `pending_overrides`, `last_meta_content_hash`, `cc-deck:sync` pipe handler, `cc-deck:request` pipe handler, session-meta.json read/write. This is a significant reduction in complexity and eliminates an entire class of race conditions.

## Pipe Targeting Mechanism

Verified against Zellij 0.44 source. Three targeting modes exist:

### Controller to Sidebars (broadcast by plugin URL)

```rust
pipe_message_to_plugin(MessageToPlugin {
    plugin_url: Some("file:~/.config/zellij/plugins/cc_deck_sidebar.wasm".into()),
    destination_plugin_id: None,
    message_name: "cc-deck:render".into(),
    message_payload: Some(render_json),
    ..Default::default()
});
```

This sends to ALL running sidebar instances (same location + config match). Each sidebar receives the render payload and decides locally whether to render (active tab) or skip.

### Sidebar to Controller (targeted by plugin URL)

```rust
pipe_message_to_plugin(MessageToPlugin {
    plugin_url: Some("file:~/.config/zellij/plugins/cc_deck_controller.wasm".into()),
    destination_plugin_id: None,
    message_name: "cc-deck:action".into(),
    message_payload: Some(action_json),
    ..Default::default()
});
```

Only one controller instance exists, so URL targeting is sufficient.

### CLI Hook to Controller (targeted via `zellij pipe --plugin`)

```bash
zellij pipe --plugin "file:~/.config/zellij/plugins/cc_deck_controller.wasm" \
  --name cc-deck:hook -- "$payload"
```

The `--plugin` flag routes the pipe directly to the controller. No broadcast to sidebars needed.

### Plugin ID Alternative

Both plugins can discover their own ID via `get_plugin_ids().plugin_id`. The controller could include its plugin_id in the render payload, allowing sidebars to target it by ID for lower-overhead messaging. This is an optimization, not a requirement.

## Keybinding Registration

Keybindings are registered by the **controller**, not the sidebars.

Rationale: keybindings trigger pipe messages routed by Zellij to the plugin that registered them. If the controller registers, all keybinding actions (attend, navigate, etc.) arrive at the controller. The controller then either handles them directly (attend, new session) or forwards to the active-tab sidebar (enter navigate mode).

The current codebase defers keybinding registration to the first TabUpdate and only the active-tab instance registers. In the new model, the controller registers once during initialization (it receives TabUpdate as the single subscriber).

The controller includes the active tab's sidebar plugin_id in the render payload so it can forward navigation commands to the correct sidebar instance.

## Tab Rename

The controller handles tab auto-rename (from git repo detection) and user-initiated rename.

Currently, `rename_tab()` is called from the plugin that detects the git repo. In the new model, the controller runs git detection and calls `rename_tab()` directly. User-initiated renames arrive as `cc-deck:action` pipe messages from the sidebar, and the controller executes them.

## Hook Routing Changes

Current flow: `cc-deck hook` CLI command runs `zellij pipe --name cc-deck:hook` (broadcast to all instances). Every instance receives the hook event, but only the first match processes it.

New flow: `cc-deck hook` CLI command runs `zellij pipe --plugin "file:.../cc_deck_controller.wasm" --name cc-deck:hook` (targeted to controller). Single delivery, single processing. No wasted WASM cycles on N-1 sidebar instances.

The Go CLI `internal/cmd/hook.go` needs a one-line change: add `--plugin` flag to the `zellij pipe` invocation. The plugin path can be read from the installation config or hardcoded to the standard XDG path.

## Go CLI Embed Changes

The CLI currently embeds one WASM binary:

```go
//go:embed cc_deck.wasm
var pluginWasm []byte
```

New model requires two embedded binaries:

```go
//go:embed cc_deck_controller.wasm
var controllerWasm []byte

//go:embed cc_deck_sidebar.wasm
var sidebarWasm []byte
```

Changes needed:
- `PluginInfo` struct gains `ControllerBinary` and `SidebarBinary` fields (replaces single `Binary`)
- `plugin install` command writes both WASM files to `~/.config/zellij/plugins/`
- Layout generation adds `load_plugins` block for controller in config.kdl
- Layout generation updates `default_tab_template` to reference sidebar WASM
- `plugin uninstall` removes both files

## Build System (Two WASM Binaries)

Use feature flags with two `[[bin]]` targets in Cargo.toml:

```toml
[features]
default = []
controller = []
sidebar = []

[[bin]]
name = "cc_deck_controller"
path = "src/main.rs"
required-features = ["controller"]

[[bin]]
name = "cc_deck_sidebar"
path = "src/main.rs"
required-features = ["sidebar"]
```

Shared code (Session, Activity, HookPayload, render payload types) lives in `src/lib.rs`. The `main.rs` entry point uses `#[cfg(feature = "controller")]` and `#[cfg(feature = "sidebar")]` to select the ZellijPlugin implementation.

Makefile builds both:

```makefile
build-wasm:
	cargo build --target wasm32-wasip1 --release --features controller --bin cc_deck_controller
	cargo build --target wasm32-wasip1 --release --features sidebar --bin cc_deck_sidebar
```

## Resolved Questions

### Q1: Can background plugins call `switch_tab_to` / `focus_terminal_pane`?

**YES.** Verified in Zellij 0.44 source (`zellij-server/src/plugins/zellij_exports.rs`).

Both `switch_tab_to()` and `focus_terminal_pane()` have NO guards checking whether the calling plugin has a rendering surface. They directly route actions via `env.senders.send_to_screen()`. The permission system is identical for background and tab-template plugins: both require `ChangeApplicationState` permission.

**Permission granting caveat:** Background plugins call `RequestPluginPermissions` like any other plugin. Zellij shows a permission dialog at the screen level. On first load, the user must grant permissions; subsequent loads use the permission cache (keyed by plugin location string). This is the same UX as any new plugin installation. The sidebar and controller are separate plugin locations, so each needs its own permission grant on first use.

### Q2: Pipe message ordering guarantees?

Zellij processes pipe messages sequentially within the plugin thread (`zellij-server/src/plugins/mod.rs`). Messages from a single source arrive in order. The controller sends render payloads sequentially, so sidebars receive them in order. No sequence numbers needed.

However, if the controller sends a render payload and then immediately sends an action response, the sidebar sees them in order. Cross-plugin message ordering is guaranteed within a single Zellij server tick.

### Q3: Startup sequencing?

Background plugins loaded via `load_plugins` are initialized during session setup, before tabs are created. Tab-template plugins load when tabs are created. So the controller starts first.

The sidebar should show a "loading..." state until it receives its first `cc-deck:render` pipe message. This is a safe default since the controller will broadcast a render payload as soon as it processes its first PaneUpdate/TabUpdate event.

### Q4: Two binaries from one crate?

**Feature flags approach.** See "Build System" section above. Both binaries share `src/lib.rs` (types, serialization) but compile different `ZellijPlugin` implementations via `#[cfg(feature)]` guards. This keeps the codebase unified while producing two distinct WASM binaries.
