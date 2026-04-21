# Tasks: Environment-to-Workspace Internal Rename

**Input**: Design documents from `specs/040-env-to-workspace-rename/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Path Conventions

All paths relative to `cc-deck/` (the Go CLI directory):

```text
internal/env/    -> internal/ws/     (41 files, package rename)
internal/cmd/    -> import updates   (5 consumer files)
internal/build/  -> string updates   (3 files)
```

---

## Phase 1: Setup

**Purpose**: Directory rename and package declaration update

- [x] T001 Rename directory `cc-deck/internal/env/` to `cc-deck/internal/ws/` using `git mv`
- [x] T002 Change `package env` to `package ws` in all 41 files under `cc-deck/internal/ws/`

**Checkpoint**: Directory renamed, package declarations updated. Code will not compile yet (import paths and identifiers still reference old names).

---

## Phase 2: Foundational (Type, Constant, and Function Renames)

**Purpose**: Rename all Environment-prefixed identifiers within the `internal/ws/` package. MUST complete before consumer updates.

**CRITICAL exclusions**: Do NOT rename `EnvironmentDefinition.Env` field (`yaml:"env"`), `composeEnvFile` constant, or Docker Compose `Environment` field in `internal/compose/generate.go`.

- [x] T003 [P] Rename types and constants in `cc-deck/internal/ws/types.go`: `EnvironmentType` -> `WorkspaceType`, `EnvironmentState` -> `WorkspaceState`, `EnvironmentInstance` -> `WorkspaceInstance`, all 12 `EnvironmentType*`/`EnvironmentState*` constants to `WorkspaceType*`/`WorkspaceState*`
- [x] T004 [P] Rename interface and status type in `cc-deck/internal/ws/interface.go`: `Environment` interface -> `Workspace`, `EnvironmentStatus` -> `WorkspaceStatus`, update `ListFilter.Type` field type reference
- [x] T005 [P] Rename `EnvironmentDefinition` -> `WorkspaceDefinition` and `DefinitionFile.Environments` -> `DefinitionFile.Workspaces` (with `yaml:"workspaces"` tag) in `cc-deck/internal/ws/definition.go`
- [x] T006 [P] Rename `NewEnvironment` -> `NewWorkspace` in `cc-deck/internal/ws/factory.go`
- [x] T007 [P] Rename `LocalEnvironment` -> `LocalWorkspace` and `ReconcileLocalEnvs` -> `ReconcileLocalWorkspaces` in `cc-deck/internal/ws/local.go`
- [x] T008 [P] Rename `ContainerEnvironment` -> `ContainerWorkspace`, `ReconcileContainerEnvs` -> `ReconcileContainerWorkspaces`, and `CleanupOrphanedContainer` param references in `cc-deck/internal/ws/container.go`
- [x] T009 [P] Rename `ComposeEnvironment` -> `ComposeWorkspace` and `ReconcileComposeEnvs` -> `ReconcileComposeWorkspaces` in `cc-deck/internal/ws/compose.go`
- [x] T010 [P] Rename `SSHEnvironment` -> `SSHWorkspace` and `ReconcileSSHEnvs` -> `ReconcileSSHWorkspaces` in `cc-deck/internal/ws/ssh.go`
- [x] T011 [P] Rename `K8sDeployEnvironment` -> `K8sDeployWorkspace` and `ReconcileK8sDeployEnvs` -> `ReconcileK8sDeployWorkspaces` in `cc-deck/internal/ws/k8s_deploy.go`
- [x] T012 [P] Rename `ValidateEnvName` -> `ValidateWsName`, `envNameRegex` -> `wsNameRegex`, `maxEnvNameLength` -> `maxWsNameLength` in `cc-deck/internal/ws/validate.go`
- [x] T013 [P] Rename `AllProjectEnvironmentNames` -> `AllProjectWorkspaceNames` in `cc-deck/internal/ws/state.go`
- [x] T014 [P] Update all internal cross-references within `cc-deck/internal/ws/*.go` (non-test files): any remaining references to old type names in method signatures, return types, struct fields, and local variables

**Checkpoint**: All identifiers renamed within the package. Internal cross-references resolved. Consumer files still broken.

---

## Phase 3: User Story 1 - Consistent Terminology for Contributors (Priority: P1) MVP

**Goal**: All Go type names use "Workspace" prefix, package is at `internal/ws/`, all imports updated, code compiles and tests pass.

**Independent Test**: Search the Go codebase for `EnvironmentDefinition`, `EnvironmentType`, `EnvironmentState`, or any `Environment`-prefixed type; zero results found. `make test` and `make lint` pass.

### Implementation for User Story 1

- [x] T015 [US1] Update all import paths from `internal/env` to `internal/ws` and change all `env.` qualifiers to `ws.` in `cc-deck/internal/cmd/ws.go` (120 references)
- [x] T016 [P] [US1] Update import paths and `env.` qualifiers in `cc-deck/internal/cmd/ws_new_test.go` (68 references)
- [x] T017 [P] [US1] Update import paths and `env.` qualifiers in `cc-deck/internal/cmd/ws_resolve_test.go` (5 references)
- [x] T018 [P] [US1] Update import paths and `env.` qualifiers in `cc-deck/internal/cmd/ws_prune_test.go` (1 reference)
- [x] T019 [P] [US1] Update import paths and `env.` qualifiers in `cc-deck/internal/integration/k8s_deploy_test.go` (24 references)
- [x] T020 [P] [US1] Update all renamed type/constant/function references in test files within `cc-deck/internal/ws/*_test.go` (update old `Environment*` references to `Workspace*`)
- [x] T021 [US1] Run `make test` and `make lint` to verify compilation and all tests pass

**Checkpoint**: Code compiles, all tests pass, no `Environment`-prefixed types remain. US1 complete.

---

## Phase 4: User Story 2 - Config and Environment Variable Rename (Priority: P2)

**Goal**: Config file is `workspaces.yaml` with `workspaces:` YAML key. Env var is `CC_DECK_WORKSPACES_FILE`. Old names not recognized.

**Independent Test**: Create a `workspaces.yaml` with `workspaces:` key, run `ws list`, confirm it loads. Set `CC_DECK_WORKSPACES_FILE`, confirm honored. Verify old names ignored.

### Implementation for User Story 2

- [x] T022 [US2] Change `definitionFileName` constant from `"environments.yaml"` to `"workspaces.yaml"` in `cc-deck/internal/ws/definition.go`
- [x] T023 [US2] Change `os.Getenv("CC_DECK_DEFINITIONS_FILE")` to `os.Getenv("CC_DECK_WORKSPACES_FILE")` in `cc-deck/internal/ws/definition.go`
- [x] T024 [US2] Update all test references from `CC_DECK_DEFINITIONS_FILE` to `CC_DECK_WORKSPACES_FILE` in `cc-deck/internal/cmd/ws_new_test.go` (~17 occurrences)
- [x] T025 [P] [US2] Update test references from `CC_DECK_DEFINITIONS_FILE` to `CC_DECK_WORKSPACES_FILE` in `cc-deck/internal/cmd/ws_integration_test.go`, `cc-deck/internal/cmd/compose_smoke_test.go`, and `cc-deck/internal/ws/definition_test.go`
- [x] T026 [P] [US2] Update test references from `environments.yaml` to `workspaces.yaml` in `cc-deck/internal/ws/definition_test.go`, `cc-deck/internal/ws/repos_test.go`, `cc-deck/internal/cmd/ws_integration_test.go`, and `cc-deck/internal/cmd/compose_smoke_test.go`
- [x] T027 [P] [US2] Update test inline YAML strings from `environments:` key to `workspaces:` key in `cc-deck/internal/ws/ssh_test.go` (lines ~220, 232, 248) and any other test files with inline YAML
- [x] T028 [US2] Update `cc-deck/test/k8s-deploy-walkthrough.md` references from `CC_DECK_DEFINITIONS_FILE` to `CC_DECK_WORKSPACES_FILE` (lines 51, 455)
- [x] T029 [US2] Run `make test` to verify config and env var changes work correctly

**Checkpoint**: Config file and env var fully renamed. US2 complete.

---

## Phase 5: User Story 3 - Build Command Descriptions (Priority: P3)

**Goal**: Build and capture command descriptions reference "workspace" instead of "environment."

**Independent Test**: Inspect build and capture command description strings; they reference "workspace."

### Implementation for User Story 3

- [x] T030 [P] [US3] Update "Build environment" to "Build workspace" and other "environment" references in `cc-deck/internal/build/commands/cc-deck.build.md` (lines 2, 23)
- [x] T031 [P] [US3] Update "Capture environment" to "Capture workspace" and other cc-deck-workspace references in `cc-deck/internal/build/commands/cc-deck.capture.md` (lines 2, 126, 133, 141, 152; DO NOT change line 317 "Environment variables" which refers to OS env vars)
- [x] T032 [P] [US3] Update "environment" to "workspace" in `cc-deck/internal/build/templates/build.yaml.tmpl` (lines 2, 4, 75; DO NOT change if context is OS env vars)
- [x] T033 [P] [US3] Update "AI-driven environment configuration" to "AI-driven workspace configuration" in `cc-deck/internal/cmd/build.go` (line 43)

**Checkpoint**: All build command descriptions use "workspace" terminology. US3 complete.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Error messages, help text, comments, and final verification

- [x] T034 [P] Update user-facing error messages from "environment" to "workspace" in `cc-deck/internal/ws/definition.go` (~4 messages, lines 148, 161, 184, 202)
- [x] T035 [P] Update user-facing error messages in `cc-deck/internal/ws/state.go` (~4 messages, lines 115, 128, 151, 169)
- [x] T036 [P] Update user-facing error messages in `cc-deck/internal/ws/ssh.go` (~2 messages, lines 95, 389)
- [x] T037 [P] Update user-facing error messages in `cc-deck/internal/ws/compose.go` (~6 messages, lines 369, 543, 556, 579, 597, 702)
- [x] T038 [P] Update user-facing error messages in `cc-deck/internal/ws/k8s_deploy.go` (~5 messages, lines 286, 486, 499, 512, 525)
- [x] T039 [P] Update user-facing error messages in `cc-deck/internal/ws/container.go` (~1 message, line 505) and `cc-deck/internal/ws/local.go` (~4 messages, lines 203, 208, 213, 218)
- [x] T040 [P] Update CLI help text in `cc-deck/internal/cmd/ws.go`: lines 29, 38, 155, 1740 (change "environments" to "workspaces" where referring to cc-deck workspaces; check `cc-deck/internal/cmd/ws.go` line 1764 error message too)
- [x] T041 [P] Update test assertions that verify error message strings containing "environment" across all `*_test.go` files in `cc-deck/internal/ws/` and `cc-deck/internal/cmd/`
- [x] T042 Run `make test` and `make lint` for final verification
- [x] T043 Verify SC-001: search entire Go codebase for `Environment`-prefixed types and `internal/env` import paths; confirm zero results (excluding compose/Docker Compose `Environment` field)

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies, start immediately
- **Foundational (Phase 2)**: Depends on Phase 1 (directory must be renamed first)
- **User Story 1 (Phase 3)**: Depends on Phase 2 (identifiers must be renamed before consumers update)
- **User Story 2 (Phase 4)**: Depends on Phase 3 (code must compile before config changes)
- **User Story 3 (Phase 5)**: Independent of US2, can start after Phase 3
- **Polish (Phase 6)**: Can start after Phase 3 (error messages are string changes, not structural)

### User Story Dependencies

- **US1 (P1)**: Depends on Foundational. Core rename, all other stories depend on this.
- **US2 (P2)**: Depends on US1 (needs compiling code to verify config changes).
- **US3 (P3)**: Depends on US1 (needs compiling code). Independent of US2.

### Parallel Opportunities

- Phase 2: All T003-T014 can run in parallel (different files within `internal/ws/`)
- Phase 3: T016-T020 can run in parallel (different consumer files). T015 should go first (largest file).
- Phase 4: T025-T027 can run in parallel (different test files)
- Phase 5: T030-T033 can run in parallel (different files)
- Phase 6: T034-T041 can run in parallel (different files)
- US2 and US3 can run in parallel after US1 completes

---

## Parallel Example: Phase 2 (Foundational)

```
# All of these touch different files and can run simultaneously:
Agent 1: T003 - types.go (types and constants)
Agent 2: T004 - interface.go (interface and status)
Agent 3: T005 - definition.go (definition types)
Agent 4: T006+T007 - factory.go + local.go
Agent 5: T008+T009 - container.go + compose.go
Agent 6: T010+T011 - ssh.go + k8s_deploy.go
Agent 7: T012+T013 - validate.go + state.go
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (directory rename)
2. Complete Phase 2: Foundational (identifier renames)
3. Complete Phase 3: User Story 1 (consumer updates)
4. **STOP and VALIDATE**: `make test` and `make lint` pass, grep confirms no `Environment`-prefixed types

### Incremental Delivery

1. Phase 1+2+3 -> US1 complete (code compiles, tests pass, internal terminology aligned)
2. Phase 4 -> US2 complete (config file and env var renamed)
3. Phase 5 -> US3 complete (build descriptions updated)
4. Phase 6 -> Polish (error messages, help text, final verification)

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story
- The rename is mechanical; no logic changes
- DO NOT rename: `Env` field in `WorkspaceDefinition` (OS env vars), `Environment` in compose/generate.go (Docker Compose), `composeEnvFile` constant
- Commit after each phase checkpoint
