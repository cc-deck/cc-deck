# Pipe Protocol Contract: Controller-Sidebar Communication

## Overview

All communication between the controller and sidebar instances uses Zellij pipe messages with the broadcast+filter pattern. Messages are sent without URL or destination targeting. Each component filters by message name prefix.

## Message Names

| Name | Direction | Payload | Purpose |
|------|-----------|---------|---------|
| `cc-deck:render` | Controller → Sidebars | RenderPayload JSON | Broadcast display data |
| `cc-deck:action` | Sidebar → Controller | ActionMessage JSON | User-initiated actions |
| `cc-deck:sidebar-hello` | Sidebar → Controller | SidebarHello JSON | Registration request |
| `cc-deck:sidebar-init` | Controller → Sidebar | SidebarInit JSON | Tab assignment response |
| `cc-deck:sidebar-reindex` | Controller → Sidebars | (empty) | Tab indices changed, re-register |
| `cc-deck:navigate` | Controller → Sidebars | `{"active_tab_index": N}` | Enter navigate mode on active tab |
| `cc-deck:hook` | CLI → Controller | HookPayload JSON | Claude Code lifecycle event |

## Behavioral Requirements

### Render Broadcast (cc-deck:render)

1. Controller MUST broadcast a render payload after every state change that affects display.
2. Controller MUST coalesce rapid changes within 100ms before broadcasting (except user-initiated actions which bypass coalescing).
3. Sidebars MUST deserialize the payload and cache it locally.
4. Sidebars MUST re-render only if they are on the active tab (check `active_tab_index == my_tab_index`).
5. Sidebars MUST update their cached payload even when not rendering (so tab switches show current data).

### Action Messages (cc-deck:action)

1. Sidebar MUST include its `sidebar_plugin_id` in every action message.
2. Controller MUST validate that the target `pane_id` exists before processing.
3. Controller MUST send a notification in the next render payload if an action fails (e.g., session not found).
4. Controller MUST ignore action messages while permissions are not yet granted.

### Sidebar Registration (cc-deck:sidebar-hello / cc-deck:sidebar-init)

1. Sidebar MUST send `cc-deck:sidebar-hello` after receiving its first render payload (not on load, to avoid racing the controller).
2. Controller MUST respond with `cc-deck:sidebar-init` containing the assigned tab_index.
3. Controller MUST determine tab_index by cross-referencing the sidebar's plugin_id with the PaneManifest.
4. If the controller cannot determine the tab_index (plugin_id not found in manifest), it MUST NOT respond. The sidebar will retry on the next render payload.

### Tab Reindexing (cc-deck:sidebar-reindex)

1. Controller MUST broadcast `cc-deck:sidebar-reindex` when TabUpdate shows a changed tab count.
2. Sidebars MUST clear their `my_tab_index` and re-send `cc-deck:sidebar-hello`.
3. Controller MUST rebuild its `sidebar_registry` from the new hello responses.

### Hook Routing (cc-deck:hook)

1. CLI MUST target the controller binary via `zellij pipe --plugin "file:.../cc_deck_controller.wasm"`.
2. Controller MUST be the sole handler of hook events.
3. Sidebars MUST ignore `cc-deck:hook` messages (they will receive them via broadcast but should drop them).

## Message Filtering Rules

### Controller ignores:
- `cc-deck:render` (its own broadcasts)
- `cc-deck:sidebar-init` (its own responses)
- `cc-deck:sidebar-reindex` (its own broadcasts)
- `cc-deck:navigate` (its own broadcasts)

### Sidebar ignores:
- `cc-deck:hook` (controller-only)
- `cc-deck:action` (only controller processes)
- `cc-deck:sidebar-hello` (only controller processes)
- `cc-deck:render` when `initialized == false` and hello not yet sent (buffer until ready)

## Serialization

All payloads use JSON via serde_json. Types are defined in the shared `lib.rs` module.
