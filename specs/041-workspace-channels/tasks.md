# Tasks: Workspace Channels

**Input**: Design documents from `specs/041-workspace-channels/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md, contracts/

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

---

## Phase 1: Setup

**Purpose**: No new project setup needed. Project structure exists. This phase validates the starting point.

- [X] T001 Verify `make test` and `make lint` pass on current branch before any changes

---

## Phase 2: Foundation (Channel Interfaces, ChannelError, ExecOutput)

**Purpose**: Define the channel type system and extend the Workspace interface without changing any existing behavior. MUST complete before any user story work begins.

**CRITICAL**: No user story work can begin until this phase is complete.

- [X] T002 [P] Create channel interfaces (PipeChannel, DataChannel, GitChannel) in `cc-deck/internal/ws/channel.go`
- [X] T003 [P] Create shared git remote lifecycle helper `withTemporaryRemote()` in `cc-deck/internal/ws/channel.go`
- [X] T004 [P] Add ChannelError struct with Error(), Unwrap(), constructor in `cc-deck/internal/ws/errors.go`
- [X] T005 [P] Add unit tests for ChannelError (Error, Unwrap, errors.Is, errors.As) in `cc-deck/internal/ws/errors_test.go`
- [X] T006 Add ExecOutput method to Workspace interface in `cc-deck/internal/ws/interface.go`
- [X] T007 [P] Implement ExecOutput for ContainerWorkspace (wrapping podman.ExecOutput) in `cc-deck/internal/ws/container.go`
- [X] T008 [P] Implement ExecOutput for ComposeWorkspace (wrapping podman.ExecOutput) in `cc-deck/internal/ws/compose.go`
- [X] T009 [P] Implement ExecOutput for SSHWorkspace (wrapping client.Run) in `cc-deck/internal/ws/ssh.go`
- [X] T010 [P] Implement ExecOutput for K8sDeployWorkspace (wrapping k8sExecOutput) in `cc-deck/internal/ws/k8s_deploy.go`
- [X] T011 [P] Implement ExecOutput for LocalWorkspace (return ErrNotSupported) in `cc-deck/internal/ws/local.go`
- [X] T012 Add PipeChannel, DataChannel, GitChannel accessor signatures to Workspace interface in `cc-deck/internal/ws/interface.go`
- [X] T013 [P] Add stub channel accessors (return ErrNotSupported) to all five workspace types in `cc-deck/internal/ws/container.go`, `compose.go`, `ssh.go`, `k8s_deploy.go`, `local.go`
- [X] T014 Verify `make test` and `make lint` pass with all foundation changes

**Checkpoint**: Foundation ready. Channel interfaces defined, ChannelError works, ExecOutput available, all stubs compile. No behavior changes yet.

---

## Phase 3: User Story 1 - Send Text Commands to Remote Workspace (Priority: P1)

**Goal**: Implement PipeChannel for all workspace types so local tools can send text payloads to named zellij pipes in remote workspaces.

**Independent Test**: Send a text payload via PipeChannel to a running workspace and verify the plugin receives it via its pipe handler.

### Implementation for User Story 1

- [X] T015 [P] [US1] Implement localPipeChannel (direct zellij pipe subprocess, matching hook.go pattern) in `cc-deck/internal/ws/channel_pipe.go`
- [X] T016 [P] [US1] Implement execPipeChannel (shared for all remote workspace types, using Exec to run zellij pipe) in `cc-deck/internal/ws/channel_pipe.go`
- [X] T017 [US1] Wire PipeChannel() accessor in LocalWorkspace to return localPipeChannel in `cc-deck/internal/ws/local.go`
- [X] T018 [P] [US1] Wire PipeChannel() accessor in ContainerWorkspace to return execPipeChannel in `cc-deck/internal/ws/container.go`
- [X] T019 [P] [US1] Wire PipeChannel() accessor in ComposeWorkspace to return execPipeChannel in `cc-deck/internal/ws/compose.go`
- [X] T020 [P] [US1] Wire PipeChannel() accessor in SSHWorkspace to return execPipeChannel in `cc-deck/internal/ws/ssh.go`
- [X] T021 [P] [US1] Wire PipeChannel() accessor in K8sDeployWorkspace to return execPipeChannel in `cc-deck/internal/ws/k8s_deploy.go`
- [X] T022 [US1] Add unit tests for localPipeChannel and execPipeChannel in `cc-deck/internal/ws/channel_pipe_test.go`
- [X] T023 [US1] Verify `make test` and `make lint` pass

**Checkpoint**: PipeChannel works for all workspace types. Text commands can be sent to remote zellij pipes.

---

## Phase 4: User Story 2 - Transfer Files to and from Remote Workspace (Priority: P2)

**Goal**: Implement DataChannel for all workspace types and refactor existing Push/Pull to delegate, including new local workspace Push/Pull support.

**Independent Test**: Push a file to a remote workspace path and pull it back, verifying content matches. Test local workspace Push/Pull (new functionality).

### Implementation for User Story 2

- [X] T024 [P] [US2] Implement localDataChannel (filesystem copy, os.CopyFile/io.Copy) in `cc-deck/internal/ws/channel_data.go`
- [X] T025 [P] [US2] Implement podmanDataChannel (wrapping podman.Cp, parameterized by container name) in `cc-deck/internal/ws/channel_data.go`
- [X] T026 [P] [US2] Implement k8sDataChannel (wrapping tar-over-exec from k8s_sync.go, parameterized by ns/pod/kubeconfig) in `cc-deck/internal/ws/channel_data.go`
- [X] T027 [P] [US2] Implement sshDataChannel (wrapping client.Rsync, parameterized by SSH client/host) in `cc-deck/internal/ws/channel_data.go`
- [X] T028 [US2] Implement PushBytes method for all DataChannel implementations in `cc-deck/internal/ws/channel_data.go`
- [X] T029 [US2] Wire DataChannel() accessor in LocalWorkspace (returns localDataChannel) in `cc-deck/internal/ws/local.go`
- [X] T030 [P] [US2] Wire DataChannel() accessor in ContainerWorkspace (returns podmanDataChannel) in `cc-deck/internal/ws/container.go`
- [X] T031 [P] [US2] Wire DataChannel() accessor in ComposeWorkspace (returns podmanDataChannel) in `cc-deck/internal/ws/compose.go`
- [X] T032 [P] [US2] Wire DataChannel() accessor in SSHWorkspace (returns sshDataChannel) in `cc-deck/internal/ws/ssh.go`
- [X] T033 [P] [US2] Wire DataChannel() accessor in K8sDeployWorkspace (returns k8sDataChannel) in `cc-deck/internal/ws/k8s_deploy.go`
- [X] T034 [US2] Refactor ContainerWorkspace.Push() and Pull() to delegate to DataChannel in `cc-deck/internal/ws/container.go`
- [X] T035 [US2] Refactor ComposeWorkspace.Push() and Pull() to delegate to DataChannel in `cc-deck/internal/ws/compose.go`
- [X] T036 [US2] Refactor SSHWorkspace.Push() and Pull() to delegate to DataChannel in `cc-deck/internal/ws/ssh.go`
- [X] T037 [US2] Refactor K8sDeployWorkspace.Push() and Pull() to delegate to DataChannel in `cc-deck/internal/ws/k8s_deploy.go`
- [X] T038 [US2] Replace LocalWorkspace.Push() and Pull() stubs with DataChannel delegation (new functionality) in `cc-deck/internal/ws/local.go`
- [X] T039 [US2] Add unit tests for all DataChannel implementations in `cc-deck/internal/ws/channel_data_test.go`
- [X] T040 [US2] Verify `make test` and `make lint` pass. Verify existing ws push/ws pull behavior unchanged for remote types.

**Checkpoint**: DataChannel works for all workspace types. Existing ws push/pull commands produce identical behavior. Local workspaces gain new Push/Pull capability.

---

## Phase 5: User Story 3 - Synchronize Git Commits with Remote Workspace (Priority: P3)

**Goal**: Implement GitChannel and refactor Harvest to delegate, including new container/compose Harvest support.

**Independent Test**: Push a commit to a remote workspace via GitChannel and fetch it back, verifying commit history matches. Test container/compose Harvest (new functionality).

### Implementation for User Story 3

- [X] T041 [P] [US3] Extract gitExec helper from k8s_sync.go into `cc-deck/internal/ws/channel_git.go`
- [X] T042 [US3] Implement podmanGitChannel (ext::podman exec URL construction, new functionality) in `cc-deck/internal/ws/channel_git.go`
- [X] T043 [P] [US3] Implement k8sGitChannel (ext::kubectl exec URL, extracted from k8s_sync.go) in `cc-deck/internal/ws/channel_git.go`
- [X] T044 [P] [US3] Implement sshGitChannel (ssh:// URL, extracted from ssh.go Harvest) in `cc-deck/internal/ws/channel_git.go`
- [X] T045 [US3] Wire GitChannel() accessor in ContainerWorkspace (returns podmanGitChannel) in `cc-deck/internal/ws/container.go`
- [X] T046 [P] [US3] Wire GitChannel() accessor in ComposeWorkspace (returns podmanGitChannel) in `cc-deck/internal/ws/compose.go`
- [X] T047 [P] [US3] Wire GitChannel() accessor in SSHWorkspace (returns sshGitChannel) in `cc-deck/internal/ws/ssh.go`
- [X] T048 [P] [US3] Wire GitChannel() accessor in K8sDeployWorkspace (returns k8sGitChannel) in `cc-deck/internal/ws/k8s_deploy.go`
- [X] T049 [US3] Wire GitChannel() accessor in LocalWorkspace (return ErrNotSupported) in `cc-deck/internal/ws/local.go`
- [X] T050 [US3] Refactor SSHWorkspace.Harvest() to delegate to GitChannel in `cc-deck/internal/ws/ssh.go`
- [X] T051 [US3] Refactor K8sDeployWorkspace.Harvest() to delegate to GitChannel in `cc-deck/internal/ws/k8s_deploy.go`
- [X] T052 [US3] Replace ContainerWorkspace.Harvest() stub with GitChannel delegation (new) in `cc-deck/internal/ws/container.go`
- [X] T053 [US3] Replace ComposeWorkspace.Harvest() stub with GitChannel delegation (new) in `cc-deck/internal/ws/compose.go`
- [X] T054 [US3] Migrate k8sGitPush (UseGit=true path in k8sPush) to k8sGitChannel.Push in `cc-deck/internal/ws/channel_git.go` and `cc-deck/internal/ws/k8s_deploy.go`
- [X] T055 [US3] Clean up k8s_sync.go: remove migrated git code (k8sHarvest, k8sGitPush, gitExec), keep k8sPush/k8sPull if still used in `cc-deck/internal/ws/k8s_sync.go`
- [X] T056 [US3] Add unit tests for git remote lifecycle helper and GitChannel implementations in `cc-deck/internal/ws/channel_git_test.go`
- [X] T057 [US3] Verify `make test` and `make lint` pass. Verify existing ws harvest behavior unchanged for SSH and K8s.

**Checkpoint**: GitChannel works for all applicable workspace types. Existing ws harvest produces identical behavior. Container/compose workspaces gain new Harvest capability.

---

## Phase 6: Polish and Cross-Cutting Concerns

**Purpose**: CLI error display, harvest CLI flags, documentation updates.

- [ ] T058 Add ChannelError-aware error formatting in `cc-deck/cmd/cc-deck/main.go` (verbose shows full chain, default shows summary)
- [ ] T059 Expose --branch and --create-pr flags on ws harvest CLI command in `cc-deck/internal/cmd/ws.go`
- [ ] T060 Update ws push/ws pull CLI help text to mention local workspace support in `cc-deck/internal/cmd/ws.go`
- [ ] T061 Update README.md: add spec 041 to Feature Specifications table, document channel architecture
- [ ] T062 [P] Update CLI reference in `docs/modules/reference/pages/cli.adoc`: add harvest flags, note local push/pull support
- [ ] T063 [P] Update architecture documentation with channel abstraction description
- [ ] T064 Run `make test` and `make lint` final verification
- [ ] T065 Run quickstart.md validation (verify consumer code patterns work)

---

## Dependencies and Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies, start immediately
- **Foundation (Phase 2)**: Depends on Phase 1, BLOCKS all user stories
- **US1 PipeChannel (Phase 3)**: Depends on Foundation only
- **US2 DataChannel (Phase 4)**: Depends on Foundation only
- **US3 GitChannel (Phase 5)**: Depends on Foundation only
- **Polish (Phase 6)**: Depends on all user stories being complete

### User Story Dependencies

- **US1 (PipeChannel)**: Independent after Foundation. No dependency on US2 or US3.
- **US2 (DataChannel)**: Independent after Foundation. No dependency on US1 or US3.
- **US3 (GitChannel)**: Independent after Foundation. Uses shared `withTemporaryRemote` from Foundation. No dependency on US1 or US2.

### Within Each User Story

- Implementations before wiring (channel struct before accessor methods)
- Wiring before refactoring (accessor returns channel before Push/Pull/Harvest delegates)
- Refactoring before cleanup (delegate to channel before removing old code)
- Tests after implementation

### Parallel Opportunities

- All Foundation T002-T005 run in parallel (different files/concerns)
- All ExecOutput implementations T007-T011 run in parallel (different workspace files)
- All PipeChannel wiring T018-T021 run in parallel (different workspace files)
- All DataChannel implementations T024-T027 run in parallel (transport types)
- All DataChannel wiring T030-T033 run in parallel (different workspace files)
- All GitChannel implementations T042-T044 run in parallel (transport types)
- All GitChannel wiring T046-T048 run in parallel (different workspace files)
- Documentation tasks T061-T063 run in parallel

---

## Parallel Example: User Story 2 (DataChannel)

```
# Launch all DataChannel transport implementations in parallel:
Task: T024 "Implement localDataChannel in channel_data.go"
Task: T025 "Implement podmanDataChannel in channel_data.go"
Task: T026 "Implement k8sDataChannel in channel_data.go"
Task: T027 "Implement sshDataChannel in channel_data.go"

# After implementations complete, wire accessors in parallel:
Task: T030 "Wire DataChannel() in ContainerWorkspace"
Task: T031 "Wire DataChannel() in ComposeWorkspace"
Task: T032 "Wire DataChannel() in SSHWorkspace"
Task: T033 "Wire DataChannel() in K8sDeployWorkspace"
```

---

## Implementation Strategy

### MVP First (User Story 1: PipeChannel)

1. Complete Phase 1: Setup (verify baseline)
2. Complete Phase 2: Foundation (interfaces, ChannelError, ExecOutput)
3. Complete Phase 3: US1 PipeChannel
4. **STOP and VALIDATE**: Send text via PipeChannel to a running workspace
5. Delivers immediate value for voice relay and remote plugin control

### Incremental Delivery

1. Foundation -> ready for any user story
2. Add US1 PipeChannel -> text relay works -> validate
3. Add US2 DataChannel -> Push/Pull consolidated, local push/pull works -> validate
4. Add US3 GitChannel -> Harvest consolidated, container/compose harvest works -> validate
5. Polish -> CLI improvements, documentation
6. Each story adds value without breaking previous stories

### Recommended Implementation Order

While the spec prioritizes PipeChannel (P1), the plan recommends DataChannel (P2) first because it refactors existing code with observable behavior to validate against. Either order works since user stories are independent after Foundation.

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story
- Each user story is independently completable and testable after Foundation
- Commit after each task or logical group
- Use `make test` and `make lint` at each checkpoint (never `go build` or `cargo build` directly)
- Channel implementations live in `cc-deck/internal/ws/` to access unexported workspace fields
- Transport grouping (podman, k8s, ssh, local) is within channel files, not separate files per workspace
