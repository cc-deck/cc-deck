# Empirical Testing Notes: Multiplayer Focus Modes

**Date**: 2026-07-23
**Tester**: Roland
**Environment**: macOS, Zellij 0.44.1, two terminal windows attached to same session

## Bugs Found

### BUG-1: Stale client_focus not cleaned up on disconnect

**Severity**: Critical
**Symptom**: Presence indicator blocks persist after the second client disconnects. Reconnecting a second time shows TWO blocks (one stale from the first connection, one new).
**Root cause**: `client_focus` cleanup is tied to `cleanup_dead_sidebars()` which only removes sidebar registry entries not in the pane manifest. Zombie sidebars (Zellij #4064) remain in the manifest after client disconnect, so their `client_focus` entries are never removed.
**Fix direction**: Track connected client count independently. Options:
- Use `TabInfo.other_focused_clients` to detect active clients
- Subscribe to `SessionUpdate` for connected_clients count
- Timer-based sweep: remove `client_focus` entries not refreshed within N seconds

### BUG-2: Auto-sort keeps rearranging sessions

**Severity**: Important
**Symptom**: Sessions in the auto-sort active zone keep shuffling on every click from either client.
**Root cause**: `handle_focus_report` calls `mark_render_dirty()` + the auto-unpause logic modifies `auto_sort_tail`. Each render broadcast recalculates the auto-sort partition, which changes session positions.
**Fix direction**: 
- Don't modify `auto_sort_tail` in `handle_focus_report` (only do the unpause, skip the tail tracking)
- Consider: should focus-report trigger a full render broadcast, or just update the client_focus map and let the next natural render cycle pick it up?

### BUG-3: Sidebar doesn't highlight active session for secondary client

**Severity**: Important
**Symptom**: After client 2 clicks a session, the sidebar doesn't show it as the active (highlighted) session.
**Root cause (hypothesis)**: The payload's `focused_pane_id` reflects the controller's (client 1's) focus. Client 2's sidebar relies on `local_focus_override`, but this might be getting cleared or not persisting across render cycles. Need to verify: does `local_focus_override` survive payload updates?

### BUG-4: ModeUpdate may not fire on first connection

**Severity**: Important
**Symptom**: On the first multiplayer connection, Zellij's own border colors and tab indicators were missing. They appeared on the second connection attempt.
**Root cause (hypothesis)**: `ModeUpdate` event delivery may be delayed or skipped on the first client attach. If `multiplayer_colors` is `None`, the `build_other_client_focus` guard returns empty, so no indicators should show. But the user DID see an indicator (single color). Either the guard isn't working, or there's another rendering path producing the block.
**Action**: Enable debug logging, reproduce, and check for the "CTRL MODE_UPDATE" log line.

### BUG-5: Single color for both clients' indicators

**Severity**: Minor
**Symptom**: Both clients' presence indicators showed the same color.
**Root cause**: Possibly tied to BUG-4 (no palette available). If `multiplayer_colors` is `None`, indicators should not render at all. If they do render with a hardcoded color, that explains the single-color issue.
**Action**: Investigate whether there's a code path that renders presence indicators without the palette check.

## Observations

- Zellij assigns incrementing `client_id` values. After disconnect + reconnect, the new connection gets a higher `client_id` (e.g., 3 instead of 2).
- Zombie sidebars (Zellij #4064) remain functional in the pane manifest. This means `cleanup_dead_sidebars()` never removes them, and `client_focus` entries for disconnected clients persist indefinitely.
- The 36-sidebar broadcast count suggests significant zombie accumulation across multiple attach/detach cycles.
- Zellij's own multiplayer indicators (border colors, tab cursor blocks) appear correctly on the second connection but not the first.

## Testing Session 2 (2026-07-23, after initial fixes)

Fixes applied so far:
- Local activation order guarded (only in multiplayer)
- Activity tier sort (working sessions at top of active zone)
- Multiplayer detection from sidebar registry (not client_focus)
- Per-client focused_pane_id override in broadcast

### BUG-6: Both terminals' highlights jump on client 2 click

**Severity**: Critical
**Symptom**: When clicking a session in terminal 2's sidebar, BOTH terminals' sidebar highlights jump to that session. Only the right terminal (client 2) correctly switches the main terminal pane.
**Root cause (hypothesis)**: The controller's `focused_pane_id` is still being updated somewhere in the focus-report path (possibly via `rebuild_pane_map` reading the manifest after `focus_terminal_pane` changes the actual Zellij focus). The per-client override in `broadcast_render` may not be working, or there's a code path that broadcasts without the per-client customization.

### BUG-7: Cyan highlight lands on unrelated sessions

**Severity**: Critical
**Symptom**: After clicking sessions on client 2, the cyan highlight on client 1 appears on a session that was NOT clicked. Non-deterministic.
**Root cause (hypothesis)**: Either `local_focus_override` is being set/cleared incorrectly, or the `focused_pane_id` in the payload doesn't match any visible session (causing the highlight to fall through to a default). May also be related to `in_flight_focus` protection in `rebuild_pane_map` overwriting `focused_pane_id` with stale data.

### Key debugging approach for next session

The right approach is instrument-first:
1. Add trace logs at EVERY read/write of `focused_pane_id`, `local_focus_override`, and `client_focus`
2. Create a step-by-step reproduction procedure
3. Capture the log and trace the exact event sequence
4. Fix based on evidence, not hypotheses

Critical code paths to instrument:
- `rebuild_pane_map()` (state.rs) - where it sets `focused_pane_id` from manifest
- `handle_focus_report()` (actions.rs) - the focus-report handler
- `broadcast_render()` (render_broadcast.rs) - per-client payload building
- `effective_focused_pane_id()` (sidebar state.rs) - what the sidebar uses for highlight
- `switch_focus_locally()` (input.rs) - where local_focus_override is set
- `in_flight_focus` handling in `rebuild_pane_map()` - may be overriding the per-client focused_pane_id

## Next Steps

1. `/clear` and start fresh debugging session
2. Read this file and `followup-tasks.md` for context
3. Add comprehensive trace logs (see "Key debugging approach" above)
4. Reproduce with two terminals, capture log, trace the event sequence
5. Fix based on evidence

## State-model refactor (2026-07-24)

The multiplayer implementation was replaced with a single authoritative
`client_views` projection keyed by Zellij client ID. Each entry owns its active
tab, exact focused pane, activation order, revision, and a short-lived pending
focus intent. The controller no longer derives focus from the global
`PaneInfo.is_focused` flag.

This addresses the observed failures as follows:

- BUG-1: connected clients are reconciled from `TabUpdate` and stale client
  views and sidebar registrations are pruned independently of zombie panes.
- BUG-2: focus reports do not modify the controller's auto-sort tail; ordering
  is projected per client without mutating the shared session list.
- BUG-3, BUG-6, BUG-7: every sidebar highlights only its own client view, with
  a predictive local override retained until that exact view acknowledges it.
  Ambiguous tabs intentionally have no highlight.
- BUG-4, BUG-5: presence is derived locally from the same client-view map and
  uses Zellij colors when available, otherwise the documented fallback palette.
- Sidebar discovery is now an explicit hello/init handshake with retry, rather
  than pane-manifest inference.
- The sidebar UI state machine is reduced to Passive, Navigate-with-overlay,
  and RenamePassive; navigation cursors are stable pane IDs rather than indices.

Automated Rust tests and fuzz invariants cover independent clients,
disconnect cleanup, ambiguous focus, protocol round trips, and cursor identity.
The two-terminal V1-V7 walkthrough still requires a live Zellij session after
installing the rebuilt WASM.

### Stable-zone correction (2026-07-24)

Per-client MRU ordering and activity-tier sorting were removed. Both caused
focus clicks or ordinary activity changes to reshuffle the active auto-sort
zone. The controller now preserves relative order within each zone. Pausing is
the only way a session leaves the active zone; reactivation appends it.
