# Feature Specification: Keyboard Navigation & Global Shortcuts

**Feature Branch**: `013-keyboard-navigation`
**Created**: 2026-03-09
**Status**: Draft
**Input**: Keyboard-driven session navigation in the cc-deck sidebar with global shortcuts, smart attend cycling, search/filter, and contextual actions (rename, delete).

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Sidebar Keyboard Navigation (Priority: P1)

A user working in a terminal pane presses a global shortcut to activate the sidebar for keyboard-driven navigation. The sidebar enters "navigation mode" where a visible cursor (large triangle `▶` prefix) appears on a session entry. The user moves the cursor with `j`/`k` or arrow keys and presses `Enter` to switch to that session's tab and pane. Pressing `Esc` exits navigation mode and returns focus to the terminal.

**Why this priority**: This is the core feature. Without keyboard navigation, users must reach for the mouse to switch sessions, breaking their flow. This single story delivers the primary value of hands-free session management.

**Independent Test**: Can be tested by starting Zellij with the cc-deck layout, creating two or more Claude sessions, pressing the navigate shortcut, using arrow keys to move between entries, and pressing Enter to confirm. Verify that the correct tab and pane receive focus.

**Acceptance Scenarios**:

1. **Given** the sidebar is in passive mode and multiple sessions exist, **When** the user presses the navigate shortcut (`Alt+s`), **Then** the sidebar enters navigation mode with a `▶` cursor on the currently active session.

2. **Given** the sidebar is in navigation mode, **When** the user presses `j` or `↓`, **Then** the cursor moves to the next session in the list.

3. **Given** the sidebar is in navigation mode, **When** the user presses `k` or `↑`, **Then** the cursor moves to the previous session in the list.

4. **Given** the sidebar is in navigation mode with cursor on a session, **When** the user presses `Enter`, **Then** focus switches to that session's tab and pane, and navigation mode exits.

5. **Given** the sidebar is in navigation mode, **When** the user presses `Esc`, **Then** navigation mode exits and focus returns to the previously active terminal pane.

6. **Given** the sidebar is in navigation mode with cursor at the first session, **When** the user presses `k` or `↑`, **Then** the cursor wraps to the last session.

7. **Given** the sidebar is in navigation mode, **When** the user presses `g` or `Home`, **Then** the cursor jumps to the first session.

8. **Given** the sidebar is in navigation mode, **When** the user presses `G` or `End`, **Then** the cursor jumps to the last session.

---

### User Story 2 - Global Shortcut Registration (Priority: P1)

When the plugin loads, it registers global keybindings with Zellij so that the navigate and attend shortcuts work from any mode (except locked). The keybindings are configurable through the plugin configuration in the layout file.

**Why this priority**: Without registered global shortcuts, there is no way to trigger navigation mode or attend from the keyboard. This is a prerequisite for all keyboard-driven features.

**Independent Test**: Start Zellij with cc-deck layout, press `Alt+s` from any tab. Verify the sidebar receives the pipe message and enters navigation mode. Press `Alt+a` and verify the attend action triggers.

**Acceptance Scenarios**:

1. **Given** the plugin has loaded and permissions are granted, **When** the user presses `Alt+s` from any tab in any Zellij mode except locked, **Then** the active tab's sidebar instance receives a navigation pipe message.

2. **Given** the plugin configuration specifies custom shortcut keys, **When** the plugin loads, **Then** the custom keys are registered instead of the defaults.

3. **Given** the user has custom Zellij keybindings that conflict with the defaults, **When** the plugin loads, **Then** the plugin's keybindings do not override user-configured bindings (user config takes precedence).

4. **Given** multiple sidebar instances exist (one per tab), **When** the navigate shortcut is pressed, **Then** only the sidebar instance on the active tab enters navigation mode.

---

### User Story 3 - Smart Attend (Priority: P2)

A user presses a global shortcut (`Alt+a`) to jump directly to the next session that needs attention. The algorithm prioritizes sessions that are blocking on user input (PermissionRequest) over informational notifications, and cycles through idle sessions in tab order (top-to-bottom). The currently focused session is skipped.

**Why this priority**: Smart attend provides a one-keystroke workflow for managing multiple Claude sessions. It's the most common keyboard action but depends on the global shortcut registration (US2).

**Independent Test**: Start multiple Claude sessions in different states (one waiting for permission, one idle, one working). Press `Alt+a` repeatedly and verify the correct cycling order.

**Acceptance Scenarios**:

1. **Given** one session is waiting for a PermissionRequest and another for a Notification, **When** the user presses attend, **Then** the PermissionRequest session is focused first.

2. **Given** multiple sessions are waiting for PermissionRequest, **When** the user presses attend, **Then** the oldest waiting session is focused first.

3. **Given** no sessions are waiting but some are idle, **When** the user presses attend, **Then** the idle session in tab order (top-to-bottom) is focused.

4. **Given** all sessions are actively working, **When** the user presses attend, **Then** a notification "All sessions busy" is displayed.

5. **Given** the currently focused session needs attention, **When** the user presses attend, **Then** it is skipped and the next session in priority order is focused.

6. **Given** only one session exists and it needs attention, **When** the user presses attend, **Then** that session is focused (no skip for single session).

---

### User Story 4 - Contextual Actions: Rename and Delete (Priority: P2)

While in navigation mode, the user can press `r` to rename the cursor session inline or `d` to delete it with a confirmation prompt. These actions operate on the cursor session, which may differ from the active session.

**Why this priority**: Contextual actions complete the keyboard workflow but are used less frequently than basic navigation and attend.

**Independent Test**: Enter navigation mode, move cursor to a session, press `r`, type a new name, press Enter. Verify the session is renamed. Repeat with `d`, press `y` to confirm deletion.

**Acceptance Scenarios**:

1. **Given** navigation mode is active with cursor on a session, **When** the user presses `r`, **Then** inline rename starts for the cursor session (not necessarily the active session).

2. **Given** rename is active, **When** the user types a name and presses Enter, **Then** the session is renamed and navigation mode resumes.

3. **Given** rename is active, **When** the user presses Esc, **Then** the rename is cancelled and navigation mode resumes.

4. **Given** navigation mode is active with cursor on a session, **When** the user presses `d`, **Then** an inline confirmation `Delete "session-name"? [y/N]` appears.

5. **Given** delete confirmation is showing, **When** the user presses `y`, **Then** the session's command pane is closed (and its tab if it was the only session), and the cursor moves to the next session.

6. **Given** delete confirmation is showing, **When** the user presses any key other than `y`, **Then** the deletion is cancelled and navigation mode resumes.

---

### User Story 5 - Search/Filter Sessions (Priority: P3)

While in navigation mode, the user presses `/` to enter a search sub-mode. A text input appears at the bottom of the sidebar. As the user types, the session list is filtered by display name (case-insensitive substring match). Pressing Enter confirms the filter, Esc clears it.

**Why this priority**: Search is valuable when managing many sessions but not essential for the core keyboard workflow. Most users will have fewer than 10 sessions.

**Independent Test**: Create 5+ sessions with distinct names. Enter navigation mode, press `/`, type a partial name. Verify the list filters in real-time. Press Enter and verify cursor lands on the first match.

**Acceptance Scenarios**:

1. **Given** navigation mode is active, **When** the user presses `/`, **Then** a search input appears at the bottom of the sidebar.

2. **Given** search mode is active and the user types "deck", **When** there are sessions named "cc-deck" and "other-project", **Then** only "cc-deck" is visible in the filtered list.

3. **Given** search mode is active with a filter, **When** the user presses Enter, **Then** the filter is applied and the cursor moves to the first matching session.

4. **Given** search mode is active, **When** the user presses Esc, **Then** the filter is cleared, all sessions are shown, and navigation mode resumes.

5. **Given** search mode is active and the filter matches no sessions, **When** the user presses Enter, **Then** a brief "No matches" notification appears and the filter is cleared.

---

### User Story 6 - New Session from Navigation Mode (Priority: P3)

While in navigation mode, the user presses `n` to create a new session. This behaves identically to clicking the [+] button: a new tab is created with claude auto-started.

**Why this priority**: Convenience shortcut that complements the existing [+] button. Low priority since the button already works.

**Independent Test**: Enter navigation mode, press `n`. Verify a new tab is created with claude running.

**Acceptance Scenarios**:

1. **Given** navigation mode is active, **When** the user presses `n`, **Then** a new tab is created with claude auto-started, and navigation mode exits.

---

### Edge Cases

- What happens when the navigate shortcut is pressed with zero sessions? Navigation mode activates with only the [+]/`n` option available.
- What happens when a session is deleted while the cursor is on the last item? The cursor moves up to the new last session.
- What happens when a new session appears (via hook) while in navigation mode? The session list updates and the cursor position is preserved by tracking the pane_id under the cursor.
- What happens when the navigate shortcut is pressed while already in navigation mode? It acts as a toggle and exits navigation mode.
- What happens when the attend shortcut is pressed while in navigation mode? Attend executes (jumps to the target session) and navigation mode exits.
- What happens when the user switches tabs while in navigation mode? Navigation mode auto-exits (detected via TabUpdate).
- What happens when the user clicks a terminal pane while in navigation mode? Navigation mode auto-exits (detected via PaneUpdate focus change).
- What happens when the user clicks the sidebar header? Navigation mode toggles on/off.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The sidebar MUST support two distinct modes: passive (default, mouse-only) and navigation (keyboard-driven).
- **FR-002**: A configurable global shortcut (default `Alt+s`) MUST activate navigation mode on the active tab's sidebar instance.
- **FR-003**: Navigation mode MUST display the cursor session with an amber tint background (`50,40,20` bg / `230,200,140` fg), visually distinct from the active session's teal highlight.
- **FR-004**: The `j`/`↓` and `k`/`↑` keys MUST move the cursor down and up through the session list in navigation mode.
- **FR-005**: The `Enter` key MUST switch focus to the cursor session's tab and pane, then exit navigation mode.
- **FR-006**: The `Esc` key MUST exit navigation mode and return focus to the previously active terminal pane.
- **FR-007**: The `r` key MUST start inline rename for the cursor session in navigation mode.
- **FR-008**: The `d` key MUST show an inline delete confirmation for the cursor session.
- **FR-009**: Delete confirmation MUST require `y` to proceed; any other key cancels.
- **FR-010**: The `/` key MUST enter search/filter sub-mode with a text input in navigation mode.
- **FR-011**: Search filtering MUST be case-insensitive substring matching on session display names.
- **FR-012**: The `n` key MUST create a new session (identical to the [+] button action) in navigation mode.
- **FR-013**: The `g`/`Home` and `G`/`End` keys MUST jump to the first and last session respectively.
- **FR-014**: A configurable global shortcut (default `Alt+a`) MUST trigger the smart attend action from any Zellij mode except locked.
- **FR-015**: Smart attend MUST prioritize sessions in this order: (1) PermissionRequest waiting (oldest first), (2) Notification waiting (oldest first), (3) idle/done sessions (tab order, top-to-bottom), (4) skip working sessions.
- **FR-016**: Smart attend MUST skip the currently focused session when cycling, unless it is the only session.
- **FR-017**: Global shortcuts MUST be registered dynamically at plugin load without modifying the user's config.kdl file.
- **FR-018**: Global shortcuts MUST NOT override user-configured Zellij keybindings.
- **FR-019**: Only plugin_id 0 MUST handle attend messages to preserve round-robin state. Both navigate and attend messages are forwarded from non-active instances to the correct handler via broadcast: navigate forwards to the active-tab instance, attend forwards to plugin_id 0. This is necessary because keybindings are registered by the last-loaded plugin instance, which may not be plugin_id 0 or on the active tab.
- **FR-020**: The cursor position MUST persist when exiting and re-entering navigation mode within the same Zellij session.
- **FR-021**: Navigation mode MUST auto-exit when the user switches to a different tab (detected via TabUpdate when the plugin is no longer on the active tab).
- **FR-022**: Navigation mode MUST auto-exit when a terminal pane gains focus (detected via PaneUpdate, indicating the user clicked away from the sidebar). The first PaneUpdate after entering navigation mode MUST be ignored because it arrives with stale focus state before `focus_plugin_pane` takes effect.
- **FR-023**: Clicking the sidebar header (row 0) MUST toggle navigation mode on and off.
- **FR-024**: The `Enter` key MUST switch to the selected session without restoring the previously focused pane (the user is intentionally navigating to a new target, not returning to their original position).
- **FR-025**: Broadcast messages between plugin instances MUST NOT use URL-based filtering (`plugin_url`). Zellij's `pipe_message_to_plugin` matches by both URL and configuration, and since running instances have varying config (mode, keys), URL filtering causes spurious floating pane creation instead of routing to existing instances.

### Key Entities

- **Navigation Mode**: A sidebar interaction state where the plugin is selectable and processes keyboard events for session list traversal and actions.
- **Cursor**: A visual pre-selection marker shown as an amber tint background, indicating which session will receive the next action. Distinct from the active session's teal highlight.
- **Smart Attend**: An enhanced session cycling algorithm that uses priority tiers (critical attention, soft attention, idle, working) to determine the next session to focus.
- **Wait Reason**: A distinction between PermissionRequest (blocking, critical) and Notification (informational, soft) waiting states, used by the smart attend algorithm.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users can switch between sessions using only the keyboard within 3 keystrokes (shortcut + optional cursor move + Enter).
- **SC-002**: The smart attend action focuses the highest-priority session within 1 keystroke from any tab.
- **SC-003**: Session rename and delete operations complete entirely through keyboard input without requiring mouse interaction.
- **SC-004**: Search/filter reduces the visible session list as the user types, allowing quick location of sessions by name.
- **SC-005**: Global shortcuts work consistently from all Zellij modes except locked, across all tabs.
- **SC-006**: Existing mouse-based interactions continue to work unchanged when not in navigation mode.

## Assumptions

- `Alt+s` and `Alt+a` do not conflict with commonly used terminal programs (vim, emacs, shell readline). Alt-key combinations are generally safe since most tools use Ctrl-based shortcuts.
- The plugin API (`rebind_keys()` or `reconfigure()`) can register keybindings that send pipe messages to the plugin.
- Claude Code hook events provide enough information to distinguish PermissionRequest from Notification waiting states via the `hook_event_name` field values.
- Cursor wrapping at list boundaries (top wraps to bottom, bottom wraps to top) is the expected behavior.
- The navigate shortcut acts as a toggle: pressing it while in navigation mode exits to passive mode.
