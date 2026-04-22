# CLI Command Contracts: Centralize Workspace Definitions

**Feature**: 041-centralize-workspace-definitions
**Date**: 2026-04-22

## Modified Commands

### `cc-deck ws new [name] [flags]`

**Changed behavior**:

| Aspect | Before | After |
|--------|--------|-------|
| Definition storage | `.cc-deck/workspace.yaml` (project-local) or `~/.config/cc-deck/workspaces.yaml` (global) | `~/.config/cc-deck/workspaces.yaml` only |
| Template support | None | Reads `.cc-deck/workspace-template.yaml` |
| `--global` flag | Forces global definition resolution | **Removed** |
| `--local` flag | Forces project-local resolution | **Removed** |
| Name collision | Error on any duplicate | Same type: error. Different type: auto-suffix (e.g., `foo-ssh`) |
| Scaffolding | Creates `.cc-deck/workspace.yaml` if missing | No scaffolding; uses template or flags |
| Project registration | Registers in `state.yaml` Projects section | Sets `project-dir` on definition |
| Status file | Writes `.cc-deck/status.yaml` | No status file written |

**Template resolution flow**:
1. If `.cc-deck/workspace-template.yaml` exists in cwd (or git root):
   a. If `--type` specified: select that variant
   b. If single variant: auto-select
   c. If multiple variants and no `--type`: error listing available types
2. Extract and prompt for `{{placeholder}}` values (show defaults from `{{name:default}}`)
3. Apply CLI flag overrides (flags take precedence over template values)
4. Store resolved definition in central store with `project-dir` set to cwd

**Name resolution** (when no positional arg):
1. Template `name` field (if template exists)
2. Directory basename of cwd

**Collision handling**:
- Same name + same type: `workspace "foo" already exists (type: container); delete it first`
- Same name + different type: auto-suffix to `foo-ssh`, `foo-container`, etc.

**Removed flags**: `--global`, `--local`

---

### `cc-deck ws attach [name]`

**Changed behavior**:

| Aspect | Before | After |
|--------|--------|-------|
| Name resolution | Walk filesystem for `.cc-deck/workspace.yaml` | Two-phase: project-dir ancestor match, then global recency |
| Auto-registration | Registers project in global registry | No registration needed |

**Default resolution** (no name argument):
1. Phase 1: Find definitions where `project-dir` is an ancestor of cwd
   - Exactly one: use it
   - Multiple: use most recently attached (by `WorkspaceInstance.LastAttached`)
2. Phase 2 (no project match): Fall back to all workspaces
   - Exactly one: use it
   - Multiple: use most recently attached
   - Multiple with no attachment history: error
   - None: error
3. Print `Using workspace "X"` to stderr

This same resolution applies to: `ws stop`, `ws start`, `ws delete`, `ws status`, `ws update`, `ws refresh-creds`.

---

### `cc-deck ws list [flags]`

**Changed behavior**:

| Aspect | Before | After |
|--------|--------|-------|
| SOURCE column | Shows `global` or `project` | **Replaced** by PROJECT column |
| PROJECT column | Not present | Shows `filepath.Base(definition.ProjectDir)` or `-` |
| Project collection | Iterates registered projects, loads project-local definitions | Not needed; all definitions are in central store |

**Output columns**: `NAME  TYPE  STATUS  PROJECT  STORAGE  LAST ATTACHED  AGE`

---

### `cc-deck ws update [name] --sync-repos`

**Changed behavior**:

| Aspect | Before | After |
|--------|--------|-------|
| Repo source | Project-local definition first, global fallback | Central definition store only |

---

### `cc-deck build [subcommand]`

**Changed behavior**:

| Aspect | Before | After |
|--------|--------|-------|
| Project root discovery | `FindProjectConfig()` looks for `.cc-deck/workspace.yaml` | `FindProjectRoot()` looks for `.cc-deck/` directory |

No changes to build command flags or output format.

## Removed Flags

| Flag | Command | Reason |
|------|---------|--------|
| `--global` | `ws new` | FR-013: single central store makes this unnecessary |
| `--local` | `ws new` | FR-013: no project-local definitions |

## New Template File Contract

**Path**: `.cc-deck/workspace-template.yaml`
**Purpose**: Git-committable template for workspace creation
**Lifecycle**: Read-only input to `ws new`; never persisted centrally

```yaml
# Required
name: <string>           # Default workspace name

# Required: at least one variant
variants:
  <workspace-type>:      # One of: ssh, container, compose, k8s-deploy
    # Any WorkspaceDefinition field except name and type
    # Fields may contain {{placeholder}} or {{placeholder:default}}
    image: "{{image:quay.io/cc-deck/demo}}"
    host: "{{ssh_user}}@marovo"
    repos:
      - url: https://github.com/org/repo.git
    # ... all other WorkspaceDefinition fields supported
```

**Validation errors**:
- Missing `name` field: `template missing required "name" field`
- No variants: `template has no variants defined`
- Invalid variant key: `unknown workspace type "foo"; valid types: ssh, container, compose, k8s-deploy`
- `--type` not matching any variant: `template has no variant for type "k8s-deploy"; available: ssh, container`
