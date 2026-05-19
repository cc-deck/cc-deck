# Brainstorm: Git Harvest Sync Strategy

**Date:** 2026-03-20
**Status:** Brainstorm (deferred)
**Depends on:** 025-container-environment (container type), 023-env-interface (sync interface)
**Applies to:** container, compose, k8s-deploy environment types

## Context

The `ext::` git transport protocol enables delta-only file synchronization between the host and any environment that supports exec. This is more efficient than copy-based sync for iterative development workflows where only a few files change between syncs.

This brainstorm was split from 025-container-environment to keep that spec focused on copy-based sync (`podman cp`). Git harvest applies uniformly across all remote environment types and deserves its own spec.

## Concept: paude-style Git Harvesting

Uses git's `ext::` protocol to tunnel git operations over exec (podman exec, kubectl exec), treating the environment as a git remote. No network access needed inside the environment.

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

### Key Patterns (from paude)

- `refs/cc-deck/base` reference to mark the initial push point (for clean diffs)
- Clone-from-origin optimization: if container can reach the git remote, clone there first, then push only local-only commits as delta
- Protected branch list: prevent harvesting to `main`, `master`, `release/*`
- Harvest creates a local branch, optionally opens a PR

### ext:: URL per Environment Type

| Type | ext:: URL |
|------|-----------|
| container | `ext::podman exec -i cc-deck-<name> %S /workspace` |
| compose | `ext::podman exec -i cc-deck-<name> %S /workspace` |
| k8s-deploy | `ext::kubectl exec -i cc-deck-<name>-0 -c session -- %S /workspace` |

### CLI Commands

```bash
# Push code to environment via git
cc-deck env push <name> --git

# Harvest agent's commits back as a local branch
cc-deck env harvest <name> [-b <branch-name>]

# Harvest and create a PR
cc-deck env harvest <name> --pr

# Reset environment workspace to match a branch
cc-deck env reset <name> [--branch main]
```

### Setup Inside Environment

On first `push --git`, cc-deck sets up the environment's workspace:

```bash
# Inside the container (via exec):
cd /workspace
git init
git config receive.denyCurrentBranch updateInstead
```

### Sync Strategy in Environment Definition

```yaml
# environments.yaml
environments:
  - name: my-project
    type: container
    sync:
      strategy: git-harvest
      workspace: /workspace
      protected-branches: [main, master, "release/*"]
```

### Comparison with Copy Sync

| Aspect | Copy (`podman cp`) | Git Harvest (`ext::`) |
|--------|-------------------|----------------------|
| Transfer size | Full directory each time | Delta only |
| History | No | Full git history preserved |
| Merge workflow | Manual | Normal git (branches, PRs) |
| Binary files | Yes | Yes (but bloats repo) |
| Setup | None | Git init in container |
| Speed (first sync) | Fast | Slower (full push) |
| Speed (subsequent) | Same (full copy) | Fast (delta only) |

## Updated Learnings from paude (May 2026)

Source: `bbrowning/paude` v0.15.0, specifically `src/paude/workflow.py` and `src/paude/domains.py`.

### Unmerged Work Safety Check

paude's `_check_unmerged_work()` uses `git merge-base --is-ancestor HEAD origin/<branch>` before allowing a reset. If the container's HEAD is not an ancestor of origin/main, it warns the user and exits unless `--force` is passed. This prevents accidental loss of unharvested work.

cc-deck should adopt this pattern for `cc-deck env reset`:

```bash
# Inside container via exec:
git fetch origin 2>/dev/null
git merge-base --is-ancestor HEAD origin/main
# Exit code 0 = safe to reset (HEAD already in main)
# Exit code 1 = unharvested work exists, warn user
```

### Protected Branch Pattern Matching

paude uses `fnmatch` against a frozen set of patterns: `main`, `master`, `release`, `release-*`, `release/*`. This is more robust than exact string matching and catches branch naming variations.

cc-deck's protected branch list should be configurable in the environment definition, with these as defaults:

```yaml
sync:
  protected-branches:
    - main
    - master
    - "release-*"
    - "release/*"
```

### Harvest with PR Creation

paude's harvest workflow includes an integrated PR path:

1. `git fetch origin` to update refs for `--force-with-lease`
2. `git push --force-with-lease -u origin <branch>` (safe force push)
3. Check if an open PR already exists for this branch using `gh pr list --head <branch> --state open`
4. If PR exists, report "PR already exists and updated"
5. If no PR, create one with `gh pr create --head <branch>`

The `--force-with-lease` is important: it prevents overwriting remote changes that someone else may have pushed to the same branch, while still allowing updates to the harvested branch.

cc-deck should support the same flow: `cc-deck harvest <name> --pr` creates or updates a PR, and `--pr-title` sets the title.

### Concurrent Session Enrichment

paude's `status_sessions()` uses `ThreadPoolExecutor(max_workers=8)` to concurrently query running sessions for activity and work summaries. Each session gets an `exec_in_session` call to gather git status, branch info, and recent commit summaries. Results are sorted by elapsed time (most recently active first).

For cc-deck's sidebar or a future `cc-deck ws status` command, this pattern is valuable when managing many concurrent workspaces. The Zellij plugin could request workspace git status via pipe messages, with the Go CLI handling concurrent exec calls and returning aggregated results.

### Session Reset with Conversation Clearing

paude's `reset_session()` does more than just git reset. After resetting the workspace, it optionally clears the agent's conversation history:

1. Deletes `.jsonl` conversation files and `sessions-index.json`
2. Removes todo directories
3. Preserves per-project settings (`settings.local.json`, `CLAUDE.md`)
4. Sends a `/clear` command via tmux to the running agent

cc-deck's reset should similarly preserve agent configuration while clearing state. The distinction between "reset workspace" and "reset everything" matters for task-to-task transitions.

### Git Remote via ext:: with Auto-Setup

paude auto-creates the `ext::` remote on first harvest, including:
1. Enabling `git ext::` protocol in global git config (if not already)
2. Initializing the container workspace as a bare-capable repo
3. Adding the remote with the correct exec command

The remote name follows a convention: `paude-<session-name>`. cc-deck should use `cc-deck-<workspace-name>` for consistency.

## Open Questions

1. Should git harvest be the default sync strategy for container/compose, or opt-in?
2. How to handle workspace directories that are not git repos? Fall back to copy?
3. Should `cc-deck env push --git` auto-detect whether the local directory is a git repo and choose the strategy accordingly?
4. How to handle submodules?
5. Should `cc-deck env harvest --pr` use `gh pr create` or `glab mr create` depending on the remote?
6. Should `--force-with-lease` be the default for harvest push, or should cc-deck offer both force-with-lease and regular push?
7. How should concurrent session enrichment interact with the Zellij plugin's render cycle? Should the plugin cache git status and refresh on a timer, or only on user request?
