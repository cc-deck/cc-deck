# Data Model: OpenShell gRPC Client

## New Entities

### grpcClient

Implements the `Client` interface via gRPC. Holds a persistent connection to the gateway.

| Field | Type | Purpose |
|-------|------|---------|
| conn | gRPC connection | Persistent connection to gateway (with mTLS or insecure) |
| client | OpenShell gRPC stub | Generated client from proto |
| cfg | GatewayConfig | Address, TLS settings |

### tunnelDialer

Creates SSH connections through the gateway's HTTP CONNECT tunnel.

| Field | Type | Purpose |
|-------|------|---------|
| gatewayAddr | string | Gateway host:port for HTTP CONNECT |
| tlsConfig | TLS config | mTLS credentials for CONNECT handshake |

## Modified Entities

### ProviderConfig

The `FromExisting` bool and `Credentials map[string]string` fields remain but their semantics change. In the gRPC client, `Credentials` is split into proto `Provider.credentials` and `Provider.config` based on provider type. The `FromExisting` bool is no longer mapped to a CLI flag; instead, it triggers credential discovery logic in the client before building the proto message.

### GatewayConfig

Gains TLS-related fields for direct connection setup:

| Field | Type | Purpose |
|-------|------|---------|
| Address | string | Gateway host:port (existing) |
| TLS | bool | Whether to use TLS (existing) |
| TLSCertPath | string | Client cert for mTLS (existing) |
| TLSKeyPath | string | Client key for mTLS (existing) |
| TLSCAPath | string | CA cert for server verification (existing) |

These fields already exist in `GatewayConfig` but are unused by `cliClient`. The `grpcClient` uses them to configure the gRPC dial options.

## Removed Entities (moved to legacy)

### cliClient (-> client_legacy.go)

The entire `cliClient` struct and its methods move behind a `cli_legacy` build tag. No structural changes, just file relocation.

Functions that move:
- `execCLI`
- `execCLICaptureName`
- `parseSandboxName`
- `parseSandboxPhase`
- `stripANSI`
- All `cliClient` method implementations
