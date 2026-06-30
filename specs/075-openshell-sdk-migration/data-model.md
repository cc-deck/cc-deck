# Data Model: OpenShell SDK Migration

## Type Mapping

This migration replaces custom cc-deck types with SDK types. No new entities are introduced.

### Removed Types (from `internal/openshell/`)

| Type | File | Replacement |
|---|---|---|
| `Client` interface | `iface.go` | `v1.ClientInterface` (SDK) |
| `cliClient` struct | `client.go` | Removed (SDK client is the implementation) |
| `SandboxState` enum | `client.go` | `types.SandboxPhase` (SDK) |
| `SandboxInfo` struct | `client.go` | `v1.Sandbox` (SDK) |
| `ExecResult` struct | `client.go` | `v1.ExecResult` (SDK) |

### Preserved Types (from `internal/openshell/`)

| Type | File | Notes |
|---|---|---|
| `GatewayConfig` struct | `client.go` | Kept. Gains `ToSDKConfig()` method. |
| `KnownProviderProfile` struct | `credentials.go` | Unchanged |
| `ProviderEndpoint` struct | `credentials.go` | Unchanged |
| `ProviderConfig` struct | `credentials.go` | Unchanged |
| `CredentialInput` struct | `credentials.go` | Unchanged |
| `DetectedCredential` struct | `credentials.go` | Unchanged |

### SDK Types Used (from `openshell-sdk-go`)

| SDK Type | Used By | Purpose |
|---|---|---|
| `v1.ClientInterface` | `ws/openshell.go` | Top-level SDK client |
| `v1.Config` | `openshell/client.go` | Client construction config |
| `v1.TLSConfig` | `openshell/client.go` | TLS parameters |
| `v1.Sandbox` | `ws/openshell.go` | Sandbox instance (Name, Spec, Status) |
| `v1.SandboxSpec` | `ws/openshell.go` | Sandbox creation parameters |
| `v1.Provider` | `ws/openshell.go` | Provider for Ensure() calls |
| `v1.ExecResult` | `ws/openshell.go`, `ws/channel_openshell.go` | Command execution output |
| `v1.StatusError` | `ws/openshell.go` | Typed gRPC errors |
| `types.SandboxReady` | `ws/openshell.go` | Phase constant for status checks |
| `types.SandboxProvisioning` | `ws/openshell.go` | Phase constant for polling |
| `types.SandboxError` | `ws/openshell.go` | Phase constant for error detection |
| `fake.NewClient()` | `*_test.go` | In-memory fake for unit tests |

## State Mapping

### Sandbox Phase (Status Checks)

```
cc-deck InfraState    ←  SDK SandboxPhase
─────────────────────────────────────────
InfraStateRunning     ←  types.SandboxReady
InfraStateStarting    ←  types.SandboxProvisioning
InfraStateError       ←  types.SandboxError
InfraStateStopped     ←  IsNotFound(err) on Get
```

### Config Mapping (GatewayConfig to v1.Config)

```
GatewayConfig              →  v1.Config
─────────────────────────────────────────
.Address                   →  .Address
.TLS == true               →  .TLS = &v1.TLSConfig{...}
.TLSCertPath               →  .TLS.CertFile
.TLSKeyPath                →  .TLS.KeyFile
.TLSCAPath                 →  .TLS.CAFile
(not configurable)         →  .Auth = v1.NoAuth()
```

## Function Signature Changes

### `internal/openshell/credentials.go`

```
InjectEnvVars(ctx, client Client, sandboxID, vars)
→ InjectEnvVars(ctx, client v1.ClientInterface, sandboxID, vars)

UploadFileCredential(ctx, client Client, sandboxID, localPath, remotePath, envVarName)
→ UploadFileCredential(ctx, client v1.ClientInterface, sandboxID, localPath, remotePath, envVarName)
```

Internal call changes:
- `client.ExecSandbox(ctx, sandboxID, cmd)` → `client.Exec().Run(ctx, sandboxID, cmd)`
- `client.Upload(ctx, sandboxID, local, remote)` → `client.Files().Upload(ctx, sandboxID, local, remote)`

### `internal/credential/transport.go`

```
type OpenShellClient interface {
    ExecSandbox(ctx, sandboxName, cmd) (*openshell.ExecResult, error)
    Upload(ctx, sandboxName, localPath, remotePath) error
}
→ Accept v1.ClientInterface directly (or define matching narrow interface)
```
