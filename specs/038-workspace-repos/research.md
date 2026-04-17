# Research: Remote Workspace Repository Provisioning

**Branch**: `038-workspace-repos` | **Date**: 2026-04-17

## Research Findings

### R1: Command Execution Across Environment Types

**Decision**: Use a common `CommandRunner` function type that each environment type provides, rather than the public `Exec` interface.

**Rationale**: The public `Environment.Exec()` method does not return command output, only an error. Repo cloning needs output capture for idempotency checks (e.g., checking if a directory exists) and for post-clone URL rewriting. Each environment type already has internal exec mechanisms that return output:
- SSH: `client.Run(ctx, cmd)` returns `(string, error)`
- Container: `podman.Exec()` can be extended with an output-returning variant
- K8s: `k8sExec()` can be extended similarly

**Alternatives considered**:
- Using `Environment.Exec()` directly: Rejected because it drops stdout and provides no way to capture git command output.
- Adding a new `ExecWithOutput` method to the interface: Rejected as over-engineering for an internal need. The command runner function type is simpler.

### R2: SSH Agent Forwarding Implementation

**Decision**: Add `AgentForwarding bool` field to `ssh.Client` struct and conditionally include `-A` in `buildArgs()`.

**Rationale**: The SSH client at `cc-deck/internal/ssh/client.go` constructs arguments via `buildArgs()`. Adding a boolean field is the minimal change. The field is set during repo cloning when the Profile has `git_credential_type: ssh`, and only affects the SSH environment type.

**Alternatives considered**:
- Per-call RunWithOpts method: More flexible but unnecessary complexity for a single boolean.
- Always enabling agent forwarding: Security concern; forwarding should only be active when the user has configured SSH credentials.

### R3: Token Injection and Cleanup

**Decision**: Inject token into HTTPS URL for cloning, then rewrite remote URL via `git remote set-url origin <clean-url>` after successful clone.

**Rationale**: Git's standard `https://token@host/path` format works universally. The post-clone rewrite removes the token from `.git/config`, preventing credential leakage. This is a two-command operation per repo (clone + set-url) that runs atomically.

**Alternatives considered**:
- `git -c credential.helper` approach: Cleaner (token never touches disk) but leaves the cloned repo without any credential configuration for subsequent push/pull, hurting developer UX on the remote.
- Environment variable `GIT_ASKPASS`: Similar to credential helper approach, same UX tradeoff.

### R4: Parallel Cloning Implementation

**Decision**: Use a worker pool of max 4 goroutines with `errgroup` (or `sync.WaitGroup` with channel-based semaphore) to clone repos concurrently.

**Rationale**: The Go stdlib `sync` package plus a channel-based semaphore is the simplest pattern. No new dependencies needed. The max of 4 was confirmed during spec clarification.

**Alternatives considered**:
- Sequential cloning: Too slow for environments with many repos.
- Unbounded parallelism: Could overwhelm remote host resources.
- External worker pool library: Unnecessary dependency for a simple semaphore pattern.

### R5: Auto-Detection of Current Git Repo

**Decision**: Use existing `project.FindGitRoot()` to detect git root, then shell out to `git remote -v` to enumerate remotes. Build a RepoEntry from the `origin` remote and collect additional remotes for post-clone configuration.

**Rationale**: The project package already detects git roots. Reading remotes via `git remote -v` is straightforward and avoids importing a Go git library. The auto-detected repo is merged with explicit repos using URL normalization for deduplication.

**Alternatives considered**:
- go-git library: Heavy dependency for simple remote listing.
- Reading `.git/config` directly: Fragile parsing when `git remote -v` is reliable.

### R6: URL Normalization for Deduplication

**Decision**: Normalize URLs by stripping `.git` suffix, lowercasing the host, and converting `git@host:path` SSH URLs to a canonical form for comparison.

**Rationale**: Users may specify the same repo with different URL formats (HTTPS vs SSH, with/without `.git`). Deduplication by normalized URL prevents duplicate clones (FR-013).

**Alternatives considered**:
- Exact string match only: Would miss `https://github.com/org/repo` vs `https://github.com/org/repo.git`.
- Full URL parsing with net/url: SSH git URLs (`git@host:path`) are not valid URLs and need special handling anyway.

### R7: Workspace Resolution

**Decision**: Use `EnvironmentDefinition.Workspace` as the base directory when set, falling back to type-specific defaults (`~/workspace` for SSH, `/workspace` for container/compose/k8s).

**Rationale**: Confirmed during spec clarification. The existing `Workspace` field on `EnvironmentDefinition` is already used by SSH environments. Extending this to other types is consistent with the existing data model.

**Alternatives considered**:
- Separate `repos-workspace` field: Unnecessary duplication when the existing field serves the same purpose.
