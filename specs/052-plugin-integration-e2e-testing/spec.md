# Feature Specification: Plugin Integration and E2E Testing

**Feature Branch**: `052-plugin-integration-e2e-testing`
**Created**: 2026-05-07
**Status**: Draft
**Input**: User description: "Add plugin-level integration tests that exercise SidebarRendererPlugin and ControllerPlugin through their ZellijPlugin trait methods with synthetic events, closing the gap between unit tests and real Zellij behavior."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Sidebar Receives Render Payload and Displays Sessions (Priority: P1)

A developer adds a new field to the render payload or changes how sessions are displayed. They run the integration test suite and immediately see whether the full pipe-to-render chain still works, without needing to start Zellij manually.

**Why this priority**: The render payload pipeline is the most exercised code path in production. Regressions here affect every user interaction with the sidebar.

**Independent Test**: Can be fully tested by constructing a SidebarRendererPlugin, calling load/update/pipe with a render payload, and asserting on internal state. Delivers confidence that payload deserialization, state updates, and session filtering work end-to-end.

**Acceptance Scenarios**:

1. **Given** a freshly created SidebarRendererPlugin with permissions granted, **When** a render payload containing three sessions is sent via pipe, **Then** the plugin state contains three filtered sessions with correct display names and activity labels.
2. **Given** a sidebar that has received an initial render payload, **When** a subsequent payload with different session data arrives, **Then** the plugin state reflects the updated sessions, replacing the previous data.
3. **Given** a sidebar that has not yet received permission grant, **When** a render payload arrives, **Then** the payload is not processed until permissions are granted.

---

### User Story 2 - Controller Processes Hook Events into Session State (Priority: P1)

A developer modifies the hook event processing logic (mapping Claude Code lifecycle events to session activities). They run integration tests that send hook payloads through the controller pipe interface and verify the resulting session state, catching protocol regressions before manual testing.

**Why this priority**: Hook event processing is the primary data ingestion path. Incorrect mapping between hook events and activity states causes visible user-facing bugs (wrong status indicators, missing sessions).

**Independent Test**: Can be tested by creating a ControllerPlugin, granting permissions, sending hook pipe messages, and asserting on session state. Delivers confidence that the hook-to-session pipeline works correctly.

**Acceptance Scenarios**:

1. **Given** a controller with permissions granted, **When** a SessionStart hook event arrives for a new pane, **Then** a new session is created with Init activity.
2. **Given** a controller with an existing session, **When** a PreToolUse hook event arrives for that session, **Then** the session activity transitions to Working.
3. **Given** a controller with an existing session, **When** a Stop hook event arrives, **Then** the session activity transitions to Done.

---

### User Story 3 - Sidebar-Controller Discovery Protocol (Priority: P2)

A developer changes the sidebar initialization or the discovery handshake. Integration tests verify that the SidebarHello/SidebarInit exchange works correctly through the pipe interface, catching serialization mismatches between the two plugin variants.

**Why this priority**: Discovery failures cause sidebars to never receive render updates, which is a total failure mode. However, this protocol is relatively stable and changes less frequently than payload handling.

**Independent Test**: Can be tested by creating both a ControllerPlugin and SidebarRendererPlugin, simulating the hello/init pipe exchange, and verifying both sides reach their initialized state.

**Acceptance Scenarios**:

1. **Given** a controller with permissions granted, **When** a SidebarHello message arrives with a plugin ID, **Then** the controller registers the sidebar in its registry and sends a SidebarInit response.
2. **Given** a sidebar that has sent SidebarHello, **When** a SidebarInit response arrives with a tab index, **Then** the sidebar stores the tab index and is ready to receive render payloads.

---

### User Story 4 - Sidebar Action Message Dispatch (Priority: P2)

A developer adds a new action type or modifies how the sidebar sends action messages to the controller. Integration tests verify that user interactions (session switching, renaming, pausing) produce correctly serialized action messages that the controller processes into state changes.

**Why this priority**: Action dispatch is the user interaction path. Serialization mismatches between sidebar and controller cause silent failures where clicks appear to do nothing.

**Independent Test**: Can be tested by constructing an ActionMessage, sending it through the controller's pipe method, and verifying the controller state reflects the requested action.

**Acceptance Scenarios**:

1. **Given** a controller with an active session, **When** a Switch action message arrives targeting that session, **Then** the controller triggers a tab/pane focus change for the targeted session.
2. **Given** a controller with an active session, **When** a Pause action message arrives, **Then** the session's paused state toggles.
3. **Given** a controller with an active session, **When** an Attend action message arrives for a done session, **Then** the session's done_attended flag is set.

---

### User Story 5 - Permission Grant and Deferred Event Replay (Priority: P3)

A developer changes the permission handling or event queuing logic. Integration tests verify that events received before permissions are granted are properly queued and replayed once permissions arrive.

**Why this priority**: Permission handling is a one-time startup sequence that rarely changes, but failures here cause complete plugin dysfunction that is hard to debug.

**Independent Test**: Can be tested by creating a plugin, sending events before granting permissions, then granting permissions and verifying all queued events were processed.

**Acceptance Scenarios**:

1. **Given** a freshly loaded controller, **When** hook events arrive before permissions are granted, **Then** those events are queued and not processed.
2. **Given** a controller with queued events, **When** permissions are granted, **Then** all queued events are processed in order and the resulting session state matches what would have occurred if permissions had been granted first.

---

### Edge Cases

- What happens when a pipe message contains malformed JSON? The plugin must not panic and should log the error gracefully.
- What happens when a hook event references a pane ID that does not exist in any tab? The controller should create a new session entry.
- What happens when a SidebarHello arrives for a plugin ID that is already registered? The controller should update the existing registration.
- What happens when a render payload arrives with zero sessions? The sidebar should display an empty state without errors.
- What happens when an action message references a non-existent pane ID? The controller should handle the missing target gracefully.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The test suite MUST exercise SidebarRendererPlugin through its ZellijPlugin trait methods (load, update, pipe, render) with synthetic events, without requiring a running Zellij instance.
- **FR-002**: The test suite MUST exercise ControllerPlugin through its ZellijPlugin trait methods (load, update, pipe) with synthetic hook event payloads, without requiring a running Zellij instance.
- **FR-003**: Tests MUST verify the full event dispatch chain: pipe message arrives, state updates, subsequent render reflects the change.
- **FR-004**: Tests MUST verify the controller-sidebar protocol by testing serialization roundtrips of RenderPayload, ActionMessage, SidebarHello, and SidebarInit through the pipe interface.
- **FR-005**: Tests MUST verify the permission grant flow, including deferred event queuing and replay for both controller and sidebar plugins.
- **FR-006**: Tests MUST verify mode transitions in the sidebar (Passive, Navigate, Rename) triggered through the pipe interface.
- **FR-007**: Test helper functions MUST provide convenient construction of pipe messages (hook events, render payloads, action messages) to reduce boilerplate in individual tests.
- **FR-008**: Tests MUST verify error handling for malformed pipe messages (invalid JSON, unknown message types) without panicking.
- **FR-009**: Tests MUST run as part of the standard `cargo test` suite on native targets, with no special environment setup required.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: At least 15 integration tests cover the five user stories defined above.
- **SC-002**: All integration tests pass on native target as part of the standard test suite, completing in under 5 seconds total.
- **SC-003**: The test suite catches at least the following regression categories: payload deserialization failures, protocol serialization mismatches, permission flow bugs, and mode transition errors.
- **SC-004**: Test helper functions reduce per-test setup boilerplate to 5 lines or fewer for common scenarios (create plugin, grant permissions, send payload).
- **SC-005**: No existing tests are broken or modified by the addition of integration tests.

## Assumptions

- The existing ZellijPlugin trait method signatures remain stable (zellij-tile 0.43.1).
- WASM host function calls (focus_plugin_pane, pipe_message_to_plugin, rename_tab) are already stubbed as no-ops in the test environment, and integration tests accept this limitation.
- Integration tests do not verify terminal rendering output (ANSI codes). They verify state changes only.
- The existing test helper infrastructure (make_payload, make_session, make_state_with_sessions) can be extended for integration test use.
- Timer-dependent behavior (grace periods, stale session cleanup) is tested through direct state manipulation rather than real time delays.
- Multi-instance coordination (multiple sidebar instances communicating through a shared controller) is out of scope for this feature.
