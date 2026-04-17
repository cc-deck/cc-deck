# Feature Specification: Remote Workspace Repository Provisioning

**Feature Branch**: `038-workspace-repos`  
**Created**: 2026-04-17  
**Status**: Draft  
**Input**: User description: "Remote workspace repository provisioning for non-local environments"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Clone repos into remote workspace during environment creation (Priority: P1)

A developer creates a non-local environment to work on a remote machine, container, or Kubernetes pod. They declare git repositories in their environment definition. When they run `env create`, the repositories are automatically cloned into the workspace directory, so the developer can immediately start working without manual setup.

**Why this priority**: This is the core value proposition. Without this, developers must manually clone repos every time they set up a new environment.

**Independent Test**: Can be tested by creating an SSH environment with a `repos` field containing one public repository and verifying the repo exists in the workspace after creation.

**Acceptance Scenarios**:

1. **Given** an environment definition with `repos: [{url: "https://github.com/example/repo.git"}]`, **When** the user runs `cc-deck env create my-env --type ssh`, **Then** the repository is cloned into `~/workspace/repo/` on the remote host.
2. **Given** a `repos` entry with `branch: develop`, **When** the environment is created, **Then** the repository is cloned with the `develop` branch checked out.
3. **Given** a `repos` entry with `target: my-project`, **When** the environment is created, **Then** the repository is cloned into `~/workspace/my-project/` instead of the default directory name derived from the URL.
4. **Given** a repository that already exists in the workspace, **When** the environment is created (idempotent re-run), **Then** the existing repository is skipped with a log message and no error occurs.

---

### User Story 2 - Auto-detect current git repo (Priority: P1)

A developer runs `cc-deck env create` from inside a git repository. The tool automatically detects the current repo's `origin` remote URL and includes it in the repos to clone on the remote. Additional remotes (upstream, fork, etc.) are added as named remotes on the cloned repo. The auto-detected repo becomes the Zellij session working directory.

**Why this priority**: Most environments are created in the context of a specific project. Auto-detection eliminates the need to manually configure the repos field for the common case.

**Independent Test**: Can be tested by running `env create` from inside a git repo and verifying the repo is cloned on the remote and set as the Zellij working directory.

**Acceptance Scenarios**:

1. **Given** the user is inside a git repository with remote `origin` pointing to `git@github.com:cc-deck/cc-deck.git`, **When** they run `cc-deck env create demo --type ssh`, **Then** `cc-deck` is cloned into `~/workspace/cc-deck/` on the remote. The Zellij session opens with its working directory set to `~/workspace/cc-deck/`.
2. **Given** the local repo also has a remote `upstream` pointing to a different URL, **When** the environment is created, **Then** the cloned repo on the remote has `upstream` configured as an additional remote.
3. **Given** the user is in a directory that is not a git repository, **When** they run `env create`, **Then** no auto-detection occurs and only explicitly declared repos are cloned.
4. **Given** the auto-detected repo URL already appears in the explicit `repos` list, **When** the environment is created, **Then** the repo is cloned only once (no duplicate).

---

### User Story 3 - Git credential transport for private repositories (Priority: P2)

A developer needs to clone private repositories on a remote environment. The system uses the existing Profile credential model (`git_credential_type` and `git_credential_secret` on Profile) to determine the git authentication method. During environment creation, credentials are transported to the remote so that git clone operations succeed for private repos.

**Why this priority**: Many real-world projects use private repositories. Without credential transport, only public repos can be cloned, severely limiting usefulness.

**Independent Test**: Can be tested by configuring a Profile with `git_credential_type: token` and verifying a private repo can be cloned during SSH environment creation.

**Acceptance Scenarios**:

1. **Given** the active Profile has `git_credential_type: ssh` and SSH agent forwarding is enabled, **When** the environment is created, **Then** private repos accessible via SSH keys are cloned successfully.
2. **Given** the active Profile has `git_credential_type: token` and `git_credential_secret` references a K8s Secret or env var containing a token, **When** the environment is created, **Then** the token is used to authenticate HTTPS git clone operations on the remote.
3. **Given** no git credentials configured in the active Profile, **When** a private repo clone fails, **Then** a warning is logged with guidance on configuring credentials, and environment creation continues.

---

### User Story 4 - CLI flag for ad-hoc repos (Priority: P3)

A developer wants to clone a repository into the workspace without editing the environment definition file. They pass `--repo <url>` (and optionally `--branch <branch>`) on the command line during environment creation.

**Why this priority**: Provides a quick way to add repos for one-off use without modifying persistent configuration.

**Independent Test**: Can be tested by running `env create --repo https://github.com/example/repo.git` and verifying the repo is cloned.

**Acceptance Scenarios**:

1. **Given** the user passes `--repo git@github.com:example/repo.git`, **When** the environment is created, **Then** the repository is cloned into the workspace alongside any repos from the definition.
2. **Given** the user passes `--repo git@github.com:example/repo.git --branch develop`, **When** the environment is created, **Then** the repository is cloned with the `develop` branch checked out.
3. **Given** the user passes multiple `--repo` flags, **When** the environment is created, **Then** all specified repositories are cloned.
4. **Given** the user passes `--branch` without a preceding `--repo`, **When** the command is parsed, **Then** an error is returned explaining that `--branch` requires `--repo`.

---

### Edge Cases

- What happens when a git clone fails due to network issues? Warning is logged, environment creation continues with remaining repos.
- What happens when the remote workspace directory has insufficient disk space? The git clone fails, warning is logged.
- What happens when the repo URL is malformed? Warning is logged with the invalid URL, clone is skipped.
- What happens when SSH agent forwarding is requested but the local agent has no keys loaded? Clone fails for SSH URLs, warning includes guidance to check SSH agent.
- What happens when the same repo URL appears in both `repos` definition and `--repo` flag? Cloned only once, deduplicated by normalized URL.
- What happens when two repos from different hosts have the same name (e.g., `github.com/alice/utils` and `gitlab.com/bob/utils`)? The second clone fails because the target directory already exists. User should use the `target` field to disambiguate.
- What happens when `env create` is re-run for an existing environment with new repos added? The operation is idempotent: existing repos are skipped, new repos are cloned.
- What happens when the `repos` field is set on a local environment? A warning is logged that repos are not supported for local environments. The field is ignored.

## Requirements *(mandatory)*

### Functional Requirements

#### Core Repo Cloning

- **FR-001**: Environment definitions MUST support a `repos` field containing a list of git repository entries.
- **FR-002**: Each repo entry MUST accept a `url` field (required), an optional `branch` field, and an optional `target` field.
- **FR-003**: During `env create`, the system MUST clone each declared repo into the workspace directory on the target environment.
- **FR-004**: The target directory for a clone MUST default to the repository name derived from the URL (the last path component without `.git` suffix), matching `git clone` default behavior.
- **FR-005**: When a `target` field is specified, the repo MUST be cloned into `<workspace>/<target>/` instead of the default.
- **FR-006**: If a repo's target directory already exists in the workspace, the system MUST skip the clone and log a message.
- **FR-007**: Clone failures MUST be treated as warnings, not fatal errors. Environment creation MUST continue with remaining repos.
- **FR-008**: The `env create` operation MUST be idempotent with respect to repo cloning. Re-running `env create` MUST clone only repos whose target directories do not yet exist.
- **FR-009**: Repos MUST be cloned in parallel where possible, with a maximum concurrency of 4 simultaneous clone operations.

#### Auto-Detection

- **FR-010**: When `env create` is run inside a git repository, the system MUST auto-detect the `origin` remote URL and include it in the clone list.
- **FR-011**: If the local repo has additional remotes beyond `origin`, the system MUST add them as named remotes on the cloned repo after cloning.
- **FR-012**: The auto-detected repo MUST become the primary workspace directory (the Zellij session working directory is set to `<workspace>/<repo-name>/`).
- **FR-013**: If the auto-detected repo URL duplicates an explicit `repos` entry, the system MUST clone it only once.

#### Environment Type Support

- **FR-014**: The `repos` field MUST be supported for SSH, compose, container, and k8s-deploy environment types. Local environments MUST log a warning and ignore it.
- **FR-015**: For SSH environments, repos MUST be cloned via commands executed over SSH (`client.Run`).
- **FR-016**: For container environments, repos MUST be cloned via commands executed inside the running container (`podman exec`).
- **FR-017**: For compose environments, repos MUST be cloned via commands executed inside the primary service container.
- **FR-018**: For k8s-deploy environments, repos MUST be cloned via commands executed inside the Pod (`kubectl exec`).
- **FR-019**: The workspace base directory MUST be resolved from the `Workspace` field on `EnvironmentDefinition` when set. When empty, it falls back to the type default: `~/workspace` for SSH, `/workspace` for container, compose, and k8s-deploy.

#### Credential Transport

- **FR-020**: The system MUST use the existing Profile credential model (`git_credential_type` and `git_credential_secret` fields on Profile) for git authentication configuration. No new `git.credentials` config section is introduced.
- **FR-021**: The system MUST support SSH agent forwarding as a credential method for git SSH URLs on SSH environments only. The SSH client MUST use the `-A` flag when cloning. Container, compose, and k8s-deploy environments MUST use token-based auth for private repos; SSH key auth is not supported for these types.
- **FR-022**: The system MUST support token-based authentication for git HTTPS URLs. For token method, the token value MUST be resolved from the environment variable or K8s Secret referenced by `git_credential_secret`.
- **FR-023**: For token-based auth, the token MUST be injected into HTTPS clone URLs using the standard `https://<token>@host/path` format. After a successful clone, the system MUST rewrite the remote origin URL to remove the embedded token (using `git remote set-url origin <clean-url>`). The token MUST NOT persist in `.git/config` or any other file on the remote.

#### CLI Flag

- **FR-024**: The `env create` command MUST accept a repeatable `--repo` flag for ad-hoc repository URLs.
- **FR-025**: The `env create` command MUST accept a `--branch` flag that specifies the branch for the most recent `--repo` flag. Using `--branch` without `--repo` MUST produce a validation error.
- **FR-026**: Repos specified via `--repo` MUST be cloned alongside repos from the definition, without persisting to the definition file.

### Key Entities

- **RepoEntry**: A git repository to clone. Contains `url` (string, required), `branch` (string, optional), and `target` (string, optional, defaults to repo name from URL).
- **Profile**: Extended with existing `git_credential_type` (ssh or token) and `git_credential_secret` (string) fields. No new entity needed.
- **EnvironmentDefinition**: Extended with an optional `repos` field (list of RepoEntry).

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A developer can create a remote SSH environment with 3 declared repos and find all 3 cloned in the workspace within the time of environment creation (excluding network transfer time for large repos).
- **SC-002**: Running `env create` from inside a git repo automatically clones that repo on the remote without any manual configuration, and opens the Zellij session in that repo's directory.
- **SC-003**: Private repositories are accessible on the remote using either SSH agent forwarding or token-based authentication from the active Profile, with zero manual credential setup on the remote.
- **SC-004**: A failed repo clone does not prevent environment creation or block access to successfully cloned repos.
- **SC-005**: The feature works consistently across SSH, compose, container, and k8s-deploy environment types, each using its native execution mechanism (SSH exec, podman exec, kubectl exec).
- **SC-006**: Re-running `env create` after adding new repos to the definition clones only the new repos.

## Clarifications

### Session 2026-04-17

- Q: Should repo cloning use the existing `Workspace` field on EnvironmentDefinition as the base directory? → A: Yes, use `Workspace` field as base path when set, fall back to type defaults (`~/workspace` for SSH, `/workspace` for containers) when empty.
- Q: How should SSH-key-based git auth work for non-SSH environment types (container, k8s-deploy)? → A: SSH key auth is only supported for SSH environments. Container and k8s-deploy environments require token-based auth for private repos.
- Q: What concurrency limit for parallel repo cloning? → A: Maximum 4 concurrent clones.
- Q: Should RepoEntry support a `depth` option for shallow clones? → A: No, default to full clones. No depth option in this version; can be added later.
- Q: Should the system prevent token leakage in `.git/config` after token-based clone? → A: Yes, rewrite the remote origin URL after clone to remove the embedded token.

## Assumptions

- Git is installed on all target environments (SSH hosts, container images, Kubernetes pods). The existing provisioning pipeline ensures this for SSH environments.
- For SSH environments, the SSH client supports agent forwarding (`-A` flag).
- Token-based credentials reference environment variables already set on the local machine, or K8s Secrets accessible in the cluster. The system does not manage token creation or rotation.
- Repos are cloned as siblings in the workspace directory (e.g., `~/workspace/repo-a/`, `~/workspace/repo-b/`). Nested structures are not supported.
- The Ansible provisioning pipeline (`cc-deck.build --target ssh`) does not consume the `repos` field. Provisioning handles tool installation; repo cloning is handled by `env create`.
- Worktree support is explicitly out of scope for this version. The YAML structure can accommodate an optional `worktrees` field in a future iteration.
- Shallow clone (`--depth`) support is out of scope. All repos are cloned with full history. A `depth` field on RepoEntry can be added in a future iteration.
- The existing Profile model's `git_credential_type` and `git_credential_secret` fields are sufficient for git authentication. These fields already exist in the codebase and do not require schema changes.
