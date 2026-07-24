# Research: Multiplayer Focus Modes

**Date**: 2026-07-22
**Feature**: 083-multiplayer-focus-modes

## R1: Per-Client Plugin API Behavior

**Decision**: Sidebar calls `focus_terminal_pane()` and `switch_tab_to()` directly from its own plugin instance.

**Rationale**: Zellij's plugin API is inherently per-client. Each plugin instance is associated with a `client_id` via `get_plugin_ids()`. API actions implicitly target the owning client. Confirmed by `group_and_ungroup_panes()` which has an explicit `for_all_clients: bool` parameter, demonstrating per-client is the default. No client_id parameter needed on focus/tab calls.

**Alternatives considered**:
- Route through controller with client_id parameter: rejected because the API already handles per-client routing implicitly
- Use `list_clients()` to iterate and target: overcomplicated, not needed

## R2: Focus Reporting Mechanism

**Decision**: Sidebar sends `cc-deck:focus-report` pipe message to controller after local focus switch.

**Rationale**: The controller needs to track which session each client is focused on for two purposes: (1) activity tracking (last-active timestamps) which the controller already does for Switch actions, and (2) populating presence indicator data in the render payload. Pipe messages are the established inter-plugin communication channel in cc-deck.

**Alternatives considered**:
- Controller polls sidebars for their focus state: adds latency, violates push-based architecture
- Direct sidebar-to-sidebar communication: sidebars don't know each other's plugin_ids without controller mediation

## R3: Local vs Global Activation Order

**Decision**: Each sidebar maintains a `local_activation_order: Vec<u32>` (pane IDs, most-recently-focused first). Sessions not in the local order are appended using the controller's global sort.

**Rationale**: The controller's `sort_order` and `auto_sort_tail` are global. When client 2 switches sessions, the controller updates the global order, which reorders client 1's sidebar. Per-client ordering requires the sidebar to own its sort state. The sidebar already caches the render payload (`cached_payload`), so applying a local sort is a lightweight operation.

**Alternatives considered**:
- Controller tracks per-client sort orders: adds complexity to controller state and render payload serialization
- Don't change ordering (keep global): users confirmed this is disruptive in multiplayer

## R4: ModeUpdate Subscription for Theme Colors

**Decision**: Controller subscribes to `EventType::ModeUpdate` and extracts `multiplayer_user_colors` from `ModeInfo.style.colors`.

**Rationale**: The multiplayer user colors in the Zellij palette are the same colors used in the tab bar for client indicators. Using them ensures visual consistency. The controller already subscribes to multiple event types; adding one more is minimal overhead. `ModeUpdate` fires on mode changes (normal, locked, etc.) which are infrequent.

**Alternatives considered**:
- Hardcode RGB values matching Zellij's default theme: breaks with custom themes
- Subscribe from sidebar instead of controller: each sidebar subscribes independently (more subscriptions), harder to include colors in render payload

## R5: Render Payload Extension

**Decision**: Add `other_client_focus` field to `RenderPayload` containing per-session presence data for other clients. Sidebar's own focus continues using `local_focus_override` / `effective_focused_pane_id()`.

**Rationale**: The render payload already carries all data sidebars need to render. Adding focus data for other clients fits this pattern. The sidebar already has `local_focus_override` for immediate predictive highlighting (set on click before the render cycle completes), and `effective_focused_pane_id()` which prefers the override. This means the sidebar doesn't rely on the payload's `focused_pane_id` for its own highlighting when the override is set.

**Alternatives considered**:
- Build separate render payloads per client with different `focused_pane_id`: more serialization work, harder to maintain, and sidebars already handle their own focus via `local_focus_override`
- Send focus map in a separate pipe message: adds complexity, splits data that renders together

## R6: Keyboard Shortcut Limitation

**Decision**: Accept that keyboard shortcuts (alt-s, alt-a, alt-w) only work for the primary client. Document the limitation.

**Rationale**: Keybindings are registered via `reconfigure()` with `MessagePlugin` (broadcast). Pipe messages don't include `client_id` (Zellij PR #4094 unmerged). The controller cannot determine which client pressed the key, so it forwards to its own client's sidebar. Other clients use mouse clicks on the sidebar, which work correctly via local focus handling.

**Alternatives considered**:
- Register keybindings per-client: `reconfigure()` is global, not per-client
- Forward to all clients' sidebars: all clients would enter navigate mode simultaneously, confusing
- Wait for Zellij PR #4094: indeterminate timeline, blocks the feature

## R7: Existing local_focus_override Pattern

**Decision**: Leverage the existing `local_focus_override` pattern for per-client focus highlighting.

**Rationale**: The sidebar already has a `local_focus_override: Option<u32>` field (state.rs line 43) that provides immediate predictive highlighting when a user clicks a session, before the controller's render cycle completes. The `effective_focused_pane_id()` method (line 162) prefers this override over the payload's `focused_pane_id`. For multiplayer independent focus, we extend this: the sidebar always sets `local_focus_override` on click/keyboard selection and never clears it until the next explicit focus action. This means each sidebar's highlight is controlled locally, regardless of what the controller's global `focused_pane_id` says.

**Alternatives considered**:
- Replace `focused_pane_id` with per-client map in payload: would require more payload changes and break single-client backward compatibility
- Remove `focused_pane_id` from payload entirely: breaks the single-client case where local_focus_override hasn't been set yet
