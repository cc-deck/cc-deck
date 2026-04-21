# Feature Specification: Environment-to-Workspace Internal Rename

**Feature Branch**: `040-env-to-workspace-rename`  
**Created**: 2026-04-21  
**Status**: Draft  
**Input**: User description: "Align internal Go types, package name, config file names, and build command descriptions with the CLI's existing 'workspace' terminology"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Consistent Terminology for Contributors (Priority: P1)

A developer reading or modifying the cc-deck Go codebase encounters types, constants, and package paths that use "workspace" terminology consistently, matching the CLI commands (`ws new`, `ws attach`, `ws list`) they already use. There is no cognitive disconnect between the user-facing vocabulary and the internal code.

**Why this priority**: The primary goal of this feature is eliminating the terminology split. Every other story depends on this rename completing correctly.

**Independent Test**: After the rename, searching the Go codebase for `Environment` (as a type, constant, or package-qualifying identifier in the `internal/env` package) returns zero results. All references use `Workspace` or the `ws` package qualifier instead.

**Acceptance Scenarios**:

1. **Given** the cc-deck Go codebase, **When** a developer searches for `EnvironmentDefinition`, `EnvironmentType`, `EnvironmentState`, or any `Environment`-prefixed type, **Then** no results are found; the equivalent `Workspace`-prefixed types exist instead.
2. **Given** the Go import paths, **When** a developer looks for the workspace types package, **Then** it is located at `internal/ws/` (not `internal/env/`).
3. **Given** the renamed codebase, **When** `make test` and `make lint` are run, **Then** both pass with no errors.

---

### User Story 2 - Config File Migration (Priority: P2)

An existing cc-deck user who has an `environments.yaml` config file upgrades to the new version. The tool reads the existing file seamlessly. On the next write operation, the file is saved as `workspaces.yaml`. The user does not lose any workspace definitions.

**Why this priority**: This is the only user-visible change. Existing installations must continue to work without manual intervention.

**Independent Test**: Place an `environments.yaml` file in the XDG config directory, run any `ws` command that reads definitions, confirm it loads successfully. Run a write operation, confirm `workspaces.yaml` is created and `environments.yaml` is no longer needed.

**Acceptance Scenarios**:

1. **Given** an existing `environments.yaml` in the config directory and no `workspaces.yaml`, **When** a `ws list` command runs, **Then** definitions are loaded from `environments.yaml`.
2. **Given** an existing `environments.yaml` only, **When** a write operation occurs (e.g., `ws new`), **Then** the file is saved as `workspaces.yaml`.
3. **Given** both `workspaces.yaml` and `environments.yaml` exist, **When** definitions are loaded, **Then** `workspaces.yaml` takes precedence.

---

### User Story 3 - Environment Variable Migration (Priority: P3)

A user or CI pipeline that sets `CC_DECK_DEFINITIONS_FILE` to override the definitions file path continues to work. The new variable name `CC_DECK_WORKSPACES_FILE` is also recognized, with the new name taking precedence if both are set.

**Why this priority**: Fewer users rely on environment variable overrides than on the config file directly. Backward compatibility is still required but affects a smaller audience.

**Independent Test**: Set `CC_DECK_DEFINITIONS_FILE` to a custom path, confirm it is honored. Set `CC_DECK_WORKSPACES_FILE` to a different path, confirm it takes precedence. Unset both, confirm the default path is used.

**Acceptance Scenarios**:

1. **Given** `CC_DECK_DEFINITIONS_FILE` is set and `CC_DECK_WORKSPACES_FILE` is not, **When** definitions are loaded, **Then** the path from `CC_DECK_DEFINITIONS_FILE` is used.
2. **Given** both `CC_DECK_DEFINITIONS_FILE` and `CC_DECK_WORKSPACES_FILE` are set, **When** definitions are loaded, **Then** `CC_DECK_WORKSPACES_FILE` takes precedence.
3. **Given** neither variable is set, **When** definitions are loaded, **Then** the default `workspaces.yaml` path is used (with `environments.yaml` fallback).

---

### User Story 4 - Build Command Descriptions (Priority: P3)

A user invoking the build or capture commands sees descriptions that reference "workspace" instead of "environment," consistent with all other CLI terminology.

**Why this priority**: Cosmetic alignment. No functional impact, but prevents confusion for new users reading command help text.

**Independent Test**: Inspect the build and capture command description strings; they reference "workspace" rather than "environment."

**Acceptance Scenarios**:

1. **Given** the build command metadata, **When** the description is displayed, **Then** it reads "Build workspace" (not "Build environment").
2. **Given** the capture command metadata, **When** the description is displayed, **Then** it reads "Capture workspace" (not "Capture environment").

### Edge Cases

- What happens when `environments.yaml` exists but is corrupted or empty? Behavior should be identical to pre-rename: the existing error handling applies unchanged.
- What happens when a user downgrades to a version that only knows `environments.yaml`? The old version will not find `workspaces.yaml` and behave as if no definitions exist. This is acceptable; downgrade is not a supported path.
- What happens when third-party code imports `internal/env`? The `internal/` prefix means no external consumers exist (Go enforces this). The rename is safe.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: All Go type names containing "Environment" MUST be renamed to use "Workspace" (e.g., `EnvironmentDefinition` to `WorkspaceDefinition`, `EnvironmentType` to `WorkspaceType`).
- **FR-002**: The Go package at `internal/env/` MUST be renamed to `internal/ws/`.
- **FR-003**: All import paths referencing `internal/env` MUST be updated to `internal/ws`.
- **FR-004**: The definitions config file MUST be renamed from `environments.yaml` to `workspaces.yaml`.
- **FR-005**: Loading definitions MUST check for `workspaces.yaml` first, then fall back to `environments.yaml` for backward compatibility.
- **FR-006**: Writing definitions MUST always use the `workspaces.yaml` filename.
- **FR-007**: The environment variable `CC_DECK_DEFINITIONS_FILE` MUST continue to be honored, with `CC_DECK_WORKSPACES_FILE` as the preferred name taking precedence when both are set.
- **FR-008**: Build command descriptions in `cc-deck.build.md` and `cc-deck.capture.md` MUST reference "workspace" instead of "environment."
- **FR-009**: All existing tests MUST pass after the rename with no logic changes.
- **FR-010**: All linting checks MUST pass after the rename.

### Key Entities

- **WorkspaceDefinition**: A named configuration describing how to set up a development workspace (type, image, host, etc.).
- **WorkspaceInstance**: A runtime state record tracking an active workspace's status, timestamps, and attached sessions.
- **WorkspaceType**: An enumeration of workspace backend types (container, SSH, compose, K8s deploy, K8s sandbox, local).
- **WorkspaceState**: An enumeration of workspace lifecycle states (running, stopped, available, creating, error, unknown).

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Zero occurrences of `Environment`-prefixed types or `internal/env` import paths remain in the Go codebase after the rename.
- **SC-002**: `make test` passes with all existing tests succeeding unchanged (no test logic modifications).
- **SC-003**: `make lint` passes with no warnings or errors.
- **SC-004**: An existing `environments.yaml` file is loaded correctly without user intervention after the upgrade.
- **SC-005**: Build and capture command descriptions contain "workspace" terminology.

## Assumptions

- This is a pure mechanical rename with no behavioral or logic changes to any function.
- The `internal/` package prefix guarantees no external Go modules import `internal/env`, so the package rename has no downstream consumers.
- The backward-compatible fallback for `environments.yaml` will be removed in a future release (outside the scope of this feature).
- Documentation updates (Antora docs, README, landing page) for the terminology change are out of scope and will follow separately.
- The CLI command rename from `env` to `ws` is handled separately in spec 039 and is already complete at the CLI level.
- The `Environment` field in `internal/compose/generate.go` refers to Docker Compose's YAML `environment:` key, not cc-deck's workspace concept. It MUST NOT be renamed.
