# Feature Specification: Workspace Channels

**Feature Branch**: `041-workspace-channels`
**Created**: 2026-04-21
**Status**: Draft
**Input**: [Brainstorm 040 - Workspace Channels (Unified Local-Remote Transport)](../../brainstorm/040-workspace-channels.md)

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Send Text Commands to Remote Workspace (Priority: P1)

A developer working locally needs to send text data to a running remote workspace session. For example, a voice transcription tool produces text that should be injected into a focused pane in the remote Zellij session, or a local automation script triggers a plugin action remotely. Today, each feature that needs this capability builds its own exec-based plumbing. With PipeChannel, the developer (or any local tool) sends text through a single, consistent interface regardless of whether the workspace runs in a container, on a remote SSH host, or in a Kubernetes Pod.

**Why this priority**: Text command relay is the simplest channel type (unidirectional, small payloads) and unlocks voice relay, remote plugin control, and local-to-remote automation. It validates the core channel abstraction with minimal complexity.

**Independent Test**: Can be fully tested by sending a text payload to a named zellij pipe in a running workspace and verifying the plugin receives it. Delivers value immediately for any tool that needs to inject text into remote sessions.

**Acceptance Scenarios**:

1. **Given** a running container workspace, **When** a local tool sends "hello world" via PipeChannel to pipe name "cc-deck:voice", **Then** the text arrives at the remote zellij pipe within the workspace and the plugin can process it.
2. **Given** a running SSH workspace, **When** a local tool sends text via PipeChannel, **Then** the text arrives at the remote zellij pipe using the SSH connection to the remote host.
3. **Given** a running K8s Deploy workspace, **When** a local tool sends text via PipeChannel, **Then** the text arrives via kubectl exec into the correct Pod and namespace.
4. **Given** a running compose workspace, **When** a local tool sends text via PipeChannel, **Then** the text arrives at the remote zellij pipe inside the compose service container.
5. **Given** a local workspace, **When** a local tool sends text via PipeChannel, **Then** the text is delivered via a local zellij pipe command (no remote transport needed).
6. **Given** a workspace that is not running, **When** a local tool attempts to send text via PipeChannel, **Then** a clear error is returned indicating the workspace is unavailable.

---

### User Story 2 - Transfer Files to and from Remote Workspace (Priority: P2)

A developer needs to move files between their local machine and a remote workspace. This covers pushing clipboard images to a staging directory, uploading configuration files, or downloading build artifacts. Today, each consumer (clipboard bridge, file sync) constructs its own copy commands per workspace type. With DataChannel, any tool pushes or pulls files through a uniform interface. The existing `Push()` and `Pull()` methods on the Workspace interface delegate to DataChannel internally, eliminating duplicated transport code.

**Why this priority**: File transfer is used by multiple planned features (clipboard bridge, file sync, workspace provisioning) and refactoring existing `Push()`/`Pull()` to delegate to DataChannel eliminates duplicated transport code across workspace types.

**Independent Test**: Can be tested by pushing a file to a remote workspace path and pulling it back, then verifying the content matches. Works for all workspace types.

**Acceptance Scenarios**:

1. **Given** a running container workspace, **When** a local tool pushes a file via DataChannel, **Then** the file appears at the specified remote path inside the container.
2. **Given** a running SSH workspace, **When** a local tool pushes a file via DataChannel, **Then** the file is transferred to the remote host at the specified path.
3. **Given** a running K8s Deploy workspace, **When** a local tool pulls a file via DataChannel, **Then** the file is transferred from the Pod to the specified local path.
4. **Given** a running compose workspace, **When** a local tool pushes a file via DataChannel, **Then** the file appears at the specified remote path inside the compose service container.
5. **Given** a running workspace, **When** a local tool pushes raw bytes (e.g., clipboard image data) via DataChannel with a target path, **Then** the bytes are written to a file at the specified remote path.
6. **Given** the existing `Push()` method on any workspace type, **When** a user calls `cc-deck ws push`, **Then** the operation delegates to DataChannel internally (same behavior, consolidated code path).
7. **Given** the existing `Pull()` method on any workspace type, **When** a user calls `cc-deck ws pull`, **Then** the operation delegates to DataChannel internally.
8. **Given** a local workspace, **When** a tool pushes or pulls via DataChannel, **Then** the operation uses direct filesystem copy (no exec overhead).

---

### User Story 3 - Synchronize Git Commits with Remote Workspace (Priority: P3)

A developer needs to push local commits into a remote workspace or harvest commits made by an agent in the remote workspace back to the local machine. Today, each workspace type constructs its own git transport plumbing with significant code duplication. With GitChannel, git synchronization is encapsulated per workspace type behind a uniform interface. The existing `Harvest()` method delegates to GitChannel internally.

**Why this priority**: Git sync is essential for the agent workflow (harvest agent commits) but is less frequently used than text relay or file transfer. The existing git sync already works; this story consolidates it into the channel abstraction.

**Independent Test**: Can be tested by pushing a commit to a remote workspace and fetching it back, verifying the commit history matches. Requires a workspace with git installed.

**Acceptance Scenarios**:

1. **Given** a running container workspace with a git repository, **When** a local tool pushes commits via GitChannel, **Then** the commits appear in the remote repository inside the container.
2. **Given** a running K8s Deploy workspace, **When** a local tool fetches commits via GitChannel, **Then** the commits are retrieved from the Pod into the local repository.
3. **Given** a running SSH workspace, **When** a local tool pushes commits via GitChannel, **Then** the commits are transferred to the remote host over the SSH connection.
4. **Given** a running compose workspace with a git repository, **When** a local tool pushes commits via GitChannel, **Then** the commits appear in the remote repository inside the compose service container.
5. **Given** the existing `Harvest()` method, **When** a user calls `cc-deck ws harvest`, **Then** the operation delegates to GitChannel internally (same behavior, consolidated code path).
6. **Given** a local workspace, **When** a local tool attempts a git channel operation, **Then** the operation returns a clear indication that git channels are not applicable for local workspaces (same filesystem).

---

### Edge Cases

- **Workspace restarts mid-transfer** (Pod eviction, container restart, SSH disconnect): The in-flight channel operation fails with a clear error indicating the workspace became unavailable. No automatic retry; the user re-runs the command. The channel reference itself remains valid and transparently reconnects once the workspace is running again (no need to discard and re-obtain the channel).
- **Concurrent channel operations** from multiple local processes to the same workspace: Each operation runs independently. The system does not coordinate across processes; concurrent writes to the same remote file or pipe produce undefined ordering (consistent with existing Exec behavior).
- **Target zellij pipe name does not exist** in the remote workspace: PipeChannel returns an error indicating the pipe name was not found. The user is told to verify the workspace has an active Zellij session with the expected plugin.
- **Large file transfers** that exceed available memory: DataChannel uses the same transfer mechanism as the current Push/Pull operations. Transfer size limits are inherited from the underlying workspace transport.
- **Remote path does not exist or lacks write permissions**: DataChannel returns an error with the remote path and the permission or "not found" cause. The user must create the directory or fix permissions before retrying.
- **Git merge conflicts** when pushing to a diverged remote branch: GitChannel fails the push and reports the conflict. The user must resolve the divergence manually before retrying (consistent with standard git behavior).

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST provide a PipeChannel interface that sends text payloads from the local machine to a named zellij pipe in the remote workspace. The interface MUST use a request-response pattern (accepting a request and returning a response) so that future features can receive data back from the remote workspace without requiring a new channel type. For this feature, only the send direction is implemented; responses are not used.
- **FR-002**: The system MUST provide a DataChannel interface with explicit Push (local-to-remote) and Pull (remote-to-local) methods for file and binary data transfer.
- **FR-003**: The system MUST provide a DataChannel method that accepts raw bytes and a remote path, writing the bytes as a file at the destination (for clipboard images, generated data).
- **FR-004**: The system MUST provide a GitChannel interface that encapsulates git transport setup and commit synchronization per workspace type.
- **FR-005**: Each channel type MUST have a working implementation for all applicable workspace types: local, container, compose, SSH, and k8s-deploy. GitChannel is not applicable for local workspaces (same filesystem, use standard git commands directly). PipeChannel and DataChannel apply to all workspace types. Note: k8s-sandbox uses the same transport as k8s-deploy (kubectl exec); channel support for k8s-sandbox is deferred until the workspace type has a factory implementation.
- **FR-006**: The existing `Push()` and `Pull()` methods on the Workspace interface MUST be refactored to delegate to DataChannel internally, preserving identical external behavior for remote workspace types. For local workspaces (where Push/Pull currently return "not supported"), DataChannel enables new file transfer functionality via direct filesystem copy.
- **FR-007**: The existing `Harvest()` method on the Workspace interface MUST be refactored to delegate to GitChannel internally, preserving identical external behavior.
- **FR-008**: Channels MUST be available when first used, without requiring explicit setup by the user or consumer code.
- **FR-009**: Channel errors MUST provide a human-readable summary that describes the failed operation and workspace context, while preserving the underlying cause for diagnostic purposes.
- **FR-010**: CLI commands MUST display the human-readable error summary by default and the full error chain when a verbose flag is active.
- **FR-011**: Each workspace type MUST use its native transport mechanism for channels, consistent with how the workspace type already handles command execution and file operations.
- **FR-012**: The GitChannel MUST manage the complete round-trip for git synchronization (establishing the connection, transferring commits, and cleaning up), not just providing connection parameters.

### Key Entities

- **Channel**: A typed communication path between the local machine and a remote workspace. Each channel type (Pipe, Data, Git) has a distinct interface tailored to its data transfer pattern.
- **ChannelError**: A structured error that wraps transport-level failures with a human-readable channel context (channel type, operation attempted, workspace state).
- **Workspace**: The existing abstraction for all workspace types, extended with channel accessor methods that return typed channel instances.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Text payloads sent via PipeChannel arrive at the remote workspace within 500ms for co-located workspaces and within 2 seconds for network-remote workspaces.
- **SC-002**: File transfers via DataChannel complete at the same speed (within 10% variance) as the current file transfer operations they replace.
- **SC-003**: Git operations via GitChannel produce identical results (same commits, same refs) as the current git synchronization behavior.
- **SC-004**: Adding support for a new workspace type requires implementing channel behavior in one location per channel type, rather than updating every feature that transfers data.
- **SC-005**: All existing `cc-deck ws push`, `cc-deck ws pull`, and `cc-deck ws harvest` commands produce identical user-visible behavior after refactoring to use channels.
- **SC-006**: Channel error messages are understandable to users without requiring knowledge of the underlying transport (no raw infrastructure error output in default mode).

### Interface Contract Requirements

Per Constitution Principle VII, the new channel interfaces (PipeChannel, DataChannel, GitChannel) extend the existing [Workspace interface](../../cc-deck/internal/ws/interface.go) with typed communication capabilities. Each channel interface MUST have a documented behavioral contract covering error handling patterns, concurrency guarantees, and lifecycle expectations. Implementations of Push/Pull/Harvest on the Workspace interface will delegate to the corresponding channel, so the channel contracts must satisfy the behavioral requirements already established for those methods.

## Clarifications

### Session 2026-04-22

- Q: Local workspace Push/Pull currently returns ErrNotSupported. Does the channel abstraction add new Push/Pull functionality to local workspaces, or should they remain unsupported? → A: Local Push/Pull is new functionality. DataChannel enables it as a deliberate side effect of the channel abstraction.
- Q: Is PipeChannel strictly send-only (local to remote), or should it support receiving data back from the remote workspace? → A: Send-only for now, but the interface must accommodate a request-response pattern (send a message, receive a response) to support future TUI status queries without requiring a new channel type.
- Q: After a workspace stops and restarts, should a previously obtained channel reference work again automatically or require re-creation? → A: Transparent reconnect. Channels work again automatically once the workspace is running. No action needed by the consumer.

## Assumptions

- All workspace types already have a working `Exec()` implementation that the channel abstraction can build upon.
- The zellij pipe mechanism (`zellij pipe --name <name> --payload <data>`) is available and functional in all remote workspace environments where Zellij runs.
- Git is installed in remote workspaces that use GitChannel (this is already a requirement for the existing harvest workflow).
- The existing integration test infrastructure (from spec 016) provides access to real K8s clusters for testing channel implementations against K8s workspace types.
- SSH ControlMaster (connection multiplexing) is a performance optimization that will be addressed in a separate effort; channels work without it, just with higher per-call latency for SSH workspaces.
- Encryption of channel data (e.g., for clipboard images) is a consumer-level concern, not a channel-level concern. Channels transport bytes; consumers decide whether to encrypt before sending.
- Concurrent access to the same channel instance from multiple goroutines is expected (e.g., multiple consumers sending pipe messages). Channel implementations must be safe for concurrent use.
- Documentation updates (README, CLI reference) are required as part of feature completion per constitution principle IX. Since channels are an internal abstraction with no new CLI commands, documentation updates focus on updated architecture descriptions and any behavioral changes to existing commands.
