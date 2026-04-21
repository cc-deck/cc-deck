# Brainstorm: Workspace Channels (Unified Local-Remote Transport)

**Date:** 2026-04-21
**Status:** Active
**Trigger:** Comparative analysis with [lince](https://github.com/RisorseArtificiali/lince) revealed voice relay, clipboard bridging, file transfer, and git harvest all solve the same fundamental problem: bridging the gap between the local machine and a remote workspace. Each currently reinvents its own transport plumbing over the environment's `Exec()` mechanism.

## Problem Statement

cc-deck supports six workspace types (local, container, compose, SSH, k8s-deploy, k8s-sandbox). Multiple features need to send data between the local machine and a remote workspace:

| Feature | Direction | Current/Planned Transport | Brainstorm |
|---|---|---|---|
| Clipboard (images) | local -> remote | kubectl exec pipe to staging file | 05 |
| Voice text relay | local -> remote | Exec() + zellij pipe | (from lince analysis, 022) |
| Git push | local -> remote | ext:: over exec | 026 |
| Git harvest | remote -> local | ext:: over exec | 026 |
| File sync (push) | local -> remote | podman cp / kubectl cp / rsync | 025 |
| File sync (pull) | remote -> local | podman cp / kubectl cp / rsync | 025 |
| Text paste | local -> remote | Terminal TTY (works already) | N/A |

Each feature builds its own exec-based transport. The clipboard brainstorm (05) designs a background goroutine with polling and its own reconnection logic. Voice relay would build its own per-utterance Exec() call. Git harvest constructs ext:: URLs per environment type. File sync uses copy commands specific to each runtime.

This is the same pattern repeated five times with different plumbing.

## Scope

This brainstorm covers the **channel abstraction layer only**: a set of typed Go interfaces that wrap the workspace's `Exec()` capability into reusable, per-workspace-type transport channels.

**In scope:**
- Channel interface definitions (PipeChannel, DataChannel, GitChannel)
- Per-workspace-type implementations
- Refactoring existing `Push()`, `Pull()`, `Harvest()` to delegate to channels internally
- Error model with wrapped/unwrappable errors

**Out of scope (separate brainstorms):**
- Voice capture and transcription (malgo, whisper.cpp, companion binary)
- Clipboard watcher and bridge
- SSH ControlMaster optimization (performance enhancement, not prerequisite)

The channel interfaces are validated against voice and clipboard use cases to ensure they can back those features later.

## Design Principle: Channel as Abstraction

A "workspace channel" is a typed communication path between the local machine and a remote workspace. The channel abstracts away the transport mechanism (SSH, kubectl exec, podman exec) and provides a clean interface for higher-level features to send and receive data.

The channel is NOT a new network protocol. It is a Go abstraction that wraps the workspace's existing `Exec()` capability into reusable, typed interfaces.

## Channel Types

### 1. PipeChannel (unidirectional text/commands)

Sends text or commands to the remote Zellij session via `zellij pipe`.

**Future consumers:**
- Voice relay: send transcribed speech to focused agent session
- Remote plugin control: trigger attend, navigate, pause from local

**Transport:** Single Exec() call per message: `zellij --session <name> pipe --name <pipe-name> --payload <data>`

**Latency characteristics per workspace type:**
- SSH (without ControlMaster): ~100-300ms (TCP handshake + auth)
- SSH (with ControlMaster): ~5-10ms (reuses connection)
- kubectl exec: ~50-200ms (API server round-trip, in-cluster)
- podman exec: ~10-20ms (local socket)

These latencies are acceptable for utterance-level voice relay (sentences arriving every few seconds). Keystroke-level streaming is not a use case.

**Interface:**

```go
type PipeChannel interface {
    // SendPipe delivers a text payload to a named zellij pipe.
    SendPipe(ctx context.Context, pipeName string, payload string) error

    // Close releases any resources held by the channel.
    Close() error
}
```

### 2. DataChannel (bidirectional binary data)

Transfers files or binary data between local and remote.

**Future consumers:**
- Clipboard bridge: push image data to staging directory
- File upload: send files to workspace
- File download: retrieve files from workspace

**Transport per workspace type:**

| Workspace | Push | Pull |
|---|---|---|
| SSH | rsync / scp | rsync / scp |
| Container | podman cp | podman cp |
| Compose | podman cp (into compose container) | podman cp |
| K8s Deploy | kubectl cp / tar-over-exec | kubectl cp / tar-over-exec |
| K8s Sandbox | kubectl cp / tar-over-exec | kubectl cp / tar-over-exec |
| Local | filesystem (no transport needed) | filesystem |

**Interface:**

```go
type DataChannel interface {
    // Push transfers a local file or byte payload to a remote path.
    Push(ctx context.Context, localPath string, remotePath string) error

    // PushBytes transfers raw bytes to a remote path (for clipboard images, generated data).
    PushBytes(ctx context.Context, data []byte, remotePath string) error

    // Pull retrieves a remote file to a local path.
    Pull(ctx context.Context, remotePath string, localPath string) error

    // Close releases any resources held by the channel.
    Close() error
}
```

### 3. GitChannel (bidirectional delta sync)

Uses git's ext:: protocol to tunnel git operations over exec. Owns the full git workflow including temporary remote add/remove.

**Future consumers:**
- Git push: send local commits to remote workspace
- Git harvest: retrieve agent's commits back to local

**Transport:** `git push/fetch ext::<exec-command> %S /workspace`

**ext:: URL per workspace type:**

| Workspace | ext:: URL |
|---|---|
| Container | `ext::podman exec -i cc-deck-<name> %S /workspace` |
| Compose | `ext::podman exec -i cc-deck-<name> %S /workspace` |
| K8s Deploy | `ext::kubectl exec -i -n <ns> <pod> -c session -- %S /workspace` |
| SSH | `ext::ssh <user>@<host> %S /workspace` |
| Local | N/A (same filesystem) |

**Interface:**

```go
type GitChannel interface {
    // GitPush pushes local commits to the remote workspace.
    // The branch parameter specifies which branch to push.
    GitPush(ctx context.Context, localRepoPath string, branch string) error

    // GitFetch fetches commits from the remote workspace to local.
    // The branch parameter specifies which branch to fetch.
    GitFetch(ctx context.Context, localRepoPath string, branch string) error

    // Close releases any resources held by the channel.
    Close() error
}
```

## Architecture

### Workspace Interface Extension

Each workspace type already implements `Exec()`. The channel abstraction adds typed channel accessors:

```go
type Workspace interface {
    // ... existing methods ...

    // PipeChannel returns a channel for sending text/commands to zellij pipes.
    PipeChannel(ctx context.Context) (PipeChannel, error)

    // DataChannel returns a channel for file/binary data transfer.
    DataChannel(ctx context.Context) (DataChannel, error)

    // GitChannel returns a channel for git protocol tunneling.
    GitChannel(ctx context.Context) (GitChannel, error)
}
```

Channels are created **on demand** with lazy initialization. Each workspace type caches its channel instances internally. No pre-creation during `cc-deck attach`.

### Refactoring Existing Methods

The existing `Push()`, `Pull()`, and `Harvest()` methods on the Workspace interface will be refactored to delegate to channels internally:

```go
func (e *K8sDeployWorkspace) Push(ctx context.Context, opts SyncOpts) error {
    if opts.UseGit {
        ch, err := e.GitChannel(ctx)
        if err != nil {
            return err
        }
        return ch.GitPush(ctx, opts.LocalPath, "main")
    }
    ch, err := e.DataChannel(ctx)
    if err != nil {
        return err
    }
    return ch.Push(ctx, opts.LocalPath, opts.RemotePath)
}

func (e *K8sDeployWorkspace) Harvest(ctx context.Context, opts HarvestOpts) error {
    ch, err := e.GitChannel(ctx)
    if err != nil {
        return err
    }
    return ch.GitFetch(ctx, ".", opts.Branch)
}
```

This eliminates the duplicated transport plumbing across workspace types. The existing `k8s_sync.go` code (~90 lines of git remote management) moves into the GitChannel implementation.

### Error Model

Channel errors are wrapped with a `ChannelError` type that provides a human-readable summary while preserving the underlying error for debugging:

```go
type ChannelError struct {
    Channel string // "pipe", "data", "git"
    Op      string // "send", "push", "pull", "fetch"
    Err     error  // underlying error (exec failure, network, etc.)
}

func (e *ChannelError) Error() string {
    return fmt.Sprintf("channel %s %s failed: %s", e.Channel, e.Op, e.humanMessage())
}

func (e *ChannelError) Unwrap() error {
    return e.Err
}

func (e *ChannelError) humanMessage() string {
    // Maps common underlying errors to user-friendly messages:
    // "pod not found" -> "workspace not running"
    // "connection refused" -> "workspace unreachable"
    // etc.
}
```

CLI commands display the human-readable message by default. The `--verbose` flag shows the unwrapped error chain for debugging.

## Consumer Validation

The channel interfaces are designed against these future consumer patterns:

### Voice Relay (PipeChannel consumer)

```go
func relayVoiceText(ctx context.Context, ws Workspace, text string) error {
    ch, err := ws.PipeChannel(ctx)
    if err != nil {
        return err
    }
    return ch.SendPipe(ctx, "cc-deck:voice", text)
}
```

Works for any workspace type. The voice capture/transcription mechanism (separate brainstorm) produces text and calls this function.

### Clipboard Bridge (DataChannel consumer)

```go
func pushClipboardImage(ctx context.Context, ws Workspace, imageData []byte) error {
    ch, err := ws.DataChannel(ctx)
    if err != nil {
        return err
    }
    return ch.PushBytes(ctx, imageData, "/tmp/.cc-clipboard/latest.png")
}
```

Works for any workspace type. The clipboard watcher (separate brainstorm) detects changes and calls this function. No custom background goroutine for exec management needed.

## Sequencing

### Phase 1: Interface and PipeChannel
1. Define channel interfaces (`PipeChannel`, `DataChannel`, `GitChannel`)
2. Define `ChannelError` type
3. Implement `PipeChannel` for all workspace types (wraps Exec() + zellij pipe)
4. Add `PipeChannel()` method to Workspace interface
5. Integration tests against real workspaces

### Phase 2: DataChannel and Refactoring
1. Implement `DataChannel` for all workspace types
2. Refactor `Push()` and `Pull()` to delegate to DataChannel
3. Integration tests for all workspace types

### Phase 3: GitChannel and Refactoring
1. Implement `GitChannel` with ext:: URL construction and full git remote management
2. Move `k8s_sync.go` git logic into GitChannel
3. Refactor `Harvest()` to delegate to GitChannel
4. Integration tests for all workspace types

## Testing Strategy

All channel implementations are tested against real infrastructure using the existing K8s integration test setup from spec 016. No mock channel implementations. Tests verify actual data transfer across workspace types.

## Open Questions

1. **Local workspace passthrough.** For local workspaces, channels are trivial (filesystem access, local zellij pipe). Should local workspaces skip the channel abstraction entirely, or go through it for API consistency? Recommendation: go through it for consistency, with thin implementations.

2. **Security for DataChannel.** The clipboard brainstorm (05) proposed AES-256-GCM encryption for clipboard data. Should this be a channel-level concern (encrypt all DataChannel data) or a consumer-level concern? Recommendation: consumer-level, since not all data transfers need encryption.

## Prior Art

| Project | Approach | What we learn |
|---|---|---|
| lince (RisorseArtificiali) | VoxCode + zellij pipe for voice relay | Pipe-based voice relay works, but only for local Zellij |
| DevPod | SSH-based dev environment with port forwarding | Persistent SSH connection for all operations |
| VS Code Remote | Persistent websocket channel for all remote ops | Multiplexed channel over single connection |
| git ext:: protocol | Tunnel git over arbitrary exec commands | Proven pattern for exec-based data transfer |

## Related Brainstorms

- **05 (clipboard-bridge):** Will become a DataChannel consumer
- **026 (git-harvest-sync):** Will become a GitChannel consumer
- **038 (workspace-repos):** Workspace setup will use DataChannel for initial repo provisioning
- **TBD (voice-capture):** Will become a PipeChannel consumer; covers audio capture, VAD, whisper.cpp, companion binary
- **TBD (ssh-controlmaster):** Performance optimization for SSH channels; not a prerequisite
