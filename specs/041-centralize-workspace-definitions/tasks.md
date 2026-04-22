# Tasks: Centralize Workspace Definitions

**Feature**: 041-centralize-workspace-definitions
**Branch**: `041-centralize-workspace-definitions`
**Plan**: [plan.md](plan.md)
**Generated**: 2026-04-22

## Phase 1: Foundation (P1)

### Task 1.1: Template Types and Parser
- [X] Define `WorkspaceTemplate` struct in `internal/ws/template.go`
- [X] Define `TemplateVariant` type (YAML unmarshaling to WorkspaceDefinition fields)
- [X] Implement `LoadTemplate(projectRoot string)` to read `.cc-deck/workspace-template.yaml`
- [X] Implement `ValidateTemplate()` (name required, variants non-empty, valid type keys)
- [X] Implement `ExtractPlaceholders()` with regex for `{{name}}` and `{{name:default}}`
- [X] Implement `ResolvePlaceholders()` for string substitution
- [X] Implement `PromptForPlaceholders()` with default value display
- [X] Write tests: valid template load, missing name error, unknown variant type, placeholder extraction, default values, resolution

**Files**: `internal/ws/template.go` (create), `internal/ws/template_test.go` (create)
**Acceptance**: FR-002, FR-003, FR-004, FR-005

### Task 1.2: Extend DefinitionStore
- [X] Add `FindByProjectDir(path string)` method with ancestor matching
- [X] Add `AddWithCollisionHandling(def)` method (same-type error, different-type auto-suffix)
- [X] Write tests: exact match, subdirectory match, no match, no collision, same-type collision, different-type auto-suffix

**Files**: `internal/ws/definition.go` (modify), `internal/ws/definition_test.go` (modify)
**Acceptance**: FR-001, FR-008, FR-009

### Task 1.3: Rewrite Default Resolution
- [X] Replace `resolveWorkspaceName` with two-phase lookup (project-dir ancestor, then global)
- [X] Implement recency selection via `WorkspaceInstance.LastAttached`
- [X] Print `Using workspace "X"` to stderr on auto-resolution
- [X] Remove project auto-registration and gitignore self-healing calls
- [X] Write tests: single workspace, multiple with recency, project-scoped, no workspaces error, multiple with no history error

**Files**: `internal/cmd/ws.go` (modify), `internal/cmd/ws_new_test.go` (modify)
**Acceptance**: FR-006, FR-007

### Task 1.4: Rewrite `ws new` Command
- [X] Add template loading after project root detection
- [X] Add variant selection (by `--type` flag or auto-select if single)
- [X] Add placeholder prompting and resolution
- [X] Apply CLI flag overrides over template values
- [X] Set `project-dir` on definition for all workspace types
- [X] Use `AddWithCollisionHandling` for central storage
- [X] Remove `--global` and `--local` flags and `MarkFlagsMutuallyExclusive`
- [X] Remove `ProjectStatusStore` writes
- [X] Remove `store.RegisterProject` call
- [X] Remove scaffolding of `.cc-deck/workspace.yaml`
- [X] Remove `AllProjectWorkspaceNames` cross-project collision check
- [X] Preserve type-specific option setting, repo handling, image/storage resolution, Create() call
- [X] Write tests: template with placeholders, pure flags (no template), collision handling, explicit name override, single-variant auto-select, multi-variant without --type error

**Files**: `internal/cmd/ws.go` (modify), `internal/cmd/ws_new_test.go` (modify)
**Acceptance**: FR-001, FR-002, FR-003, FR-004, FR-005, FR-008, FR-009, FR-013

### Task 1.5: Update `ws list`
- [X] Replace `buildSourceMap` with `buildProjectMap` (workspace name to `filepath.Base(def.ProjectDir)` or "-")
- [X] Replace SOURCE column with PROJECT column in table and structured output
- [X] Remove project-local workspace collection loop
- [X] Remove `PruneStaleProjects` call
- [X] Update `wsListEntry` struct (rename `Source` to `Project`, update JSON/YAML key)
- [X] Write tests: list with project associations, standalone workspaces showing "-"

**Files**: `internal/cmd/ws.go` (modify)
**Acceptance**: FR-010

### Task 1.6: Update `ws update --sync-repos`
- [X] Remove project-local definition lookup fallback
- [X] Read repos from central store via `defs.FindByName(name)` only
- [X] Write tests: sync from central definition, error when no repos defined

**Files**: `internal/cmd/ws.go` (modify)
**Acceptance**: FR-011

## Phase 2: Cleanup (P3)

### Task 2.1: Remove Project Registry and Status Store
- [X] Remove `RegisterProject`, `UnregisterProject`, `ListProjects`, `AllProjectWorkspaceNames`, `PruneStaleProjects`, `PruneStaleProjectsVerbose` from `state.go`
- [X] Remove `ProjectEntry`, `ProjectStatusFile` from `types.go`
- [X] Remove `Projects` field from `StateFile`
- [X] Remove `LoadProjectDefinition`, `SaveProjectDefinition` from `definition.go`
- [X] Delete `project_status.go` and `project_status_test.go`
- [X] Fix all compile errors from removed types/functions
- [X] Remove corresponding tests from `state_test.go` and `definition_test.go`

**Files**: `internal/ws/state.go`, `internal/ws/types.go`, `internal/ws/definition.go` (modify); `internal/ws/project_status.go`, `internal/ws/project_status_test.go` (delete)
**Acceptance**: FR-012, FR-015, FR-016, FR-017, FR-018

### Task 2.2: Rename FindProjectConfig and Update Build Command
- [X] Rename `FindProjectConfig` to `FindProjectRoot` in `project.go`
- [X] Change lookup from `.cc-deck/workspace.yaml` to `.cc-deck/` directory
- [X] Update `FindWorkspaceRoot` to look for `.cc-deck/` directory
- [X] Rename `ErrNoProjectConfig` to `ErrNoProjectRoot`
- [X] Remove `projectConfigPath` constant
- [X] Update callers in `build.go` (`resolveBuildDirAndRoot`)
- [X] Update tests in `project_test.go`

**Files**: `internal/project/project.go`, `internal/project/project_test.go`, `internal/cmd/build.go` (modify)
**Acceptance**: FR-014

### Task 2.3: Update Gitignore
- [X] Remove `status.yaml` from required entries in `EnsureCCDeckGitignore`
- [X] Keep `run/` entry
- [X] Update tests to expect only `run/` in generated gitignore

**Files**: `internal/ws/gitignore.go`, `internal/ws/gitignore_test.go` (modify)
**Acceptance**: FR-018

## Phase 3: Documentation and Verification

### Task 3.1: Update Documentation
- [X] Update `README.md` with template workflow and changed commands
- [X] Add spec entry to README feature table
- [X] Update `docs/modules/reference/pages/cli.adoc` (removed flags, template support)
- [X] Add Antora guide page for workspace templates
- [X] Run all docs through prose plugin with cc-deck voice

**Files**: `README.md`, `docs/` (modify/create)
**Acceptance**: Constitution IX, X, XII

### Task 3.2: Final Verification
- [X] `make test` passes
- [X] `make lint` passes
- [X] Grep for removed symbols yields zero results
- [X] `--global` and `--local` flags produce unrecognized flag errors

**Acceptance**: SC-001, SC-002, SC-003, SC-004, SC-005, SC-006
