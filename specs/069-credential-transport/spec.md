# Feature Specification: Credential Transport Abstraction

**Feature Branch**: `069-credential-transport`
**Created**: 2026-06-07
**Updated**: 2026-06-08
**Status**: Draft
**Input**: Brainstorm 069-credential-transport-abstraction

## Core Model: Detect-All with Opt-Out

Workspaces are **not** tied to a single agent for credentials. A workspace can host sessions from multiple agents simultaneously (e.g., Claude Code and OpenCode in the same container). The credential system scans ALL registered agents, detects ALL available credentials on the host, merges them into one set, and injects everything into the workspace.

When two auth modes produce conflicting env vars (e.g., Claude "api" and Claude "vertex" both want to set credentials but with different semantics), the user is prompted to exclude one. Exclusions are stored in the workspace definition as an opt-out list.

This model means:
- No `--agent` flag needed for credential purposes (agents are still relevant for hooks, sidebar indicators, etc.)
- Workspaces get all available credentials by default
- Users only interact with the credential system when conflicts exist
- The AUTH column in `ws ls` is derived at display time, not stored

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

### User Story 2 - Automatic multi-agent credential injection (Priority: P1)

A user runs `cc-deck ws new` to create a workspace. The system scans all registered agents, detects which auth modes have credentials available on the host, and injects all of them into the workspace. The user does not need to specify an agent or auth mode.

**Why this priority**: Users should get all their credentials injected by default without manual configuration.

**Independent Test**: Set up a host environment with `ANTHROPIC_API_KEY` and `OPENAI_API_KEY`. Run `cc-deck ws new`. Verify both credentials are injected into the workspace.

**Acceptance Scenarios**:

1. **Given** a host with only `ANTHROPIC_API_KEY` set and Vertex credentials available, **When** the user creates a workspace, **Then** both Claude "api" and "vertex" credential sets are detected. Since they conflict (both are Claude auth modes with different semantics), the user is prompted to exclude one.
2. **Given** a host with `ANTHROPIC_API_KEY` and `OPENAI_API_KEY` set, **When** the user creates a workspace, **Then** both credentials are injected without prompting (no conflict between different agents' credentials).
3. **Given** a host with no credentials for any auth mode, **When** the user creates a workspace, **Then** the workspace is created with no credentials injected (warning, not error).
4. **Given** a host with Vertex credentials only, **When** the user creates a workspace, **Then** vertex credentials are auto-injected without prompting (single available Claude mode, no conflict).

---

### User Story 3 - Conflict resolution via opt-out (Priority: P1)

A user has both `ANTHROPIC_API_KEY` and Vertex AI credentials on the host. When creating a workspace, the system detects both Claude auth modes as available. Since they inject contradictory env vars (vertex sets `CLAUDE_CODE_USE_VERTEX=1`, api does not), the user is prompted to exclude one. The exclusion is persisted so future workspace starts skip the excluded mode.

**Why this priority**: Without conflict resolution, contradictory credentials would be injected, causing unpredictable agent behavior.

**Independent Test**: Set up a host with both API key and Vertex credentials. Create a workspace. Verify the user is prompted. Verify the exclusion is persisted. Recreate and verify no prompt.

**Acceptance Scenarios**:

1. **Given** both "api" and "vertex" Claude modes are available, **When** the user creates a workspace, **Then** the system identifies the conflict (both set credentials for the same agent with different semantics) and prompts the user to exclude one.
2. **Given** the user excludes "api" mode, **When** the workspace definition is saved, **Then** it contains `exclude-auth-modes: ["claude/api"]`.
3. **Given** a workspace with `exclude-auth-modes: ["claude/api"]`, **When** the workspace starts, **Then** only vertex credentials are injected.
4. **Given** a user wants to change their exclusion, **When** they run `cc-deck ws update --include claude/api --exclude claude/vertex`, **Then** the exclusion list is updated and validated.

---

### User Story 4 - Eager credential validation at workspace start (Priority: P2)

A user starts a workspace. Before launching the container or SSH session, the system validates that all required credentials for the non-excluded auth modes are present. If a credential is missing, the system reports a clear error message naming the missing env var or file.

**Why this priority**: Failing fast prevents wasted time waiting for containers to start only to hit auth errors.

**Independent Test**: Create a workspace with vertex credentials available. Unset the credential file. Start the workspace and verify the error message names the missing credential.

**Acceptance Scenarios**:

1. **Given** a workspace where all detected credentials are still present, **When** the workspace starts, **Then** validation passes and the workspace launches normally.
2. **Given** a workspace where a credential file has been removed since creation, **When** the workspace starts, **Then** the system reports the missing credential before attempting to launch.
3. **Given** a workspace with credentials marked as "externally provided" (K8s Secret), **When** the workspace starts, **Then** host-side validation is skipped for those credentials.

---

### User Story 5 - Credential injection across workspace types (Priority: P1)

A user creates workspaces of different types (local, Podman, SSH, K8s, Compose, OpenShell). The credential transport layer handles the differences: env var injection for containers, file copying for SSH, Secret references for K8s, provider creation for OpenShell. All detected (non-excluded) credentials are injected regardless of workspace type.

**Why this priority**: Multi-workspace-type support is a core cc-deck feature. Credentials must work everywhere.

**Independent Test**: Create a Podman workspace and an SSH workspace on the same host. Verify both receive the same credential set adapted to their transport mechanism.

**Acceptance Scenarios**:

1. **Given** a Podman workspace with API key and OpenAI key available, **When** the workspace starts, **Then** both `ANTHROPIC_API_KEY` and `OPENAI_API_KEY` are injected as env vars.
2. **Given** an SSH workspace with vertex credentials, **When** the workspace starts, **Then** the JSON credential file is copied to the remote host and env vars are written to the credential env file.
3. **Given** a K8s workspace with credentials marked as "externally provided", **When** the workspace starts, **Then** the Secret reference is used without host-side file operations.

---

### User Story 6 - Workspace listing shows credentials in verbose mode (Priority: P2)

A user runs `cc-deck ws ls -v` and sees the injected auth modes for each workspace. The default `ws ls` output does not show credentials (keeping it clean for everyday use).

**Why this priority**: Visibility into credential configuration helps debugging without cluttering the default view.

**Acceptance Scenarios**:

1. **Given** a workspace with vertex and OpenAI credentials injected, **When** the user runs `cc-deck ws ls -v`, **Then** the AUTH column shows `claude/vertex opencode/openai`.
2. **Given** the same workspace, **When** the user runs `cc-deck ws ls` (no `-v`), **Then** no AUTH column is displayed.

---

### User Story 7 - Generalized SSH and OpenShell credential handling (Priority: P2)

SSH and OpenShell credential resolution uses agent-declared specs instead of hardcoded Claude-only detection. The `detectAuthMode()` function in SSH and the `KnownProviderProfiles` map in OpenShell are replaced by the detect-all model.

**Why this priority**: Removes the Claude-only hardcoding that blocks multi-agent support in SSH and OpenShell workspaces.

**Acceptance Scenarios**:

1. **Given** an SSH workspace on a host with `OPENAI_API_KEY` set, **When** the workspace starts, **Then** the credential file contains `OPENAI_API_KEY`.
2. **Given** an OpenShell workspace on a host with multiple agents' credentials, **When** the workspace starts, **Then** all available credentials are resolved from agent specs without requiring changes to `KnownProviderProfiles`.

---

### Edge Cases

- What happens when two agents declare the same env var name with the same semantics (e.g., both Claude and OpenCode accept `ANTHROPIC_API_KEY`)?
  The env var is resolved once and injected once. No conflict because the resolved value is identical.
- What happens when two auth modes from the SAME agent are both available?
  This is a conflict. The user is prompted to exclude one. Example: Claude "api" and "vertex" both available means the user must choose which auth path Claude Code should use.
- What happens when a credential file path contains spaces or special characters?
  File paths are quoted/escaped in all transport mechanisms (shell commands, env files, container mounts).
- What happens when an auth mode declares `UnsetVars` (e.g., Gemini CLI needs `GEMINI_API_KEY` unset for Vertex)?
  The credential transport layer explicitly unsets those vars in the workspace environment after injecting the chosen mode's credentials.
- What happens when no credentials are available at all?
  The workspace is created with a warning (not an error). Some workspace types (local) may not need credentials.

## Clarifications

### Session 2026-06-07

- Q: Does CredentialSpec priority auto-select silently or only control prompt ordering? → A: Priority controls prompt ordering and default suggestion; the user always confirms when multiple auth modes are available.
- Q: How should existing workspaces without an auth mode field be handled? → A: No migration. Existing workspaces will be deleted and recreated. The auth mode field is required on all new workspaces.
- Q: Should credential values be logged and what file permissions for credential files? → A: Never log credential values (not even at debug level); all generated credential files use 0600 permissions.
- Q: At what granularity is the "externally provided" marker applied? → A: Per workspace (all-or-nothing). When marked, all credentials for that workspace skip host-side validation.
- Q: When switching auth mode on a workspace, validate immediately or at next start? → A: Validate immediately at switch time to give instant feedback on missing credentials.

### Session 2026-06-08

- Q: Should a workspace be tied to a single agent? → A: No. A workspace can host multiple agents. The credential system detects all available credentials from all agents and injects them all.
- Q: How to handle conflicts between auth modes? → A: Opt-out model. When two modes from the same agent produce conflicting env vars, prompt the user to exclude one. Store exclusions in the workspace definition.
- Q: Should AUTH column be shown by default in ws ls? → A: No. Only in verbose mode (`ws ls -v`).

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The Agent interface MUST include a `CredentialSpecs()` method returning a slice of credential specifications.
- **FR-002**: Each CredentialSpec MUST declare: name (auth mode identifier), env vars (with optional fixed values), file credential (env var name and default host path), network endpoints (host:port pairs), vars to unset, and priority (integer for prompt ordering).
- **FR-003**: The Claude Code agent adapter MUST declare at least three auth modes: api (ANTHROPIC_API_KEY), vertex (Vertex AI credentials), and bedrock (AWS credentials).
- **FR-004**: The OpenCode agent adapter MUST declare at least two auth modes: openai (OPENAI_API_KEY) and anthropic (ANTHROPIC_API_KEY).
- **FR-005**: The `cc-deck ws new` command MUST detect available auth modes from ALL registered agents by checking the host environment against each agent's declared credential specs.
- **FR-006**: When auth modes from the same agent conflict (inject contradictory env vars), `cc-deck ws new` MUST prompt the user to exclude one and persist the exclusion.
- **FR-007**: The exclusion list MUST be persisted in the workspace definition as `exclude-auth-modes` (list of `agent/mode` strings).
- **FR-008**: `cc-deck ws ls -v` MUST display the active auth modes for each workspace in an AUTH column. The default `ws ls` output MUST NOT show the AUTH column.
- **FR-009**: Credential validation MUST run eagerly at workspace start by default, checking that all required env vars are set and all required files exist on the host for all non-excluded auth modes.
- **FR-010**: Workspace definitions MUST support marking credentials as "externally provided" to skip host-side validation (for K8s Secrets, OpenShell providers).
- **FR-011**: A shared credential transport package MUST handle env var injection, file copying, path remapping, and permission setting across all workspace types.
- **FR-012**: The `internal/ssh/credentials.go` module MUST be refactored to use agent-declared credential specs instead of hardcoded auth mode detection.
- **FR-013**: The `internal/openshell/credentials.go` module MUST be refactored to resolve credentials from agent-declared specs instead of `KnownProviderProfiles`.
- **FR-014**: CredentialSpecs MUST support a list of env vars that need to be unset in the workspace environment to avoid conflicts (e.g., Gemini CLI's Vertex mode).
- **FR-015**: Existing Claude Code credential flows MUST not regress. All current SSH, Podman, K8s, Compose, and OpenShell credential behavior MUST be preserved.
- **FR-016**: Credential values MUST NOT be logged at any log level (including debug). Only credential key names and auth mode names may appear in logs.
- **FR-017**: All generated credential files (env files, copied credential files) MUST be created with 0600 permissions.
- **FR-018**: `cc-deck ws update` MUST support `--exclude` and `--include` flags to modify the opt-out list, with immediate credential validation.

### Key Entities

- **CredentialSpec**: Represents one auth mode for an agent. Contains name, env var declarations, file credential info, endpoint list, unset vars, and priority.
- **Credential transport**: Shared utilities for resolving credentials from the host environment and injecting them into workspaces. Handles file copying, env var injection, path remapping.
- **Exclusion list**: The list of `agent/mode` strings stored in the workspace definition. Auth modes in this list are skipped during credential detection and injection.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Adding a new agent with custom credentials requires implementing only the `CredentialSpecs()` method. No workspace type code changes needed.
- **SC-002**: All six workspace types (local, Podman, K8s, SSH, Compose, OpenShell) support multi-agent credentials through the shared transport layer.
- **SC-003**: Users see a clear, actionable error message within 2 seconds of starting a workspace with missing credentials, before any container or remote session is created.
- **SC-004**: Workspaces created without any `--agent` or `--auth-mode` flags automatically receive all available credentials from all registered agents.
- **SC-005**: `cc-deck ws ls -v` output includes auth mode information for every workspace.
- **SC-006**: The hardcoded `detectAuthMode()` function in SSH credentials and the `KnownProviderProfiles` map in OpenShell credentials are fully replaced by agent-declared specs.

## Assumptions

- Agent credential requirements are static at compile time. Dynamic credential discovery (e.g., querying an agent binary for its supported providers) is out of scope.
- Credential rotation is out of scope for this feature. Long-running workspaces may need to be restarted to pick up new credentials.
- The `cc-deck credentials check` standalone command is out of scope. Eager validation at workspace start covers the primary use case.
- OAuth-based authentication flows (e.g., Claude Code's browser-based login) are out of scope. This feature handles env var and file-based credentials only.
- The credential transport package does not validate that credentials are *correct* (e.g., that an API key is valid). It only checks that required values are present.
- Per the project constitution, documentation updates (README.md, CLI reference, configuration reference) are required as part of this feature delivery.
- Conflict detection operates at the agent level: two modes from the SAME agent that are both available constitute a conflict. Modes from DIFFERENT agents never conflict (they inject independent credential sets).
