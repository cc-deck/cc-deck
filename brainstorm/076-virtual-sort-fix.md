# Brainstorm: Virtual Sort Fix for Sidebar Session Sort

**Date:** 2026-06-28
**Status:** active

## Problem Framing

The existing sidebar session sort (spec 067) physically reorders Zellij tabs via `Action::MoveTab` when the user presses S in navigate mode. This causes a race condition: `MoveTab` is async (fire-and-forget), and Zellij sends `TabUpdate` events between moves that overwrite the plugin's proactive state updates. The result is that the sort appears to work for a moment, then reverts to the original order on the next sidebar refresh.

Root cause: `rebuild_pane_map()` in the controller unconditionally sets `session.tab_index = Some(*idx)` from Zellij's authoritative `TabInfo.position`, overwriting the proactive updates made by `handle_sort()`.

The Zellij API has no synchronous tab reorder function or `reorder_tab(from, to)` API. The only option is `MoveTab { direction }`, which moves the focused tab one position at a time.

## Approaches Considered

### A: Step-by-step queue (reliable but slow)

Queue all moves as `SortStep` structs, execute one per `TabUpdate` round-trip to ensure each move is confirmed before the next.

- Pros: Reliable, tabs physically move.
- Cons: Slow (~50-100ms per step, visible tab shuffling for 5+ sessions). Adds complexity with a sort queue in `ControllerState` and interception of `handle_tab_update`.

### B: Virtual/display-only sort (chosen)

The sidebar renders sessions in tier-sorted order without touching Zellij tabs. The controller stores a `sort_active: bool` flag; when set, the render broadcast sorts sessions by `(tier, tab_index)` instead of just `tab_index`.

- Pros: Instant, zero race conditions, minimal code change (~30-40 lines). No Zellij API calls needed.
- Cons: Zellij tab bar still shows original order, but the sidebar is the primary navigation interface.

### C: Hybrid (virtual view + deferred physical reorder)

Show sorted order immediately in sidebar, then queue physical moves in the background.

- Pros: Best UX (instant visual + eventual physical consistency).
- Cons: Highest complexity, combines both mechanisms.

## Decision

Approach B: Virtual/display-only sort. The sorting is a convenience feature for the sidebar. Users interact through the sidebar, not the tab bar. The spec's FR-004 ("physically reorder Zellij tabs") was written assuming a reliable reorder API exists, but the async `MoveTab` API makes physical reordering unreliable. The intent is "sorting should be consistent," which virtual sort achieves.

## Key Changes from Spec 067

- Remove all `MoveTab` / `switch_tab_to` calls from `handle_sort`
- Replace with a simple `sort_active = true` flag + `mark_render_dirty()`
- Modify render broadcast to sort by `(tier, tab_index)` when `sort_active` is set
- Clear `sort_active` when tab count changes (new/closed tab invalidates sort)
- Toggle behavior: pressing S again while `sort_active` clears the flag (unsort)
- Cursor tracking remains the same (by pane_id)

## Key Requirements

- S keybinding in navigate mode toggles virtual sort on/off
- Sort tiers unchanged: Active (Working, Waiting) > Inactive (Idle, Done, AgentDone, Init) > Paused
- Stable sort within tiers (preserves relative tab order)
- Cursor follows current session after sort
- Help overlay documents S keybinding (already done)
- Sort indicator in sidebar header when sort is active (visual feedback that display order differs from tab order)

## Open Questions

- None. This is a focused bug fix with a clear implementation path.
