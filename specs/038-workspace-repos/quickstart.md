# Quickstart: Remote Workspace Repository Provisioning

**Branch**: `038-workspace-repos`

## What This Feature Does

Automatically clones git repositories into remote workspace directories during `env create`. Works across SSH, container, compose, and k8s-deploy environment types.

## Quick Test

1. Create an SSH environment with a repo:

```bash
cc-deck env create test-repos --type ssh \
  --host user@dev.example.com \
  --repo https://github.com/cc-deck/cc-deck.git
```

2. Verify the repo was cloned:

```bash
cc-deck env exec test-repos -- ls ~/workspace/cc-deck/
```

## Key Files to Understand First

| File | Purpose |
|------|---------|
| `cc-deck/internal/env/repos.go` | NEW: RepoEntry type, clone logic, URL normalization |
| `cc-deck/internal/env/definition.go` | MODIFIED: Repos field on EnvironmentDefinition |
| `cc-deck/internal/ssh/client.go` | MODIFIED: AgentForwarding support |
| `cc-deck/internal/cmd/env.go` | MODIFIED: --repo/--branch flags, auto-detection |
| `cc-deck/internal/env/ssh.go` | MODIFIED: Clone repos in Create() |
| `cc-deck/internal/env/container.go` | MODIFIED: Clone repos in Create() |
| `cc-deck/internal/env/compose.go` | MODIFIED: Clone repos in Create() |
| `cc-deck/internal/env/k8s_deploy.go` | MODIFIED: Clone repos in Create() |

## Build and Test

```bash
make test    # Run all tests
make lint    # Run linters
make install # Full build + install
```

## Architecture at a Glance

```
env create --repo URL
    │
    ├── Auto-detect current git repo (if in git dir)
    ├── Merge explicit repos + CLI repos + auto-detected
    ├── Deduplicate by normalized URL
    ├── Resolve credentials from active Profile
    │
    └── After environment is provisioned:
        └── cloneRepos(runner, repos, workspace, creds)
            ├── Check directory exists (skip if so)
            ├── git clone [with token if needed]
            ├── git checkout branch (if specified)
            ├── git remote add (extra remotes)
            └── git remote set-url (clean token)
```
