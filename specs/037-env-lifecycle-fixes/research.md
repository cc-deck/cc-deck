# Research: Environment Lifecycle Fixes

**Branch**: `037-env-lifecycle-fixes` | **Date**: 2026-04-14

## R1: Current Type Resolution Chain in `env create`

**Decision**: The existing 3-tier resolution (`CLI --type` > `projDef.Type` > `local` default) must be extended with a global definition lookup step between project-local and default.

**Findings**:
- `runEnvCreate()` at `cc-deck/internal/cmd/env.go:222-230` resolves type
- When `projDef != nil`, it always uses `projDef.Type` regardless of whether the explicit name matches `projDef.Name`
- No global definition lookup exists in the type resolution path
- The shadowing check at lines 232-238 only warns but does not influence type selection

**New resolution chain**: `CLI --type` > `--global/--local` flag > name-match project-local > global definition lookup > `local` default

**Alternatives considered**:
- Always prefer global over project-local: Rejected because it would break existing project-local workflows
- Error on name mismatch: Rejected because it is valid to create a globally-defined environment from within a project directory

## R2: SSH Delete Definition Cleanup Gap

**Decision**: Add `defs.Remove()` call to `SSHEnvironment.Delete()`, matching container and compose patterns.

**Findings**:
- `SSHEnvironment.Delete()` at `ssh.go:195-220` only calls `store.RemoveInstance()`
- `ContainerEnvironment.Delete()` at `container.go:362-366` calls `defs.Remove()` as best-effort
- `ComposeEnvironment.Delete()` at `compose.go:482-486` calls `defs.Remove()` as best-effort
- Both wrap `defs.Remove()` with a log warning on failure (consistent pattern)
- `DefinitionStore.Remove()` at `definition.go:177-193` only removes from the global file

**Alternatives considered**:
- Separate method for project-local definition removal: Not needed because `defs.Remove()` handles the global store, and project-local definitions are scoped to the `.cc-deck/` directory. Project-local cleanup happens via project deregistration, not per-environment delete.

## R3: List Command SOURCE Column Implementation

**Decision**: Track source origin during list assembly and add SOURCE to both table and structured output.

**Findings**:
- `runEnvList()` at `env.go:633-734` merges three data sources in order:
  1. State instances (global runtime state)
  2. Global definitions (definitions without instances)
  3. Project-local environments (from project registry)
- Each source is already processed in distinct code blocks, making source tagging straightforward
- `envListEntry` struct at `env.go:737-745` needs a new `Source` field
- `projectListEntry` struct at `env.go:591-597` already separates project entries
- State instances need a lookup against both definition stores to determine source

**Alternatives considered**:
- Derive source from instance metadata: Rejected because instances do not currently store their definition origin
- Store source in state.yaml: Rejected because it would require migration and is derivable at list time

## R4: PATH Column Removal and Status Enhancement

**Decision**: Remove PATH column from table output, add project path to status output.

**Findings**:
- PATH column rendered at `env.go:803-806` conditionally when `len(projectEnvs) > 0`
- `writeEnvStatusText()` at `env.go:1021-1059` does not currently show project path
- `envStatusOutput` struct at `env.go:941-950` has no project path field
- Project path is available from the state store's project registry via `ListProjects()`
- Need to match environment name to project by loading each project's definition

## R5: `--global` and `--local` Flag Implementation

**Decision**: Add mutually exclusive `--global` and `--local` boolean flags to `env create` that override automatic definition resolution.

**Findings**:
- Cobra supports `MarkFlagsMutuallyExclusive()` for flag validation (available in cobra v1.10.2)
- The flags should be processed early in `runEnvCreate()`, before the existing resolution chain
- When `--global` is set: skip project-local definition, require global definition exists, skip scaffolding
- When `--local` is set: require project-local definition exists, ignore global definition
- Both flags are compatible with `--type` (type override) and other setting flags
