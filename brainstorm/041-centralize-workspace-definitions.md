# 041: Centralize Workspace Definitions

## Status: brainstorm

## Depends On

- 040 (Environment-to-Workspace rename) should be completed first

## Problem

cc-deck has a dual-store workspace definition model that causes UX problems:

1. **Bug**: `ws new` in a directory with an existing workspace overwrites `.cc-deck/workspace.yaml`. When the explicit name differs from projDef.Name, projDef is set to nil (ws.go:317), then scaffolding (line 362) creates a new file, overwriting the previous definition silently.

2. **Confusing SOURCE column**: The "global" vs "project" distinction in `cc-deck ls` is hard to reason about. A workspace created from a project-local definition can show as "global" if the project isn't registered or the definition was overwritten.

3. **Architectural complexity**: Two-store reconciliation (global `workspaces.yaml` + project-local `workspace.yaml`) creates fragile precedence logic with edge cases around name matching, type resolution, and cross-project collisions.

## Design Decisions

- **D1**: Central `~/.config/cc-deck/workspaces.yaml` is the single source of truth for all definitions
- **D2**: `.cc-deck/workspace-template.yaml` replaces `workspace.yaml` as a git-committable import template
- **D3**: Template format uses one name with type-keyed variants, plus `{{placeholder}}` support for user-specific values
- **D4**: Multiple workspaces per project directory are allowed (same name + different type auto-suffixes)
- **D5**: Drop SOURCE column, add PROJECT column showing `filepath.Base(project-dir)` or "-"
- **D6**: Placeholders are resolved at import time via user prompt, stored in central workspaces.yaml

## Template Format

```yaml
# .cc-deck/workspace-template.yaml (committed to git)
name: cc-deck
templates:
  container:
    image: docker.io/rhuss/cc-deck:latest
    auth: auto
  ssh:
    host: "{{ssh_user}}@marovo"
    workspace: ~/workspace
  k8s-deploy:
    namespace: cc-deck
    image: docker.io/rhuss/cc-deck:latest
```

### Template Semantics

- `name`: default workspace name (can be overridden via CLI)
- `templates`: map of type-key to variant configuration
- Type keys map to WorkspaceType constants: `container`, `ssh`, `k8s-deploy`, `compose`, `local`
- `{{var_name}}` placeholders prompt the user during `ws new` and resolve into the central definition

### Usage

```bash
# In a project with workspace-template.yaml:
cc-deck ws new                    # error if multiple types, pick default if one
cc-deck ws new --type container   # uses container variant
cc-deck ws new --type ssh         # uses ssh variant, prompts for {{ssh_user}}
cc-deck ws new my-name --type ssh # explicit name override
```

## New `ws list` Output

```
NAME         TYPE       STATUS   PROJECT    STORAGE       LAST ATTACHED  AGE
marovo       ssh        running  -          -             2m ago         1d
podman-test  container  running  cc-deck    named-volume  14h ago        14h
cc-deck-ssh  ssh        running  cc-deck    -             5m ago         2h
```

## Name Collision Rules (D4)

| Scenario | Behavior |
|----------|----------|
| Same name, same type | Error: "workspace 'X' already exists (type: Y); delete it first" |
| Same name, different type | Auto-suffix: `X-ssh`, `X-container`, etc. |
| User provides explicit name | Always honored, no auto-suffixing |

## Implementation

### Phase 1: Foundation (additive, no breaking changes)

**Step 1: Create template types and parser**
- New file: `internal/env/template.go`
- Types: `WorkspaceTemplate`, `TemplateVariant`
- Functions:
  - `LoadWorkspaceTemplate(projectRoot) (*WorkspaceTemplate, error)` - reads `.cc-deck/workspace-template.yaml`
  - `ResolvePlaceholders(variant *TemplateVariant, values map[string]string) (*TemplateVariant, []string, error)` - resolves `{{var}}` patterns
  - `PromptForPlaceholders(unresolved []string) (map[string]string, error)` - interactive prompt for missing values
  - `TemplateToDefinition(name string, wsType WorkspaceType, variant *TemplateVariant, projectDir string) *WorkspaceDefinition`
- New file: `internal/env/template_test.go`

**Step 2: Add query and collision-handling methods to WorkspaceStore**
- File: `internal/env/definition.go`
- `FindByProjectDir(dir string) ([]*WorkspaceDefinition, error)` - returns all definitions with matching `project-dir`
- `AddWithCollisionHandling(def *WorkspaceDefinition) (string, error)` - implements D4 collision rules, returns final name
- Repurpose `ProjectDir` field from "compose only" to general use for all workspace types

### Phase 2: Migration

**Step 3: Migration from workspace.yaml to central store**
- New file: `internal/env/migrate_project.go`
- `MigrateProjectDefinitions(store, defs) (int, error)`:
  - Iterate registered projects from `store.ListProjects()`
  - For each with `.cc-deck/workspace.yaml`: load, set `ProjectDir`, import to central store
  - Rename `.cc-deck/workspace.yaml` to `.cc-deck/workspace.yaml.migrated`
  - Return count of migrated definitions
- Run automatically on first `ws new` or `ws list` invocation
- New file: `internal/env/migrate_project_test.go`

### Phase 3: Core refactoring

**Step 4: Refactor `runWsNew`** (ws.go)
- Remove `--global` and `--local` flags
- New flow:
  1. Find project root (same as current)
  2. If `.cc-deck/workspace-template.yaml` exists:
     - Load template
     - Select variant by `--type` (error if ambiguous and no flag)
     - Resolve placeholders (prompt user)
     - Determine name: explicit arg > template name > directory basename
     - Import to central store via `AddWithCollisionHandling` with `ProjectDir` set
  3. If no template: build definition from CLI flags, add to central store
  4. Create instance as before
- Remove all `SaveProjectDefinition` calls
- Remove all `projDef` variable usage and the complex precedence logic

**Step 5: Refactor `ws list`** (ws.go)
- Remove `buildSourceMap` function
- Remove `projectListEntry` type and project-scanning loop
- Replace `Source` with `Project` in list entry (derived from `def.ProjectDir`)
- New header: `NAME  TYPE  STATUS  PROJECT  STORAGE  LAST ATTACHED  AGE`

**Step 6: Simplify `resolveEnvironmentName`** (ws.go)
- Replace `.cc-deck/workspace.yaml` lookup with `FindByProjectDir` on central store
- If exactly one match: use that name
- If multiple: list available and error, asking user to specify

**Step 7: Update `ws delete`** (ws.go)
- Remove project-local definition cleanup
- Keep `defs.Remove(name)` since definitions are central

**Step 8: Update project.go**
- Change `projectConfigPath` from `".cc-deck/workspace.yaml"` to `".cc-deck/workspace-template.yaml"`

### Phase 4: Cleanup

**Step 9: Remove deprecated code**
- Remove `LoadProjectDefinition`, `SaveProjectDefinition` from definition.go
- Remove `AllProjectEnvironmentNames` from state.go
- Remove `projectDefinitionFile` constant

## Key Files

| File | Changes |
|------|---------|
| `internal/env/template.go` | NEW: template loading, placeholder resolution |
| `internal/env/template_test.go` | NEW: template tests |
| `internal/env/definition.go` | Add `FindByProjectDir`, `AddWithCollisionHandling`; deprecate project-local functions |
| `internal/env/migrate_project.go` | NEW: migration from workspace.yaml to central store |
| `internal/env/migrate_project_test.go` | NEW: migration tests |
| `internal/cmd/ws.go` | Refactor `runWsNew`, `runWsList`, `resolveEnvironmentName`, `runWsDelete` |
| `internal/project/project.go` | Update `projectConfigPath` to workspace-template.yaml |
| `internal/env/state.go` | Remove `AllProjectEnvironmentNames` |

## Verification

1. `make test` passes
2. `make lint` passes
3. Create a workspace from a directory with workspace-template.yaml: `cc-deck ws new --type container`
4. Verify definition is in `~/.config/cc-deck/workspaces.yaml` with `project-dir` set
5. Run `cc-deck ls` and verify PROJECT column shows correctly
6. Create second workspace of different type in same project: `cc-deck ws new --type ssh`
7. Verify auto-suffixed name (e.g., `cc-deck-ssh`) and both appear in `cc-deck ls`
8. Test migration: place old `workspace.yaml` in a project, run `cc-deck ls`, verify auto-imported
9. Test placeholder resolution: template with `{{ssh_user}}`, verify prompt and resolution

## Out of Scope

- Build/Claude plugin integration for workspace-template.yaml creation (future work)
- Changes to `.cc-deck/image/` or build file locations (staying project-local)
- CLI command rename (`env` to `ws`) covered in brainstorm 039
- Internal type rename covered in brainstorm 040
