# Feature Specification: Workspace Lifecycle Redesign

**Feature Branch**: `043-workspace-lifecycle`
**Created**: 2026-04-25
**Status**: Draft
**Input**: Redesign workspace lifecycle to separate infrastructure management from Zellij session management, fix layout bug on local workspaces, add `kill-session` command, make `attach` always lazy, and introduce a two-dimensional state model.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Attach to a local workspace with correct layout (Priority: P1)

A user creates a local workspace and attaches to it. The Zellij session is created with the cc-deck layout applied, including the sidebar plugin. This is the most common workflow and the one currently broken: running `ws start` then `ws attach` results in a session without the cc-deck layout.

**Why this priority**: This is the original bug that motivated the redesign. Local workspaces are the most common type and every user hits this.

**Independent Test**: Create a local workspace, run `ws attach`, verify the cc-deck sidebar plugin is visible in the Zellij session.

**Acceptance Scenarios**:

1. **Given** a newly created local workspace with no existing Zellij session, **When** the user runs `cc-deck ws attach mydev`, **Then** a Zellij session named `cc-deck-mydev` is created with the cc-deck layout and the user is attached to it.
2. **Given** a local workspace with an existing Zellij session (user previously detached), **When** the user runs `cc-deck ws attach mydev`, **Then** the user is reattached to the existing session with all panes and tabs preserved.
3. **Given** a local workspace where the cc-deck layout is not installed, **When** the user runs `cc-deck ws attach mydev`, **Then** the session is created with the default Zellij layout and a warning is printed.

---

### User Story 2 - Kill a Zellij session without affecting infrastructure (Priority: P1)

A user wants to reset their Zellij session (corrupted layout, too many panes, fresh start) without stopping the underlying container or pod. The `kill-session` command kills only the Zellij session. The next `attach` creates a fresh session with the layout.

**Why this priority**: Equally critical to the attach fix. Without this, users have no clean way to reset a session inside a running container.

**Independent Test**: Attach to a container workspace, run `ws kill-session`, verify the container is still running, run `ws attach` again, verify a fresh session with layout is created.

**Acceptance Scenarios**:

1. **Given** a container workspace with infra running and a Zellij session inside, **When** the user runs `cc-deck ws kill-session mycontainer`, **Then** the Zellij session inside the container is killed and the container remains running.
2. **Given** a local workspace with an existing Zellij session, **When** the user runs `cc-deck ws kill-session mydev`, **Then** the local Zellij session is killed.
3. **Given** an SSH workspace with a remote Zellij session, **When** the user runs `cc-deck ws kill-session myremote`, **Then** the remote Zellij session is killed via SSH.
4. **Given** a workspace with no existing Zellij session, **When** the user runs `cc-deck ws kill-session mydev`, **Then** a message indicates no session exists and the command exits cleanly.

---

### User Story 3 - Lazy attach starts infrastructure automatically (Priority: P2)

A user has a container workspace that was stopped (to save resources). Instead of running `ws start` then `ws attach`, the user runs `ws attach` directly. The command starts the container, creates the Zellij session with layout, and attaches, all in one step.

**Why this priority**: Reduces friction for the most common workflow. Users should not need to remember whether infrastructure needs starting.

**Independent Test**: Stop a container workspace, run `ws attach`, verify the container starts, a Zellij session is created with layout, and the user is connected.

**Acceptance Scenarios**:

1. **Given** a container workspace in stopped state, **When** the user runs `cc-deck ws attach mycontainer`, **Then** existing startup progress messages are displayed while the container starts, a Zellij session is created with the cc-deck layout, and the user is attached.
2. **Given** a k8s-deploy workspace scaled to zero, **When** the user runs `cc-deck ws attach myk8s`, **Then** the StatefulSet is scaled up, the pod becomes ready, a Zellij session is created, and the user is attached.
3. **Given** a container workspace already running with no Zellij session, **When** the user runs `cc-deck ws attach mycontainer`, **Then** only the Zellij session is created (no redundant start attempt) and the user is attached.

---

### User Story 4 - Infrastructure start/stop for resource management (Priority: P2)

A user wants to stop a container workspace to free resources (CPU, memory) without deleting it. They use `ws stop` to stop the infrastructure and `ws start` to bring it back. These commands are only available for workspace types that manage infrastructure (container, compose, k8s-deploy).

**Why this priority**: Important for resource management, but less frequent than attach/detach cycles.

**Independent Test**: Run `ws stop` on a container workspace, verify the container is stopped and the Zellij session is gone. Run `ws start`, verify the container is running. Run `ws attach`, verify a fresh session is created.

**Acceptance Scenarios**:

1. **Given** a container workspace with infra running and a Zellij session, **When** the user runs `cc-deck ws stop mycontainer`, **Then** the Zellij session is killed first, then the container is stopped.
2. **Given** a stopped container workspace, **When** the user runs `cc-deck ws start mycontainer`, **Then** the container is started (no Zellij session is created).
3. **Given** a local workspace, **When** the user runs `cc-deck ws start mydev`, **Then** a warning is printed: "Local workspaces have no infrastructure to start. Use 'cc-deck ws attach mydev' to connect."
4. **Given** an SSH workspace, **When** the user runs `cc-deck ws stop myremote`, **Then** a warning is printed: "SSH workspaces have no infrastructure to stop. Use 'cc-deck ws kill-session myremote' to end the session."

---

### User Story 5 - Two-dimensional state in status and list (Priority: P3)

A user runs `ws list` or `ws status` and sees clear, type-appropriate state information. For container workspaces, both infrastructure state and session state are shown. For local workspaces, only session state is shown (no confusing "available" vs "stopped" distinction).

**Why this priority**: Informational improvement. Important for usability but does not block core workflows.

**Independent Test**: Create workspaces of different types, put them in various states, run `ws list`, verify the output shows the correct state per type.

**Acceptance Scenarios**:

1. **Given** a local workspace with a Zellij session, **When** the user runs `cc-deck ws list`, **Then** the status shows "session: exists" (not "running").
2. **Given** a local workspace with no Zellij session, **When** the user runs `cc-deck ws list`, **Then** the status shows "no session" (not "stopped" or "available").
3. **Given** a container workspace with infra running and a session, **When** the user runs `cc-deck ws list`, **Then** the status shows "running, session: exists".
4. **Given** a container workspace with infra stopped, **When** the user runs `cc-deck ws list`, **Then** the status shows "stopped".

---

### Edge Cases

- What happens when `ws kill-session` is called while the user is attached to that session? The session is killed and the user's terminal returns to their shell.
- What happens when `ws stop` is called while someone is attached? The Zellij session is killed (disconnecting the user), then infrastructure is stopped.
- What happens when `ws attach` is called for a workspace whose infrastructure failed to start? An error is reported with the infrastructure failure details. No Zellij session is created.
- What happens when `ws kill-session` targets a container that is stopped? A message indicates no session exists (the session cannot exist without running infrastructure).
- What happens when reconciliation runs? It checks both infrastructure state (via podman inspect, kubectl, etc.) and session state (via zellij list-sessions) independently and updates both dimensions.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The `Workspace` interface MUST include `Attach()`, `KillSession()`, `Delete()`, and `Status()` methods that all workspace types implement.
- **FR-002**: An `InfraManager` interface MUST define `Start()` and `Stop()` methods, implemented only by workspace types that manage infrastructure (container, compose, k8s-deploy).
- **FR-003**: `Attach()` MUST lazily start infrastructure (if the workspace implements `InfraManager` and infrastructure is stopped) before creating or reattaching to a Zellij session.
- **FR-004**: `Attach()` MUST create a new Zellij session with the cc-deck layout when no session exists.
- **FR-005**: `Attach()` MUST reattach to an existing Zellij session when one exists, preserving all panes and tabs.
- **FR-006**: `KillSession()` MUST kill only the Zellij session without affecting infrastructure state.
- **FR-007**: `Stop()` (InfraManager) MUST kill the Zellij session first, then stop the infrastructure.
- **FR-008**: `Start()` (InfraManager) MUST start infrastructure only, without creating a Zellij session.
- **FR-009**: When `start` or `stop` is invoked on a workspace that does not implement `InfraManager`, the CLI MUST print a warning message indicating the command is not applicable and suggest the correct alternative.
- **FR-010**: The state model MUST track two independent dimensions: `infra_state` (for InfraManager types) and `session_state` (for all types).
- **FR-011**: `infra_state` MUST be omitted (null) for workspace types that do not implement `InfraManager`.
- **FR-012**: `session_state` MUST have two values: `none` (no Zellij session exists) and `exists` (a Zellij session exists, whether attached or detached).
- **FR-013**: Reconciliation MUST check infrastructure state and session state independently.
- **FR-014**: The `ws list` and `ws status` commands MUST display state appropriate to the workspace type (no confusing labels for non-InfraManager types).
- **FR-015**: `Delete()` MUST kill the session, stop infrastructure (if applicable), remove resources, and clean up state.
- **FR-016**: The existing `Start()` and `Stop()` methods MUST be removed from the base `Workspace` interface and moved to the `InfraManager` interface. The current `LocalWorkspace.Start()` behavior (creating a bare Zellij session without layout) MUST be eliminated.
- **FR-017**: A new `ws kill-session` CLI command MUST be added to the workspace command group.
- **FR-018**: State file migration MUST convert the old single `state` field to the new `infra_state`/`session_state` model on first read.

### Key Entities

- **Workspace**: A named development environment with a type, definition, and runtime state. All workspaces support session management (attach, kill-session). Some also manage infrastructure.
- **InfraManager**: A capability implemented by workspace types that manage compute resources (containers, pods, compose stacks). Provides start/stop operations.
- **Zellij Session**: A terminal multiplexer session with panes, tabs, and layout state. Created by `attach`, killed by `kill-session` or `stop`. Survives detach, does not survive kill.
- **WorkspaceStatus**: Two-dimensional state consisting of `infra_state` (running/stopped/error, or null) and `session_state` (none/exists).

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users can attach to any workspace type and get the cc-deck layout on the first attach, every time.
- **SC-002**: Users can reset a Zellij session inside a running container without restarting the container.
- **SC-003**: A single `ws attach` command connects the user to any workspace regardless of its current state (stopped infrastructure, no session, existing session).
- **SC-004**: The `ws list` output shows unambiguous, type-appropriate state for every workspace.
- **SC-005**: Running `ws start` or `ws stop` on a local or SSH workspace produces a clear, helpful warning instead of an error or silent no-op.

## Clarifications

### Session 2026-04-25

- Q: How should `ws attach` communicate progress during lazy infrastructure startup? → A: Reuse existing startup progress messages already implemented in Create/Start flows. No new UX pattern needed.

## Assumptions

- The cc-deck layout file is installed via `cc-deck plugin install` and is available to Zellij by name. If not installed, fallback to default layout with a warning (existing behavior, unchanged).
- The Zellij session naming convention (`cc-deck-<name>`) remains unchanged.
- Session state detection relies on `zellij list-sessions` (local), `podman exec ... zellij list-sessions` (container), `kubectl exec ... zellij list-sessions` (k8s), or `ssh ... zellij list-sessions` (SSH). These are already used in the current implementation.
- The state file format change (single state to two dimensions) is a breaking change handled by migration on first read. No backward compatibility shim is needed.
- This is a clean break in CLI semantics. No deprecation period. Documentation and release notes communicate the changes.
- The behavioral contract for the `Workspace` interface (constitution Principle VII) will be updated to reflect the new lifecycle semantics.
