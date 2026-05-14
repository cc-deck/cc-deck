# Data Model: Render Pipeline Stability

**Date**: 2026-05-14

## Entities

This feature modifies existing entities rather than introducing new ones. Key entities affected:

### ControllerState (modified)

Existing entity in `controller/state.rs`. Changes:

- **`disabled: bool`** (new field): Set to true when the startup probe detects a higher-priority controller. When true, the controller skips all pipe message processing and timer handling.

### SidebarState (modified)

Existing entity in `sidebar_plugin/state.rs`. Changes:

- **`render_request_sent: bool`** (new field): Tracks whether the one-shot render request has been sent. Set to true after the fallback request fires (3 ticks after init with no payload). Prevents duplicate requests.
- **`ticks_since_init: u8`** (new field): Counter incremented on each timer tick after initialization. Used to trigger the one-shot render request at tick 3.

### PipeAction (modified)

Existing enum in `pipe_handler.rs`. New variants:

- **`ControllerPing`**: Startup probe message (`cc-deck:controller-ping`). Payload contains the sender's plugin_id.
- **`ControllerPong`**: Startup probe response (`cc-deck:controller-pong`). Payload contains the responder's plugin_id.
- **`RenderRequest`**: Sidebar requests initial render (`cc-deck:render-request`). Payload contains the sidebar's plugin_id.

### PerfTracker (unchanged structure, new events)

New event labels recorded via existing `record_raw()`:

- `render:broadcast` - render broadcast invocations (count + serialization time in us)
- `render:pipe_send` - individual pipe_message_to_plugin calls (count)
- `render:skipped` - flush_render calls where render_dirty was false (count)

### DebugLogger (new internal concept, not a struct)

The buffered debug logging approach uses a module-level buffer:

- **`LOG_BUFFER: Vec<String>`** (static mutable): Accumulates log lines between flushes
- **`BUFFER_CAPACITY: usize`**: Maximum lines before forced flush (suggested: 50)
- Flushed on timer tick or when capacity exceeded

## State Transitions

### Controller Startup Probe

```
load() -> permissions_granted -> send ControllerPing (broadcast)
  |
  +-> receive ControllerPong from lower plugin_id -> set disabled=true, log warning
  |
  +-> receive ControllerPing from higher plugin_id -> send ControllerPong, continue normally
  |
  +-> no response within 3 ticks -> continue normally (sole controller)
```

### Sidebar Render Request Fallback

```
load() -> receive SidebarInit -> ticks_since_init=0
  |
  +-> receive cc-deck:render -> initialized=true (normal path)
  |
  +-> tick 1, tick 2, tick 3 (no render received):
      send cc-deck:render-request -> render_request_sent=true
  |
  +-> receive cc-deck:render -> initialized=true (delayed path)
```

## Pipe Message Summary

| Message | Direction | Targeted? | Purpose |
|---------|-----------|-----------|---------|
| `cc-deck:controller-ping` | Controller -> all | Broadcast | Startup probe |
| `cc-deck:controller-pong` | Controller -> controller | Targeted (by plugin_id) | Probe response |
| `cc-deck:render-request` | Sidebar -> controller | Targeted (by controller_plugin_id) | One-shot render request |
| `cc-deck:render` | Controller -> sidebar | Targeted | Render payload (existing) |
