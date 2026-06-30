# Tasks: OpenShell SDK Migration

**Input**: Design documents from `specs/075-openshell-sdk-migration/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: Setup

**Purpose**: Add SDK dependency and prepare go.mod

- [X] T001 Add `github.com/rhuss/openshell-sdk-go` to `cc-deck/go.mod` with `replace` directive pointing to local SDK path, run `go mod tidy` to update `cc-deck/go.sum`

---

## Phase 2: Foundational (Client Factory and Config Mapping)

**Purpose**: Replace the client construction layer. MUST complete before user story work.

- [X] T002 Replace `cc-deck/internal/openshell/iface.go`: Remove the custom `Client` interface. Add a `NewSDKClient(cfg GatewayConfig) (v1.ClientInterface, error)` factory function that creates an SDK client from a `GatewayConfig`
- [X] T003 Replace `cc-deck/internal/openshell/client.go`: Remove `cliClient` struct, `execCLI`, `execCLICaptureName`, `parseSandboxName`, `parseSandboxPhase`, `parseSandboxState`, `stripANSI` functions. Keep `GatewayConfig`, `ResolveGatewayConfig`, `isLocalhost`. Add `ToSDKConfig()` method on `GatewayConfig` that maps Address/TLS fields to `v1.Config` with `v1.NoAuth()` default. Remove `SandboxState`, `SandboxInfo`, `ExecResult` types (replaced by SDK types)
- [X] T004 Update `cc-deck/internal/openshell/credentials.go`: Change `InjectEnvVars` parameter type from `Client` to `v1.ClientInterface`, replace `client.ExecSandbox()` calls with `client.Exec().Run()` (note: return type changes from `(*ExecResult, error)` to `(*v1.ExecResult, error)`, update callers to use `result.Stdout` / `result.ExitCode` from the SDK type). Change `UploadFileCredential` parameter type from `Client` to `v1.ClientInterface`, replace `client.Upload()` with `client.Files().Upload()`. Keep all other public signatures unchanged
- [X] T004b Update `cc-deck/internal/credential/transport.go`: Change `OpenShellClient` interface to match SDK method signatures. Replace `ExecSandbox(ctx, sandboxID, cmd) (string, error)` with a method using `v1.ClientInterface` (either accept `v1.ClientInterface` directly or define a narrow interface it satisfies). Update `InjectOpenShell` and `injectOpenShellEnvVar` to use `client.Exec().Run()` and `client.Files().Upload()`. Update `mockOpenShellClient` in `cc-deck/internal/credential/transport_test.go` to match the new interface

**Checkpoint**: Foundation ready. `internal/openshell/` package exports `NewSDKClient` factory and `GatewayConfig`, no longer wraps CLI.

---

## Phase 3: User Story 1 - Sandbox Lifecycle Without CLI Binary (Priority: P1)

**Goal**: All workspace operations (create, attach, delete, exec, status, push, pull) work via SDK without the CLI binary.

**Independent Test**: Create, attach to, and delete an OpenShell workspace with no `openshell` CLI installed.

### Implementation for User Story 1

- [X] T005 [US1] Update `cc-deck/internal/ws/openshell.go`: Change `client` field type from `openshell.Client` to `v1.ClientInterface`. Update `ensureClient()` to call `openshell.NewSDKClient(gwCfg)`. Replace all `client.CreateSandbox()` calls with `client.Sandboxes().Create()`, `client.DeleteSandbox()` with `client.Sandboxes().Delete()`, `client.GetSandbox()` with `client.Sandboxes().Get()`. Replace `openshell.SandboxState*` comparisons with `sb.Status.Phase` checks using `types.SandboxReady`, `types.SandboxProvisioning`, `types.SandboxError`
- [X] T006 [US1] Update exec calls in `cc-deck/internal/ws/openshell.go`: Replace `client.ExecSandbox()` with `client.Exec().Run()`, `client.ExecSandboxStream()` with `client.Exec().Stream()`, `client.AttachExec()` with `client.Exec().Interactive()`. Map `ExecResult` fields from SDK type
- [X] T007 [US1] Update provider calls in `cc-deck/internal/ws/openshell.go`: Replace `client.EnsureProvider(ctx, name, providerType, fromExisting, credentials)` with `client.Providers().Ensure(ctx, &v1.Provider{...})` constructing `v1.Provider` from `ProviderConfig` fields. Replace `client.CreateProvider`/`UpdateProvider`/`DeleteProvider` calls similarly
- [X] T008 [US1] Update `cc-deck/internal/ws/channel_openshell.go`: Data channel: replace `client.Upload()` with `client.Files().Upload()`, `client.Download()` with `client.Files().Download()`. `PushBytes`: replace `exec.CommandContext("openshell", ...)` with writing data to a temp file then calling `client.Files().Upload()`, then removing the temp file. Remove the `os/exec` import

**Checkpoint**: Sandbox lifecycle works via SDK. Git channel still uses CLI (out of scope per spec).

---

## Phase 4: User Story 2 - Structured Error Handling (Priority: P2)

**Goal**: Error messages include typed error information instead of raw CLI output.

**Independent Test**: Trigger not-found, unavailable, and already-exists errors and verify structured error types.

### Implementation for User Story 2

- [X] T009 [US2] Update error handling in `cc-deck/internal/ws/openshell.go`: Replace string-based error checks (`strings.Contains(err.Error(), "not found")`) with SDK error helpers (`v1.IsNotFound(err)`, `v1.IsAlreadyExists(err)`, `v1.IsUnavailable(err)`). Update error messages to include the typed error category

**Checkpoint**: Errors from OpenShell operations are structured and inspectable.

---

## Phase 5: User Story 3 - Unit Testing with Fake Client (Priority: P3)

**Goal**: Unit tests pass without a running gateway using the SDK fake client.

**Independent Test**: Run `make test` with no gateway or CLI binary available.

### Implementation for User Story 3

- [X] T010 [P] [US3] Replace `cc-deck/internal/openshell/client_test.go`: Write tests using `fake.NewClient()` for `NewSDKClient` factory (config mapping, TLS config, NoAuth default). Test `GatewayConfig.ToSDKConfig()` mapping. Test sandbox Create/Get/Delete lifecycle via fake client
- [X] T011 [P] [US3] Add data channel validation tests in `cc-deck/internal/ws/channel_openshell_test.go`: Update existing tests to use SDK types. Test error paths (no sandbox, no path) with the migrated channel. Note: `PushBytes` and file transfer operations cannot use `fake.NewClient()` (fake returns Unimplemented for Exec/Files per R6), so these remain integration-level tests

**Checkpoint**: `make test` passes with no external dependencies for OpenShell tests.

---

## Phase 6: Polish and Cross-Cutting Concerns

**Purpose**: Documentation, cleanup, verification

- [X] T013 [P] Update `cc-deck/README.md`: Document the SDK dependency and replace directive for development setup
- [X] T014 Run `make verify` to confirm zero regressions across all tests and linting
- [X] T015 Verify no references to `os/exec` with `"openshell"` remain in `cc-deck/internal/openshell/client.go`

---

## Dependencies and Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies, start immediately
- **Foundational (Phase 2)**: Depends on Phase 1 (SDK available in go.mod)
- **US1 (Phase 3)**: Depends on Phase 2 (client factory exists)
- **US2 (Phase 4)**: Depends on Phase 3 (SDK errors available after workspace migration)
- **US3 (Phase 5)**: Depends on Phase 2 (fake client usable after factory exists)
- **Polish (Phase 6)**: Depends on all user stories complete

### User Story Dependencies

- **US1**: Depends on Foundational only. Core migration.
- **US2**: Depends on US1 (error handling uses SDK errors from migrated code).
- **US3**: T010/T011 can start after Foundational.

### Parallel Opportunities

- T010 and T011 can run in parallel (different test files)
- T013 can run in parallel with T014/T015

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Phase 1: Add SDK dependency
2. Phase 2: Replace client factory and config mapping
3. Phase 3: Migrate workspace layer to SDK calls
4. **STOP and VALIDATE**: All workspace operations work without CLI binary

### Incremental Delivery

1. Setup + Foundational: SDK wired in, factory works
2. US1: Full workspace migration, functional parity
3. US2: Structured errors, better debugging
4. US3: Fake client tests, CI-friendly
5. Polish: Docs, verification, cleanup

---

## Notes

- Git channel (`openShellGitChannel`) remains as-is. It uses `ext::openshell` CLI transport which is explicitly out of scope per spec SC-001.
- The `replace` directive path is developer-specific. CI builds need a published SDK version (follow-up work).
- Fake client supports sandbox/provider CRUD but returns Unimplemented for Exec/Files. Credential injection tests that need Exec/Files remain integration-level.
