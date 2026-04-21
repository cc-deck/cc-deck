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

### User Story 2 - Config and Environment Variable Rename (Priority: P2)

The definitions config file is renamed from `environments.yaml` to `workspaces.yaml`, and the environment variable override is renamed from `CC_DECK_DEFINITIONS_FILE` to `CC_DECK_WORKSPACES_FILE`. The top-level YAML key changes from `environments:` to `workspaces:`. No backward compatibility with the old names is provided.

**Why this priority**: This is the user-visible part of the rename. It is a straightforward name change with no migration logic.

**Independent Test**: Create a `workspaces.yaml` with a `workspaces:` key, run `ws list`, confirm it loads. Set `CC_DECK_WORKSPACES_FILE`, confirm it is honored. Verify the old names (`environments.yaml`, `CC_DECK_DEFINITIONS_FILE`) are no longer recognized.

**Acceptance Scenarios**:

1. **Given** a `workspaces.yaml` in the config directory, **When** `ws list` runs, **Then** definitions are loaded successfully.
2. **Given** only an `environments.yaml` (no `workspaces.yaml`), **When** `ws list` runs, **Then** no definitions are found.
3. **Given** `CC_DECK_WORKSPACES_FILE` is set, **When** definitions are loaded, **Then** the path from `CC_DECK_WORKSPACES_FILE` is used.
4. **Given** `CC_DECK_DEFINITIONS_FILE` is set (without `CC_DECK_WORKSPACES_FILE`), **When** definitions are loaded, **Then** it is ignored; the default path is used.

---

### User Story 3 - Build Command Descriptions (Priority: P3)

A user invoking the build or capture commands sees descriptions that reference "workspace" instead of "environment," consistent with all other CLI terminology.

**Why this priority**: Cosmetic alignment. No functional impact, but prevents confusion for new users reading command help text.

**Independent Test**: Inspect the build and capture command description strings; they reference "workspace" rather than "environment."

**Acceptance Scenarios**:

1. **Given** the build command metadata, **When** the description is displayed, **Then** it reads "Build workspace" (not "Build environment").
2. **Given** the capture command metadata, **When** the description is displayed, **Then** it reads "Capture workspace" (not "Capture environment").

### Edge Cases

- What happens when `workspaces.yaml` is corrupted or empty? Behavior is identical to pre-rename: existing error handling applies unchanged.
- What happens when third-party code imports `internal/env`? The `internal/` prefix means no external consumers exist (Go enforces this). The rename is safe.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: All Go type names containing "Environment" MUST be renamed to use "Workspace" (e.g., `EnvironmentDefinition` to `WorkspaceDefinition`, `EnvironmentType` to `WorkspaceType`).
- **FR-002**: The Go package at `internal/env/` MUST be renamed to `internal/ws/`.
- **FR-003**: All import paths referencing `internal/env` MUST be updated to `internal/ws`.
- **FR-004**: The definitions config file MUST be renamed from `environments.yaml` to `workspaces.yaml`. The top-level YAML key MUST change from `environments:` to `workspaces:`. No backward compatibility with the old filename or key is required.
- **FR-005**: The environment variable `CC_DECK_DEFINITIONS_FILE` MUST be renamed to `CC_DECK_WORKSPACES_FILE`. The old variable name is no longer recognized.
- **FR-006**: Build command descriptions in `cc-deck.build.md` and `cc-deck.capture.md` MUST reference "workspace" instead of "environment."
- **FR-007**: User-facing error and log messages that reference "environment" (e.g., "environment %q already exists") MUST be updated to say "workspace."
- **FR-008**: All existing tests MUST pass after the rename with no logic changes beyond the terminology updates specified above.
- **FR-009**: All linting checks MUST pass after the rename.

### Key Entities

- **WorkspaceDefinition**: A named configuration describing how to set up a development workspace (type, image, host, etc.).
- **WorkspaceInstance**: A runtime state record tracking an active workspace's status, timestamps, and attached sessions.
- **WorkspaceType**: An enumeration of workspace backend types (container, SSH, compose, K8s deploy, K8s sandbox, local).
- **WorkspaceState**: An enumeration of workspace lifecycle states (running, stopped, available, creating, error, unknown).

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Zero occurrences of `Environment`-prefixed types or `internal/env` import paths remain in the Go codebase after the rename.
- **SC-002**: `make test` passes with all existing tests succeeding (test assertions updated only for renamed strings, no logic modifications).
- **SC-003**: `make lint` passes with no warnings or errors.
- **SC-004**: The definitions file is `workspaces.yaml` with a `workspaces:` top-level key. The environment variable is `CC_DECK_WORKSPACES_FILE`.
- **SC-005**: Build and capture command descriptions contain "workspace" terminology.
- **SC-006**: No user-facing error messages reference "environment" when they mean a cc-deck workspace.

## Clarifications

### Session 2026-04-21

- Q: After the first write creates `workspaces.yaml`, what happens to the old `environments.yaml`? → A: No backward compatibility. Old filenames, YAML keys, and environment variable names are simply not recognized. Users must adopt the new names.

## Assumptions

- This is a mechanical rename with no behavioral or logic changes to any function, aside from updating user-facing strings (error messages, command descriptions) from "environment" to "workspace."
- No backward compatibility is provided for the old `environments.yaml` filename, the old `environments:` YAML key, or the old `CC_DECK_DEFINITIONS_FILE` environment variable. Users must adopt the new names.
- The `internal/` package prefix guarantees no external Go modules import `internal/env`, so the package rename has no downstream consumers.
- YAML serialization tags in `state.yaml` (e.g., `yaml:"type"`, `yaml:"instances"`) are not renamed. Only Go identifiers change. No state file migration is required.
- The `CC_DECK_STATE_FILE` environment variable does not contain "environment" in its name and requires no change.
- Documentation updates (Antora docs, README, landing page) for the terminology change are out of scope and will follow separately.
- The CLI command rename from `env` to `ws` is handled separately in spec 039 and is already complete at the CLI level.
- The `Environment` field in `internal/compose/generate.go` refers to Docker Compose's YAML `environment:` key, not cc-deck's workspace concept. It MUST NOT be renamed.
