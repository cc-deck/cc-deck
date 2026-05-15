# Data Model: OpenShell Backend

## Entities

### OpenShellWorkspace

Implements the `Workspace` and `InfraManager` interfaces.

**Fields**:
- `name` (string): Workspace name, unique within cc-deck state
- `store` (*FileStateStore): Reference to cc-deck's runtime state store
- `defs` (*DefinitionStore): Reference to workspace definition store
- `client` (*openshell.Client): gRPC client for OpenShell gateway
- `sandboxID` (string): OpenShell sandbox identifier, set after Create
- `sshTunnel` (*sshTunnelState): Active SSH tunnel state (nil if not attached)

**Lazy-initialized channels** (sync.Once pattern, matching other backends):
- `pipeOnce` / `pipeCh` (PipeChannel)
- `dataOnce` / `dataCh` (DataChannel)
- `gitOnce` / `gitCh` (GitChannel)

### GatewayConfig

Configuration for connecting to the OpenShell gateway.

**Fields**:
- `address` (string): Gateway host:port (default: `localhost:8080`)
- `tls` (bool): Whether to use TLS (default: false for localhost, warning for non-localhost)
- `tlsCertPath` (string, optional): Path to TLS certificate
- `tlsKeyPath` (string, optional): Path to TLS key
- `tlsCAPath` (string, optional): Path to CA certificate

**Resolution order**:
1. Workspace definition YAML (`gateway:` section)
2. Environment variable `OPENSHELL_GATEWAY_URL`
3. Default: `localhost:8080`

### SandboxConfig

Configuration for the sandbox provisioned by OpenShell.

**Fields**:
- `image` (string): Container image reference (default: `cc-deck/openshell-sandbox:latest`)
- `command` (string): Agent command (default: `zellij`)
- `policy` (string, optional): Path to network policy YAML
- `provider` (string, optional): OpenShell provider name for credential injection

### sshTunnelState

Internal state for the active SSH tunnel.

**Fields**:
- `sessionID` (string): OpenShell SSH session identifier
- `localPort` (int): Local port for SSH forwarding
- `pid` (int): PID of the SSH process (for liveness checking)
- `connected` (bool): Whether the tunnel is currently active

## State Transitions

### Workspace Lifecycle

```
[not exists] --Create--> running --Stop--> [deleted]
                 |                   |
                 +--error-->  error  |
                 |                   |
             running --Attach--> attached --Detach--> running
                                     |
                                 [tunnel drop] --> running (Zellij survives)
```

### InfraState Mapping

| Workspace Operation | InfraState Before | InfraState After |
|---|---|---|
| Create (success) | (none) | running |
| Create (failure) | (none) | error |
| Stop / Delete | running | (removed) |
| Gateway restart | running | running (reconciled via GetSandbox) |
| Sandbox crash | running | error (detected via GetSandbox) |

## Relationships

- OpenShellWorkspace 1:1 GatewayConfig (resolved at Create time)
- OpenShellWorkspace 1:1 SandboxConfig (from workspace definition)
- OpenShellWorkspace 0..1 sshTunnelState (only when attached)
- GatewayConfig resolves to openshell.Client (gRPC connection)
