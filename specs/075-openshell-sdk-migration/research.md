# Research: OpenShell SDK Migration

## R1: GatewayConfig to SDK Config Mapping

**Decision**: Map `GatewayConfig` fields directly to `v1.Config` fields via a `ToSDKConfig()` method.

**Rationale**: The SDK's `Config` struct has a superset of what `GatewayConfig` provides. The mapping is straightforward:

| GatewayConfig field | SDK Config field | Notes |
|---|---|---|
| `Address` | `Address` | Direct copy |
| `TLS` (bool) | `TLS` (`*TLSConfig`) | If true, create `TLSConfig` struct |
| `TLSCertPath` | `TLS.CertFile` | Only when `TLS` is true |
| `TLSKeyPath` | `TLS.KeyFile` | Only when `TLS` is true |
| `TLSCAPath` | `TLS.CAFile` | Only when `TLS` is true |
| (none) | `Auth` | Default to `v1.NoAuth()` |
| (none) | `Timeout` | Use SDK default (no override) |
| (none) | `RetryPolicy` | Use SDK default (no override) |

**Alternatives considered**: Replacing `GatewayConfig` entirely with `v1.Config`. Rejected because `GatewayConfig` is used in workspace definitions and `ResolveGatewayConfig` handles env var fallback logic that's specific to cc-deck.

## R2: SandboxState to SandboxPhase Mapping

**Decision**: Replace `openshell.SandboxState` constants with checks on `sb.Status.Phase` using the SDK's `types.SandboxPhase` values.

**Rationale**: The SDK uses a richer status model. The mapping:

| cc-deck SandboxState | SDK SandboxPhase | Usage in code |
|---|---|---|
| `SandboxStateRunning` | `types.SandboxReady` | Status check, create polling |
| `SandboxStateCreating` | `types.SandboxProvisioning` | Create polling |
| `SandboxStateSuspended` | (no direct equivalent) | Not used in practice |
| `SandboxStateError` | `types.SandboxError` | Error reporting |
| `SandboxStateDeleted` | (handled by `IsNotFound` error) | Delete confirmation |

**Alternatives considered**: Keeping a local enum that wraps the SDK's phase. Rejected because the SDK's phase type is already an enum with string values, and adding a wrapper adds no value.

## R3: ExecResult Type Compatibility

**Decision**: Use the SDK's `v1.ExecResult` directly. It has `Stdout`, `Stderr`, and `ExitCode` fields matching the current custom type.

**Rationale**: The SDK's `ExecResult` is a type alias for `types.ExecResult` which has the same fields. The calling convention changes from `client.ExecSandbox(ctx, name, cmd)` to `client.Exec().Run(ctx, name, cmd)` but the return type shape is compatible.

**Alternatives considered**: None. The types are structurally identical.

## R4: PushBytes Migration

**Decision**: Replace `exec.CommandContext("openshell", "sandbox", "exec", ...)` in `PushBytes` with `client.Exec().Run()` passing data via the command's stdin.

**Rationale**: `PushBytes` currently spawns a CLI subprocess to pipe bytes via `cat > remotePath`. The SDK's `Exec().Run()` can execute the same `sh -c "cat > path"` command inside the sandbox. However, the SDK's `Run` captures output but does not stream stdin. The correct approach is to use `Exec().Stream()` or fall back to uploading a temp file via `Files().Upload()`.

**Alternatives considered**: Using `Files().Upload()` with a temp file. This is simpler and avoids the stdin piping question. The data is already in memory as `[]byte`, so writing to a temp file, uploading, and cleaning up is reliable. This is the recommended approach.

## R5: Credential Transport Interface

**Decision**: Change the `OpenShellClient` interface in `credential/transport.go` to accept `v1.ClientInterface` instead of the custom narrow interface.

**Rationale**: The current `OpenShellClient` interface is:
```go
type OpenShellClient interface {
    ExecSandbox(ctx context.Context, sandboxName string, cmd []string) (*openshell.ExecResult, error)
    Upload(ctx context.Context, sandboxName, localPath, remotePath string) error
}
```

This can be replaced with the SDK's broader `v1.ClientInterface`, calling `Exec().Run()` and `Files().Upload()`. Alternatively, define a narrow interface that `v1.ClientInterface` satisfies.

**Alternatives considered**: Keeping a narrow interface. This is cleaner for testing but the fake client already implements the full `ClientInterface`, so there's no benefit to narrowing.

## R6: Fake Client Test Coverage

**Decision**: Use `fake.NewClient()` for sandbox lifecycle and provider tests. Accept that `Exec()`, `Files()`, `SSH()`, and `TCP()` return `Unimplemented` in the fake.

**Rationale**: The fake client supports Create/Get/List/Delete/WaitReady for sandboxes and full CRUD for providers. This covers the core lifecycle tests. Exec and file transfer operations return `Unimplemented`, which means tests for `PushBytes`, `InjectEnvVars`, and `UploadFileCredential` cannot use the fake. These are integration-level operations that require a real gateway.

**Alternatives considered**: Writing custom fake implementations for Exec and Files. Rejected as premature; the fake package is maintained upstream and custom overrides would diverge.
