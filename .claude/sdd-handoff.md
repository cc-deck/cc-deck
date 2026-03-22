# SDD Handoff: 026-project-local-config

## Feature
Project-Local Environment Configuration

## Status
- [x] Brainstorm (`brainstorm/028-project-config.md`)
- [x] Specification (`specs/026-project-local-config/spec.md`)
- [x] Spec Review (4.0/5 initial, all blocking + non-blocking issues fixed)
- [ ] Clarify
- [x] Plan (`specs/026-project-local-config/plan.md`)
- [x] Tasks (`specs/026-project-local-config/tasks.md`)
- [ ] Implementation

## Key Context

**What**: Move environment definitions, image build artifacts, and runtime state into a project-scoped `.cc-deck/` directory at the git root. The global state file stores path references so `cc-deck env list` discovers all projects.

**Key Design Decisions** (from brainstorm):
- **Single env per project** with name defaulting to directory basename
- **Git-boundary walk**: cwd upward to `.git` for implicit name resolution
- **Directory layout**: `environment.yaml` (committed), `image/` (committed), `run/` (gitignored), `status.yaml` (gitignored)
- **`.cc-deck/.gitignore`** handles the commit/ignore boundary (explicit exception to Principle XIV)
- **Global project registry** in `state.yaml` with auto-registration and stale detection
- **`--variant` flag** for multiple container instances from same definition (worktree isolation)
- **`cc-deck env init`** scaffolds definition, `cc-deck env create` provisions runtime
- **CLI overrides are runtime-only** (stored in `status.yaml`, not persisted to `environment.yaml`)
- **Image artifacts** (`cc-deck-build.yaml`, `Containerfile`, settings) move to `.cc-deck/image/`
- **Precedence**: CLI flags > project config > global config > hardcoded defaults
- **Worktree support**: In-container worktrees via `git worktree list` discovery; host-side worktrees via `--variant`

**Plan Artifacts**:
- `specs/026-project-local-config/plan.md` - Implementation plan
- `specs/026-project-local-config/research.md` - Research findings (6 decisions)
- `specs/026-project-local-config/data-model.md` - Entity definitions
- `specs/026-project-local-config/contracts/project-discovery.md` - Interface contracts
- `specs/026-project-local-config/quickstart.md` - Implementation phasing

**New Package**: `cc-deck/internal/project/` for git root detection, project config discovery, symlink resolution, worktree listing.

**Existing Code to Modify**:
- `cc-deck/internal/env/types.go`: Add ProjectEntry, ProjectStatusFile, Env field
- `cc-deck/internal/env/state.go`: Add registry methods (RegisterProject, etc.)
- `cc-deck/internal/env/definition.go`: Add project-local loader
- `cc-deck/internal/cmd/env.go`: Add init/prune, optional name, variant, worktrees, branch
- `cc-deck/internal/env/compose.go`: Artifact paths to run/, delete behavior
- `cc-deck/internal/compose/generate.go`: Proxy volume paths
- `cc-deck/internal/build/init.go`: Default to .cc-deck/image/
- `cc-deck/internal/cmd/build.go`: Default dir resolution

**Interface Contract**: All environment operations satisfy behavioral requirements from `specs/023-env-interface/contracts/environment-interface.md` (FR-023).

## Key Files
- `specs/026-project-local-config/spec.md` - Feature specification (30 FRs, 8 SCs)
- `specs/026-project-local-config/checklists/requirements.md` - Quality checklist
- `cc-deck/internal/env/state.go` - State store (needs project registry)
- `cc-deck/internal/env/definition.go` - Definition store (needs project-local support)
- `cc-deck/internal/cmd/env.go` - CLI commands (needs init, walk, variant)

## Notes
- No public release yet, so no migration concerns
- Unstaged changes in worktree from earlier work: autosave tiered snapshots, plugin files, Makefile
- Constitution compliance: VII (FR-023), XIV (FR-024 exception), XIII (XDG paths)
- 5 implementation phases: Foundation, CLI commands, Implicit resolution, Artifact relocation, List enhancements

## Next Step
Commit spec artifacts and create spec PR. All quality gates complete (spec review, plan review, clarify, tasks).

## SDD State
sdd-initialized: true
