# Feature Specification: Single-Instance Architecture

**Feature Branch**: `030-single-instance-arch`
**Created**: 2026-03-31
**Status**: Draft
**Input**: Brainstorm document: `brainstorm/030-single-instance-architecture.md`

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Responsive Sidebar at Scale (Priority: P1)

A developer runs 10 or more concurrent Claude Code sessions across multiple tabs. The sidebar displays all sessions with their current activity status. When the user clicks a session or navigates with keyboard shortcuts, the response is immediate regardless of how many tabs are open.

**Why this priority**: This is the core problem the feature solves. The current architecture degrades noticeably at 10+ tabs because every event triggers processing in every tab instance. Users with many active sessions experience sluggish mouse clicks and erratic keyboard navigation, which directly undermines the tool's usability.

**Independent Test**: Open 15+ tabs with active Claude Code sessions and verify that mouse clicks on sidebar sessions respond within a perceptible instant, and keyboard navigation (j/k, Enter, Esc) feels smooth with no input lag.

**Acceptance Scenarios**:

1. **Given** 15 open tabs with active sessions, **When** the user clicks a session in the sidebar, **Then** the tab switch completes without perceptible delay (comparable to a setup with 2 tabs)
2. **Given** 15 open tabs, **When** the user presses navigation keys (j/k) rapidly, **Then** the cursor moves smoothly with each keypress reflected immediately
3. **Given** 20 open tabs, **When** a Claude Code session changes activity state (idle to working), **Then** the sidebar updates within 1 second across all tabs

---

### User Story 2 - Consistent Session State Across Tabs (Priority: P1)

A developer switches between tabs while working with multiple Claude Code sessions. The sidebar in each tab shows the same session list, the same activity indicators, and the same notification state. There is a single source of truth for session data, so no tab ever shows stale or conflicting information.

**Why this priority**: The current multi-instance architecture requires complex synchronization between independent copies of session state. This leads to race conditions, stale metadata, and merge conflicts. A single authoritative state eliminates these consistency bugs entirely.

**Independent Test**: Open 5 tabs, trigger a session rename in one tab, and verify the rename appears in all other tabs on the next sidebar refresh without requiring manual intervention.

**Acceptance Scenarios**:

1. **Given** sessions visible in multiple tabs, **When** a session is renamed in one tab, **Then** all other tabs reflect the new name on their next render update
2. **Given** a new Claude Code session starts, **When** the controller detects it, **Then** all sidebar instances display the new session simultaneously
3. **Given** a session is deleted from any tab, **When** the deletion completes, **Then** no tab shows the deleted session on subsequent renders

---

### User Story 3 - Sidebar Interaction Without Cross-Tab Interference (Priority: P2)

A developer enters navigation mode in the active tab's sidebar to browse, filter, and act on sessions. This interaction (cursor movement, filter text, mode transitions, rename editing) is entirely local to that tab's sidebar. Other tabs are unaffected, and the interactive state does not leak or conflict.

**Why this priority**: Keeping interaction state local avoids round-trip latency for every keypress and prevents one tab's interaction from interfering with another. It enables snappy, responsive local UX while still delegating state-changing actions to the central authority.

**Independent Test**: Enter navigate mode in Tab 1, type a filter query, switch to Tab 2 (which should be in passive mode), return to Tab 1 and confirm the filter state is preserved.

**Acceptance Scenarios**:

1. **Given** the sidebar in Tab 1 is in navigate mode with a filter active, **When** the user switches to Tab 2, **Then** Tab 2's sidebar is in passive mode with no filter applied
2. **Given** the user is typing a rename in Tab 1's sidebar, **When** Tab 2 receives a render update, **Then** Tab 2 displays the update without interrupting Tab 1's rename input
3. **Given** the sidebar shows a delete confirmation prompt, **When** the user switches away and back, **Then** the confirmation prompt is dismissed (safe default)

---

### User Story 4 - Automatic Sidebar Discovery on Tab Creation (Priority: P2)

When a new tab opens, its sidebar instance automatically registers with the central session authority and begins displaying the current session list. No manual configuration or restart is required. When tabs are closed, their sidebar registrations are cleaned up silently.

**Why this priority**: Tabs are created and destroyed frequently. The system must handle this lifecycle transparently without user intervention or error accumulation.

**Independent Test**: Open a new tab and verify the sidebar populates with the current session list within 2 seconds. Close a tab and verify no errors or orphaned state result.

**Acceptance Scenarios**:

1. **Given** 3 tabs are open with active sessions, **When** the user opens a 4th tab, **Then** the new tab's sidebar displays all current sessions within 2 seconds
2. **Given** 5 tabs are open, **When** the user closes Tab 3, **Then** the remaining tabs continue operating normally with no errors or state corruption
3. **Given** a tab is closed while its sidebar was in navigate mode, **When** the system processes the closure, **Then** no orphaned state or error messages result

---

### User Story 5 - Hook Routing to Single Processor (Priority: P3)

Claude Code lifecycle hooks (session start, activity changes, completion) are delivered to a single processing point rather than broadcast to all tab instances. This eliminates redundant processing and ensures each hook event is handled exactly once.

**Why this priority**: While functionally equivalent to the current "first match wins" approach, single-delivery routing removes wasted processing cycles on N-1 unused instances and simplifies the hook handling logic.

**Independent Test**: Trigger a Claude Code hook event and verify it is processed exactly once, with the resulting state change visible across all sidebar instances.

**Acceptance Scenarios**:

1. **Given** a Claude Code session starts in any tab, **When** the start hook fires, **Then** exactly one processing instance handles the hook and updates all sidebars
2. **Given** a session transitions from working to idle, **When** the activity hook fires, **Then** all sidebars reflect the new activity state after a single processing pass

---

### Edge Cases

- What happens when the central session authority has not yet initialized but a sidebar instance loads? The sidebar should display a "loading" state until it receives its first data payload.
- What happens when all tabs are closed and reopened (session reattach)? The central authority should restore persisted session state from its cache file and broadcast to newly created sidebar instances.
- What happens when a sidebar sends an action (rename, delete) but the target session no longer exists? The central authority should respond with an appropriate notification (e.g., "Session not found") that the sidebar displays.
- What happens when tab indices shift due to tab insertion or closure in the middle? The central authority must re-assign tab indices to all sidebars and broadcast updated assignments.
- What happens if the central authority crashes or is unloaded? The central authority restores from its persisted cache file on restart. Sidebar instances detect the absence of render updates (timeout) and display a "controller unavailable" status. Upon receiving the next render payload after recovery, sidebars reconnect automatically without user intervention.

## Clarifications

### Session 2026-03-31

- Q: What recovery behavior when the controller is unloaded or crashes mid-session? → A: Controller restores from persisted cache file on restart; sidebars reconnect automatically upon receiving next render payload.
- Q: Should the controller coalesce rapid state changes to prevent render storms during burst events? → A: Yes, coalesce rapid changes within a short window before broadcasting to prevent overwhelming sidebars during bulk session transitions.
- Q: How should existing installations transition from the single-plugin to the two-binary model? → A: Clean replacement during install. The CLI install command removes the old single binary and writes both new binaries. No data migration needed since the cache file format is preserved.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST provide a single authoritative source of session state that all sidebar instances read from, eliminating independent state copies and synchronization logic
- **FR-002**: The system MUST process heavyweight events (pane updates, tab updates, timers, command results) in exactly one instance, not once per tab
- **FR-003**: The system MUST broadcast pre-computed display data to all sidebar instances after each state change, so sidebars perform only trivial rendering work
- **FR-004**: The system MUST handle sidebar lifecycle (creation, registration, tab index assignment, cleanup on tab close) automatically without user intervention
- **FR-005**: The system MUST support all existing user interactions (session switching, renaming, deleting, pausing, filtering, keyboard navigation) with the same or better responsiveness
- **FR-006**: The system MUST persist session state to survive session reattach (detach and reattach to the terminal multiplexer)
- **FR-007**: The system MUST deliver hook events from the CLI to a single processing point rather than broadcasting to all instances
- **FR-008**: The system MUST reassign tab indices to sidebars when tabs are added or removed, ensuring each sidebar knows which tab it belongs to
- **FR-009**: The system MUST support a two-level notification system: centrally-managed notifications for state changes (included in render payloads) and locally-managed notifications for interaction feedback (mode transitions, rename editing, confirmation prompts)
- **FR-010**: The system MUST support local session filtering within each sidebar instance, operating on cached display data without requiring a round trip to the central authority
- **FR-011**: The system MUST register keyboard shortcuts once (centrally) rather than per-tab, and route triggered actions to the appropriate handler
- **FR-012**: The system MUST produce two separate deployable artifacts from a single codebase, sharing common type definitions and serialization logic
- **FR-013**: The CLI tool MUST embed and install both artifacts, and generate appropriate configuration for both the central processor and per-tab renderers
- **FR-014**: The central authority MUST coalesce rapid state changes within a short window before broadcasting render payloads, preventing sidebar render storms during burst events (e.g., bulk session transitions)
- **FR-015**: The CLI install command MUST perform a clean replacement when upgrading from the single-plugin architecture, removing the old single binary and writing both new binaries without requiring data migration
- **FR-016**: The central authority MUST restore session state from its persisted cache file after a crash or restart, and sidebars MUST reconnect automatically upon receiving the next render payload

### Key Entities

- **Session**: Represents a Claude Code process. Key attributes: pane identifier, display name, activity state (working/idle/waiting/attended), indicator symbol, color, associated branch name, tab location, paused flag
- **Render Payload**: A pre-computed snapshot of all session data plus contextual state (focused pane, active tab, notification, summary counts) sent from the central authority to all sidebar instances
- **Sidebar Registration**: The association between a sidebar instance and its tab, managed by the central authority through a discovery handshake protocol
- **Action Message**: A user-initiated command (switch, rename, delete, pause, navigate) sent from a sidebar instance to the central authority for processing

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Sidebar click and keyboard response time remains constant regardless of tab count (no degradation from 2 tabs to 20 tabs)
- **SC-002**: Per-event processing overhead scales as O(1) for heavyweight operations (state management, session detection, cleanup), not O(N) where N is tab count
- **SC-003**: Session state is consistent across all open tabs at all times, with zero synchronization conflicts or stale-state displays
- **SC-004**: New tabs display the current session list within 2 seconds of opening
- **SC-005**: Tab closure completes cleanly with no orphaned registrations, error messages, or state corruption
- **SC-006**: All existing sidebar functionality (switching, renaming, deleting, pausing, filtering, keyboard navigation, help overlay) works identically to the current implementation
- **SC-007**: The codebase eliminates the existing synchronization subsystem (pipe-based sync, file-based metadata merging, peer request protocols) entirely, reducing complexity and removing an entire class of race conditions
- **SC-008**: Hook events from the CLI are processed exactly once, not N times
