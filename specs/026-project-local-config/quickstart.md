# Quickstart: Project-Local Environment Configuration

**Feature**: 026-project-local-config | **Date**: 2026-03-22

## Implementation Order

### Phase 1: Foundation (no existing behavior changes)

1. **New `project` package** (`internal/project/`)
   - `FindGitRoot()`, `FindProjectConfig()`, `CanonicalPath()`, `ProjectName()`
   - Unit tests with temp git repos
   - No integration with existing commands yet

2. **Data model extensions** (`internal/env/types.go`)
   - Add `ProjectEntry` struct
   - Add `ProjectStatusFile` struct
   - Add `Projects []ProjectEntry` to `StateFile`
   - Add `Env map[string]string` to `EnvironmentDefinition`

3. **Project status store** (`internal/env/project_status.go`)
   - `ProjectStatusStore` with Load/Save/Remove
   - Project-local definition loader/saver
   - Unit tests

4. **Registry methods** (`internal/env/state.go`)
   - `RegisterProject()`, `UnregisterProject()`, `ListProjects()`, `PruneStaleProjects()`
   - Unit tests

### Phase 2: CLI Commands (new commands, no breaking changes)

5. **`env prune` command** (`internal/cmd/env.go`)
   - Removes stale project registry entries
   - Integration test

### Phase 3: Implicit Resolution (modifies existing commands)

7. **Name resolution with walk** (`internal/cmd/env.go`)
   - Modify `create`, `attach`, `delete`, `status`, `start`, `stop` to accept optional name
   - When name omitted, use `FindProjectConfig()` to resolve
   - Display "Using environment X from Y" message (FR-018)
   - Auto-register project on discovery (FR-007)

8. **`env create` with project-local config** (`internal/cmd/env.go`, `internal/env/compose.go`)
   - When `.cc-deck/environment.yaml` exists, use it as source of truth (FR-019)
   - When no definition exists in git repo, auto-scaffold (FR-025)
   - Store CLI overrides in `status.yaml`, not `environment.yaml`
   - Add `--variant` flag (FR-010)

### Phase 4: Artifact Relocation (modifies paths)

9. **Move compose artifacts to `run/`** (`internal/env/compose.go`, `internal/compose/generate.go`)
   - Change compose project dir from `.cc-deck/` to `.cc-deck/run/`
   - Update proxy volume paths
   - Update delete to clean `run/` and `status.yaml` only (FR-027)
   - Replace root `.gitignore` handling with `.cc-deck/.gitignore` (FR-016)

10. **Move image artifacts to `.cc-deck/image/`** (`internal/cmd/build.go`, `internal/build/init.go`)
    - Default `--dir` to `.cc-deck/image/` when project config exists
    - Update verify/diff path resolution

### Phase 5: List Enhancements

11. **Enhanced `env list`** (`internal/cmd/env.go`)
    - Add PATH column for project-local environments (FR-012)
    - Add VARIANT column when variants present (FR-011)
    - Add MISSING status for stale registry entries (FR-008)
    - Add `--worktrees` flag using `ListWorktrees()` (FR-020)
    - Merge project-local and global environments in unified view (FR-026)

12. **`env attach --branch`** (`internal/cmd/env.go`)
    - Add `--branch` flag to attach command (FR-022)
    - Find worktree directory inside container matching branch name

## Key Integration Points

- `resolveEnvironment()` in `cmd/env.go` is the main dispatch point; it must check project-local status before global state
- `compose.go` Create/Delete methods need the most path changes
- `build.go` init/verify/diff need default directory changes
- The `project` package is consumed by `cmd/` only (no dependency from `env/` package to avoid cycles)

## Testing Strategy

- **Unit tests**: `project/` package (git root, worktrees), status store, registry methods
- **Integration tests**:
  - `env create` roundtrip (scaffold + provision)
  - `env list` with mixed sources (project-local + global)
  - `env prune`
  - **State split reconciliation** (RF-1): Create one environment via project-local config and one via global definition. Verify `env list` merges both correctly in a single view.
  - **Auto-scaffold on create** (RF-3): Run `env create --type compose` in a clean git repo with no `.cc-deck/`. Verify both `.cc-deck/environment.yaml` is created AND the environment is provisioned.
  - **No dual-state writes** (P2-2): Run `env create` for a project-local environment. Verify `status.yaml` contains the instance state AND the global `state.yaml` instances list does NOT contain a duplicate entry.
- **Manual tests**: Worktree scenarios, symlink edge cases, variant collisions
