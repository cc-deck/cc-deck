# Feature Specification: Workspace Channels

**Feature Branch**: `041-workspace-channels`
**Created**: 2026-04-21
**Status**: Draft
**Input**: Brainstorm 040 - Unified local-remote transport abstraction

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Send Text Commands to Remote Workspace (Priority: P1)

A developer working locally needs to send text data to a running remote workspace session. For example, a voice transcription tool produces text that should be injected into a focused pane in the remote Zellij session, or a local automation script triggers a plugin action remotely. Today, each feature that needs this capability builds its own exec-based plumbing. With PipeChannel, the developer (or any local tool) sends text through a single, consistent interface regardless of whether the workspace runs in a container, on a remote SSH host, or in a Kubernetes Pod.

**Why this priority**: Text command relay is the simplest channel type (unidirectional, small payloads) and unlocks voice relay, remote plugin control, and local-to-remote automation. It validates the core channel abstraction with minimal complexity.

**Independent Test**: Can be fully tested by sending a text payload to a named zellij pipe in a running workspace and verifying the plugin receives it. Delivers value immediately for any tool that needs to inject text into remote sessions.

**Acceptance Scenarios**:

1. **Given** a running container workspace, **When** a local tool sends "hello world" via PipeChannel to pipe name "cc-deck:voice", **Then** the text arrives at the remote zellij pipe within the workspace and the plugin can process it.
2. **Given** a running SSH workspace, **When** a local tool sends text via PipeChannel, **Then** the text arrives at the remote zellij pipe using the SSH connection to the remote host.
3. **Given** a running K8s Deploy workspace, **When** a local tool sends text via PipeChannel, **Then** the text arrives via kubectl exec into the correct Pod and namespace.
4. **Given** a local workspace, **When** a local tool sends text via PipeChannel, **Then** the text is delivered via a local zellij pipe command (no remote transport needed).
5. **Given** a workspace that is not running, **When** a local tool attempts to send text via PipeChannel, **Then** a clear error is returned indicating the workspace is unavailable.

---

### User Story 2 - Transfer Files to and from Remote Workspace (Priority: P2)

A developer needs to move files between their local machine and a remote workspace. This covers pushing clipboard images to a staging directory, uploading configuration files, or downloading build artifacts. Today, each consumer (clipboard bridge, file sync) constructs its own copy commands per workspace type. With DataChannel, any tool pushes or pulls files through a uniform interface. The existing `Push()` and `Pull()` methods on the Workspace interface delegate to DataChannel internally, eliminating duplicated transport code.

**Why this priority**: File transfer is used by multiple planned features (clipboard bridge, file sync, workspace provisioning) and refactoring existing `Push()`/`Pull()` to delegate to DataChannel eliminates duplicated transport code across workspace types.

**Independent Test**: Can be tested by pushing a file to a remote workspace path and pulling it back, then verifying the content matches. Works for all workspace types.

**Acceptance Scenarios**:

1. **Given** a running container workspace, **When** a local tool pushes a file via DataChannel, **Then** the file appears at the specified remote path inside the container.
2. **Given** a running SSH workspace, **When** a local tool pushes a file via DataChannel, **Then** the file is transferred to the remote host at the specified path.
3. **Given** a running K8s Deploy workspace, **When** a local tool pulls a file via DataChannel, **Then** the file is transferred from the Pod to the specified local path.
4. **Given** a running workspace, **When** a local tool pushes raw bytes (e.g., clipboard image data) via DataChannel with a target path, **Then** the bytes are written to a file at the specified remote path.
5. **Given** the existing `Push()` method on any workspace type, **When** a user calls `cc-deck ws push`, **Then** the operation delegates to DataChannel internally (same behavior, consolidated code path).
6. **Given** the existing `Pull()` method on any workspace type, **When** a user calls `cc-deck ws pull`, **Then** the operation delegates to DataChannel internally.
7. **Given** a local workspace, **When** a tool pushes or pulls via DataChannel, **Then** the operation uses direct filesystem copy (no exec overhead).

---

### User Story 3 - Synchronize Git Commits with Remote Workspace (Priority: P3)

A developer needs to push local commits into a remote workspace or harvest commits made by an agent in the remote workspace back to the local machine. Today, `k8s_sync.go` constructs ext:: URLs and manages temporary git remotes with ~90 lines of plumbing. With GitChannel, the ext:: URL construction and temporary remote management are encapsulated per workspace type. The existing `Harvest()` method delegates to GitChannel internally.

**Why this priority**: Git sync is essential for the agent workflow (harvest agent commits) but is less frequently used than text relay or file transfer. The existing code in `k8s_sync.go` already works; this story consolidates it into the channel abstraction.

**Independent Test**: Can be tested by pushing a commit to a remote workspace and fetching it back, verifying the commit history matches. Requires a workspace with git installed.

**Acceptance Scenarios**:

1. **Given** a running container workspace with a git repository, **When** a local tool pushes commits via GitChannel, **Then** the commits appear in the remote repository using the ext:: protocol over podman exec.
2. **Given** a running K8s Deploy workspace, **When** a local tool fetches commits via GitChannel, **Then** the commits are retrieved using ext:: over kubectl exec.
3. **Given** a running SSH workspace, **When** a local tool pushes commits via GitChannel, **Then** the commits are transferred using ext:: over SSH.
4. **Given** the existing `Harvest()` method, **When** a user calls `cc-deck ws harvest`, **Then** the operation delegates to GitChannel internally (same behavior, consolidated code path).
5. **Given** a local workspace, **When** a local tool attempts a git channel operation, **Then** the operation returns a clear indication that git channels are not applicable for local workspaces (same filesystem).

---

### Edge Cases

- What happens when the remote workspace restarts mid-transfer (Pod eviction, container restart, SSH disconnect)?
- How does the system handle concurrent channel operations to the same workspace from multiple local processes?
- What happens when the target zellij session or pipe name does not exist in the remote workspace?
- How does the system handle large file transfers that exceed available memory (streaming vs. buffering)?
- What happens when the remote path for DataChannel does not exist or lacks write permissions?
- How does GitChannel handle merge conflicts when pushing to a remote branch that has diverged?

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST provide a PipeChannel interface that sends text payloads to a named zellij pipe in the remote workspace via a single method call.
- **FR-002**: The system MUST provide a DataChannel interface with explicit Push (local-to-remote) and Pull (remote-to-local) methods for file and binary data transfer.
- **FR-003**: The system MUST provide a DataChannel method that accepts raw bytes and a remote path, writing the bytes as a file at the destination (for clipboard images, generated data).
- **FR-004**: The system MUST provide a GitChannel interface that encapsulates ext:: URL construction and temporary git remote management per workspace type.
- **FR-005**: Each channel type MUST have a working implementation for all applicable workspace types: local, container, compose, SSH, k8s-deploy, and k8s-sandbox. GitChannel is not applicable for local workspaces (same filesystem, use standard git commands directly). PipeChannel and DataChannel apply to all workspace types.
- **FR-006**: The existing `Push()` and `Pull()` methods on the Workspace interface MUST be refactored to delegate to DataChannel internally, preserving identical external behavior.
- **FR-007**: The existing `Harvest()` method on the Workspace interface MUST be refactored to delegate to GitChannel internally, preserving identical external behavior.
- **FR-008**: Channels MUST be created on demand (lazy initialization) when first requested, not pre-created during workspace attach.
- **FR-009**: Channel errors MUST be wrapped in a structured error type that provides a human-readable summary while preserving the underlying error via standard unwrapping.
- **FR-010**: CLI commands MUST display the human-readable error summary by default and the full error chain when a verbose flag is active.
- **FR-011**: Each workspace type MUST use its native transport mechanism for channels (podman exec for containers, kubectl exec for K8s, SSH for remote hosts, filesystem for local).
- **FR-012**: The GitChannel MUST manage the full git workflow including temporary remote add, fetch/push, and remote cleanup, not just URL construction.

### Key Entities

- **Channel**: A typed communication path between the local machine and a remote workspace. Each channel type (Pipe, Data, Git) has a distinct interface tailored to its data transfer pattern.
- **ChannelError**: A structured error that wraps transport-level failures with a human-readable channel context (channel type, operation attempted, workspace state).
- **Workspace**: The existing abstraction for all workspace types, extended with channel accessor methods that return typed channel instances.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Text payloads sent via PipeChannel arrive at the remote zellij pipe within 500ms for container and local workspaces, and within 2 seconds for SSH and K8s workspaces.
- **SC-002**: File transfers via DataChannel complete at the same speed (within 10% variance) as the current direct copy/rsync/kubectl-cp implementations they replace.
- **SC-003**: Git operations via GitChannel produce identical results (same commits, same refs) as the current `k8s_sync.go` implementation.
- **SC-004**: After refactoring, each transport mechanism (exec-based pipe, file copy, git ext:: tunneling) is implemented once per workspace type in the channel layer, not duplicated across multiple features or methods.
- **SC-005**: All existing `cc-deck ws push`, `cc-deck ws pull`, and `cc-deck ws harvest` commands produce identical user-visible behavior after refactoring to use channels.
- **SC-006**: Channel error messages are understandable to users without requiring knowledge of the underlying transport (no raw kubectl or podman error output in default mode).

## Assumptions

- All workspace types already have a working `Exec()` implementation that the channel abstraction can build upon.
- The zellij pipe mechanism (`zellij pipe --name <name> --payload <data>`) is available and functional in all remote workspace environments where Zellij runs.
- Git is installed in remote workspaces that use GitChannel (this is already a requirement for the existing harvest workflow).
- The existing integration test infrastructure (from spec 016) provides access to real K8s clusters for testing channel implementations against K8s workspace types.
- SSH ControlMaster (connection multiplexing) is a performance optimization that will be addressed in a separate effort; channels work without it, just with higher per-call latency for SSH workspaces.
- Encryption of channel data (e.g., for clipboard images) is a consumer-level concern, not a channel-level concern. Channels transport bytes; consumers decide whether to encrypt before sending.
- Concurrent access to the same channel instance from multiple goroutines is expected (e.g., multiple consumers sending pipe messages). Channel implementations must be safe for concurrent use.
- Documentation updates (README, CLI reference) are required as part of feature completion per constitution principle IX. Since channels are an internal abstraction with no new CLI commands, documentation updates focus on updated architecture descriptions and any behavioral changes to existing commands.
