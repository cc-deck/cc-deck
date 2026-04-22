# Quickstart: Workspace Channels

## What Changed

The Workspace interface gains three channel accessors (`PipeChannel()`, `DataChannel()`, `GitChannel()`) plus `ExecOutput()`. Existing `Push()`, `Pull()`, and `Harvest()` methods delegate to channels internally, preserving identical CLI behavior.

## Key Files

| File | Purpose |
|------|---------|
| `cc-deck/internal/ws/channel.go` | Channel interfaces, ChannelError type, shared helpers |
| `cc-deck/internal/ws/channel_pipe.go` | PipeChannel implementations (local, podman, k8s, ssh) |
| `cc-deck/internal/ws/channel_data.go` | DataChannel implementations (local, podman, k8s, ssh) |
| `cc-deck/internal/ws/channel_git.go` | GitChannel implementations (podman, k8s, ssh) + git remote helper |
| `cc-deck/internal/ws/interface.go` | Extended Workspace interface |

## Using Channels (Consumer Code)

```go
// Get a workspace
ws, err := resolveWorkspace(name, store, defs)

// Send text to a zellij pipe in the remote workspace
pipe, err := ws.PipeChannel(ctx)
err = pipe.Send(ctx, "cc-deck:voice", "hello world")

// Push a file to the remote workspace
data, err := ws.DataChannel(ctx)
err = data.Push(ctx, ws.SyncOpts{LocalPath: "config.yaml", RemotePath: "/workspace/config.yaml"})

// Fetch commits from the remote workspace
git, err := ws.GitChannel(ctx)
err = git.Fetch(ctx, ws.HarvestOpts{Branch: "feature-x"})
```

## Existing Commands (Unchanged Behavior)

```bash
# These commands work exactly as before, but internally use channels:
cc-deck ws push myworkspace ./file.txt /workspace/file.txt
cc-deck ws pull myworkspace /workspace/output.txt ./output.txt
cc-deck ws harvest myworkspace
```

## Error Handling

```go
err := pipe.Send(ctx, "cc-deck:voice", payload)
if err != nil {
    // Default: human-readable summary
    fmt.Println(err) // "pipe send failed: workspace 'dev' is not running"

    // Verbose: full error chain
    var chErr *ws.ChannelError
    if errors.As(err, &chErr) {
        fmt.Printf("Channel: %s, Op: %s, Cause: %v\n", chErr.Channel, chErr.Op, chErr.Err)
    }
}
```

## Transport Matrix

| Workspace Type | PipeChannel | DataChannel | GitChannel |
|---------------|-------------|-------------|------------|
| local | zellij pipe (direct) | filesystem copy | N/A |
| container | Exec (podman) | podman cp | ext::podman exec |
| compose | Exec (podman) | podman cp | ext::podman exec |
| ssh | Exec (SSH) | rsync/scp | ssh:// URL |
| k8s-deploy | Exec (kubectl) | tar-over-exec | ext::kubectl exec |
| k8s-sandbox | Exec (kubectl) | tar-over-exec | ext::kubectl exec |
