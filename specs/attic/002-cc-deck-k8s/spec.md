# Feature Specification: cc-deck (Kubernetes CLI)

**Feature Branch**: `002-cc-deck-k8s`
**Created**: 2026-03-03
**Status**: Draft
**Input**: Go CLI for deploying and managing Claude Code + Zellij sessions on Kubernetes/OpenShift with StatefulSets, credential profiles, egress NetworkPolicies, git sync, and persistent storage

## Purpose

cc-deck is a CLI tool that enables developers to deploy, configure, and manage Claude Code + Zellij sessions running on Kubernetes and OpenShift clusters. It solves the problem of setting up isolated, network-secured, persistent Claude Code environments on remote infrastructure, with support for multiple credential backends (Anthropic API and Google Vertex AI), easy session reconnection, and bidirectional git synchronization.

Target user: a developer who wants to run Claude Code sessions on a Kubernetes or OpenShift cluster they have access to, with proper security controls and persistent work directories.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Deploy a Claude Code Session (Priority: P1)

A developer runs `cc-deck deploy myproject` and a new Claude Code + Zellij session starts on their Kubernetes cluster. The session runs in a Pod with persistent storage for the work directory. The developer's default credential profile is used to configure Claude Code access.

**Why this priority**: Without deployment, nothing else works. This is the foundational capability.

**Independent Test**: Run `cc-deck deploy test-session`, verify a Pod is running with Claude Code accessible.

**Acceptance Scenarios**:

1. **Given** a valid kubeconfig and a configured credential profile, **When** the user runs `cc-deck deploy myproject`, **Then** a StatefulSet with one replica is created, a PVC is provisioned for persistent storage, and the Pod reaches Running state within 60 seconds
2. **Given** a deployment with `--profile vertex-prod`, **When** the Pod starts, **Then** the correct Vertex AI credentials and region are available to Claude Code inside the container
3. **Given** no profile specified, **When** the user runs `cc-deck deploy myproject`, **Then** the default profile from the config file is used
4. **Given** a session named "myproject" already exists, **When** the user runs `cc-deck deploy myproject`, **Then** an error is shown indicating the session already exists

---

### User Story 2 - Connect to a Session (Priority: P1)

A developer runs `cc-deck connect myproject` and is attached to the running Zellij session inside the Pod. The connection method is auto-detected, and the session details are remembered locally for quick reconnection.

**Why this priority**: Deployment without connection is useless. This completes the core workflow.

**Independent Test**: Deploy a session, then run `cc-deck connect myproject` and verify interactive terminal access.

**Acceptance Scenarios**:

1. **Given** a running session "myproject", **When** the user runs `cc-deck connect myproject`, **Then** an interactive terminal session opens attached to the Zellij instance in the Pod
2. **Given** a running session on OpenShift with a Route, **When** the user runs `cc-deck connect myproject --web`, **Then** the browser opens with the Zellij web client URL
3. **Given** a session was previously connected, **When** the user runs `cc-deck connect myproject` again after disconnecting, **Then** the session is reattached without re-entering connection details
4. **Given** a session that has been stopped or deleted, **When** the user runs `cc-deck connect myproject`, **Then** an informative error message is shown

---

### User Story 3 - Manage Credential Profiles (Priority: P1)

A developer configures multiple credential profiles (e.g., "anthropic-dev" with an API key, "vertex-prod" with GCP Vertex AI credentials) and switches between them when deploying sessions.

**Why this priority**: Credential management is required for any deployment. Profiles enable flexible multi-environment usage.

**Independent Test**: Create two profiles, deploy sessions with each, verify Claude Code uses the correct backend.

**Acceptance Scenarios**:

1. **Given** the user runs `cc-deck profile add anthropic-dev`, **When** they provide backend type and credential Secret reference, **Then** the profile is saved to the config file
2. **Given** multiple profiles exist, **When** the user runs `cc-deck profile list`, **Then** all profiles are shown with their backend type and default marker
3. **Given** a profile "vertex-prod" exists, **When** the user runs `cc-deck profile use vertex-prod`, **Then** it becomes the default profile for subsequent deployments
4. **Given** a profile references a non-existent credential Secret, **When** the user deploys with that profile, **Then** a clear error indicates the missing Secret and how to create it

---

### User Story 4 - Secure Egress with Network Policies (Priority: P2)

A developer deploys a session and by default all outbound network traffic is blocked except for the AI backend (Anthropic API or Vertex AI endpoints) and configured git hosts. Additional egress targets can be allowlisted.

**Why this priority**: Security is important but not blocking for initial deployment functionality.

**Independent Test**: Deploy a session, attempt to curl an unapproved external site from within the Pod, verify it's blocked.

**Acceptance Scenarios**:

1. **Given** a new deployment, **When** the Pod starts, **Then** a NetworkPolicy exists that denies all egress except DNS, the AI backend, and any configured allowlist entries
2. **Given** a deployment with `--allow-egress github.com`, **When** the NetworkPolicy is created, **Then** egress to github.com is permitted
3. **Given** a Vertex AI profile, **When** the NetworkPolicy is created, **Then** egress to `*.googleapis.com` is permitted
4. **Given** an Anthropic profile, **When** the NetworkPolicy is created, **Then** egress to `api.anthropic.com` is permitted

---

### User Story 5 - Git Repository Sync (Priority: P2)

A developer syncs their local git repository into the running Pod so Claude Code can work on it. Changes made by Claude in the Pod can be synced back locally.

**Why this priority**: Git sync is essential for real work but the session can be useful without it (e.g., starting fresh in the Pod).

**Independent Test**: Deploy a session, sync a local repo, verify files appear in the Pod, make changes, sync back.

**Acceptance Scenarios**:

1. **Given** a running session "myproject", **When** the user runs `cc-deck sync myproject`, **Then** the current local directory is copied into the Pod's persistent volume
2. **Given** Claude made changes in the Pod, **When** the user runs `cc-deck sync --pull myproject`, **Then** the changes are copied from the Pod to the local directory
3. **Given** a deployment with `--sync-dir /path/to/repo`, **When** the session starts, **Then** the directory is synced into the Pod automatically
4. **Given** a running session with git credentials configured, **When** Claude runs `git push` inside the Pod, **Then** the push succeeds to the remote repository

---

### User Story 6 - Session Lifecycle Management (Priority: P3)

A developer lists, stops, resumes, and deletes sessions. Session metadata persists locally for quick access.

**Why this priority**: Management operations improve the day-to-day experience but are not essential for basic usage.

**Independent Test**: Deploy a session, list it, delete it, verify all resources are cleaned up.

**Acceptance Scenarios**:

1. **Given** sessions exist on the cluster, **When** the user runs `cc-deck list`, **Then** all sessions are shown with name, namespace, status, age, profile, and connection info
2. **Given** a running session, **When** the user runs `cc-deck delete myproject`, **Then** the StatefulSet, PVC, Service, NetworkPolicy, and Route are all removed
3. **Given** a deleted session, **When** the user runs `cc-deck list`, **Then** the session no longer appears
4. **Given** a running session, **When** the user runs `cc-deck logs myproject`, **Then** the Pod logs are streamed to the terminal

---

### Edge Cases

- **Namespace doesn't exist**: If the target namespace doesn't exist, show an error with instructions to create it
- **Cluster unreachable**: If kubeconfig is invalid or cluster is unreachable, show a clear error before attempting deployment
- **PVC quota exceeded**: If PVC creation fails due to quota, display the quota error and suggest a smaller volume size
- **Pod scheduling fails**: If the Pod stays Pending (no resources, node selector mismatch), display the scheduling reason from Pod events
- **Stale local sessions**: If the local config lists sessions that no longer exist on the cluster, reconcile and mark them as "deleted"
- **Concurrent deploys**: If two users deploy the same session name in the same namespace, the second deploy should fail with a conflict error
- **Network policy not supported**: If the cluster doesn't support NetworkPolicies, warn the user but proceed with deployment

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: cc-deck MUST deploy Claude Code + Zellij sessions as StatefulSets with a single replica on Kubernetes and OpenShift clusters
- **FR-002**: Each deployment MUST create: a StatefulSet (replicas=1), a headless Service, a PersistentVolumeClaim (via volumeClaimTemplate), and a NetworkPolicy
- **FR-003**: On OpenShift, cc-deck MUST additionally create a Route for web client access
- **FR-004**: cc-deck MUST use a pre-built container image containing Zellij and Claude Code, configured at runtime via environment variables, ConfigMaps, and volume mounts
- **FR-005**: cc-deck MUST support credential profiles stored in an XDG-conformant config file (`$XDG_CONFIG_HOME/cc-deck/config.yaml`, defaulting to `~/.config/cc-deck/config.yaml`)
- **FR-006**: Profiles MUST support two backends: Anthropic (API key from a referenced credential source) and Google Vertex AI (GCP project, region, credentials from a referenced credential source or Workload Identity)
- **FR-007**: cc-deck MUST support three connection methods: terminal exec (default), Zellij web client (via port-forward or Route), and direct port-forward
- **FR-008**: cc-deck MUST track active sessions in the local config file with name, namespace, profile, and connection details
- **FR-009**: cc-deck MUST create a default-deny egress NetworkPolicy for each session, with allowlisted exceptions for DNS, the AI backend endpoints, and user-specified hosts
- **FR-010**: cc-deck MUST support bidirectional file sync between the local filesystem and the Pod's persistent volume
- **FR-011**: cc-deck MUST support mounting git credentials (SSH keys or tokens) into the Pod for remote repository access
- **FR-012**: cc-deck MUST support user-provided overlays for environment-specific customization of generated resources
- **FR-013**: cc-deck MUST provide session lifecycle commands: deploy, connect, list, delete, logs, sync
- **FR-014**: Pod names MUST be stable and predictable (via StatefulSet naming: `cc-deck-<session-name>-0`)
- **FR-015**: Work directories MUST persist across Pod restarts via PersistentVolumeClaims

### Key Entities

- **Session**: A running Claude Code + Zellij environment on a cluster. Attributes: name, namespace, profile, status, connection info, created timestamp
- **Profile**: A named credential configuration. Attributes: name, backend type (anthropic/vertex), credential references, model, permissions level, allowed egress hosts
- **Config**: Local configuration file. Contains: default profile, profiles list, active sessions list

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A developer can go from zero to a running Claude Code session on a cluster in under 2 minutes using `cc-deck deploy`
- **SC-002**: Reconnecting to an existing session via `cc-deck connect` takes under 5 seconds
- **SC-003**: Switching between Anthropic and Vertex AI backends requires only changing the `--profile` flag
- **SC-004**: A deployed session's egress is restricted to only the allowlisted destinations, verified by attempting blocked connections from within the Pod
- **SC-005**: Files synced into the Pod persist across Pod restarts
- **SC-006**: The CLI supports both Kubernetes and OpenShift clusters without requiring different commands

## Error Handling

- If the kubeconfig is missing or the cluster is unreachable, cc-deck MUST show a clear error with troubleshooting guidance
- If a referenced credential source does not exist, cc-deck MUST show an error explaining which credential is missing and how to create it
- If Pod scheduling fails (Pending state for > 60 seconds), cc-deck MUST display the scheduling failure reason from Pod events
- If PVC provisioning fails, cc-deck MUST display the storage error and suggest checking StorageClass and quotas
- If the container image pull fails, cc-deck MUST display the image pull error
- If sync fails (Pod not running, network error), cc-deck MUST show the specific failure reason

## Dependencies

- A Kubernetes 1.24+ or OpenShift 4.12+ cluster with access via kubeconfig
- A pre-built container image with Zellij and Claude Code installed
- Credentials pre-configured for the chosen AI backend
- NetworkPolicy support in the cluster's CNI plugin (for egress controls)

## Assumptions

- The developer has a valid kubeconfig with permissions to create StatefulSets, Services, PVCs, NetworkPolicies, and (on OpenShift) Routes in their target namespace
- The cluster has a default StorageClass for dynamic PVC provisioning
- The developer has pre-configured the required credentials for their profile
- The container image is accessible from the cluster's container registry
- DNS resolution within the cluster works (for NetworkPolicy DNS allowlisting)

## Out of Scope

- Building or managing the base container image (provided separately)
- Multi-tenant session sharing (each session is single-user)
- GPU support or model hosting (Claude Code uses remote API, not local inference)
- Automated credential rotation
- CI/CD integration
- Monitoring or alerting for running sessions
- Session auto-scaling (each session is exactly one Pod)
- Integration with the cc-zellij-plugin (the Zellij plugin runs independently inside the container)
- Repository restructuring (separate effort, brainstorm #03)
