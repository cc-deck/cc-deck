# Contract: OpenShell Workspace Interface

## Workspace Interface Compliance

The OpenShellWorkspace MUST satisfy the cc-deck Workspace interface as defined in `internal/ws/interface.go`. This contract specifies OpenShell-specific behavior for each method.

### Lifecycle Methods

**Create(ctx, opts)**
- Calls `CreateSandbox` gRPC RPC with sandbox config from workspace definition
- Blocks until sandbox reaches "running" state (polls GetSandbox)
- Stores sandbox ID in workspace instance state
- Returns error if gateway unreachable or sandbox creation fails
- Timeout: 60 seconds (accounts for image pull on first run)

**Delete(ctx, force)**
- Kills Zellij session inside sandbox via ExecSandbox (best-effort)
- Calls `DeleteSandbox` gRPC RPC
- Removes workspace from cc-deck state store
- If force=true and gateway unreachable: removes local state, warns about orphaned sandbox

**KillSession(ctx)**
- Executes `zellij kill-all-sessions` inside sandbox via ExecSandbox
- Does NOT destroy the sandbox (infra continues running)
- Clears SSH tunnel state if attached

### Session Methods

**Attach(ctx)**
- Checks for existing active tunnel (single-attach enforcement)
- If tunnel exists and PID alive: returns "workspace already attached" error
- If tunnel exists but PID dead: clears stale state, proceeds
- Calls `CreateSshSession` gRPC RPC to establish SSH tunnel
- Runs `zellij attach --create` over the SSH tunnel
- Stores tunnel state (session ID, PID, local port)

**Status(ctx)**
- Calls `GetSandbox` gRPC RPC
- Maps OpenShell state to cc-deck InfraState:
  - creating -> (transient, not stored)
  - running -> InfraStateRunning
  - suspended -> InfraStateStopped
  - error -> InfraStateError
  - deleted -> (remove from state, return nil InfraState)
- Checks SSH tunnel PID liveness for SessionState
- Returns reconciled WorkspaceStatus

### Execution Methods

**Exec(ctx, cmd)**
- Calls `ExecSandbox` gRPC RPC with command array
- Streams stdout/stderr to terminal
- Returns exit code from sandbox

**ExecOutput(ctx, cmd)**
- Same as Exec but captures stdout as string return value

### InfraManager Methods

**Start(ctx)**
- Equivalent to Create with stored definition
- Used for resuming a previously stopped workspace

**Stop(ctx)**
- Calls KillSession first
- Calls DeleteSandbox
- Updates InfraState to stopped

### Channel Methods

**PipeChannel(ctx)**
- Returns OpenShellPipeChannel (lazy init via sync.Once)
- Pipe messages are sent via ExecSandbox running `zellij pipe` commands
- Pipe receives via ExecSandbox running `zellij pipe --name <name> --direction from`

**DataChannel(ctx)**
- Returns OpenShellDataChannel (lazy init via sync.Once)
- Push: tar archive piped over SSH tunnel
- Pull: tar archive piped back over SSH tunnel
- Respects exclusion lists from workspace definition

**GitChannel(ctx)**
- Returns OpenShellGitChannel (lazy init via sync.Once)
- Uses git ext:: protocol handler over SSH tunnel
- Fetch: creates temporary git remote with ext:: transport, fetches commits
- Push: reverse direction over same transport

## gRPC Client Contract

The `openshell.Client` wrapper MUST:
- Accept GatewayConfig for connection setup
- Handle TLS negotiation (optional, with localhost bypass)
- Implement automatic reconnection on transient gRPC errors
- Expose typed methods for each RPC (not raw proto calls)
- Log connection state changes at debug level

## Error Handling

All gRPC errors MUST be wrapped in cc-deck's ChannelError type with:
- Channel type (pipe/data/git)
- Operation name
- Workspace name
- Original gRPC status code and message
