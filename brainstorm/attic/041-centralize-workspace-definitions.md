# 041: Centralize Workspace Definitions

## Status: brainstorm

## Depends On

- 040 (Environment-to-Workspace rename) -- completed

## Problem

cc-deck has a dual-store workspace definition model that causes UX problems:

1. **Bug**: `ws new` in a directory with an existing workspace overwrites `.cc-deck/workspace.yaml`. When the explicit name differs from projDef.Name, projDef is set to nil (ws.go:317), then scaffolding (line 362) creates a new file, overwriting the previous definition silently.

2. **Confusing SOURCE column**: The "global" vs "project" distinction in `cc-deck ls` is hard to reason about. A workspace created from a project-local definition can show as "global" if the project isn't registered or the definition was overwritten.

3. **Architectural complexity**: Two-store reconciliation (global `workspaces.yaml` + project-local `workspace.yaml`) creates fragile precedence logic with edge cases around name matching, type resolution, and cross-project collisions.

4. **Over-smart name resolution**: The current `resolveWorkspaceName` walks up directories looking for `.cc-deck/workspace.yaml` to infer which workspace the user means. This is surprising because behavior changes depending on which directory you are in, and users wonder "which workspace did it pick and why?"

## Design Decisions

- **D1**: Central `~/.config/cc-deck/workspaces.yaml` is the single source of truth for all definitions
- **D2**: `.cc-deck/workspace-template.yaml` replaces `workspace.yaml` as a git-committable import template
- **D3**: Template format uses one name with type-keyed variants, plus `{{placeholder}}` support for user-specific values
- **D4**: Multiple workspaces per project directory are allowed (same name + different type auto-suffixes)
- **D5**: Drop SOURCE column, add PROJECT column showing `filepath.Base(project-dir)` or "-"
- **D6**: Placeholders are resolved at import time via user prompt, stored in central workspaces.yaml
- **D7**: No migration of old project-local `workspace.yaml` files. They become dead weight; users delete them manually.
- **D8**: Drop project registry from state.yaml entirely. Project association lives in the definition's `project-dir` field.
- **D9**: Default workspace resolution uses "most recently attached" (from state.yaml timestamps), not directory-based lookup. If only one workspace exists, use that. If no recent workspace and multiple exist, error with a clear message.
- **D10**: Templates support all `WorkspaceDefinition` fields (repos, remote-bg, credentials, allowed-domains, mounts, ports, storage, namespace, etc.), with `{{placeholder}}` on any string field.

## Default Workspace Resolution (D9)

When a `ws` subcommand is called without a workspace name:

1. If exactly one workspace instance exists: use it
2. If multiple exist: use the one with the most recent `last_attached` timestamp
3. If no `last_attached` on any workspace: error with "no workspace specified; run 'cc-deck ws list' to see available workspaces"
4. Print a message: `Using workspace "X"` so the user knows what was selected

This replaces the directory-walking approach entirely. No marker directories, no project scanning, no `FindProjectConfig`.

## Template Format

```yaml
# .cc-deck/workspace-template.yaml (committed to git)
name: cc-deck
templates:
  container:
    image: docker.io/rhuss/cc-deck:latest
    auth: auto
    repos:
      - url: git@github.com:cc-deck/cc-deck.git
    remote-bg: "#0d1b2a"
  ssh:
    host: "{{ssh_user}}@marovo"
    workspace: ~/workspace
    repos:
      - url: git@github.com:cc-deck/cc-deck.git
    credentials:
      - "ANTHROPIC_API_KEY={{anthropic_key}}"
  k8s-deploy:
    namespace: cc-deck
    image: docker.io/rhuss/cc-deck:latest
    storage-size: 20Gi
```

### Template Semantics

- `name`: default workspace name (can be overridden via CLI)
- `templates`: map of type-key to variant configuration
- Type keys map to WorkspaceType constants: `container`, `ssh`, `k8s-deploy`, `compose`, `local`
- `{{var_name}}` placeholders prompt the user during `ws new` and resolve into the central definition
- All `WorkspaceDefinition` fields are valid inside a template variant (D10)

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

## `ws update --sync-repos`

After centralization, `ws update --sync-repos` reads repos from the workspace definition in the central `workspaces.yaml` (looked up by workspace name). The project-local lookup path is removed.

## Build Command

The build command (`cc-deck build`) is unaffected. It reads from `.cc-deck/setup/build.yaml`, which is independent of workspace definitions. The `.cc-deck/` directory continues to exist for build/setup artifacts but is not used as a workspace marker.

## Implementation

### Phase 1: Foundation (additive, no breaking changes)

**Step 1: Create template types and parser**
- New file: `internal/ws/template.go`
- Types: `WorkspaceTemplate`, `TemplateVariant`
- Functions:
  - `LoadWorkspaceTemplate(projectRoot) (*WorkspaceTemplate, error)` -- reads `.cc-deck/workspace-template.yaml`
  - `ResolvePlaceholders(variant *TemplateVariant, values map[string]string) (*TemplateVariant, []string, error)` -- resolves `{{var}}` patterns
  - `PromptForPlaceholders(unresolved []string) (map[string]string, error)` -- interactive prompt for missing values
  - `TemplateToDefinition(name string, wsType WorkspaceType, variant *TemplateVariant, projectDir string) *WorkspaceDefinition`
- New file: `internal/ws/template_test.go`

**Step 2: Add query and collision-handling methods to DefinitionStore**
- File: `internal/ws/definition.go`
- `FindByProjectDir(dir string) ([]*WorkspaceDefinition, error)` -- returns all definitions with matching `project-dir`
- `AddWithCollisionHandling(def *WorkspaceDefinition) (string, error)` -- implements D4 collision rules, returns final name
- Repurpose `ProjectDir` field from "compose only" to general use for all workspace types

### Phase 2: Core refactoring

**Step 3: Refactor `runWsNew`** (ws.go)
- Remove `--global` and `--local` flags
- New flow:
  1. Check if cwd has `.cc-deck/workspace-template.yaml`
  2. If template exists:
     - Load template
     - Select variant by `--type` (error if ambiguous and no flag)
     - Resolve placeholders (prompt user)
     - Determine name: explicit arg > template name > directory basename
     - Import to central store via `AddWithCollisionHandling` with `ProjectDir` set
  3. If no template: build definition from CLI flags, add to central store
  4. Create instance as before
- Remove all `SaveProjectDefinition` calls
- Remove all `projDef` variable usage and the complex precedence logic

**Step 4: Refactor default workspace resolution** (ws.go)
- Replace `resolveWorkspaceName` with new logic (D9):
  1. If args provided: use that name
  2. If one workspace exists: use it
  3. If multiple: use most recently attached
  4. If no recent: error
- Remove `FindProjectConfig` usage from all ws subcommands
- Print `Using workspace "X"` when auto-resolving

**Step 5: Refactor `ws list`** (ws.go)
- Remove `buildSourceMap` function
- Remove `projectListEntry` type and project-scanning loop
- Replace `Source` with `Project` in list entry (derived from `def.ProjectDir`)
- New header: `NAME  TYPE  STATUS  PROJECT  STORAGE  LAST ATTACHED  AGE`

**Step 6: Update `ws delete`** (ws.go)
- Remove project-local definition cleanup
- Keep `defs.Remove(name)` since definitions are central

### Phase 3: Cleanup

**Step 7: Remove deprecated code**
- Delete `LoadProjectDefinition`, `SaveProjectDefinition` from definition.go
- Delete `AllProjectWorkspaceNames` from state.go
- Delete `ListProjects`, `RegisterProject` from state.go
- Delete `ProjectEntry` type from types.go
- Remove `Projects` section handling from state file I/O
- Rename `FindProjectConfig` to `FindProjectRoot` in project.go (look for `.cc-deck/` directory, used by build command only)
- Remove `projectConfigPath` constant (was `.cc-deck/workspace.yaml`)
- Update project_test.go, project_registry_test.go accordingly

## Key Files

| File | Changes |
|------|---------|
| `internal/ws/template.go` | NEW: template loading, placeholder resolution |
| `internal/ws/template_test.go` | NEW: template tests |
| `internal/ws/definition.go` | Add `FindByProjectDir`, `AddWithCollisionHandling`; remove `LoadProjectDefinition`, `SaveProjectDefinition` |
| `internal/cmd/ws.go` | Refactor `runWsNew`, `runWsList`, `resolveWorkspaceName`, `runWsDelete`; remove project-local logic |
| `internal/project/project.go` | Rename `FindProjectConfig` to `FindProjectRoot`, look for `.cc-deck/` dir |
| `internal/ws/state.go` | Remove `AllProjectWorkspaceNames`, `ListProjects`, `RegisterProject` |
| `internal/ws/types.go` | Remove `ProjectEntry` |
| `internal/ws/remote_bg.go` | `LoadRemoteBG` reads from central definition store only |

## Verification

1. `make test` passes
2. `make lint` passes
3. Create a workspace from a directory with workspace-template.yaml: `cc-deck ws new --type container`
4. Verify definition is in `~/.config/cc-deck/workspaces.yaml` with `project-dir` set
5. Run `cc-deck ws list` and verify PROJECT column shows correctly
6. Create second workspace of different type in same project: `cc-deck ws new --type ssh`
7. Verify auto-suffixed name (e.g., `cc-deck-ssh`) and both appear in `cc-deck ws list`
8. Test placeholder resolution: template with `{{ssh_user}}`, verify prompt and resolution
9. Test default resolution: with one workspace, `cc-deck ws attach` picks it; with multiple, picks most recently attached
10. Test no-workspace error: delete all workspaces, `cc-deck ws attach` gives clear error

## Out of Scope

- Build/Claude plugin integration for workspace-template.yaml creation (future work)
- Changes to `.cc-deck/image/` or build file locations (staying project-local)
- Migration of old `.cc-deck/workspace.yaml` files (users delete manually)
