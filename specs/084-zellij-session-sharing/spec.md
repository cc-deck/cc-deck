# Feature Specification: Zellij Session Sharing

**Feature Branch**: `084-zellij-session-sharing`

**Created**: 2026-07-24

**Status**: Draft

**Input**: Workspace-centric, ephemeral sharing of one local workspace's canonical Zellij session with named interactive and observer invitations, browser and terminal access, and pluggable exposure providers.

## Clarifications

### Session 2026-07-24

- Q: How many sharing operations may one host run at once? → A: One active sharing operation at a time.
- Q: How should the next command behave after an unclean host or provider exit? → A: Detect and clean stale sharing resources before starting or reporting status.
- Q: What constitutes successful stop completion? → A: All clients disconnected, credentials revoked, session unshared, and endpoint closed; otherwise status remains degraded with residual exposure identified.
- Q: Where does sharing live in the CLI? → A: Under `cc-deck ws`; the standalone public sharing command is removed.
- Q: Can an existing private session be converted in place? → A: No. `--share` never replaces a private running session; the host must explicitly kill and restart it.
- Q: What happens after a shared session dies? → A: A plain restart recreates the canonical session privately; sharing requires another explicit `--share`.

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

**Independent Test**: Start sharing, join with the observer browser invitation and observer terminal command, and verify that the observer can follow rendered output and use only client-local viewing behavior, such as scrolling when supported, but cannot send keyboard, mouse, paste, resize, focus, or control input to the shared session.

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

As a host, I use the initial default exposure provider through a provider-independent sharing experience that permits additional providers to be added without changing invitations or lifecycle behavior.

**Why this priority**: Different networks require different exposure mechanisms, but provider choice must not complicate the core collaboration experience.

**Independent Test**: Exercise the default production provider and an isolated provider-contract test implementation, verifying that each produces the same four invitation forms and supports the same status, failure-cleanup, and stop behavior.

**Acceptance Scenarios**:

1. **Given** the default provider is available, **When** the host starts sharing without choosing a provider, **Then** sharing uses that provider and reports which one is active.
2. **Given** an additional provider implementation is installed and configured, **When** the host selects it, **Then** the same interactive and observer invitations are produced through that provider.
3. **Given** the selected provider or a required runtime capability is missing, incompatible, or cannot start, **When** sharing is requested, **Then** the host receives an actionable prerequisite error and no partially active sharing operation remains.

### Edge Cases

- Sharing is requested when the selected session is already shared.
- Sharing is requested when no active Zellij session can be identified.
- A generated endpoint becomes unavailable after invitations have been issued.
- A provider exits unexpectedly while clients are connected.
- The host process exits without explicitly stopping an active sharing operation.
- A new start or status request encounters resources left by an unclean prior exit.
- One credential is created successfully but creation of the other fails.
- Credential revocation succeeds for one role but fails for the other.
- Multiple hosts or commands attempt to start or stop sharing concurrently.
- Session names contain spaces or characters that require safe invitation encoding.
- An observer attempts to send keyboard, mouse, resize, paste, or control input.

### Error Handling and Recovery

- If the selected session is already the active sharing operation, start is idempotent and returns its safe status without creating new credentials or another endpoint.
- If a different sharing operation is active, start is rejected until the host stops it successfully.
- If no running session can be selected, startup fails before creating credentials or an endpoint.
- If the endpoint or provider exits unexpectedly, status becomes degraded, existing credentials are revoked, session sharing is disabled, and the host is told whether any residual endpoint remains.
- If the sharing controller exits uncleanly, an independent lifecycle guard detects the loss and automatically starts safety teardown; a later start or status action also detects and cleans any residual resources before permitting new invitations.
- If either credential cannot be created, startup revokes any credential already created, disables session sharing, closes the endpoint, and issues no invitation.
- Concurrent lifecycle actions are serialized; redundant starts are idempotent for the same operation and stops remain safe to repeat.
- Every partial startup or teardown failure produces a degraded status naming each resource whose safe state could not be confirmed.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST start an ephemeral sharing operation for exactly one local workspace's canonical Zellij session.
- **FR-001a**: The system MUST allow no more than one active sharing operation per host at a time.
- **FR-002**: The sharing operation MUST expose the complete selected Zellij session, including all tabs, panes, sidebars, and session-level controls available to an attached client.
- **FR-003**: The system MUST ensure that the sharing operation does not grant access to any other Zellij session owned by the host.
- **FR-004**: Initial sharing MUST create one named interactive invitation and one named observer invitation, and the host MAY add multiple independently revocable invitations for either role.
- **FR-005**: The system MUST allow multiple concurrent clients to use invitations without persisting their raw tokens.
- **FR-006**: The system MUST provide a session-specific browser invitation for interactive collaborators.
- **FR-007**: The system MUST provide a session-specific terminal attach command for interactive collaborators.
- **FR-008**: The system MUST provide a session-specific browser invitation for read-only observers.
- **FR-009**: The system MUST provide a session-specific terminal attach command for read-only observers.
- **FR-010**: Interactive clients MUST be able to control the complete shared session according to the normal capabilities of an attached Zellij client.
- **FR-011**: Read-only clients MUST be prevented from sending keyboard, mouse, paste, resize, tab or pane focus, or terminal control input to the shared session; client-local viewing behavior that cannot change shared state MAY remain available.
- **FR-012**: Invitations MUST clearly identify their access role and connection method so the host cannot accidentally distribute the wrong level of access.
- **FR-013**: The system MUST define one provider contract covering endpoint start, readiness, safe status, failure reporting, and idempotent stop so additional interchangeable providers can preserve the same sharing experience.
- **FR-013a**: V1 MUST include Cloudflare Quick Tunnel as its default production exposure provider and MUST validate provider interchangeability with an isolated contract test implementation; additional production providers are out of scope.
- **FR-013b**: Every production exposure provider MUST encrypt credentials and session traffic in transit. V1 terminal invitations MAY require an explicit certificate-validation bypass when the selected provider is incompatible with Zellij terminal validation, but MUST label the command experimental and display a prominent interception-risk warning before revealing it.
- **FR-014**: The system MUST provide a default exposure provider requiring no inbound network configuration from the host.
- **FR-015**: The configured provider MUST be used through the provider-independent service contract.
- **FR-016**: Workspace list/status MUST report infrastructure, session, and sharing state plus safe endpoint and invitation label/role metadata without redisplaying secret credential values after initial creation.
- **FR-017**: The system MUST NOT issue invitations until the endpoint, session sharing, and both credentials are ready.
- **FR-018**: If startup fails, the system MUST remove any endpoint or credentials created during that attempt and report the failure.
- **FR-019**: The host MUST be able to stop the active sharing operation with one action.
- **FR-020**: Unsharing MUST revoke every active invitation, close the public endpoint, and disconnect remote clients while leaving the canonical session running.
- **FR-021**: Teardown MUST attempt all safety actions even if an earlier action fails, and MUST report any credential, session, client, or endpoint that may remain active.
- **FR-021a**: Stop MUST be reported as complete only after remote clients are disconnected, both credentials are revoked, the selected session is no longer shared, and the public endpoint is closed; otherwise status MUST remain degraded and identify residual exposure.
- **FR-022**: Old invitations MUST remain invalid after sharing stops or after a new sharing operation begins.
- **FR-023**: The system MUST prevent conflicting concurrent start and stop operations from creating multiple endpoints or leaving untracked credentials active.
- **FR-023a**: Before starting a new operation or reporting inactive status, the system MUST detect resources left by an unclean prior exit and attempt to clean them up; if safe cleanup cannot be confirmed, it MUST refuse to issue new invitations and identify the residual exposure.
- **FR-023b**: V1 MUST run a detached lifecycle guard that attempts teardown when it receives a supported termination signal or observes provider exit. Uncatchable guard termination MAY leave resources active until a later lifecycle command reconciles them; status and documentation MUST identify this limitation.
- **FR-024**: The system MUST communicate that interactive access grants trusted users control over the host's terminal session before displaying or copying an interactive invitation.
- **FR-025**: User-facing documentation MUST explain prerequisites, interactive versus observer access, browser and terminal joining, provider selection, lifecycle commands, security implications, and recovery from partial failures.
- **FR-025a**: Documentation delivery MUST include README updates, CLI reference coverage, an Antora sharing guide, configuration reference coverage, and successful prose-profile validation.
- **FR-026**: Before creating sharing resources, the system MUST verify that the installed Zellij runtime supports web serving, session-specific sharing, interactive credentials, read-only credentials, remote terminal attach, and forced shutdown of remote access; unsupported or incompatible runtimes MUST fail with an actionable prerequisite message.
- **FR-027**: Session names and all invitation values MUST be encoded separately for URL and command contexts so spaces and reserved characters produce valid invitations without altering command structure or targeting another session.
- **FR-028**: Public commands MUST be `ws new`, `ws start`, `ws attach`, `ws invite`, `ws revoke`, `ws unshare`, `ws list`, and `ws status`; sharing MUST be rejected for non-local workspace backends.
- **FR-029**: `ws new` MUST be ready and private by default, `--no-start` MUST leave it stopped, and `--share` MUST conflict with `--no-start`.
- **FR-030**: `--share` MUST create a missing canonical session with web sharing enabled and MUST NOT kill or mutate an existing private session.
- **FR-031**: If the canonical shared session disappears, the guard MUST initiate sharing teardown. A later plain start or attach MUST recreate the session privately.

### Key Entities

- **Sharing Operation**: One ephemeral exposure of one local workspace's canonical Zellij session; tracks workspace identity, lifecycle state, provider, endpoint, and invitation metadata.
- **Invitation**: A session-specific connection instruction combining an endpoint, session identity, access role, and connection method; exists in browser and terminal forms.
- **Invitation Record**: A named, independently revocable role credential represented persistently only by label, role, state, and creation time.
- **Exposure Provider**: A selectable mechanism that opens, reports, and closes the externally reachable endpoint while preserving common sharing behavior.
- **Sharing Status**: The safe, non-secret summary of the current operation, including session, provider, endpoint, role availability, and any teardown warnings.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A host can start sharing and obtain all four invitation forms within 15 seconds when the provider can establish its endpoint within 10 seconds and the host-to-provider round-trip latency is at most 250 milliseconds.
- **SC-002**: At least two interactive collaborators and two observers can connect concurrently to the same complete session using the two shared role credentials.
- **SC-003**: In acceptance testing, 100% of observer attempts to change the session through keyboard, mouse, resize, paste, tab or pane focus, or terminal control sequences are rejected.
- **SC-004**: In acceptance testing with multiple local sessions present, 100% of invitations expose only the host-selected session.
- **SC-005**: A host can stop sharing with one action, and all connected remote clients lose access within 5 seconds when the host-to-provider round-trip latency is at most 250 milliseconds.
- **SC-006**: After stop completes, 100% of attempts to reuse either old invitation are denied.
- **SC-007**: The production provider and isolated contract test implementation pass the same endpoint-start, readiness, status, startup-rollback, invitation, idempotent-stop, and partial-failure test suite.
- **SC-008**: A first-time host can complete the documented start, invite, inspect-status, and stop flow without external networking configuration or undocumented steps.
- **SC-009**: Forced sharing-controller termination tests document which resources are cleaned automatically and verify that a subsequent stop, status, or start action detects and attempts cleanup of every residual resource.
- **SC-010**: Invitations generated for a representative set of session names containing spaces and URL or shell reserved characters connect only to the intended session and never alter command structure.

## Assumptions

- The host intentionally trusts interactive collaborators with the same terminal-level capabilities available in the shared Zellij session.
- Initial sharing creates one invitation per role; additional named invitations and per-invitation revocation are supported, while identity verification and audit attribution remain out of scope.
- Sharing is temporary. Credentials and endpoints are not reused between sharing operations.
- Zellij 0.44.3 is the validated minimum baseline for web serving, session-specific sharing, interactive and read-only credentials, browser access, remote terminal attach, and stopping remote access. Runtime capability checks remain authoritative so incompatible builds fail safely even when their version string appears sufficient.
- Supported browser and terminal clients provide the complete-session experience and enforce the selected access role; compatibility is rejected rather than silently degraded when a required capability is absent.
- Cloudflare Quick Tunnel can create an outbound encrypted public endpoint with no inbound firewall or router changes. Browser invitations validate the public endpoint normally; V1 terminal invitations are explicitly experimental because the validated spike required a certificate-validation bypass.
- Provider-specific accounts or configuration may be required by non-default providers and are supplied by the host outside this feature.
- Presence displays, authenticated user identities, hand-off controls, persistent URLs, hosted multi-tenancy, and sharing non-local workspace backends are out of scope.
- The existing multiplayer resilience and independent focus behavior remain prerequisites for a stable multi-client experience.

## Known V1 Security Limitations

- Terminal attachment through the default provider is experimental because current evidence requires disabling server-identity validation. The invitation MUST explain the interception risk and MUST NOT hide or silently add the bypass.
- Cleanup after an unclean sharing-controller exit is best effort. Credentials or an endpoint may remain usable until the host runs a later stop, status, or start action.
- Hosts are expected to remain present during V1 sharing, distribute invitations only to trusted recipients, and explicitly stop sharing when collaboration ends.
- Brainstorm 089 tracks the hardening spike required to remove these limitations; they are accepted V1 risks, not evidence that secure teardown or validated terminal TLS has been achieved.
