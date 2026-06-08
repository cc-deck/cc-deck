# Research: Sidebar Session Sort by Activity

## R1: Zellij Tab Reorder API

**Decision**: Use `move_focus_or_tab(Direction)` from zellij-tile 0.44

**Rationale**: This is the only API in zellij-tile that can physically move a tab's position. It moves the currently focused tab one position in the given direction (Left or Right). Confirmed present in `zellij-tile-0.44.3/src/shim.rs:1120`. The project already uses zellij-tile 0.44 (`Cargo.toml`).

**Alternatives considered**:
- Direct `reorder_tab(from, to)`: Does not exist in zellij-tile API
- Close and recreate tabs: Destructive, loses terminal state
- Display-only sort: Does not solve the tab position problem

**Constraint**: `move_focus_or_tab` operates on the currently focused tab. To move an arbitrary tab, the controller must first switch focus to that tab (`switch_tab_to`), then call `move_focus_or_tab`. After the sort sequence completes, focus must be restored to the sidebar plugin pane.

## R2: Sort Algorithm for Swap-Based Reorder

**Decision**: Compute target order, then execute minimal swap sequence

**Rationale**: The sort has three steps:
1. **Snapshot** current sessions with their tab indices and states
2. **Compute target order**: stable partition into tiers (Active > Inactive > Paused), preserving relative order within each tier
3. **Execute swaps**: Compare current positions to target positions. Use a bubble-sort approach since `move_focus_or_tab` can only move one position at a time. Start from the leftmost target position and bubble each session into place.

**Complexity**: For N sessions with K out-of-place sessions, the number of swap operations is O(K*N) in the worst case. For typical use (3-5 sessions moving), this is 10-20 swaps, each being a single API call.

**Alternatives considered**:
- Insertion sort approach: Similar complexity, harder to implement with one-position-at-a-time moves
- Selection sort: Would require more focus switches

## R3: Focus Management During Sort

**Decision**: The controller handles the entire sort sequence internally, restoring focus after completion

**Rationale**: The sort action is sent from the sidebar to the controller. The controller:
1. Saves the current focus state
2. Executes the swap sequence (switching tabs and moving them)
3. After completion, broadcasts a render update which causes the sidebar to re-render
4. The sidebar's navigate mode cursor tracks by pane_id, not by position, so cursor follow is handled by recalculating the cursor index from the session list after the sort

**Key insight**: The sidebar does NOT need to know about individual swap steps. It sends `Sort`, the controller handles everything, and the next render broadcast reflects the new tab order. The sidebar just needs to update `cursor_index` to match the new position of the currently highlighted session.

## R4: Paused Field vs Activity Enum

**Decision**: Check `session.paused` boolean separately from `Activity` enum for tier classification

**Rationale**: In the Session struct (`session.rs`), `paused` is a separate `bool` field, not an `Activity` variant. A paused session can have any underlying activity state. The tier classification should be:
1. If `session.paused == true` -> Tier 3 (Paused), regardless of activity
2. If `activity` is Working or Waiting -> Tier 1 (Active)
3. Otherwise -> Tier 2 (Inactive)

This means a paused-but-working session goes to Tier 3, which is correct: the user explicitly paused it, so it should sink to the bottom.
