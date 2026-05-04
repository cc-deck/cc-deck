# Tasks: OpenShell Backend for cc-deck

**Input**: Design documents from `specs/049-openshell-backend/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/workspace-interface.md

**Tests**: Unit tests per phase. The spec references cc-deck's existing behavioral contract tests (SC-005) as the acceptance bar; T029 adds compile-time interface checks and Phase 9 adds unit tests for each component.

**Organization**: Tasks grouped by user story for independent implementation and testing.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Project initialization, gRPC client, and type registration

- [ ] T001 Add `WorkspaceTypeOpenShell WorkspaceType = "openshell"` to `cc-deck/internal/ws/types.go`
- [ ] T002 Add openshell case to `NewWorkspace` switch in `cc-deck/internal/ws/factory.go`, returning `&OpenShellWorkspace{name: name, store: store, defs: defs}`
- [ ] T003 Copy OpenShell proto files from the upstream openshell repository (`github.com/openshell-project/openshell/api/proto/`) to `cc-deck/internal/openshell/proto/` and run `protoc` with `protoc-gen-go` and `protoc-gen-go-grpc` to generate Go client code (`*.pb.go`, `*_grpc.pb.go`)
- [ ] T004 Create `cc-deck/internal/openshell/client.go` with `Client` struct wrapping gRPC connection: `NewClient(cfg GatewayConfig) (*Client, error)` with TLS optional (plaintext for localhost, warning for non-localhost), typed methods for `CreateSandbox`, `DeleteSandbox`, `GetSandbox`, `ExecSandbox`, `CreateSshSession`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core workspace struct, gateway config resolution, and state mapping

**CRITICAL**: No user story work can begin until this phase is complete

- [ ] T005 Create `cc-deck/internal/ws/openshell.go` with `OpenShellWorkspace` struct: fields for `name`, `store`, `defs`, `client` (*openshell.Client), `sandboxID` (string), `sshTunnel` (*sshTunnelState), lazy channel fields (`pipeOnce`/`pipeCh`, `dataOnce`/`dataCh`, `gitOnce`/`gitCh`), and `Type()`, `Name()` methods
- [ ] T006 [P] Create `GatewayConfig` struct in `cc-deck/internal/openshell/client.go` with fields: `Address` (string), `TLS` (bool), `TLSCertPath`, `TLSKeyPath`, `TLSCAPath` (string). Add `ResolveGatewayConfig(defs *DefinitionStore, name string) GatewayConfig` that checks workspace definition YAML first, then `OPENSHELL_GATEWAY_URL` env var, then defaults to `localhost:8080`
- [ ] T007 [P] Create `SandboxConfig` struct in `cc-deck/internal/ws/openshell.go` with fields: `Image` (string, default `cc-deck/openshell-sandbox:latest`), `Command` (string, default `zellij`), `Policy` (string, optional), `Provider` (string, optional). Add `resolveSandboxConfig(defs *DefinitionStore, name string) SandboxConfig`
- [ ] T008 [P] Create `sshTunnelState` struct in `cc-deck/internal/ws/openshell.go` with fields: `sessionID` (string), `localPort` (int), `pid` (int), `connected` (bool). Add `isAlive() bool` method that checks PID liveness via `os.FindProcess` + signal 0
- [ ] T009 Implement `Status(ctx context.Context) (*WorkspaceStatus, error)` in `cc-deck/internal/ws/openshell.go`: call `client.GetSandbox(sandboxID)`, map OpenShell state to InfraState (creating/error -> transient, running -> `InfraStateRunning`, suspended -> `InfraStateStopped`, error -> `InfraStateError`, deleted -> nil InfraState + remove from store), check SSH tunnel PID liveness for SessionState

**Checkpoint**: Foundation ready. OpenShellWorkspace struct compiles, gRPC client connects to gateway, state mapping works.

---

## Phase 3: User Story 1 - Create a Sandboxed Workspace (Priority: P1) MVP

**Goal**: Developer runs `cc-deck create --type openshell my-workspace` and gets a running sandbox with Zellij inside.

**Independent Test**: Create workspace, verify sandbox is running via `cc-deck status my-workspace`, verify Zellij process exists inside sandbox via `cc-deck exec my-workspace -- pgrep zellij`.

### Implementation for User Story 1

- [ ] T010 [US1] Implement `Create(ctx context.Context, opts CreateOpts) error` in `cc-deck/internal/ws/openshell.go`: resolve GatewayConfig, create openshell.Client, resolve SandboxConfig from workspace definition, call `client.CreateSandbox(image, command, policy)`, poll `client.GetSandbox` until state is "running" (timeout 60s), store sandboxID in workspace instance state via `store`
- [ ] T011 [US1] Implement `Start(ctx context.Context) error` in `cc-deck/internal/ws/openshell.go` (InfraManager): load stored sandbox config from definition, delegate to Create logic
- [ ] T012 [US1] Add `openshell` type validation to the create subcommand in `cc-deck/internal/cmd/ws.go`: validate that workspace definition includes gateway config (or env var is set), validate sandbox image is specified or use default
- [ ] T013 [US1] Create default network policy YAML at `cc-deck/internal/openshell/default-policy.yaml` with allowed domains: `api.anthropic.com`, `api.openai.com`, `registry.npmjs.org`, `pypi.org`, `files.pythonhosted.org`, `proxy.golang.org`, `crates.io`, `github.com`, `gitlab.com`. Default deny for everything else.
- [ ] T014 [US1] Create `cc-deck/build/Dockerfile.openshell` with sandbox image: base from debian:bookworm-slim, install Zellij (latest stable), Claude Code (via npm), git, Node.js runtime, create /sandbox working directory, set entrypoint compatible with OpenShell supervisor injection

**Checkpoint**: `cc-deck create --type openshell my-workspace` provisions a sandbox. Status shows "running". Zellij is active inside.

---

## Phase 4: User Story 2 - Attach to a Sandboxed Workspace (Priority: P1)

**Goal**: Developer runs `cc-deck attach my-workspace` and lands inside the Zellij session running in the sandbox. Reattach works after tunnel drops.

**Independent Test**: Create workspace, attach, verify Zellij UI appears, detach (ctrl-b d), reattach, verify same session.

### Implementation for User Story 2

- [ ] T015 [US2] Implement `Attach(ctx context.Context) error` in `cc-deck/internal/ws/openshell.go`: check `sshTunnel.isAlive()` and return "workspace already attached" error (FR-013) if active tunnel exists, clear stale tunnel if PID dead, call `client.CreateSshSession(sandboxID)` to establish SSH tunnel, run `zellij attach --create` over the tunnel, store tunnel state
- [ ] T016 [US2] Implement `KillSession(ctx context.Context) error` in `cc-deck/internal/ws/openshell.go`: call `client.ExecSandbox(sandboxID, ["zellij", "kill-all-sessions"])`, clear sshTunnel state, do NOT destroy sandbox
- [ ] T017 [US2] Add attach flow handling for openshell type in `cc-deck/internal/cmd/ws.go` (attach subcommand): detect workspace type, call workspace.Attach, handle "workspace already attached" error with user-friendly message

**Checkpoint**: `cc-deck attach my-workspace` opens Zellij. Detach and reattach preserves session. Second concurrent attach fails cleanly.

---

## Phase 5: User Story 3 - Sync Files Into and Out of the Sandbox (Priority: P2)

**Goal**: Developer pushes local files into the sandbox and pulls results back. Git harvest extracts commits.

**Independent Test**: Push a project, verify files exist inside sandbox via exec, pull back, verify contents match.

### Implementation for User Story 3

- [ ] T018 [P] [US3] Create `cc-deck/internal/ws/channel_openshell.go` with `OpenShellDataChannel` struct implementing `DataChannel` interface: `Push(ctx, opts SyncOpts) error` pipes tar archive over SSH tunnel into sandbox, `Pull(ctx, opts SyncOpts) error` pipes tar archive back, `PushBytes(ctx, data []byte, remotePath string) error` writes bytes via SSH. Respect exclusion lists from SyncOpts.
- [ ] T019 [P] [US3] Create `OpenShellGitChannel` struct in `cc-deck/internal/ws/channel_openshell.go` implementing `GitChannel` interface: `Fetch(ctx, opts HarvestOpts) error` creates temporary git remote with `ext::ssh -W %h:%p gateway` transport, fetches commits from sandbox repo. `Push(ctx) error` reverse direction.
- [ ] T020 [US3] Create `OpenShellPipeChannel` struct in `cc-deck/internal/ws/channel_openshell.go` implementing `PipeChannel` interface: `Send(ctx, pipeName, payload string) error` calls `client.ExecSandbox(sandboxID, ["zellij", "pipe", "--name", pipeName, payload])`. `SendReceive` not supported initially (return ErrNotSupported).
- [ ] T021 [US3] Wire channel lazy initialization in `cc-deck/internal/ws/openshell.go`: implement `PipeChannel(ctx)`, `DataChannel(ctx)`, `GitChannel(ctx)` methods using sync.Once pattern, `Push(ctx, opts)`, `Pull(ctx, opts)`, `Harvest(ctx, opts)` delegates to channels

**Checkpoint**: `cc-deck push/pull/harvest my-workspace` works. Files transfer correctly with exclusions.

---

## Phase 6: User Story 4 - Delete a Workspace and Clean Up (Priority: P2)

**Goal**: Developer deletes a workspace. All resources cleaned up.

**Independent Test**: Create workspace, do some work, delete, verify no orphaned containers or state.

### Implementation for User Story 4

- [ ] T022 [US4] Implement `Delete(ctx context.Context, force bool) error` in `cc-deck/internal/ws/openshell.go`: call KillSession (best-effort), call `client.DeleteSandbox(sandboxID)`, remove workspace from state store. If force=true and gateway unreachable: remove local state, log warning about potentially orphaned sandbox.
- [ ] T023 [US4] Implement `Stop(ctx context.Context) error` in `cc-deck/internal/ws/openshell.go` (InfraManager): call KillSession, call DeleteSandbox, update InfraState

**Checkpoint**: `cc-deck delete my-workspace` removes sandbox and state. Force delete works when gateway is down.

---

## Phase 7: User Story 5 - Execute Commands Inside the Sandbox (Priority: P3)

**Goal**: Developer runs one-off commands inside the sandbox without attaching.

**Independent Test**: Create workspace, run `cc-deck exec my-workspace -- echo hello`, verify output.

### Implementation for User Story 5

- [ ] T024 [US5] Implement `Exec(ctx context.Context, cmd []string) error` in `cc-deck/internal/ws/openshell.go`: call `client.ExecSandbox(sandboxID, cmd)`, stream stdout/stderr to terminal, return exit code error if non-zero
- [ ] T025 [US5] Implement `ExecOutput(ctx context.Context, cmd []string) (string, error)` in `cc-deck/internal/ws/openshell.go`: same as Exec but capture stdout as string return, stderr still to terminal

**Checkpoint**: `cc-deck exec my-workspace -- ls /sandbox` shows sandbox filesystem.

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: State reconciliation, error wrapping, and workspace definition schema

- [ ] T026 [P] Implement state reconciliation in `cc-deck/internal/ws/openshell.go`: on any operation, if stored sandboxID exists but GetSandbox returns "not found", clear local state and return appropriate error. Handle gateway reconnection after restart.
- [ ] T027 [P] Wrap all gRPC errors in cc-deck's `ChannelError` type in `cc-deck/internal/ws/channel_openshell.go`: include channel type, operation name, workspace name, and original gRPC status code
- [ ] T028 [P] Add workspace definition YAML schema for openshell type in cc-deck documentation: `gateway:` section (address, tls, certs), `sandbox:` section (image, command, policy, provider), example workspace definition file
- [ ] T029 Add compile-time interface check in `cc-deck/internal/ws/interface_test.go`: `var _ Workspace = (*OpenShellWorkspace)(nil)` and `var _ InfraManager = (*OpenShellWorkspace)(nil)`
- [ ] T036 [P] Add debug-level logging for all gRPC call outcomes in `cc-deck/internal/openshell/client.go` (FR-014): log connect, CreateSandbox, DeleteSandbox, GetSandbox, ExecSandbox, CreateSshSession results with operation name, sandbox ID, and duration using cc-deck's existing log output

---

## Phase 9: Tests

**Purpose**: Unit tests for all OpenShell backend components

- [ ] T030 [P] Create `cc-deck/internal/openshell/client_test.go`: test `NewClient` with valid/invalid GatewayConfig, test `ResolveGatewayConfig` resolution order (definition YAML > env var > default), test TLS warning for non-localhost plaintext
- [ ] T031 [P] Create `cc-deck/internal/ws/openshell_test.go`: test `OpenShellWorkspace.Status` state mapping (all five OpenShell states to cc-deck InfraState), test `sshTunnelState.isAlive` with live/dead PIDs, test `resolveSandboxConfig` defaults and overrides
- [ ] T032 [P] Create `cc-deck/internal/ws/channel_openshell_test.go`: test `OpenShellDataChannel.Push/Pull` with mock SSH, test `OpenShellGitChannel.Fetch` transport construction, test `OpenShellPipeChannel.Send` command assembly, test `SendReceive` returns ErrNotSupported
- [ ] T033 Test `Create` flow in `cc-deck/internal/ws/openshell_test.go`: test sandbox creation with mock gRPC client, test polling timeout, test error propagation from gateway
- [ ] T034 Test `Attach` flow in `cc-deck/internal/ws/openshell_test.go`: test single-attach enforcement (second attach fails), test stale tunnel cleanup (dead PID), test tunnel establishment with mock CreateSshSession
- [ ] T035 Test `Delete` flow in `cc-deck/internal/ws/openshell_test.go`: test normal delete (KillSession + DeleteSandbox + state removal), test force delete with unreachable gateway (local state removed, warning logged)

**Checkpoint**: `make test` passes. All OpenShell backend code has unit test coverage.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies, start immediately
- **Foundational (Phase 2)**: Depends on Phase 1 (types and factory registered, gRPC client built)
- **US1 Create (Phase 3)**: Depends on Phase 2 (workspace struct, config resolution, state mapping)
- **US2 Attach (Phase 4)**: Depends on Phase 3 (need a created workspace to attach to)
- **US3 File Sync (Phase 5)**: Depends on Phase 3 (need a created workspace). Can run in parallel with Phase 4.
- **US4 Delete (Phase 6)**: Depends on Phase 3 (need a created workspace). Can run in parallel with Phases 4-5.
- **US5 Exec (Phase 7)**: Depends on Phase 3 (need a created workspace). Can run in parallel with Phases 4-6.
- **Polish (Phase 8)**: Depends on all user story phases

### User Story Dependencies

- **US1 (P1)**: Foundation only. No other story dependencies. MVP.
- **US2 (P1)**: Depends on US1 (needs created workspace). Core interaction.
- **US3 (P2)**: Depends on US1 only. Independent of US2.
- **US4 (P2)**: Depends on US1 only. Independent of US2 and US3.
- **US5 (P3)**: Depends on US1 only. Independent of US2, US3, US4.

### Parallel Opportunities

- T006, T007, T008 can run in parallel (different structs, no dependencies)
- T018, T019, T020 can run in parallel (different channel implementations)
- T026, T027, T028 can run in parallel (different concerns)
- US3, US4, US5 can all run in parallel after US1 completes

---

## Parallel Example: User Story 3

```
# Launch all channel implementations together:
Task: "Create OpenShellDataChannel in cc-deck/internal/ws/channel_openshell.go"
Task: "Create OpenShellGitChannel in cc-deck/internal/ws/channel_openshell.go"
Task: "Create OpenShellPipeChannel in cc-deck/internal/ws/channel_openshell.go"
```

Note: T018 and T019 write to the same file but different structs/methods. If using parallel agents, split the file or serialize these two tasks.

---

## Implementation Strategy

### MVP First (User Story 1 + User Story 2)

1. Complete Phase 1: Setup (types, factory, proto codegen)
2. Complete Phase 2: Foundational (struct, config, state mapping)
3. Complete Phase 3: US1 Create (sandbox provisioning)
4. Complete Phase 4: US2 Attach (SSH tunnel + Zellij)
5. **STOP and VALIDATE**: Create a workspace, attach, use Claude Code in the sandbox
6. Demo/iterate

### Incremental Delivery

1. Setup + Foundation -> types registered, gRPC client works
2. US1 Create -> sandbox provisioning works (MVP core)
3. US2 Attach -> interactive sessions work (MVP complete)
4. US3 File Sync -> push/pull/harvest works
5. US4 Delete -> clean lifecycle
6. US5 Exec -> scripting support
7. Polish -> resilience, error handling, docs

---

## Notes

- [P] tasks = different files or different structs in same file, no dependencies
- [Story] label maps task to specific user story for traceability
- All tasks target the cc-deck CLI at `cc-deck/` within this repository
- The OpenShell gateway must be running locally with Podman driver for integration testing
- Commit after each task or logical group
