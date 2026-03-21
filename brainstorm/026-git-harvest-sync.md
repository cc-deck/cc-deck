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

## Open Questions

1. Should git harvest be the default sync strategy for container/compose, or opt-in?
2. How to handle workspace directories that are not git repos? Fall back to copy?
3. Should `cc-deck env push --git` auto-detect whether the local directory is a git repo and choose the strategy accordingly?
4. How to handle submodules?
5. Should `cc-deck env harvest --pr` use `gh pr create` or `glab mr create` depending on the remote?
