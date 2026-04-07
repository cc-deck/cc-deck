# Feature Specification: SSH Remote Execution Environment

**Feature Branch**: `033-ssh-environment`  
**Created**: 2026-04-07  
**Status**: Draft  
**Input**: User description: "SSH Remote Execution Environment - run Claude Code on persistent remote machines via SSH, managing Zellij sessions, credential forwarding, and file sync over SSH connections"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Connect to a Remote Development Machine (Priority: P1)

A developer defines an SSH environment pointing to a persistent remote machine (e.g., a Hetzner VM or cloud dev box) and connects to it. The remote machine runs Zellij with the cc-deck plugin, and the developer works inside the remote session. When the developer disconnects, the remote Zellij session keeps running autonomously, allowing Claude Code to continue working.

**Why this priority**: This is the fundamental value proposition. Without the ability to define, create, and attach to an SSH environment, no other features matter. This story delivers the core "detach and walk away" workflow that distinguishes SSH environments from local ones.

**Independent Test**: Can be fully tested by defining an SSH environment in the environments file, running the create flow, attaching to the remote Zellij session, verifying Claude Code is accessible, detaching, and confirming the remote session persists.

**Acceptance Scenarios**:

1. **Given** a remote machine accessible via SSH with valid credentials, **When** the user defines an SSH environment with `type: ssh` and a `host` field in the environments file, **Then** the system accepts the definition and makes it available for creation.
2. **Given** a valid SSH environment definition, **When** the user creates the environment, **Then** the system runs pre-flight checks (SSH connectivity, OS detection, tool availability) and reports results for each check.
3. **Given** a successfully created SSH environment, **When** the user attaches, **Then** the system opens an SSH connection and attaches to (or creates) a remote Zellij session with the cc-deck layout.
4. **Given** the user is attached to a remote Zellij session, **When** the user detaches from Zellij (Ctrl+o d), **Then** the SSH connection closes and the remote Zellij session continues running.
5. **Given** the user is already inside a Zellij session on the local machine, **When** the user attempts to attach to an SSH environment, **Then** the system warns about nested Zellij and refuses to attach.

---

### User Story 2 - Pre-flight Bootstrap with Tool Installation (Priority: P1)

During environment creation, the system checks whether all required tools (Zellij, Claude Code, cc-deck CLI, cc-deck plugin) are installed on the remote machine. For each missing tool, the system offers to install it automatically based on the detected OS and architecture.

**Why this priority**: Without tools on the remote machine, the attach flow cannot work. The pre-flight checklist is part of the creation workflow and must be available from day one. Offering automated installation removes friction for first-time setup.

**Independent Test**: Can be tested by pointing at a bare remote machine (no Zellij, no Claude Code) and verifying each pre-flight check detects missing tools and, when the user accepts, installs them correctly.

**Acceptance Scenarios**:

1. **Given** a remote machine without Zellij installed, **When** the create flow runs pre-flight checks, **Then** the system detects Zellij is missing and offers to install it.
2. **Given** the user accepts the installation offer, **When** the system installs the tool, **Then** it downloads the correct binary for the detected OS/architecture and confirms the installation succeeded.
3. **Given** the user declines an installation offer, **When** the user chooses "skip", **Then** the system shows manual installation instructions and allows the user to press Enter after installing manually.
4. **Given** a remote machine running an unsupported OS/architecture, **When** the create flow detects the platform, **Then** the system warns and skips installation offers.

---

### User Story 3 - Credential Forwarding and Persistence (Priority: P2)

The developer configures how API credentials (for Claude Code) are forwarded to the remote machine. The system supports multiple credential modes (auto-detect, explicit API key, Vertex AI, Bedrock, none) and writes credentials to a file on the remote so they persist across Zellij detach/reattach cycles. Each attach refreshes the credential file with the latest local values.

**Why this priority**: Claude Code requires API credentials to function. Without credential forwarding, the remote session cannot use Claude Code. Credentials must also survive detach/reattach: new panes opened after reattaching need access to credentials, which requires a persistent credential file rather than env-var-only injection.

**Independent Test**: Can be tested by configuring different auth modes, attaching, detaching, reattaching, opening a new pane, and verifying that credentials are available in both old and new panes.

**Acceptance Scenarios**:

1. **Given** `auth: auto` in the environment definition, **When** the user attaches, **Then** the system detects available credentials from the local environment, writes them to a credential file on the remote, and ensures the remote shell sources that file.
2. **Given** `auth: api` in the definition, **When** the user attaches, **Then** the system writes `ANTHROPIC_API_KEY` to the remote credential file.
3. **Given** `auth: none` in the definition, **When** the user attaches, **Then** the system does not write any credential file (assumes credentials are already on the remote machine).
4. **Given** explicit credential names in the `credentials` list, **When** the user attaches, **Then** exactly those environment variables are written to the remote credential file.
5. **Given** the required credential is not available locally, **When** the user attempts to attach with a non-"none" auth mode, **Then** the system warns that the credential is missing and lets the user decide whether to continue without it.
6. **Given** `auth: vertex` with a `GOOGLE_APPLICATION_CREDENTIALS` file path, **When** the user attaches, **Then** the system copies the referenced credential file to the remote and sets the env var in the credential file to point to the remote copy.
7. **Given** a user has detached and reattached to an SSH environment, **When** a new pane is opened in Zellij, **Then** the new pane has access to the credentials from the most recent attach.
8. **Given** credentials have changed locally since the last attach, **When** the user reattaches, **Then** the credential file on the remote is updated with the new values.

---

### User Story 4 - Remote Status and Monitoring (Priority: P2)

The developer checks the status of SSH environments to see whether the remote Zellij session is running, what sessions are active, and whether the remote machine is reachable.

**Why this priority**: Users need visibility into remote sessions, especially since the remote runs autonomously after detaching. Status checks are essential for the "walk away and check later" pattern.

**Independent Test**: Can be tested by creating an SSH environment, attaching/detaching, and verifying status reports accurately reflect remote session state (running, not found, unreachable).

**Acceptance Scenarios**:

1. **Given** an SSH environment with a running remote Zellij session, **When** the user checks status, **Then** the system reports the session as running with session details.
2. **Given** an SSH environment where the remote Zellij session has been terminated, **When** the user checks status, **Then** the system reports the session as not found.
3. **Given** an SSH environment where the remote machine is unreachable, **When** the user checks status, **Then** the system reports a connection error with diagnostic details.
4. **Given** multiple SSH environments, **When** the user lists all environments, **Then** the system queries each remote in parallel and reports status within a reasonable time even if some remotes are slow or unreachable.

---

### User Story 5 - Refresh Credentials Without Attaching (Priority: P2)

The developer refreshes credentials on the remote machine without attaching to the Zellij session, so that a long-running autonomous Claude Code session regains API access after tokens expire.

**Why this priority**: Long-running sessions are the core value of SSH environments. If credentials expire (short-lived tokens, rotated API keys), the remote Claude Code session stops working. The user needs a lightweight way to push fresh credentials without disrupting the running session. This is a natural extension of credential persistence and is cheap to implement on top of the credential file mechanism.

**Independent Test**: Can be tested by creating an SSH environment, attaching, detaching, changing local credentials, running refresh-creds, and verifying the remote session picks up the new credentials.

**Acceptance Scenarios**:

1. **Given** an SSH environment with a running remote session, **When** the user runs refresh-creds, **Then** the system writes fresh credentials from the local environment to the remote credential file without attaching.
2. **Given** `auth: vertex` with a credential file, **When** the user runs refresh-creds, **Then** the system copies the updated credential file to the remote.
3. **Given** `auth: none`, **When** the user runs refresh-creds, **Then** the system reports that credential management is disabled for this environment.

---

### User Story 6 - File Synchronization (Priority: P3)

The developer pushes local files to the remote workspace or pulls remote files back to the local machine, using efficient incremental transfer.

**Why this priority**: File sync enables collaboration between local editing and remote execution. It is valuable but not blocking for the core workflow (users can also push code via git). This can be delivered after the core attach/detach cycle works.

**Independent Test**: Can be tested by pushing a local directory to the remote workspace, modifying files on the remote, and pulling them back, verifying file contents match.

**Acceptance Scenarios**:

1. **Given** a local directory and an SSH environment, **When** the user pushes files, **Then** the system syncs the local directory to the configured remote workspace using incremental file transfer.
2. **Given** files in the remote workspace, **When** the user pulls files, **Then** the system syncs the remote directory to the specified local path.
3. **Given** sync exclusion patterns are configured, **When** files are synced, **Then** excluded patterns (e.g., `.git`) are respected.
4. **Given** the sync tool is not available on the remote, **When** the user attempts push or pull, **Then** the system falls back to a basic copy mechanism and warns about the fallback.

---

### User Story 7 - Remote Command Execution (Priority: P3)

The developer runs commands on the remote machine without attaching to the full Zellij session, for quick operations like checking git status, running builds, or inspecting logs.

**Why this priority**: Exec is a convenience feature for quick remote operations. The core workflow does not depend on it since users can always attach and run commands interactively.

**Independent Test**: Can be tested by running a simple command via exec and verifying the output is returned correctly, including that it runs in the configured workspace directory.

**Acceptance Scenarios**:

1. **Given** a created SSH environment, **When** the user runs a command via exec, **Then** the command runs on the remote machine in the configured workspace directory and output is returned.
2. **Given** an unreachable remote machine, **When** the user attempts exec, **Then** the system reports the connection error clearly.

---

### User Story 8 - Harvest Git Commits (Priority: P3)

The developer retrieves git commits made by Claude Code on the remote machine, bringing them back to the local repository for review, PR creation, or integration.

**Why this priority**: Harvesting is the final step in the "remote autonomous work" cycle. It is important but depends on the remote session having done meaningful work first, so it is lower priority than the core connect/monitor/sync features.

**Independent Test**: Can be tested by having commits on the remote, running harvest, and verifying the commits appear in the local repository.

**Acceptance Scenarios**:

1. **Given** a remote repository with commits not present locally, **When** the user harvests, **Then** the system fetches those commits to the local repository.
2. **Given** the user requests a PR after harvest, **When** the harvest completes, **Then** the system creates a pull request from the harvested branch.

---

### Edge Cases

- What happens when SSH connectivity drops during an attach session? The SSH connection terminates, the remote Zellij session continues running, and the user can reattach later.
- What happens when the remote machine reboots? The Zellij session is lost. Status reports the session as not found. The user can reattach, which creates a new session.
- What happens when multiple users attach to the same SSH environment simultaneously? The system does not prevent this since Zellij supports shared sessions, but behavior depends on Zellij's multi-attach semantics.
- What happens when credential environment variables change locally between attach sessions? The system rewrites the credential file on the remote at each attach, so the latest local values are always pushed.
- What happens when credentials expire while a remote session is running autonomously? The remote Claude Code session loses API access. The user can run refresh-creds to push fresh credentials without attaching. For long-running sessions with expiring tokens, configuring the remote machine's own auth CLI (gcloud, aws) is recommended.
- What happens if the credential file on the remote is readable by other users? The system writes the file with mode 600 (owner-read-write only). If the remote filesystem does not support permissions, the system warns during pre-flight checks.
- What happens when rsync is unavailable on both local and remote machines? The push/pull operations fail with a clear error indicating that rsync (or scp as fallback) is required.
- What happens when the user deletes an SSH environment while a remote Zellij session is still running? Without the `--force` flag, deletion is refused. With `--force`, the remote Zellij session is killed before removing the state record.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST support a new environment type `ssh` that manages Zellij sessions on a remote machine over SSH connections.
- **FR-002**: System MUST accept SSH environment definitions with at minimum a `host` field (user@host format) and `type: ssh`.
- **FR-003**: System MUST support optional SSH configuration overrides: `port`, `identity-file`, `jump-host`, `ssh-config`, and `workspace`.
- **FR-004**: System MUST respect the user's `~/.ssh/config` by default for connection parameters not explicitly overridden.
- **FR-005**: System MUST run pre-flight checks during environment creation: SSH connectivity, OS/architecture detection, Zellij availability, Claude Code availability, cc-deck CLI availability, cc-deck plugin status, and credential verification.
- **FR-006**: System MUST offer automated installation for missing tools (Zellij, Claude Code, cc-deck CLI, cc-deck plugin) when the remote platform is supported (linux/amd64, linux/arm64).
- **FR-007**: System MUST detect nested Zellij sessions (via `$ZELLIJ` env var) and refuse to attach with a warning.
- **FR-008**: System MUST create a remote Zellij session with the `cc-deck` layout if one does not already exist when attaching.
- **FR-009**: System MUST replace the local process with the SSH connection when attaching (the user interacts directly with the remote Zellij).
- **FR-010**: System MUST keep the remote Zellij session running when the user detaches or the SSH connection drops.
- **FR-011**: System MUST support credential forwarding with configurable auth modes: `auto`, `api`, `vertex`, `bedrock`, `none`.
- **FR-012**: System MUST persist credentials to a file on the remote machine (with restrictive permissions) so that new Zellij panes pick up credentials after detach/reattach cycles.
- **FR-013**: System MUST refresh the remote credential file on every attach, writing the latest local credential values.
- **FR-014**: System MUST support file-based credentials (e.g., GCP service account JSON): when the definition references a local credential file, the system copies it to the remote and sets the env var to point to the remote copy.
- **FR-015**: System MUST support an explicit `credentials` list for forwarding arbitrary environment variables.
- **FR-016**: System MUST support arbitrary environment variables via the `env` field, set on the remote at connection time.
- **FR-017**: System MUST provide a credential refresh operation that updates the remote credential file without attaching to the Zellij session.
- **FR-018**: System MUST implement `Status` by querying the remote machine over SSH (no caching), with a per-host timeout.
- **FR-019**: System MUST implement `Push` and `Pull` using rsync over SSH, respecting configured exclusion patterns, with scp as a fallback when rsync is unavailable.
- **FR-020**: System MUST implement `Exec` to run commands on the remote machine in the configured workspace directory.
- **FR-021**: System MUST implement `Harvest` to retrieve git commits from the remote repository to the local repository.
- **FR-022**: System MUST treat `Start` and `Stop` as no-ops with informational warnings, since remote machine lifecycle is managed externally.
- **FR-023**: System MUST implement `Delete` to remove the state record and optionally kill the remote Zellij session (with `--force`).
- **FR-024**: System MUST update `LastAttached` timestamp in the state store on each attach.
- **FR-025**: System MUST comply with the Environment Interface behavioral contract (session naming via `cc-deck-<name>`, name validation, tool checks, state recording, cleanup on failure, running check before delete).
- **FR-026**: System MUST support parallel status reconciliation across multiple SSH environments.
- **FR-027**: System MUST use the system `ssh` binary for all SSH operations (not a Go SSH library) to ensure full compatibility with user SSH configurations, agents, and jump hosts.

### Key Entities

- **SSH Environment**: An environment record of type `ssh` stored in the state file, linking a name to a remote machine's SSH connection details and workspace path.
- **SSH Client**: A helper that wraps SSH operations (run, interactive attach, upload, download, connectivity check, remote info) for a specific remote host.
- **Pre-flight Check**: A verification step during environment creation that validates a specific prerequisite (connectivity, tool installation, credentials) and optionally offers remediation.
- **Credential Set**: A collection of environment variables determined by the auth mode, forwarded to the remote session at connection time.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users can define, create, attach to, and detach from an SSH environment in under 5 minutes (excluding tool installation time on the remote).
- **SC-002**: Remote Zellij sessions remain running after the user detaches, with 100% reliability for environments where the remote machine stays online.
- **SC-003**: Pre-flight checks complete within 30 seconds for a reachable remote machine with all tools already installed.
- **SC-004**: Status checks for a single SSH environment return within 10 seconds, including network latency.
- **SC-005**: File synchronization transfers only changed files (incremental sync), completing updates of small changes within 10 seconds for projects under 1 GB on a typical network.
- **SC-006**: Users can harvest remote git commits and optionally create a PR in a single operation.
- **SC-007**: Credentials persist across detach/reattach cycles: new panes opened after reattaching have access to the credentials from the most recent attach.
- **SC-008**: Credential refresh completes within 5 seconds without disrupting the running remote Zellij session.
- **SC-009**: All pre-existing cc-deck commands (env list, env status) continue to work unchanged for local and container environments after adding SSH support.

## Assumptions

- The remote machine is a Linux system (amd64 or arm64) running an SSH server with key-based authentication configured. Password-based authentication is not actively tested but may work if the system `ssh` binary supports it interactively.
- The remote machine has a persistent lifecycle managed externally (cloud provider, systemd, always-on VM). cc-deck does not manage machine provisioning, startup, or shutdown.
- The system `ssh` binary is available on the local machine's PATH.
- `rsync` is available on the local machine for push/pull operations. If rsync is unavailable on the remote, the system falls back to `scp`.
- The user has working SSH key-based authentication configured (either via ssh-agent, key files, or SSH config) before creating the environment.
- Credentials are persisted to a file on the remote (`~/.config/cc-deck/credentials.env`, mode 600) and sourced by the shell on pane startup. This ensures new panes pick up credentials after detach/reattach. The file is rewritten on each attach and can be refreshed independently via a dedicated command.
- File-based credentials (e.g., GCP service account JSON referenced by `GOOGLE_APPLICATION_CREDENTIALS`) are copied to the remote via the SSH connection. The credential file's env var is updated to point to the remote copy.
- For long-running sessions with short-lived tokens (AWS session tokens, Google Cloud access tokens), the recommended approach is to either use the refresh-creds command periodically or configure the remote machine's own auth CLI (gcloud, aws) for autonomous token renewal.
- Multi-session support is provided via `cc-deck-<name>` session naming (one Zellij session per environment name on each remote host).
- Port forwarding (`-L`, `-R`) is out of scope for the initial implementation and can be added later as an enhancement.
- Connection multiplexing (SSH ControlMaster) is out of scope for the initial implementation and can be added as a performance optimization later.
- Remote hook event forwarding (back to local for monitoring) is out of scope and may be addressed in a future feature.
