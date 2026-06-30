# Implementation Plan: OpenShell SDK Migration

**Branch**: `075-openshell-sdk-migration` | **Date**: 2026-06-30 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `specs/075-openshell-sdk-migration/spec.md`

## Summary

Replace cc-deck's OpenShell CLI wrapper (`internal/openshell/client.go`) with the OpenShell Go SDK (`github.com/rhuss/openshell-sdk-go`). The current `cliClient` shells out to the `openshell` binary for every gateway operation, parsing stdout text with ANSI stripping and process-kill hacks. The SDK talks gRPC directly, provides typed errors, and includes a fake client for testing. This migration replaces the transport layer while maintaining functional parity.

## Technical Context

**Language/Version**: Go 1.25 (from go.mod)
**Primary Dependencies**: cobra v1.10.2 (CLI), openshell-sdk-go (new, via replace directive), google.golang.org/grpc (transitive via SDK)
**Storage**: N/A (no data storage changes)
**Testing**: `make test` (Go test), `make verify` (test + lint)
**Target Platform**: Linux/macOS CLI
**Project Type**: CLI tool + WASM plugin (only CLI affected)
**Performance Goals**: Parity with CLI wrapper (gRPC should be faster since no process spawn overhead)
**Constraints**: SDK not yet published; uses `replace` directive with local path
**Scale/Scope**: ~6 files changed in `internal/openshell/` and `internal/ws/`, plus `go.mod`

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

The constitution template is unpopulated (placeholder text only). The project's real constraints are in `CLAUDE.md`:

1. **Tests required**: Unit tests using fake client (FR-008). Satisfies "every feature MUST include tests."
2. **Documentation**: README update for changed dependency. Satisfies "README.md is updated with user-facing changes."
3. **Build rules**: Use `make install`, `make test`, `make lint` only. No `go build` or `cargo build` directly.
4. **Container runtime**: podman only (not affected by this change).

All gates pass.

## Project Structure

### Documentation (this feature)

```text
specs/075-openshell-sdk-migration/
├── plan.md              # This file
├── research.md          # Phase 0: SDK mapping research
├── data-model.md        # Phase 1: type mapping
├── tasks.md             # Phase 2: task breakdown (via /speckit-tasks)
└── checklists/
    └── requirements.md  # Spec quality checklist
```

### Source Code (repository root)

```text
cc-deck/
├── go.mod                              # Add SDK dependency + replace directive
├── go.sum                              # Updated by go mod tidy
├── internal/
│   ├── openshell/
│   │   ├── iface.go                    # REPLACE: Drop Client interface, export SDK factory
│   │   ├── client.go                   # REPLACE: Remove cliClient, add NewSDKClient factory
│   │   ├── client_test.go              # REPLACE: Tests using fake client
│   │   ├── credentials.go             # KEEP: Public API unchanged
│   │   └── credentials_test.go        # KEEP: Existing tests unchanged
│   ├── ws/
│   │   ├── openshell.go               # UPDATE: Use SDK types (v1.Sandbox, v1.ExecResult)
│   │   ├── channel_openshell.go       # UPDATE: Data channel uses SDK Files/Exec
│   │   └── channel_openshell_test.go  # UPDATE: Tests with fake client
│   └── credential/
│       └── transport.go               # UPDATE: OpenShellClient interface matches SDK
└── README.md                           # UPDATE: Document SDK dependency
```

**Structure Decision**: No new directories. The existing `internal/openshell/` package is preserved as a thin adapter layer (factory + credential logic) around the SDK. Consumer files in `internal/ws/` and `internal/credential/` are updated to use SDK types.

## Global Constraints

These constraints apply to all tasks implicitly:

- **Go version**: 1.25 (from go.mod)
- **Build commands**: Use `make install`, `make test`, `make lint`, `make verify` only. Never run `go build` or `cargo build` directly
- **SDK dependency**: `github.com/rhuss/openshell-sdk-go` via `replace` directive (local path, not published)
- **Container runtime**: podman only (not affected by this change)
- **Git channel exclusion**: `openShellGitChannel` uses `ext::openshell` CLI transport and is explicitly out of scope (per spec SC-001)
- **Auth default**: `v1.NoAuth()` for all SDK client construction (per clarification session 2026-06-30)

## Implementation Strategy

### Phase 1: SDK Dependency and Client Factory

1. Add `github.com/rhuss/openshell-sdk-go` to `go.mod` with `replace` directive
2. Replace `iface.go`: Remove custom `Client` interface. Export a `NewSDKClient` factory that takes `GatewayConfig` and returns `v1.ClientInterface`
3. Replace `client.go`: Remove `cliClient`, all CLI parsing code, `execCLI`, `execCLICaptureName`, `parseSandboxName`, `parseSandboxState`, `stripANSI`. Keep `GatewayConfig`, `ResolveGatewayConfig`. Add `ToSDKConfig()` method that maps `GatewayConfig` to `v1.Config`
4. Update `client_test.go`: Tests using `fake.NewClient()` for sandbox lifecycle and config mapping

### Phase 2: Workspace Layer Migration

1. Update `ws/openshell.go`:
   - Change `client` field from `openshell.Client` to `v1.ClientInterface`
   - Replace `openshell.NewClient(gwCfg)` with `openshell.NewSDKClient(gwCfg)`
   - Replace `client.CreateSandbox(...)` with `client.Sandboxes().Create(...)`
   - Replace `client.DeleteSandbox(...)` with `client.Sandboxes().Delete(...)`
   - Replace `client.GetSandbox(...)` with `client.Sandboxes().Get(...)`
   - Replace `client.ExecSandbox(...)` with `client.Exec().Run(...)`
   - Replace `client.ExecSandboxStream(...)` with `client.Exec().Stream(...)`
   - Replace `client.AttachExec(...)` with `client.Exec().Interactive(...)`
   - Replace `openshell.SandboxState*` constants with `types.SandboxPhase*` checks via `sb.Status.Phase`
   - Replace `client.EnsureProvider(...)` with `client.Providers().Ensure(...)`
2. Update `ws/channel_openshell.go`:
   - Data channel: Replace `client.Upload/Download` with `client.Files().Upload/Download`
   - `PushBytes`: Replace `exec.CommandContext("openshell", ...)` with temp-file creation + `client.Files().Upload(ctx, sandboxID, tmpFile, remotePath)`
   - Git channel: Remains as-is (uses `ext::openshell` CLI transport, explicitly out of scope)

### Phase 3: Credential Transport Update

1. Update `credential/transport.go`:
   - Change `OpenShellClient` interface to match SDK's method signatures (or accept `v1.ClientInterface` directly)
   - Update `InjectOpenShell` to use SDK `Exec().Run()` and `Files().Upload()` instead of custom client methods
2. Update `openshell/credentials.go`:
   - `InjectEnvVars` and `UploadFileCredential` accept a client parameter; update the parameter type from the old `Client` interface to `v1.ClientInterface`
   - Internal implementation calls change from `client.ExecSandbox()` to `client.Exec().Run()` and `client.Upload()` to `client.Files().Upload()`

### Phase 4: Tests and Verification

1. Add unit tests for `NewSDKClient` factory (config mapping, TLS, NoAuth default)
2. Add unit tests for sandbox lifecycle using `fake.NewClient()` (create, get status, delete)
3. Add unit tests for credential injection using fake client
4. Verify `make verify` passes with zero regressions
5. Update README.md to document the SDK dependency

## Risk Mitigation

| Risk | Impact | Mitigation |
|------|--------|------------|
| SDK API changes before publication | Build breaks | Pin to specific commit via replace directive |
| `PushBytes` uses `exec.Command("openshell")` | Partial CLI dependency remains | Migrate to `Exec().Run()` with stdin pipe via SDK |
| Git channel requires CLI binary | Feature gap | Explicitly out of scope per spec SC-001 |
| gRPC dependency size increase | Binary bloat | Acceptable tradeoff; gRPC is already transitive via SDK |
| Fake client doesn't support Exec/Files | Test gaps | Fake supports sandbox/provider; exec/file tests use integration tests |

## Complexity Tracking

No constitution violations to justify.
