# Tasks: Remote Workspace Repository Provisioning

**Branch**: `038-workspace-repos` | **Date**: 2026-04-17

## Task List

### Phase 1: Core Data Model and Repo Logic

#### [X] Task 1.1: RepoEntry type and URL utilities
**Priority**: P1 | **Effort**: Medium | **Files**: `cc-deck/internal/env/repos.go` (NEW)
**Requirements**: FR-001, FR-002, FR-004, FR-005

Create the `RepoEntry` struct, `CommandRunner` type, `GitCredentials` struct, and `RepoCloneResult` struct. Implement:
- `NormalizeURL(url string) string` - strips `.git` suffix, lowercases host, normalizes SSH URLs
- `RepoNameFromURL(url string) string` - extracts last path component without `.git`
- `TargetDir(entry RepoEntry) string` - returns entry.Target if set, else RepoNameFromURL(entry.URL)
- `DeduplicateRepos(repos []RepoEntry) []RepoEntry` - deduplicates by normalized URL, first wins

**Acceptance**: Unit tests pass for URL normalization (HTTPS, SSH, with/without `.git`), repo name extraction, and deduplication.

#### [X] Task 1.2: Clone command generation and execution
**Priority**: P1 | **Effort**: Large | **Files**: `cc-deck/internal/env/repos.go` (EXTEND)
**Requirements**: FR-003, FR-006, FR-007, FR-008, FR-009, FR-023
**Depends on**: Task 1.1

Implement:
- `resolveGitCredentials(credType GitCredentialType, credSecret string) (*GitCredentials, error)` - resolves token value from env var or K8s Secret reference; returns nil credentials for SSH type (handled by agent forwarding) or when unconfigured
- `buildCloneCommand(entry RepoEntry, workspace string, creds *GitCredentials) string` - builds `git clone` command with optional branch and token injection
- `buildTokenCleanupCommand(entry RepoEntry, workspace string) string` - builds `git remote set-url` to remove token
- `cloneRepos(ctx context.Context, runner CommandRunner, repos []RepoEntry, workspace string, creds *GitCredentials, extraRemotes map[string]string) []RepoCloneResult` - orchestrates parallel cloning with max 4 goroutines, idempotency checks, token cleanup, and extra remote configuration

The `extraRemotes` parameter is a map of remote-name to URL, applied only to the auto-detected repo after cloning. Passed as a separate parameter (not part of RepoEntry) to keep the data model simple.

**Acceptance**: Mock CommandRunner tests pass for: clone with/without branch, token injection + cleanup, idempotent skip, parallel execution, failure as warning, credential resolution from env var.

#### [X] Task 1.3: Add Repos field to EnvironmentDefinition
**Priority**: P1 | **Effort**: Small | **Files**: `cc-deck/internal/env/definition.go` (MODIFY)
**Requirements**: FR-001

Add `Repos []RepoEntry \`yaml:"repos,omitempty"\`` field to `EnvironmentDefinition` struct, positioned after `Workspace`.

**Acceptance**: YAML round-trip test: marshal/unmarshal definition with repos field preserves data.

#### [X] Task 1.4: Unit tests for repo logic
**Priority**: P1 | **Effort**: Medium | **Files**: `cc-deck/internal/env/repos_test.go` (NEW)
**Requirements**: All Phase 1 FRs
**Depends on**: Task 1.1, Task 1.2

Write unit tests covering:
- URL normalization: `https://github.com/org/repo.git` -> normalized form, `git@github.com:org/repo.git` -> normalized form, mixed case hosts
- Repo name extraction: various URL formats
- Deduplication: same repo in HTTPS and SSH format
- Clone command: with/without branch, with/without token
- Token cleanup command generation
- cloneRepos with mock runner: idempotent skip, parallel execution (verify max 4), failure handling, token cleanup

**Acceptance**: `make test` passes with all new tests.

### Phase 2: SSH Agent Forwarding

#### [X] Task 2.1: Add AgentForwarding to SSH client
**Priority**: P1 | **Effort**: Small | **Files**: `cc-deck/internal/ssh/client.go` (MODIFY)
**Requirements**: FR-021

Add `AgentForwarding bool` field to `Client` struct. In `buildArgs()`, conditionally append `-A` when `AgentForwarding` is true. Also add `-A` to `buildInteractiveArgs()` when enabled.

**Acceptance**: Existing SSH tests still pass. New test verifies `-A` appears in args when AgentForwarding is true.

### Phase 3: Environment Type Integration

#### [X] Task 3.1: SSH environment repo cloning
**Priority**: P1 | **Effort**: Medium | **Files**: `cc-deck/internal/env/ssh.go` (MODIFY)
**Requirements**: FR-015, FR-019, FR-020, FR-021
**Depends on**: Task 1.2, Task 2.1

In `sshEnv.Create()`, after provisioning completes:
1. Resolve workspace from definition (or default `~/workspace`)
2. Call `resolveGitCredentials()` with active Profile's `GitCredentialType` and `GitCredentialSecret`
3. If credentials are SSH type, set `client.AgentForwarding = true`
4. Call `cloneRepos()` with SSH runner wrapping `client.Run()`, passing credentials and extra remotes
5. Log results

**Acceptance**: SSH environment Create() calls cloneRepos when repos are defined. Agent forwarding is enabled for SSH credential type.

#### [X] Task 3.2: Container environment repo cloning
**Priority**: P1 | **Effort**: Medium | **Files**: `cc-deck/internal/env/container.go` (MODIFY), `cc-deck/internal/podman/exec.go` (MODIFY)
**Requirements**: FR-016, FR-019, FR-020
**Depends on**: Task 1.2

Add `ExecOutput(ctx, containerName, cmd string) (string, error)` to podman package that returns stdout. In `containerEnv.Create()`, after container start, call `resolveGitCredentials()` with active Profile, then call `cloneRepos()` with podman exec runner. Workspace defaults to `/workspace` unless overridden. Only token-based auth is supported (per clarification).

**Acceptance**: Container environment Create() calls cloneRepos when repos are defined. Token credentials are resolved and passed.

#### [X] Task 3.3: Compose environment repo cloning
**Priority**: P1 | **Effort**: Small | **Files**: `cc-deck/internal/env/compose.go` (MODIFY)
**Requirements**: FR-017, FR-019, FR-020
**Depends on**: Task 1.2, Task 3.2

In `composeEnv.Create()`, after compose up, call `resolveGitCredentials()` with active Profile, then call `cloneRepos()` with runner wrapping podman exec on the session container. Workspace defaults to `/workspace` unless overridden. Only token-based auth is supported.

**Acceptance**: Compose environment Create() calls cloneRepos when repos are defined. Token credentials are resolved and passed.

#### [X] Task 3.4: K8s-deploy environment repo cloning
**Priority**: P1 | **Effort**: Medium | **Files**: `cc-deck/internal/env/k8s_deploy.go` (MODIFY)
**Requirements**: FR-018, FR-019, FR-020
**Depends on**: Task 1.2

In `k8sDeployEnv.Create()`, after pod is ready, call `resolveGitCredentials()` with active Profile, then call `cloneRepos()` with runner wrapping `k8sExec`. Add output-returning variant of k8sExec. Workspace defaults to `/workspace` unless overridden. Only token-based auth is supported.

**Acceptance**: K8s-deploy environment Create() calls cloneRepos when repos are defined. Token credentials are resolved and passed.

### Phase 4: CLI Flags and Auto-Detection

#### [X] Task 4.1: Add --repo and --branch CLI flags
**Priority**: P1 | **Effort**: Medium | **Files**: `cc-deck/internal/cmd/env.go` (MODIFY)
**Requirements**: FR-024, FR-025, FR-026

Add `repos []string` and `branches []string` to `createFlags`. Register as repeatable `StringArrayVar` flags. Parse into `[]RepoEntry` with branch-to-repo positional association. Validate that `--branch` count does not exceed `--repo` count. Merge CLI repos with definition repos.

**Acceptance**: `--repo url` works, `--repo url --branch dev` works, multiple `--repo` flags work, `--branch` without `--repo` returns validation error.

#### [X] Task 4.2: Auto-detect current git repository
**Priority**: P1 | **Effort**: Medium | **Files**: `cc-deck/internal/cmd/env.go` (MODIFY)
**Requirements**: FR-010, FR-011, FR-012, FR-013
**Depends on**: Task 4.1

In `runEnvCreate()`, after resolving the environment name:
1. Use existing `project.FindGitRoot()` to detect git repo
2. If in git repo, run `git remote -v` to enumerate remotes
3. Build RepoEntry from `origin` remote URL
4. Collect additional remotes (upstream, etc.) as `map[string]string` (name -> URL)
5. Merge with definition repos and CLI repos (deduplicate)
6. Set auto-detected repo as Zellij working directory via workspace path
7. Pass extra remotes map as a separate parameter to `cloneRepos()` (not part of RepoEntry, keeping the data model simple)

**Acceptance**: Running inside a git repo auto-detects origin and adds it to clone list. Additional remotes are configured on the cloned repo via `git remote add`. Duplicate URLs are handled.

### Phase 5: Local Environment Warning

#### [X] Task 5.1: Warn and skip repos for local environments
**Priority**: P2 | **Effort**: Small | **Files**: `cc-deck/internal/cmd/env.go` (MODIFY)
**Requirements**: FR-014

Before calling `env.Create()`, if environment type is `local` and repos list is non-empty, log a warning and clear the repos list.

**Acceptance**: Local environment with repos logs warning and does not attempt cloning.

### Phase 6: Tests

#### [X] Task 6.1: Integration-style tests with mock runner
**Priority**: P1 | **Effort**: Medium | **Files**: `cc-deck/internal/env/repos_test.go` (EXTEND)
**Requirements**: All FRs
**Depends on**: Task 1.4

Add integration-style tests using mock CommandRunner that simulate full clone workflows:
- Multiple repos with mixed success/failure
- Token credential flow (inject + cleanup)
- Idempotent re-run (directories already exist)
- Concurrency verification (ensure max 4 parallel)
- Auto-detected repo with extra remotes

**Acceptance**: All tests pass via `make test`.

#### [X] Task 6.2: CLI flag and auto-detection tests
**Priority**: P1 | **Effort**: Medium | **Files**: `cc-deck/internal/cmd/env_create_test.go` (MODIFY)
**Requirements**: FR-024, FR-025, FR-010
**Depends on**: Task 4.1, Task 4.2

Test flag parsing:
- `--repo url` parsed correctly
- `--repo url --branch dev` associates branch with repo
- Multiple `--repo` flags
- `--branch` without `--repo` validation error
- Auto-detection when CWD is a git repo vs. non-git directory

**Acceptance**: All tests pass via `make test`.

### Phase 7: Documentation

#### [X] Task 7.1: Update README and spec table
**Priority**: P2 | **Effort**: Small | **Files**: `README.md` (MODIFY)
**Requirements**: Constitution IX, X

Add workspace repos feature description. Update the Feature Specifications table with entry for 038-workspace-repos. Use prose plugin with cc-deck voice.

**Acceptance**: README reflects the new feature. Spec table has 038 entry.

#### [X] Task 7.2: Update CLI reference documentation
**Priority**: P2 | **Effort**: Small | **Files**: `docs/modules/reference/pages/cli.adoc` (MODIFY)
**Requirements**: Constitution IX

Document `--repo` and `--branch` flags on `env create` command with usage examples and flag descriptions. Use prose plugin with cc-deck voice. One sentence per line (AsciiDoc).

**Acceptance**: CLI reference documents the new flags.

#### [X] Task 7.3: Create Antora guide page
**Priority**: P3 | **Effort**: Medium | **Files**: Antora docs (NEW page, if docs/ exists)
**Requirements**: Constitution IX

Create a guide page covering workspace repository provisioning: overview, configuration (repos field in definition), auto-detection, credential setup, CLI flags, examples. Add to module nav.adoc. Use prose plugin with cc-deck voice.

**Acceptance**: Guide page exists and is linked from navigation.

## Summary

| Phase | Tasks | Priority | Total Effort |
|-------|-------|----------|-------------|
| 1. Core Data Model | 4 tasks | P1 | Large |
| 2. SSH Agent Forwarding | 1 task | P1 | Small |
| 3. Environment Integration | 4 tasks | P1 | Medium-Large |
| 4. CLI & Auto-Detection | 2 tasks | P1 | Medium |
| 5. Local Warning | 1 task | P2 | Small |
| 6. Tests | 2 tasks | P1 | Medium |
| 7. Documentation | 3 tasks | P2-P3 | Small-Medium |
| **Total** | **17 tasks** | | |
