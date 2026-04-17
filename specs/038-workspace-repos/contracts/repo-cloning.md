# Contract: Repo Cloning Behavioral Requirements

**Feature**: 038-workspace-repos

## Overview

This contract defines the behavioral requirements for repo cloning during `env create`. All environment types that support repos (SSH, container, compose, k8s-deploy) MUST satisfy these requirements.

## Behavioral Requirements

### BR-001: Idempotency

The clone operation MUST be idempotent. If the target directory already exists, the repo MUST be skipped with a log message. No error, no attempt to update or pull.

### BR-002: Non-Fatal Failures

A failed clone MUST NOT prevent environment creation from completing. Each clone failure MUST be logged as a warning. The function MUST return a summary of results (success/failure per repo).

### BR-003: Parallel Execution

Repos MUST be cloned concurrently with a maximum of 4 simultaneous operations. Logging output from concurrent clones MUST NOT interleave mid-line.

### BR-004: Workspace Resolution

The workspace base directory MUST be resolved from `EnvironmentDefinition.Workspace` when set. When empty, the type default applies:
- SSH: `~/workspace`
- Container, compose, k8s-deploy: `/workspace`

### BR-005: URL Deduplication

Before cloning, the repo list MUST be deduplicated by normalized URL. Normalization: strip `.git` suffix, lowercase host. The first occurrence wins (preserves branch/target from that entry).

### BR-006: Auto-Detected Repo Behavior

When `env create` runs inside a git repository:
- The `origin` remote URL is added to the clone list
- Additional remotes are recorded for post-clone configuration
- The auto-detected repo becomes the Zellij session working directory
- If the auto-detected URL duplicates an explicit entry, the explicit entry's branch/target take precedence

### BR-007: Credential Handling

- SSH environments with `git_credential_type: ssh`: Enable SSH agent forwarding (`-A` flag)
- Token-based auth: Inject token into HTTPS URL, clone, then rewrite origin URL to remove token
- No credentials configured: Clone proceeds without auth; failures are warnings
- Container/compose/k8s-deploy: Only token-based auth supported (no SSH agent forwarding)

### BR-008: Branch Checkout

When `branch` is specified on a RepoEntry, the system MUST clone with `git clone --branch <branch>`. If the branch does not exist, the clone fails (treated as a warning per BR-002).

### BR-009: Extra Remote Configuration

After cloning an auto-detected repo, additional remotes from the local repository MUST be added via `git remote add <name> <url>`. Failures to add remotes are warnings.

### BR-010: Local Environment Handling

When the environment type is `local`, the system MUST log a warning that repos are not supported for local environments and MUST NOT attempt any clone operations.

## CLI Contract

### --repo Flag

- Repeatable: `--repo url1 --repo url2`
- Combined with `--branch`: `--repo url --branch develop`
- `--branch` applies to the most recent `--repo`
- `--branch` without `--repo` is a validation error
- CLI repos are merged with definition repos and auto-detected repos
- CLI repos are NOT persisted to the definition file
