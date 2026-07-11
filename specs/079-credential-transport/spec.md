# Feature Specification: Credential Transport Abstraction

**Feature Branch**: `079-credential-transport`
**Created**: 2026-07-08
**Status**: Draft
**Input**: Brainstorm 069 - Credential Transport Abstraction for Multi-Agent Support

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Multi-Agent Credential Detection (Priority: P1)

A user has both Claude Code and OpenCode installed, with `ANTHROPIC_API_KEY` and `OPENAI_API_KEY` set in their environment. When they create a new workspace, the system automatically detects all available credentials across all registered agents and injects the correct credentials for whichever agent the workspace uses.

**Why this priority**: This is the foundation. Without agent-aware credential detection, multi-agent workspaces cannot function. All other stories depend on credentials being correctly detected and injected.

**Independent Test**: Set `ANTHROPIC_API_KEY` and `OPENAI_API_KEY` in the local environment. Create a workspace for Claude, verify only Claude-relevant credentials are injected. Create a workspace for OpenCode, verify only OpenCode-relevant credentials are injected.

**Acceptance Scenarios**:

1. **Given** both `ANTHROPIC_API_KEY` and `OPENAI_API_KEY` are set locally, **When** a workspace is created for Claude, **Then** the workspace receives `ANTHROPIC_API_KEY` and the OpenAI key is not injected.
2. **Given** `OPENAI_API_KEY` is set but `ANTHROPIC_API_KEY` is not, **When** a workspace is created for Claude, **Then** the system warns that required credentials are missing and skips that agent's credential injection.
3. **Given** both agents declare `ANTHROPIC_API_KEY` (Claude and OpenCode both support Anthropic), **When** credentials are merged, **Then** the shared key is resolved once and injected without duplication.

---

### User Story 2 - Legacy Credential Code Removal (Priority: P2)

The deprecated `KnownProviderProfiles` map in `openshell/credentials.go` and the hardcoded `BuildCredentialSet`/`detectAuthMode` functions in `ssh/credentials.go` are fully replaced by the `internal/credential` package. All callers of the legacy functions are migrated to use the agent-declared `CredentialSpec` model through the shared credential package.

**Why this priority**: The legacy code duplicates logic that the new credential package already provides. Keeping both creates maintenance burden and risks inconsistency. This is the primary deliverable that makes the credential system agent-agnostic.

**Independent Test**: Run `make verify` after removing the legacy functions. All existing workspace creation flows (Podman, Compose, K8s, SSH, OpenShell) continue to resolve credentials correctly.

**Acceptance Scenarios**:

1. **Given** the legacy `KnownProviderProfiles` is removed, **When** a Podman workspace is created with `ANTHROPIC_API_KEY` set, **Then** credentials are detected and injected via `credential.DetectAll()` and the workspace functions identically to before.
2. **Given** `BuildCredentialSet` is removed from `ssh/credentials.go`, **When** an SSH workspace is created with Vertex credentials, **Then** the Vertex file credential and env vars are correctly resolved via `credential.Resolve()` and transported to the remote host.
3. **Given** the legacy code is removed, **When** `rg "KnownProviderProfiles\|BuildCredentialSet\|detectAuthMode" --type go` is run, **Then** no references are found outside of test files or migration comments.

---

### User Story 3 - Credential Validation at Workspace Start (Priority: P3)

When a user starts a workspace, the system validates that all required credentials for the selected agent and auth mode are available before proceeding. Missing credentials produce a clear error message naming the specific env vars or files that need to be set.

**Why this priority**: Validation prevents confusing failures deep in the workspace startup flow. Users get immediate, actionable feedback.

**Independent Test**: Set `CLAUDE_CODE_USE_VERTEX=1` but leave `ANTHROPIC_VERTEX_PROJECT_ID` unset. Attempt to create a Claude workspace with `auth: vertex`. Verify the error message identifies the missing `ANTHROPIC_VERTEX_PROJECT_ID`.

**Acceptance Scenarios**:

1. **Given** a user selects Claude with Vertex auth mode, **When** `ANTHROPIC_VERTEX_PROJECT_ID` is not set, **Then** startup fails with a message listing the missing required variable.
2. **Given** a file credential (e.g., `GOOGLE_APPLICATION_CREDENTIALS`) points to a non-existent file, **When** validation runs, **Then** the error message identifies the file path and suggests how to fix it.
3. **Given** all required credentials are present, **When** validation runs, **Then** no errors are raised and workspace creation proceeds normally.

---

### Edge Cases

- What happens when two agents declare the same env var with different values (e.g., different `GOOGLE_CLOUD_PROJECT` settings)? The merge function detects the conflict and returns an error suggesting `--exclude` to resolve it.
- What happens when a credential spec declares `UnsetVars` that conflict with another agent's required vars? The system applies unset only within the scope of that agent's auth mode, not globally.
- What happens when the `internal/credential` package's `Resolve` function encounters a file credential with a tilde path (`~/...`) on a platform without a home directory? The path expansion returns the literal tilde path, and validation catches the missing file.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: All workspace types (Podman, Compose, K8s, SSH, OpenShell, local) MUST resolve credentials through the `internal/credential` package using agent-declared `CredentialSpec` data, not through hardcoded profile maps.
- **FR-002**: The deprecated `KnownProviderProfiles` map MUST be removed from `internal/openshell/credentials.go`. All callers MUST migrate to `credential.DetectAll()` or `credential.Detect()`.
- **FR-003**: The deprecated `BuildCredentialSet` and `detectAuthMode` functions MUST be removed from `internal/ssh/credentials.go`. All callers MUST migrate to `credential.Resolve()`.
- **FR-004**: The shared credential package MUST handle env var resolution, file credential resolution, credential merging with conflict detection, and validation without any agent-specific knowledge.
- **FR-005**: Existing Claude Code credential flows (API key, Vertex, Bedrock) MUST continue to work identically after migration. No user-visible behavior change for single-agent scenarios.
- **FR-006**: The `DetectCredentials` function in `openshell/credentials.go` MUST be replaced by `credential.DetectAll()`. The `ResolveCredentials` function MUST be replaced by `credential.Resolve()` called per-agent.
- **FR-007**: The `InjectEnvVars` and `UploadFileCredential` functions MUST be removed from `openshell/credentials.go`. Credential injection into OpenShell sandboxes MUST use `credential.InjectOpenShell()` from the shared credential package, bridged via an `OpenShellClientAdapter` that wraps the SDK client interface.
- **FR-008**: The `WriteCredentialFile` and `CopyCredentialFile` functions MUST be removed from `ssh/credentials.go`. Credential injection into SSH workspaces MUST use `credential.InjectSSH()` from the shared credential package, which the SSH client already satisfies directly.

### Key Entities

- **CredentialSpec**: Declared by each agent, describes one auth mode with its env vars, file credentials, endpoints, and priority.
- **ResolvedCredentials**: The output of resolving a spec against the host environment, containing env var values, file paths, and unset instructions.
- **DetectedMode**: Links an agent name to a resolved credential spec, used for multi-agent credential scanning.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Zero references to `KnownProviderProfiles`, `BuildCredentialSet`, or `detectAuthMode` remain in non-test production code after migration.
- **SC-002**: All existing credential integration tests pass without modification (demonstrating backward compatibility).
- **SC-003**: Creating workspaces with each supported auth mode (API, Vertex, Bedrock) produces identical credential sets before and after migration.
- **SC-004**: The `internal/credential` package has complete test coverage for detect, resolve, merge, validate, and transport operations, with at least one test per agent auth mode.

## Out of Scope

- **Agent definition changes**: The `CredentialSpec` type and agent manifest structure (from feature 066) are not modified. This feature consumes those types, not changes them.
- **New auth mode support**: No new authentication modes are added. Only existing modes (API key, Vertex, Bedrock) are migrated to the new resolution path.
- **Provider creation logic**: OpenShell provider creation calls (`google-cloud`, `anthropic`, `github`, etc.) remain unchanged. Only the credential detection and resolution that feeds into those calls changes.
- **User-facing documentation**: This is an internal refactoring with no user-visible behavior changes (FR-005). No CLI reference, configuration reference, or Antora guide updates are required. If implementation reveals any user-visible change, documentation MUST be added before merging.

## Assumptions

- The `internal/agent` package and `CredentialSpec` types are stable and will not change during this work. They were introduced in feature 066 (agent abstraction).
- The `internal/credential` package's `Detect`, `DetectAll`, `Resolve`, `MergeCredentials`, and `Validate` functions are already correct and tested. This feature migrates callers to them, not reimplements them.
- OpenShell provider types (google-cloud, anthropic, github, etc.) remain unchanged. Only the credential detection and resolution path changes, not the provider creation calls.
- Workspace definitions that store an explicit `auth_mode` field continue to work. The migration changes how credentials are resolved for a given mode, not how the mode is selected or stored.
- The Compose and Podman workspace types delegate credential handling through the build manifest layer, which calls into `openshell/credentials.go`. Migrating the openshell functions covers these workspace types transitively.
- The K8s workspace type handles credentials through its own Secret/ConfigMap injection path. During planning, verify whether K8s credential handling uses the deprecated functions. If it does, include K8s migration in the plan. If it bypasses them, no changes are needed there.
