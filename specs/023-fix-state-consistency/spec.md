# Feature Specification: Fix State Consistency and Add Refresh Command

**Feature Branch**: `001-fix-state-consistency`
**Created**: 2026-03-30
**Status**: Draft
**Input**: User description: "Fix three state consistency bugs across plugin instances and add a manual refresh command"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Session activity indicators stay consistent across all tabs (Priority: P1)

A user runs multiple Claude Code sessions across several tabs. When a session finishes its work, a green checkmark appears in the sidebar. After 30 seconds, the checkmark should disappear (transition to idle) on every tab's sidebar, not just the tab the user is currently viewing.

**Why this priority**: This is the most visible and frequently encountered issue. Stale green checkmarks create confusion about which sessions actually need attention, undermining the core value proposition of the sidebar as a reliable session status dashboard.

**Independent Test**: Start two Claude Code sessions on different tabs. Let one session finish (reaches Done state). Wait 30 seconds. Switch between tabs and verify the checkmark is gone on all tabs.

**Acceptance Scenarios**:

1. **Given** a session reaches Done state on any tab, **When** the done timeout (30 seconds) elapses, **Then** the session shows as idle on ALL sidebar instances across all tabs
2. **Given** the user is viewing tab B while tab A's session transitions from Done to Idle, **When** the user switches to tab B's sidebar, **Then** the idle indicator is already showing (no flash of stale green checkmark)
3. **Given** dead sessions are removed during startup grace cleanup, **When** the cleanup completes, **Then** the removal is reflected on all sidebar instances

---

### User Story 2 - No ghost state from previous Zellij sessions (Priority: P1)

A user ends a Zellij session (close terminal, crash, or detach-and-start-new). When they start a fresh Zellij session, the sidebar must not display session names, pause states, or other metadata from the previous session. Each new Zellij session starts with a clean slate.

**Why this priority**: Ghost state from previous sessions is deeply confusing. Users see session names and states that belong to a completely different work context, with no obvious way to clear them.

**Independent Test**: Start a Zellij session, create sessions, rename them, pause one. Kill the Zellij process. Start a new Zellij session. Verify the sidebar starts clean with no carryover from the previous session.

**Acceptance Scenarios**:

1. **Given** metadata files exist from a previous Zellij session (different process ID), **When** a new plugin instance loads, **Then** ALL cached metadata files are cleared before any state is restored
2. **Given** a fresh Zellij session, **When** the sidebar loads, **Then** no session names, pause states, or other metadata from the previous session appear
3. **Given** a user reattaches to the SAME Zellij session (same process ID), **When** the plugin loads, **Then** metadata is correctly preserved and restored (existing reattach behavior maintained)

---

### User Story 3 - Dead session metadata does not accumulate (Priority: P2)

When a user closes a tab or pane containing a Claude Code session, the session's metadata (custom name, pause state) should be cleaned up. Without cleanup, if the system reuses the same internal identifier for a new session, the old metadata could incorrectly apply to the new session.

**Why this priority**: Less frequent than P1 issues but causes subtle, hard-to-diagnose confusion in long-running sessions where identifiers get reused.

**Independent Test**: Create a session, rename it to "custom-name", close the tab. Create a new tab with a new session. Verify the new session does not inherit "custom-name".

**Acceptance Scenarios**:

1. **Given** a session with custom metadata is removed (tab/pane closed), **When** the removal is processed, **Then** the session's metadata entries are pruned from persistent storage
2. **Given** a long-running Zellij session where identifiers have been reused, **When** a new session is created, **Then** it does not inherit metadata from a previously closed session that happened to use the same identifier
3. **Given** metadata storage contains entries, **When** cleanup runs, **Then** only entries matching currently active sessions remain

---

### User Story 4 - User can force-refresh sidebar state (Priority: P2)

A user notices the sidebar shows incorrect or stale information. They can trigger a manual state refresh using a keyboard shortcut ("!" in navigation mode) or a command-line pipe command. The refresh clears all cached state, keeps the active sidebar's current view as the source of truth, and pushes that state to all other sidebar instances.

**Why this priority**: Provides a safety valve for any remaining edge cases and gives users direct control when state gets corrupted, without requiring a full session restart.

**Independent Test**: Manually corrupt the metadata file. Enter navigation mode, press "!". Verify the sidebar refreshes with clean, correct state. Alternatively, run the CLI pipe command and verify the same result.

**Acceptance Scenarios**:

1. **Given** the user is in navigation mode, **When** they press "!", **Then** all cached state files are cleared, the active sidebar's current state is broadcast to all instances, and a "State refreshed" notification appears
2. **Given** the user runs the CLI pipe command for refresh, **When** the command reaches the active sidebar instance, **Then** the same refresh logic executes as the keyboard shortcut
3. **Given** the user triggers a refresh, **When** the active sidebar broadcasts its state, **Then** all other sidebar instances replace their state with the broadcast data
4. **Given** the user is NOT in navigation mode, **When** "!" is pressed, **Then** nothing happens (the shortcut is only active in navigation mode)

---

### Edge Cases

- What happens when Done-to-Idle cleanup runs on the active tab but a broadcast arrives at another instance that has a newer timestamp for the same session? The merge logic (newest timestamp wins) resolves this correctly; the newer data takes precedence.
- What happens if two tabs both briefly appear "active" during a rapid tab switch? Only the instance matching the active tab position processes cleanup, preventing conflicts.
- What happens if the metadata file is written by one instance while another reads it? Partial reads result in invalid data that is handled gracefully (the reader ignores corrupt data and continues).
- What happens when refresh is triggered while a session is actively being updated? The active sidebar's in-memory state is authoritative, so ongoing updates continue normally after the refresh.
- What happens if the CLI pipe refresh command is received by a non-active sidebar instance? Only the active-tab instance processes the refresh; other instances ignore it (consistent with other active-tab-only actions).

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST propagate session activity state changes (Done/AgentDone to Idle transitions) to ALL sidebar instances across all tabs, not just persist them locally
- **FR-002**: System MUST propagate dead session removals (during post-startup cleanup) to ALL sidebar instances, not just persist them locally
- **FR-003**: System MUST detect and clear ALL cached state files (session data, metadata, process ID tracking) when loading in a new Zellij session that differs from the session that created the cache
- **FR-004**: System MUST prune metadata entries for sessions that no longer exist, after any session removal event (tab/pane close, dead session cleanup)
- **FR-005**: System MUST support a keyboard-triggered state refresh ("!" key) available only in navigation mode
- **FR-006**: System MUST support a CLI pipe-triggered state refresh command for programmatic/terminal access
- **FR-007**: System MUST display a brief notification ("State refreshed") after a successful refresh operation
- **FR-008**: System MUST only execute refresh logic on the active-tab sidebar instance (consistent with other active-tab-only actions)
- **FR-009**: Refresh MUST clear all file-based caches, keep the active instance's in-memory state as authoritative, and broadcast that state to all other instances

### Key Entities

- **Session state cache**: Persistent file storing full session data (activity, names, branches), with process-ID-based staleness detection
- **Session metadata**: Persistent file storing user-modified properties (custom names, pause states), currently lacking staleness detection
- **Process ID tracker**: File recording which Zellij session owns the cache, used to detect stale caches across sessions
- **Shared click state**: File used for cross-instance double-click detection

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Activity indicator transitions (Done/AgentDone to Idle) are visible on all tabs within one polling interval of the transition occurring
- **SC-002**: Zero stale session metadata from previous Zellij sessions appears after starting a new session
- **SC-003**: Metadata storage contains only entries for currently active sessions (no orphaned entries accumulate over time)
- **SC-004**: The refresh command clears all cached state and synchronizes all sidebar instances within one polling interval
- **SC-005**: All existing automated tests continue to pass after the changes
- **SC-006**: The refresh operation completes and shows a notification within 1 second of the user triggering it

## Assumptions

- The Zellij process ID remains stable across reattach operations but changes for new sessions (this is the existing assumption used by the session data cache and is validated by current behavior)
- All sidebar instances within a Zellij session share the same cache directory (by design of the plugin sandbox)
- The existing merge logic (newest timestamp wins) correctly resolves conflicts between instances and does not need modification
- File I/O on shared cache files is not atomic, but the system already handles corrupt reads gracefully by ignoring parse failures
- The "!" key does not conflict with any existing navigation mode keybinding
