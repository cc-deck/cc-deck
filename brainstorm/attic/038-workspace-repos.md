# 038: Remote Workspace Repository Provisioning

## Problem

Remote environments (SSH, Kubernetes, compose/container) start with empty workspaces. Users must manually clone repositories after creating an environment, which is tedious and error-prone, especially when setting up multiple repos.

For container/compose environments, bind-mounting the local project directory works, but for SSH and Kubernetes there is no local filesystem to mount. The workspace needs to be populated from git.

## Proposal

Add a `repos` field to environment definitions that declares git repositories to clone into the workspace during `env create`. This applies to all non-local environment types.

### Definition Format

```yaml
# .cc-deck/environment.yaml or global environments.yaml
name: remote-dev
type: ssh
host: roland@marovo
workspace: ~/workspace
repos:
  - url: git@github.com:cc-deck/cc-deck.git
  - url: git@github.com:cc-deck/cc-session.git
    branch: main
  - url: https://github.com/cc-deck/cc-setup.git
```

Each entry supports:
- `url` (required): Git clone URL (SSH or HTTPS)
- `branch` (optional): Clone a specific branch instead of the default

### Auto-Detection from Current Directory

When `env create` is run inside a git repository:
- The current repo's remote URL is automatically added to `repos` if not already present
- This becomes the "primary" repo (Zellij workspace directory points to it)

When run from a directory containing multiple git repos (one level up), those are not auto-added. Only the explicit git repo context is used.

### Clone Behavior

During `env create`, after workspace directory creation:
1. For each entry in `repos`, run `git clone <url> [--branch <branch>]` into the workspace directory
2. If the target directory already exists, skip with a log message
3. Clone failures are warnings, not fatal errors (the environment is still usable)
4. The clone runs on the remote (via SSH command for SSH envs, inside the container for compose/container, via kubectl exec for k8s)

### Primary Repo

The repo matching the current working directory's git remote becomes the primary workspace. For SSH environments, the Zellij session opens in that directory. If no match, the first repo in the list is used.

### Git Credentials

Two authentication methods for git operations on the remote:

**SSH Agent Forwarding** (default for SSH envs):
- `cc-deck attach` already runs SSH; adding `-A` (agent forwarding) makes git SSH URLs work transparently
- No credential storage needed on the remote

**Token-based auth** (for HTTPS URLs or when agent forwarding is unavailable):
- Configured in global config (`~/.config/cc-deck/config.yaml`):
  ```yaml
  git:
    credentials:
      - host: github.com
        method: token
        token_env: GITHUB_TOKEN
      - host: gitlab.com
        method: token
        token_env: GITLAB_TOKEN
  ```
- Tokens are written to the remote's credential helper or `.netrc` during `env create`
- Reuses the existing `credentials.env` mechanism for pushing env vars to the remote

### Environment Type Support

| Type | How repos are cloned | Credential transport |
|------|---------------------|---------------------|
| SSH | `git clone` via SSH command on remote | SSH agent forwarding or token via credentials.env |
| Compose | `git clone` inside running container | Token via compose env vars |
| Container | `git clone` inside running container | Token via container env vars |
| k8s-deploy | `git clone` via kubectl exec | Token via Kubernetes Secret |
| Local | N/A (repos are already on the host) | N/A |

## Decisions Made

- **No worktree support in v1.** Worktree replication is complex (branch existence checks, naming, partial failures). Users can create worktrees manually after clone. The `repos` YAML structure can accommodate an optional `worktrees:` field in a future version.
- **Skip on existing repo.** Don't pull, don't error. Log and continue. Users control when to update.
- **Clone-time only.** Repos are provisioned during `env create`, not on every `attach`.
- **Warnings, not errors.** A failed clone should not prevent environment creation.

## Future Extensions

- `worktrees:` field per repo entry for automated worktree creation
- `cc-deck env sync` command to re-run repo provisioning on an existing environment
- Scan local worktree structure and replicate on remote
- Support for shallow clones (`depth: 1`) for large repos

## Open Questions

- Should `cc-deck env create --repo <url>` be a CLI flag shortcut for ad-hoc repos not in the definition?
- Should the Ansible provisioning (`cc-deck.build --target ssh`) also use the `repos` field from the setup manifest?
