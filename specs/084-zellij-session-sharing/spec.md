# Feature Specification: Zellij Session Sharing

**Feature Branch**: `084-zellij-session-sharing`

**Created**: 2026-07-24

**Status**: Draft

**Input**: Host-controlled, ephemeral sharing of one complete Zellij session with interactive and read-only invitations, browser and terminal access, and pluggable exposure providers.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Share a Complete Session (Priority: P1)

As a host, I can temporarily expose my current complete Zellij session and receive an interactive invitation so remote collaborators can join the same workspace through either a browser or their terminal.

**Why this priority**: Interactive remote collaboration is the core value of the feature. Without it, no pair-programming session can take place.

**Independent Test**: Start sharing a session, use the resulting browser invitation and terminal invitation from separate clients, and verify that both clients can view and interact with the full shared Zellij session.

**Acceptance Scenarios**:

1. **Given** a host is attached to a Zellij session that is not shared, **When** the host starts sharing, **Then** the host receives an interactive browser invitation and an interactive terminal attach command for that exact session.
2. **Given** an active interactive invitation, **When** multiple collaborators use it concurrently, **Then** each collaborator joins the same complete Zellij session and can interact with its tabs, panes, and controls.
3. **Given** other Zellij sessions exist for the host, **When** a collaborator uses the invitation, **Then** only the session selected by the host is available through that sharing operation.
4. **Given** sharing cannot be established safely, **When** the host starts sharing, **Then** no usable invitation is issued and the host receives an actionable failure message.

---

### User Story 2 - Invite Read-Only Observers (Priority: P2)

As a host, I receive a separate observer invitation so people can follow the entire shared session without being able to alter it.

**Why this priority**: Observation supports teaching, demonstrations, and larger audiences without giving every viewer control.

**Independent Test**: Start sharing, join with the observer browser invitation and observer terminal command, and verify that the observer can navigate the rendered session view but cannot send terminal or control input that changes the session.

**Acceptance Scenarios**:

1. **Given** an active sharing operation, **When** an observer joins through the browser invitation, **Then** the observer sees the complete selected session but cannot modify it.
2. **Given** an active sharing operation, **When** an observer joins through the terminal attach command, **Then** the observer receives the same read-only access.
3. **Given** multiple observers reuse the observer invitation concurrently, **When** they connect, **Then** all can follow the session without gaining interactive privileges.

---

### User Story 3 - End Sharing Completely (Priority: P3)

As a host, I can stop sharing in one action so the public endpoint closes, both invitations become invalid, and connected remote users lose access.

**Why this priority**: A predictable and complete teardown is essential for safely exposing an interactive terminal session.

**Independent Test**: Start sharing, connect interactive and observer clients, stop sharing, and verify that existing clients disconnect and neither invitation permits reconnection.

**Acceptance Scenarios**:

1. **Given** interactive collaborators and observers are connected, **When** the host stops sharing, **Then** all remote clients lose access, both credentials are revoked, and the public endpoint closes.
2. **Given** sharing has stopped, **When** anyone retries either old invitation, **Then** access is denied.
3. **Given** part of teardown fails, **When** the host stops sharing, **Then** the system continues attempting the remaining safety actions and reports any residual exposure clearly.

---

### User Story 4 - Choose an Exposure Provider (Priority: P4)

As a host, I can use the default exposure provider or select another available provider without changing how invitations and the sharing lifecycle behave.

**Why this priority**: Different networks require different exposure mechanisms, but provider choice must not complicate the core collaboration experience.

**Independent Test**: Start equivalent sharing operations with two provider implementations and verify that each produces the same four invitation forms and supports the same status and stop behavior.

**Acceptance Scenarios**:

1. **Given** the default provider is available, **When** the host starts sharing without choosing a provider, **Then** sharing uses that provider and reports which one is active.
2. **Given** another configured provider is available, **When** the host selects it, **Then** the same interactive and observer invitations are produced through that provider.
3. **Given** the selected provider is missing or cannot start, **When** sharing is requested, **Then** the host receives an actionable error and no partially active sharing operation remains.

### Edge Cases

- Sharing is requested when the selected session is already shared.
- Sharing is requested when no active Zellij session can be identified.
- A generated endpoint becomes unavailable after invitations have been issued.
- A provider exits unexpectedly while clients are connected.
- The host process exits without explicitly stopping an active sharing operation.
- One credential is created successfully but creation of the other fails.
- Credential revocation succeeds for one role but fails for the other.
- Multiple hosts or commands attempt to start or stop sharing concurrently.
- Session names contain spaces or characters that require safe invitation encoding.
- An observer attempts to send keyboard, mouse, resize, paste, or control input.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST start an ephemeral sharing operation for exactly one host-selected, currently running Zellij session.
- **FR-002**: The sharing operation MUST expose the complete selected Zellij session, including all tabs, panes, sidebars, and session-level controls available to an attached client.
- **FR-003**: The system MUST ensure that the sharing operation does not grant access to any other Zellij session owned by the host.
- **FR-004**: The system MUST create exactly one temporary interactive credential and one temporary read-only credential for each sharing operation.
- **FR-005**: The system MUST allow multiple concurrent clients to reuse each role credential.
- **FR-006**: The system MUST provide a session-specific browser invitation for interactive collaborators.
- **FR-007**: The system MUST provide a session-specific terminal attach command for interactive collaborators.
- **FR-008**: The system MUST provide a session-specific browser invitation for read-only observers.
- **FR-009**: The system MUST provide a session-specific terminal attach command for read-only observers.
- **FR-010**: Interactive clients MUST be able to control the complete shared session according to the normal capabilities of an attached Zellij client.
- **FR-011**: Read-only clients MUST be prevented from sending any input that changes the shared session.
- **FR-012**: Invitations MUST clearly identify their access role and connection method so the host cannot accidentally distribute the wrong level of access.
- **FR-013**: The system MUST support multiple interchangeable exposure providers that present a consistent start, status, and stop experience.
- **FR-014**: The system MUST provide a default exposure provider requiring no inbound network configuration from the host.
- **FR-015**: The host MUST be able to select a non-default available provider when starting sharing.
- **FR-016**: The system MUST report whether sharing is active, the selected session, the active provider, the endpoint, and the availability of both invitation roles without redisplaying secret credential values after initial creation.
- **FR-017**: The system MUST NOT issue invitations until the endpoint, session sharing, and both credentials are ready.
- **FR-018**: If startup fails, the system MUST remove any endpoint or credentials created during that attempt and report the failure.
- **FR-019**: The host MUST be able to stop the active sharing operation with one action.
- **FR-020**: Stopping sharing MUST revoke both role credentials, stop sharing the selected session, close the public endpoint, and disconnect already-authenticated remote clients.
- **FR-021**: Teardown MUST attempt all safety actions even if an earlier action fails, and MUST report any credential, session, client, or endpoint that may remain active.
- **FR-022**: Old invitations MUST remain invalid after sharing stops or after a new sharing operation begins.
- **FR-023**: The system MUST prevent conflicting concurrent start and stop operations from creating multiple endpoints or leaving untracked credentials active.
- **FR-024**: The system MUST communicate that interactive access grants trusted users control over the host's terminal session before displaying or copying an interactive invitation.
- **FR-025**: User-facing documentation MUST explain prerequisites, interactive versus observer access, browser and terminal joining, provider selection, lifecycle commands, security implications, and recovery from partial failures.

### Key Entities

- **Sharing Operation**: One ephemeral exposure of one selected Zellij session; tracks lifecycle state, provider, endpoint, and its two role credentials.
- **Invitation**: A session-specific connection instruction combining an endpoint, session identity, access role, and connection method; exists in browser and terminal forms.
- **Role Credential**: A temporary secret shared by either all interactive collaborators or all observers for one sharing operation.
- **Exposure Provider**: A selectable mechanism that opens, reports, and closes the externally reachable endpoint while preserving common sharing behavior.
- **Sharing Status**: The safe, non-secret summary of the current operation, including session, provider, endpoint, role availability, and any teardown warnings.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A host can start sharing and obtain all four invitation forms within 15 seconds under normal network conditions.
- **SC-002**: At least two interactive collaborators and two observers can connect concurrently to the same complete session using the two shared role credentials.
- **SC-003**: In acceptance testing, 100% of observer attempts to change the session through keyboard, mouse, resize, paste, or control input are rejected.
- **SC-004**: In acceptance testing with multiple local sessions present, 100% of invitations expose only the host-selected session.
- **SC-005**: A host can stop sharing with one action, and all connected remote clients lose access within 5 seconds under normal network conditions.
- **SC-006**: After stop completes, 100% of attempts to reuse either old invitation are denied.
- **SC-007**: Equivalent tests against any conforming exposure provider produce the same invitation roles, connection methods, status information, and teardown outcome.
- **SC-008**: A first-time host can complete the documented start, invite, inspect-status, and stop flow without external networking configuration or undocumented steps.

## Assumptions

- The host intentionally trusts interactive collaborators with the same terminal-level capabilities available in the shared Zellij session.
- One interactive credential and one observer credential are shared by role; individual identity, audit attribution, and per-person revocation are out of scope.
- Sharing is temporary. Credentials and endpoints are not reused between sharing operations.
- Both browser and terminal clients support the complete-session experience and the selected access role.
- The initial default provider can create an outbound, automatically secured public endpoint without inbound firewall or router changes.
- Provider-specific accounts or configuration may be required by non-default providers and are supplied by the host outside this feature.
- Presence displays, named users, pair-programming roles, hand-off controls, persistent URLs, hosted multi-tenancy, and sharing remote workspace backends are out of scope.
- The existing multiplayer resilience and independent focus behavior remain prerequisites for a stable multi-client experience.

