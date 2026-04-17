# Feature Specification: Remote Workspace Repository Provisioning

**Feature Branch**: `038-workspace-repos`  
**Created**: 2026-04-17  
**Status**: Draft  
**Input**: User description: "Remote workspace repository provisioning for non-local environments"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Clone repos into remote workspace during environment creation (Priority: P1)

A developer creates an SSH environment to work on a remote machine. They declare git repositories in their environment definition. When they run `env create`, the repositories are automatically cloned into the remote workspace directory, so the developer can immediately start working without manual setup.

**Why this priority**: This is the core value proposition. Without this, developers must SSH in and clone repos manually every time they set up a new remote environment.

**Independent Test**: Can be tested by creating an SSH environment with a `repos` field containing one public repository and verifying the repo exists in the workspace after creation.

**Acceptance Scenarios**:

1. **Given** an environment definition with `repos: [{url: "https://github.com/example/repo.git"}]`, **When** the user runs `cc-deck env create my-env --type ssh`, **Then** the repository is cloned into the workspace directory on the remote host.
2. **Given** a `repos` entry with `branch: develop`, **When** the environment is created, **Then** the repository is cloned with the `develop` branch checked out.
3. **Given** a repository that already exists in the workspace, **When** the environment is created, **Then** the existing repository is skipped with a log message and no error occurs.

---

### User Story 2 - Auto-detect current git repo (Priority: P1)

A developer runs `cc-deck env create` from inside a git repository. The tool automatically detects the current repo's remote URL and includes it in the repos to clone on the remote. This repo becomes the primary workspace directory where the Zellij session opens.

**Why this priority**: Most environments are created in the context of a specific project. Auto-detection eliminates the need to manually configure the repos field for the common case.

**Independent Test**: Can be tested by running `env create` from inside a git repo and verifying the repo is cloned on the remote and set as the Zellij working directory.

**Acceptance Scenarios**:

1. **Given** the user is inside a git repository with remote `origin` pointing to `git@github.com:cc-deck/cc-deck.git`, **When** they run `cc-deck env create demo --type ssh`, **Then** `cc-deck` is cloned into the remote workspace and the Zellij session opens in that directory.
2. **Given** the user is in a directory that is not a git repository, **When** they run `env create`, **Then** no auto-detection occurs and only explicitly declared repos are cloned.
3. **Given** the auto-detected repo URL already appears in the explicit `repos` list, **When** the environment is created, **Then** the repo is cloned only once (no duplicate).

---

### User Story 3 - Git credential transport for private repositories (Priority: P2)

A developer needs to clone private repositories on a remote environment. They configure git credentials in the global cc-deck config. During environment creation, credentials are transported to the remote so that git clone operations succeed for private repos.

**Why this priority**: Many real-world projects use private repositories. Without credential transport, only public repos can be cloned, severely limiting usefulness.

**Independent Test**: Can be tested by configuring a token credential for GitHub in the global config and verifying a private repo can be cloned during SSH environment creation.

**Acceptance Scenarios**:

1. **Given** git credentials configured with `method: ssh-agent` and SSH agent forwarding enabled, **When** the environment is created, **Then** private repos accessible via SSH keys are cloned successfully.
2. **Given** git credentials configured with `method: token` and `token_env: GITHUB_TOKEN`, **When** the environment is created, **Then** the token is used to authenticate HTTPS git clone operations on the remote.
3. **Given** no git credentials configured, **When** a private repo clone fails, **Then** a warning is logged with guidance on configuring credentials, and environment creation continues.

---

### User Story 4 - CLI flag for ad-hoc repos (Priority: P3)

A developer wants to clone a repository into the workspace without editing the environment definition file. They pass `--repo <url>` on the command line during environment creation.

**Why this priority**: Provides a quick way to add repos for one-off use without modifying persistent configuration.

**Independent Test**: Can be tested by running `env create --repo https://github.com/example/repo.git` and verifying the repo is cloned.

**Acceptance Scenarios**:

1. **Given** the user passes `--repo git@github.com:example/repo.git`, **When** the environment is created, **Then** the repository is cloned into the workspace alongside any repos from the definition.
2. **Given** the user passes multiple `--repo` flags, **When** the environment is created, **Then** all specified repositories are cloned.

---

### Edge Cases

- What happens when a git clone fails due to network issues? Warning is logged, environment creation continues with remaining repos.
- What happens when the remote workspace directory has insufficient disk space? The git clone fails, warning is logged.
- What happens when the repo URL is malformed? Warning is logged with the invalid URL, clone is skipped.
- What happens when SSH agent forwarding is requested but the local agent has no keys loaded? Clone fails for SSH URLs, warning includes guidance to check SSH agent.
- What happens when the same repo URL appears in both `repos` definition and `--repo` flag? Cloned only once, deduplicated by URL.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: Environment definitions MUST support a `repos` field containing a list of git repository entries.
- **FR-002**: Each repo entry MUST accept a `url` field (required) and an optional `branch` field.
- **FR-003**: During `env create`, the system MUST clone each declared repo into the workspace directory on the target environment.
- **FR-004**: If a repo's target directory already exists in the workspace, the system MUST skip the clone and log a message.
- **FR-005**: Clone failures MUST be treated as warnings, not fatal errors. Environment creation MUST continue with remaining repos.
- **FR-006**: When `env create` is run inside a git repository, the system MUST auto-detect the current repo's remote URL and include it in the clone list.
- **FR-007**: The auto-detected repo MUST become the primary workspace directory (where the Zellij session opens).
- **FR-008**: If the auto-detected repo URL duplicates an explicit `repos` entry, the system MUST clone it only once.
- **FR-009**: The `repos` field MUST be supported for SSH, compose, container, and k8s-deploy environment types. Local environments MUST ignore it.
- **FR-010**: The system MUST support SSH agent forwarding as a credential method for git SSH URLs on SSH environments.
- **FR-011**: The system MUST support token-based authentication for git HTTPS URLs, configured in the global cc-deck config.
- **FR-012**: Git credential configuration MUST be stored in the global config file under a `git.credentials` section.
- **FR-013**: Each credential entry MUST specify a `host`, `method` (ssh-agent or token), and for token method, a `token_env` referencing an environment variable.
- **FR-014**: The `env create` command MUST accept a repeatable `--repo` flag for ad-hoc repository URLs.
- **FR-015**: Repos specified via `--repo` MUST be cloned alongside repos from the definition, without persisting to the definition file.

### Key Entities

- **RepoEntry**: A git repository to clone. Contains `url` (string, required) and `branch` (string, optional).
- **GitCredential**: Authentication configuration for a git host. Contains `host` (string), `method` (ssh-agent or token), and `token_env` (string, for token method).
- **EnvironmentDefinition**: Extended with an optional `repos` field (list of RepoEntry).
- **GlobalConfig**: Extended with an optional `git.credentials` section (list of GitCredential).

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A developer can create a remote SSH environment with 3 declared repos and find all 3 cloned in the workspace within the time of environment creation (excluding network transfer time for large repos).
- **SC-002**: Running `env create` from inside a git repo automatically clones that repo on the remote without any manual configuration.
- **SC-003**: Private repositories are accessible on the remote using either SSH agent forwarding or token-based authentication, with zero manual credential setup on the remote.
- **SC-004**: A failed repo clone does not prevent environment creation or block access to successfully cloned repos.
- **SC-005**: The feature works consistently across SSH, compose, container, and k8s-deploy environment types.

## Assumptions

- Git is installed on all target environments (SSH hosts, container images, Kubernetes pods). The existing provisioning pipeline ensures this for SSH environments.
- For SSH environments, the SSH client supports agent forwarding (`-A` flag).
- Token-based credentials reference environment variables already set on the local machine. The system does not manage token creation or rotation.
- Repos are cloned as siblings in the workspace directory (e.g., `~/workspace/repo-a/`, `~/workspace/repo-b/`). Nested structures are not supported.
- The Ansible provisioning pipeline (`cc-deck.build --target ssh`) does not consume the `repos` field. Provisioning handles tool installation; repo cloning is handled by `env create`.
- Worktree support is explicitly out of scope for this version. The YAML structure can accommodate an optional `worktrees` field in a future iteration.
