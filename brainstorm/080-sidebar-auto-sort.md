# Brainstorm: Sidebar Auto-Sort with Pause Zone

**Date:** 2026-07-11
**Status:** active

## Problem Framing

The existing sidebar sort (spec 067, refined in spec 074) requires a manual keypress (S in navigate mode) to reorder sessions. In practice, the most common scenario is that paused sessions clutter the sidebar among active ones. Users want paused sessions to automatically drop to the bottom without needing to press S every time a session state changes.

The goal: paused sessions should automatically sink to the bottom of the sidebar, with a visual separator marking the boundary. Everything else stays in the active zone. Minimal movement, no surprises.

## Approaches Considered

### A: Always-on auto-sort with separator (chosen)

The sidebar automatically maintains two zones: Active (top) and Paused (bottom), separated by a visual line. When a session is paused, it moves below the separator. When unpaused, it rejoins the active zone at the bottom (least disruptive position). The existing S keybinding still works for finer three-tier sorting within the active zone.

- Pros: Zero manual intervention for the common case. Separator provides clear visual feedback. Coexists with manual sort. Configurable via config file.
- Cons: Adds a new rendering element (separator line). Sessions shift position on pause/unpause which could be briefly disorienting.

### B: Extend manual sort to auto-trigger on state changes

Make the existing S sort re-run automatically whenever a session changes state.

- Pros: Reuses existing sort logic entirely.
- Cons: Three-tier sort on every state change is too aggressive. Sessions jump around constantly as they transition between Working/Waiting/Idle. The manual sort is deliberate by design.

### C: Pin/unpin model

Let users pin specific sessions to the top. Unpinned sessions fall to a lower section.

- Pros: Full user control over which sessions are "important."
- Cons: Requires manual pinning per session. Does not solve the automatic clustering problem.

## Decision

Approach A: Always-on auto-sort with two zones (Active/Paused) separated by a visual line. The separator doubles as an indicator that auto-sort is enabled. Configurable via `sidebar.auto_sort` in `~/.config/cc-deck/config.yaml` (default: true). Manual S keybinding coexists for finer three-tier sorting within the active zone.

## Key Requirements

- Two zones: Active (Working, Waiting, Idle, Done, AgentDone, Init) on top, Paused on bottom
- Visual separator line between zones when auto-sort is active
- Separator only visible when at least one session is paused (no empty paused zone)
- Triggered automatically on session state transitions (pause/unpause)
- Pausing sinks session below separator; unpausing adds to end of active zone
- Configurable: `sidebar.auto_sort: true` (default) in cc-deck config
- No separator rendered when auto-sort is disabled (feature completely transparent)
- Coexists with existing S keybinding for manual three-tier sort
- Virtual sort only (no physical tab reordering, consistent with spec 074)
- Stable ordering within each zone (preserve relative tab order)

## Open Questions

- What style for the separator line? Thin horizontal rule, dashed line, or a labeled divider (e.g., "--- paused ---")?
- Should the separator be a full-width line or indented to match session entry alignment?
