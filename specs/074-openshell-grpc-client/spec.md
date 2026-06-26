# Feature Specification: OpenShell gRPC Client

**Feature Branch**: `074-openshell-grpc-client`
**Created**: 2026-06-26
**Status**: Draft
**Input**: Replace CLI wrapping with direct gRPC client for OpenShell gateway communication

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Workspace Creation Without CLI Binary (Priority: P1)

A user creates an OpenShell workspace without needing the `openshell` CLI binary installed. The workspace creation, provider setup, sandbox lifecycle, and credential injection all happen through direct gateway communication. The only prerequisite is a running gateway (started separately via brew service or `openshell-dev`).

**Why this priority**: This is the core value proposition. Eliminating the CLI runtime dependency removes the version coupling that caused three bugs during the Vertex provider migration.

**Independent Test**: Can be fully tested by removing the `openshell` CLI from PATH, creating a workspace with `cc-deck ws new --type openshell`, and verifying the sandbox starts and runs correctly.

**Acceptance Scenarios**:

1. **Given** a running OpenShell gateway and no `openshell` CLI on PATH, **When** the user runs `cc-deck ws new --type openshell`, **Then** the workspace is created successfully with providers, sandbox, and credential injection all working.
2. **Given** a gateway with mTLS enabled, **When** cc-deck connects, **Then** it automatically discovers and loads the correct TLS certificates from standard locations.
3. **Given** a gateway on localhost without TLS, **When** cc-deck connects, **Then** it falls back to an insecure connection without requiring user configuration.

---

### User Story 2 - Provider Management with Structured Types (Priority: P1)

Provider creation and updates use structured typed messages instead of CLI flag strings. The `credentials` and `config` fields are separate maps, eliminating the flag confusion that caused the Vertex provider bugs (`--from-gcloud-adc` vs `--from-existing`, `--config` vs `--credential`).

**Why this priority**: This directly addresses the three runtime bugs from brainstorm #075. Compile-time type safety prevents this class of errors entirely.

**Independent Test**: Can be tested by creating a `google-cloud` provider with project_id and region config, verifying the provider is created correctly, and confirming that provider updates work without delete+recreate workarounds.

**Acceptance Scenarios**:

1. **Given** Vertex AI credentials on the host, **When** cc-deck creates a `google-cloud` provider, **Then** the `config` map contains `project_id` and `region` as separate fields from `credentials`, and the provider is created in a single call.
2. **Given** an existing provider, **When** cc-deck updates it, **Then** the update succeeds using the same structured message format as create (no flag asymmetry).
3. **Given** a provider type that doesn't exist in the proto schema, **When** cc-deck attempts to create it, **Then** the error is detected at compile time (missing enum value or type mismatch).

---

### User Story 3 - Command Execution with Streaming Output (Priority: P2)

Commands executed inside sandboxes use streaming communication that separates stdout, stderr, and exit codes. This replaces the current flat subprocess output capture that loses stderr separation and error context.

**Why this priority**: Improves debugging experience for users when commands fail inside sandboxes. Not a blocking issue but a significant quality improvement.

**Independent Test**: Can be tested by running a command that writes to both stdout and stderr inside a sandbox, and verifying that the outputs are correctly separated in cc-deck's display.

**Acceptance Scenarios**:

1. **Given** a running sandbox, **When** the user executes a command that writes to both stdout and stderr, **Then** cc-deck receives and displays stdout and stderr as separate streams.
2. **Given** a command that exits with a non-zero code, **When** cc-deck receives the result, **Then** the exit code is available as a structured field (not parsed from process exit status).

---

### User Story 4 - File Transfer via SSH Tunnel (Priority: P2)

File upload and download between the host and sandbox work through a Go-native SSH tunnel, eliminating the dependency on the CLI binary for file transfer operations.

**Why this priority**: File transfer is essential for credential injection (uploading config files) and workspace setup. The SSH tunnel also enables interactive attach sessions.

**Independent Test**: Can be tested by uploading a file to a sandbox, downloading it back, and comparing the contents.

**Acceptance Scenarios**:

1. **Given** a running sandbox, **When** the user uploads a file, **Then** the file appears at the specified path inside the sandbox with correct contents and permissions.
2. **Given** a running sandbox with files, **When** the user downloads a file, **Then** the file is retrieved to the host with correct contents.
3. **Given** a sandbox behind a gateway with mTLS, **When** a file transfer is initiated, **Then** the SSH tunnel authenticates through the gateway's HTTP CONNECT endpoint using the session token from the gateway.

---

### User Story 5 - Compile-Time API Change Detection (Priority: P2)

When the OpenShell gateway API changes between releases, cc-deck detects the change at compile time (during proto regeneration) rather than at runtime (during workspace creation). This gives developers a clear signal about what changed and what needs updating.

**Why this priority**: Forward-looking safety as OpenShell evolves through its alpha phase. Each proto update produces a clear diff of API changes.

**Independent Test**: Can be tested by updating the vendored proto files to a version with a changed RPC signature, running code generation, and verifying the compiler reports the incompatibility.

**Acceptance Scenarios**:

1. **Given** vendored proto files from OpenShell release v0.0.46, **When** a developer updates to v0.0.50 protos with a changed field type, **Then** the Go compiler reports the type mismatch in the affected client code.
2. **Given** a new RPC added to the OpenShell proto, **When** proto regeneration runs, **Then** the new RPC stub is generated and available for implementation without manual coding.

---

### Edge Cases

- What happens when the gateway is unreachable? cc-deck should report a clear connection error with the gateway address, not a generic timeout.
- What happens when the TLS certificates are expired or invalid? cc-deck should report the specific TLS error and suggest checking certificate paths.
- What happens when the proto version doesn't match the gateway version? The gRPC layer should report version mismatches as structured errors (unrecognized fields are silently ignored by protobuf; removed fields cause compile errors).
- What happens during a long-running exec when the gateway restarts? The streaming connection should detect the disconnect and report it, not hang indefinitely.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: cc-deck MUST communicate with the OpenShell gateway using direct protocol communication instead of wrapping the CLI binary.
- **FR-002**: cc-deck MUST implement all existing `Client` interface methods through direct gateway calls: `CreateSandbox`, `GetSandbox`, `DeleteSandbox`, `ExecSandbox`, `ExecSandboxStream`, `AttachExec`, `Upload`, `Download`, `CreateProvider`, `UpdateProvider`, `DeleteProvider`, `EnsureProvider`.
- **FR-003**: The `Client` interface MUST remain unchanged. All callers (`ws/openshell.go`, tests, etc.) MUST work without modification.
- **FR-004**: Provider creation and update MUST use structured message types with separate `credentials` and `config` fields (not CLI flag strings).
- **FR-005**: Command execution MUST support streaming output with separate stdout and stderr channels.
- **FR-006**: File transfer (upload and download) MUST work through a gateway-authenticated tunnel without requiring the CLI binary.
- **FR-007**: Connection setup MUST support mTLS with automatic certificate discovery from standard platform locations.
- **FR-008**: Connection setup MUST fall back to insecure (no TLS) for localhost connections, matching current CLI behavior.
- **FR-009**: The gateway API schema MUST be vendored from a specific OpenShell release tag and regenerated via a build system target.
- **FR-010**: API schema changes between OpenShell releases MUST be detectable at compile time through the regenerated types.
- **FR-011**: The previous CLI-based client MUST be retained behind a build tag for one release cycle as a fallback.
- **FR-012**: Interactive terminal sessions (attach) MUST work through bidirectional streaming communication with PTY support.

### Key Entities

- **Client interface**: The existing contract between workspace management (`ws/openshell.go`) and the gateway communication layer. Stays unchanged.
- **Gateway connection**: A persistent authenticated connection to the OpenShell gateway, supporting mTLS and insecure modes.
- **Provider**: A credential bundle with separate `credentials` (secrets) and `config` (non-secret settings) maps.
- **SSH tunnel**: An authenticated connection through the gateway for file transfer and interactive sessions, using session tokens from the gateway.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: All existing OpenShell workspace operations (create, exec, attach, upload, delete) work without the `openshell` CLI binary on PATH.
- **SC-002**: Provider creation for all supported types (claude, anthropic, github, gitlab, openai, nvidia, google-cloud, generic) succeeds through the direct client.
- **SC-003**: All existing tests pass with the new client implementation (or with updates only to reflect the new transport).
- **SC-004**: Updating vendored API schema files to a new OpenShell release with breaking changes produces compile-time errors (not runtime failures).
- **SC-005**: File upload and download between host and sandbox complete successfully for files up to 100 MB.
- **SC-006**: Streaming command execution correctly separates stdout and stderr for commands that write to both.

## Assumptions

- The OpenShell gateway is running and accessible before workspace operations. cc-deck does not manage gateway lifecycle (that's handled by brew service, `openshell-dev`, or Kubernetes deployment).
- TLS certificates are stored in standard locations (`~/.local/state/openshell/homebrew/tls/` for brew installs, XDG data directory for manual installs). cc-deck does not generate certificates.
- The vendored API schema files are updated manually by the developer when upgrading to a new OpenShell release. This is an intentional workflow (not automatic).
- The SSH tunnel for file transfer uses HTTP CONNECT through the gateway, matching how the CLI implements it. The gateway must support this tunnel endpoint.
- The edge tunnel (WebSocket-based, for gateways behind Cloudflare) is out of scope for the initial implementation. It can be added later when needed for cloud deployments.
- The bidirectional streaming for interactive exec (`AttachExec`) may not fully replace SSH-based attach for all terminal emulation scenarios. If limitations are found, SSH-based attach via the Go SSH library is the fallback.
