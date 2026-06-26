# Tasks: OpenShell gRPC Client

**Input**: Design documents from `specs/074-openshell-grpc-client/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)

---

## Phase 1: Setup (Proto Infrastructure)

**Purpose**: Vendor proto files, set up codegen, add dependencies

- [ ] T001 Vendor proto files (`openshell.proto`, `datamodel.proto`, `sandbox.proto`) from OpenShell v0.0.46 release into `cc-deck/internal/openshell/proto/`
- [ ] T002 Add `proto` target to Makefile for Go code generation via protoc with go and go-grpc plugins
- [ ] T003 Run proto codegen and verify generated Go files compile (`openshell.pb.go`, `openshell_grpc.pb.go`, `datamodel.pb.go`, `sandbox.pb.go`)
- [ ] T004 Add `google.golang.org/grpc` and `golang.org/x/crypto` as direct dependencies in `cc-deck/go.mod`

**Checkpoint**: Proto infrastructure ready, generated code compiles

---

## Phase 2: Foundational (Connection + Legacy Split)

**Purpose**: gRPC connection setup and legacy client extraction. MUST complete before user stories.

- [ ] T005 Create `cc-deck/internal/openshell/conn.go` with gRPC connection factory: mTLS cert discovery from standard paths, insecure fallback for localhost, dial options
- [ ] T006 Move current `cliClient` implementation from `cc-deck/internal/openshell/client.go` to `cc-deck/internal/openshell/client_legacy.go` with `//go:build cli_legacy` build tag
- [ ] T007 Create `cc-deck/internal/openshell/client_grpc.go` with `grpcClient` struct implementing `Client` interface, `NewClient` factory using `conn.go` for connection setup
- [ ] T008 Add `//go:build !cli_legacy` tag to `client_grpc.go` so it is the default and `client_legacy.go` is opt-in
- [ ] T009 Run `make test` to verify the build compiles with the new grpcClient stub (methods can return `unimplemented` errors initially)

**Checkpoint**: Build compiles with grpcClient as default, cliClient available via `-tags cli_legacy`

---

## Phase 3: User Story 1 - Sandbox Lifecycle (Priority: P1) 🎯 MVP

**Goal**: Workspace creation works through gRPC without the CLI binary.

- [ ] T010 [US1] Implement `grpcClient.CreateSandbox` in `cc-deck/internal/openshell/client_grpc.go`: build `CreateSandboxRequest` from image, command, policy, providers; call RPC; extract sandbox name from response
- [ ] T011 [US1] Implement `grpcClient.GetSandbox` in `cc-deck/internal/openshell/client_grpc.go`: call `GetSandbox` RPC, map proto sandbox state to `SandboxInfo` struct
- [ ] T012 [US1] Implement `grpcClient.DeleteSandbox` in `cc-deck/internal/openshell/client_grpc.go`: call `DeleteSandbox` RPC
- [ ] T013 [US1] Implement `grpcClient.ExecSandbox` in `cc-deck/internal/openshell/client_grpc.go`: call `ExecSandbox` server-streaming RPC, collect stdout/stderr/exit code into `ExecResult`
- [ ] T014 [US1] Implement `grpcClient.ExecSandboxStream` in `cc-deck/internal/openshell/client_grpc.go`: call `ExecSandbox` RPC, stream output chunks directly to os.Stdout/os.Stderr
- [ ] T015 [US1] Add unit tests for sandbox lifecycle methods in `cc-deck/internal/openshell/client_grpc_test.go` using a mock gRPC server
- [ ] T016 [US1] Run `make test` to verify sandbox lifecycle works end-to-end

**Checkpoint**: `cc-deck ws new --type openshell` creates sandboxes via gRPC (providers and file transfer still stubbed)

---

## Phase 4: User Story 2 - Provider Management (Priority: P1)

**Goal**: Provider CRUD uses structured proto types with separate credentials and config maps.

- [ ] T017 [US2] Implement `grpcClient.CreateProvider` in `cc-deck/internal/openshell/client_grpc.go`: build `Provider` proto with separate `credentials` and `config` maps based on provider type; call `CreateProvider` RPC
- [ ] T018 [US2] Implement `grpcClient.UpdateProvider` in `cc-deck/internal/openshell/client_grpc.go`: build `Provider` proto, call `UpdateProvider` RPC (same message format as create, no flag asymmetry)
- [ ] T019 [US2] Implement `grpcClient.DeleteProvider` in `cc-deck/internal/openshell/client_grpc.go`: call `DeleteProvider` RPC
- [ ] T020 [US2] Implement `grpcClient.EnsureProvider` in `cc-deck/internal/openshell/client_grpc.go`: call CreateProvider, catch AlreadyExists status, call UpdateProvider
- [ ] T021 [US2] Add unit tests for provider methods in `cc-deck/internal/openshell/client_grpc_test.go`, including google-cloud provider with config map
- [ ] T022 [US2] Run `make test` to verify provider management works

**Checkpoint**: All provider types (claude, github, google-cloud, generic, etc.) create/update via gRPC

---

## Phase 5: User Story 3 - Streaming Exec (Priority: P2)

**Goal**: Command execution with separated stdout/stderr streams.

- [ ] T023 [US3] Implement `grpcClient.AttachExec` in `cc-deck/internal/openshell/client_grpc.go`: call `ExecSandboxInteractive` bidirectional streaming RPC with PTY support, pipe to terminal
- [ ] T024 [US3] Add test for streaming exec separation (stdout vs stderr) in `cc-deck/internal/openshell/client_grpc_test.go`
- [ ] T025 [US3] Run `make test`

**Checkpoint**: Interactive attach and streaming exec work via gRPC

---

## Phase 6: User Story 4 - SSH Tunnel + File Transfer (Priority: P2)

**Goal**: Upload and download via Go-native SSH tunnel through the gateway.

- [ ] T026 [US4] Create `cc-deck/internal/openshell/tunnel.go`: implement HTTP CONNECT dialer to gateway, return raw `net.Conn` for SSH
- [ ] T027 [US4] Implement SSH session establishment in `tunnel.go`: use `golang.org/x/crypto/ssh` over the HTTP CONNECT connection, authenticate with session token from `CreateSshSession` RPC
- [ ] T028 [US4] Implement `grpcClient.Upload` in `cc-deck/internal/openshell/client_grpc.go`: call `CreateSshSession` RPC, establish SSH tunnel via `tunnel.go`, pipe tar archive to `tar xf -` on remote
- [ ] T029 [US4] Implement `grpcClient.Download` in `cc-deck/internal/openshell/client_grpc.go`: call `CreateSshSession` RPC, establish SSH tunnel, pipe `tar cf -` from remote to local extraction
- [ ] T030 [US4] Add unit tests for tunnel and file transfer in `cc-deck/internal/openshell/tunnel_test.go`
- [ ] T031 [US4] Run `make test`

**Checkpoint**: File upload/download works through Go SSH tunnel, no CLI needed

---

## Phase 7: User Story 5 - Proto Pinning + Compile Safety (Priority: P2)

**Goal**: Proto vendoring workflow with compile-time change detection.

- [ ] T032 [US5] Document proto update workflow in `cc-deck/internal/openshell/proto/README.md`: how to update protos from a new OpenShell release, regenerate, and fix compile errors
- [ ] T033 [US5] Add `proto-check` target to Makefile that verifies generated code is up-to-date (for CI)

**Checkpoint**: Developer workflow for proto updates documented and verifiable

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: Documentation, cleanup, final verification

- [ ] T034 [P] Update README.md: document that CLI is no longer a runtime dependency, update setup instructions
- [ ] T035 [P] Update `cc-deck/internal/openshell/credentials.go`: remove the `google-cloud` special-casing from `EnsureProvider` (delete+recreate workaround no longer needed with gRPC)
- [ ] T036 Run `make verify` (test + lint) to confirm everything passes

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies, start immediately
- **Foundational (Phase 2)**: Depends on Phase 1 (proto codegen)
- **User Story 1 (Phase 3)**: Depends on Phase 2 (grpcClient stub)
- **User Story 2 (Phase 4)**: Depends on Phase 2 (grpcClient stub), can run in parallel with US1
- **User Story 3 (Phase 5)**: Depends on Phase 3 (basic exec works)
- **User Story 4 (Phase 6)**: Depends on Phase 2 (grpcClient stub), independent of US1-3
- **User Story 5 (Phase 7)**: Depends on Phase 1 (proto infrastructure)
- **Polish (Phase 8)**: Depends on all user stories

### Parallel Opportunities

- T001-T004: Proto setup tasks are sequential (each depends on prior)
- T010-T014: Sandbox methods can be implemented in parallel (different methods, same file)
- T017-T020: Provider methods can be implemented in parallel
- T026-T029: Tunnel + file transfer are sequential (tunnel before upload/download)
- T034-T035: Polish tasks can run in parallel

---

## Implementation Strategy

### MVP First (User Stories 1 + 2)

1. Phase 1: Proto infrastructure
2. Phase 2: Connection + legacy split
3. Phase 3: Sandbox lifecycle (US1)
4. Phase 4: Provider management (US2)
5. **STOP and VALIDATE**: `cc-deck ws new --type openshell` works without CLI

### Incremental Delivery

1. MVP (US1 + US2): Workspace creation works
2. Add US3: Streaming exec with separated output
3. Add US4: SSH tunnel + file transfer
4. Add US5: Proto update documentation
5. Polish: README, cleanup, final lint

---

## Notes

- The `Client` interface stays unchanged, so `ws/openshell.go` needs zero modifications
- Proto codegen adds ~3 files to the repo (generated Go code), but they're deterministic from the proto source
- The SSH tunnel is the most complex new code (~200 lines in tunnel.go)
- The `google-cloud` provider workaround in `credentials.go` (delete+recreate) can be removed since gRPC `UpdateProvider` uses the same `Provider` message as `CreateProvider`
