# Feature Specification: Container Environment

**Feature Branch**: `024-container-env`
**Created**: 2026-03-20
**Status**: Draft
**Input**: Brainstorm: "025-container-environment.md" - single podman run lifecycle, definition/state separation, podman interaction layer

## Context

cc-deck manages agent sessions across different execution environments. Spec 023 established the Environment interface and a local (host-only) implementation. The next step is running agent sessions inside isolated containers, giving users a self-contained Zellij instance with its own filesystem, credentials, and lifecycle, without requiring Kubernetes or multi-container orchestration.

This spec covers the `container` environment type, which uses a single `podman run` container. A separate `compose` type (for multi-container setups with network filtering and sidecars) builds on top of this foundation in a later spec.

This spec also introduces the definition/state separation: environment definitions (human-editable, declarative) live in `environments.yaml`, while runtime state (machine-managed) remains in `state.yaml`. This separation enables users to hand-edit their environment configurations and version-control them.

### Spec Context (from brainstorm)

| Spec | Scope | Priority |
|------|-------|----------|
| 023 (done) | Interface definitions, `cc-deck env` commands, `state.yaml`, local environment | High (foundation) |
| **024 (this)** | Container environment (`podman run`), definition/state separation, podman package | High |
| 026 | Compose environment (`podman-compose`, network filtering, sidecars) | Medium |
| (future) | K8s environment refactor, K8s sandbox | Lower |

## User Scenarios & Testing

### User Story 1 - Create and Attach to a Container Environment (Priority: P1)

A developer wants to create an isolated container running Zellij with the cc-deck sidebar, attach to it interactively, and later delete it. The container uses a pre-built image (such as the cc-deck demo image) and persists workspace data in a named volume.

**Why this priority**: This is the core lifecycle. Without create/attach/delete, no other container operations are possible. It validates the entire podman interaction layer.

**Independent Test**: Run `cc-deck env create mydev --type container --image quay.io/cc-deck/cc-deck-demo:latest`, then `cc-deck env attach mydev` to verify an interactive Zellij session starts inside the container.

**Acceptance Scenarios**:

1. **Given** podman is installed and no environment named "mydev" exists, **When** the user runs `cc-deck env create mydev --type container --image quay.io/cc-deck/cc-deck-demo:latest`, **Then** a container named `cc-deck-mydev` is created and running, a named volume `cc-deck-mydev-data` is mounted at `/workspace`, and the environment record appears in both `environments.yaml` and `state.yaml`.
2. **Given** a running container environment "mydev" exists, **When** the user runs `cc-deck env attach mydev`, **Then** an interactive terminal opens into the container's Zellij session.
3. **Given** a running container environment "mydev" exists, **When** the user runs `cc-deck env delete mydev`, **Then** the container is stopped and removed, the named volume is deleted, and the records are removed from both `environments.yaml` and `state.yaml`.
4. **Given** a running container environment "mydev" exists, **When** the user runs `cc-deck env delete mydev --keep-volumes`, **Then** the container is removed but the named volume is preserved for later reuse.
5. **Given** no `--image` flag is provided and no config default exists, **When** the user runs `cc-deck env create mydev --type container`, **Then** the demo image is used as fallback and a warning is shown suggesting the user configure a default.

---

### User Story 2 - Stop and Restart a Container (Priority: P1)

A developer wants to stop a running container to free resources and restart it later without losing workspace data.

**Why this priority**: Stop/start is essential for resource management. The named volume preserves data across container restarts.

**Independent Test**: Run `cc-deck env stop mydev`, verify the container stops. Run `cc-deck env start mydev`, verify the container resumes and the workspace data is intact.

**Acceptance Scenarios**:

1. **Given** a running container environment, **When** the user runs `cc-deck env stop mydev`, **Then** the container is stopped and the state updates to "stopped" in `state.yaml`.
2. **Given** a stopped container environment, **When** the user runs `cc-deck env start mydev`, **Then** the container resumes and the state updates to "running".
3. **Given** a stopped container environment, **When** the user runs `cc-deck env attach mydev`, **Then** the system starts the container first and then attaches.

---

### User Story 3 - List and Inspect Container Environments (Priority: P1)

A developer wants to see all their environments (local and container) in one list with accurate status, and inspect a specific environment for detailed information.

**Why this priority**: Visibility is required for managing multiple environments. Reconciliation ensures the displayed status matches reality.

**Independent Test**: Create a container environment, then run `cc-deck env list` to verify it appears with correct type and status. Run `cc-deck env status mydev` for detailed information.

**Acceptance Scenarios**:

1. **Given** both local and container environments exist, **When** the user runs `cc-deck env list`, **Then** all environments are shown with correct type, status, storage, and age, with container statuses reconciled against actual podman state.
2. **Given** a container was stopped externally (via `podman stop`), **When** the user runs `cc-deck env list`, **Then** the environment shows status "stopped" (reconciled from podman).
3. **Given** a running container environment, **When** the user runs `cc-deck env status mydev`, **Then** detailed information is shown including container image, ports, storage type, uptime, and agent session states (read via exec into the container).

---

### User Story 4 - Transfer Files To and From a Container (Priority: P2)

A developer wants to push local project files into the container workspace and pull results back to the host.

**Why this priority**: File transfer is essential for the development workflow but the environment is functional without it (users can clone repos inside the container).

**Independent Test**: Run `cc-deck env push mydev ./src` to copy files in, then `cc-deck env pull mydev /workspace/results ./results` to copy files out.

**Acceptance Scenarios**:

1. **Given** a running container environment, **When** the user runs `cc-deck env push mydev ./my-project`, **Then** the local directory is copied into the container at `/workspace/my-project`.
2. **Given** a running container environment with files in `/workspace`, **When** the user runs `cc-deck env pull mydev /workspace/output ./local-output`, **Then** the remote files are copied to the local directory.
3. **Given** a stopped container environment, **When** the user runs `cc-deck env push mydev ./src`, **Then** the system reports an error indicating the container must be running.

---

### User Story 5 - Inject Credentials Securely (Priority: P2)

A developer wants their API keys (Anthropic, Vertex AI) injected into the container securely, without exposing them in `podman inspect` output or process tables.

**Why this priority**: Credentials are required for agent sessions to function, but the container can be created and explored without them.

**Independent Test**: Create an environment with `--credential ANTHROPIC_API_KEY=$ANTHROPIC_API_KEY`, attach, and verify the key is available inside the container as an environment variable.

**Acceptance Scenarios**:

1. **Given** the user provides `--credential ANTHROPIC_API_KEY=sk-ant-...`, **When** the environment is created, **Then** a podman secret is created and mounted into the container, and the key is available as an environment variable inside.
2. **Given** the host has `ANTHROPIC_API_KEY` set and no `--credential` flag is provided, **When** the environment is created, **Then** the host's API key is auto-detected and injected via podman secret.
3. **Given** credentials are injected via secrets, **When** the user runs `podman inspect cc-deck-mydev`, **Then** the API key value is not visible in the inspect output.

---

### User Story 6 - Execute Commands Inside a Container (Priority: P3)

A developer wants to run one-off commands inside the container without attaching to the full Zellij session.

**Why this priority**: Convenience operation for quick tasks. The full Zellij attach already provides interactive access.

**Independent Test**: Run `cc-deck env exec mydev -- git status` and verify the output.

**Acceptance Scenarios**:

1. **Given** a running container environment, **When** the user runs `cc-deck env exec mydev -- ls /workspace`, **Then** the command output is displayed on the host terminal.
2. **Given** a stopped container environment, **When** the user runs `cc-deck env exec mydev -- ls`, **Then** an error indicates the environment must be running.

---

### User Story 7 - Edit Environment Definitions by Hand (Priority: P3)

A developer wants to change the image or storage configuration of an environment by editing the `environments.yaml` file directly, without re-creating the environment through CLI commands.

**Why this priority**: Power-user workflow. The CLI handles all creation, but hand-editing enables batch changes and version control.

**Independent Test**: Edit the image field in `environments.yaml` for an existing stopped environment, then `cc-deck env delete mydev && cc-deck env create mydev` to pick up the new definition.

**Acceptance Scenarios**:

1. **Given** an environment definition exists in `environments.yaml`, **When** the user edits the image field and re-creates the environment, **Then** the new image is used.
2. **Given** an environment definition exists in `environments.yaml`, **When** the user runs `cc-deck env list`, **Then** definitions and state are joined correctly for display.

---

### Edge Cases

- What happens when podman is not installed or not in PATH? The system fails fast with a clear error and installation instructions.
- What happens when the specified image does not exist or cannot be pulled? The system reports the podman pull error and does not create a state record.
- What happens when the container is deleted externally (via `podman rm`)? Reconciliation detects the missing container and updates the state to "error". The user can then delete the stale record with `cc-deck env delete`.
- What happens when disk space is exhausted during container creation? The system reports the podman error. Any partially created resources (volume, secret) are cleaned up.
- What happens when rootless podman is in use? The system auto-detects rootless mode and adapts socket paths and permissions accordingly.
- What happens when the user specifies `--storage host-path` without `--path`? The system uses the current working directory as the bind mount source.
- What happens when two environments try to use the same container name? The name is derived from the environment name, which is already validated as unique in the state store.

## Requirements

### Functional Requirements

- **FR-001**: System MUST implement the `container` environment type using `podman run` for single-container lifecycle management.
- **FR-002**: System MUST separate environment definitions (declarative, human-editable) from runtime state (machine-managed) into two files: `$XDG_CONFIG_HOME/cc-deck/environments.yaml` for definitions and `$XDG_STATE_HOME/cc-deck/state.yaml` for runtime state.
- **FR-003**: System MUST provide a shared podman interaction layer used by the container implementation (and future compose implementation).
- **FR-004**: System MUST support two storage backends for container environments: named volume (default, created as `cc-deck-<name>-data`) and host bind mount (via `--storage host-path --path <dir>`).
- **FR-005**: System MUST run containers with `sleep infinity` as the entrypoint command, keeping them alive independently of interactive sessions.
- **FR-006**: System MUST attach to container environments via `podman exec -it cc-deck-<name> zellij attach cc-deck --create`, where the Zellij session inside the container is always named `cc-deck`.
- **FR-007**: System MUST inject credentials using podman secrets (not environment variables), so that sensitive values are not visible in `podman inspect` output. Credential definitions in `environments.yaml` store only key names (e.g., `credentials: [ANTHROPIC_API_KEY]`), never secret values. Values are resolved at runtime from the host environment or `--credential KEY=VALUE` flags.
- **FR-008**: System MUST auto-detect host environment variables (`ANTHROPIC_API_KEY`, `GOOGLE_APPLICATION_CREDENTIALS`) when no explicit `--credential` flags are provided.
- **FR-009**: System MUST support file transfer via `podman cp` for push (host to container) and pull (container to host) operations.
- **FR-010**: System MUST reconcile container environments against actual podman state when listing or inspecting, using `podman inspect` to determine whether containers are running, stopped, or missing.
- **FR-011**: System MUST auto-detect rootless podman mode and adapt socket paths and behavior accordingly.
- **FR-012**: System MUST expose ports only when explicitly requested via `--port host:container` (repeatable) or `--all-ports` flags. No ports are exposed by default. The `--all-ports` flag maps to `podman run -P`, publishing all ports declared via EXPOSE directives in the container image.
- **FR-013**: System MUST delete named volumes by default when deleting an environment. The `--keep-volumes` flag preserves volumes.
- **FR-014**: System MUST clean up all associated resources (container, volume, secrets) when deleting an environment. Partial cleanup failures are reported as warnings, not errors.
- **FR-015**: System MUST fall back to `quay.io/cc-deck/cc-deck-demo:latest` as the default image when no `--image` flag and no config default is provided, showing a warning to the user.
- **FR-016**: System MUST reject operations that are not applicable to the container type (such as network filtering or git harvest) with a clear message suggesting the appropriate environment type.
- **FR-017**: System MUST support config-file defaults for container environments (image, storage type) in the existing `config.yaml`.
- **FR-018**: System MUST automatically start a stopped container when the user runs `cc-deck env attach` on it, rather than requiring a separate start command.

### Key Entities

- **Environment Definition**: The declarative, user-editable description of an environment stored in `environments.yaml`. Contains name, type, image, storage configuration, ports, and credential references. Persists across environment lifecycle (create/delete/recreate).
- **Environment Instance State**: The runtime state of a created environment stored in `state.yaml`. Contains current state (running/stopped/error), timestamps, container ID, and container name. Updated automatically by lifecycle operations.
- **Container**: A podman container named `cc-deck-<env-name>` running a cc-deck image with `sleep infinity`. Each container runs its own Zellij instance with a session named `cc-deck`.
- **Named Volume**: A podman volume named `cc-deck-<env-name>-data` mounted at `/workspace` inside the container. Persists workspace data across stop/start cycles.
- **Podman Secret**: A credential stored via `podman secret create`, mounted into the container as an environment variable. Named `cc-deck-<env-name>-<key-name>`.

## Success Criteria

### Measurable Outcomes

- **SC-001**: Users can create a container environment and attach to a running Zellij session inside it in under 30 seconds (excluding image pull time).
- **SC-002**: Users can stop and restart an environment with workspace data intact, verified by file presence after restart.
- **SC-003**: `cc-deck env list` reconciles container status and displays results in under 2 seconds, regardless of the number of tracked environments.
- **SC-004**: Credentials injected via podman secrets are not visible in `podman inspect` output.
- **SC-005**: File transfer (push/pull) works for directories up to 100MB without errors.
- **SC-006**: Adding a future `compose` environment type requires only a new implementation file, not changes to the definition/state schema or CLI command structure.
- **SC-007**: Users can hand-edit `environments.yaml` to change environment configuration, and the system respects those changes on the next lifecycle operation.

## Assumptions

- The user has podman installed and accessible in PATH. The system checks this and fails fast with instructions if not.
- The container image contains Zellij, the cc-deck plugin, and an agent (Claude Code). The cc-deck demo image satisfies this.
- Named volumes are the default storage because they offer better isolation and portability than bind mounts.
- The `environments.yaml` file uses a single-file format (not one-file-per-environment) since most users will have fewer than 10 environments.
- Rootless podman is the expected default on developer workstations. The system auto-detects and adapts.
- The existing `config.yaml` continues to hold global defaults and profiles. `environments.yaml` is a new file alongside it.
- Git harvest sync is deferred to a separate spec (see `brainstorm/026-git-harvest-sync.md`).
- Network filtering is not possible with a single container and is deferred to the compose environment type.
- Environment templates (reusable definitions for quick creation) are deferred to a future spec.

## Scope Boundaries

**In scope for this spec:**
- `ContainerEnvironment` implementing the Environment interface
- Definition/state file separation (`environments.yaml` + `state.yaml`)
- Shared `internal/podman/` interaction package
- `ContainerFields` type (replaces `PodmanFields`)
- Named volume and bind mount storage
- Credential injection via podman secrets
- File transfer via `podman cp`
- Reconciliation via `podman inspect`
- Rootless podman auto-detection
- CLI flags: `--image`, `--port`, `--all-ports`, `--storage`, `--path`, `--credential`, `--keep-volumes`

**Out of scope (separate specs):**
- Compose environment with multi-container orchestration (spec 026)
- Network filtering via proxy sidecar (compose spec)
- MCP container sidecars (future)
- Git harvest sync (see `brainstorm/026-git-harvest-sync.md`)
- Environment templates
- Resource limits (CPU/memory)
- Port forwarding for web UI

## Clarifications

### Session 2026-03-20

- Q: What does `--all-ports` expose? → A: Image EXPOSE directives via `podman run -P` (standard OCI convention)
- Q: How are credentials represented in `environments.yaml`? → A: Key names only (resolved at runtime from host env or `--credential` flags), never secret values
