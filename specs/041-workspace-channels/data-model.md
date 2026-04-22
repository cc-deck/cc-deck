# Data Model: Workspace Channels

**Date**: 2026-04-22

## Entities

### PipeChannel

A unidirectional (send-only for now) text transport from local machine to a named zellij pipe in the remote workspace. Interface designed for request-response to support future TUI status queries.

**Fields** (interface, not struct):
- `Send(ctx, pipeName, payload) error` - fire-and-forget text delivery
- `SendReceive(ctx, pipeName, payload) (string, error)` - future: request-response

**Applicability**: All workspace types (local, container, compose, SSH, k8s-deploy, k8s-sandbox).

**Transport per type**:
| Type | Send mechanism |
|------|---------------|
| local | Direct `zellij pipe` subprocess |
| container | `Exec(["zellij", "pipe", ...])` via podman |
| compose | `Exec(["zellij", "pipe", ...])` via podman |
| ssh | `Exec(["zellij", "pipe", ...])` via SSH |
| k8s-deploy | `Exec(["zellij", "pipe", ...])` via kubectl |
| k8s-sandbox | `Exec(["zellij", "pipe", ...])` via kubectl |

### DataChannel

Bidirectional file and binary data transport between local machine and remote workspace. Replaces the current Push/Pull implementations.

**Fields** (interface):
- `Push(ctx, opts SyncOpts) error` - local to remote file transfer
- `Pull(ctx, opts SyncOpts) error` - remote to local file transfer
- `PushBytes(ctx, data []byte, remotePath string) error` - raw bytes to remote file

**Applicability**: All workspace types.

**Transport per type**:
| Type | Push/Pull mechanism | Excludes support |
|------|-------------------|-----------------|
| local | Filesystem copy (NEW) | No |
| container | `podman cp` | No |
| compose | `podman cp` | No |
| ssh | `rsync` (fallback: `scp`) | Yes |
| k8s-deploy | tar-over-exec | No |
| k8s-sandbox | tar-over-exec | No |

### GitChannel

Git commit synchronization between local and remote repositories. Encapsulates the add-remote / fetch-or-push / cleanup lifecycle.

**Fields** (interface):
- `Fetch(ctx, opts HarvestOpts) error` - fetch remote commits to local
- `Push(ctx) error` - push local commits to remote

**Applicability**: All workspace types except local (same filesystem).

**Transport per type**:
| Type | Remote URL format |
|------|------------------|
| container | `ext::podman exec -i <name> -- %S /workspace` (NEW) |
| compose | `ext::podman exec -i <name> -- %S /workspace` (NEW) |
| ssh | `ssh://<host><workspace>` |
| k8s-deploy | `ext::kubectl [--kubeconfig X] [--context X] exec -i -n <ns> <pod> -- %S /workspace` |
| k8s-sandbox | `ext::kubectl [--kubeconfig X] [--context X] exec -i -n <ns> <pod> -- %S /workspace` |

### ChannelError

Structured error wrapping transport-level failures with channel context. First custom error type in the project.

**Fields**:
- `Channel string` - channel type: "pipe", "data", "git"
- `Op string` - operation: "send", "push", "pull", "fetch"
- `Workspace string` - workspace name for context
- `Summary string` - human-readable message
- `Err error` - underlying cause (supports Unwrap)

**Behavior**:
- `Error()` returns `Summary`
- `Unwrap()` returns `Err`
- Compatible with `errors.Is()` (chains through underlying error)
- Compatible with `errors.As()` (matches ChannelError struct type)

## Relationships

```
Workspace interface
├── PipeChannel() → PipeChannel
├── DataChannel() → DataChannel
├── GitChannel() → GitChannel (returns ErrNotSupported for local)
├── Push() → delegates to DataChannel.Push()
├── Pull() → delegates to DataChannel.Pull()
├── Harvest() → delegates to GitChannel.Fetch()
└── ExecOutput() → NEW: captures stdout (needed for PipeChannel request-response)

Channel implementations (grouped by transport):
├── Podman-based (container + compose)
│   ├── podmanPipeChannel
│   ├── podmanDataChannel
│   └── podmanGitChannel
├── K8s-based (k8s-deploy + k8s-sandbox)
│   ├── k8sPipeChannel
│   ├── k8sDataChannel
│   └── k8sGitChannel
├── SSH-based
│   ├── sshPipeChannel
│   ├── sshDataChannel
│   └── sshGitChannel
└── Local
    ├── localPipeChannel
    └── localDataChannel (no GitChannel)
```

## State Transitions

Channels are stateless. They do not track workspace state internally. Each method call resolves the current workspace target (pod name, container name, SSH host) at invocation time, enabling transparent reconnection after workspace restart.

## Validation Rules

- PipeChannel.Send: pipeName must be non-empty, payload may be empty
- DataChannel.Push: SyncOpts.LocalPath must be non-empty
- DataChannel.Pull: SyncOpts.RemotePath must be non-empty
- DataChannel.PushBytes: data may be empty (creates empty file), remotePath must be non-empty
- GitChannel.Fetch: workspace must have git installed remotely
- GitChannel.Push: local repository must have commits ahead of remote
- All channel operations: workspace must be running (checked by wrapper, not by channel)
