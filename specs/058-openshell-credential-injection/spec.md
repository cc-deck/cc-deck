# Feature Specification: OpenShell Credential Injection

**Feature Branch**: `058-openshell-credential-injection`  
**Created**: 2026-05-19  
**Status**: Draft  
**Input**: Bridge cc-deck's credential system to OpenShell's provider architecture for sandbox workspaces

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Create OpenShell Workspace with API Key Credentials (Priority: P1)

A developer runs `cc-deck ws new --type openshell` for a project whose `build.yaml` declares `claude` and `github` credential providers. cc-deck reads the credential declarations, creates OpenShell providers from the host environment, and attaches them to the sandbox. The developer attaches to the workspace and Claude Code works with full API access, while `gh` commands authenticate against GitHub.

**Why this priority**: Without credential injection, OpenShell sandboxes cannot run Claude Code or access GitHub repos. This is the minimum viable functionality.

**Independent Test**: Run `cc-deck ws new --type openshell` with a manifest declaring `claude` and `github` credentials. Attach, run `claude --version` (verifies Claude Code can reach the API), and run `gh auth status` (verifies GitHub token is available).

**Acceptance Scenarios**:

1. **Given** a `build.yaml` with `credentials: [{type: claude}]` and `ANTHROPIC_API_KEY` set in the host environment, **When** the user runs `cc-deck ws new test --type openshell`, **Then** an OpenShell provider named `cc-deck-test-claude` is created and attached to the sandbox, and Claude Code inside the sandbox can authenticate against the Anthropic API.
2. **Given** a `build.yaml` with `credentials: [{type: github}]` and `GITHUB_TOKEN` set in the host environment, **When** the user creates a workspace, **Then** a `github` provider is created and attached, and `gh` commands inside the sandbox authenticate successfully.
3. **Given** a `build.yaml` with `credentials: [{type: claude}]` but `ANTHROPIC_API_KEY` is NOT set in the host environment, **When** the user creates a workspace, **Then** the command warns about the missing credential and continues without creating the provider (sandbox still starts, but Claude Code will not authenticate).

---

### User Story 2 - Capture Credential Requirements (Priority: P2)

A developer runs `/cc-deck.capture` in a project. The capture wizard detects which credentials are available in the host environment, maps them to known OpenShell provider profiles, and presents the findings for confirmation. The user selects which credentials the workspace needs, and these are recorded in the `credentials` section of `build.yaml`.

**Why this priority**: Without capture, users must manually edit `build.yaml` to add credential entries. Capture automates discovery and reduces configuration errors.

**Independent Test**: Run `/cc-deck.capture` in a project with `ANTHROPIC_API_KEY` and `GITHUB_TOKEN` set. Verify the capture wizard presents detected credentials. Confirm the selections are written to `build.yaml` under a `credentials` section.

**Acceptance Scenarios**:

1. **Given** a host environment with `ANTHROPIC_API_KEY` and `GITHUB_TOKEN` set, **When** the user runs `/cc-deck.capture`, **Then** the capture wizard presents a step showing detected credentials mapped to provider profiles (`claude`, `github`) and asks for confirmation.
2. **Given** the user confirms the detected credentials, **When** capture completes, **Then** `build.yaml` contains a `credentials` section with entries for each confirmed provider type and their associated env var names (never values).
3. **Given** a host environment with no recognized credential env vars, **When** the user runs capture, **Then** the credential step reports "No credentials detected" and allows manual addition.

---

### User Story 3 - File-Based Credential Upload for Vertex (Priority: P3)

A developer using Google Vertex AI has `GOOGLE_APPLICATION_CREDENTIALS` pointing to a service account JSON file. When creating an OpenShell workspace, cc-deck uploads the file into the sandbox after creation and sets the environment variable to point to the uploaded location. The build phase also adds GCP endpoints to the network policy so Vertex API calls can reach Google's servers.

**Why this priority**: Vertex AI requires file-based authentication, which is more complex than API key injection. This is a workaround until OpenShell adds native Vertex provider support.

**Independent Test**: Set `GOOGLE_APPLICATION_CREDENTIALS=/path/to/sa.json` and `ANTHROPIC_VERTEX_PROJECT_ID` in the host environment. Run `ws new` with a manifest declaring a `vertex` credential. Verify the file is uploaded into the sandbox, the env var is set, and the network policy includes GCP endpoints.

**Acceptance Scenarios**:

1. **Given** a `build.yaml` with `credentials: [{type: vertex, file: GOOGLE_APPLICATION_CREDENTIALS}]` and the env var pointing to a valid JSON file, **When** the user creates an OpenShell workspace, **Then** the file is uploaded to `/sandbox/.config/gcloud/credentials.json` inside the sandbox, and `GOOGLE_APPLICATION_CREDENTIALS` is set to that path.
2. **Given** a `vertex` credential in the manifest, **When** the build command generates `policy.yaml`, **Then** the policy includes network entries for `oauth2.googleapis.com:443` and `{region}-aiplatform.googleapis.com:443` (where region comes from `CLOUD_ML_REGION` or defaults to `us-east1`).
3. **Given** `GOOGLE_APPLICATION_CREDENTIALS` is set but the file does not exist, **When** the user creates a workspace, **Then** the command errors with a clear message identifying the missing file.

---

### Edge Cases

- What happens when the manifest declares a credential type not recognized by OpenShell's provider profiles? The system should fall back to `generic` provider type with explicitly passed credentials.
- What happens when a provider with the same name already exists on the gateway? The system should attempt to update the existing provider rather than failing.
- What happens when multiple workspaces share the same credential provider? Providers should be named with a workspace-specific prefix to avoid conflicts.
- What happens when `openshell provider create` fails (gateway unreachable, auth error)? The workspace creation should fail with a clear error message before the sandbox is created.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The `build.yaml` manifest MUST support a `credentials` section containing a list of credential entries, each with a `type` field (required) and optional `env_vars` and `file` fields.
- **FR-002**: The `credentials` section MUST never store actual credential values, only provider type identifiers and environment variable names.
- **FR-003**: The capture command (`/cc-deck.capture`) MUST include a credential detection step that scans the host environment for known credential env vars and maps them to OpenShell provider profiles.
- **FR-004**: The capture credential step MUST present detected credentials to the user for confirmation before writing to the manifest.
- **FR-005**: The `cc-deck ws new --type openshell` command MUST create OpenShell providers for each entry in the manifest's `credentials` section before creating the sandbox.
- **FR-006**: For API-key credential types (`claude`, `github`, `anthropic`, `openai`, `gitlab`, `nvidia`), the system MUST use `openshell provider create --type <type> --from-existing` to resolve credentials from the host environment.
- **FR-007**: For file-based credential types (`vertex`), the system MUST upload the referenced file into the sandbox via `openshell sandbox upload` after the sandbox starts, and set the corresponding environment variable inside the sandbox.
- **FR-008**: Provider names MUST be scoped per-workspace using the pattern `cc-deck-<workspace-name>-<provider-type>` to avoid conflicts between workspaces.
- **FR-009**: When a credential's required env var is not set in the host environment, the system MUST emit a warning and skip that provider (not fail the entire workspace creation).
- **FR-010**: The build command (`/cc-deck.build --target openshell`) MUST add network policy entries for credential endpoints that are not auto-injected by OpenShell providers (e.g., GCP endpoints for Vertex).
- **FR-011**: Provider creation MUST be idempotent. If a provider with the same name already exists, the system MUST update it rather than failing.
- **FR-012**: The manifest's `credentials` section MUST support a `generic` type for custom services, accepting explicit `env_vars` and optional `endpoints` for network policy generation.

### Key Entities

- **CredentialEntry**: A declaration in `build.yaml` describing a credential provider requirement. Contains: type (string), env_vars (list of strings), file (optional string), endpoints (optional list for generic type).
- **OpenShell Provider**: An OpenShell gateway resource that manages credential injection into sandboxes. Created via `openshell provider create` and attached via `--provider` flag on `sandbox create`.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users can create an OpenShell workspace with Claude Code API access in under 30 seconds (provider creation + sandbox start), without manually configuring OpenShell providers.
- **SC-002**: The capture command detects and correctly maps at least the following credential types: `claude`, `github`, `anthropic`, `openai` (covers 90% of use cases).
- **SC-003**: Workspace creation with a `vertex` credential successfully uploads the service account file and sets the env var, enabling Vertex API calls from within the sandbox.
- **SC-004**: Missing credentials produce clear warning messages that identify which env var is expected, without blocking workspace creation for other valid credentials.

## Assumptions

- OpenShell gateway is running and accessible at workspace creation time. cc-deck does not manage gateway lifecycle.
- The `openshell` CLI binary is installed on the host and available in PATH.
- OpenShell's `providers_v2_enabled` setting is active on the gateway for auto-injected network policy entries. If not enabled, the user must enable it manually.
- Credential values are resolved from the host environment at `ws new` time, not stored persistently by cc-deck. Each workspace creation re-reads the current environment.
- Vertex AI support is a workaround (file upload + manual policy entries). When OpenShell adds a native `vertex` provider type, cc-deck should adopt it without manifest schema changes.
- The `generic` provider type in OpenShell accepts arbitrary `--credential KEY=VALUE` pairs, enabling support for any custom service.
- Provider cleanup (deleting providers when a workspace is deleted) is out of scope for the initial implementation but should be considered for a follow-up.
