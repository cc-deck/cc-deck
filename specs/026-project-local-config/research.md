# Research: Project-Local Environment Configuration

**Feature**: 026-project-local-config | **Date**: 2026-03-22

## R1: Compose Environment Artifact Paths

**Decision**: Move generated compose artifacts from `.cc-deck/` to `.cc-deck/run/`.

**Rationale**: The `.cc-deck/` directory currently serves dual purposes: it holds generated compose artifacts (compose.yaml, env, proxy/) AND will hold committed definitions (environment.yaml, image/). Separating generated artifacts into `run/` creates a clean committed-vs-gitignored boundary.

**Current state** (`compose.go`):
- Constant `composeDir = ".cc-deck"` at line 19
- `composeProjectDir()` returns `filepath.Join(projectDir, ".cc-deck")` at line 52
- Generated files: `compose.yaml`, `env`, `proxy/tinyproxy.conf`, `proxy/whitelist`
- Proxy volume paths in `generate.go` use `./proxy/...` relative to compose project dir

**Changes needed**:
- Add `runSubdir = "run"` constant; update compose project dir to `.cc-deck/run/`
- Update proxy volume paths from `./proxy/...` to `./run/proxy/...` in `generate.go`
- Update `Delete()` to remove `run/` and `status.yaml`, not the entire `.cc-deck/` directory
- Replace project-root `.gitignore` handling with `.cc-deck/.gitignore` creation

**Alternatives considered**: Keeping artifacts directly in `.cc-deck/` (rejected: no clean commit boundary).

## R2: Image Build Artifact Relocation

**Decision**: Default image build directory to `.cc-deck/image/` when inside a project with `.cc-deck/`.

**Rationale**: Image artifacts (cc-deck-build.yaml, Containerfile, settings) are project-specific and committed. Placing them under `.cc-deck/image/` groups all project config in one directory.

**Current state** (`build/init.go`, `cmd/build.go`):
- `InitBuildDir(dir)` creates artifacts at `filepath.Join(dir, "cc-deck-build.yaml")`
- `verify` and `diff` commands accept `--dir` flag, default to cwd
- All paths are relative to the specified directory, no hardcoded absolute paths
- `.gitignore` generation writes `Containerfile` and `.build-context/` entries

**Changes needed**:
- When `--dir` not specified and `.cc-deck/` exists, default to `.cc-deck/image/`
- Git walk to find `.cc-deck/` applies here too (reuse from R3)
- No changes to `InitBuildDir()` internals, only default path resolution

**Alternatives considered**: Separate `InitImageBuildDir()` function (rejected: unnecessary, same logic with different default).

## R3: Git Boundary Walk

**Decision**: Create a `cc-deck/internal/project/` package with git root detection and project-local discovery functions. Use `git rev-parse --show-toplevel` for git root detection.

**Rationale**: Multiple commands need to find `.cc-deck/` at the git root. A shared package avoids duplication. The `git rev-parse` approach is already proven in the codebase (`session/restore.go:85-97`) and correctly handles worktrees.

**Existing patterns**:
- `session/restore.go`: `resolveProjectDir()` uses `git -C dir rev-parse --show-toplevel`
- `podman/exec.go`: `resolveLocalPath()` uses `filepath.EvalSymlinks()` with parent fallback
- `compose.go`: Simple `os.Stat(filepath.Join(projDir, ".git"))` check

**New package functions**:
- `FindGitRoot(startDir string) (string, error)`: Finds git root via `git rev-parse --show-toplevel`
- `FindProjectConfig(startDir string) (projectRoot string, err error)`: Finds `.cc-deck/environment.yaml` at git root
- `CanonicalPath(path string) string`: Symlink-resolved path for registry storage
- `ListWorktrees(gitRoot string) ([]WorktreeInfo, error)`: Parses `git worktree list --porcelain`

**Alternatives considered**:
- Manual walk checking `.git` at each level (rejected: `git rev-parse` handles edge cases like `.git` files in worktrees)
- Using go-git library (rejected: external dependency for simple operations; CLI is more reliable for worktrees)
- Putting functions in existing `env` package (rejected: creates circular dependency risk, separate concern)

## R4: Project Registry in Global State

**Decision**: Add a `Projects` section to `StateFile` in `types.go`. Store only path + last-seen timestamp.

**Rationale**: The global state file already holds environment instances. Adding a projects section keeps all global state in one file with atomic writes. The project registry is a lightweight index, not a full definition store.

**Current state** (`types.go:128-132`):
```go
type StateFile struct {
    Version      int                   `yaml:"version"`
    Environments []EnvironmentRecord   `yaml:"environments,omitempty"`
    Instances    []EnvironmentInstance  `yaml:"instances,omitempty"`
}
```

**Changes needed**:
- Add `Projects []ProjectEntry` field to `StateFile`
- Add registry methods to `FileStateStore`: `RegisterProject()`, `UnregisterProject()`, `ListProjects()`, `PruneStale()`
- Auto-registration on `env create` and walk-based discovery
- Canonical path storage via `filepath.EvalSymlinks()`

## R5: Project-Local Status Store

**Decision**: Create a lightweight `ProjectStatusFile` stored at `.cc-deck/status.yaml`. Separate from the global state store.

**Rationale**: Per-project status (variant, container name, lifecycle state, timestamps) is gitignored and specific to the local machine. Storing it in the project directory keeps it close to the definition and avoids polluting the global state file with project-specific data.

**Schema**:
```yaml
variant: ""
state: running
container_name: cc-deck-my-api
created_at: 2026-03-22T10:00:00Z
last_attached: 2026-03-22T11:00:00Z
overrides:
  image: custom:latest
```

The `overrides` section captures CLI flag values that differ from `environment.yaml` (FR-019).

## R6: CLI Command Changes

**Decision**: Modify existing commands to support optional name argument with walk-based resolution. Add `env init` and `env prune` subcommands.

**Current state** (`cmd/env.go`):
- `create` requires `cobra.ExactArgs(1)` (name is mandatory)
- `attach`, `delete`, `status`, `start`, `stop` all require `cobra.ExactArgs(1)`
- `resolveEnvironment()` looks up by name in v1 records, then v2 instances

**Changes needed**:
- Change `create` to `cobra.MaximumNArgs(1)`: name optional if `.cc-deck/` found
- Change other commands similarly: `cobra.MaximumNArgs(1)` with walk fallback
- Add `resolveEnvironmentName()` that walks to find project config when name omitted
- Add `--variant` flag to `create`
- Add `--worktrees` flag to `list`
- Add `--branch` flag to `attach`
- Add `env init` subcommand
- Add `env prune` subcommand
- Update `resolveEnvironment()` to check project-local status before global state
