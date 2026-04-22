# Channel Interface Behavioral Contracts

**Date**: 2026-04-22
**Constitution Reference**: Principle VII (Interface Behavioral Contracts)

## PipeChannel Contract

### Method: Send(ctx context.Context, pipeName string, payload string) error

**Preconditions**:
- `pipeName` is non-empty
- Workspace is running (checked by caller, not by channel)

**Postconditions on success**:
- Payload delivered to the named zellij pipe in the remote workspace
- The zellij plugin's `pipe()` handler receives a `PipeMessage` with the given name and payload

**Error behavior**:
- Workspace not running: returns ChannelError with Op="send", wrapping the transport error
- Pipe name not found in remote workspace: returns ChannelError with summary indicating pipe name and suggesting user verify active Zellij session
- Transport timeout: returns ChannelError wrapping context deadline exceeded

**Concurrency**: Safe for concurrent use. Each call creates an independent subprocess.

**Lifecycle**: Stateless. No initialization required. Transparently works after workspace restart.

### Method: SendReceive(ctx context.Context, pipeName string, payload string) (string, error)

**Note**: Defined in the interface for future use. Initial implementation returns `("", ErrNotSupported)` for all workspace types. When implemented, follows the DumpState pattern: send via `zellij pipe`, capture stdout from `cli_pipe_output()`.

## DataChannel Contract

### Method: Push(ctx context.Context, opts SyncOpts) error

**Preconditions**:
- `opts.LocalPath` is non-empty and refers to an existing local file or directory
- Workspace is running (checked by caller)

**Postconditions on success**:
- File or directory at `opts.LocalPath` exists at `opts.RemotePath` in the remote workspace
- If `opts.RemotePath` is empty, defaults to `/workspace/<basename(opts.LocalPath)>`
- File contents are byte-identical to the source

**Error behavior**:
- Local path does not exist: returns ChannelError with Op="push" wrapping os.ErrNotExist
- Remote path not writable: returns ChannelError with remote path in summary
- Transport failure: returns ChannelError wrapping the underlying exec/copy error

**Concurrency**: Safe for concurrent use. Concurrent pushes to the same remote path produce last-writer-wins semantics.

### Method: Pull(ctx context.Context, opts SyncOpts) error

**Preconditions**:
- `opts.RemotePath` is non-empty
- Workspace is running (checked by caller)

**Postconditions on success**:
- File or directory at `opts.RemotePath` in the remote workspace exists at `opts.LocalPath` locally
- If `opts.LocalPath` is empty, defaults to current directory (".")

**Error behavior**:
- Remote path does not exist: returns ChannelError with Op="pull" wrapping the transport error
- Local path not writable: returns ChannelError wrapping os permission error

**Concurrency**: Safe for concurrent use.

### Method: PushBytes(ctx context.Context, data []byte, remotePath string) error

**Preconditions**:
- `remotePath` is non-empty
- Workspace is running (checked by caller)

**Postconditions on success**:
- A file exists at `remotePath` in the remote workspace with contents equal to `data`
- If `data` is empty, an empty file is created

**Error behavior**: Same as Push.

**Concurrency**: Safe for concurrent use.

## GitChannel Contract

### Method: Fetch(ctx context.Context, opts HarvestOpts) error

**Preconditions**:
- Local directory is a git repository
- Remote workspace has git installed and a git repository at the workspace path
- Workspace is running (checked by caller)

**Postconditions on success**:
- Remote commits are fetched into the local repository
- A temporary git remote is added, used for fetch, and removed (cleanup guaranteed via defer)
- Remote name follows convention: `cc-deck-<workspaceName>`
- If `opts.Branch` is set: local branch created from fetched refs
- If `opts.CreatePR` is true: branch pushed to origin and PR created via `gh`

**Error behavior**:
- Git not installed remotely: returns ChannelError with Op="fetch"
- Remote has no commits: returns ChannelError
- Temporary remote cleanup failure: logged but does not fail the operation (best-effort cleanup)

**Concurrency**: NOT safe for concurrent use with the same workspace. Git remote add/remove creates shared state in the local .git/config. Callers must serialize GitChannel operations per workspace.

### Method: Push(ctx context.Context) error

**Preconditions**:
- Local directory is a git repository with commits ahead of remote
- Remote workspace has git installed
- Workspace is running (checked by caller)

**Postconditions on success**:
- Local commits are pushed to the remote workspace's repository
- Temporary git remote added, used for push, and removed

**Error behavior**:
- Remote branch has diverged: returns ChannelError with summary indicating conflict
- No commits to push: returns ChannelError

**Concurrency**: Same restriction as Fetch.

## Channel Accessor Contract (on Workspace interface)

### Methods: PipeChannel(ctx) / DataChannel(ctx) / GitChannel(ctx)

**Behavior**:
- Returns a channel instance on first call, caches for subsequent calls (lazy initialization)
- Channel instance remains valid across workspace stop/restart cycles (transparent reconnect)
- GitChannel() returns ErrNotSupported for local workspaces

**Concurrency**: Accessor methods are safe for concurrent use (use sync.Once or equivalent for lazy init).
