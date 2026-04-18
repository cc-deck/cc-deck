# Tasks: CLI Rename - Workspace & Build

**Input**: Design documents from `specs/039-cli-rename-ws-build/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/cli-commands.md

**Tests**: Existing tests are updated as part of Phase 5. No new test creation needed since this is a pure rename with no behavioral changes.

**Organization**: Tasks are grouped by user story where possible. Because a rename feature touches shared files across multiple stories, foundational tasks handle the structural renames, then story-specific tasks add the unique behaviors each story requires.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: Setup

**Purpose**: No setup needed. This is a rename within an existing project.

(No tasks)

---

## Phase 2: Foundational (Core File Renames)

**Purpose**: Rename source files and update package structure. MUST complete before story-specific work.

**CRITICAL**: These renames affect all user stories. Complete this phase first.

- [X] T001 Rename `cc-deck/internal/cmd/env.go` to `cc-deck/internal/cmd/ws.go` and update package references: `NewEnvCmd` → `NewWsCmd`, `newEnvCreateCmd` → `newWsNewCmd`, `newEnvDeleteCmd` → `newWsKillCmd`, `newEnvAttachCmd` → `newWsAttachCmd`, `newEnvListCmd` → `newWsListCmd`, `newEnvStatusCmd` → `newWsStatusCmd`, `newEnvStartCmd` → `newWsStartCmd`, `newEnvStopCmd` → `newWsStopCmd`, `newEnvLogsCmd` → `newWsLogsCmd`, `newEnvExecCmd` → `newWsExecCmd`, `newEnvPushCmd` → `newWsPushCmd`, `newEnvPullCmd` → `newWsPullCmd`, `newEnvHarvestCmd` → `newWsHarvestCmd`, `newEnvPruneCmd` → `newWsPruneCmd`, `newEnvRefreshCredsCmd` → `newWsRefreshCredsCmd`. Update all `*CmdCore` function names similarly. Update the `Use` field to `"ws"`, add `Aliases: []string{"workspace"}`. Update subcommand group titles from environment-centric to workspace-centric.
- [X] T002 Rename `cc-deck/internal/cmd/env_promote.go` to `cc-deck/internal/cmd/ws_promote.go` and update exported function names: `NewAttachCmd`, `NewListCmd` stay. Remove exports for `NewStatusCmd`, `NewStartCmd`, `NewStopCmd`, `NewLogsCmd`. Extract `newExecCmdCore` from `newWsExecCmd` (following the attach/list CmdCore pattern) and add new exported `NewExecCmd`.
- [X] T003 Rename `cc-deck/internal/cmd/setup.go` to `cc-deck/internal/cmd/build.go` and update: `NewSetupCmd` → `NewBuildCmd`, `Use` field to `"build"`, update all help text and error messages referencing "setup".
- [X] T004 Rename directory `cc-deck/internal/setup/` to `cc-deck/internal/build/` and update: all import paths referencing `internal/setup` across the codebase, package declaration from `setup` to `build`. This covers ALL files in the directory: `init.go`, `manifest.go`, `runtime.go`, `embed.go` (update template path `"templates/cc-deck-setup.yaml.tmpl"` to `"templates/cc-deck-build.yaml.tmpl"`), `init_test.go`, `manifest_test.go`. Rename template file `templates/cc-deck-setup.yaml.tmpl` to `templates/cc-deck-build.yaml.tmpl`. Update manifest filename string from `cc-deck-setup.yaml` to `cc-deck-build.yaml` in all Go source files. Update embedded command docs `commands/cc-deck.build.md` (~15 refs) and `commands/cc-deck.capture.md` (~3 refs) replacing `cc-deck-setup.yaml` with `cc-deck-build.yaml`. Update shell scripts `scripts/validate-manifest.sh` (~2 refs) and `scripts/update-manifest.sh` (~3 refs) replacing `cc-deck-setup.yaml` with `cc-deck-build.yaml`.

**Checkpoint**: All source files renamed. Code does not compile yet (main.go still references old names).

---

## Phase 3: User Story 1 - Daily workspace operations (Priority: P1)

**Goal**: `cc-deck ws new`, `ws delete`, `ws list` (alias `ls`), `ws attach`, and `workspace` alias all work.

**Independent Test**: Run `cc-deck ws new mydev`, `cc-deck ws list`, `cc-deck ws delete mydev --force` and verify identical behavior to former env commands.

- [X] T005 [US1] Update subcommand `Use` fields in `cc-deck/internal/cmd/ws.go`: change `"create [name]"` to `"new [name]"`, `"delete [name]"` to `"kill [name]"`. Ensure `list` subcommand retains `Aliases: []string{"ls"}`. Update the ws parent command's Long description to reference "workspaces" instead of "environments". Update all help text strings within ws.go that reference old command names (e.g., `"Use 'cc-deck env create' to get started."` → `"Use 'cc-deck ws new' to get started."`).
- [X] T006 [US1] Update internal function names in `cc-deck/internal/cmd/ws.go` for create/delete: `createFlags` struct and `runEnvCreate` function to `newFlags`/`runWsNew`, `runEnvDelete` to `runWsKill`. Update `cmd_context` and `resolveEnvironmentName` references if they appear in help text (keep function names since they reference internal env package).

**Checkpoint**: `ws` command tree works with all renamed subcommands and workspace alias.

---

## Phase 4: User Story 2 - Build artifact management (Priority: P2)

**Goal**: `cc-deck build init`, `build run`, `build verify`, `build diff` work identically to former setup commands.

**Independent Test**: Run `cc-deck build init` in a project directory and verify manifest is created as `cc-deck-build.yaml`.

- [X] T007 [US2] Update help text and error messages in `cc-deck/internal/cmd/build.go` to reference "build" instead of "setup". Update the parent command Short/Long descriptions.
- [X] T008 [US2] Update help text and user-facing messages in `cc-deck/internal/cmd/build.go` referencing "setup" to say "build". Note: manifest filename updates in `internal/build/` are handled by T004.

**Checkpoint**: Build commands work with new names and manifest filename.

---

## Phase 5: User Story 3 - Config parent command (Priority: P2)

**Goal**: `cc-deck config plugin`, `config profile`, `config domains`, `config completion` all work.

**Independent Test**: Run `cc-deck config plugin status` and verify identical behavior to former `cc-deck plugin status`.

- [X] T009 [US3] Create `cc-deck/internal/cmd/config.go` with `NewConfigCmd` function. The config parent command (`Use: "config"`, `Short: "System configuration"`) registers `NewPluginCmd`, `NewProfileCmd`, `NewDomainsCmd` as subcommands.
- [X] T010 [US3] Move `newCompletionCmd` from `cc-deck/cmd/cc-deck/main.go` to `cc-deck/internal/cmd/config.go` as exported `NewCompletionCmd`. Register it as a subcommand of `config`. Remove the old `newCompletionCmd` from main.go.

**Checkpoint**: All config subcommands accessible under `cc-deck config`.

---

## Phase 6: User Story 4 + 5 - Promoted shortcuts and hidden hook (Priority: P3)

**Goal**: `attach`, `ls`, `exec` work at top level. `hook` hidden from help.

**Independent Test**: Run `cc-deck ls`, `cc-deck attach mydev`, `cc-deck exec mydev -- echo hi` at top level. Run `cc-deck --help` and verify hook is absent.

- [X] T011 [US4] Update `cc-deck/cmd/cc-deck/main.go`: Replace group definitions with new groups (`workspace`, `session`, `build`, `config`). Register promoted commands in workspace group: `NewAttachCmd`, `NewListCmd` (which already has `ls` alias), `NewExecCmd` (new export from T002). Register `NewWsCmd` in workspace group. Register `NewSnapshotCmd` in session group. Register `NewBuildCmd` in build group. Register `NewConfigCmd` in config group.
- [X] T012 [US4] [US5] Remove demoted commands from top-level registration in `cc-deck/cmd/cc-deck/main.go`: remove `NewStatusCmd`, `NewStartCmd`, `NewStopCmd`, `NewLogsCmd` from the daily/workspace group (they remain as ws subcommands only). Verify `hook` command is registered via `rootCmd.AddCommand(cmd.NewHookCmd())` without a group (already `Hidden: true` in hook.go). Remove the old completion command registration.

**Checkpoint**: `make lint` passes. The full CLI structure matches contracts/cli-commands.md.

---

## Phase 7: Test Updates

**Purpose**: Rename test files and update all command string references so `make test` passes.

- [X] T013 [P] Rename `cc-deck/internal/cmd/env_promote_test.go` to `cc-deck/internal/cmd/ws_promote_test.go` and update: constructor references (`NewEnvCmd` → `NewWsCmd`, `NewSetupCmd` → `NewBuildCmd`), group IDs (`"environment"` → `"workspace"`, `"setup"` split into `"build"` and `"config"`), command name assertions (`"env"` → `"ws"`, `"create"` → `"new"`, `"delete"` → `"kill"`), `envOnly` list, promoted commands list (reduce to attach/ls/exec), setup commands list (plugin/profile/domains move to config).
- [X] T014 [P] Rename `cc-deck/internal/cmd/env_integration_test.go` to `cc-deck/internal/cmd/ws_integration_test.go` and update: all `run(t, gf, "env", ...)` calls to `run(t, gf, "ws", ...)`, `"create"` → `"new"`, `"delete"` → `"kill"`, `buildRootCmd()` to use `NewWsCmd`, test function names from `TestEnv*` to `TestWs*`.
- [X] T015 [P] Rename `cc-deck/internal/cmd/env_create_test.go` to `cc-deck/internal/cmd/ws_new_test.go` and update: `runEnvCreate` → `runWsNew`, `createFlags` → `newFlags`, `newTestCreateCmd` → `newTestNewCmd`, test function names from `TestRunEnvCreate*` to `TestRunWsNew*`.
- [X] T016 [P] Rename `cc-deck/internal/cmd/env_prune_test.go` to `cc-deck/internal/cmd/ws_prune_test.go` and update: `runEnvPrune` → `runWsPrune`, test function names.
- [X] T017 [P] Rename `cc-deck/internal/cmd/env_resolve_test.go` to `cc-deck/internal/cmd/ws_resolve_test.go` and update function references.
- [X] T018 [P] Rename `cc-deck/internal/cmd/setup_test.go` to `cc-deck/internal/cmd/build_test.go` and update function references.
- [X] T019 [P] Update `cc-deck/internal/cmd/compose_smoke_test.go` (keep filename): replace all `"env"` → `"ws"`, `"create"` → `"new"`, `"delete"` → `"kill"` in `ccd()` call arguments.
- [X] T020 [P] Rename `cc-deck/internal/e2e/env_test.go` to `cc-deck/internal/e2e/ws_test.go` and update: all `te.mustRun("env", ...)` → `te.mustRun("ws", ...)`, `"create"` → `"new"`, `"delete"` → `"kill"`, test function names.
- [X] T021 [P] Update `cc-deck/internal/build/init_test.go` and `cc-deck/internal/build/manifest_test.go`: replace `"cc-deck-setup.yaml"` -> `"cc-deck-build.yaml"` in all string references. Note: if T004 already updated these during the directory rename, verify and skip.
- [X] T021b [P] Update `cc-deck/tests/domain-smoke-test.sh`: replace `cc-deck-setup.yaml` -> `cc-deck-build.yaml` (1 occurrence at line 69).
- [X] T022 Run `make test` and `make lint` to verify all tests pass and no lint errors exist. Fix any remaining references to old names found by the compiler or linter.

**Checkpoint**: `make test` and `make lint` pass cleanly.

---

## Phase 8: Documentation Updates

**Purpose**: Update all documentation to reflect new command names. Use prose plugin for content per constitution Principle XII.

- [X] T023 [P] Update `docs/modules/reference/pages/cli.adoc`: rename all command references (~79 occurrences), update section headings (`=== env` → `=== ws`, `=== setup` → `=== build`), add new `=== config` section, update promoted commands list, update examples.
- [X] T024 [P] Update `README.md`: rename command references (~28 occurrences), update section headings ("Environment Management" -> "Workspace Management", "Unified Setup Command" -> "Build Command"), update spec table with 039 entry per constitution Principle X. Add a migration note that `cc-deck-setup.yaml` has been renamed to `cc-deck-build.yaml` for users with existing build manifests.
- [X] T025 [P] Rename `docs/modules/using/pages/setup.adoc` to `docs/modules/using/pages/build.adoc` and update all content (~24 occurrences). Update `docs/modules/using/nav.adoc` to reference new filename.
- [X] T026 [P] Update `docs/modules/running/pages/container-env.adoc` (~18 refs), `compose-env.adoc` (~35 refs), `k8s-deploy.adoc` (~25 refs), `ssh-environments.adoc` (~14 refs), `workspace-repos.adoc` (~4 refs): replace all `cc-deck env` → `cc-deck ws`, `env create` → `ws new`, `env delete` → `ws delete`, `cc-deck setup` → `cc-deck build`.
- [X] T027 [P] Update `docs/modules/ROOT/pages/index.adoc` and `docs/modules/ROOT/pages/first-session.adoc`: replace command references.
- [X] T028 [P] Update `docs/walkthroughs/024-container-env.md`, `025-compose-env.md`, `018-build-image.md`: replace all `ccd env` → `ccd ws`, `env create` → `ws new`, `env delete` → `ws delete`.
- [X] T029 Assess `docs/quickstart-k8s.md` (legacy): decide whether to update references or mark as deprecated. Update if still relevant.

**Checkpoint**: All documentation reflects new command structure.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 2 (Foundational)**: No dependencies, start immediately. BLOCKS all subsequent phases.
- **Phase 3 (US1)**: Depends on T001 (ws.go rename)
- **Phase 4 (US2)**: Depends on T003, T004 (build.go rename, internal/build rename). Can run parallel with Phase 3.
- **Phase 5 (US3)**: Depends on T003 (build.go, since config.go needs to know what's NOT in config). Can run parallel with Phases 3-4.
- **Phase 6 (US4+US5)**: Depends on T001, T002, T003, T009 (all command files must exist). Run after Phases 3-5.
- **Phase 7 (Tests)**: Depends on Phase 6 completion (all source changes done). All T013-T021 run in parallel.
- **Phase 8 (Docs)**: Can start after Phase 6. All T023-T029 run in parallel. Can also run parallel with Phase 7.

### User Story Dependencies

- **US1 (P1)**: Core rename. No dependency on other stories.
- **US2 (P2)**: Independent of US1. Can run in parallel.
- **US3 (P2)**: Independent of US1/US2. Can run in parallel.
- **US4 (P3)**: Depends on US1 (needs NewExecCmd from ws_promote.go) and US3 (needs NewConfigCmd for main.go registration).
- **US5 (P3)**: Independent but naturally combined with US4 (both modify main.go registration).

### Parallel Opportunities

```
Phase 2: T001, T002, T003, T004 must be sequential (shared imports)

Phase 3+4+5 (after Phase 2):
  T005, T006     (US1, ws.go)
  T007, T008     (US2, build.go)  ← parallel with US1
  T009, T010     (US3, config.go) ← parallel with US1+US2

Phase 7 (after Phase 6):
  T013-T021 all [P] ← all 9 test tasks run in parallel

Phase 8 (after Phase 6):
  T023-T029 all [P] ← all 7 doc tasks run in parallel
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 2: Core file renames
2. Complete Phase 3: ws command with new/kill/ls subcommands
3. **STOP and VALIDATE**: `make lint` passes, ws subcommands resolve correctly
4. The CLI works with `cc-deck ws new`, `cc-deck ws delete`, etc.

### Incremental Delivery

1. Phase 2 → Foundation ready
2. Phase 3 (US1) → ws commands work (MVP)
3. Phase 4 (US2) → build commands work
4. Phase 5 (US3) → config parent works
5. Phase 6 (US4+US5) → promoted shortcuts + hidden hook
6. Phase 7 → All tests pass
7. Phase 8 → Documentation current

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- This is a pure rename: no behavioral changes, no new features
- The internal `env` package (`internal/env/`) is NOT renamed (it is the environment abstraction, not a user-facing name)
- Commit after each phase completion
- `make test` and `make lint` are the verification gates (Phase 7)
