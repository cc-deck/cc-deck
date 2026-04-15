# Feature Specification: Environment Lifecycle Fixes

**Feature Branch**: `037-env-lifecycle-fixes`  
**Created**: 2026-04-14  
**Status**: Draft  
**Input**: Fix environment lifecycle: type resolution when explicit name differs from project-local definition, symmetric delete cleanup for SSH environments, source indicator in env list

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Create environment by name from global definition (Priority: P1)

A user has a global environment definition (e.g., `marovo-test` of type `ssh` in `~/.config/cc-deck/environments.yaml`). They run `cc-deck env create marovo-test` from inside a project directory that has its own `.cc-deck/environment.yaml` defining a different environment (e.g., `smoke-full` of type `compose`). The system should use the global definition's type and settings, not the project-local definition's.

**Why this priority**: This is the most confusing bug. It silently creates the wrong environment type, leading to wasted time and cleanup. Users lose trust in the tool when it does something unexpected without error.

**Independent Test**: Can be tested by creating a global SSH definition, then running `env create` with that name from a directory with a different project-local definition. Verify the created environment matches the global definition's type.

**Acceptance Scenarios**:

1. **Given** a global definition `marovo-test` (type: ssh) and a project-local definition `smoke-full` (type: compose), **When** user runs `cc-deck env create marovo-test` without `--type`, **Then** the environment is created as type `ssh` using settings from the global definition.
2. **Given** a project-local definition `smoke-full` (type: compose), **When** user runs `cc-deck env create smoke-full` without `--type`, **Then** the environment is created as type `compose` using project-local settings (existing behavior preserved).
3. **Given** no global definition for `new-env` and a project-local definition `smoke-full`, **When** user runs `cc-deck env create new-env` without `--type`, **Then** the environment is created as type `local` (default fallback) and the project-local definition is not used.
4. **Given** a global definition `marovo-test` (type: ssh), **When** user runs `cc-deck env create marovo-test --type container`, **Then** the `--type` flag overrides the global definition and creates a container environment.

---

### User Story 5 - Explicit location flags for env create (Priority: P2)

A user has both a global and project-local definition with the same name. The automatic precedence rules pick the project-local version, but the user wants the global one. They run `cc-deck env create myenv --global` to force resolution from the global definition store. Conversely, `--local` forces project-local resolution.

**Why this priority**: Provides an escape hatch for the name collision edge case where automatic precedence does not match user intent. Without this, users must rename definitions to disambiguate.

**Independent Test**: Create definitions with the same name in both global and project-local stores. Verify `--global` uses the global definition and `--local` uses the project-local definition.

**Acceptance Scenarios**:

1. **Given** a global definition `myenv` (type: ssh) and a project-local definition `myenv` (type: compose), **When** user runs `cc-deck env create myenv --global`, **Then** the environment is created as type `ssh` using settings from the global definition.
2. **Given** a global definition `myenv` (type: ssh) and a project-local definition `myenv` (type: compose), **When** user runs `cc-deck env create myenv --local`, **Then** the environment is created as type `compose` using settings from the project-local definition.
3. **Given** `--global` is specified but no global definition exists for the name, **When** user runs `cc-deck env create unknown --global`, **Then** the command returns an error indicating no global definition was found.
4. **Given** `--local` is specified but no project-local definition exists, **When** user runs `cc-deck env create unknown --local`, **Then** the command returns an error indicating no project-local definition was found.
5. **Given** both `--global` and `--local` are specified, **When** user runs the command, **Then** the command returns an error (mutually exclusive flags).

---

### User Story 2 - Delete SSH environment cleans up definition (Priority: P1)

A user creates an SSH environment with `cc-deck env create remote-dev --type ssh --host user@host`. Later they delete it with `cc-deck env delete remote-dev`. The definition should be removed from `environments.yaml`, just like container and compose environments already do.

**Why this priority**: Ghost entries in `cc-deck ls` showing "not created" after explicit deletion are confusing and require manual cleanup. This breaks the symmetry users expect between create and delete.

**Independent Test**: Create an SSH environment, delete it, verify the definition no longer appears in `cc-deck ls`.

**Acceptance Scenarios**:

1. **Given** a created SSH environment `remote-dev`, **When** user runs `cc-deck env delete remote-dev --force`, **Then** both the state instance and the global definition are removed.
2. **Given** a created SSH environment `remote-dev`, **When** user runs `cc-deck env delete remote-dev --force` and then `cc-deck ls`, **Then** `remote-dev` does not appear in the list.
3. **Given** an SSH definition `remote-dev` that was never created (status: "not created"), **When** user runs `cc-deck env delete remote-dev`, **Then** the definition is removed (existing fallback behavior preserved).

---

### User Story 3 - List shows environment source (Priority: P2)

A user runs `cc-deck ls` and sees environments from different sources (global definitions, project-local definitions, running instances). A `SOURCE` column indicates where each entry originates, making it clear which config file governs each environment.

**Why this priority**: Without source indication, users cannot tell whether an environment comes from their global config or a project-local file. This makes troubleshooting harder, especially when shadowing occurs.

**Independent Test**: Create environments from both global and project-local definitions, run `cc-deck ls`, verify each row shows the correct source.

**Acceptance Scenarios**:

1. **Given** a global definition `demo` (type: ssh), **When** user runs `cc-deck ls`, **Then** the `demo` row shows `global` in the SOURCE column.
2. **Given** a project-local definition `smoke-full` (type: compose) and the user is inside that project, **When** user runs `cc-deck ls`, **Then** the `smoke-full` row shows `project` in the SOURCE column.
3. **Given** an instance exists but its definition was removed, **When** user runs `cc-deck ls`, **Then** the SOURCE column is empty for that row.

---

### User Story 4 - Remove PATH column from list, show in status (Priority: P2)

The `cc-deck ls` table no longer includes a PATH column, since project-scoped environments only appear when the user is inside that directory (making the path redundant). The full project path is shown in `cc-deck status <name>` instead.

**Why this priority**: Reduces table width clutter while keeping the information accessible where it is most useful: detailed single-environment output.

**Independent Test**: Run `cc-deck ls` from inside a project directory and verify no PATH column exists. Run `cc-deck status <name>` and verify the project path is shown.

**Acceptance Scenarios**:

1. **Given** a project-local environment, **When** user runs `cc-deck ls`, **Then** the table has no PATH column.
2. **Given** a project-local environment `smoke-full`, **When** user runs `cc-deck status smoke-full`, **Then** the output includes the project directory path.

---

### Edge Cases

- What happens when a user provides an explicit name that matches the project-local definition name but with a `--type` flag that differs? The `--type` flag wins (CLI always highest precedence).
- What happens when SSH environment delete fails to reach the remote host? Definition removal should still proceed (best-effort cleanup, matching container/compose behavior).
- What happens when an environment has both a global and project-local definition with the same name? The project-local definition takes precedence when no explicit name is given or when the explicit name matches the project-local name. Otherwise the global definition is used.
- What happens to JSON/YAML output format for `cc-deck ls`? The source field should be included in structured output as well.
- What happens when a project-local definition name collides with a global definition name and the user wants the global version? Use `--global` to force resolution from the global definition store. Without `--global`, the project-local definition takes precedence per default rules.
- What happens when `--global` or `--local` is combined with `--type`? The `--type` flag still wins for type resolution, but the location flag controls which definition's other settings (host, image, etc.) are used.

## Clarifications

### Session 2026-04-14

- Q: When `env create` resolves a name against the global definition (FR-002), should it scaffold a project-local `.cc-deck/environment.yaml`? → A: No. Global-only; no project-local file is created. SOURCE shows `global`.
- Q: How do users disambiguate when global and project-local definitions share a name? → A: Add `--global` and `--local` flags to `env create` as explicit location selectors. Mutually exclusive. Error if the requested definition does not exist in the specified store.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: When `env create` receives an explicit name that differs from the project-local definition's name, the system MUST ignore the project-local definition for type and settings resolution.
- **FR-002**: When `env create` receives an explicit name that differs from the project-local definition's name, the system MUST add a new lookup step: query the global definition store for a matching name and use the matched definition's type and settings. This lookup does not exist in the current code and must be introduced into the type resolution chain.
- **FR-002a**: When `env create` uses a global definition (per FR-002), the system MUST NOT scaffold a project-local `.cc-deck/environment.yaml` for that environment. The environment remains global-only.
- **FR-003**: When `env create` receives an explicit name not found in any definition store and no `--type` flag, the system MUST fall back to `local` type.
- **FR-004**: When `env create` receives an explicit name matching the project-local definition's name, the system MUST use the project-local definition (existing behavior preserved).
- **FR-005**: `SSHEnvironment.Delete()` MUST remove the environment's definition from the definition store (both global and project-local), matching the exact behavior of container and compose environments which call `defs.Remove()` as a best-effort step.
- **FR-006**: Definition removal during delete MUST be best-effort (log warning on failure, do not block the delete operation).
- **FR-007**: `cc-deck ls` MUST include a SOURCE column showing `global`, `project`, or empty for each environment entry.
- **FR-008**: `cc-deck ls` MUST NOT include a PATH column.
- **FR-009**: `cc-deck status <name>` MUST show the project directory path for project-local environments. The path is retrieved from the state store's project registry (not inferred from the current working directory).
- **FR-010**: The SOURCE field MUST be included in JSON and YAML output formats for `cc-deck ls`, using `"source"` as the key name.
- **FR-011**: CLI `--type` flag MUST always take precedence over any definition source (existing behavior preserved).
- **FR-012**: `env create` MUST accept mutually exclusive `--global` and `--local` flags that force definition resolution from the specified store.
- **FR-013**: When `--global` is specified and no global definition exists for the given name, the command MUST return an error.
- **FR-014**: When `--local` is specified and no project-local definition exists, the command MUST return an error.
- **FR-015**: When `--global` or `--local` is combined with `--type`, the location flag controls which definition's settings are used while `--type` overrides the type.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Running `cc-deck env create <global-name>` from inside a project directory creates an environment matching the global definition's type 100% of the time.
- **SC-002**: After `cc-deck env delete` of any environment type (container, compose, SSH), zero ghost "not created" entries remain in `cc-deck ls`.
- **SC-003**: Every row in `cc-deck ls` output shows a correct SOURCE value that matches the actual definition source.
- **SC-004**: All existing tests continue to pass without modification (no regressions).

## Assumptions

- The project-local definition file (`.cc-deck/environment.yaml`) contains exactly one environment definition. Multi-environment project files are out of scope.
- The `env create` scaffolding behavior (creating `.cc-deck/environment.yaml` for new projects) is unchanged for the default case. Scaffolding is skipped when resolving from a global definition (FR-002a) or when `--global` is specified (FR-012).
- The precedence of CLI flags over all definition sources remains unchanged.
- Existing container and compose delete behavior (already removing definitions) is correct and serves as the reference implementation.
