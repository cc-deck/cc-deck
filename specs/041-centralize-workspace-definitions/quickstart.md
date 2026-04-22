# Quickstart: Centralize Workspace Definitions

**Feature**: 041-centralize-workspace-definitions
**Date**: 2026-04-22

## Implementation Order

This feature has three implementation phases matching the spec priority tiers:

### Phase 1: Central Store + Templates + Default Resolution (P1)

**Goal**: All definitions stored centrally, templates work, default resolution works.

**Start here**: `internal/ws/template.go` (new file)

1. **Create template types and parser** (`internal/ws/template.go`)
   - `WorkspaceTemplate` struct with `Name` and `Variants` map
   - `LoadTemplate(projectRoot string)` function
   - `ExtractPlaceholders(variant)` function
   - `ResolvePlaceholders(variant, answers)` function
   - `PromptForPlaceholders(placeholders)` function

2. **Extend DefinitionStore** (`internal/ws/definition.go`)
   - Add `FindByProjectDir(path string)` method (ancestor matching)
   - Add `AddWithCollisionHandling(def *WorkspaceDefinition)` method

3. **Rewrite `resolveWorkspaceName`** (`internal/cmd/ws.go`)
   - Replace filesystem walk with two-phase central store lookup
   - Phase 1: filter by project-dir ancestor match
   - Phase 2: fall back to global pool with recency

4. **Rewrite `ws new` flow** (`internal/cmd/ws.go`)
   - Add template loading and variant selection
   - Add placeholder prompting
   - Remove scaffolding of `.cc-deck/workspace.yaml`
   - Remove project registration
   - Remove status file writing
   - Set `project-dir` on definition before storing centrally
   - Add collision handling (same name + different type = auto-suffix)

5. **Update `ws list`** (`internal/cmd/ws.go`)
   - Replace SOURCE column with PROJECT column
   - Remove project-local workspace collection
   - Derive PROJECT from `definition.ProjectDir`

6. **Update `ws update --sync-repos`** (`internal/cmd/ws.go`)
   - Remove project-local definition lookup
   - Read repos from central store only

### Phase 2: Sync Repos from Central Store (P2)

Already covered by step 6 in Phase 1 (minimal change).

### Phase 3: Cleanup (P3)

7. **Remove `--global` and `--local` flags** from `ws new`
8. **Delete functions**: `LoadProjectDefinition`, `SaveProjectDefinition`, `AllProjectWorkspaceNames`, `ListProjects`, `RegisterProject`, `UnregisterProject`, `PruneStaleProjects`, `PruneStaleProjectsVerbose`
9. **Delete types**: `ProjectEntry`, `ProjectStatusFile`
10. **Delete file**: `internal/ws/project_status.go`
11. **Remove `Projects` field** from `StateFile`
12. **Rename `FindProjectConfig` to `FindProjectRoot`** and change lookup to `.cc-deck/` directory
13. **Update gitignore**: Remove `status.yaml` from auto-generated `.cc-deck/.gitignore` entries
14. **Update tests** for all changed behavior
15. **Run `make test` and `make lint`** to verify

## Key Files

| File | Action | Scope |
|------|--------|-------|
| `internal/ws/template.go` | **Create** | Template types, parsing, placeholder resolution |
| `internal/ws/template_test.go` | **Create** | Template parsing and placeholder tests |
| `internal/ws/definition.go` | **Modify** | Add FindByProjectDir, AddWithCollisionHandling; remove LoadProjectDefinition, SaveProjectDefinition |
| `internal/ws/definition_test.go` | **Modify** | Tests for new methods; remove project definition tests |
| `internal/ws/state.go` | **Modify** | Remove project registry functions |
| `internal/ws/state_test.go` | **Modify** | Remove project registry tests |
| `internal/ws/types.go` | **Modify** | Remove ProjectEntry, ProjectStatusFile; remove Projects from StateFile |
| `internal/ws/project_status.go` | **Delete** | Entire file |
| `internal/ws/project_status_test.go` | **Delete** | Entire file |
| `internal/ws/gitignore.go` | **Modify** | Remove status.yaml entry |
| `internal/ws/gitignore_test.go` | **Modify** | Update expected entries |
| `internal/cmd/ws.go` | **Modify** | Major: ws new, resolveWorkspaceName, ws list, ws update |
| `internal/cmd/ws_new_test.go` | **Modify** | Rewrite for template-based and central-store flow |
| `internal/project/project.go` | **Modify** | Rename FindProjectConfig to FindProjectRoot |
| `internal/project/project_test.go` | **Modify** | Update for directory-based lookup |

## Verification Commands

```bash
make test    # All Go + Rust tests pass
make lint    # Clean linter output
```

## Risk Areas

1. **`ws new` rewrite scope**: The RunE handler is ~500 lines with complex flag/definition/type precedence logic. Rewriting while preserving non-template creation paths (pure CLI flags) requires careful attention.

2. **`resolveWorkspaceName` is called by 8 commands**: `attach`, `update`, `delete`, `status`, `stop`, `start`, `refresh-creds`, plus indirectly by `ws list`. All share the same resolution logic, so the rewrite affects all of them.

3. **Compose ProjectDir semantics**: The `ComposeWorkspace` implementation derives `.cc-deck/run/` paths from `ProjectDir`. After repurposing this field, compose path derivation must still work correctly.

4. **Test isolation**: Current tests use `SaveProjectDefinition` to set up test fixtures. New tests must use `DefinitionStore.Add` with temp directories for the central store.
