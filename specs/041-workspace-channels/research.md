# Research: Workspace Channels

**Date**: 2026-04-22
**Method**: Parallel codebase research via 4 agent teams

## Decision 1: Channel Interface Placement

**Decision**: Add `PipeChannel()`, `DataChannel()`, `GitChannel()` methods directly to the `Workspace` interface.

**Rationale**: The codebase follows a single-interface pattern consistently. All 12 existing methods live on `Workspace`. Adding 3 channel accessors brings it to 15, which is reasonable. The alternative (separate `ChannelProvider` interface) adds type assertions at every call site and diverges from established patterns.

**Alternatives considered**:
- Separate `ChannelProvider` interface with type assertion at call sites (rejected: breaks single-interface convention)
- Standalone channel factory taking a Workspace parameter (rejected: channels need internal workspace state like container name, SSH client, kubeconfig args)

## Decision 2: Transport Grouping

**Decision**: Group channel implementations by transport mechanism, not by workspace type.

**Rationale**: Research revealed four distinct transport categories:
1. **Podman-based** (container + compose): Both use `podman.Cp()` for data, `ext::podman exec` for git. Code is near-identical between container.go and compose.go.
2. **K8s-based** (k8s-deploy + k8s-sandbox): Both use tar-over-exec for data, `ext::kubectl exec` for git. k8s_sync.go already shares helpers.
3. **SSH-based** (ssh): Uses rsync/scp for data, `ssh://` URLs for git. Unique transport.
4. **Local** (local): Filesystem copy for data, direct zellij pipe for pipe. No git channel.

Sharing implementations within groups avoids the current duplication (container.go and compose.go Push/Pull are copy-pasted).

**Alternatives considered**:
- One implementation per workspace type (rejected: perpetuates container/compose duplication)
- Single generic implementation with strategy pattern (rejected: transports differ too fundamentally)

## Decision 3: ExecOutput on Workspace Interface

**Decision**: Add `ExecOutput(ctx context.Context, cmd []string) (string, error)` to the Workspace interface.

**Rationale**: PipeChannel's request-response pattern (FR-001) requires capturing stdout from remote commands. Each backend already has this capability internally (`podman.ExecOutput`, `k8sExecOutput`, `ssh.Client.Run`) but it is not exposed through the Workspace interface. Adding it aligns the interface with actual capability and enables PipeChannel request-response without each channel reaching into backend internals.

**Alternatives considered**:
- Channel implementations call internal exec functions directly (rejected: channels would need access to private workspace fields, breaking encapsulation)
- Add ExecOutput only to a subset of types (rejected: inconsistent interface, all remote types support it)

## Decision 4: ChannelError as First Custom Error Type

**Decision**: Introduce `ChannelError` struct as the project's first custom error type, implementing `Error()` and `Unwrap()`.

**Rationale**: The codebase currently uses only sentinel errors (`var ErrX = errors.New(...)`) with `fmt.Errorf` wrapping. ChannelError needs structured context (channel type, operation, workspace name, human-readable summary) that sentinels cannot carry. Go's `errors.As()` enables consumers to extract channel-specific context. This is compatible with existing `errors.Is()` usage since `Unwrap()` preserves the chain.

**Alternatives considered**:
- Continue with fmt.Errorf wrapping only (rejected: FR-009 requires structured error with human-readable summary + underlying cause)
- Third-party error library (rejected: unnecessary dependency for one error type)

## Decision 5: Local Workspace PipeChannel Transport

**Decision**: Local PipeChannel uses direct `zellij pipe` subprocess, not Exec().

**Rationale**: LocalWorkspace.Exec() returns `ErrNotSupported`. But the CLI already sends pipe messages locally via `exec.CommandContext(ctx, "zellij", "pipe", ...)` in `hook.go` and `session/save.go`. Local PipeChannel follows this existing pattern rather than requiring Exec() changes.

**Alternatives considered**:
- Implement Exec() for local workspace (rejected: Exec() semantics are "run inside workspace", which is meaningless for local)
- Skip PipeChannel for local (rejected: FR-005 requires all channel types for local)

## Decision 6: Git Channel for Container/Compose

**Decision**: Enable GitChannel for container and compose workspaces using `ext::podman exec -i <name> -- %S /workspace`.

**Rationale**: Container and compose Harvest() currently return `ErrNotSupported`. The `ext::` protocol with podman exec is structurally identical to the working K8s pattern (`ext::kubectl exec`). This is new functionality enabled by the channel abstraction, similar to how DataChannel enables local workspace Push/Pull.

**Alternatives considered**:
- Keep container/compose Harvest unsupported (rejected: spec FR-005 requires GitChannel for all non-local types)

## Decision 7: Channel File Organization

**Decision**: New files in `cc-deck/internal/ws/` package, one file per channel type plus a shared base.

**Rationale**: Channels need access to workspace struct internals (container name, SSH client, kubeconfig args) which are unexported fields. Placing channels in the same package avoids exporting these internals. One file per channel type keeps files focused.

Files:
- `channel.go` - Channel interfaces, ChannelError, shared helpers
- `channel_pipe.go` - PipeChannel implementations per transport group
- `channel_data.go` - DataChannel implementations per transport group
- `channel_git.go` - GitChannel implementations, git remote lifecycle helper

**Alternatives considered**:
- Separate `internal/channel/` package (rejected: would require exporting workspace internals)
- One file per workspace type (rejected: duplicates transport-group sharing)

## Decision 8: Running State Checks

**Decision**: Running-state pre-checks remain in the workspace's Push/Pull/Harvest wrapper methods, not in channel implementations.

**Rationale**: Container, compose, and k8s-deploy all check `inst.State != WorkspaceStateRunning` before calling sync helpers. SSH does not (relies on connection failure). Keeping this check in the workspace method preserves existing behavior and keeps channel implementations focused on transport. The channel transparently reconnects after restart (per clarification), so the channel itself should not cache state.

**Alternatives considered**:
- Move state checks into channel constructors (rejected: contradicts transparent reconnect requirement)
- Add state check to every channel method (rejected: duplicates check logic, SSH doesn't need it)

## Codebase Research Basis

Explored by 4 parallel agents covering:
- All Push/Pull/Harvest implementations across 6 workspace types (container, compose, ssh, k8s-deploy, k8s-sandbox, local)
- Git sync implementation in k8s_sync.go and ssh.go (ext:: URLs, temporary remotes)
- Zellij pipe mechanism (plugin pipe handlers, CLI pipe usage, request-response via DumpState)
- Error handling patterns and Workspace interface structure

Key discoveries:
- Container/compose Push/Pull code is near-identical (prime consolidation target)
- K8s ext:: URL construction is duplicated between k8sHarvest and k8sGitPush
- ExecOutput exists per-backend but is not on the Workspace interface
- No custom error types exist yet; only sentinel errors
- K8s-sandbox type is defined but not implemented (no factory case)
- CLI harvest command does not expose --branch or --create-pr flags
- CommandRunner pattern in repos.go is a good precedent for transport abstraction
