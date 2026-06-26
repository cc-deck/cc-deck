# Feature Specification: OpenShell Native Vertex Provider

**Feature Branch**: `073-openshell-native-vertex`
**Created**: 2026-06-26
**Status**: Draft
**Input**: Replace cc-deck's homegrown Vertex AI credential handling for OpenShell workspaces with OpenShell's native google-cloud provider type

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Create OpenShell Workspace with Vertex AI (Priority: P1)

A user with GCP Application Default Credentials configured on their host creates an OpenShell workspace that uses Claude Code with Vertex AI. The workspace creation uses OpenShell's native `google-cloud` provider instead of cc-deck's custom file upload workaround.

**Why this priority**: This is the core functional change. Without it, OpenShell workspaces continue using the deprecated homegrown Vertex credential injection.

**Independent Test**: Can be fully tested by running `cc-deck ws new --type openshell` with Vertex credentials configured, and verifying that Claude Code inside the sandbox can make API calls through Vertex AI.

**Acceptance Scenarios**:

1. **Given** a host with `gcloud` ADC configured and `ANTHROPIC_VERTEX_PROJECT_ID` set, **When** the user runs `cc-deck ws new --type openshell`, **Then** cc-deck creates an OpenShell `google-cloud` provider via `--from-gcloud-adc` and the sandbox starts with Claude Code able to use Vertex AI.
2. **Given** a host with `gcloud` ADC configured, **When** the workspace starts, **Then** no `GOOGLE_APPLICATION_CREDENTIALS` file is uploaded into the sandbox (OpenShell's metadata emulator handles GCP auth instead).
3. **Given** a running OpenShell workspace with Vertex AI, **When** Claude Code makes an API call, **Then** the request is authenticated through OpenShell's GCE metadata emulator and proxy, and the sandbox process never holds real GCP credentials.

---

### User Story 2 - Non-OpenShell Workspaces Unchanged (Priority: P1)

Users who create container, SSH, K8s, or compose workspaces with Vertex AI see no change in behavior. The existing credential injection (env vars + file upload) continues to work as before.

**Why this priority**: Preventing regressions in non-OpenShell workspace types is equally critical to the new functionality.

**Independent Test**: Can be tested by running `cc-deck ws new --type container` with Vertex credentials and verifying that `GOOGLE_APPLICATION_CREDENTIALS` file is still mounted and Vertex env vars are injected.

**Acceptance Scenarios**:

1. **Given** a host with Vertex credentials, **When** the user creates a container workspace, **Then** the `GOOGLE_APPLICATION_CREDENTIALS` file is mounted into the container and Vertex env vars are injected as before.
2. **Given** a host with Vertex credentials, **When** the user creates an SSH workspace, **Then** the credential file is copied to the remote host and env vars are set in the remote shell as before.

---

### User Story 3 - Credential Detection During Capture (Priority: P2)

During the capture phase, cc-deck detects Vertex credentials and records them in the manifest. For OpenShell workspaces, the manifest entry uses the `google-cloud` provider type instead of the old `vertex` or `claude-vertex` types.

**Why this priority**: Capture phase correctness ensures that workspaces created from the manifest use the right provider type.

**Independent Test**: Can be tested by running `cc-deck init --capture` with Vertex credentials set and inspecting the generated manifest for correct credential entries.

**Acceptance Scenarios**:

1. **Given** a host with `CLAUDE_CODE_USE_VERTEX=1` and `ANTHROPIC_VERTEX_PROJECT_ID` set, **When** the user runs the capture wizard for an OpenShell workspace, **Then** the detected credential entry uses the appropriate type for OpenShell's native `google-cloud` provider.

---

### Edge Cases

- What happens when `gcloud` ADC is not configured but `ANTHROPIC_VERTEX_PROJECT_ID` is set? The workspace creation should warn that GCP credentials are missing and skip the `google-cloud` provider creation.
- What happens when an existing workspace was created with the old `vertex` provider type? The old manifest entries should continue to work (the `ResolveCredentials` function handles unknown types gracefully).
- What happens when OpenShell's `google-cloud` provider creation fails (e.g., expired ADC token)? The error should propagate clearly so the user knows to refresh their credentials with `gcloud auth application-default login`.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: When creating an OpenShell workspace with Vertex AI credentials, cc-deck MUST create an OpenShell provider of type `google-cloud` using the `--from-gcloud-adc` flag instead of injecting env vars directly.
- **FR-002**: cc-deck MUST pass the GCP project ID and region as provider config options (e.g., `--config project_id=... --config region=global`).
- **FR-003**: cc-deck MUST continue injecting `CLAUDE_CODE_USE_VERTEX=1`, `ANTHROPIC_VERTEX_PROJECT_ID`, and `CLOUD_ML_REGION` as env vars into the sandbox (Claude Code requires these regardless of auth method).
- **FR-004**: cc-deck MUST NOT upload `GOOGLE_APPLICATION_CREDENTIALS` files into OpenShell sandboxes (OpenShell's metadata emulator handles GCP auth).
- **FR-005**: cc-deck MUST remove the standalone `vertex` provider profile from `KnownProviderProfiles` (dead code since OpenShell now handles it natively).
- **FR-006**: cc-deck MUST remove Vertex-specific domains (e.g., `aiplatform.googleapis.com`, `oauth2.googleapis.com`) from the OpenShell network policy generation path (OpenShell auto-generates policy from the `google-cloud` provider).
- **FR-007**: cc-deck MUST preserve the Vertex domain allowlist in `internal/network/builtin.go` for non-OpenShell workspace types that still need explicit domain filtering.
- **FR-008**: The `claude-vertex` profile in `KnownProviderProfiles` MUST be updated to use OpenShell's `google-cloud` provider type instead of setting `SkipProvider: true` with direct env var injection.
- **FR-009**: Non-OpenShell workspace types (container, SSH, K8s, compose) MUST retain their existing Vertex credential handling unchanged.
- **FR-010**: Profile configuration MUST skip the `credentials_secret` prompt when the workspace type is OpenShell (OpenShell manages secrets through its provider system).

### Key Entities

- **KnownProviderProfile**: Maps credential types to OpenShell provider profiles, env vars, and endpoints. The `claude-vertex` and `vertex` entries are the primary targets for modification and removal.
- **ProviderConfig**: Resolved configuration for creating an OpenShell provider at workspace start. The `SkipProvider` flag and file credential fields become unnecessary for the Vertex path.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: OpenShell workspaces with Vertex AI start successfully using OpenShell's native `google-cloud` provider, with Claude Code able to complete API calls through Vertex AI.
- **SC-002**: No `GOOGLE_APPLICATION_CREDENTIALS` file is uploaded into OpenShell sandboxes during workspace creation.
- **SC-003**: Non-OpenShell workspace types (container, SSH, K8s, compose) continue to function identically with Vertex credentials.
- **SC-004**: The `vertex` provider profile is removed from `KnownProviderProfiles` and no references to it remain in the OpenShell credential path.
- **SC-005**: All existing tests pass without modification (or with updates only to reflect the new provider type).

## Assumptions

- OpenShell version in use includes the GCE metadata emulator (merged in NVIDIA/OpenShell PR #1763). No backwards compatibility with older versions is needed.
- The `google-cloud` provider type does not require the `providers_v2_enabled` flag (that flag is for `inference.local` only, per the gist).
- Users have `gcloud` Application Default Credentials configured on their host before creating an OpenShell workspace with Vertex AI.
- The `CLAUDE_CODE_USE_VERTEX=1` env var and companion project/region vars are still required by Claude Code even when GCP auth is handled by the metadata emulator. These are not secrets and are injected as plain env vars.
- The `UploadFileCredential` function in `openshell/credentials.go` may still be used for other file-based credentials (not Vertex), so it is not removed entirely.
