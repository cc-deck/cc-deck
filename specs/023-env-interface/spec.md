# Feature Specification: Environment Interface and CLI

**Feature Branch**: `023-env-interface`
**Created**: 2026-03-20
**Status**: Draft
**Input**: User description: "the remote env specification that we brainstormed"

## Context

cc-deck currently has two separate, incompatible code paths for running agent sessions: local (user manages Zellij manually) and K8s (`cc-deck deploy` with hardcoded StatefulSet logic). There is no Podman container mode despite a full image build pipeline, no unified interface for environment lifecycle, and no local state tracking. Adding a new environment type requires touching multiple packages.

This spec defines the foundation layer: a unified Environment interface, the `cc-deck env` CLI command group, local state persistence, and a Local environment implementation (thin wrapper). Concrete remote implementations (Podman, K8s refactor, K8s sandbox) are separate specs that build on this interface.

### Spec Split (from brainstorm)

| Spec | Scope | Priority |
|------|-------|----------|
| **023 (this)** | Interface definitions, `cc-deck env` commands, `state.yaml`, local environment | High (foundation) |
| **024** | K8s environment refactor behind Environment interface | Medium |
| **025** | Podman environment implementation | High |
| **026** | K8s sandbox (ephemeral Pods) | Lower |

## User Scenarios & Testing

### User Story 1 - List All Environments (Priority: P1)

A developer wants to see all their cc-deck environments (local, container, K8s) in one place, with current status, to decide which one to attach to or manage.

**Why this priority**: Without visibility into existing environments, users cannot manage them. This is the entry point for all environment operations.

**Independent Test**: Run `cc-deck env list` and verify it shows tracked environments with type, status, storage, and age columns.

**Acceptance Scenarios**:

1. **Given** no environments are tracked, **When** the user runs `cc-deck env list`, **Then** the output shows an empty table with column headers and a hint about creating environments.
2. **Given** multiple environments of different types exist, **When** the user runs `cc-deck env list`, **Then** each environment is shown with its name, type, status, storage type, last-attached time, and age.
3. **Given** some environments exist, **When** the user runs `cc-deck env list --type podman`, **Then** only Podman environments are shown.

---

### User Story 2 - Create and Attach to a Local Environment (Priority: P1)

A developer wants to register their local Zellij session as a tracked environment so it appears in `cc-deck env list` alongside remote environments.

**Why this priority**: The local environment is the simplest implementation and validates the entire interface contract without external dependencies.

**Independent Test**: Run `cc-deck env create mydev --type local`, then `cc-deck env list` and verify it appears. Run `cc-deck env attach mydev` and verify Zellij starts or attaches.

**Acceptance Scenarios**:

1. **Given** no local environment is tracked, **When** the user runs `cc-deck env create mydev --type local`, **Then** a local environment record is persisted to the state file.
2. **Given** a local environment named "mydev" exists, **When** the user runs `cc-deck env attach mydev`, **Then** Zellij starts with the cc-deck layout (or attaches to the existing session).
3. **Given** a local environment named "mydev" exists, **When** the user runs `cc-deck env delete mydev`, **Then** the environment record is removed from the state file.

---

### User Story 3 - Inspect Environment Details (Priority: P2)

A developer wants to see detailed information about a specific environment, including agent session states when the environment is running.

**Why this priority**: Detailed status provides actionable information (which sessions need attention) without requiring attachment.

**Independent Test**: Run `cc-deck env status mydev` and verify it shows environment metadata. For running environments, verify it also shows agent session states.

**Acceptance Scenarios**:

1. **Given** a running local environment, **When** the user runs `cc-deck env status mydev`, **Then** the output shows type, status, storage, uptime, and agent session states.
2. **Given** a stopped environment, **When** the user runs `cc-deck env status mydev`, **Then** the output shows type, status as "stopped", and last-attached time, without attempting to read session states.

---

### User Story 4 - Stop and Restart an Environment (Priority: P2)

A developer wants to stop an environment to free resources and restart it later with preserved state.

**Why this priority**: Stop/start enables resource management for container and K8s environments. For local, stop is a no-op (documented as such).

**Independent Test**: Run `cc-deck env stop myenv` and verify the environment transitions to stopped. Run `cc-deck env start myenv` and verify it resumes.

**Acceptance Scenarios**:

1. **Given** a running environment, **When** the user runs `cc-deck env stop myenv`, **Then** the environment transitions to "stopped" and the state file is updated.
2. **Given** a stopped environment, **When** the user runs `cc-deck env start myenv`, **Then** the environment resumes and transitions to "running".
3. **Given** a local environment, **When** the user runs `cc-deck env stop mydev`, **Then** the command reports that stop is not supported for local environments.

---

### User Story 5 - Backward-Compatible Aliases (Priority: P3)

A developer who uses the existing `cc-deck deploy`/`connect`/`delete` commands wants them to continue working during the transition to `cc-deck env`.

**Why this priority**: Avoids breaking existing workflows. Can be implemented as thin wrappers.

**Independent Test**: Run `cc-deck deploy ...` and verify it delegates to `cc-deck env create --type k8s`.

**Acceptance Scenarios**:

1. **Given** the new `cc-deck env` commands exist, **When** the user runs `cc-deck deploy`, **Then** it delegates to `cc-deck env create --type k8s` with equivalent flags.
2. **Given** the new `cc-deck env` commands exist, **When** the user runs `cc-deck connect`, **Then** it delegates to `cc-deck env attach`.

---

### Edge Cases

- What happens when the user creates an environment with a name that already exists? The system rejects with a clear error message.
- What happens when `state.yaml` is corrupted or missing? The system initializes a fresh state file and warns the user.
- What happens when `cc-deck env status` runs against an environment whose container/pod was deleted externally? The system detects the missing resource, updates state to "error" or removes the record, and reports the discrepancy.
- What happens when `cc-deck env attach` targets a stopped environment? The system offers to start it first or reports an error.
- What happens when two `cc-deck` processes modify `state.yaml` concurrently? The system uses atomic writes (write-temp-rename) to prevent corruption.

## Requirements

### Functional Requirements

- **FR-001**: System MUST define an Environment interface with lifecycle methods (Create, Start, Stop, Delete, Status), interaction methods (Attach, Exec), and data transfer methods (Push, Pull, Harvest).
- **FR-002**: System MUST define environment types: Local, Podman, K8sDeploy, K8sSandbox.
- **FR-003**: System MUST persist environment records in a state file at `$XDG_CONFIG_HOME/cc-deck/state.yaml`, separate from user configuration.
- **FR-004**: System MUST provide a `cc-deck env` command group with subcommands: `create`, `attach`, `start`, `stop`, `delete`, `list`, `status`, `exec`, `push`, `pull`, `harvest`. Subcommands that only apply to remote environments (`exec`, `push`, `pull`, `harvest`) MUST be registered in the CLI but return "not supported for local environments" until concrete environment implementations land.
- **FR-005**: System MUST implement a Local environment type as a thin wrapper: Create registers the record, Attach starts or attaches to Zellij with the cc-deck layout, Stop/Start are no-ops with clear messaging, Delete removes the record.
- **FR-006**: System MUST reconcile local state with actual runtime status when listing or inspecting environments. For Local environments, reconciliation means checking whether a Zellij session with the expected name exists. Remote environment reconciliation (container/pod existence) is delegated to each environment implementation in its own spec.
- **FR-007**: System MUST support filtering `cc-deck env list` by environment type via `--type` flag.
- **FR-008**: System MUST provide backward-compatible aliases for existing commands (`deploy` -> `env create --type k8s`, `connect` -> `env attach`, `delete` -> `env delete`, `list` -> `env list`, `logs` -> `env logs`). Aliases delegate to the `env` command group. If the target environment type is not yet implemented, the alias reports the same error as the underlying `env` subcommand.
- **FR-009**: System MUST reject environment creation when a name conflicts with an existing environment.
- **FR-010**: System MUST require `--type` flag for `cc-deck env create` (no auto-inference).
- **FR-011**: System MUST define storage and sync interfaces as part of the Environment contract, with type enums (HostPath, NamedVolume, EmptyDir, PVC for storage; Copy, GitHarvest, RemoteGit for sync).
- **FR-012**: System MUST display environment status using a structured format showing name, type, state, storage, last-attached time, and age in list view.
- **FR-013**: System MUST support detailed status via `cc-deck env status <name>` that reads agent session states from inside running environments via exec.

### Key Entities

- **Environment**: A tracked execution context where a Zellij instance runs with cc-deck sidebar and agent sessions. Has a name, type, lifecycle state, storage configuration, and sync configuration.
- **EnvironmentState**: The lifecycle state of an environment (Running, Stopped, Creating, Error, Unknown).
- **StorageConfig**: Storage backend configuration (type, size, storage class, host path).
- **SyncConfig**: Data transfer strategy configuration (strategy, workspace path, excludes, git settings).
- **StateFile**: Persistent YAML file tracking all environment records with their metadata, storage, sync, and runtime-specific fields.

## Success Criteria

### Measurable Outcomes

- **SC-001**: Users can list all tracked environments in under 1 second (no exec calls).
- **SC-002**: Users can create and attach to a local environment in under 3 seconds.
- **SC-003**: Adding a new environment type requires implementing only the Environment interface, with no changes to CLI commands or state management.
- **SC-004**: All existing `cc-deck deploy`/`connect`/`delete` commands continue to work identically through aliases.
- **SC-005**: Environment state file survives concurrent access without corruption.
- **SC-006**: Detailed status for a running environment (including agent sessions) completes in under 5 seconds.

## Assumptions

- The `state.yaml` file is separate from `config.yaml` because environment state changes frequently (timestamps, container IDs) while config is user-edited.
- The `--type` flag is always required for `create` to keep the interface explicit. Per-project defaults via a config file are deferred.
- Port forwarding (`cc-deck env port-forward`) is deferred to a future spec.
- TUI environment manager is deferred (separate brainstorm at `brainstorm/024-tui-environment-manager.md`).
- Multi-user attach (pair-SDD) works via Zellij's native multiplayer support and requires no special handling in cc-deck.

## Scope Boundaries

**In scope for this spec:**
- Environment interface definition
- `cc-deck env` CLI command group
- `state.yaml` state file format and persistence
- Local environment implementation
- Backward-compatible aliases
- Storage and sync interface definitions (types/enums only, not implementations)

**Out of scope (separate specs):**
- Podman environment implementation (spec 025)
- K8s environment refactor (spec 024)
- K8s sandbox implementation (spec 026)
- TUI environment manager
- Port forwarding
- Per-project configuration file (`cc-deck.yaml`)
