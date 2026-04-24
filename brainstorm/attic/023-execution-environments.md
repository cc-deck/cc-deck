# Brainstorm: Execution Environment Abstraction

**Date:** 2026-03-19
**Status:** Brainstorm
**Depends on:** 022-multi-agent-support (agent interface)
**Dependency for:** Multi-agent credential transport, unified session lifecycle

## Core Concept

An "execution environment" is where a **full Zellij instance** runs, with the cc-deck sidebar plugin and one or more agent sessions inside it. The sidebar always shows sessions local to its own Zellij instance. There is no cross-environment visibility; you attach to one environment at a time.

```
┌─ Environment (local / podman / k8s-deploy / k8s-sandbox) ─┐
│                                                             │
│  ┌─ Zellij Instance ──────────────────────────────────────┐ │
│  │                                                         │ │
│  │  ┌─ Sidebar ─┐  ┌─ Tab 1 ─────┐  ┌─ Tab 2 ─────┐     │ │
│  │  │ session-1  │  │ claude      │  │ codex        │     │ │
│  │  │ session-2  │  │ (agent)     │  │ (agent)      │     │ │
│  │  │ session-3  │  │             │  │              │     │ │
│  │  └────────────┘  └─────────────┘  └──────────────┘     │ │
│  │                                                         │ │
│  │  Hook flow: agent -> cc-deck hook -> zellij pipe ->     │ │
│  │             sidebar (all within this Zellij instance)   │ │
│  └─────────────────────────────────────────────────────────┘ │
│                                                             │
│  Storage, credentials, networking managed by environment    │
└─────────────────────────────────────────────────────────────┘
```

The cc-deck CLI manages the lifecycle of these environments from the host: create, attach, stop, delete, sync files, inject credentials. Once attached, the user is inside a self-contained Zellij session.

## Proposed Spec Split

This brainstorm covers a large surface area. It should be split into 4 focused specs:

| Spec | Scope | Priority |
|---|---|---|
| **023: Environment Interface + CLI** | Go interface definitions, unified `cc-deck env` commands, `state.yaml` persistence, status display, local environment (thin wrapper) | High (foundation) |
| **024: K8s Environment Refactor** | Refactor existing deploy/connect/delete behind Environment interface, add start/stop, add git harvesting sync, backward-compatible aliases | Medium (reorganization) |
| **025: Podman Environment** | New PodmanEnvironment implementation, named volumes + bind mounts, git harvesting via `ext::podman exec`, credential injection, image pipeline integration | High (locally testable) |
| **026: K8s Sandbox** | Ephemeral Pods with emptyDir, strict NetworkPolicy, auto-delete, copy-only sync, result extraction | Lower |

Storage and sync are embedded in each environment spec rather than standalone. The interfaces are defined in 023, implementations live where they are used.

**TUI for environment management** is deferred to a separate brainstorm (see `brainstorm/024-tui-environment-manager.md`).

**Port forwarding** (`cc-deck env port-forward`) is deferred to a future spec.

### Dependency Graph

```
023 (Interface + CLI)
 ├── 025 (Podman, highest priority, testable locally)
 ├── 024 (K8s Refactor, reorganize existing code)
 │    └── 026 (K8s Sandbox, reuses K8s primitives)
 └── (future) TUI Environment Manager

022 (Multi-Agent) ── parallel track, ties into all via Agent interface
```

### Implementation Order

1. **023: Interface + CLI + State** (foundation, all others depend on this)
2. **025: Podman Environment** (highest priority, enables local testing without a cluster)
3. **024: K8s Refactor** (reorganize existing working code behind the interface)
4. **026: K8s Sandbox** (new capability, builds on K8s primitives from 024)

## Problem Statement

cc-deck currently has two separate, incompatible code paths:

1. **Local:** User runs Zellij on the host, manually starts agents. No lifecycle management by cc-deck.
2. **K8s:** `cc-deck deploy` creates a StatefulSet with an image that contains Zellij + agents. Connect via `kubectl exec`. Managed lifecycle but separate code path.

There is no Podman container mode (despite a full image build pipeline), no unified interface, storage strategy is hardcoded (PVC via StatefulSet), and the sync mechanism is limited to tar-over-exec. Adding a new environment type requires touching multiple packages.

## Current Architecture (What Exists)

### Local
- User starts Zellij, plugin sidebar loads via layout
- Hook events flow locally: agent -> `cc-deck hook` -> `zellij pipe` -> plugin
- No cc-deck lifecycle management, no state tracking

### K8s Deployment
- `cc-deck deploy` creates: StatefulSet (replicas=1), headless Service, PVC (via volumeClaimTemplates), ConfigMap, NetworkPolicy, optional EgressFirewall
- Connect via exec, web, or port-forward into the container's Zellij
- Sync via `kubectl exec + tar` (push/pull)
- Profiles define credentials (Anthropic API key or Vertex AI)

### Container Image Pipeline
- Base image (Fedora + dev tools) + demo image (base + cc-deck + agent + Zellij)
- Multi-arch build via Podman/Makefile
- Image is a complete, self-contained Zellij environment

## K8s Workload Type: Keep StatefulSet

The current K8s implementation uses a StatefulSet. After code analysis, the recommendation is to **keep StatefulSet** for K8s deploy environments rather than switching to Deployment + PVC.

### Why StatefulSet Stays

The existing code relies heavily on StatefulSet's predictable naming:
- `connect.go:63`: constructs pod name directly as `cc-deck-<sessionName>-0`
- `delete.go:44`: constructs PVC name as `data-cc-deck-<sessionName>-0`

| Aspect | StatefulSet (current, keep) | Deployment + PVC (rejected) |
|---|---|---|
| Pod naming | Predictable: `cc-deck-myenv-0` | Random: `cc-deck-myenv-7f8d9-abc12` |
| PVC creation | Automatic via `volumeClaimTemplates` | Separate API call needed |
| Connect code | Direct name construction (simple) | Label-selector pod discovery (more complex) |
| PVC naming | Automatic: `data-cc-deck-myenv-0` | Manual naming required |
| emptyDir support | Not needed (K8s deploy always uses PVC) | More natural |

Switching to Deployment would require refactoring the connect code to use label selectors for pod discovery, adding separate PVC creation/deletion logic, with no meaningful gain for a single-replica workload.

The emptyDir use case is handled by K8s Sandbox (bare Pod), which is a separate environment type. K8s deploy environments always use PVC for persistent storage, making StatefulSet's `volumeClaimTemplates` a perfect fit.

## Multi-User Attach (Pair-SDD)

Zellij has built-in multiplayer support since v0.23.0. Multiple users can attach to the same Zellij session simultaneously, each with their own colored cursor (Google Docs-style collaborative editing in the terminal). This enables **pair-SDD**: two or more developers collaborating inside the same cc-deck environment.

### How It Works Per Environment

| Environment | Multi-user attach mechanism |
|---|---|
| Local | Second terminal runs `zellij attach <session-name>` |
| Podman | Second terminal runs `podman exec -it <name> zellij attach <session-name>` |
| K8s Deploy | Second terminal runs `kubectl exec` into same pod, then `zellij attach` |
| K8s Deploy (web) | Second browser tab opens the same Zellij web client URL |

`cc-deck env attach <name>` naturally supports concurrent users. Each call opens a new terminal connection to the same Zellij session inside the environment. No special handling needed in cc-deck, this is a Zellij-native capability.

### Pair-SDD Workflow

```bash
# Developer A creates and attaches
cc-deck env create my-project --type podman \
  --image quay.io/cc-deck/cc-deck-demo:latest
cc-deck env attach my-project

# Developer B attaches to the same environment (separate terminal)
cc-deck env attach my-project
# Both see the same sidebar, same tabs, each has their own cursor color
```

For remote collaboration, Developer B would SSH into the same machine and run `cc-deck env attach`, or use the Zellij web client if the environment exposes port 8082.

### Security Note

All users attached to the same environment share the same Unix user context inside the container/pod. There is no per-user permission isolation within a Zellij session. For cross-user sharing on the same host, Zellij supports a `ZELLIJ_SOCKET_GROUP` environment variable (PR [#3406](https://github.com/zellij-org/zellij/pull/3406)) to allow users in the same Unix group to share sessions.

## Storage Abstraction

### Storage Backends

Not every storage backend makes sense for every environment. The table below shows valid combinations:

| Backend | Local | Podman | K8s Deploy | K8s Sandbox |
|---|---|---|---|---|
| **Host filesystem** | Default (native) | Via bind mount (`-v`) | N/A | N/A |
| **Named volume** | N/A | `podman volume` (default) | N/A | N/A |
| **emptyDir** | N/A | N/A | N/A | Default |
| **PVC** | N/A | N/A | Default (via StatefulSet) | Optional |

### Storage Interface

```go
type StorageBackend interface {
    Type() StorageType           // HostPath, NamedVolume, EmptyDir, PVC
    Provision(opts StorageOpts) error
    Cleanup() error
    // Returns volume spec for the environment to mount
    VolumeSpec() interface{}     // podman -v flag, K8s Volume+VolumeMount, etc.
}

type StorageOpts struct {
    Size         string    // "10Gi" (PVC, named volume)
    StorageClass string    // K8s storage class (PVC only)
    Path         string    // host path (bind mount only)
    ReadOnly     bool
}
```

### Per-Environment Defaults

```yaml
# ~/.config/cc-deck/config.yaml (or per-profile)
defaults:
  storage:
    podman: named-volume    # or: host-path
    k8s: pvc                # always PVC (via StatefulSet volumeClaimTemplates)
    sandbox: empty-dir      # ephemeral by default
    podman-size: 20Gi
    k8s-size: 10Gi
```

### Storage Lifecycle

| Operation | Host Path | Named Volume | emptyDir | PVC (StatefulSet) |
|---|---|---|---|---|
| Create | Mkdir | `podman volume create` | Auto (Pod spec) | Auto (volumeClaimTemplates) |
| Stop env | Persists | Persists | **Lost** (Pod deleted) | Persists |
| Delete env | Persists (user's fs) | Optional cleanup | Lost | Optional cleanup |
| Resize | N/A | N/A | N/A | PVC expansion (if StorageClass supports) |

## Sync Strategies

Data transfer between the host and the environment. Multiple strategies exist, with different trade-offs:

### Strategy 1: Copy (tar/cp)

The current mechanism. Direct file transfer via exec pipe.

```
Host                          Environment
  |                               |
  |-- tar cf - <dir> --pipe-->   tar xf - -C /workspace
  |                               |
  |   tar xf - -C <dir>  <--pipe-- tar cf - -C /workspace
  |                               |
```

| Aspect | Details |
|---|---|
| Implementation | Podman: `podman cp`. K8s: `kubectl exec + tar` (existing code) |
| Pros | Simple, works everywhere, no git needed, handles binary files |
| Cons | Full copy each time (no delta), slow for large repos, no history |
| Best for | Initial code push, result extraction, non-git projects |

### Strategy 2: Git Harvesting (paude-style)

Uses git's `ext::` protocol to tunnel git operations over exec, treating the container as a git remote. No network access needed.

```
Host                                    Environment
  |                                         |
  |  git remote add env                     |
  |    "ext::podman exec -i <name> %S       |
  |     /workspace"                         |
  |                                         |
  |-- git push env <branch> ----exec-->    (receives push, updates worktree)
  |                                         |
  |                                         |  Agent works, makes commits
  |                                         |
  |   git fetch env           <--exec--    (sends commits back)
  |-- git checkout -B harvest               |
  |     env/<branch>                        |
  |                                         |
```

| Aspect | Details |
|---|---|
| Implementation | `git remote add` with `ext::podman exec -i` or `ext::kubectl exec -i` URL |
| Setup | Init bare-ish repo in container, set `receive.denyCurrentBranch updateInstead` |
| Pros | Delta-only transfers, full git history preserved, merge/review via normal git workflow, no network needed |
| Cons | Requires git in image, only works for git repos, initial clone can be slow |
| Best for | Normal development workflow, code review via PRs, long-running environments |
| Inspired by | [paude](https://github.com/rhuss/paude) git remote mechanism |

**Key paude patterns to adopt:**
- `refs/cc-deck/base` reference to mark the initial push point (for clean diffs)
- Clone-from-origin optimization: if container can reach the git remote, clone there first, then push only local-only commits as delta
- Protected branch list: prevent harvesting to `main`, `master`, `release/*`
- `cc-deck env harvest` command for pulling changes back as a local branch

### Strategy 3: Remote Git Repository

Both host and environment interact with a shared remote git repository (GitHub, GitLab, etc.). No direct exec-based transfer.

```
Host                     Remote Repo              Environment
  |                          |                        |
  |-- git push -------->    |                        |
  |                          |    <---- git pull -----|
  |                          |                        |
  |                          |                        |  Agent works, commits
  |                          |                        |
  |                          |    <---- git push -----|
  |   git pull  <--------   |                        |
  |                          |                        |
```

| Aspect | Details |
|---|---|
| Implementation | Standard git push/pull via HTTPS or SSH |
| Pros | Works across any environment, no exec needed, natural git workflow, enables async collaboration |
| Cons | Requires network access to git remote (conflicts with strict sandbox egress), requires git credentials in environment, exposes work-in-progress to remote |
| Best for | K8s environments where exec is unreliable, multi-user scenarios, CI/CD integration |
| Security concern | Agent needs git push credentials, which could be abused in sandbox mode |

### Sync Strategy Compatibility Matrix

| Strategy | Local | Podman | K8s Deploy | K8s Sandbox |
|---|---|---|---|---|
| **Copy (tar/cp)** | N/A | Yes (`podman cp`) | Yes (`kubectl exec + tar`) | Yes (push only recommended) |
| **Git Harvesting** | N/A | Yes (`ext::podman exec`) | Yes (`ext::kubectl exec`) | Possible but awkward (ephemeral) |
| **Remote Git** | N/A | Yes (if network allowed) | Yes (if egress allows git host) | No (strict egress) |

### Default Sync Strategy per Environment

| Environment | Default | Rationale |
|---|---|---|
| Local | None needed | Already on host filesystem |
| Podman | Git harvesting | Local container, exec always available, delta transfers |
| K8s Deploy | Git harvesting | Long-running, delta transfers essential, exec available |
| K8s Sandbox | Copy (push at create) | Ephemeral, no git history needed, strict network |

### Sync Configuration

```yaml
# Per-environment override in config
environments:
  - name: my-project
    type: k8s
    sync:
      strategy: git-harvest        # or: copy, remote-git
      workspace: /workspace        # remote working directory
      excludes:                    # for copy strategy
        - .git
        - node_modules
        - target
      remote-git:                  # for remote-git strategy
        url: git@github.com:org/repo.git
        branch: agent-work
```

### Sync CLI Commands

```bash
# Copy strategy
cc-deck env push <name> [local-path]           # push files to environment
cc-deck env pull <name> [remote-path]           # pull files from environment

# Git harvesting
cc-deck env push <name> --git                   # git push to environment via ext::
cc-deck env harvest <name> [-b <branch>]        # fetch agent's commits, create local branch
cc-deck env harvest <name> --pr                 # harvest + create PR
cc-deck env reset <name>                        # reset environment workspace to origin

# Remote git (mostly manual, cc-deck just triggers)
cc-deck env sync <name> --pull                  # git pull inside environment
cc-deck env sync <name> --push                  # git push inside environment
```

## Environment Status and Session Tracking

### Local State File

All environments are tracked in `~/.config/cc-deck/state.yaml` (separate from `config.yaml` to keep config clean):

```yaml
environments:
  - name: local-default
    type: local
    created_at: 2026-03-19T10:00:00Z
    last_attached: 2026-03-19T15:30:00Z

  - name: my-project
    type: podman
    created_at: 2026-03-19T11:00:00Z
    last_attached: 2026-03-19T14:00:00Z
    storage:
      type: named-volume
      volume_name: cc-deck-my-project
    sync:
      strategy: git-harvest
      remote_name: cc-deck-my-project
      base_ref: refs/cc-deck/base
      last_push: 2026-03-19T11:05:00Z
      last_harvest: 2026-03-19T14:30:00Z
    podman:
      container_id: abc123def456
      container_name: cc-deck-my-project
      image: quay.io/cc-deck/cc-deck-demo:latest
      ports: ["8082:8082"]

  - name: backend-work
    type: k8s
    created_at: 2026-03-19T12:00:00Z
    last_attached: 2026-03-19T16:00:00Z
    storage:
      type: pvc
      pvc_name: data-cc-deck-backend-work-0
      size: 10Gi
      storage_class: gp3
    sync:
      strategy: git-harvest
      remote_name: cc-deck-backend-work
      base_ref: refs/cc-deck/base
    k8s:
      namespace: cc-deck
      statefulset: cc-deck-backend-work
      profile: anthropic-prod
      kubeconfig: ~/.kube/config

  - name: eval-run-42
    type: sandbox
    created_at: 2026-03-19T13:00:00Z
    storage:
      type: empty-dir
    sync:
      strategy: copy
      last_push: 2026-03-19T13:01:00Z
    sandbox:
      namespace: cc-deck-sandbox
      pod_name: eval-run-42
      profile: anthropic-eval
      kubeconfig: ~/.kube/config
      expires_at: 2026-03-19T17:00:00Z
```

### Status Summary

Getting "is the environment still active" is straightforward (check container/pod status). The harder question is: "what are the agent sessions doing inside?"

**What we can know without attaching:**

| Data point | Local | Podman | K8s |
|---|---|---|---|
| Environment running? | Check Zellij process | `podman inspect` | K8s API (Pod status) |
| Pod/container uptime | Process uptime | Container started_at | Pod start time |
| Resource usage | N/A | `podman stats` | K8s metrics API |
| Agent process running? | `pgrep claude` | `podman exec pgrep claude` | `kubectl exec pgrep claude` |

**What we cannot know without exec:**
- Individual agent session states (Working, Permission, Done, etc.)
- Which sessions need attention
- Session names, git branches

The session states live inside the Zellij plugin's WASI cache (`/cache/sessions.json`). To read them from outside requires exec into the environment.

**`cc-deck env list` (fast, no exec):**

```
NAME            TYPE      STATUS    AGENTS    STORAGE     LAST ATTACHED    AGE
local-default   local     running   claude    host        5m ago           3d
my-project      podman    running   claude    volume      2h ago           1d
backend-work    k8s       running   claude    pvc/10Gi    30m ago          5d
eval-run-42     sandbox   running   claude    emptyDir    never            2h
old-project     podman    stopped   claude    volume      3d ago           7d
```

**`cc-deck env status <name>` (detailed, uses exec):**

```
Environment: backend-work
Type:        k8s
Status:      Running
Storage:     PVC (10Gi, gp3)
Sync:        git-harvest (last push: 2h ago, last harvest: 30m ago)
Namespace:   cc-deck
Profile:     anthropic-prod
Uptime:      5d 3h
Attached:    30m ago

Agent Sessions (from container):
  NAME              STATUS        BRANCH          LAST EVENT
  api-refactor      ⚠ Permission  feat/api-v2     2m ago
  docs-update       ● Working     docs/quickstart 1m ago
  bugfix-123        ✓ Done        fix/null-ptr    15m ago
```

### Status Reconciliation

`cc-deck env list` reconciles local records with actual state:
- Podman: `podman inspect` to check container exists and is running
- K8s: check Pod status via K8s API
- Remove stale records (container/pod deleted externally)
- Update status (running, stopped, error)

## Design: The Environment Interface

```go
// Environment manages the lifecycle of a Zellij instance
// running in a specific execution context.
type Environment interface {
    // Identity
    Type() EnvironmentType       // Local, Podman, K8sDeploy, K8sSandbox
    Name() string                // user-chosen name

    // Lifecycle
    Create(opts CreateOpts) error
    Start() error                // resume a stopped environment
    Stop() error                 // stop without destroying (preserves state)
    Delete() error               // destroy and clean up all resources
    Status() (EnvironmentStatus, error)

    // Interaction
    Attach() error               // interactive terminal into the Zellij instance
    Exec(cmd string) error       // run a command inside the environment

    // Data transfer (strategy-aware)
    Push(opts SyncOpts) error    // transfer data into the environment
    Pull(opts SyncOpts) error    // transfer data out of the environment
    Harvest(opts HarvestOpts) error  // git harvest (returns error if not git strategy)
}
```

### CreateOpts

```go
type CreateOpts struct {
    // Common
    Image       string            // OCI image (ignored for local)
    Agents      []string          // agents to expect/verify
    Credentials CredentialSet     // API keys, tokens

    // Storage
    Storage     StorageConfig     // backend + options

    // Sync
    Sync        SyncConfig        // strategy + initial sync dir

    // Resources
    Resources   ResourceSpec      // CPU/memory limits

    // K8s-specific
    Namespace   string
    Profile     string
    Kubeconfig  string
    Egress      EgressPolicy
    AllowEgress []string

    // Podman-specific
    Ports       []PortMapping
    ExtraVolumes []VolumeMount    // additional bind mounts beyond storage
}

type StorageConfig struct {
    Type         StorageType      // HostPath, NamedVolume, EmptyDir, PVC
    Size         string           // "10Gi"
    StorageClass string           // K8s only
    HostPath     string           // for bind mounts
}

type SyncConfig struct {
    Strategy     SyncStrategy     // Copy, GitHarvest, RemoteGit
    InitialDir   string           // local directory to push on creation
    Workspace    string           // remote path (default: /workspace)
    Excludes     []string         // for copy strategy
    GitRemoteURL string           // for remote-git strategy
    GitBranch    string           // for remote-git strategy
}
```

### EnvironmentStatus

```go
type EnvironmentStatus struct {
    State        EnvironmentState  // Running, Stopped, Creating, Error, Unknown
    Since        time.Time
    Message      string            // error details if State == Error
    Agents       []string          // agents detected
    Connection   ConnectionType    // how Attach() will connect
    Storage      StorageConfig
    Sync         SyncConfig
    // Optional: populated via exec if environment is running
    Sessions     []SessionInfo     // agent sessions inside (from /cache/sessions.json)
}

type SessionInfo struct {
    Name     string
    Activity string    // "Working", "Permission", "Done", etc.
    Branch   string
    LastEvent time.Time
}
```

## Environment Types

### 1. Local

The user's host machine. Zellij is already running. Implemented as a thin wrapper in spec 023 (no separate spec needed).

| Method | Implementation |
|---|---|
| Create | Start Zellij with cc-deck layout if not running |
| Start/Stop | No-op |
| Delete | Optionally kill Zellij session |
| Attach | No-op (already there) |
| Push/Pull | No-op (already local) |
| Storage | Host filesystem (implicit) |
| Sync | Not needed |

### 2. Podman Container

A local container with its own Zellij instance.

| Method | Implementation |
|---|---|
| Create | `podman run -d` with storage volume, secrets, ports |
| Start/Stop | `podman start/stop` |
| Delete | `podman rm` + optional volume cleanup |
| Attach | `podman exec -it <name> zellij attach --create` |
| Exec | `podman exec <name> <cmd>` |
| Push/Pull | Copy: `podman cp`. Git: `ext::podman exec -i <name> %S /workspace` |
| Storage | Named volume (default) or host bind mount |
| Credentials | Podman secrets or env vars |

### 3. K8s Deploy

Persistent workload using **StatefulSet** (replicas=1) with PVC via `volumeClaimTemplates`.

| Method | Implementation |
|---|---|
| Create | StatefulSet + headless Service + ConfigMap + NetworkPolicy (existing code) |
| Start | Scale StatefulSet replicas to 1 |
| Stop | Scale StatefulSet replicas to 0 (PVC persists) |
| Delete | Delete StatefulSet + Service + ConfigMap + NetworkPolicy. PVC optionally preserved. |
| Attach | Auto-detect: exec (default), web (OpenShift Route), port-forward |
| Exec | `kubectl exec -it <pod> -- <cmd>` (predictable pod name: `cc-deck-<name>-0`) |
| Push/Pull | Copy: `kubectl exec + tar`. Git: `ext::kubectl exec -i <pod> -c <container> -- %S /workspace` |
| Storage | PVC (always, via StatefulSet volumeClaimTemplates) |
| Credentials | K8s Secrets via profile system |

### 4. K8s Sandbox

Ephemeral, restricted.

| Method | Implementation |
|---|---|
| Create | Pod (not StatefulSet), emptyDir, strict NetworkPolicy |
| Start/Stop | Not supported (ephemeral) |
| Delete | Delete Pod + NetworkPolicy |
| Attach | `kubectl exec` only |
| Push/Pull | Copy only (push at create, pull before delete) |
| Storage | emptyDir (default). PVC optional for longer-running sandboxes. |
| Credentials | Minimal (AI backend key only) |

## Credential Transport

See 022-multi-agent-support for per-agent credential requirements.

### Per-Environment Injection

| Environment | Mechanism |
|---|---|
| Local | Shell env (already available) |
| Podman | `podman secret create` + `--secret`, or `-e KEY=val` |
| K8s Deploy | K8s Secrets via `envFrom` or `env.valueFrom.secretKeyRef` |
| K8s Sandbox | K8s Secrets (restricted set) |

### Credential Resolution Order

1. Explicit `--credential` or `--profile` flag
2. Default profile in config
3. Host environment variable
4. Interactive prompt (local/Podman only)

## Unified CLI Surface

### Commands

`--type` is always required for `create`. No auto-inference for now. A future project layout config file (`cc-deck.yaml` or similar) could provide per-project defaults for environment type, image, agents, sync strategy, and initial directories to transfer. This intersects with the existing `cc-deck-build.yaml` manifest and needs its own design pass.

```bash
# Environment lifecycle
cc-deck env create <name> --type <local|podman|k8s|sandbox> [options]
cc-deck env attach <name>
cc-deck env start <name>
cc-deck env stop <name>
cc-deck env delete <name>
cc-deck env list [--type <type>]
cc-deck env status <name>             # detailed, reads session state from inside

# Data transfer
cc-deck env push <name> [local-path]             # copy strategy
cc-deck env pull <name> [remote-path]             # copy strategy
cc-deck env push <name> --git                     # git push via ext::
cc-deck env harvest <name> [-b <branch>] [--pr]   # git fetch + local branch [+ PR]
cc-deck env reset <name>                          # reset workspace to origin

# Convenience
cc-deck env exec <name> -- <cmd>
cc-deck env logs <name>

# Profiles (unchanged)
cc-deck profile add|list|use|delete

# Plugin (unchanged)
cc-deck plugin install [--agents claude,codex]

# Image pipeline (unchanged)
cc-deck image init|verify|diff
```

### Example Workflows

**Local development (implicit, current behavior):**
```bash
zellij --layout cc-deck        # start Zellij with sidebar
# Open tabs, run agents manually
```

**Podman container with git sync:**
```bash
cc-deck env create my-project --type podman \
  --image quay.io/cc-deck/cc-deck-demo:latest \
  --sync git-harvest --sync-dir ./my-project
# Pushes code via ext::podman exec, sets up git remote

cc-deck env attach my-project
# Work inside container's Zellij, agents make commits

cc-deck env harvest my-project -b feature/agent-work --pr
# Fetches commits, creates local branch, opens PR
```

**Pair-SDD on Podman:**
```bash
# Developer A
cc-deck env attach my-project

# Developer B (separate terminal, same machine)
cc-deck env attach my-project
# Both see the same Zellij session with independent cursors
```

**K8s deployment with PVC:**
```bash
cc-deck env create backend --type k8s \
  --profile anthropic-prod --storage-size 20Gi \
  --sync git-harvest --sync-dir ./backend-service

cc-deck env attach backend
# kubectl exec into Zellij (pod name: cc-deck-backend-0)

cc-deck env stop backend    # scale to 0, PVC preserved
cc-deck env start backend   # scale to 1, data still there
```

**K8s sandbox for evaluation:**
```bash
cc-deck env create eval-42 --type sandbox \
  --profile anthropic-eval \
  --sync copy --sync-dir ./benchmark-suite

cc-deck env attach eval-42
# Run agent task

cc-deck env pull eval-42 /workspace/results ./results/eval-42
cc-deck env delete eval-42
```

### Backward Compatibility

Keep existing commands as aliases during transition:
- `cc-deck deploy` -> `cc-deck env create --type k8s`
- `cc-deck connect` -> `cc-deck env attach`
- `cc-deck delete` -> `cc-deck env delete`
- `cc-deck sync` -> `cc-deck env push` / `cc-deck env pull`
- `cc-deck list` -> `cc-deck env list`
- `cc-deck logs` -> `cc-deck env logs`

## Why Not OSC Escape Sequences

Terminal escape sequences (OSC 9/99/777) were investigated as an alternative to hook-based agent detection. Ruled out for two reasons:

1. **Zellij does not support OSC notification sequences.** OSC 99 is an open issue ([zellij#3451](https://github.com/zellij-org/zellij/issues/3451)) since June 2024.
2. **Zellij's plugin API cannot observe pane output.** The `zellij-tile 0.43` API has no event for reading raw terminal data from other panes.

The hook-based approach is always local to the Zellij instance, so it works identically in all environments.

## Open Questions

1. **`state.yaml` vs `config.yaml`:** Should environment tracking be in a separate state file or in the existing config? Recommendation: separate `state.yaml` because environment state changes frequently (last_attached, container_id) while config is user-edited.

2. **Git harvesting in sandboxes:** Should sandboxes support git harvesting at all? It adds complexity for environments designed to be ephemeral. Counter-argument: some sandbox tasks produce meaningful code that should be reviewed via PR.

3. **Storage migration:** Can a running environment switch storage backends? Probably not without recreating the environment. Acceptable for now.

4. **Exec latency for status:** Reading session state via exec adds 1-2 seconds per environment. Recommendation: `list` shows only environment-level status (fast), `status <name>` reads session details (exec, slower).

5. **Clone-from-origin optimization:** When the container can reach the git remote (allowed in egress), should `cc-deck env push --git` clone from origin inside the container and only push local-only commits? Saves significant time for large repos.

6. **Podman rootless auto-detection:** Rootless Podman uses different socket paths. Recommendation: auto-detect via `podman info --format '{{.Host.RemoteSocket.Path}}'`.

7. **Project layout config:** Should there be a `cc-deck.yaml` per-project file that defines default environment type, image, agents, sync strategy, and initial projects to transfer? This would reduce CLI flag verbosity and enable repeatable setups. Intersects with `cc-deck-build.yaml` manifest. Deferred to future design.

8. **Multi-user security model:** When multiple users attach to the same environment, they share the same Unix user context. Is this acceptable for all use cases? Should we document security recommendations for pair-SDD?
