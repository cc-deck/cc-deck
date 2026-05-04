# Brainstorm: cc-deck OpenShell Backend

**Date:** 2026-04-30
**Status:** active

## Problem Framing

cc-deck is a terminal-native workspace manager for Claude Code sessions. It supports multiple execution environments (local, container, SSH, Kubernetes) through a clean Go interface (`Workspace`). Each backend implements lifecycle management, command execution, file sync, and three channel types (Pipe, Data, Git).

OpenShell is NVIDIA's agent sandbox platform. It provides six-layer security isolation (network namespace, per-binary OPA policy, L7 HTTP inspection, credential placeholders, Landlock filesystem, seccomp syscall filtering) with a gRPC gateway API and pluggable compute drivers (Podman, Docker, Kubernetes).

The problem: cc-deck has no sandboxed execution environment. Running Claude Code in a cc-deck workspace gives the agent full access to the host's filesystem, network, and credentials. OpenShell solves this, but cc-deck doesn't know how to talk to it.

The goal is to add an OpenShell backend to cc-deck so that workspaces run inside OpenShell sandboxes, getting all six security layers transparently.

## Approaches Considered

### A: Zellij Inside Sandbox (Chosen)

The sandbox IS the workspace. cc-deck calls `CreateSandbox` via gRPC with a custom image that includes Zellij and Claude Code. Zellij runs as the agent command inside the sandbox. Claude Code runs inside Zellij panes. cc-deck attaches via OpenShell's SSH tunnel (`CreateSshSession`).

```
cc-deck (host)
  |
  +-- gRPC --> OpenShell Gateway
                |
                +-- Podman container (sandbox)
                      |-- Supervisor (PID 1)
                      |-- Zellij (agent process)
                      |     |-- Claude Code (pane 1)
                      |     |-- Claude Code (pane 2)
                      |     +-- shell (pane 3)
                      +-- Network namespace + OPA + Landlock
```

- Pros: Full security perimeter around everything Claude does. Per-binary network policy works (Claude vs. npm postinstall). Credential placeholders work transparently. Zellij pipes work natively (local to sandbox). Clean mapping to cc-deck's interface.
- Cons: Requires custom sandbox image with Zellij. Zellij sidebar plugin may not work inside the sandbox initially. SSH tunnel depends on gateway availability.

### B: Zellij Outside, Claude Code Inside

Zellij runs on the host. Each pane calls `ExecSandbox` to run commands inside the sandbox.

- Pros: No custom image needed. Sidebar plugin works natively. Simpler session management.
- Cons: Breaks the pipe channel (Zellij pipes can't cross sandbox boundary). `ExecSandbox` is non-interactive, Claude Code's interactive mode needs SSH. Multiple execs create separate processes with separate policy evaluation. Fundamentally splits the security model.

### C: Hybrid (gRPC Lifecycle + SSH Session)

Same as A but explicitly separates gRPC for sandbox lifecycle and SSH for session management. Reuses most of cc-deck's existing SSH backend code.

- Pros: Maximum code reuse from SSH backend. Clean transport separation.
- Cons: Two transport layers add complexity. SSH tunnel has known issues with K8s ingress (HTTP CONNECT vs. SPDY). Effectively the same as A with more plumbing.

## Decision

**Approach A: Zellij inside the sandbox.**

The reasoning: OpenShell is designed so that the sandbox runs the agent command and everything it spawns stays inside the security perimeter. Zellij inside the sandbox means Claude Code, its subprocesses, and all tool calls are subject to network policy, Landlock, and seccomp. Approach B breaks this by letting Zellij pipes bypass the boundary. Approach C is A with unnecessary transport complexity for the Podman driver target.

## Key Requirements

### Integration Architecture

- cc-deck talks to OpenShell via the **gRPC gateway API** (not CLI wrapping, not direct Podman calls)
- Target the **Podman compute driver** first, design the interface to accommodate the K8s driver later
- New workspace type: `WorkspaceTypeOpenShell`
- Implements `InfraManager` (sandbox lifecycle maps to Start/Stop)

### Interface Mapping

| cc-deck Interface | OpenShell gRPC Call | Notes |
|---|---|---|
| `Create(ctx, opts)` | `CreateSandbox` | Custom image with Zellij, policy from workspace definition |
| `Delete(ctx, force)` | `DeleteSandbox` | Cascade: kill Zellij session, then destroy sandbox |
| `Attach(ctx)` | `CreateSshSession` + Zellij attach | SSH tunnel into sandbox, attach to Zellij session |
| `KillSession(ctx)` | Kill Zellij inside sandbox (via exec) | Leaves sandbox infra running |
| `Exec(ctx, cmd)` | `ExecSandbox` | Runs command inside sandbox as sandbox user |
| `ExecOutput(ctx, cmd)` | `ExecSandbox` + capture stdout | Same, with output capture |
| `Status(ctx)` | `GetSandbox` | Map sandbox state to cc-deck's InfraState + SessionState |
| `Start(ctx)` | `CreateSandbox` | InfraManager: provision the sandbox |
| `Stop(ctx)` | `DeleteSandbox` | InfraManager: tear down the sandbox |
| `PipeChannel` | Zellij pipes (local inside sandbox) | Works natively, Zellij runs in sandbox |
| `DataChannel` | tar/rsync over SSH tunnel | Similar to SSH backend's data channel |
| `GitChannel` | git ext:: over SSH tunnel | Similar to SSH backend's git channel |
| `Push(ctx, opts)` | DataChannel.Push | File sync into sandbox |
| `Pull(ctx, opts)` | DataChannel.Pull | File sync out of sandbox |
| `Harvest(ctx, opts)` | GitChannel.Fetch | Extract git commits from sandbox |

### Sandbox Image

A custom Dockerfile (or OpenShell base image) that includes:
- Zellij (terminal multiplexer)
- Claude Code (the agent binary)
- Git, standard development tools
- cc-deck's Zellij plugin (sidebar, if compatible)

The image needs to work with OpenShell's supervisor injection. OpenShell injects the supervisor binary via init container (K8s) or volume mount (Podman). The Dockerfile should not conflict with this.

### Gateway Discovery

cc-deck needs to find the OpenShell gateway:
- Configuration in workspace definition YAML (`~/.config/cc-deck/`)
- Environment variable fallback (`OPENSHELL_GATEWAY_URL`)
- Auto-detect local gateway if running on same host

### Network Policy Defaults

A default policy for cc-deck workspaces that allows:
- API endpoints for inference providers (api.anthropic.com, api.openai.com)
- Package registries (registry.npmjs.org, pypi.org, crates.io)
- Git hosting (github.com, gitlab.com)
- Blocks everything else by default

Users can customize via workspace definition or `openshell policy set`.

### Session Resilience

- Gateway restart drops SSH tunnels. cc-deck should detect disconnection and re-establish via `CreateSshSession`.
- Sandbox survives gateway restart (Podman containers persist). cc-deck should reconcile stored state with actual sandbox state on reconnection.
- Zellij session inside sandbox survives SSH tunnel drops (it's a local process). Reattaching the SSH tunnel and reconnecting to Zellij should restore the session.

## Open Questions

- What goes in the custom sandbox Dockerfile? Should cc-deck provide pre-built images, or should users bring their own?
- How does the Zellij sidebar plugin work when Zellij runs inside the sandbox? The plugin communicates via Zellij pipes, which are local to the sandbox. The host-side cc-deck process can't reach them without tunneling.
- Should cc-deck support multiple sandboxes per workspace (one sandbox per Claude Code session in a multi-pane layout), or one sandbox with multiple panes?
- How does network filtering configuration from cc-deck's existing domain groups map to OpenShell's OPA/Rego policy format?
- What happens when the Podman driver is replaced or updated? How stable is the gRPC API contract across OpenShell versions?
- How does credential provider configuration flow from cc-deck workspace definitions to OpenShell's `CreateProvider` API?
