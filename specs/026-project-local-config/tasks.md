# Tasks: Project-Local Environment Configuration

**Input**: Design documents from `specs/026-project-local-config/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/project-discovery.md, quickstart.md

**Tests**: Tests are MANDATORY per project constitution. Each user story phase includes integration tests.

**Organization**: Tasks grouped by user story to enable independent implementation and testing.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2)
- Exact file paths included in descriptions

## Path Conventions

```text
cc-deck/internal/
├── project/             # NEW: Git root detection, project discovery
├── env/                 # MODIFIED: types, state, definition, status store
├── compose/             # MODIFIED: proxy volume paths
├── build/               # MODIFIED: default artifact directory
└── cmd/                 # MODIFIED: env.go (init, prune, optional name, variant)
```

---

## Phase 1: Setup

**Purpose**: Create the new `project` package with git root detection and project discovery functions.

- [x] T001 Create `cc-deck/internal/project/project.go` with FindGitRoot, FindProjectConfig, CanonicalPath, ProjectName per contracts/project-discovery.md
- [x] T002 [P] Create `cc-deck/internal/project/worktree.go` with WorktreeInfo struct and ListWorktrees function per contracts/project-discovery.md
- [x] T003 Create `cc-deck/internal/project/project_test.go` with unit tests for FindGitRoot (regular repo, worktree, non-git dir), FindProjectConfig (found, not found), CanonicalPath (symlink, no symlink), ProjectName, and ListWorktrees

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Data model extensions, status store, registry methods, and compose path migration. MUST complete before any user story.

**WARNING**: No user story work can begin until this phase is complete.

- [x] T004 [P] Add ProjectEntry and ProjectStatusFile structs to `cc-deck/internal/env/types.go`; add `Projects []ProjectEntry` to StateFile; add `Env map[string]string` yaml tag to EnvironmentDefinition
- [x] T005 [P] Create `cc-deck/internal/env/project_status.go` with ProjectStatusStore (NewProjectStatusStore, Load, Save, Remove) per contracts/project-discovery.md
- [x] T006 [P] Add RegisterProject, UnregisterProject, ListProjects, PruneStaleProjects methods to `cc-deck/internal/env/state.go` per contracts/project-discovery.md
- [x] T007 [P] Add LoadProjectDefinition and SaveProjectDefinition functions to `cc-deck/internal/env/definition.go` per contracts/project-discovery.md (bare EnvironmentDefinition, not DefinitionFile wrapper)
- [x] T008 [P] Create ensureCCDeckGitignore helper function in `cc-deck/internal/env/gitignore.go` that idempotently creates `.cc-deck/.gitignore` with `status.yaml` and `run/` entries (FR-016, FR-030)
- [x] T009 [P] Unit tests for ProjectStatusStore, registry methods, LoadProjectDefinition, SaveProjectDefinition, ensureCCDeckGitignore in their respective `_test.go` files
- [x] T010 Update compose project dir from `.cc-deck/` to `.cc-deck/run/` in `cc-deck/internal/env/compose.go` (change composeProjectDir method, update all artifact write paths) (FR-014)
- [x] T011 Update proxy volume paths from `./proxy/` to `./run/proxy/` in `cc-deck/internal/compose/generate.go` (FR-014)
- [x] T012 Update compose Delete method in `cc-deck/internal/env/compose.go` to remove `.cc-deck/run/` and `.cc-deck/status.yaml` only, preserving `.cc-deck/environment.yaml` and `.cc-deck/image/` (FR-027)
- [x] T013 Replace project-root `.gitignore` handling (handleGitignore) in `cc-deck/internal/env/compose.go` with ensureCCDeckGitignore call from T008 (FR-016)

**Checkpoint**: Foundation ready. Data model, stores, and compose path migration complete. User story implementation can begin.

---

## Phase 3: User Story 1+2 - Create and Clone (Priority: P1) MVP

NOTE: User Story 2 (Initialize) was merged into User Story 1 (Create). `env create` handles both scaffolding and provisioning. There is no separate `env init` command. T014-T015 are superseded by the auto-scaffold logic in T018.

**Goal**: `cc-deck env create` from a project with `.cc-deck/environment.yaml` provisions an environment using the definition as source of truth. No flags needed.

**Independent Test**: Create a temp git repo with `.cc-deck/environment.yaml`, run `cc-deck env create`, verify environment is created with definition settings and project is registered in global registry.

- [x] T016 [US1] Modify `cc-deck env create` in `cc-deck/internal/cmd/env.go` to accept optional name (cobra.MaximumNArgs(1)). When name omitted and in a git repo, use FindProjectConfig to resolve name from definition. Display "Using environment X from Y" message (FR-018).
- [x] T017 [US1] Add project-local definition reading to `cc-deck env create` in `cc-deck/internal/cmd/env.go`: when `.cc-deck/environment.yaml` exists, load it via LoadProjectDefinition and use as source of truth (FR-019). Auto-detect type from definition (FR-013). Apply CLI flag overrides as runtime-only values.
- [x] T018 [US1] Add auto-scaffold behavior to `cc-deck env create` in `cc-deck/internal/cmd/env.go`: when in a git repo with no `.cc-deck/environment.yaml`, scaffold definition from CLI flags before provisioning (FR-025). When outside git repo with no definition, require explicit name.
- [x] T019 [US1] Store CLI overrides in `.cc-deck/status.yaml` via ProjectStatusStore after successful create in `cc-deck/internal/cmd/env.go`. Auto-register project in global registry via RegisterProject (FR-007). Call ensureCCDeckGitignore (FR-030).
- [x] T020 [US1] Add project-local vs global definition precedence check in `cc-deck/internal/cmd/env.go`: when both exist with same name, use project-local and emit warning (FR-026).
- [x] T021 [US1] Integration tests in `cc-deck/internal/cmd/env_create_test.go`: (1) create from existing definition, (2) auto-scaffold when no definition, (3) CLI override stored in status.yaml not environment.yaml, (4) project registered in global registry, (5) state split: status in status.yaml not global instances, (6) no dual-state writes

**Checkpoint**: Clone-and-create workflow works end-to-end. MVP deliverable.

---

## Phase 5: User Story 3 - Implicit Name Resolution (Priority: P2)

**Goal**: All env commands work without specifying an environment name when run from within a project directory.

**Independent Test**: cd into `~/projects/my-api/src/pkg/`, run `cc-deck env attach` (no name), verify it resolves the environment from `.cc-deck/` at the git root.

- [x] T022 [US3] Create resolveEnvironmentName helper in `cc-deck/internal/cmd/env.go` that walks to find project config when name is omitted, returns (name, projectRoot, error). Auto-registers project on walk-based discovery (FR-007).
- [x] T023 [US3] Modify attach, delete, status, start, stop commands in `cc-deck/internal/cmd/env.go` to use cobra.MaximumNArgs(1) and call resolveEnvironmentName when no name provided. Display "Using environment X from Y" (FR-018). Fail with clear error when no name and no project config found.
- [x] T024 [US3] Update resolveEnvironment in `cc-deck/internal/cmd/env.go` to check project-local ProjectStatusStore before global state. Self-heal .gitignore on env operations (FR-030).
- [x] T025 [US3] Integration test in `cc-deck/internal/cmd/env_resolve_test.go`: (1) resolve from subdirectory, (2) fail with no name and no config, (3) auto-register on discovery, (4) display message shows correct path

**Checkpoint**: All env commands work without explicit name inside project directories.

---

## Phase 6: User Story 4 - Global List Shows All Projects (Priority: P2)

**Goal**: `cc-deck env list` shows all registered project environments with paths, types, status, variants, and MISSING indicators.

**Independent Test**: Register three projects (two existing, one moved), run `cc-deck env list`, verify output shows all three with correct columns and MISSING status for the moved project.

- [x] T026 [US4] Enhance writeEnvTable and writeEnvStructured in `cc-deck/internal/cmd/env.go` to include project-local environments from ListProjects + LoadProjectDefinition. Add PATH column for project-local environments (FR-012). Show MISSING status for stale registry entries (FR-008). Merge project-local and global in unified view, warning on shadowed definitions (FR-026).
- [x] T027 [US4] Add --worktrees flag to `cc-deck env list` in `cc-deck/internal/cmd/env.go`. When set, call ListWorktrees for each project and display worktree sub-entries with branch names (FR-020).
- [x] T028 [US4] Integration test in `cc-deck/internal/cmd/env_list_test.go`: (1) list shows project-local and global together, (2) MISSING status for removed directory, (3) --worktrees shows branches

**Checkpoint**: `cc-deck env list` provides complete visibility across all projects.

---

## Phase 7: User Story 5 - Variant for Worktree Isolation (Priority: P3)

**Goal**: `--variant` flag creates uniquely named containers from the same definition. `--branch` flag attaches to a specific worktree.

**Independent Test**: Create two worktrees, run `cc-deck env create --variant auth` in the second, verify separate container named `cc-deck-my-api-auth`.

- [x] T029 [US5] Add --variant flag to `cc-deck env create` in `cc-deck/internal/cmd/env.go`. Store variant in ProjectStatusFile. Append variant to container name as `cc-deck-<name>-<variant>` (FR-010).
- [x] T030 [US5] Add VARIANT column to `cc-deck env list` output in `cc-deck/internal/cmd/env.go`, shown conditionally when any variant is present (FR-011).
- [x] T031 [US5] Add --branch flag to `cc-deck env attach` in `cc-deck/internal/cmd/env.go`. Find matching worktree inside container via `git worktree list`, cd into it. Fail with error listing available worktrees if branch not found (FR-022).
- [x] T032 [US5] Integration test in `cc-deck/internal/cmd/env_variant_test.go`: (1) create with --variant produces container named cc-deck-<name>-<variant>, (2) variant shown in env list, (3) --branch fails with error when branch not found

**Checkpoint**: Variant mechanism enables per-worktree container isolation.

---

## Phase 8: User Story 6 - Image Build Artifacts in .cc-deck/image/ (Priority: P3)

**Goal**: Image build commands default to `.cc-deck/image/` for artifacts instead of the project root.

**Independent Test**: Run `cc-deck image init` in a project with `.cc-deck/`, verify `cc-deck-build.yaml` is created at `.cc-deck/image/cc-deck-build.yaml`.

- [x] T033 [US6] Update default --dir resolution in `cc-deck/internal/cmd/build.go` for init, verify, and diff subcommands: when --dir not specified and `.cc-deck/` exists at git root (via FindProjectConfig), default to `.cc-deck/image/` (FR-017).
- [x] T034 [US6] Integration test in `cc-deck/internal/cmd/build_image_test.go`: (1) image init creates artifacts in .cc-deck/image/, (2) verify and diff find artifacts in .cc-deck/image/

**Checkpoint**: Image build artifacts stored under `.cc-deck/image/`.

---

## Phase 9: User Story 7 - Prune Stale Entries (Priority: P3)

**Goal**: `cc-deck env prune` removes stale project registry entries whose directories no longer exist.

**Independent Test**: Register a project, remove its directory, run `cc-deck env prune`, verify entry removed and count reported.

- [x] T035 [US7] Add `env prune` subcommand to `cc-deck/internal/cmd/env.go`. Calls PruneStaleProjects on FileStateStore. Reports count of removed entries (FR-009).
- [x] T036 [US7] Integration test in `cc-deck/internal/cmd/env_prune_test.go`: (1) prune removes stale entry, (2) prune is idempotent, (3) prune preserves valid entries

**Checkpoint**: Registry housekeeping works.

---

## Phase 10: Polish and Cross-Cutting Concerns

**Purpose**: Documentation, validation, and cleanup.

- [x] T037 [P] Update README.md with project-local config feature description, usage examples, and updated Feature Specifications table
- [x] T038 [P] Update CLI reference in `docs/modules/reference/pages/cli.adoc` with env prune, --variant, --worktrees, --branch flags
- [x] T039 Run `make test` and `make lint` to verify all tests pass and no lint issues
- [x] T040 Run quickstart.md validation: verify all 12 implementation steps are covered by tasks

---

## Dependencies and Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies, start immediately
- **Foundational (Phase 2)**: Depends on Phase 1 completion. BLOCKS all user stories.
- **US2 (Phase 3)**: Depends on Phase 2. Can run in parallel with other P1 stories.
- **US1 (Phase 4)**: Depends on Phase 2. Can run in parallel with US2 (but US2 is simpler, do first).
- **US3 (Phase 5)**: Depends on US1 completion (needs create to work first).
- **US4 (Phase 6)**: Depends on Phase 2 only. Can start as soon as foundational is done.
- **US5 (Phase 7)**: Depends on US1 (variant extends create).
- **US6 (Phase 8)**: Depends on Phase 2 only. Independent of other user stories.
- **US7 (Phase 9)**: Depends on Phase 2 only. Independent of other user stories.
- **Polish (Phase 10)**: Depends on all desired user stories being complete.

### User Story Dependencies

```
Phase 1 (Setup)
  └──► Phase 2 (Foundational) ──BLOCKS──┬──► US2 (P1) ──► US1 (P1) ──► US3 (P2)
                                         ├──► US4 (P2)       │
                                         ├──► US6 (P3)       └──► US5 (P3)
                                         └──► US7 (P3)
                                         └────────────────────────► Polish
```

### Within Each User Story

- Implementation tasks in dependency order
- Integration tests after implementation
- Commit after each completed user story

### Parallel Opportunities

- Phase 1: T001 and T002 can run in parallel (different files)
- Phase 2: T004-T009 can all run in parallel (different files, no dependencies)
- T010-T013 are sequential (same file, compose.go)
- After Phase 2: US4, US6, US7 can start in parallel with US2/US1
- US2 and US1 are both P1 but US2 is simpler and creates the foundation US1 consumes

---

## Parallel Example: Phase 2 (Foundational)

```bash
# All run in parallel (different files):
Task T004: "Add ProjectEntry, ProjectStatusFile to cc-deck/internal/env/types.go"
Task T005: "Create ProjectStatusStore in cc-deck/internal/env/project_status.go"
Task T006: "Add registry methods to cc-deck/internal/env/state.go"
Task T007: "Add LoadProjectDefinition to cc-deck/internal/env/definition.go"
Task T008: "Create ensureCCDeckGitignore in cc-deck/internal/env/gitignore.go"

# Then sequential (same file, compose.go):
Task T010: "Update compose project dir to .cc-deck/run/"
Task T012: "Update compose Delete for project-local cleanup"
Task T013: "Replace handleGitignore with ensureCCDeckGitignore"
```

---

## Implementation Strategy

### MVP First (User Stories 1 + 2 Only)

1. Complete Phase 1: Setup (project package)
2. Complete Phase 2: Foundational (data model, stores, compose migration)
3. Complete Phase 3: US1+2 (create with auto-scaffold)
4. Complete Phase 4: US1 (clone and create)
5. **STOP AND VALIDATE**: Test clone-and-create workflow end-to-end
6. This delivers SC-001: "git clone + env create + env attach without flags"

### Incremental Delivery

1. Setup + Foundational: Foundation ready
2. US2 + US1: Clone-and-create workflow (MVP)
3. US3: Implicit name resolution (no-name commands)
4. US4: Full project visibility in env list
5. US5 + US6 + US7: Power-user features (variant, image, prune)
6. Polish: Documentation and validation

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story is independently completable and testable
- Commit after each completed user story phase
- No public release yet, so no migration code needed
- Use `make test` and `make lint` (never `go build` directly, per Constitution VI)
