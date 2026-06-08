# Brainstorm: Sidebar Session Sort by Activity

**Date:** 2026-06-08
**Status:** active

## Problem Framing

On small displays with many sessions, the sidebar requires scrolling to see all entries. Active sessions (Working, Waiting) can be scattered among paused and idle sessions, forcing the user to scroll past inactive entries to find the ones that need attention. The goal is to cluster active sessions at the top of the sidebar so the "hot zone" is compact and visible without scrolling.

## Approaches Considered

### A: Swap-based tab reorder (chosen)

A keyboard shortcut (`S` in navigate/amber mode) physically reorders Zellij tabs using `move_focus_or_tab(Direction)` swap operations. Sessions are sorted into three tiers: Active (Working/Waiting) at the top, Inactive (Idle/Done/AgentDone/Init) in the middle, Paused at the bottom. Within each tier, the existing relative tab order is preserved (stable sort). The sidebar re-renders with the new order; no additional visual feedback.

- Pros: Tabs physically move, so sidebar position, tab shortcuts, and mental model all stay consistent. Swap-based stable sort means minimal tab movement (only cross-tier sessions move). Deliberate action avoids disorienting automatic reordering.
- Cons: Zellij has no direct `reorder_tab` API; must use `move_focus_or_tab` which moves one position at a time and requires the target tab to be focused. The swap sequence may be briefly visible.

### B: Close-and-recreate reorder

Close tabs and recreate them in the desired order.

- Pros: Achieves arbitrary reorder in one shot.
- Cons: Destructive. Loses terminal state, scrollback, running processes. Not viable.

### C: Sidebar-only virtual sort

Sort sessions visually in the sidebar without moving tabs. Tab indices shown as badges.

- Pros: Zero tab disruption. Instant.
- Cons: Tab numbers and sidebar positions diverge, creating confusion. Does not solve the underlying tab order problem.

## Decision

Approach A: Swap-based tab reorder via `S` in navigate mode. This directly solves the scrolling problem by physically moving tabs so active sessions cluster at the top. The action is deliberate (explicit keypress) so tab order remains stable and predictable during normal use.

## Key Requirements

- New `S` (shift-s) keybinding in navigate (amber) mode triggers sort
- Sort tiers: Active (Working, Waiting) > Inactive (Idle, Done, AgentDone, Init) > Paused
- Stable sort within each tier (preserves relative tab order among peers)
- Cursor follows the current session after sort completes
- No visual feedback beyond the re-rendered sidebar in its new order
- Help overlay (`?`) should document the new keybinding

## Open Questions

- How to handle `move_focus_or_tab` requiring the target tab to be focused: may need to temporarily switch focus during the sort sequence, then restore the original focus
- Whether the sort should be computed in the sidebar and sent as a series of swap actions, or sent as a single `Sort` action to the controller which handles the swap sequence internally
- Performance for large session counts (10+): how many swap operations are needed in the worst case and whether the visual shuffling is acceptable
