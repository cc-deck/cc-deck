# Tasks: Centralize Workspace Definitions

**Feature**: 041-centralize-workspace-definitions
**Branch**: `041-centralize-workspace-definitions`
**Plan**: [plan.md](plan.md)
**Generated**: 2026-04-22

## Phase 1: Foundation (P1)

### Task 1.1: Template Types and Parser
- [ ] Define `WorkspaceTemplate` struct in `internal/ws/template.go`
- [ ] Define `TemplateVariant` type (YAML unmarshaling to WorkspaceDefinition fields)
- [ ] Implement `LoadTemplate(projectRoot string)` to read `.cc-deck/workspace-template.yaml`
- [ ] Implement `ValidateTemplate()` (name required, variants non-empty, valid type keys)
- [ ] Implement `ExtractPlaceholders()` with regex for `{{name}}` and `{{name:default}}`
- [ ] Implement `ResolvePlaceholders()` for string substitution
- [ ] Implement `PromptForPlaceholders()` with default value display
- [ ] Write tests: valid template load, missing name error, unknown variant type, placeholder extraction, default values, resolution

**Files**: `internal/ws/template.go` (create), `internal/ws/template_test.go` (create)
**Acceptance**: FR-002, FR-003, FR-004, FR-005

### Task 1.2: Extend DefinitionStore
- [ ] Add `FindByProjectDir(path string)` method with ancestor matching
- [ ] Add `AddWithCollisionHandling(def)` method (same-type error, different-type auto-suffix)
- [ ] Write tests: exact match, subdirectory match, no match, no collision, same-type collision, different-type auto-suffix

**Files**: `internal/ws/definition.go` (modify), `internal/ws/definition_test.go` (modify)
**Acceptance**: FR-001, FR-008, FR-009

### Task 1.3: Rewrite Default Resolution
- [ ] Replace `resolveWorkspaceName` with two-phase lookup (project-dir ancestor, then global)
- [ ] Implement recency selection via `WorkspaceInstance.LastAttached`
- [ ] Print `Using workspace "X"` to stderr on auto-resolution
- [ ] Remove project auto-registration and gitignore self-healing calls
- [ ] Write tests: single workspace, multiple with recency, project-scoped, no workspaces error, multiple with no history error

**Files**: `internal/cmd/ws.go` (modify), `internal/cmd/ws_new_test.go` (modify)
**Acceptance**: FR-006, FR-007

### Task 1.4: Rewrite `ws new` Command
- [ ] Add template loading after project root detection
- [ ] Add variant selection (by `--type` flag or auto-select if single)
- [ ] Add placeholder prompting and resolution
- [ ] Apply CLI flag overrides over template values
- [ ] Set `project-dir` on definition for all workspace types
- [ ] Use `AddWithCollisionHandling` for central storage
- [ ] Remove `--global` and `--local` flags and `MarkFlagsMutuallyExclusive`
- [ ] Remove `ProjectStatusStore` writes
- [ ] Remove `store.RegisterProject` call
- [ ] Remove scaffolding of `.cc-deck/workspace.yaml`
- [ ] Remove `AllProjectWorkspaceNames` cross-project collision check
- [ ] Preserve type-specific option setting, repo handling, image/storage resolution, Create() call
- [ ] Write tests: template with placeholders, pure flags (no template), collision handling, explicit name override, single-variant auto-select, multi-variant without --type error

**Files**: `internal/cmd/ws.go` (modify), `internal/cmd/ws_new_test.go` (modify)
**Acceptance**: FR-001, FR-002, FR-003, FR-004, FR-005, FR-008, FR-009, FR-013

### Task 1.5: Update `ws list`
- [ ] Replace `buildSourceMap` with `buildProjectMap` (workspace name to `filepath.Base(def.ProjectDir)` or "-")
- [ ] Replace SOURCE column with PROJECT column in table and structured output
- [ ] Remove project-local workspace collection loop
- [ ] Remove `PruneStaleProjects` call
- [ ] Update `wsListEntry` struct (rename `Source` to `Project`, update JSON/YAML key)
- [ ] Write tests: list with project associations, standalone workspaces showing "-"

**Files**: `internal/cmd/ws.go` (modify)
**Acceptance**: FR-010

### Task 1.6: Update `ws update --sync-repos`
- [ ] Remove project-local definition lookup fallback
- [ ] Read repos from central store via `defs.FindByName(name)` only
- [ ] Write tests: sync from central definition, error when no repos defined

**Files**: `internal/cmd/ws.go` (modify)
**Acceptance**: FR-011

## Phase 2: Cleanup (P3)

### Task 2.1: Remove Project Registry and Status Store
- [ ] Remove `RegisterProject`, `UnregisterProject`, `ListProjects`, `AllProjectWorkspaceNames`, `PruneStaleProjects`, `PruneStaleProjectsVerbose` from `state.go`
- [ ] Remove `ProjectEntry`, `ProjectStatusFile` from `types.go`
- [ ] Remove `Projects` field from `StateFile`
- [ ] Remove `LoadProjectDefinition`, `SaveProjectDefinition` from `definition.go`
- [ ] Delete `project_status.go` and `project_status_test.go`
- [ ] Fix all compile errors from removed types/functions
- [ ] Remove corresponding tests from `state_test.go` and `definition_test.go`

**Files**: `internal/ws/state.go`, `internal/ws/types.go`, `internal/ws/definition.go` (modify); `internal/ws/project_status.go`, `internal/ws/project_status_test.go` (delete)
**Acceptance**: FR-012, FR-015, FR-016, FR-017, FR-018

### Task 2.2: Rename FindProjectConfig and Update Build Command
- [ ] Rename `FindProjectConfig` to `FindProjectRoot` in `project.go`
- [ ] Change lookup from `.cc-deck/workspace.yaml` to `.cc-deck/` directory
- [ ] Update `FindWorkspaceRoot` to look for `.cc-deck/` directory
- [ ] Rename `ErrNoProjectConfig` to `ErrNoProjectRoot`
- [ ] Remove `projectConfigPath` constant
- [ ] Update callers in `build.go` (`resolveBuildDirAndRoot`)
- [ ] Update tests in `project_test.go`

**Files**: `internal/project/project.go`, `internal/project/project_test.go`, `internal/cmd/build.go` (modify)
**Acceptance**: FR-014

### Task 2.3: Update Gitignore
- [ ] Remove `status.yaml` from required entries in `EnsureCCDeckGitignore`
- [ ] Keep `run/` entry
- [ ] Update tests to expect only `run/` in generated gitignore

**Files**: `internal/ws/gitignore.go`, `internal/ws/gitignore_test.go` (modify)
**Acceptance**: FR-018

## Phase 3: Documentation and Verification

### Task 3.1: Update Documentation
- [ ] Update `README.md` with template workflow and changed commands
- [ ] Add spec entry to README feature table
- [ ] Update `docs/modules/reference/pages/cli.adoc` (removed flags, template support)
- [ ] Add Antora guide page for workspace templates
- [ ] Run all docs through prose plugin with cc-deck voice

**Files**: `README.md`, `docs/` (modify/create)
**Acceptance**: Constitution IX, X, XII

### Task 3.2: Final Verification
- [ ] `make test` passes
- [ ] `make lint` passes
- [ ] Grep for removed symbols yields zero results
- [ ] `--global` and `--local` flags produce unrecognized flag errors

**Acceptance**: SC-001, SC-002, SC-003, SC-004, SC-005, SC-006
