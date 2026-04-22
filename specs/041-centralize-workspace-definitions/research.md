# Research: Centralize Workspace Definitions

**Feature**: 041-centralize-workspace-definitions
**Date**: 2026-04-22

## Decision 1: Template File Format and Parsing

**Decision**: Use a custom YAML structure with `name` (required), `variants` (map of type to definition fields), and `{{placeholder:default}}` syntax for interactive prompting.

**Rationale**: The template format mirrors `WorkspaceDefinition` fields (minus `name` and `type`) under type-keyed variant blocks. This keeps the template familiar to users who already edit `workspaces.yaml`. The `{{placeholder}}` syntax is simple regex-replaceable and avoids pulling in a Go template engine, which would be overkill for string field substitution.

**Alternatives considered**:
- Go `text/template`: Too powerful for the use case (conditionals, loops not needed). Introduces security surface area and confusing error messages for YAML authors.
- envsubst / `$VAR` syntax: Conflicts with shell expansion if users cat the file. No default value support without additional conventions.
- Helm-style values.yaml: Overengineered for single-file workspace definitions.

## Decision 2: Default Workspace Resolution Strategy

**Decision**: Two-phase resolution: (1) filter by `project-dir` ancestor match against cwd, (2) fall back to global recency-based selection.

**Rationale**: The current `resolveWorkspaceName` walks the filesystem to find `.cc-deck/workspace.yaml`, which is inherently project-scoped. Removing project-local files means resolution must use the central store's `project-dir` field for project scoping. Ancestor matching (cwd is at or below project-dir) preserves the convenience of the current walk-up behavior without requiring an actual file to exist.

**Alternatives considered**:
- Exact cwd match only: Too strict; users deep in `src/cmd/` would get "no workspace found."
- Git-root-then-match: Adds a `git rev-parse` call on every ws subcommand. Slower and fails outside git repos.
- Global-only (no project scoping): Loses the "right workspace for this project" behavior that users expect.

## Decision 3: Collision Handling Approach

**Decision**: Auto-suffix with type name on same-name + different-type collision. Error on same-name + same-type.

**Rationale**: Users creating workspaces from templates will naturally reuse the template name (typically the project name). Having both `myproject` (container) and `myproject` (ssh) is a common scenario. Auto-suffixing to `myproject-ssh` avoids forcing the user to invent unique names while keeping names predictable and discoverable.

**Alternatives considered**:
- Always error on collision: Annoying for the common multi-type case.
- Numeric suffix (`myproject-2`): Uninformative; doesn't tell the user which type it is.
- Interactive prompt on collision: Breaks scriptability.

## Decision 4: ProjectStatusStore Removal

**Decision**: Remove `ProjectStatusStore`, `ProjectStatusFile`, and `.cc-deck/status.yaml` entirely. Do not migrate data.

**Rationale**: The status file stores five categories of data, all of which are redundant after centralization:
- `Variant`: No longer needed (variants are a project-local concept that templates replace).
- `State`: Already tracked in `WorkspaceInstance.State` in `state.yaml`.
- `ContainerName`: Already tracked in `WorkspaceInstance.Container.ContainerName`.
- `CreatedAt`, `LastAttached`: Already tracked in `WorkspaceInstance`.
- `Overrides`: Unnecessary because users can edit the central definition directly.

**Alternatives considered**:
- Keep status.yaml for container name tracking: Redundant with `WorkspaceInstance.Container.ContainerName`.
- Migrate overrides into central definitions: Overrides were a workaround for not being able to edit the committed workspace.yaml. With central definitions, users own the definition and can edit it directly.

## Decision 5: FindProjectRoot Behavior Change

**Decision**: Rename `FindProjectConfig` to `FindProjectRoot`. Change lookup from "directory containing `.cc-deck/workspace.yaml`" to "directory containing `.cc-deck/`".

**Rationale**: After centralization, `.cc-deck/workspace.yaml` no longer exists in project directories. The `.cc-deck/` directory itself (which contains `setup/`, templates, and gitignore) becomes the project marker. The build command is the only remaining caller, and it needs the project root to find `.cc-deck/setup/build.yaml`.

**Alternatives considered**:
- Look for `.cc-deck/setup/` specifically: Too narrow; would break for projects that have `.cc-deck/` but no setup directory.
- Look for `.cc-deck/workspace-template.yaml`: Not all projects will have templates.
- Remove FindProjectRoot entirely and always use git root: Would break for non-git workspace directories.

## Decision 6: project-dir Field Semantic Change

**Decision**: Repurpose the existing `project-dir` field on `WorkspaceDefinition` from "compose project directory" to "project association for all workspace types." Preserve compose-specific path derivation behavior.

**Rationale**: The field already exists in the YAML schema. Adding a separate field would create confusion about which one to use. The compose code derives `.cc-deck/run/` paths from `project-dir`, and this behavior is preserved since the value remains the project root directory.

**Alternatives considered**:
- New field `associated-project`: Adds a new field that is semantically identical to `project-dir` for compose workspaces. Increases confusion.
- Separate `compose-dir` + `project-dir`: Two fields for the same directory. Overengineered.
