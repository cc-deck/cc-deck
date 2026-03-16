# 23: Git-Based Code Sync and Harvest

**Date**: 2026-03-16
**Status**: brainstorm
**Feature**: Git workflow for code synchronization between host and containers
**Inspired by**: paude project (Ben Browning)

## Problem

When running Claude Code in a container (especially on remote Kubernetes clusters), getting code in and out is a core workflow challenge. Volume mounts work for local Podman but are unavailable for Kubernetes. The current cc-deck K8s sync uses `kubectl cp` via tar streaming (as implemented in `internal/sync/sync.go`), which works but does not preserve git history and lacks semantic understanding of code changes. A git-based approach provides efficient, incremental sync with full history preservation, meaningful diffs, and natural integration with pull request workflows.

## paude's Approach

The paude project demonstrates a git-native workflow for container-based development:

### Git `ext::` Protocol Tunneling

Git's external transport protocol allows git operations over arbitrary command streams. The remote URL format is:

```
ext::podman exec -i %S container-name git %G/repo
```

Where:
- `%S` is replaced by the remote name (origin, paude, etc.)
- `%G` is replaced by the git command (upload-pack, receive-pack)
- `git` inside the container handles the actual pack protocol over stdin/stdout

For Kubernetes:
```
ext::kubectl exec -i pod-name -n namespace -- git %G/repo
```

This allows seamless `git push` and `git fetch` operations without mounting volumes or copying files.

### Clone-From-Origin Optimization

Instead of pushing the entire repository from the host to the container, paude uses a two-stage approach:

1. Container clones directly from GitHub (datacenter bandwidth, fast)
2. Host pushes only deltas (local branch, uncommitted changes)

This is critical for large repositories on remote clusters where network bandwidth between host and cluster is limited.

### Receive Configuration

To allow pushing to a checked-out branch (normally disallowed by git), paude sets:

```
git config receive.denyCurrentBranch updateInstead
```

This makes the working tree update immediately when a push arrives, keeping container state synchronized with host pushes.

### Baseline Tracking

paude uses `refs/paude/base` to mark the initial sync point. This enables clean diffs showing only the work done by the agent:

```bash
git diff refs/paude/base..HEAD
```

### Harvest Workflow

When the agent completes a task, harvest retrieves the work:

1. Fetch from container: `git fetch paude main:paude/task-123`
2. Create local branch: `git checkout -b task-123 paude/task-123`
3. Show diff statistics: `git diff --stat main..task-123`
4. Optional PR creation: `gh pr create --base main --head task-123`

### Reset Workflow

To prepare the container for a new task:

```bash
git reset --hard origin/main
```

Clear conversation history (implementation-specific), then ready for next assignment.

### Protected Branch Validation

paude prevents harvesting onto protected branches (main, master, release-*) to avoid accidental overwrites.

## Decisions

| Question | Decision | Rationale |
|----------|----------|-----------|
| Sync mechanism (Podman) | Volume mounts (default), git sync (opt-in) | Volume mounts are simpler for local use; git sync available when isolation is needed |
| Sync mechanism (K8s) | Git ext:: protocol via kubectl exec | Only viable option without volume mounts; efficient incremental sync |
| Initial clone | Clone-from-origin on container, push deltas from host | Leverages datacenter bandwidth, avoids pushing entire repo over kubectl exec |
| Working branch | Agent works on detached HEAD or feature branch | Prevents conflicts with host branches |
| Harvest format | Local branch + diff summary + optional PR | Familiar git workflow, easy review before merge |
| History tracking | refs/cc-deck/base ref marker | Tracks sync baseline for clean diffs |
| Existing tar sync | Keep as fallback for non-git content | Git sync for repos, tar sync for configs/data files |

## Adaptation: Podman

- Volume mounts remain the default (bidirectional, simple, zero configuration)
- Git sync available as opt-in via `cc-deck deploy --git` or `cc-deck sync init --git`
- When using git sync: `ext::podman exec -i %S container-name git %G/repo`
- Useful when multiple agents work on the same repo without stepping on each other
- Example: one container per feature branch, isolated work-in-progress

## Adaptation: Kubernetes

- Git sync is the recommended option for code repositories (no volume mounts to host available)
- Existing tar-based sync (`cc-deck sync push/pull` in `internal/sync/sync.go`) remains available for non-git content
- `cc-deck sync init --git <session-name>` sets up ext:: remote via `kubectl exec`
- Clone-from-origin optimization is critical here (datacenter bandwidth between container and GitHub)
- Workflow: deploy creates PVC-backed workspace, container clones from origin, host pushes feature branch deltas
- `cc-deck sync push --git` and `cc-deck sync pull --git` for manual git operations
- Consider: automatic sync via file watcher or periodic background fetch

## Adaptation: OpenShift

- Same as Kubernetes but uses `oc exec` for ext:: protocol
- PVC-backed workspaces already exist in cc-deck's OpenShift support
- `oc rsync` as a fallback for non-git content (configs, data files)
- Integration with OpenShift's built-in git server (if available in the cluster)

## CLI Interface

### Git Sync Commands

```bash
# Initialize git sync for a session (sets up ext:: remote)
cc-deck sync init --git [session-name]

# Push local changes to container via git
cc-deck sync push --git [session-name]

# Pull container changes to host via git
cc-deck sync pull --git [session-name]

# Show git sync status (commits ahead/behind)
cc-deck sync status --git [session-name]
```

### Harvest Commands

```bash
# Fetch changes and show diff summary
cc-deck harvest [session-name]

# Fetch, create branch, and show diff
cc-deck harvest [session-name] --branch task-123

# Harvest and create PR
cc-deck harvest [session-name] --branch task-123 --pr

# Show harvest preview without creating branch
cc-deck harvest [session-name] --dry-run
```

### Reset Commands

```bash
# Reset container to clean state (git reset --hard)
cc-deck reset [session-name]

# Reset and clear conversation history
cc-deck reset [session-name] --clear-history

# Reset to specific ref
cc-deck reset [session-name] --ref origin/main
```

### Remote Management

```bash
# Show configured git remotes for container
cc-deck remote [session-name]

# Add a git remote in the container
cc-deck remote [session-name] add upstream https://github.com/org/repo

# Remove a git remote from the container
cc-deck remote [session-name] remove upstream
```

## Sidebar Integration

Visual indicators for git sync status:

- Branch name display next to session name
- Commits ahead/behind indicator (e.g., ↑3 ↓1)
- Uncommitted changes indicator (*)
- Dirty working tree indicator (!)

Quick actions:

- `Alt+h` in navigation mode: harvest current session
- `Alt+Shift+r`: reset current session
- Visual confirmation before destructive operations

## Implementation Details

### Container Setup

When git sync is initialized, the container receives:

```bash
# Inside container during deploy
git clone https://github.com/user/repo /workspace/repo
cd /workspace/repo
git config receive.denyCurrentBranch updateInstead
```

### Host Setup

On the host, add the ext:: remote:

```bash
# For Podman
git remote add cc-deck-session ext::podman exec -i %S container-name git %G/workspace/repo

# For Kubernetes
git remote add cc-deck-session ext::kubectl exec -i pod-name -n namespace -- git %G/workspace/repo
```

### Baseline Marker

After initial clone and first push:

```bash
# In container
git update-ref refs/cc-deck/base HEAD
```

This marks the starting point for diff calculations during harvest.

### Authentication for Private Repos

To support private repository clones in containers:

- SSH key forwarding via podman `--volume $SSH_AUTH_SOCK:/ssh-agent --env SSH_AUTH_SOCK=/ssh-agent`
- Kubernetes Secret with SSH private key, mounted in Pod
- GitHub personal access token in environment variable for HTTPS clones

### Conflict Handling

When `git push` to the container encounters conflicts:

1. Fetch current container state
2. Attempt automatic merge
3. If conflicts arise, abort and inform user
4. User resolves conflicts locally, pushes again

## Open Questions

- Should harvest automatically stash uncommitted changes in the container?
  (Probably yes, with `--unstash` option to restore them.)
- How to handle merge conflicts during `sync push --git`?
  (Fetch + merge locally, or abort and require manual resolution?)
- Should the clone-from-origin step support private repos via SSH keys in the container?
  (Yes, via Kubernetes Secrets or Podman volume mounts for ~/.ssh.)
- Performance: how does ext:: over kubectl exec compare to tar-based sync for typical repos?
  (Benchmark needed, but git pack protocol is likely more efficient for incremental updates.)
- Should reset also clear the Claude Code conversation, or just the git state?
  (Provide both options: `--git-only` and `--all`.)
- How to integrate with existing `internal/sync/sync.go` tar-based implementation?
  (Keep both: tar for non-git content, git for repositories.)
- Should sidebar show both tar sync and git sync status simultaneously?
  (Yes, if a session has both active. Git status takes precedence visually.)

## Integration with Existing Sync

The current `internal/sync/sync.go` implementation provides tar-based bidirectional sync:
- Push: tar local files, stream via kubectl exec, extract in Pod
- Pull: tar remote files in Pod, stream via kubectl exec, extract locally
- Excludes: `.git`, `node_modules`, `target`, `__pycache__`

Git sync complements this:
- Use git sync for repository code (preserves history, enables PR workflow)
- Use tar sync for non-git content (configs, data files, build artifacts)
- Both can coexist for the same session

The `cc-deck sync` command should detect repository context:
```bash
# In a git repo: default to git sync
cc-deck sync push my-session

# Force tar sync even in a git repo
cc-deck sync push my-session --tar

# In a non-git directory: automatic tar sync
cc-deck sync push my-session
```

## Success Metrics

- Time to sync a 100MB repository: <10 seconds for initial clone, <2 seconds for incremental pushes
- Harvest workflow creates PR-ready branches without manual file copying
- Zero data loss during sync operations (git reflog preserves all states)
- Git history preserved: meaningful commit messages from agent work
