# Tasks: SSH Remote Execution Environment

**Input**: Design documents from `/specs/033-ssh-environment/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md, contracts/ssh-environment.md

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Register SSH as a new environment type, add type definitions, and extend the factory

- [X] T001 Add `EnvironmentTypeSSH` constant and `SSHFields` struct to cc-deck/internal/env/types.go
- [X] T002 [P] Add `ErrSSHNotFound` error to cc-deck/internal/env/errors.go
- [X] T003 [P] Add SSH-specific fields (Host, Port, IdentityFile, JumpHost, SSHConfig, Workspace) to `EnvironmentDefinition` in cc-deck/internal/env/definition.go
- [X] T004 Add `SSH *SSHFields` pointer field to `EnvironmentInstance` in cc-deck/internal/env/types.go
- [X] T005 Add SSH case to `NewEnvironment()` factory in cc-deck/internal/env/factory.go

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: SSH client package that all user stories depend on

**CRITICAL**: No user story work can begin until this phase is complete

- [X] T006 Create cc-deck/internal/ssh/client.go with `Client` struct (Host, Port, IdentityFile, JumpHost, SSHConfig fields), `NewClient()` constructor, and `buildArgs()` internal helper for SSH argument construction
- [X] T007 Implement `Run(ctx, cmd) (string, error)` for non-interactive SSH command execution in cc-deck/internal/ssh/client.go
- [X] T008 Implement `RunInteractive(cmd) error` using `syscall.Exec()` for process replacement in cc-deck/internal/ssh/client.go
- [X] T009 [P] Implement `Check(ctx) error` for SSH connectivity test in cc-deck/internal/ssh/client.go
- [X] T010 [P] Implement `RemoteInfo(ctx) (os, arch string, error)` for OS/architecture detection in cc-deck/internal/ssh/client.go
- [X] T011 Unit tests for SSH client in cc-deck/internal/ssh/client_test.go

**Checkpoint**: SSH client package operational, all user stories can now proceed

---

## Phase 3: User Story 1 - Connect to a Remote Development Machine (Priority: P1) MVP

**Goal**: Users can define, create, attach to, and detach from SSH environments. Remote Zellij sessions persist after detach.

**Independent Test**: Define SSH environment in environments file, run create, attach to remote Zellij session, verify Claude Code accessible, detach, confirm remote session persists.

### Implementation for User Story 1

- [X] T012 [US1] Create cc-deck/internal/env/ssh.go with `SSHEnvironment` struct (name, store, defs fields), `Type()` and `Name()` identity methods
- [X] T013 [US1] Implement `Create()` in cc-deck/internal/env/ssh.go: name validation via `ValidateEnvName`, conflict check via `store.FindInstanceByName`, `ssh` binary lookup via `exec.LookPath`, load definition, SSH connectivity test via client.Check, record state with SSHFields
- [X] T014 [US1] Implement `Attach()` in cc-deck/internal/env/ssh.go: nested Zellij detection ($ZELLIJ env var), load definition, update LastAttached timestamp, check/create remote Zellij session (`zellij list-sessions -n` / `zellij attach --create-background cc-deck-<name> --layout cc-deck`), process replacement via syscall.Exec
- [X] T015 [US1] Implement `Delete()` in cc-deck/internal/env/ssh.go: running check (refuse unless force=true), best-effort remote session kill with force, state removal via store.RemoveInstance
- [X] T016 [US1] Implement `Start()` and `Stop()` returning `ErrNotSupported` in cc-deck/internal/env/ssh.go
- [X] T017 [US1] Add stub implementations for remaining interface methods (Status, Exec, Push, Pull, Harvest) - implemented as full versions instead of stubs
- [X] T018 [US1] Add SSH flags (--host, --port, --identity-file, --jump-host, --ssh-config, --workspace) to `createFlags` struct and flag registration in cc-deck/internal/cmd/env.go
- [X] T019 [US1] Wire SSH flags into `runEnvCreate()` to populate EnvironmentDefinition SSH fields in cc-deck/internal/cmd/env.go
- [X] T020 [US1] Unit tests for SSHEnvironment Create, Attach, Delete in cc-deck/internal/env/ssh_test.go

**Checkpoint**: Users can create, attach to, and delete SSH environments. Core "detach and walk away" workflow works.

---

## Phase 4: User Story 2 - Pre-flight Bootstrap with Tool Installation (Priority: P1)

**Goal**: During environment creation, detect missing tools on remote and offer automated installation.

**Independent Test**: Point at bare remote machine (no Zellij, no Claude Code), verify each pre-flight check detects missing tools and offers installation.

### Implementation for User Story 2

- [X] T021 [US2] Create cc-deck/internal/ssh/bootstrap.go with `PreflightCheck` interface (Name, Run, HasRemedy, Remedy, ManualInstructions methods)
- [X] T022 [US2] Implement `ConnectivityCheck` and `OSDetectionCheck` pre-flight checks in cc-deck/internal/ssh/bootstrap.go
- [X] T023 [US2] Implement `ZellijCheck` pre-flight check with installation remedy in cc-deck/internal/ssh/bootstrap.go
- [X] T024 [P] [US2] Implement `ClaudeCodeCheck` pre-flight check with installation remedy in cc-deck/internal/ssh/bootstrap.go
- [X] T025 [P] [US2] Implement `CcDeckCheck` and `PluginCheck` pre-flight checks with installation remedies in cc-deck/internal/ssh/bootstrap.go
- [X] T026 [US2] Implement `CredentialCheck` pre-flight check in cc-deck/internal/ssh/bootstrap.go
- [X] T027 [US2] Implement `RunPreflightChecks(ctx, client, stdin, stdout)` orchestrator with interactive prompts (install/skip/manual) in cc-deck/internal/ssh/bootstrap.go
- [X] T028 [US2] Replace simple connectivity test in `SSHEnvironment.Create()` with full pre-flight check orchestration in cc-deck/internal/env/ssh.go
- [X] T029 [US2] Unit tests for pre-flight checks with mock SSH responses in cc-deck/internal/ssh/bootstrap_test.go

**Checkpoint**: Create flow runs full pre-flight bootstrap with interactive tool installation offers.

---

## Phase 5: User Story 3 - Credential Forwarding and Persistence (Priority: P2)

**Goal**: Credentials are forwarded to the remote and persist across detach/reattach cycles via a credential file.

**Independent Test**: Configure different auth modes, attach, detach, reattach, open new pane, verify credentials available.

### Implementation for User Story 3

- [X] T030 [US3] Create cc-deck/internal/ssh/credentials.go with `BuildCredentialSet(def) (map[string]string, error)` resolving credentials from definition + local env (auto/api/vertex/bedrock/none modes)
- [X] T031 [US3] Implement `WriteCredentialFile(ctx, client, creds) error` writing env file on remote at ~/.config/cc-deck/credentials.env (mode 600) in cc-deck/internal/ssh/credentials.go
- [X] T032 [US3] Implement `CopyCredentialFile(ctx, client, localPath, remoteName) error` for file-based credentials (GCP JSON) in cc-deck/internal/ssh/credentials.go
- [X] T033 [US3] Integrate credential writing into `SSHEnvironment.Attach()` before session creation/attach in cc-deck/internal/env/ssh.go
- [X] T034 [US3] Unit tests for credential building and writing in cc-deck/internal/ssh/credentials_test.go

**Checkpoint**: Credentials persist across detach/reattach cycles and new panes pick them up.

---

## Phase 6: User Story 4 - Remote Status and Monitoring (Priority: P2)

**Goal**: Users can check SSH environment status (remote session running, stopped, unreachable).

**Independent Test**: Create SSH environment, attach/detach, verify status accurately reflects remote state.

### Implementation for User Story 4

- [X] T035 [US4] Replace Status stub with full implementation in cc-deck/internal/env/ssh.go: SSH query with timeout, parse `zellij list-sessions -n` output, return running/stopped/error states
- [X] T036 [US4] Add SSH reconciliation function (parallel per-host queries with timeout) following `ReconcileContainerEnvs` pattern in cc-deck/internal/env/ssh.go
- [X] T037 [US4] Wire SSH reconciliation into `runEnvList()` for parallel status display in cc-deck/internal/cmd/env.go
- [X] T038 [US4] Unit tests for Status and reconciliation in cc-deck/internal/env/ssh_test.go

**Checkpoint**: `env list` and `env status` show accurate SSH environment status with timeout handling.

---

## Phase 7: User Story 5 - Refresh Credentials Without Attaching (Priority: P2)

**Goal**: Users can push fresh credentials to the remote without attaching, keeping long-running sessions alive.

**Independent Test**: Create SSH env, attach, detach, change local creds, run refresh-creds, verify remote picks up new creds.

### Implementation for User Story 5

- [X] T039 [US5] Add `newEnvRefreshCredsCmd()` subcommand to cc-deck/internal/cmd/env.go: resolve environment, load definition, build credential set, write to remote, handle auth=none
- [X] T040 [US5] Register refresh-creds subcommand in the env command group in cc-deck/internal/cmd/env.go
- [X] T041 [US5] Unit tests for refresh-creds command in cc-deck/internal/cmd/env_test.go

**Checkpoint**: `env refresh-creds <name>` pushes fresh credentials without session disruption.

---

## Phase 8: User Story 6 - File Synchronization (Priority: P3)

**Goal**: Users can push local files to remote and pull remote files back, using rsync with scp fallback.

**Independent Test**: Push local directory, modify on remote, pull back, verify contents match.

### Implementation for User Story 6

- [X] T042 [US6] Add `Upload(ctx, localPath, remotePath) error` and `Download(ctx, remotePath, localPath) error` methods to SSH client (scp-based) in cc-deck/internal/ssh/client.go
- [X] T043 [US6] Add `Rsync(ctx, src, dst, excludes, push) error` method with scp fallback to SSH client in cc-deck/internal/ssh/client.go
- [X] T044 [US6] Replace Push stub with rsync-based implementation in cc-deck/internal/env/ssh.go: use configured workspace, respect SyncOpts exclusions
- [X] T045 [US6] Replace Pull stub with rsync-based implementation in cc-deck/internal/env/ssh.go: use configured workspace, respect SyncOpts exclusions
- [X] T046 [US6] Unit tests for Push, Pull, and rsync fallback in cc-deck/internal/env/ssh_test.go

**Checkpoint**: File sync works with rsync, falls back to scp when rsync unavailable on remote.

---

## Phase 9: User Story 7 - Remote Command Execution (Priority: P3)

**Goal**: Users can run commands on the remote in the workspace directory without full Zellij attach.

**Independent Test**: Run command via exec, verify output returned and command runs in workspace directory.

### Implementation for User Story 7

- [X] T047 [US7] Replace Exec stub with full implementation in cc-deck/internal/env/ssh.go: run command via SSH in workspace directory (`cd <workspace> && <cmd>`), return output
- [X] T048 [US7] Unit tests for Exec in cc-deck/internal/env/ssh_test.go

**Checkpoint**: `env exec <name> -- <cmd>` runs commands on the remote in the configured workspace.

---

## Phase 10: User Story 8 - Harvest Git Commits (Priority: P3)

**Goal**: Users can retrieve git commits from the remote repository to the local repo and optionally create a PR.

**Independent Test**: Have commits on remote, run harvest, verify commits appear locally.

### Implementation for User Story 8

- [X] T049 [US8] Replace Harvest stub with full implementation in cc-deck/internal/env/ssh.go: add temporary git remote (`ssh://<host>/<workspace>`), `git fetch`, remove temporary remote, optional PR creation via `gh` CLI
- [X] T050 [US8] Unit tests for Harvest in cc-deck/internal/env/ssh_test.go

**Checkpoint**: `env harvest <name>` fetches remote commits and optionally creates a PR.

---

## Phase 11: Polish & Cross-Cutting Concerns

**Purpose**: Documentation, edge cases, and final validation

- [X] T051 [P] Update README.md with SSH environment description, usage examples, and spec table entry
- [X] T052 [P] Update CLI reference (docs/modules/reference/pages/cli.adoc) with SSH create flags, refresh-creds command, SSH-specific behaviors
- [X] T053 Create Antora guide page for SSH environments (docs/modules/running/pages/ssh-environments.adoc)
- [X] T054 Edge case handling: unreachable hosts (timeout), deleted remote sessions (status reporting), invalid SSH configs (clear errors) in cc-deck/internal/env/ssh.go
- [X] T055 Run quickstart.md validation against implemented commands

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies, can start immediately
- **Foundational (Phase 2)**: Depends on Phase 1 (needs SSHFields, EnvironmentTypeSSH) - BLOCKS all user stories
- **US1 (Phase 3)**: Depends on Phase 2 (needs SSH client)
- **US2 (Phase 4)**: Depends on Phase 3 (extends Create flow from US1)
- **US3 (Phase 5)**: Depends on Phase 3 (integrates into Attach from US1)
- **US4 (Phase 6)**: Depends on Phase 2 (needs SSH client for queries)
- **US5 (Phase 7)**: Depends on Phase 5 (uses credential infrastructure from US3)
- **US6 (Phase 8)**: Depends on Phase 2 (needs SSH client, independent of other stories)
- **US7 (Phase 9)**: Depends on Phase 2 (needs SSH client, independent of other stories)
- **US8 (Phase 10)**: Depends on Phase 2 (needs SSH client, independent of other stories)
- **Polish (Phase 11)**: Depends on all user stories being complete

### User Story Dependencies

- **US1 (P1)**: Foundational only. No dependencies on other stories.
- **US2 (P1)**: Depends on US1 (extends Create). Must follow US1.
- **US3 (P2)**: Depends on US1 (extends Attach). Can parallel with US2.
- **US4 (P2)**: Foundational only. Can parallel with US1+.
- **US5 (P2)**: Depends on US3 (credential infrastructure). Must follow US3.
- **US6 (P3)**: Foundational only. Can parallel with US1+.
- **US7 (P3)**: Foundational only. Can parallel with US1+.
- **US8 (P3)**: Foundational only. Can parallel with US1+.

### Within Each User Story

- Models/types before implementations
- Core implementation before integration
- Story complete before moving to next priority

### Parallel Opportunities

- T002, T003 can run in parallel (different files)
- T009, T010 can run in parallel (independent client methods)
- T024, T025 can run in parallel (independent check implementations)
- US4, US6, US7, US8 can all run in parallel after Foundational phase
- T051, T052 can run in parallel (different doc files)

---

## Parallel Example: After Foundational Phase

```text
# These user stories can run in parallel (independent, different files):
US4: Status and Monitoring (cc-deck/internal/env/ssh.go Status method)
US6: File Sync (cc-deck/internal/ssh/client.go + env/ssh.go Push/Pull)
US7: Remote Exec (cc-deck/internal/env/ssh.go Exec method)
US8: Harvest (cc-deck/internal/env/ssh.go Harvest method)

# Note: US4/US6/US7/US8 touch the same ssh.go file but different methods,
# so they must be coordinated if truly parallel.
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (types, errors, factory)
2. Complete Phase 2: Foundational (SSH client)
3. Complete Phase 3: User Story 1 (create, attach, delete)
4. **STOP and VALIDATE**: Test create/attach/detach cycle
5. This delivers the core "detach and walk away" workflow

### Incremental Delivery

1. Setup + Foundational -> SSH client operational
2. Add US1 -> Core connect/detach cycle (MVP!)
3. Add US2 -> Pre-flight bootstrap (complete P1 stories)
4. Add US3 -> Credential forwarding
5. Add US4 -> Status monitoring
6. Add US5 -> Credential refresh (completes P2 stories)
7. Add US6/US7/US8 -> File sync, exec, harvest (P3 stories)
8. Polish -> Documentation and edge cases

### Sequential Recommended Order

US1 -> US2 -> US3 -> US4 -> US5 -> US6 -> US7 -> US8 -> Polish
