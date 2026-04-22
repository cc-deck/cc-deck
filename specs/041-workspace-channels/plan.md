# Implementation Plan: Workspace Channels

**Branch**: `041-workspace-channels` | **Date**: 2026-04-22 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `specs/041-workspace-channels/spec.md`

## Summary

Introduce three typed channel abstractions (PipeChannel, DataChannel, GitChannel) that unify local-to-remote transport across all workspace types. The existing Push/Pull/Harvest methods delegate to channels internally, eliminating duplicated transport code between container/compose and enabling new capabilities (local Push/Pull, container/compose Harvest). The Workspace interface gains channel accessors and ExecOutput, plus a ChannelError structured error type.

## Technical Context

**Language/Version**: Go 1.25 (from go.mod)
**Primary Dependencies**: cobra v1.10.2 (CLI), adrg/xdg v0.5.3 (XDG paths), gopkg.in/yaml.v3 (YAML), client-go v0.35.2 (K8s API)
**Storage**: N/A (channels are stateless transport abstractions)
**Testing**: `go test ./...` via `make test`, `go vet` + clippy via `make lint`
**Target Platform**: Linux/macOS (CLI), wasm32-wasip1 (plugin, pipe handler changes only)
**Project Type**: CLI tool + Zellij WASM plugin
**Performance Goals**: PipeChannel < 500ms co-located / < 2s network-remote; DataChannel within 10% of current
**Constraints**: Channels must be safe for concurrent use; GitChannel serialized per workspace
**Scale/Scope**: 6 workspace types, 3 channel types, ~4 transport implementations per channel

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Two-Component Architecture | PASS | Plugin changes limited to pipe handler dispatch for new pipe names |
| II. Plugin Installation | N/A | No plugin installation changes |
| III. WASM Filename Convention | N/A | No WASM build changes |
| IV. WASM Host Function Gating | PASS | Any new pipe handling code must be gated |
| V. Zellij API Research | PASS | Pipe mechanism thoroughly researched |
| VI. Build via Makefile Only | PASS | Use `make test` / `make lint` / `make install` |
| VII. Interface Behavioral Contracts | PASS | [Channel contracts documented](contracts/channel-interfaces.md) |
| VIII. Simplicity | PASS | Transport grouping avoids premature abstraction; 3 similar lines > generic framework |
| IX. Documentation Freshness | PENDING | README, CLI reference updates required at completion |
| X. Spec Tracking in README | PENDING | Add spec 041 to README table |
| XI. Release Process | N/A | No release changes |
| XII. Prose Plugin | PENDING | Documentation must use prose plugin with cc-deck voice |
| XIII. XDG Paths | N/A | No path changes |
| XIV. No Dotfile Nesting | N/A | No dotfiles created |

## Project Structure

### Documentation (this feature)

```text
specs/041-workspace-channels/
├── plan.md              # This file
├── spec.md              # Feature specification
├── research.md          # Phase 0: research decisions
├── data-model.md        # Phase 1: entity model
├── quickstart.md        # Phase 1: consumer guide
├── contracts/
│   └── channel-interfaces.md  # Phase 1: behavioral contracts
├── checklists/
│   └── requirements.md  # Specification quality checklist
└── tasks.md             # Phase 2: task breakdown (via /speckit-tasks)
```

### Source Code (repository root)

```text
cc-deck/internal/ws/
├── interface.go         # MODIFY: add PipeChannel/DataChannel/GitChannel/ExecOutput to Workspace
├── channel.go           # NEW: channel interfaces, ChannelError, shared git remote helper
├── channel_pipe.go      # NEW: PipeChannel implementations (local, podman, k8s, ssh)
├── channel_data.go      # NEW: DataChannel implementations (local, podman, k8s, ssh)
├── channel_git.go       # NEW: GitChannel implementations (podman, k8s, ssh)
├── channel_test.go      # NEW: channel unit tests
├── errors.go            # MODIFY: add ChannelError type
├── container.go         # MODIFY: add channel accessors, delegate Push/Pull to DataChannel
├── compose.go           # MODIFY: add channel accessors, delegate Push/Pull to DataChannel
├── ssh.go               # MODIFY: add channel accessors, delegate Push/Pull/Harvest
├── k8s_deploy.go        # MODIFY: add channel accessors, delegate Push/Pull/Harvest
├── k8s_sync.go          # MODIFY: extract git helpers into channel_git.go, keep validation
├── local.go             # MODIFY: add channel accessors, implement local DataChannel/PipeChannel
└── factory.go           # UNCHANGED (channels are lazy-initialized, not factory-created)

cc-deck/internal/cmd/
├── ws.go                # MODIFY: verbose error display for ChannelError (FR-010)
└── flags.go             # UNCHANGED (Verbose flag already exists)

cc-deck/cmd/cc-deck/
└── main.go              # MODIFY: error formatting with ChannelError support

cc-zellij-plugin/src/
└── pipe_handler.rs      # MODIFY: add dispatch for new channel pipe names (if needed)
```

**Structure Decision**: All channel code lives in `cc-deck/internal/ws/` to access unexported workspace fields. Transport grouping (podman, k8s, ssh, local) is reflected in implementation structs, not file organization; files are organized by channel type for discoverability.

## Implementation Phases

### Phase 1: Foundation (channel interfaces, ChannelError, ExecOutput)

**Goal**: Define the channel type system without changing any existing behavior.

1. Create `channel.go` with:
   - `PipeChannel` interface (Send, SendReceive)
   - `DataChannel` interface (Push, Pull, PushBytes)
   - `GitChannel` interface (Fetch, Push)
   - Shared git remote lifecycle helper: `withTemporaryRemote(remoteName, remoteURL string, fn func() error) error`

2. Add `ChannelError` to `errors.go`:
   - Struct with Channel, Op, Workspace, Summary, Err fields
   - `Error()` returns Summary
   - `Unwrap()` returns Err
   - Constructor: `newChannelError(channel, op, workspace, summary string, err error) *ChannelError`

3. Add `ExecOutput(ctx context.Context, cmd []string) (string, error)` to `Workspace` interface in `interface.go`. Implement in each workspace type by exposing existing internal capabilities:
   - container: `podman.ExecOutput()`
   - compose: `podman.ExecOutput()`
   - ssh: `client.Run()`
   - k8s-deploy: `k8sExecOutput()`
   - local: return `ErrNotSupported`

4. Add channel accessor method signatures to `Workspace` interface:
   - `PipeChannel(ctx context.Context) (PipeChannel, error)`
   - `DataChannel(ctx context.Context) (DataChannel, error)`
   - `GitChannel(ctx context.Context) (GitChannel, error)`

5. Add stub implementations returning `ErrNotSupported` for all channel accessors in all workspace types (compiles, tests pass, no behavior change).

6. Tests: Unit tests for ChannelError (Error, Unwrap, errors.Is, errors.As).

### Phase 2: DataChannel (consolidate Push/Pull)

**Goal**: Implement DataChannel for all workspace types and refactor Push/Pull to delegate.

1. Implement transport-grouped DataChannel types in `channel_data.go`:
   - `localDataChannel`: filesystem copy via `os.CopyFile` or `io.Copy`
   - `podmanDataChannel`: wraps `podman.Cp()`, parameterized by container name func
   - `k8sDataChannel`: wraps tar-over-exec from k8s_sync.go, parameterized by ns/pod/kubeconfig
   - `sshDataChannel`: wraps `client.Rsync()`, parameterized by SSH client and host

2. Implement `PushBytes` for each type:
   - Podman/K8s: pipe bytes via stdin to `cat > remotePath` in Exec
   - SSH: pipe bytes via stdin over SSH
   - Local: `os.WriteFile`

3. Wire channel accessors: Each workspace type's `DataChannel()` returns the appropriate implementation (lazy-initialized via sync.Once).

4. Refactor Push/Pull methods on each workspace type to delegate to `DataChannel().Push()` / `DataChannel().Pull()`:
   - Container.Push/Pull -> podmanDataChannel
   - Compose.Push/Pull -> podmanDataChannel
   - SSH.Push/Pull -> sshDataChannel
   - K8sDeployWorkspace.Push/Pull -> k8sDataChannel
   - Local.Push/Pull -> localDataChannel (NEW: replaces ErrNotSupported)

5. Tests:
   - Unit tests for each DataChannel implementation (mock Exec/Cp)
   - Verify existing `make test` passes (no behavior regression for remote types)
   - Test local Push/Pull (new functionality)

### Phase 3: GitChannel (consolidate Harvest)

**Goal**: Implement GitChannel and refactor Harvest to delegate.

1. Extract shared git remote helper from k8s_sync.go into `channel_git.go`:
   - `withTemporaryRemote(ctx, remoteName, remoteURL string, fn func() error) error`
   - `gitExec(ctx, args...) error` (already exists, move here)

2. Implement transport-grouped GitChannel types in `channel_git.go`:
   - `podmanGitChannel`: ext:: URL with `podman exec -i <name> -- %S /workspace` (NEW)
   - `k8sGitChannel`: ext:: URL with `kubectl exec -i -n <ns> <pod> -- %S /workspace` (from k8s_sync.go)
   - `sshGitChannel`: `ssh://<host><workspace>` URL (from ssh.go)

3. Each implementation provides both Fetch and Push using the shared `withTemporaryRemote` helper.

4. Wire channel accessors: Each workspace type's `GitChannel()` returns the appropriate implementation:
   - Container -> podmanGitChannel
   - Compose -> podmanGitChannel
   - SSH -> sshGitChannel
   - K8sDeployWorkspace -> k8sGitChannel
   - Local -> return ErrNotSupported

5. Refactor Harvest methods to delegate to `GitChannel().Fetch()`:
   - SSH.Harvest -> sshGitChannel.Fetch
   - K8sDeployWorkspace.Harvest -> k8sGitChannel.Fetch
   - Container.Harvest -> podmanGitChannel.Fetch (NEW: replaces ErrNotSupported)
   - Compose.Harvest -> podmanGitChannel.Fetch (NEW: replaces ErrNotSupported)

6. Migrate k8sGitPush to k8sGitChannel.Push (used when SyncOpts.UseGit=true).

7. Clean up k8s_sync.go: remove migrated git code (k8sHarvest, k8sGitPush, gitExec). Keep k8sPush/k8sPull validation helpers if still used by k8sDataChannel.

8. Tests:
   - Unit tests for git remote lifecycle helper
   - Verify `make test` passes
   - Test container/compose Harvest (new functionality)

### Phase 4: PipeChannel

**Goal**: Implement PipeChannel for all workspace types.

1. Implement transport-grouped PipeChannel types in `channel_pipe.go`:
   - `localPipeChannel`: direct `exec.CommandContext(ctx, "zellij", "pipe", "--name", name, "--", payload)`
   - `execPipeChannel`: shared implementation for all remote types using `Exec(["zellij", "pipe", "--name", name, "--", payload])`, parameterized by the workspace's Exec method

2. SendReceive: return `ErrNotSupported` for all types (future implementation).

3. Wire channel accessors: Each workspace type's `PipeChannel()` returns:
   - Local -> localPipeChannel
   - All remote types -> execPipeChannel (wrapping the workspace's Exec)

4. Tests:
   - Unit tests for PipeChannel implementations (mock Exec)
   - Integration test: send pipe message to running local Zellij session (if feasible in CI)

### Phase 5: CLI Error Display and Polish

**Goal**: Wire ChannelError verbose display and update CLI harvest flags.

1. Add error formatting in `main.go`:
   - Intercept errors from `Execute()`
   - If `--verbose` and error is `ChannelError`: print full chain via `errors.Unwrap()`
   - Otherwise: print `err.Error()` (the human-readable summary)

2. Expose `--branch` and `--create-pr` flags on `cc-deck ws harvest` command (currently defined in HarvestOpts but not wired to CLI).

3. Update CLI help text for `ws push` and `ws pull` to mention local workspace support.

4. Tests: Verify verbose error output format.

### Phase 6: Documentation

**Goal**: Update all documentation per Constitution Principle IX.

1. Update README.md:
   - Add spec 041 to Feature Specifications table
   - Document channel architecture in the "How it works" section
   - Update Push/Pull/Harvest command descriptions to mention local workspace support

2. Update CLI reference (`docs/modules/reference/pages/cli.adoc`):
   - Add `--branch` and `--create-pr` flags to `ws harvest`
   - Update `ws push` and `ws pull` to note local workspace support

3. Use prose plugin for all documentation content.

## Complexity Tracking

No constitution violations requiring justification.

## Research Basis

Plan informed by parallel codebase research across 4 agents. See [research.md](research.md) for detailed decisions, rationale, and alternatives considered. Key discoveries that shaped the plan:

- Container/compose Push/Pull are near-identical (-> shared podmanDataChannel)
- K8s ext:: URL construction is duplicated between harvest and git push (-> shared helper)
- ExecOutput exists per-backend but not on interface (-> add to Workspace)
- ChannelError is the first custom error type (-> follows existing sentinel pattern)
- Local PipeChannel uses direct subprocess, not Exec() (-> separate implementation)
- Container/compose Harvest enabled by ext::podman exec (-> new functionality)
