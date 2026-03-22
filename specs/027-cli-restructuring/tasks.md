# Tasks: CLI Command Restructuring

**Input**: Design documents from `/specs/027-cli-restructuring/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md, contracts/

**Tests**: Tests are included for verifying promoted command behavior, help output structure, and removal of legacy commands.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: Foundational - Remove Legacy K8s Commands (US4)

**Purpose**: Clean up the codebase by removing all Kubernetes-specific top-level commands and their backing packages before restructuring.

**Goal**: All six legacy K8s commands (deploy, connect, list, delete, logs, sync) and their backing code are removed. The CLI compiles and tests pass.

**Independent Test**: Running `cc-deck deploy`, `cc-deck connect`, `cc-deck sync` returns "unknown command" errors. `make test` and `make lint` pass.

- [ ] T001 [US4] Remove legacy command registrations (NewDeployCmd, NewConnectCmd, NewListCmd, NewDeleteCmd, NewLogsCmd) from `cc-deck/cmd/cc-deck/main.go`
- [ ] T002 [P] [US4] Delete legacy command files: `cc-deck/internal/cmd/deploy.go`, `cc-deck/internal/cmd/connect.go`, `cc-deck/internal/cmd/list.go`, `cc-deck/internal/cmd/delete.go`, `cc-deck/internal/cmd/logs.go`, `cc-deck/internal/cmd/sync.go`
- [ ] T003 [P] [US4] Delete K8s session functions: `cc-deck/internal/session/deploy.go`, `cc-deck/internal/session/connect.go`, `cc-deck/internal/session/list.go`, `cc-deck/internal/session/delete.go`, `cc-deck/internal/session/logs.go`, `cc-deck/internal/session/validate.go`
- [ ] T004 [P] [US4] Delete entire `cc-deck/internal/k8s/` directory (client.go, discovery.go, apply.go, errors.go, network.go, overlay.go, resources.go, and tests)
- [ ] T005 [P] [US4] Delete entire `cc-deck/internal/sync/` directory (sync.go, sync_test.go)
- [ ] T006 [P] [US4] Delete entire `cc-deck/internal/integration/` directory (integration_test.go, helpers_test.go)
- [ ] T007 [US4] Remove K8s Secret validation from `cc-deck/internal/cmd/profile.go` (remove k8s import, keep profile CRUD functionality)
- [ ] T008 [US4] Run `go mod tidy` in `cc-deck/` to remove unused dependencies (k8s.io/client-go and transitive deps)
- [ ] T009 [US4] Verify `make test` and `make lint` pass after removal

**Checkpoint**: Legacy K8s code removed. CLI compiles cleanly with reduced dependency tree.

---

## Phase 2: User Story 1 - Daily Commands at Top Level (Priority: P1)

**Goal**: Six high-frequency env commands (attach, list, status, start, stop, logs) are accessible at the top level while retaining the env subcommand path.

**Independent Test**: `cc-deck attach mydev` and `cc-deck env attach mydev` produce identical output. Same for list, status, start, stop, logs.

- [ ] T010 [US1] Extract shared constructor `newAttachCmdCore(gf)` from `newEnvAttachCmd(gf)` in `cc-deck/internal/cmd/env.go`, then have `newEnvAttachCmd` call the shared constructor
- [ ] T011 [P] [US1] Extract shared constructor `newListCmdCore(gf)` from `newEnvListCmd(gf)` in `cc-deck/internal/cmd/env.go`
- [ ] T012 [P] [US1] Extract shared constructor `newStatusCmdCore(gf)` from `newEnvStatusCmd(gf)` in `cc-deck/internal/cmd/env.go`
- [ ] T013 [P] [US1] Extract shared constructor `newStartCmdCore(gf)` from `newEnvStartCmd(gf)` in `cc-deck/internal/cmd/env.go`
- [ ] T014 [P] [US1] Extract shared constructor `newStopCmdCore(gf)` from `newEnvStopCmd(gf)` in `cc-deck/internal/cmd/env.go`
- [ ] T015 [P] [US1] Extract shared constructor `newLogsCmdCore(gf)` from `newEnvLogsCmd(gf)` in `cc-deck/internal/cmd/env.go`
- [ ] T016 [US1] Create `cc-deck/internal/cmd/env_promote.go` with exported factories: `NewAttachCmd(gf)`, `NewListCmd(gf)`, `NewStatusCmd(gf)`, `NewStartCmd(gf)`, `NewStopCmd(gf)`, `NewLogsCmd(gf)`, each calling their shared constructor
- [ ] T017 [US1] Register six promoted commands in `cc-deck/cmd/cc-deck/main.go` via the new exported factories
- [ ] T018 [US1] Add test in `cc-deck/internal/cmd/env_promote_test.go` verifying all six promoted commands exist on root, have correct Use/Short/Aliases, and share RunE behavior with env counterparts

**Checkpoint**: Both `cc-deck <cmd>` and `cc-deck env <cmd>` work identically for all six promoted commands.

---

## Phase 3: User Story 2 - Organized Help Output (Priority: P2)

**Goal**: Help output organizes commands into four named groups: Daily, Session, Environment, Setup.

**Independent Test**: `cc-deck --help` shows commands under correct group headings in the expected display order.

- [ ] T019 [US2] Add four command groups (Daily, Session, Environment, Setup) to root command in `cc-deck/cmd/cc-deck/main.go` using `rootCmd.AddGroup()` in display order
- [ ] T020 [US2] Assign GroupID to each command registration in `cc-deck/cmd/cc-deck/main.go`: Daily (attach, list, status, start, stop, logs), Session (snapshot), Environment (env), Setup (plugin, profile, domains, image)
- [ ] T021 [US2] Add test in `cc-deck/internal/cmd/env_promote_test.go` verifying help output contains all four group headings and correct command placement

**Checkpoint**: `cc-deck --help` shows organized command groups matching the contract in `contracts/command-hierarchy.md`.

---

## Phase 4: User Story 3 - Backward Compatibility (Priority: P2)

**Goal**: Verify that existing `cc-deck env <cmd>` paths continue to work identically after promotion.

**Independent Test**: Running every promoted command through both paths produces identical output, exit codes, and side effects.

- [ ] T022 [US3] Add test in `cc-deck/internal/cmd/env_promote_test.go` verifying all six commands exist under both root and env with identical flags, args, and aliases
- [ ] T023 [US3] Add test verifying shell completion includes both top-level promoted commands and `env` subcommand with all its subcommands

**Checkpoint**: All dual-path commands verified. Shell completion works for both paths.

---

## Phase 5: Polish & Cross-Cutting Concerns

**Purpose**: Documentation updates and final validation

- [ ] T024 [P] Update `README.md` with new command structure: promoted top-level commands, removed K8s commands, help group organization
- [ ] T025 [P] Update CLI reference documentation in `docs/modules/reference/pages/cli.adoc` to reflect new command hierarchy (add promoted commands, remove legacy K8s commands)
- [ ] T026 [P] Add feature 027 to the "Feature Specifications" table in `README.md`
- [ ] T027 Run `make test` and `make lint` for final validation
- [ ] T028 Verify `cc-deck --help` output matches contract in `specs/027-cli-restructuring/contracts/command-hierarchy.md`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Foundational/US4)**: No dependencies, can start immediately. BLOCKS Phase 2.
- **Phase 2 (US1)**: Depends on Phase 1 completion (clean codebase without K8s command name collisions).
- **Phase 3 (US2)**: Depends on Phase 2 (promoted commands must exist before grouping them).
- **Phase 4 (US3)**: Depends on Phase 2 (commands must exist in both paths before testing compatibility).
- **Phase 5 (Polish)**: Depends on Phases 2, 3, 4 completion.

### User Story Dependencies

- **US4 (P3)**: Foundational, done first despite lower priority because it simplifies the codebase.
- **US1 (P1)**: Depends on US4 completion. Core feature.
- **US2 (P2)**: Depends on US1 (groups reference promoted commands).
- **US3 (P2)**: Depends on US1 (tests verify dual-path behavior). Can run in parallel with US2.

### Within Each Phase

- Tasks marked [P] within a phase can run in parallel.
- Non-[P] tasks must run sequentially.

### Parallel Opportunities

- T002, T003, T004, T005, T006 can all run in parallel (deleting independent files/directories)
- T011, T012, T013, T014, T015 can all run in parallel (extracting independent shared constructors)
- T024, T025, T026 can run in parallel (independent documentation files)
- US2 (Phase 3) and US3 (Phase 4) can run in parallel after US1 completes

---

## Parallel Example: Phase 1

```bash
# Delete all legacy packages in parallel:
Task: "Delete legacy command files (deploy, connect, list, delete, logs, sync)"
Task: "Delete K8s session functions"
Task: "Delete internal/k8s/ directory"
Task: "Delete internal/sync/ directory"
Task: "Delete internal/integration/ directory"
```

## Parallel Example: Phase 2

```bash
# Extract all shared constructors in parallel (after T010 establishes the pattern):
Task: "Extract newListCmdCore from newEnvListCmd"
Task: "Extract newStatusCmdCore from newEnvStatusCmd"
Task: "Extract newStartCmdCore from newEnvStartCmd"
Task: "Extract newStopCmdCore from newEnvStopCmd"
Task: "Extract newLogsCmdCore from newEnvLogsCmd"
```

---

## Implementation Strategy

### MVP First (US4 + US1)

1. Complete Phase 1: Remove legacy K8s commands
2. Complete Phase 2: Promote daily commands
3. **STOP and VALIDATE**: Both `cc-deck attach` and `cc-deck env attach` work
4. This is a functional MVP delivering the core UX improvement

### Incremental Delivery

1. Remove K8s legacy (Phase 1) -> cleaner codebase
2. Promote commands (Phase 2) -> core UX improvement (MVP!)
3. Add help groups (Phase 3) -> polished help output
4. Verify compatibility (Phase 4) -> confidence in dual-path
5. Documentation (Phase 5) -> complete feature

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- US4 is done first (foundational) despite being P3 priority because it reduces complexity for subsequent phases
- Commit after each phase or logical group of tasks
- The `runXxx` business logic functions in env.go are NOT modified, only the command constructors are refactored
