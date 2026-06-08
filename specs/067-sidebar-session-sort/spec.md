# Feature Specification: Sidebar Session Sort by Activity

**Feature Branch**: `067-sidebar-session-sort`
**Created**: 2026-06-08
**Status**: Draft
**Input**: User description: "Sidebar session sort by activity via S keybinding in navigate mode"

## User Scenarios & Testing

### User Story 1 - Sort Sessions by Activity (Priority: P1)

A user has 8+ sessions open across multiple tabs. Some are actively working, some are idle, and two are paused. The active sessions are scattered at positions 2, 5, and 7 in the tab bar. The user enters navigate mode (Alt+s) and presses Shift+S. The sidebar re-renders with the three active sessions at the top, idle sessions in the middle, and paused sessions at the bottom. The underlying Zellij tabs physically move to match.

**Why this priority**: This is the core feature. Without it, there is nothing.

**Independent Test**: Can be fully tested by creating sessions in various activity states, pressing S in navigate mode, and verifying the tab order matches the expected tier grouping.

**Acceptance Scenarios**:

1. **Given** 5 sessions with mixed states (2 Working, 1 Idle, 1 Done, 1 Paused) in tab order [Idle, Working, Paused, Done, Working], **When** the user presses S in navigate mode, **Then** the tab order becomes [Working, Working, Idle, Done, Paused].
2. **Given** sessions are already sorted by activity tiers, **When** the user presses S, **Then** no tabs move (the sort is a no-op).
3. **Given** 3 Working sessions at tab positions 1, 4, 6, **When** the user presses S, **Then** the 3 Working sessions appear at positions 1, 2, 3 with their original relative order preserved.

---

### User Story 2 - Cursor Follows Current Session (Priority: P1)

The user is in navigate mode with the cursor on session "api-server". They press S to sort. After the sort completes, the cursor remains on "api-server" even though its tab position changed.

**Why this priority**: Losing the cursor position after sort would be disorienting and break the user's workflow.

**Independent Test**: Enter navigate mode, position cursor on a specific session, press S, verify cursor still highlights the same session.

**Acceptance Scenarios**:

1. **Given** the cursor is on session "api" at position 3, **When** the user presses S and "api" moves to position 1, **Then** the cursor is on position 1 (still highlighting "api").
2. **Given** the cursor is on a session that does not move, **When** the user presses S, **Then** the cursor position is unchanged.

---

### User Story 3 - Help Overlay Documents Sort (Priority: P2)

The user presses ? in navigate mode to see the help overlay. The help text includes the S keybinding with a description of the sort action.

**Why this priority**: Discoverability. Users need to find out the sort shortcut exists.

**Independent Test**: Enter navigate mode, press ?, verify S is listed in the help overlay.

**Acceptance Scenarios**:

1. **Given** the user is in navigate mode, **When** the user presses ?, **Then** the help overlay includes an entry for S describing the sort-by-activity action.

---

### Edge Cases

- What happens when there are 0 or 1 sessions? The sort is a no-op.
- What happens when all sessions are in the same tier? The sort is a no-op (relative order preserved).
- What happens if the sort is triggered while a session changes state mid-sort? The sort uses a snapshot of session states at the moment S is pressed.
- What happens if tabs cannot be moved (Zellij API failure)? The sort fails silently; the sidebar re-renders with whatever order resulted.

## Requirements

### Functional Requirements

- **FR-001**: The sidebar MUST add a new S (Shift+s) keybinding in navigate mode that triggers a sort-by-activity action.
- **FR-002**: The sort MUST group sessions into three tiers: Active (Working, Waiting) at top, Inactive (Idle, Done, AgentDone, Init) in middle, Paused (`session.paused == true`, regardless of activity state) at bottom.
- **FR-003**: The sort MUST be stable within each tier, preserving the relative tab order of sessions that belong to the same tier.
- **FR-004**: The sort MUST physically reorder Zellij tabs so that sidebar position, tab indices, and keyboard shortcuts all remain consistent.
- **FR-005**: The navigate mode cursor MUST follow the current session after the sort completes, updating its position to match the session's new index.
- **FR-006**: The sidebar MUST send a single Sort action to the controller, which computes the target order and executes the tab swap sequence.
- **FR-007**: The help overlay (? key) MUST document the S keybinding with a brief description.
- **FR-008**: The sort MUST be a no-op when sessions are already in the correct tier order.

## Success Criteria

### Measurable Outcomes

- **SC-001**: After pressing S in navigate mode, all Active sessions appear before all Inactive sessions, which appear before all Paused sessions.
- **SC-002**: Within each tier, sessions retain their prior relative ordering.
- **SC-003**: The cursor highlights the same session before and after the sort.
- **SC-004**: The sort completes without user-visible delay for up to 15 sessions.
- **SC-005**: Pressing S when sessions are already sorted produces no tab movement.

## Out of Scope

- Persistent sort preference (sort is one-shot, not remembered across sessions)
- Auto-re-sort when session states change
- Custom sort orders or user-defined tier assignments
- Sort by other criteria (name, last activity time)
- External documentation updates (handled by constitution enforcement in tasks)

## Assumptions

- The user is in navigate (amber) mode when pressing S. The keybinding has no effect in other modes.
- Zellij's `move_focus_or_tab(Direction)` API is available and supports moving the currently focused tab by one position in a given direction. The sort algorithm uses this to swap tabs into their target positions.
- The controller has access to session activity state and tab index for all sessions.
- The sort is a one-shot action; it does not persist or automatically re-sort when session states change.
