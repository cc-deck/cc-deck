# Feature Specification: Plugin Bugfixes

**Feature Branch**: `010-plugin-bugfixes`
**Created**: 2026-03-05
**Status**: Draft
**Input**: User description: "plugin bugfixes"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Dynamic Tab Titles (Priority: P1)

A user creates multiple Claude Code sessions via the plugin. Each session's tab title in the Zellij tab bar reflects the project name and current activity status (working, waiting, idle, done). When Claude starts working on a task, the tab title updates to show the activity. When the user renames a session, the tab title updates to match.

**Why this priority**: Lowest risk fix with immediate visual payoff. Makes multi-session workflows usable by showing what each tab is doing at a glance.

**Independent Test**: Create two sessions, trigger status changes, and verify tab titles update in the tab bar.

**Acceptance Scenarios**:

1. **Given** a session is created, **When** git detection completes, **Then** the tab title updates from the default name to the project name prefixed with a status indicator.
2. **Given** a session receives a "working" status hook, **When** the status changes, **Then** the tab title updates to show the working indicator alongside the project name.
3. **Given** a session receives a "waiting" status hook, **When** the status changes, **Then** the tab title updates to show the waiting indicator.
4. **Given** a session transitions to idle after the timeout, **When** the idle timeout fires, **Then** the tab title updates to show the idle indicator.
5. **Given** the user renames a session via the plugin, **When** the rename is confirmed, **Then** the tab title updates to reflect the new name.

---

### User Story 2 - Automatic Session Detection (Priority: P1)

A user starts Claude Code manually in a terminal pane (not via the plugin's new session command). The plugin detects that Claude is running by observing pane title changes and automatically registers the pane as a tracked session. The session appears in the status bar and responds to status hooks just like plugin-created sessions.

**Why this priority**: Critical for usability. Users often start Claude manually or have existing sessions from before the plugin was installed. Without this, the plugin appears broken for the most common workflow.

**Independent Test**: Start `claude` manually in a Zellij pane, verify it appears in the plugin's status bar without using any plugin commands.

**Acceptance Scenarios**:

1. **Given** the plugin is loaded, **When** the user starts `claude` in any terminal pane, **Then** the plugin detects the session within 5 seconds and adds it to the tracked sessions list.
2. **Given** a manually started Claude session is detected, **When** the user views the status bar, **Then** the session appears with the correct project name (from git detection) and status indicator.
3. **Given** a manually started Claude session is detected, **When** Claude sends status hook messages, **Then** the session status updates just like plugin-created sessions.
4. **Given** a detected session's pane closes, **When** the pane closes, **Then** the session is removed from the tracked list.
5. **Given** a session is already tracked by the plugin, **When** the pane title changes to include "claude", **Then** the plugin does not create a duplicate session entry.

---

### User Story 3 - Auto-Start Claude in New Sessions (Priority: P2)

A user creates a new session via the plugin. Claude Code starts automatically in the new tab without requiring the user to type `claude` manually. If Claude is not available on the system, the user sees a clear error message and gets a regular shell instead.

**Why this priority**: Important for the "one command to start a session" experience, but users can work around it by typing `claude` manually (which US2 will detect).

**Independent Test**: Create a session via the plugin, verify Claude starts automatically in the new tab.

**Acceptance Scenarios**:

1. **Given** Claude is installed on the system, **When** the user creates a new session, **Then** Claude starts automatically in the new tab.
2. **Given** Claude is not installed on the system, **When** the user creates a new session, **Then** a regular shell opens and an error message appears in the plugin status bar.
3. **Given** Claude starts automatically, **When** the session is tracked, **Then** it behaves identically to a manually started session (status hooks, naming, tab title updates).

---

### User Story 4 - Session Picker as Floating Overlay (Priority: P2)

A user opens the session picker to switch between sessions. The picker appears as a floating overlay that covers part of the terminal without displacing content. The user can type to filter sessions, use arrow keys to navigate, and press Enter to switch. Pressing Escape dismisses the picker and returns focus to the previous pane.

**Why this priority**: The picker is unusable in the current single-row status bar pane. This fix is needed for session switching to work, but users can use tab switching as a workaround.

**Independent Test**: Open the picker, verify it appears as a floating overlay, select a session, verify focus switches.

**Acceptance Scenarios**:

1. **Given** multiple sessions exist, **When** the user triggers the picker command, **Then** a floating overlay appears listing all sessions with their status indicators and project names.
2. **Given** the picker is open, **When** the user types text, **Then** the session list filters in real time using fuzzy matching.
3. **Given** the picker is open, **When** the user presses Enter on a session, **Then** focus switches to that session's tab and the picker closes.
4. **Given** the picker is open, **When** the user presses Escape, **Then** the picker closes and focus returns to the previously focused pane.
5. **Given** only one session exists, **When** the user triggers the picker, **Then** the picker still opens (allowing the user to see the session list) rather than silently doing nothing.

---

### User Story 5 - Automated Plugin Test Suite (Priority: P3)

A developer runs an automated test script that exercises all plugin features without manual interaction. The test creates a Zellij session, sends commands, verifies state, and reports pass/fail results. The test can run in CI environments.

**Why this priority**: Important for preventing regressions but not blocking for end users. The other fixes must work before tests can validate them.

**Independent Test**: Run the test script, verify it produces a pass/fail report without manual intervention.

**Acceptance Scenarios**:

1. **Given** the plugin is installed and Zellij is available, **When** the test script runs, **Then** it creates a test session, exercises all plugin commands, and reports results.
2. **Given** all plugin features work correctly, **When** the test suite completes, **Then** all tests pass and the test session is cleaned up.
3. **Given** a plugin feature is broken, **When** the test for that feature runs, **Then** the test fails with a descriptive error message indicating what broke.
4. **Given** the test suite runs in a CI environment, **When** no display is available, **Then** the tests still execute (headless mode) or skip gracefully with a clear message.

---

### Edge Cases

- **Pane title detection timing**: Claude may not set the pane title immediately on startup. The plugin must handle delayed title changes without missing the detection window.
- **Multiple Claude instances in one pane**: If a user exits Claude and restarts it in the same pane, the plugin should not create a duplicate session.
- **Tab index shifting**: When tabs are closed or reordered, the plugin must update its tab-to-session mapping to keep tab titles accurate.
- **Claude not on PATH**: When Claude is unavailable, auto-start should fail gracefully with an actionable error, not crash the plugin or leave orphaned tabs.
- **Rapid session creation**: Creating multiple sessions in quick succession should not cause race conditions in session registration or tab naming.
- **Floating picker dismissed by external action**: If the user closes the floating picker pane via Zellij's built-in close command instead of Escape, the plugin should detect this and reset its picker state.

## Requirements *(mandatory)*

### Functional Requirements

**Tab Titles:**
- **FR-001**: System MUST update tab titles when session status changes (working, waiting, done, idle, exited, unknown).
- **FR-002**: System MUST update tab titles when session display name changes (git detection, manual rename).
- **FR-003**: Tab title format MUST include a status indicator and the session display name.
- **FR-004**: System MUST maintain a mapping between sessions and their tab indices, updated on every tab state change.

**Session Detection:**
- **FR-005**: System MUST detect when a terminal pane's title contains "claude" (case-insensitive substring match) and auto-register it as a tracked session. Detection MUST check pane titles on every PaneUpdate event and on periodic timer intervals (every 5 seconds).
- **FR-006**: System MUST not create duplicate sessions for panes already tracked.
- **FR-007**: Detected sessions MUST behave identically to plugin-created sessions (status updates, naming, tab title management).
- **FR-008**: System MUST perform git repo detection on auto-detected sessions to determine the project name.
- **FR-009**: Detection MUST handle delayed pane title changes (title not set immediately on Claude startup).

**Auto-Start:**
- **FR-010**: System MUST launch Claude automatically when a new session is created via the plugin.
- **FR-011**: System MUST display an actionable error in the status bar if Claude cannot be started (not found on PATH).
- **FR-012**: System MUST fall back to a regular shell if Claude fails to start, preserving the tab for the user.
- **FR-012a**: Auto-start error messages MUST be displayed in the status bar for at least 10 seconds, then auto-dismiss. The error MUST include guidance on how to install Claude.

**Floating Picker:**
- **FR-013**: System MUST display the session picker as a floating overlay when triggered. If floating overlays are not supported by the runtime, the system MUST temporarily expand the plugin pane to display the picker, then shrink back on dismissal.
- **FR-014**: The floating picker MUST support type-ahead fuzzy filtering of sessions.
- **FR-015**: The floating picker MUST close and return focus to the previous pane on selection or dismissal.
- **FR-016**: System MUST handle the case where the floating picker is closed externally (not via Escape) by resetting picker state.

**Automated Tests:**
- **FR-017**: System MUST provide an automated test script that exercises session creation, switching, detection, status updates, and removal.
- **FR-018**: The test script MUST clean up all test resources (sessions, tabs) after completion.
- **FR-019**: The test script MUST report clear pass/fail results for each tested feature.

### Key Entities

- **Session**: A tracked Claude Code instance running in a Zellij pane. Has a display name, status, pane ID, tab index, and activity timestamps.
- **Tab Mapping**: The association between a session and its Zellij tab index. Must be kept current as tabs are created, closed, or reordered.
- **Floating Picker**: A temporary overlay pane that displays the session list for switching. Has a search query, selected index, and filtered results.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Tab titles update within 1 second of a status change, reflecting the current session status and project name.
- **SC-002**: Manually started Claude sessions are detected and tracked within 5 seconds of launch.
- **SC-003**: Creating a new session via the plugin results in Claude running automatically within 3 seconds.
- **SC-004**: The session picker renders as a visible overlay (not a 1-pixel bar) and allows session selection in under 2 seconds.
- **SC-005**: The automated test suite completes in under 60 seconds and produces a pass/fail report for each feature.
- **SC-006**: All 5 user stories can be demonstrated working together: create a session (auto-starts Claude), see tab title update with status, open picker to switch, start Claude manually in another tab (auto-detected), see both sessions in the picker and status bar.

## Assumptions

- Zellij 0.40+ is installed (matching the plugin SDK version).
- Claude Code sets the terminal title to a string containing "claude" when running (e.g., "claude" or "Claude Code"). Detection uses case-insensitive substring matching.
- The Zellij plugin API provides a way to create floating plugin panes or floating terminal panes for the picker overlay.
- Tab indices are available from `TabUpdate` events and can be used with `rename_tab()`.
- The `open_command_pane` or equivalent API can launch Claude in a newly created tab.

## Scope Boundaries

**In scope**:
- Dynamic tab title updates with status and project name
- Auto-detection of manually started Claude sessions via pane title
- Auto-starting Claude when creating sessions via the plugin
- Session picker as floating overlay
- Automated integration test script

**Out of scope**:
- Plugin lifecycle management (install/status/remove, covered by 009-plugin-lifecycle)
- Clipboard integration (covered by brainstorm 05)
- Session flavors/templates (covered by brainstorm 04)
- Remote/K8s session management
- Keybinding configuration changes (Alt+Shift bindings are already set)
