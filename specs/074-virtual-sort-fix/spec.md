# Feature Specification: Virtual Sort Fix for Sidebar Session Sort

**Feature Branch**: `074-virtual-sort-fix`
**Created**: 2026-06-28
**Status**: Draft
**Input**: User description: "Replace broken physical tab reorder with virtual display-only sort in sidebar"

## User Scenarios & Testing

### User Story 1 - Sort Sessions by Activity (Priority: P1)

A user has 8+ sessions open across multiple tabs. Some are actively working, some are idle, and two are paused. The active sessions are scattered at positions 2, 5, and 7 in the tab bar. The user enters navigate mode (Alt+s) and presses Shift+S. The sidebar re-renders with the three active sessions at the top, idle sessions in the middle, and paused sessions at the bottom. The Zellij tab bar remains unchanged; the sidebar shows a virtual sorted view.

**Why this priority**: This is the core feature. Without it, the sort is broken due to the MoveTab race condition.

**Independent Test**: Create sessions in various activity states, press S in navigate mode, and verify the sidebar displays sessions grouped by tier while the underlying tab positions remain unchanged.

**Acceptance Scenarios**:

1. **Given** 5 sessions with mixed states (2 Working, 1 Idle, 1 Done, 1 Paused) in tab order [Idle, Working, Paused, Done, Working], **When** the user presses S in navigate mode, **Then** the sidebar displays [Working, Working, Done, Idle, Paused] and the Zellij tab bar order is unchanged.
2. **Given** sessions are already sorted by activity tiers, **When** the user presses S, **Then** the sidebar shows the same order (no visible change) and sort becomes active.
3. **Given** 3 Working sessions at tab positions 1, 4, 6, **When** the user presses S, **Then** the sidebar displays the 3 Working sessions at the top with their original relative order preserved.

---

### User Story 2 - Re-Sort with Updated States (Priority: P1)

The user has activated sort mode and session states have changed since the last sort. They press S again in navigate mode. The sidebar re-computes the sort with the current activity states, producing a fresh snapshot.

**Why this priority**: Users need the ability to refresh the sort after session activity changes.

**Independent Test**: Activate sort with S, change a session's activity state, press S again, verify the new order reflects the updated states.

**Acceptance Scenarios**:

1. **Given** sort is not active, **When** the user presses S, **Then** sort becomes active and the sidebar displays sessions grouped by tier.
2. **Given** sort is active and a session changed from Working to Idle, **When** the user presses S, **Then** the sort is re-computed and the session moves to the Idle tier.

---

### User Story 3 - Cursor Follows Current Session (Priority: P1)

The user is in navigate mode with the cursor on session "api-server". They press S to sort. After the sort completes, the cursor remains on "api-server" even though its display position changed.

**Why this priority**: Losing the cursor position after sort would be disorienting and break the user's workflow.

**Independent Test**: Enter navigate mode, position cursor on a specific session, press S, verify cursor still highlights the same session.

**Acceptance Scenarios**:

1. **Given** the cursor is on session "api" at display position 3, **When** the user presses S and "api" moves to display position 1, **Then** the cursor is on display position 1 (still highlighting "api").
2. **Given** the cursor is on a session that does not change display position, **When** the user presses S, **Then** the cursor position is unchanged.

---

### User Story 4 - Sort Indicator (Priority: P2)

When sort is active, the sidebar header shows a visual indicator so the user knows the display order differs from the tab bar order.

**Why this priority**: Without a visual indicator, users may be confused about why the sidebar and tab bar show different orders.

**Independent Test**: Activate sort, verify indicator appears in header. Deactivate sort, verify indicator disappears.

**Acceptance Scenarios**:

1. **Given** sort is not active, **When** the user views the sidebar header, **Then** no sort indicator is present.
2. **Given** sort is active, **When** the user views the sidebar header, **Then** a sort indicator is visible in the header area.

---

### User Story 5 - Sort Persists Across Tab Changes (Priority: P2)

When a new tab is opened or a tab is closed while sort is active, the sort order is preserved. New sessions are appended at the end of the sorted list, and removed sessions are pruned from the sort order.

**Why this priority**: Preserving sort across tab changes prevents the user from losing their carefully arranged session order.

**Independent Test**: Activate sort, open a new session tab, verify sort persists with new session at the bottom. Close a tab, verify sort persists without the removed session.

**Acceptance Scenarios**:

1. **Given** sort is active with 5 sessions, **When** a new session tab is opened, **Then** sort remains active and the new session appears at the bottom of the sorted list.
2. **Given** sort is active with 5 sessions, **When** a session tab is closed, **Then** sort remains active and the closed session is removed from the sorted list.

---

### User Story 6 - Manual Reorder with Shift+J/K (Priority: P2)

The user wants to manually adjust the position of a session within the sorted (or unsorted) view. In navigate mode, they press Shift+J to move the session at the cursor down, or Shift+K to move it up. If no sort order exists, one is initialized from the current tab order.

**Why this priority**: Users need fine-grained control over session ordering beyond automatic tier-based sorting.

**Independent Test**: Enter navigate mode, position cursor on a session, press Shift+J to move it down and Shift+K to move it up. Verify the session swaps with its neighbor and the cursor follows.

**Acceptance Scenarios**:

1. **Given** sort is active and the cursor is on a session at position 2, **When** the user presses Shift+J, **Then** the session moves to position 3 and the cursor follows.
2. **Given** sort is active and the cursor is on the first session, **When** the user presses Shift+K, **Then** nothing happens (boundary no-op).
3. **Given** sort is NOT active, **When** the user presses Shift+J, **Then** sort_order is initialized from tab order, the session is moved, and the sort indicator appears.

---

### Edge Cases

- What happens when there are 0 or 1 sessions? The sort is a no-op; pressing S has no visible effect.
- What happens when all sessions are in the same tier? The sort activates but the display order matches the natural order (no visible change).
- What happens if a session changes activity state while sort is active? The sort is a frozen snapshot. Activity changes do NOT auto-re-sort. The user must press S again to re-sort with current states.
- What happens if sort is pressed outside navigate mode? The keybinding has no effect; sort only works in navigate mode.

## Requirements

### Functional Requirements

- **FR-001**: The sidebar MUST retain the existing S (Shift+s) keybinding in navigate mode that triggers a sort-by-activity action.
- **FR-002**: The sort MUST group sessions into four tiers: Tier 0 Active (Working, Waiting) at top, Tier 1 Done (Done, AgentDone) next, Tier 2 Idle (Idle, Init) below, and Tier 3 Paused (`session.paused == true`, regardless of activity state) at bottom. Done sessions are separated from Idle to keep sessions that need attention visible above fully idle ones.
- **FR-003**: The sort MUST be stable within each tier, preserving the relative tab order of sessions that belong to the same tier.
- **FR-004**: The sort MUST be display-only; it MUST NOT call any Zellij tab reorder APIs (`MoveTab`, `switch_tab_to` for sort purposes). Zellij tab positions MUST remain unchanged.
- **FR-005**: The navigate mode cursor MUST follow the current session after the sort completes, updating its display position to match the session's new index in the sorted view.
- **FR-006**: Pressing S MUST always re-compute the sort with current activity states (one-shot snapshot). Each press produces a fresh frozen order reflecting the latest session activities. There is no toggle-off behavior; the sort remains active until the user navigates away or the sort_order is otherwise cleared.
- **FR-007**: The sidebar MUST display a visual sort indicator when sort is active, so the user can distinguish sorted view from natural order.
- **FR-008**: When the tab count changes (new tab opened or tab closed), the sort MUST be preserved. Dead pane_ids are removed from the sort order, and new sessions are appended at the end in tab_index order. The sort does NOT auto-deactivate on tab count changes.
- **FR-009**: The `sort_active` flag MUST be maintained in the controller state (not the sidebar) so all sidebar instances reflect the same sort state. Transient cursor-tracking state (`sort_cursor_pane_id`) MAY remain in the sidebar, as it is consumed locally on each render cycle.
- **FR-010**: The help overlay (? key) MUST continue to document the S keybinding.
- **FR-011**: Shift+J and Shift+K in navigate mode MUST move the session at the cursor down or up in the sort order. If no sort_order exists, one is initialized from the current tab order. The cursor MUST follow the moved session.
- **FR-012**: When sessions are removed via any path (delete action, pane closed, dead session cleanup, unconfirmed session removal), the corresponding pane_id MUST be removed from the sort_order to prevent ghost entries.
- **FR-013**: When sort is active, the scroll viewport anchor MUST use the focused terminal pane_id (or restore_pane_id in navigate mode) instead of active_tab_index, to prevent viewport jumps when tabs are physically reordered via MoveTab (Alt-I/O).

## Success Criteria

### Measurable Outcomes

- **SC-001**: After pressing S in navigate mode, sessions are grouped in four tiers: Active (Working, Waiting) first, then Done (Done, AgentDone), then Idle (Idle, Init), then Paused.
- **SC-002**: Within each tier, sessions retain their prior relative ordering.
- **SC-003**: The cursor highlights the same session before and after the sort.
- **SC-004**: The sort activates instantly with no perceptible delay.
- **SC-005**: Pressing S when sort is already active re-computes the sort with current session states.
- **SC-006**: The sort remains stable across sidebar refreshes, timer-driven render updates, and tab count changes. Sort persists until explicitly cleared.
- **SC-007**: No Zellij tab reorder API calls are made during the sort operation.

## Error Handling

- If there are 0 or 1 sessions when S is pressed, the sort is a no-op with no error or visual feedback.
- If S is pressed outside navigate mode, the keybinding has no effect (key event is ignored).
- If a session has no `tab_index` (e.g., during startup before TabUpdate arrives), it is excluded from the sort computation.
- If the `sort_active` flag is set but a subsequent render finds no sessions, the sidebar renders an empty list as normal (sort flag is irrelevant with no data).

## Out of Scope

- Persistent sort preference (sort is one-shot, not remembered across Zellij restarts)
- Auto-re-sort when session states change while sort is active
- Sort by other criteria (name, last activity time)
- Physical tab reordering (removed due to Zellij API limitations)

## Assumptions

- The user is in navigate (amber) mode when pressing S. The keybinding has no effect in other modes.
- The controller has access to session activity state and tab index for all sessions.
- The sort is one-shot: each press of S produces a fresh frozen snapshot. There is no toggle-off; pressing S always re-sorts.
- The existing `sort_tier` classification function and `sort_cursor_pane_id` tracking mechanism from spec 067 remain valid and can be reused.
- The render broadcast pipeline already sorts sessions by `tab_index`; the change adds an alternative sort key when sort is active.
