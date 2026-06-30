# Feature Specification: OpenShell SDK Migration

**Feature Branch**: `075-openshell-sdk-migration`
**Created**: 2026-06-30
**Status**: Draft
**Input**: Replace cc-deck's OpenShell CLI wrapper with the OpenShell Go SDK

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Sandbox Lifecycle Without CLI Binary (Priority: P1)

A developer uses `cc-deck ws new --type openshell` to create a sandboxed workspace. The system communicates directly with the OpenShell gateway over gRPC via the SDK, without requiring the `openshell` CLI binary to be installed on the host.

**Why this priority**: This is the core value proposition. Every OpenShell operation (create, attach, delete, exec, upload, download) currently requires the CLI binary. Eliminating this dependency unblocks users who cannot install the CLI and removes the fragile output-parsing layer.

**Independent Test**: Can be fully tested by creating, attaching to, and deleting an OpenShell workspace. Delivers immediate value by removing the CLI binary requirement.

**Acceptance Scenarios**:

1. **Given** a host without the `openshell` CLI binary installed but with a reachable OpenShell gateway, **When** the user runs `cc-deck ws new --type openshell`, **Then** the workspace is created successfully via the SDK's gRPC connection.
2. **Given** an existing OpenShell workspace, **When** the user runs `cc-deck ws attach`, **Then** the system attaches to the sandbox via the SDK without spawning any CLI subprocess.
3. **Given** an existing OpenShell workspace, **When** the user runs `cc-deck ws delete`, **Then** the sandbox is destroyed via the SDK and the workspace is removed.

---

### User Story 2 - Structured Error Handling (Priority: P2)

When an OpenShell operation fails, the system reports structured error information (not found, already exists, permission denied, unavailable) instead of raw CLI stderr output with ANSI escape codes.

**Why this priority**: The current CLI wrapper loses gRPC status codes during text parsing. Structured errors enable better error messages, retry logic, and debugging.

**Independent Test**: Can be tested by triggering known error conditions (deleting a non-existent sandbox, creating a duplicate) and verifying the error type is correctly identified.

**Acceptance Scenarios**:

1. **Given** a sandbox name that does not exist, **When** the user runs `cc-deck ws delete <name>`, **Then** the system reports a "not found" error with the sandbox name, not a raw CLI error string.
2. **Given** a gateway that is unreachable, **When** the user runs `cc-deck ws new`, **Then** the system reports an "unavailable" error with the gateway address, not a connection refused stack trace.

---

### User Story 3 - Unit Testing with Fake Client (Priority: P3)

Developers contributing to cc-deck can run OpenShell-related unit tests without a running gateway or CLI binary. Tests use the SDK's in-memory fake client to simulate sandbox and provider lifecycle operations.

**Why this priority**: The current integration has no unit test coverage for gateway interactions because tests would require a running gateway and CLI binary. The fake client removes this barrier.

**Independent Test**: Can be tested by running `make test` and verifying that OpenShell workspace tests pass without any external dependencies.

**Acceptance Scenarios**:

1. **Given** a developer machine with no OpenShell gateway running, **When** the developer runs `make test`, **Then** all OpenShell workspace unit tests pass using the fake client.
2. **Given** a test that creates a sandbox via the fake client, **When** the test calls `WaitReady`, **Then** the fake client transitions the sandbox to the ready state synchronously.

---

### Edge Cases

- What happens when the gateway address is empty or malformed? The SDK returns an InvalidArgument error during client construction, before any RPC is attempted.
- What happens when TLS is configured but certificate files are missing? The SDK returns an error during connection setup with a clear message about the missing file.
- What happens when the gateway is reachable but returns an unexpected gRPC status code? The SDK's typed StatusError is propagated with the original code and message.
- What happens when credential detection finds environment variables but provider creation fails? The error is logged as a warning (existing behavior preserved) and workspace creation continues without that provider.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST communicate with the OpenShell gateway via the SDK's gRPC client, not by spawning the `openshell` CLI binary.
- **FR-002**: System MUST remove the `cliClient` struct, `execCLI`, `execCLICaptureName`, `parseSandboxName`, `parseSandboxState`, and `stripANSI` functions from `internal/openshell/client.go`.
- **FR-003**: System MUST use the SDK's `v1.ClientInterface` and sub-clients (`Sandboxes()`, `Providers()`, `Exec()`, `Files()`) as the primary API surface for gateway communication.
- **FR-004**: System MUST map the existing `GatewayConfig` (Address, TLS, TLSCertPath, TLSKeyPath, TLSCAPath) to the SDK's `v1.Config` type, producing a valid SDK client configuration. The default authentication mode is `NoAuth` (no credentials).
- **FR-005**: System MUST preserve the credential detection and resolution logic in `credentials.go` (`DetectCredentials`, `ResolveCredentials`, `KnownProviderProfiles`, `InjectEnvVars`, `UploadFileCredential`) without changes to their public signatures. Note: `InjectEnvVars` and `UploadFileCredential` accept a `Client` parameter, so they will transparently use the new SDK-backed implementation without signature changes.
- **FR-006**: System MUST use the SDK's `v1.Provider` type when calling `Providers().Ensure()` for credential injection, replacing the CLI-based `CreateProvider`/`UpdateProvider`/`DeleteProvider` calls.
- **FR-007**: System MUST add `github.com/rhuss/openshell-sdk-go` to `go.mod` with a `replace` directive pointing to the local SDK checkout path during development.
- **FR-008**: System MUST use the SDK's `fake.NewClient()` for unit tests of OpenShell workspace operations, enabling tests to run without a gateway or CLI binary.
- **FR-009**: System MUST propagate SDK `StatusError` types to callers so they can use helper functions (`IsNotFound`, `IsAlreadyExists`, `IsUnavailable`) for error handling.
- **FR-010**: System MUST update the `Client` interface in `internal/openshell/iface.go` and all types/functions in the package that depend on it, so that the SDK client is the backing implementation. No files outside `internal/openshell/` import the package directly; consumers access it via the `Client` interface field on workspace structs.

### Key Entities

- **SDK Client** (`v1.ClientInterface`): The top-level SDK client holding a gRPC connection and providing sub-client accessors. Replaces the custom `Client` interface and `cliClient` struct.
- **Sandbox** (`v1.Sandbox`): Represents a sandbox instance with Name, Spec, Status, and Labels. Replaces `SandboxInfo` and `SandboxState`.
- **Provider** (`v1.Provider`): Represents an AI provider registration with Name, Spec (type, credentials). Replaces the CLI-based provider creation calls.
- **ExecResult** (`v1.ExecResult`): Holds collected command output (stdout, stderr, exit code). Replaces the custom `ExecResult` struct.
- **GatewayConfig**: Existing cc-deck struct that resolves gateway connection parameters. Becomes a factory for `v1.Config`.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: All OpenShell workspace operations (create, attach, delete, exec, upload, download, provider management) work without the `openshell` CLI binary installed on the host. The git sync channel is excluded (it requires the CLI for `ext::` transport).
- **SC-002**: OpenShell workspace unit tests pass without a running gateway, using only the SDK fake client.
- **SC-003**: Error messages from failed OpenShell operations include the error type (not found, unavailable, permission denied) instead of raw CLI output.
- **SC-004**: `make verify` passes with zero regressions (all existing tests continue to pass).
- **SC-005**: No references to `os/exec` with the `openshell` binary remain in `internal/openshell/client.go`.

## Documentation Impact

This migration replaces an internal transport layer without changing the CLI surface (`cc-deck ws` subcommands, flags, output format). No new commands or flags are added. Documentation updates are limited to:

- **README.md**: Note that the `openshell` CLI binary is no longer required for core sandbox operations (create, attach, delete, exec, upload, download). The git sync channel still requires the CLI binary.
- **CLI reference / Antora docs**: No changes needed (CLI surface is unchanged).
- **Configuration reference**: No changes needed (`GatewayConfig` fields are unchanged).

## Clarifications

### Session 2026-06-30

- Q: GatewayConfig has no auth field; which SDK AuthProvider should the migration use? → A: `v1.NoAuth()` by default. The current CLI wrapper uses no authentication. Auth configuration is a follow-up concern.
- Q: Should `credentials.go` output types change to `v1.Provider`? → A: No. Keep `ProviderConfig` as-is. The workspace layer maps `ProviderConfig` to `v1.Provider` at call time, minimizing changes to `credentials.go`.

## Assumptions

- The OpenShell SDK at `github.com/rhuss/openshell-sdk-go` is functionally complete for sandbox lifecycle, provider CRUD, command execution, and file transfer operations.
- The SDK's `fake` package supports sandbox Create/Get/List/Delete/WaitReady and provider Create/Get/Ensure/Delete operations sufficiently for unit testing.
- The `replace` directive in `go.mod` is acceptable for development. CI and release builds will use a published SDK version (tracked as a follow-up, not part of this spec).
- New SDK capabilities (Watch, WaitReady for production polling, InteractiveSession for PTY attach) are out of scope. This migration replaces the transport layer only, maintaining functional parity with the CLI wrapper.
- The credential detection logic (`credentials.go`) does not need changes to its public API. Internal calls to `Client.CreateProvider`/`UpdateProvider`/`DeleteProvider` are replaced with SDK `Providers().Ensure()` calls at the workspace layer, not inside `credentials.go` itself.
- The `internal/ws/channel_openshell.go` data channel (`Push`, `Pull`) already delegates to the `Client` interface's `Upload`/`Download` methods, so it migrates automatically when the `Client` implementation changes. The `PushBytes` method currently uses `os/exec` directly and MUST be migrated to use temp-file creation with SDK `Files().Upload()` (chosen over `Exec().Run` stdin piping for reliability, per research.md R4).
- The git channel (`openShellGitChannel`) uses `ext::openshell` URLs for git `ext::` transport, which requires a CLI binary on the PATH for stdin/stdout piping. This is architecturally incompatible with SDK-only communication and is OUT OF SCOPE for this migration. The git channel will continue to require the `openshell` CLI binary until a dedicated git sync mechanism is implemented.
