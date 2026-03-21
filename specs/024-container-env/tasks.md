# Tasks: Container Environment

**Input**: Design documents from `/specs/024-container-env/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/

**Tests**: Unit tests for new packages (podman, definition store). Integration tests require podman and are skipped when unavailable.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

---

## Phase 1: Setup

**Purpose**: Type renames and project structure preparation

- [x] T001 Rename `EnvironmentTypePodman` to `EnvironmentTypeContainer` in `cc-deck/internal/env/types.go`
- [x] T002 Rename `PodmanFields` to `ContainerFields` in `cc-deck/internal/env/types.go`
- [x] T003 Update `EnvironmentRecord.Podman` field to `EnvironmentRecord.Container` with yaml tag `container` in `cc-deck/internal/env/types.go`
- [x] T004 Update all references to old type/field names across `cc-deck/internal/env/*.go` and `cc-deck/internal/cmd/env.go`
- [x] T005 Update CLI help text to show `container` instead of `podman` in `cc-deck/internal/cmd/env.go`
- [x] T006 Add `ErrPodmanNotFound` sentinel error in `cc-deck/internal/env/errors.go`
- [x] T007 Verify `make test` passes after renames

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Shared infrastructure that all user stories depend on

**CRITICAL**: No user story work can begin until this phase is complete

### Podman Interaction Layer

- [x] T008 Create `cc-deck/internal/podman/podman.go` with `Available()`, `IsRootless()`, and shared `run()` command runner per contract `contracts/podman-package.md`
- [x] T009 [P] Create `cc-deck/internal/podman/types.go` with `RunOpts`, `SecretMount`, `ContainerInfo` types per contract `contracts/podman-package.md`
- [x] T010 [P] Create `cc-deck/internal/podman/container.go` with `Run()`, `Start()`, `Stop()`, `Remove()`, `Inspect()` per contract `contracts/podman-package.md`
- [x] T011 [P] Create `cc-deck/internal/podman/volume.go` with `VolumeCreate()`, `VolumeRemove()`, `VolumeExists()` per contract `contracts/podman-package.md`
- [x] T012 [P] Create `cc-deck/internal/podman/secret.go` with `SecretCreate()`, `SecretRemove()`, `SecretExists()` per contract `contracts/podman-package.md`
- [x] T013 [P] Create `cc-deck/internal/podman/exec.go` with `Exec()` and `Cp()` per contract `contracts/podman-package.md`

### Definition Store

- [x] T014 Create `EnvironmentDefinition` type and `DefinitionFile` struct in `cc-deck/internal/env/definition.go` per contract `contracts/definition-store.md`
- [x] T015 Implement `DefinitionStore` with `Load()`, `Save()`, `FindByName()`, `Add()`, `Update()`, `Remove()`, `List()` in `cc-deck/internal/env/definition.go`
- [x] T016 [P] Create `cc-deck/internal/env/definition_test.go` with tests for CRUD operations, atomic writes, missing file handling, and `CC_DECK_DEFINITIONS_FILE` override

### State Schema v2

- [x] T017 Update `StateFile` to version 2: rename `Environments` to `Instances`, use `EnvironmentInstance` type (slim, no definition data) in `cc-deck/internal/env/state.go`
- [x] T018 Update `FileStateStore` methods to work with `EnvironmentInstance` records in `cc-deck/internal/env/state.go`
- [x] T019 [P] Update `cc-deck/internal/env/state_test.go` for v2 schema changes

**Checkpoint**: Foundation ready. `make test` passes. Podman package, definition store, and state v2 are available.

---

## Phase 3: User Story 1 - Create/Attach/Delete Container (Priority: P1) MVP

**Goal**: Users can create an isolated container, attach to its Zellij session, and delete it with cleanup.

**Independent Test**: Run `cc-deck env create mydev --type container --image quay.io/cc-deck/cc-deck-demo:latest`, then `cc-deck env attach mydev`, then `cc-deck env delete mydev`.

### Implementation for User Story 1

- [x] T020 [US1] Create `ContainerEnvironment` struct with `name`, `store`, `defs`, and type-specific option fields (`Ports`, `Credentials`, `AllPorts`, `KeepVolumes`) in `cc-deck/internal/env/container.go` per contract `contracts/container-environment.md`
- [x] T021 [US1] Implement naming helpers `containerName()`, `volumeName()`, `secretName()` in `cc-deck/internal/env/container.go`
- [x] T022 [US1] Implement `Type()`, `Name()` identity methods in `cc-deck/internal/env/container.go`
- [x] T023 [US1] Implement `Create()` method with full flow: validate name, check podman available, resolve image (flag/config/fallback), create volume, run container with `sleep infinity`, write definition + state in `cc-deck/internal/env/container.go`
- [x] T024 [US1] Implement `Attach()` method: auto-start if stopped, `podman exec -it cc-deck-<name> zellij attach cc-deck --create` via syscall.Exec in `cc-deck/internal/env/container.go`
- [x] T025 [US1] Implement `Delete()` method: stop if force, remove container, remove volume (unless KeepVolumes), remove secrets, remove definition + state in `cc-deck/internal/env/container.go`
- [x] T026 [US1] Implement stub methods returning `ErrNotSupported` for `Exec()`, `Push()`, `Pull()`, `Harvest()` in `cc-deck/internal/env/container.go`
- [x] T027 [US1] Add `EnvironmentTypeContainer` case to factory in `cc-deck/internal/env/factory.go`, passing both `FileStateStore` and `DefinitionStore`
- [x] T028 [US1] Add `--image`, `--port`, `--all-ports`, `--storage`, `--path`, `--credential` flags to `newEnvCreateCmd()` in `cc-deck/internal/cmd/env.go`
- [x] T029 [US1] Update `runEnvCreate()` to pass container-specific options to `ContainerEnvironment` fields before calling `Create()` in `cc-deck/internal/cmd/env.go`
- [x] T030 [US1] Add `--keep-volumes` flag to `newEnvDeleteCmd()` and pass to `ContainerEnvironment.KeepVolumes` in `cc-deck/internal/cmd/env.go`
- [x] T031 [US1] Extend `Defaults` struct with `Container` sub-struct (image, storage) in `cc-deck/internal/config/config.go`
- [x] T032 [US1] Implement image resolution: CLI flag, then config default, then `quay.io/cc-deck/cc-deck-demo:latest` fallback with warning in `cc-deck/internal/cmd/env.go`

**Checkpoint**: User Story 1 complete. Users can create, attach, and delete container environments.

---

## Phase 4: User Story 2 - Stop and Restart (Priority: P1)

**Goal**: Users can stop a running container to free resources and restart it later with workspace data intact.

**Independent Test**: Run `cc-deck env stop mydev`, verify stopped. Run `cc-deck env start mydev`, verify running and workspace intact.

### Implementation for User Story 2

- [x] T033 [US2] Implement `Start()` method: `podman.Start()`, update state to running in `cc-deck/internal/env/container.go`
- [x] T034 [US2] Implement `Stop()` method: `podman.Stop()`, update state to stopped in `cc-deck/internal/env/container.go`
- [x] T035 [US2] Update `runEnvStart()` and `runEnvStop()` to pass `DefinitionStore` to container environments in `cc-deck/internal/cmd/env.go`

**Checkpoint**: User Story 2 complete. Stop/start lifecycle works with data persistence.

---

## Phase 5: User Story 3 - List and Inspect (Priority: P1)

**Goal**: Users see all environments with reconciled status and can inspect detailed container info.

**Independent Test**: Create a container environment, run `cc-deck env list` to verify it appears with correct type and status. Run `cc-deck env status mydev` for details.

### Implementation for User Story 3

- [x] T036 [US3] Implement `Status()` method: `podman.Inspect()` for container state, reconcile with state store, return uptime and image info in `cc-deck/internal/env/container.go`
- [x] T037 [US3] Create `ReconcileContainerEnvs()` function: iterate container-type instances, reconcile each via `podman.Inspect()`, update state in `cc-deck/internal/env/container.go`
- [x] T038 [US3] Update `runEnvList()` to call `ReconcileContainerEnvs()` alongside existing `ReconcileLocalEnvs()` in `cc-deck/internal/cmd/env.go`
- [x] T039 [US3] Update `runEnvList()` to join definitions and state for display: show definition-only entries as "not created", warn on orphaned state in `cc-deck/internal/cmd/env.go`
- [x] T040 [US3] Update `writeEnvStatusText()` to show container-specific fields (image, ports, container name) for container environments in `cc-deck/internal/cmd/env.go`

**Checkpoint**: User Story 3 complete. List shows reconciled status across local and container environments.

---

## Phase 6: User Story 4 - File Transfer (Priority: P2)

**Goal**: Users can push local files into and pull files out of a container workspace.

**Independent Test**: Run `cc-deck env push mydev ./src` to copy files in. Run `cc-deck env pull mydev /workspace/results ./results` to copy out.

### Implementation for User Story 4

- [x] T041 [US4] Implement `Push()` method: validate container running, `podman.Cp(localPath, containerName+":/workspace/"+basename)` in `cc-deck/internal/env/container.go`
- [x] T042 [US4] Implement `Pull()` method: validate container running, `podman.Cp(containerName+":"+remotePath, localPath)` in `cc-deck/internal/env/container.go`
- [x] T043 [US4] Update `newEnvPushCmd()` and `newEnvPullCmd()` to resolve environment, pass `SyncOpts`, and call implementation in `cc-deck/internal/cmd/env.go`

**Checkpoint**: User Story 4 complete. File transfer works via `podman cp`.

---

## Phase 7: User Story 5 - Credential Injection (Priority: P2)

**Goal**: API keys are injected securely via podman secrets, not visible in `podman inspect`.

**Independent Test**: Create with `--credential ANTHROPIC_API_KEY=$ANTHROPIC_API_KEY`, attach, verify the key is available inside.

### Implementation for User Story 5

- [x] T044 [US5] Implement credential resolution in `Create()`: explicit `--credential` values, then auto-detect `ANTHROPIC_API_KEY` and `GOOGLE_APPLICATION_CREDENTIALS` from host env in `cc-deck/internal/env/container.go`
- [x] T045 [US5] Create podman secrets during `Create()` flow: `podman.SecretCreate("cc-deck-<name>-<key>", value)` for each credential in `cc-deck/internal/env/container.go`
- [x] T046 [US5] Add secret mounts to `RunOpts.Secrets` with `type=env,target=<KEY>` for injection as environment variables in `cc-deck/internal/env/container.go`
- [x] T047 [US5] Store credential key names in `EnvironmentDefinition.Credentials` for cleanup on delete in `cc-deck/internal/env/container.go`

**Checkpoint**: User Story 5 complete. Credentials injected securely, not visible in inspect output.

---

## Phase 8: User Story 6 - Execute Commands (Priority: P3)

**Goal**: Users can run one-off commands inside a container without full Zellij attach.

**Independent Test**: Run `cc-deck env exec mydev -- git status` and verify output.

### Implementation for User Story 6

- [x] T048 [US6] Implement `Exec()` method: validate container running, `podman.Exec(containerName, cmd, false)` in `cc-deck/internal/env/container.go`
- [x] T049 [US6] Update `newEnvExecCmd()` to resolve environment and call `Exec()` with args after `--` in `cc-deck/internal/cmd/env.go`

**Checkpoint**: User Story 6 complete. One-off command execution works.

---

## Phase 9: User Story 7 - Hand-Edit Definitions (Priority: P3)

**Goal**: Users can edit `environments.yaml` directly and the system respects changes on next lifecycle operation.

**Independent Test**: Edit image field in `environments.yaml`, delete and recreate the environment, verify new image is used.

### Implementation for User Story 7

- [x] T050 [US7] Update `Create()` to check for existing definition in `DefinitionStore` and use its values as defaults (definition takes precedence over CLI defaults, explicit flags override both) in `cc-deck/internal/env/container.go`
- [x] T051 [US7] Ensure `env list` correctly joins definitions without matching state records (showing "not created" status) in `cc-deck/internal/cmd/env.go`

**Checkpoint**: User Story 7 complete. Hand-edited definitions are respected.

---

## Phase 10: Polish and Cross-Cutting Concerns

**Purpose**: Documentation, cleanup, and validation

- [x] T052 [P] Add unit tests for `ContainerEnvironment` methods (mocking podman calls) in `cc-deck/internal/env/container_test.go`
- [x] T053 [P] Add unit tests for `internal/podman` package functions in `cc-deck/internal/podman/podman_test.go`
- [x] T054 [P] Update README.md with container environment feature description and spec table entry
- [x] T055 [P] Update Antora docs: create container environment guide page in `docs/modules/` (using prose plugin with cc-deck voice)
- [x] T056 [P] Update CLI reference in `docs/modules/reference/pages/cli.adoc` with new env commands and flags
- [x] T057 Run `quickstart.md` validation: execute each example command and verify expected behavior
- [x] T058 Run `make test` and `make lint` for final validation

---

## Dependencies and Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies, start immediately
- **Foundational (Phase 2)**: Depends on Phase 1 completion, BLOCKS all user stories
- **US1 (Phase 3)**: Depends on Phase 2 completion, MVP
- **US2 (Phase 4)**: Depends on US1 (needs running container)
- **US3 (Phase 5)**: Depends on US1 (needs created container)
- **US4 (Phase 6)**: Depends on US1 (needs running container)
- **US5 (Phase 7)**: Depends on US1 (enhances create flow)
- **US6 (Phase 8)**: Depends on US1 (needs running container)
- **US7 (Phase 9)**: Depends on US1 + US3 (needs definitions + list)
- **Polish (Phase 10)**: Depends on all desired stories being complete

### User Story Dependencies

```
Phase 1 (Setup) → Phase 2 (Foundational) → US1 (Create/Attach/Delete)
                                              │
                                              ├──> US2 (Stop/Start)
                                              ├──> US3 (List/Inspect)
                                              ├──> US4 (File Transfer)
                                              ├──> US5 (Credentials)
                                              ├──> US6 (Exec)
                                              └──> US7 (Hand-Edit) [also needs US3]
```

### Parallel Opportunities

After US1 completes, US2 through US6 can proceed in parallel (different methods, minimal file overlap).

---

## Parallel Example: Phase 2 (Foundational)

```
# Podman package files (all [P], no dependencies between files):
Task T009: types.go
Task T010: container.go
Task T011: volume.go
Task T012: secret.go
Task T013: exec.go

# Definition store and state v2 (parallel with podman package):
Task T014-T016: definition.go + tests
Task T017-T019: state.go v2 + tests
```

## Parallel Example: After US1

```
# All of these can run in parallel after US1 checkpoint:
US2: T033-T035 (Stop/Start)
US3: T036-T040 (List/Inspect)
US4: T041-T043 (File Transfer)
US5: T044-T047 (Credentials)
US6: T048-T049 (Exec)
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (type renames)
2. Complete Phase 2: Foundational (podman package, definition store, state v2)
3. Complete Phase 3: User Story 1 (create, attach, delete)
4. **STOP and VALIDATE**: Test create/attach/delete cycle manually
5. Commit and verify

### Incremental Delivery

1. Setup + Foundational → Foundation ready
2. US1 → Create/attach/delete works (MVP)
3. US2 → Stop/start lifecycle
4. US3 → List with reconciliation
5. US4 → File transfer
6. US5 → Secure credentials
7. US6 → Exec commands
8. US7 → Hand-edit definitions
9. Polish → Docs + tests + validation

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story
- Each user story is independently testable after US1
- Skip podman integration tests when podman not available (`t.Skip`)
- Commit after each phase checkpoint
- Constitution: documentation tasks (T054-T056) use prose plugin with cc-deck voice
