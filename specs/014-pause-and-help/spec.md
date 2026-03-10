# Feature Specification: Session Pause Mode & Keyboard Help

**Feature Branch**: `014-pause-and-help`
**Created**: 2026-03-10
**Status**: Draft
**Input**: Session pause mode to exclude sessions from attend cycling, and keyboard help overlay showing all shortcuts.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Pause/Unpause Sessions (Priority: P1)

A user managing multiple Claude sessions intentionally parks some for later (e.g., waiting for a long build, or set aside while focusing on other work). They press `p` in navigation mode to toggle pause on the cursor session. Paused sessions display a pause icon with a greyed-out name and are excluded from `Alt+a` attend cycling. The session remains navigable via click, Enter, or cursor selection.

**Why this priority**: This is the core feature. Without pause, attend cycles through all sessions including ones the user wants to ignore, making attend less useful as session count grows.

**Independent Test**: Start 3 Claude sessions. Pause one with `p`. Press `Alt+a` repeatedly and verify the paused session is never selected. Unpause with `p` and verify it re-enters the cycling.

**Acceptance Scenarios**:

1. **Given** navigation mode is active with cursor on an unpaused session, **When** the user presses `p`, **Then** the session shows a pause icon (⏸) and the name becomes dimmed/grey.

2. **Given** navigation mode is active with cursor on a paused session, **When** the user presses `p`, **Then** the session returns to its normal activity icon and name styling.

3. **Given** a session is paused, **When** the user presses `Alt+a`, **Then** the paused session is skipped in the attend cycling order.

4. **Given** all non-working sessions are paused, **When** the user presses `Alt+a`, **Then** a notification "No sessions available" is displayed.

5. **Given** a session is paused, **When** the user clicks on it or selects it with Enter, **Then** focus switches to it normally (pause does not prevent manual navigation).

6. **Given** a session is paused and another sidebar instance syncs state, **Then** the pause flag is preserved across instances.

7. **Given** a paused session receives a PermissionRequest hook event, **Then** it remains paused (pause is intentional, not overridden by activity changes).

---

### User Story 2 - Keyboard Help Overlay (Priority: P2)

A user in navigation mode presses `?` to see a help overlay listing all available keyboard shortcuts. The overlay replaces the session list temporarily. Pressing any key dismisses the overlay and returns to the normal session list.

**Why this priority**: Discoverability is important as the shortcut set grows, but users can learn shortcuts from documentation. This is a convenience feature.

**Independent Test**: Enter navigation mode, press `?`. Verify the help screen appears with all shortcuts listed. Press any key and verify the session list returns.

**Acceptance Scenarios**:

1. **Given** navigation mode is active, **When** the user presses `?`, **Then** the sidebar displays a help screen listing all keyboard shortcuts.

2. **Given** the help screen is displayed, **When** the user presses any key, **Then** the help screen is dismissed and the session list returns.

3. **Given** the help screen is displayed, **When** the user presses `Esc`, **Then** the help screen is dismissed (does not exit navigation mode).

4. **Given** the sidebar is in passive mode (not navigation), **When** the user presses `?`, **Then** nothing happens (help only available in navigation mode).

---

### Edge Cases

- What happens when all sessions are paused and the user presses `Alt+a`? Show "No sessions available" notification.
- What happens when a session is paused while the cursor is on it and `d` (delete) is pressed? Delete still works, pause does not prevent deletion.
- What happens when the help overlay is shown and `Alt+a` is pressed? The attend action executes (help is dismissed, attend runs).
- What happens when the sidebar is too small to show all help content? Truncate the help text to fit available rows.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The `p` key MUST toggle a pause flag on the cursor session in navigation mode.
- **FR-002**: Paused sessions MUST display a pause icon (⏸) replacing the activity icon.
- **FR-003**: Paused sessions MUST display their name in dimmed/grey text.
- **FR-004**: Paused sessions MUST be excluded from the attend cycling mechanism (`Alt+a`).
- **FR-005**: Paused sessions MUST remain navigable via click, cursor selection (Enter), and tab switching.
- **FR-006**: The pause flag MUST persist across sidebar instance synchronization.
- **FR-007**: The pause flag MUST NOT be automatically cleared by activity changes (hooks).
- **FR-008**: The `?` key MUST display a help overlay listing all keyboard shortcuts when in navigation mode.
- **FR-009**: The help overlay MUST be dismissed by pressing any key.
- **FR-010**: The help overlay MUST include shortcuts for navigation (j/k, Enter, Esc), actions (r, d, p, n, /), and global shortcuts (Alt+s, Alt+a).
- **FR-011**: When all non-working and non-paused sessions are exhausted by attend, a "No sessions available" notification MUST be shown.

### Key Entities

- **Pause Flag**: A per-session boolean that excludes the session from automatic attend cycling while preserving all other interactions.
- **Help Overlay**: A temporary screen replacing the session list that shows all keyboard shortcuts grouped by category.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users can exclude specific sessions from attend cycling with a single keystroke (`p`).
- **SC-002**: Paused sessions are visually distinguishable from active sessions at a glance.
- **SC-003**: All keyboard shortcuts are discoverable within the sidebar via the `?` key.
- **SC-004**: Pause state survives sidebar re-renders and cross-instance synchronization.

## Assumptions

- The pause icon ⏸ is widely supported in terminal fonts used with Zellij.
- The help overlay content fits within 15 lines, sufficient for most sidebar heights.
- Pause is a user-level concept separate from session activity state. A paused session can still be Working, Idle, etc. internally.
