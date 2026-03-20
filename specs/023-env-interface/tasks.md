# Tasks: Environment Interface and CLI

**Input**: Design documents from `/specs/023-env-interface/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md, contracts/

**Tests**: Tests included for foundational components (state store, validation, migration, local environment) as these are critical infrastructure for specs 024-026.

**Organization**: Tasks grouped by user story. US2 precedes US1 because create produces data that list reads.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2)
- Include exact file paths in descriptions

---

## Phase 1: Setup

**Purpose**: Create the new `internal/env/` package structure

- [ ] T001 Create package directory `cc-deck/internal/env/` and verify `make test` still passes

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core types, interfaces, state management, and validation that ALL user stories depend on

- [ ] T002 [P] Define type enums and interface in `cc-deck/internal/env/types.go`: EnvironmentType (Local, Podman, K8sDeploy, K8sSandbox), EnvironmentState (Running, Stopped, Creating, Error, Unknown), StorageType (HostPath, NamedVolume, EmptyDir, PVC), SyncStrategy (Copy, GitHarvest, RemoteGit), plus StorageConfig, SyncConfig, K8sFields, PodmanFields, SandboxFields structs per data-model.md
- [ ] T003 [P] Define Environment interface and option structs in `cc-deck/internal/env/interface.go`: Environment interface with all methods (Type, Name, Create, Start, Stop, Delete, Status, Attach, Exec, Push, Pull, Harvest), CreateOpts, SyncOpts, HarvestOpts, EnvironmentStatus, SessionInfo per contracts/environment-interface.md
- [ ] T004 [P] Define sentinel errors in `cc-deck/internal/env/errors.go`: ErrNotSupported, ErrNotImplemented, ErrNameConflict, ErrNotFound, ErrInvalidName, ErrZellijNotFound, ErrRunning per contracts/environment-interface.md
- [ ] T005 [P] Implement name validation in `cc-deck/internal/env/validate.go` and `cc-deck/internal/env/validate_test.go`: ValidateEnvName function with regex `^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`, max 40 chars, table-driven tests for valid names, invalid names (uppercase, special chars, too long, leading/trailing hyphens), and single-char names
- [ ] T006 Implement StateStore in `cc-deck/internal/env/state.go` and `cc-deck/internal/env/state_test.go`: StateFile struct with version field, EnvironmentRecord struct, FileStateStore implementing Load/Save/FindByName/Add/Update/Remove/List with atomic writes (write-temp-rename), ListFilter by type, auto-create directory on save, handle missing/corrupted state file gracefully, use `xdg.StateHome` for default path. Tests: CRUD operations, atomic write, missing file, version field, type filtering
- [ ] T007 Implement config migration in `cc-deck/internal/env/migrate.go` and `cc-deck/internal/env/migrate_test.go`: MigrateFromConfig function that reads config.yaml sessions, converts to K8s-type EnvironmentRecords per data-model.md migration table, writes to state.yaml, removes sessions from config.yaml. Called from StateStore.Load when state file does not exist. Tests: migration with sessions, migration with empty sessions, idempotency
- [ ] T008 Implement factory in `cc-deck/internal/env/factory.go`: NewEnvironment function that creates LocalEnvironment for type Local, returns ErrNotImplemented for Podman/K8sDeploy/K8sSandbox

**Checkpoint**: Foundation ready. All types, interfaces, state management, and validation are in place.

---

## Phase 3: User Story 2 - Create and Attach to a Local Environment (Priority: P1)

**Goal**: Register a local Zellij session as a tracked environment, attach to it, and delete the record

**Independent Test**: `cc-deck env create mydev --type local && cc-deck env list && cc-deck env attach mydev`

### Implementation for User Story 2

- [ ] T009 [US2] Implement LocalEnvironment in `cc-deck/internal/env/local.go`: struct with name and store fields, Create validates name + checks Zellij binary (reuse plugin/zellij.go FindZellij) + adds record to store with state=running, Attach runs `zellij attach cc-deck-<name> --create --layout cc-deck`, Delete removes record (optionally kills Zellij session via `zellij kill-session cc-deck-<name>`), Start/Stop return ErrNotSupported, Exec/Push/Pull/Harvest return ErrNotSupported, Status checks `zellij list-sessions` for session `cc-deck-<name>` and returns Running/Unknown
- [ ] T010 [US2] Write tests in `cc-deck/internal/env/local_test.go`: test Create adds record to store, test Create rejects duplicate name, test Create rejects invalid name, test Start/Stop return ErrNotSupported, test Delete removes record from store
- [ ] T011 [US2] Implement `cc-deck env` parent command and `create`, `attach`, `delete` subcommands in `cc-deck/internal/cmd/env.go`: NewEnvCmd parent (no RunE), newEnvCreateCmd with required `--type` flag and name arg (validates name, creates StateStore, calls factory + env.Create), newEnvAttachCmd with name arg (loads store, finds record, calls factory + env.Attach), newEnvDeleteCmd with name arg and `--force` flag (loads store, checks state, calls factory + env.Delete)
- [ ] T012 [US2] Register env command in `cc-deck/cmd/cc-deck/main.go`: add `rootCmd.AddCommand(cmd.NewEnvCmd(gf))` alongside existing commands

**Checkpoint**: `cc-deck env create mydev --type local`, `cc-deck env attach mydev`, and `cc-deck env delete mydev` all work. State persisted to `~/.local/state/cc-deck/state.yaml`.

---

## Phase 4: User Story 1 - List All Environments (Priority: P1)

**Goal**: Show all tracked environments with type, status, storage, and age in table or JSON format

**Independent Test**: Create environments, then run `cc-deck env list` and `cc-deck env list -o json`

### Implementation for User Story 1

- [ ] T013 [US1] Implement `list` subcommand in `cc-deck/internal/cmd/env.go`: newEnvListCmd with `--type` filter flag, loads StateStore, calls List with optional type filter, reconciles local environments by checking `zellij list-sessions`, formats output as table (default) or json/yaml using global `-o` flag. Table columns: NAME, TYPE, STATUS, STORAGE, LAST ATTACHED, AGE. Empty state shows headers + hint about `cc-deck env create`. Follow output pattern from existing `session/list.go`
- [ ] T014 [US1] Add reconciliation helper in `cc-deck/internal/env/local.go`: ReconcileLocalEnvs function that runs `zellij list-sessions`, parses output, updates state of local environment records (running if session found, unknown if not). Called from list command before displaying results

**Checkpoint**: `cc-deck env list`, `cc-deck env list --type local`, and `cc-deck env list -o json` all work correctly.

---

## Phase 5: User Story 3 - Inspect Environment Details (Priority: P2)

**Goal**: Show detailed environment info including agent session states for running environments

**Independent Test**: `cc-deck env status mydev` shows metadata and agent sessions

### Implementation for User Story 3

- [ ] T015 [US3] Implement local Status method enhancement in `cc-deck/internal/env/local.go`: Status reads `/tmp/cc-deck-pane-map.json` to populate SessionInfo slice (name, activity, branch, last_event) for running local environments. Return EnvironmentStatus with state, since (created_at), and sessions
- [ ] T016 [US3] Implement `status` subcommand in `cc-deck/internal/cmd/env.go`: newEnvStatusCmd with name arg, loads store, finds record, calls factory + env.Status, formats detailed output as key-value block (Environment, Type, Status, Storage, Uptime, Attached) plus agent sessions table. Supports `-o json/yaml` via global flag. For stopped environments, skip session reading

**Checkpoint**: `cc-deck env status mydev` shows detailed info. JSON output works via `-o json`.

---

## Phase 6: User Story 4 - Stop and Restart an Environment (Priority: P2)

**Goal**: Stop/start commands with state transition validation (local returns "not supported")

**Independent Test**: `cc-deck env stop mydev` reports "not supported for local environments"

### Implementation for User Story 4

- [ ] T017 [US4] Implement `start` and `stop` subcommands in `cc-deck/internal/cmd/env.go`: newEnvStartCmd and newEnvStopCmd with name arg, load store, find record, validate state transition (stop: must be running, start: must be stopped), call factory + env.Start/Stop, update record state in store. For local environments, the interface methods return ErrNotSupported which the CLI formats as an informational message

**Checkpoint**: `cc-deck env stop mydev` shows "stop is not supported for local environments". State transition validation works.

---

## Phase 7: User Story 5 - Backward-Compatible Aliases (Priority: P3)

**Goal**: Existing deploy/connect/delete/list/logs commands continue working as aliases

**Independent Test**: `cc-deck list` delegates to `cc-deck env list` and produces identical output

### Implementation for User Story 5

- [ ] T018 [P] [US5] Convert `cc-deck/internal/cmd/deploy.go` to alias: keep existing RunE but mark Hidden, add deprecation note in Long text pointing to `cc-deck env create --type k8s`
- [ ] T019 [P] [US5] Convert `cc-deck/internal/cmd/connect.go` to alias: keep existing RunE but mark Hidden, add deprecation note pointing to `cc-deck env attach`
- [ ] T020 [P] [US5] Convert `cc-deck/internal/cmd/delete.go` to alias: keep existing RunE but mark Hidden, add deprecation note pointing to `cc-deck env delete`
- [ ] T021 [P] [US5] Convert `cc-deck/internal/cmd/list.go` to alias: keep existing RunE but mark Hidden, add deprecation note pointing to `cc-deck env list`
- [ ] T022 [P] [US5] Convert `cc-deck/internal/cmd/logs.go` to alias: keep existing RunE but mark Hidden, add deprecation note pointing to `cc-deck env logs`

**Checkpoint**: All existing commands still work. `--help` shows deprecation notes. New `cc-deck env` commands are the primary interface.

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: Stub commands, documentation, final validation

- [ ] T023 [P] Add stub subcommands in `cc-deck/internal/cmd/env.go`: newEnvExecCmd, newEnvPushCmd, newEnvPullCmd, newEnvHarvestCmd, newEnvLogsCmd, each returning "not yet implemented" error with hint about which spec will implement them
- [ ] T024 [P] Update `README.md`: add spec 023 to Feature Specifications table with status, add `cc-deck env` command group to CLI reference section with subcommand descriptions
- [ ] T025 Run `quickstart.md` validation: execute all verification commands from quickstart.md against a clean build (`make install`), verify create/list/attach/status/delete cycle works end-to-end

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies, start immediately
- **Foundational (Phase 2)**: Depends on Setup. BLOCKS all user stories
- **US2 (Phase 3)**: Depends on Foundational. Produces data for US1
- **US1 (Phase 4)**: Depends on Foundational. Can run in parallel with US2 but benefits from US2 being done first for testing
- **US3 (Phase 5)**: Depends on US2 (needs environments to inspect)
- **US4 (Phase 6)**: Depends on US2 (needs environments to start/stop)
- **US5 (Phase 7)**: Depends on Foundational. Can run in parallel with US1-US4
- **Polish (Phase 8)**: Depends on all user stories

### User Story Dependencies

- **US2 (P1)**: Depends on Foundational only. Creates environments for all other stories to operate on
- **US1 (P1)**: Depends on Foundational. Independently testable (can list even with empty state file)
- **US3 (P2)**: Depends on US2 (needs a created environment to inspect)
- **US4 (P2)**: Depends on US2 (needs a created environment to start/stop)
- **US5 (P3)**: Depends on Foundational only. Modifies existing commands, independent of new env commands

### Within Each User Story

- Types/interface before implementation
- Implementation before CLI commands
- CLI commands before integration testing

### Parallel Opportunities

- T002, T003, T004, T005 can all run in parallel (different files, no dependencies)
- T009 and T010 can run in parallel (implementation + tests for local env)
- T018-T022 can all run in parallel (each modifies a different existing file)
- T023 and T024 can run in parallel (stubs vs docs)
- US5 can run in parallel with US1-US4

---

## Parallel Example: Foundational Phase

```
# Launch all type/interface/error definitions in parallel:
Task T002: "Define type enums in cc-deck/internal/env/types.go"
Task T003: "Define Environment interface in cc-deck/internal/env/interface.go"
Task T004: "Define sentinel errors in cc-deck/internal/env/errors.go"
Task T005: "Implement name validation in cc-deck/internal/env/validate.go"

# Then sequentially (depends on types):
Task T006: "Implement StateStore in cc-deck/internal/env/state.go"
Task T007: "Implement config migration in cc-deck/internal/env/migrate.go"
Task T008: "Implement factory in cc-deck/internal/env/factory.go"
```

## Parallel Example: User Story 5 (Aliases)

```
# All alias conversions in parallel (different files):
Task T018: "Convert deploy.go to alias"
Task T019: "Convert connect.go to alias"
Task T020: "Convert delete.go to alias"
Task T021: "Convert list.go to alias"
Task T022: "Convert logs.go to alias"
```

---

## Implementation Strategy

### MVP First (User Story 2 + User Story 1)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational (CRITICAL)
3. Complete Phase 3: US2 (create/attach/delete)
4. Complete Phase 4: US1 (list)
5. **STOP and VALIDATE**: Create a local env, list it, attach, delete

### Incremental Delivery

1. Setup + Foundational -> Foundation ready
2. US2 (create/attach/delete) -> Core workflow works (MVP)
3. US1 (list) -> Visibility into environments
4. US3 (status) -> Detailed inspection
5. US4 (start/stop) -> Lifecycle management (mostly prep for remote envs)
6. US5 (aliases) -> Backward compatibility
7. Polish -> Stubs, docs, validation

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story
- Local environment is a thin wrapper: most methods return ErrNotSupported
- Stubs for exec/push/pull/harvest/logs are deferred to Polish since they just return errors
- Migration (T007) runs automatically on first StateStore.Load, no manual trigger needed
- All existing K8s functionality continues working through aliases until spec 024 refactors it behind the Environment interface
