# 035: Environment Lifecycle Fixes

## Status: brainstorm

## Problem

Three related UX issues in the environment lifecycle:

### Issue 1: Project-local definition leaks into unrelated environments

When running `cc-deck env create marovo-test` from a directory with `.cc-deck/environment.yaml` (defining `smoke-full` as type `compose`), the type resolution picks up `compose` from the project-local definition even though the explicit name `marovo-test` has nothing to do with it. A global definition for `marovo-test` (type `ssh`) exists but is ignored for type resolution.

**Root cause:** In `runEnvCreate`, type resolution uses `projDef.Type` whenever no `--type` flag is given, regardless of whether the explicit name matches `projDef.Name`.

### Issue 2: SSH environment delete does not remove global definition

Container and compose environments remove their definition from `environments.yaml` on delete. SSH environments do not, leaving ghost "not created" entries in `cc-deck ls`.

**Root cause:** `SSHEnvironment.Delete()` calls `store.RemoveInstance()` but not `defs.Remove()`.

### Issue 3: `cc-deck ls` does not indicate definition source

The list output shows environments from global definitions, project-local definitions, and state instances without indicating where each entry comes from. When an environment shows "not created", it is unclear whether the definition is global or project-local.

## Design Decisions

### D1: When explicit name differs from project-local name, ignore project-local definition

When a user provides an explicit name to `env create` and that name does not match `projDef.Name`:

1. Discard `projDef` for type and settings resolution
2. Look up the explicit name in the global definition store
3. If found globally, use that definition's type and settings
4. If not found anywhere, require `--type` or fall back to `local`

This follows **Model B**: the project-local file defines *the* environment for this project. If you name something else, the project-local file is irrelevant.

**Resolution flow:**

```
cc-deck env create [name]

1. name == "" → use projDef (project-local), inherit everything
2. name == projDef.Name → use projDef, inherit everything
3. name != projDef.Name → ignore projDef, look up global defs
   a. Found in global → use global def's type and settings
   b. Not found → use CLI flags, require --type or default to local
```

### D2: All environment types clean up definitions on delete

Symmetric behavior: if `env create` adds a definition, `env delete` removes it.

### D3: Show source indicator in `cc-deck ls`, drop PATH column

Add a `SOURCE` column to `cc-deck ls` output:

| Value | Meaning |
|-------|---------|
| `global` | From `~/.config/cc-deck/environments.yaml` |
| `project` | From `.cc-deck/environment.yaml` in a registered project |
| (empty) | Instance exists but no definition found |

Remove the `PATH` column from `cc-deck ls`. Project-scoped environments only appear when the user is inside that directory, so the path is redundant in the list view. Move the full project path to `cc-deck status <name>` output instead, where detailed single-environment info belongs.

## Scope

- Fix type resolution in `runEnvCreate` when explicit name differs from project-local name
- Add `defs.Remove()` to `SSHEnvironment.Delete()`
- Add source column to `cc-deck ls` table output, remove PATH column
- Add project path to `cc-deck status` output
- Add/update tests for all three fixes
- Update CLI reference docs

## Out of Scope

- Multi-environment project-local definitions (future work)
- Project-local definition as a list (would be a separate spec)
- Changes to `env create` scaffolding behavior
