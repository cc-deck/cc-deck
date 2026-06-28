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

1. **Given** 5 sessions with mixed states (2 Working, 1 Idle, 1 Done, 1 Paused) in tab order [Idle, Working, Paused, Done, Working], **When** the user presses S in navigate mode, **Then** the sidebar displays [Working, Working, Idle, Done, Paused] and the Zellij tab bar order is unchanged.
2. **Given** sessions are already sorted by activity tiers, **When** the user presses S, **Then** the sidebar shows the same order (no visible change) and sort becomes active.
3. **Given** 3 Working sessions at tab positions 1, 4, 6, **When** the user presses S, **Then** the sidebar displays the 3 Working sessions at the top with their original relative order preserved.

---

### User Story 2 - Toggle Sort On and Off (Priority: P1)

The user has activated sort mode and now wants to return to the natural tab order. They press S again in navigate mode. The sidebar reverts to displaying sessions in their original tab index order.

**Why this priority**: Users need the ability to undo the sort to see sessions in their natural tab position order.

**Independent Test**: Activate sort with S, verify sorted order, press S again, verify original order is restored.

**Acceptance Scenarios**:

1. **Given** sort is not active, **When** the user presses S, **Then** sort becomes active and the sidebar displays sessions grouped by tier.
2. **Given** sort is active, **When** the user presses S, **Then** sort is deactivated and the sidebar displays sessions in their natural tab index order.

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

### User Story 5 - Sort Auto-Clears on Tab Change (Priority: P2)

When a new tab is opened or a tab is closed while sort is active, the sort automatically deactivates because the tab-to-session mapping has changed and the virtual order may be stale.

**Why this priority**: Stale sort order after tab changes could show incorrect groupings.

**Independent Test**: Activate sort, open a new session tab, verify sort deactivates and sidebar shows natural order.

**Acceptance Scenarios**:

1. **Given** sort is active with 5 sessions, **When** a new session tab is opened, **Then** sort deactivates and the sidebar shows sessions in natural tab order including the new session.
2. **Given** sort is active with 5 sessions, **When** a session tab is closed, **Then** sort deactivates and the sidebar shows remaining sessions in natural tab order.

---

### Edge Cases

- What happens when there are 0 or 1 sessions? The sort is a no-op; pressing S has no visible effect.
- What happens when all sessions are in the same tier? The sort activates but the display order matches the natural order (no visible change).
- What happens if a session changes activity state while sort is active? The next render re-applies the tier sort using the current session states, so the sidebar order may update while sort remains enabled.
- What happens if sort is pressed outside navigate mode? The keybinding has no effect; sort only works in navigate mode.

## Requirements

### Functional Requirements

- **FR-001**: The sidebar MUST retain the existing S (Shift+s) keybinding in navigate mode that triggers a sort-by-activity action.
- **FR-002**: The sort MUST group sessions into three tiers: Active (Working, Waiting) at top, Inactive (Idle, Done, AgentDone, Init) in middle, Paused (`session.paused == true`, regardless of activity state) at bottom.
- **FR-003**: The sort MUST be stable within each tier, preserving the relative tab order of sessions that belong to the same tier.
- **FR-004**: The sort MUST be display-only; it MUST NOT call any Zellij tab reorder APIs (`MoveTab`, `switch_tab_to` for sort purposes). Zellij tab positions MUST remain unchanged.
- **FR-005**: The navigate mode cursor MUST follow the current session after the sort completes, updating its display position to match the session's new index in the sorted view.
- **FR-006**: Pressing S while sort is active MUST deactivate the sort and revert the sidebar to natural tab index order (toggle behavior).
- **FR-007**: The sidebar MUST display a visual sort indicator when sort is active, so the user can distinguish sorted view from natural order.
- **FR-008**: The sort MUST auto-deactivate when the tab count changes (new tab opened or tab closed), reverting to natural tab order.
- **FR-009**: The `sort_active` flag MUST be maintained in the controller state (not the sidebar) so all sidebar instances reflect the same sort state. Transient cursor-tracking state (`sort_cursor_pane_id`) MAY remain in the sidebar, as it is consumed locally on each render cycle.
- **FR-010**: The help overlay (? key) MUST continue to document the S keybinding.

## Success Criteria

### Measurable Outcomes

- **SC-001**: After pressing S in navigate mode, all Active sessions appear before all Inactive sessions, which appear before all Paused sessions in the sidebar display.
- **SC-002**: Within each tier, sessions retain their prior relative ordering.
- **SC-003**: The cursor highlights the same session before and after the sort.
- **SC-004**: The sort activates instantly with no perceptible delay.
- **SC-005**: Pressing S when sort is already active reverts the sidebar to natural tab order.
- **SC-006**: The sort remains stable across sidebar refreshes and timer-driven render updates until explicitly toggled off or tab count changes.
- **SC-007**: No Zellij tab reorder API calls are made during the sort operation.

## Error Handling

- If there are 0 or 1 sessions when S is pressed, the sort is a no-op with no error or visual feedback.
- If S is pressed outside navigate mode, the keybinding has no effect (key event is ignored).
- If a session has no `tab_index` (e.g., during startup before TabUpdate arrives), it is excluded from the sort computation.
- If the `sort_active` flag is set but a subsequent render finds no sessions, the sidebar renders an empty list as normal (sort flag is irrelevant with no data).

## Out of Scope

- Persistent sort preference (sort is one-shot, not remembered across sessions)
- Auto-re-sort when session states change while sort is active
- Custom sort orders or user-defined tier assignments
- Sort by other criteria (name, last activity time)
- Physical tab reordering (removed due to Zellij API limitations)

## Assumptions

- The user is in navigate (amber) mode when pressing S. The keybinding has no effect in other modes.
- The controller has access to session activity state and tab index for all sessions.
- The sort is a toggle: first press activates, second press deactivates. There is no persistent sort preference.
- The existing `sort_tier` classification function and `sort_cursor_pane_id` tracking mechanism from spec 067 remain valid and can be reused.
- The render broadcast pipeline already sorts sessions by `tab_index`; the change adds an alternative sort key when sort is active.
