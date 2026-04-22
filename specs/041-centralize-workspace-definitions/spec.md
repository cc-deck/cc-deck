# Feature Specification: Centralize Workspace Definitions

**Feature Branch**: `041-centralize-workspace-definitions`
**Created**: 2026-04-21
**Status**: Draft
**Input**: Centralize workspace definitions into a single global store, replace project-local definitions with git-committable templates, simplify default workspace resolution

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Create Workspace from Template (Priority: P1)

A developer clones a project that includes a `.cc-deck/workspace-template.yaml` file. They run `cc-deck ws new --type ssh` and cc-deck reads the template, prompts for any placeholder values (like SSH username), creates the workspace definition in the central store, and registers the instance. The developer can immediately attach.

**Why this priority**: This is the primary workflow for onboarding to a project. Without it, users must manually specify all flags on every `ws new` invocation.

**Independent Test**: Can be tested by placing a `workspace-template.yaml` with `{{placeholder}}` values in a project directory, running `ws new --type <type>`, verifying the prompt appears, and confirming the resolved definition lands in `~/.config/cc-deck/workspaces.yaml`.

**Acceptance Scenarios**:

1. **Given** a project directory with `.cc-deck/workspace-template.yaml` containing an SSH variant with `{{ssh_user}}` placeholder, **When** user runs `cc-deck ws new --type ssh`, **Then** the system prompts for `ssh_user`, resolves the placeholder, stores the definition in `~/.config/cc-deck/workspaces.yaml` with `project-dir` set, and creates the workspace instance.
2. **Given** a template with a single variant (only `container`), **When** user runs `cc-deck ws new` without `--type`, **Then** the system uses the single variant without asking.
3. **Given** a template with multiple variants, **When** user runs `cc-deck ws new` without `--type`, **Then** the system returns an error listing available types.
4. **Given** a template with name `cc-deck`, **When** user runs `cc-deck ws new my-custom-name --type container`, **Then** the workspace is created with name `my-custom-name` (explicit name overrides template name).

---

### User Story 2 - Default Workspace Resolution (Priority: P1)

A developer has multiple workspaces and runs `cc-deck ws attach` without specifying a name. The system selects the most recently attached workspace and prints which one it chose. If only one workspace exists, it uses that one. If no recent attachment exists and multiple workspaces are present, the system returns an error with guidance.

**Why this priority**: Equally critical to templates because it affects every `ws attach`, `ws stop`, and `ws delete` invocation without explicit names. The current directory-walking approach is confusing and unpredictable.

**Independent Test**: Can be tested by creating multiple workspace instances, attaching to one, then running `ws attach` without arguments and verifying the most recently attached workspace is selected.

**Acceptance Scenarios**:

1. **Given** exactly one workspace instance exists, **When** user runs `cc-deck ws attach` without arguments, **Then** the system uses that workspace and prints `Using workspace "X"`.
2. **Given** multiple workspace instances exist and workspace "marovo" was most recently attached, **When** user runs `cc-deck ws attach` without arguments, **Then** the system selects "marovo" and prints `Using workspace "marovo"`.
3. **Given** multiple workspace instances exist and none have been attached yet, **When** user runs `cc-deck ws attach` without arguments, **Then** the system returns an error: `no workspace specified; run 'cc-deck ws list' to see available workspaces`.
4. **Given** no workspace instances exist, **When** user runs `cc-deck ws attach`, **Then** the system returns an error: `no workspaces found; create one with 'cc-deck ws new'`.
5. **Given** the same default resolution rules apply to all workspace subcommands (`ws stop`, `ws delete`, `ws start`, `ws status`, `ws update`, `ws refresh-creds`), **When** user runs `cc-deck ws delete` without arguments and only one workspace exists, **Then** the system uses that workspace and prints `Using workspace "X"` before proceeding with deletion.

---

### User Story 3 - Central Definition Store (Priority: P1)

All workspace definitions are stored in a single file (`~/.config/cc-deck/workspaces.yaml`). When a developer creates a workspace (with or without a template), the definition is written to this central file. The `ws list` command shows all workspaces with their project association.

**Why this priority**: This is the foundational data model change that stories 1 and 2 depend on. Without central storage, templates and simplified resolution cannot work.

**Independent Test**: Can be tested by creating workspaces of different types, verifying all definitions appear in `~/.config/cc-deck/workspaces.yaml`, and confirming `ws list` shows the PROJECT column correctly.

**Acceptance Scenarios**:

1. **Given** a user creates a workspace with `cc-deck ws new test --type container --image foo:latest`, **When** the workspace is created, **Then** the definition is stored in `~/.config/cc-deck/workspaces.yaml` (not in any project-local file).
2. **Given** multiple workspaces exist with different `project-dir` values, **When** user runs `cc-deck ws list`, **Then** the output shows a PROJECT column with `filepath.Base(project-dir)` or "-" for workspaces without a project association.
3. **Given** a workspace named "foo" of type "container" exists, **When** user runs `cc-deck ws new foo --type ssh`, **Then** the system auto-suffixes the name to "foo-ssh" and stores both definitions.
4. **Given** a workspace named "foo" of type "container" exists, **When** user runs `cc-deck ws new foo --type container`, **Then** the system returns an error: `workspace "foo" already exists (type: container); delete it first`.

---

### User Story 4 - Workspace Update with Sync Repos (Priority: P2)

A developer runs `cc-deck ws update marovo --sync-repos` to clone missing repositories on a remote workspace. The repos list is read from the workspace definition in the central store.

**Why this priority**: Depends on the central store (P1) but is a separate workflow. Users who do not use repos are unaffected.

**Independent Test**: Can be tested by creating an SSH workspace with repos in the central definition, running `ws update --sync-repos`, and verifying repos are cloned on the remote.

**Acceptance Scenarios**:

1. **Given** workspace "marovo" has repos defined in the central definition store, **When** user runs `cc-deck ws update marovo --sync-repos`, **Then** the system reads repos from the central definition and clones missing repos on the remote.
2. **Given** workspace "marovo" has no repos in the central definition, **When** user runs `cc-deck ws update marovo --sync-repos`, **Then** the system returns an error: `no repos defined in workspace definition for "marovo"`.

---

### User Story 5 - Cleanup of Legacy Code (Priority: P3)

After the central store and templates are working, the project-local definition code paths are removed. The `--global` and `--local` flags on `ws new` are removed. The project registry in state.yaml is removed. `FindProjectConfig` is simplified to `FindProjectRoot` (used by the build command only).

**Why this priority**: This is cleanup after the functional work is done. It reduces code complexity but delivers no new user-facing value.

**Independent Test**: Can be tested by verifying that `make test` and `make lint` pass, the removed functions no longer exist in the codebase, and existing commands still work.

**Acceptance Scenarios**:

1. **Given** the codebase after cleanup, **When** user runs `cc-deck ws new --global`, **Then** the command returns an error for the unrecognized flag.
2. **Given** the codebase after cleanup, **When** searching for `LoadProjectDefinition`, `SaveProjectDefinition`, `AllProjectWorkspaceNames`, `ListProjects`, `RegisterProject`, `ProjectEntry`, `ProjectStatusStore`, or `ProjectStatusFile`, **Then** none of these symbols exist in the codebase.
3. **Given** a project directory with `.cc-deck/setup/build.yaml`, **When** user runs `cc-deck build`, **Then** the build command still finds the project root correctly using `FindProjectRoot`.

---

### Edge Cases

- What happens when a user runs `ws new` in a directory with an old `.cc-deck/workspace.yaml` file? The file is ignored; the system only reads `workspace-template.yaml` for templates.
- What happens when two different project directories use the same template name? Name collision rules apply (D4): same name + same type errors, same name + different type auto-suffixes.
- What happens when the central `workspaces.yaml` file does not exist yet? The first `ws new` creates it with `version: 1`.
- What happens when `ws attach` is called with no workspaces and no arguments? Clear error: `no workspaces found; create one with 'cc-deck ws new'`.
- What happens when a workspace's `project-dir` points to a directory that no longer exists? The PROJECT column still shows the basename. The workspace remains functional (project-dir is informational, not operational).
- What happens when a template field value needs literal `{{...}}`? This is not supported. The `{{placeholder}}` syntax is intentionally simple; literal double-braces in configuration values are not a realistic use case for workspace definitions.

## Clarifications

### Session 2026-04-22

- Q: Does FR-006 `project-dir` matching require exact path equality, or should a workspace match when cwd is a subdirectory of `project-dir`? → A: Ancestor match (workspace matches if cwd is at or below project-dir).
- Q: Can template placeholders specify default values (e.g., `{{ssh_user:roland}}`)? → A: Yes, `{{name:default}}` syntax supported; prompt shows default, Enter accepts it.
- Q: When `ws new` is run with a template present and explicit CLI flags, how do they interact? → A: Template variant is loaded first, then explicit flags override individual fields (flags take precedence).

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST store all workspace definitions in a single central file at `~/.config/cc-deck/workspaces.yaml`
- **FR-002**: System MUST support `.cc-deck/workspace-template.yaml` as a git-committable template format with type-keyed variants
- **FR-003**: Templates MUST support `{{placeholder}}` syntax on any string field, with interactive prompting during `ws new`. Placeholders MAY include a default value using `{{name:default}}` syntax (e.g., `{{ssh_user:roland}}`); the prompt displays the default and pressing Enter accepts it.
- **FR-004**: Templates MUST support all `WorkspaceDefinition` fields (repos, remote-bg, credentials, allowed-domains, mounts, ports, storage, namespace, identity-file, jump-host, etc.). When a template is present and `ws new` is invoked with explicit CLI flags, the template variant is loaded first and then explicit flags override individual fields (flags take precedence over template values).
- **FR-005**: When `ws new` is called without a workspace name and a template exists, the system MUST use the template's `name` field as default; if no template, use the directory basename
- **FR-006**: When `ws attach` (or other ws subcommands) is called without a name, resolve in two phases: (1) filter workspaces whose `project-dir` is an ancestor of (or equal to) the current working directory; if exactly one match use it, if multiple matches use the most recently attached among them; (2) if no project-dir matches, fall back to the global pool: if one workspace exists use it, if multiple exist use the most recently attached, if no recent attachment exists and multiple workspaces exist, return an error. This two-phase approach ensures project context is respected when available, even when the user is deep in a subdirectory.
- **FR-007**: System MUST print `Using workspace "X"` to stderr when auto-resolving the workspace name
- **FR-008**: Name collision handling: same name + same type MUST error; same name + different type MUST auto-suffix with the type name (e.g., `foo-ssh`). Auto-suffixing applies regardless of whether the name came from a template, directory basename, or explicit argument.
- **FR-009**: When a user provides an explicit name argument to override a template's default name (e.g., `cc-deck ws new my-custom-name --type container`), the explicit name MUST replace the template name. This does not bypass collision handling (FR-008); if the explicit name collides with an existing workspace of a different type, auto-suffixing still applies.
- **FR-010**: The `ws list` output MUST show a PROJECT column derived from `filepath.Base(definition.ProjectDir)`, replacing the current SOURCE column
- **FR-011**: The `ws update --sync-repos` command MUST read repos from the central definition store (not project-local files)
- **FR-012**: The project registry (`Projects` section in state.yaml) MUST be removed; project association MUST be stored in the definition's `project-dir` field
- **FR-013**: The `--global` and `--local` flags on `ws new` MUST be removed
- **FR-014**: `FindProjectConfig` MUST be renamed to `FindProjectRoot` and MUST look for the `.cc-deck/` directory (not a specific file); used by the build command only
- **FR-015**: The following functions MUST be removed: `LoadProjectDefinition`, `SaveProjectDefinition`, `AllProjectWorkspaceNames`, `ListProjects`, `RegisterProject`
- **FR-016**: The `ProjectEntry` type MUST be removed from types.go
- **FR-017**: The `ProjectStatusStore` and `ProjectStatusFile` MUST be removed. With centralized definitions, CLI overrides are unnecessary (users edit the central definition directly) and runtime state fields (`State`, `ContainerName`, `CreatedAt`, `LastAttached`) are already tracked in `WorkspaceInstance` within state.yaml.
- **FR-018**: The `.cc-deck/status.yaml` file MUST no longer be created or read. Existing status files are ignored (no migration needed).

### Key Entities

- **WorkspaceDefinition**: The declarative, user-editable description of a workspace. Stored centrally in `workspaces.yaml`. Contains all configuration fields (type, image, host, repos, etc.) plus a `project-dir` field for project association.
- **WorkspaceTemplate**: A git-committable template file (`.cc-deck/workspace-template.yaml`) containing a name and type-keyed variants with `{{placeholder}}` support. Used as input to `ws new` but not stored. The `name` field is required. Each variant key is a workspace type (`ssh`, `container`, `compose`, `k8s`), and variant bodies use the same fields as `WorkspaceDefinition` (minus `name` and `type`, which are derived from the template structure). Example:
  ```yaml
  name: my-project
  variants:
    ssh:
      host: "{{ssh_user:roland}}@marovo"
      repos:
        - url: https://github.com/org/repo.git
    container:
      image: quay.io/cc-deck/cc-deck-demo
      storage:
        type: named-volume
  ```
- **WorkspaceInstance**: The runtime state for a workspace. Stored in `state.yaml`. Contains status, timestamps, and type-specific runtime fields. Unchanged by this feature.
- **DefinitionStore**: Manages reading/writing workspace definitions from the central file. Extended with `FindByProjectDir` and `AddWithCollisionHandling` methods.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: All workspace definitions are stored in a single file; no workspace data is read from or written to project-local `workspace.yaml` files
- **SC-002**: Users can create a workspace from a template with placeholder prompting in a single `ws new` command
- **SC-003**: `ws attach` without arguments selects the correct workspace (most recent or sole) in 100% of test cases
- **SC-004**: The `ws list` PROJECT column correctly shows the project name for project-associated workspaces and "-" for standalone ones
- **SC-005**: All tests pass (`make test`) and linter is clean (`make lint`) after implementation
- **SC-006**: The removed code paths (project-local definitions, project registry, `--global`/`--local` flags, `ProjectStatusStore`, `ProjectStatusFile`) have zero remaining references in the codebase

## Assumptions

- The existing `WorkspaceDefinition` struct is sufficient for all template fields; no new definition fields are needed. The existing `project-dir` field (currently used for compose project directory only) is repurposed as a general project association field for all workspace types. Compose-specific behavior that derives paths from `project-dir` must be preserved.
- Users are willing to recreate workspaces that were previously defined only in project-local files (no automated migration)
- The `.cc-deck/` directory will continue to exist in project roots for build/setup purposes, even though it is no longer used as a workspace marker
- The build command (`cc-deck build`) reads exclusively from `.cc-deck/setup/build.yaml` and is unaffected by this change
- Interactive placeholder prompting during `ws new` is acceptable UX for template-based workspace creation
