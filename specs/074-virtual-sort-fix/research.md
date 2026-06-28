# Research: Virtual Sort Fix for Sidebar Session Sort

## R1: Root Cause of Current Sort Failure

**Decision**: The current physical tab reorder via `Action::MoveTab` is unreliable due to async race conditions.

**Rationale**: `handle_sort()` fires multiple `switch_tab_to` + `move_tab_wasm` calls in a tight loop. Each is fire-and-forget (`object_to_stdout` + `host_run_plugin_command`). Zellij processes them asynchronously and sends `TabUpdate` events between moves. When `handle_tab_update` processes these intermediate events, `rebuild_pane_map()` at `state.rs:200` unconditionally overwrites `session.tab_index = Some(*idx)` with Zellij's authoritative `TabInfo.position`, destroying the proactive updates from `handle_sort()` lines 381-386. The sidebar then re-renders showing the pre-sort order.

**Alternatives considered**:
- Step-by-step queue (one move per TabUpdate round-trip): Reliable but slow (~50-100ms per step, visible shuffling)
- Hybrid (virtual view + deferred physical): Highest complexity, combines both mechanisms
- Suppressing TabUpdate during sort window: Risky, may miss real state changes

## R2: Virtual Sort Implementation Strategy

**Decision**: Add `sort_active: bool` to `ControllerState` and sort the render payload by `(tier, tab_index)` when active.

**Rationale**: The sort is a display concern for the sidebar. The `build_render_payload()` function in `render_broadcast.rs:13` already sorts sessions by `tab_index` (line 16). When `sort_active` is true, we instead sort by `(sort_tier(s), tab_index)` which groups sessions by activity tier while preserving relative order within tiers. No Zellij API calls needed.

**Key files**:
- `cc-zellij-plugin/src/controller/state.rs`: Add `sort_active: bool` field to `ControllerState`
- `cc-zellij-plugin/src/controller/actions.rs`: Simplify `handle_sort()` to toggle `sort_active`
- `cc-zellij-plugin/src/controller/render_broadcast.rs`: Conditional sort key in `build_render_payload()`
- `cc-zellij-plugin/src/controller/events.rs`: Clear `sort_active` on tab count change in `handle_tab_update()`
- `cc-zellij-plugin/src/lib.rs`: Add `sort_active: bool` to `RenderPayload` for sidebar indicator
- `cc-zellij-plugin/src/sidebar_plugin/render.rs`: Show sort indicator in header

## R3: Sort Indicator Design

**Decision**: Append a small sort symbol to the sidebar header when `sort_active` is true.

**Rationale**: The sidebar header already contains status counters. Adding a sort indicator (e.g., `↕` or `⇅`) is minimal and non-intrusive. The indicator appears when sort is active and disappears when deactivated or auto-cleared.

**Alternatives considered**:
- Color change on header: Too subtle, may conflict with existing color semantics
- Separate status line: Uses vertical space, not worth it for a boolean state

## R4: Toggle Behavior

**Decision**: Pressing S toggles `sort_active` on/off.

**Rationale**: The existing S handler in `sidebar_plugin/input.rs:449` sends `ActionType::Sort` to the controller. The controller's `handle_sort()` currently executes the swap sequence. We simplify it to toggle `sort_active` and call `mark_render_dirty()`. The sidebar input handler no longer needs the grace period reset (lines 465-467) since no tab switching occurs.

## R5: Auto-Deactivation on Tab Count Change

**Decision**: Clear `sort_active` in `handle_tab_update()` when `tab_count_changed` is true.

**Rationale**: When tabs are added or removed, the virtual sort order may be stale (new sessions are unclassified, removed sessions leave gaps). Clearing `sort_active` returns to natural order and forces the user to re-sort if desired. The existing `tab_count_changed` detection at `events.rs:23` is the right hook point.
