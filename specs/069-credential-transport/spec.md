# Feature Specification: Credential Transport Abstraction

**Feature Branch**: `069-credential-transport`
**Created**: 2026-06-07
**Status**: Draft
**Input**: Brainstorm 069-credential-transport-abstraction

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Agent declares credential requirements (Priority: P1)

A developer adds a new agent adapter (e.g., Codex CLI) to cc-deck. They implement the `CredentialSpecs()` method on the agent, declaring which auth modes the agent supports and what env vars, file credentials, and endpoints each mode requires. No other files need modification for credentials to work across all workspace types.

**Why this priority**: This is the foundation. Without agent-declared credential specs, no other credential feature can function.

**Independent Test**: Register a test agent with known credential specs. Verify that the credential system can read the agent's requirements, detect available credentials on the host, and produce the correct env var and file set for injection.

**Acceptance Scenarios**:

1. **Given** a registered agent with `CredentialSpecs()` returning two auth modes (api, vertex), **When** the credential system queries the agent, **Then** it receives two CredentialSpec entries with distinct env var lists, file credentials, and endpoint declarations.
2. **Given** an agent declaring a file credential with a default path, **When** the file exists at the default path, **Then** the credential system detects it as available.
3. **Given** an agent declaring a file credential with a default path, **When** the file does not exist, **Then** the credential system reports that auth mode as unavailable.

---

### User Story 2 - Auth mode selection during workspace creation (Priority: P1)

A user runs `cc-deck ws new` to create a workspace for an agent that supports multiple auth modes (e.g., Claude Code with API key, Vertex AI, and Bedrock). The system detects which auth modes have credentials available on the host. If multiple modes are available, it prompts the user to choose. The chosen mode is stored in the workspace definition.

**Why this priority**: Users need to control which credentials are used, especially in environments where multiple API keys coexist.

**Independent Test**: Set up a host environment with both `ANTHROPIC_API_KEY` and Vertex AI credentials. Run `cc-deck ws new` and verify the user is prompted to choose. Verify the choice is persisted in the workspace definition.

**Acceptance Scenarios**:

1. **Given** a host with only `ANTHROPIC_API_KEY` set, **When** the user creates a Claude Code workspace, **Then** the "api" auth mode is auto-selected without prompting.
2. **Given** a host with both API key and Vertex credentials, **When** the user creates a workspace, **Then** the system presents available modes and lets the user choose.
3. **Given** a host with both API key and Vertex credentials, **When** the user passes `--auth-mode vertex`, **Then** the system uses Vertex without prompting.
4. **Given** a host with no credentials for any auth mode, **When** the user creates a workspace, **Then** the system reports an error listing what credentials are needed.

---

### User Story 3 - Workspace listing shows auth mode (Priority: P2)

A user runs `cc-deck ws ls` and sees the active auth mode for each workspace alongside the agent name (e.g., "claude/vertex", "opencode/api"). This makes it clear which credentials each workspace uses.

**Why this priority**: Visibility into credential configuration prevents confusion when debugging auth failures.

**Independent Test**: Create workspaces with different auth modes. Run `cc-deck ws ls` and verify the auth mode column displays correctly.

**Acceptance Scenarios**:

1. **Given** a workspace created with `--auth-mode vertex`, **When** the user runs `cc-deck ws ls`, **Then** the output includes the auth mode (e.g., "vertex") for that workspace.
2. **Given** a workspace created with auto-selected auth mode, **When** the user runs `cc-deck ws ls`, **Then** the auto-selected mode is displayed the same as an explicitly chosen one.

---

### User Story 4 - Eager credential validation at workspace start (Priority: P2)

A user starts a workspace. Before launching the container or SSH session, the system validates that all required credentials for the chosen auth mode are present. If a credential is missing, the system reports a clear error message naming the missing env var or file.

**Why this priority**: Failing fast prevents wasted time waiting for containers to start only to hit auth errors.

**Independent Test**: Create a workspace with "vertex" auth mode. Unset the `GOOGLE_APPLICATION_CREDENTIALS` file. Start the workspace and verify the error message names the missing credential.

**Acceptance Scenarios**:

1. **Given** a workspace with "api" auth mode and `ANTHROPIC_API_KEY` set, **When** the workspace starts, **Then** validation passes and the workspace launches normally.
2. **Given** a workspace with "vertex" auth mode and the JSON credential file missing, **When** the workspace starts, **Then** the system reports "missing credential file: GOOGLE_APPLICATION_CREDENTIALS" before attempting to launch.
3. **Given** a workspace with credentials marked as "externally provided" (K8s Secret), **When** the workspace starts, **Then** host-side validation is skipped for those credentials.

---

### User Story 5 - Credential injection across workspace types (Priority: P1)

A user creates workspaces of different types (local, Podman, SSH, K8s, Compose, OpenShell) for the same agent with the same auth mode. The credential transport layer handles the differences: env var injection for containers, file copying for SSH, Secret references for K8s, provider creation for OpenShell.

**Why this priority**: Multi-workspace-type support is a core cc-deck feature. Credentials must work everywhere.

**Independent Test**: Create a Podman workspace and an SSH workspace for Claude Code with Vertex auth. Verify that both receive the correct env vars and the JSON credential file in the right location.

**Acceptance Scenarios**:

1. **Given** a Podman workspace with "api" auth mode, **When** the workspace starts, **Then** `ANTHROPIC_API_KEY` is injected as an env var in the container.
2. **Given** an SSH workspace with "vertex" auth mode, **When** the workspace starts, **Then** the JSON credential file is copied to the remote host and env vars are written to the credential env file.
3. **Given** a K8s workspace with "api" auth mode and credentials marked as "externally provided", **When** the workspace starts, **Then** the Secret reference is used without host-side file operations.
4. **Given** an OpenShell workspace with "vertex" auth mode, **When** the workspace starts, **Then** the credential file is uploaded to the sandbox and env vars are injected into shell rc files.

---

### User Story 6 - Generalized SSH and OpenShell credential handling (Priority: P2)

A user creates an SSH workspace for OpenCode (not Claude Code). The SSH credential logic resolves credentials from the OpenCode agent's declared specs instead of the hardcoded Claude-only auth mode detection. Similarly, OpenShell credential resolution uses agent-declared specs instead of the `KnownProviderProfiles` map.

**Why this priority**: Removes the Claude-only hardcoding that blocks multi-agent support in SSH and OpenShell workspaces.

**Independent Test**: Create an SSH workspace specifying `--agent opencode`. Verify that `OPENAI_API_KEY` is resolved (not `ANTHROPIC_API_KEY`).

**Acceptance Scenarios**:

1. **Given** an SSH workspace for OpenCode with `OPENAI_API_KEY` set, **When** the workspace starts, **Then** the credential file contains `OPENAI_API_KEY` and does not contain Claude-specific env vars.
2. **Given** an OpenShell workspace for a new agent with custom credential specs, **When** the workspace starts, **Then** credentials are resolved from the agent's specs without requiring changes to `KnownProviderProfiles`.

---

### Edge Cases

- What happens when an agent's credential env var conflicts with another agent's (e.g., both need `ANTHROPIC_API_KEY` but for different purposes)?
  Both agents receive the same resolved value. This is correct since the env var represents the same secret.
- What happens when a user switches auth mode on an existing workspace?
  The workspace definition is updated. The next start uses the new mode's credentials.
- What happens when a credential file path contains spaces or special characters?
  File paths are quoted/escaped in all transport mechanisms (shell commands, env files, container mounts).
- What happens when an agent declares an auth mode with `UnsetVars` (e.g., Gemini CLI needs `GEMINI_API_KEY` unset for Vertex)?
  The credential transport layer explicitly unsets those vars in the workspace environment after injecting the chosen mode's credentials.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The Agent interface MUST include a `CredentialSpecs()` method returning a slice of credential specifications.
- **FR-002**: Each CredentialSpec MUST declare: name (auth mode identifier), env vars (with optional fixed values), file credential (env var name and default host path), network endpoints (host:port pairs), vars to unset, and priority for auto-selection.
- **FR-003**: The Claude Code agent adapter MUST declare at least three auth modes: api (ANTHROPIC_API_KEY), vertex (Vertex AI credentials), and bedrock (AWS credentials).
- **FR-004**: The OpenCode agent adapter MUST declare at least two auth modes: openai (OPENAI_API_KEY) and anthropic (ANTHROPIC_API_KEY).
- **FR-005**: The `cc-deck ws new` command MUST detect available auth modes by checking the host environment against each agent's declared credential specs.
- **FR-006**: When multiple auth modes are available, `cc-deck ws new` MUST either prompt the user or accept a `--auth-mode` flag.
- **FR-007**: The chosen auth mode MUST be persisted in the workspace definition.
- **FR-008**: `cc-deck ws ls` MUST display the active auth mode for each workspace.
- **FR-009**: Credential validation MUST run eagerly at workspace start by default, checking that all required env vars are set and all required files exist on the host.
- **FR-010**: Workspace definitions MUST support marking credentials as "externally provided" to skip host-side validation (for K8s Secrets, OpenShell providers).
- **FR-011**: A shared credential transport package MUST handle env var injection, file copying, path remapping, and permission setting across all workspace types.
- **FR-012**: The `internal/ssh/credentials.go` module MUST be refactored to use agent-declared credential specs instead of hardcoded auth mode detection.
- **FR-013**: The `internal/openshell/credentials.go` module MUST be refactored to resolve credentials from agent-declared specs instead of `KnownProviderProfiles`.
- **FR-014**: CredentialSpecs MUST support a list of env vars that need to be unset in the workspace environment to avoid conflicts (e.g., Gemini CLI's Vertex mode).
- **FR-015**: Existing Claude Code credential flows MUST not regress. All current SSH, Podman, K8s, Compose, and OpenShell credential behavior MUST be preserved.

### Key Entities

- **CredentialSpec**: Represents one auth mode for an agent. Contains name, env var declarations, file credential info, endpoint list, unset vars, and priority.
- **Credential transport**: Shared utilities for resolving credentials from the host environment and injecting them into workspaces. Handles file copying, env var injection, path remapping.
- **Workspace auth mode**: The selected CredentialSpec name stored in the workspace definition. Determines which credentials are validated and injected at start time.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Adding a new agent with custom credentials requires implementing only the `CredentialSpecs()` method. No workspace type code changes needed.
- **SC-002**: All six workspace types (local, Podman, K8s, SSH, Compose, OpenShell) support multi-agent credentials through the shared transport layer.
- **SC-003**: Users see a clear, actionable error message within 2 seconds of starting a workspace with missing credentials, before any container or remote session is created.
- **SC-004**: Existing Claude Code workspaces created before this feature continue to work without modification (backward compatibility).
- **SC-005**: `cc-deck ws ls` output includes auth mode information for every workspace.
- **SC-006**: The hardcoded `detectAuthMode()` function in SSH credentials and the `KnownProviderProfiles` map in OpenShell credentials are fully replaced by agent-declared specs.

## Assumptions

- Agent credential requirements are static at compile time. Dynamic credential discovery (e.g., querying an agent binary for its supported providers) is out of scope.
- Credential rotation is out of scope for this feature. Long-running workspaces may need to be restarted to pick up new credentials.
- The `cc-deck credentials check` standalone command is out of scope. Eager validation at workspace start covers the primary use case.
- OAuth-based authentication flows (e.g., Claude Code's browser-based login) are out of scope. This feature handles env var and file-based credentials only.
- The credential transport package does not validate that credentials are *correct* (e.g., that an API key is valid). It only checks that required values are present.
- Per the project constitution, documentation updates (README.md, CLI reference for `--auth-mode` flag, configuration reference for workspace auth mode and "externally provided" markers) are required as part of this feature delivery.
