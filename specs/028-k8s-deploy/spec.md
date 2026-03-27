# Feature Specification: Kubernetes Deploy Environment

**Feature Branch**: `028-k8s-deploy`
**Created**: 2026-03-27
**Status**: Draft
**Input**: User description: "K8sDeployEnvironment implementing the Environment interface with StatefulSet-based persistent workloads, credential management via K8s Secrets and External Secrets Operator, MCP sidecar generation, network policy, OpenShift detection, and integration tests with kind"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Deploy a Persistent K8s Environment (Priority: P1)

A developer wants to run a Claude Code session on a remote Kubernetes cluster rather than locally. They create a new environment of type `k8s-deploy`, specifying their cluster credentials and an AI API key. The system provisions a persistent workload (with stable naming and storage), and the developer attaches to work inside it. When they detach and come back later, their workspace (files, git history, installed tools) is still there.

**Why this priority**: This is the core value proposition. Without a working deploy-attach-persist loop, nothing else matters.

**Independent Test**: Can be tested by creating a k8s-deploy environment against a kind cluster, attaching, creating a file, detaching, re-attaching, and verifying the file persists. Delivers immediate value: remote persistent coding environments.

**Acceptance Scenarios**:

1. **Given** a valid kubeconfig and namespace, **When** the user runs `cc-deck env create my-env --type k8s-deploy --namespace cc-deck --credential anthropic-api-key=sk-ant-...`, **Then** the system creates a StatefulSet (replicas=1), headless Service, ConfigMap, and PVC in the specified namespace, and reports success with the environment name.
2. **Given** a deployed environment, **When** the user runs `cc-deck env attach my-env`, **Then** the system connects via kubectl exec into the running Pod and opens a Zellij session with the cc-deck sidebar layout.
3. **Given** an attached session where the user created files, **When** the user detaches and later re-attaches, **Then** the files persist because the PVC retains data across Pod restarts.
4. **Given** a running environment, **When** the user runs `cc-deck env stop my-env`, **Then** the workload scales to zero replicas and the PVC is preserved.
5. **Given** a stopped environment, **When** the user runs `cc-deck env start my-env`, **Then** the workload scales back to one replica, re-mounting the preserved PVC, and the workspace contents are intact.
6. **Given** a running environment, **When** the user runs `cc-deck env delete my-env --force`, **Then** all K8s resources (StatefulSet, Service, ConfigMap, NetworkPolicy, PVC) are removed and the state record is cleared.
7. **Given** a running environment, **When** the user runs `cc-deck env delete my-env --force --keep-volumes`, **Then** all K8s resources except the PVC are removed, preserving workspace data for potential reuse.

---

### User Story 2 - Credential Management (Priority: P1)

A developer needs to securely provide AI backend credentials (Anthropic API key, Vertex AI service account, Bedrock credentials) to their remote environment. They can either pass credentials inline at creation time or reference an existing Kubernetes Secret. For teams with a centralized vault, the developer can configure the system to generate External Secrets Operator resources that sync credentials from an external secret management system.

**Why this priority**: Without credentials, the AI agents inside the environment cannot function. Security of credential handling is foundational.

**Independent Test**: Can be tested by creating an environment with inline credentials and verifying the Secret exists with correct data, then creating another with `--existing-secret` and verifying no new Secret is created but the volume mount still works.

**Acceptance Scenarios**:

1. **Given** an inline credential flag (`--credential anthropic-api-key=sk-ant-...`), **When** the environment is created, **Then** a Kubernetes Secret is created with the specified key-value pair and is volume-mounted (not injected as env var) into the workload at `/run/secrets/cc-deck/`.
2. **Given** an existing Secret reference (`--existing-secret my-api-keys`), **When** the environment is created, **Then** no new Secret is created, and the existing Secret is volume-mounted into the workload.
3. **Given** an External Secrets Operator configuration (`--secret-store vault --secret-store-ref my-vault --secret-path secret/data/cc-deck/anthropic`), **When** the environment is created, **Then** an ExternalSecret custom resource is generated that references the user's SecretStore and syncs the specified secret path into a Kubernetes Secret.
4. **Given** credentials provided inline, **When** the environment is deleted, **Then** the generated Secret is also deleted as part of resource cleanup.
5. **Given** an `--existing-secret` reference, **When** the environment is deleted, **Then** the referenced Secret is NOT deleted (it is user-managed).
6. **Given** multiple credential sources (some inline, some existing), **When** the environment is created, **Then** all sources are volume-mounted and accessible at their respective paths.

---

### User Story 3 - Network Egress Filtering (Priority: P2)

A developer wants to restrict what their remote environment can reach over the network. By default, the system generates a deny-all egress NetworkPolicy with allowlisted domains for the AI backend. The developer can add custom allowed domains. The domain resolution and UX is consistent with how network filtering works for Podman and compose environments.

**Why this priority**: Network isolation is a security requirement for many organizations, but environments are functional without it if the cluster has no CNI enforcement.

**Independent Test**: Can be tested by creating an environment and verifying the NetworkPolicy resource exists with the expected egress rules containing resolved IP addresses for the AI backend domains.

**Acceptance Scenarios**:

1. **Given** default creation (no `--no-network-policy` flag), **When** the environment is created, **Then** a NetworkPolicy with deny-all egress and allowlisted rules for the configured AI backend domains is generated.
2. **Given** the `--no-network-policy` flag, **When** the environment is created, **Then** no NetworkPolicy resource is created.
3. **Given** custom allowed domains (`--allow-domain github.com --allow-domain registry.npmjs.org`), **When** the environment is created, **Then** the NetworkPolicy includes egress rules for those domains in addition to the AI backend domains.
4. **Given** a domain group configured in the user's domain configuration file, **When** the user specifies `--allow-group dev-tools`, **Then** all domains in that group are resolved and added to the egress rules.
5. **Given** the same domain filtering configuration, **When** the user creates a Podman environment versus a K8s environment, **Then** the filtering UX (flags, domain groups, domain config file) is identical.

---

### User Story 4 - MCP Sidecar Containers (Priority: P2)

A developer has a build manifest (`cc-deck-build.yaml`) that defines MCP servers (GitHub MCP, Slack MCP, etc.) alongside the main cc-deck image. When deploying to Kubernetes, the system generates sidecar containers within the same Pod for each MCP server, sharing the loopback interface so no TLS is needed between them. Credentials for MCP servers are also handled through the same Secret mechanism.

**Why this priority**: MCP sidecars extend the AI agent's capabilities significantly, but a basic environment without MCP servers is still functional.

**Independent Test**: Can be tested by creating an environment from a build directory containing MCP definitions and verifying the Pod spec contains the expected sidecar containers with correct image, ports, and environment references.

**Acceptance Scenarios**:

1. **Given** a build manifest with MCP server entries, **When** the user runs `cc-deck env create my-env --type k8s-deploy --build-dir ./my-project`, **Then** each MCP entry becomes a sidecar container in the same Pod sharing the network namespace (localhost communication).
2. **Given** MCP sidecars with credential requirements, **When** the environment is created, **Then** MCP credentials are stored in Secrets and volume-mounted into the respective sidecar containers.
3. **Given** an environment with MCP sidecars, **When** the user attaches, **Then** the MCP servers are reachable from the main container via localhost on their configured ports.
4. **Given** a build manifest without MCP entries, **When** the environment is created, **Then** the Pod contains only the main cc-deck container (no sidecars).

---

### User Story 5 - OpenShift Compatibility (Priority: P2)

A developer deploys to an OpenShift cluster instead of vanilla Kubernetes. The system detects the OpenShift API and generates additional platform-specific resources: a Route for web access (instead of Ingress) and an EgressFirewall (in addition to NetworkPolicy) for network filtering. The developer does not need to specify that the cluster is OpenShift; detection is automatic.

**Why this priority**: OpenShift is a primary deployment target, but the system must work on vanilla Kubernetes first.

**Independent Test**: Can be tested by mocking the API discovery response to indicate OpenShift capabilities and verifying the generated resources include Route and EgressFirewall.

**Acceptance Scenarios**:

1. **Given** a cluster that reports OpenShift API groups (Route, EgressFirewall), **When** the environment is created, **Then** the system generates an OpenShift Route for the Zellij web client port in addition to vanilla K8s resources.
2. **Given** a cluster that does not report OpenShift API groups, **When** the environment is created, **Then** only vanilla Kubernetes resources are generated (no Route, no EgressFirewall).
3. **Given** an OpenShift cluster with network filtering enabled, **When** the environment is created, **Then** both a NetworkPolicy and an EgressFirewall are generated with consistent egress rules.
4. **Given** an OpenShift Route is created, **When** the user runs `cc-deck env status my-env`, **Then** the output includes the Route URL for web access.

---

### User Story 6 - File Synchronization (Priority: P2)

A developer wants to transfer code between their local machine and the remote environment. They can push files in (to seed a workspace) and pull files out (to retrieve results). For git-based projects, the developer can use the git harvesting strategy that tunnels git operations over kubectl exec, preserving full commit history for review and PR creation.

**Why this priority**: Sync is essential for the development workflow, but initial testing can be done by editing files directly inside the attached environment.

**Independent Test**: Can be tested by pushing a local directory into the environment, verifying files appear at the remote path, then pulling them back and verifying content matches.

**Acceptance Scenarios**:

1. **Given** a running environment, **When** the user runs `cc-deck env push my-env ./src`, **Then** the contents of `./src` are transferred into the environment at `/workspace/src` via tar-over-exec.
2. **Given** a running environment with files at `/workspace/results`, **When** the user runs `cc-deck env pull my-env /workspace/results ./local-results`, **Then** the remote files are transferred to the local path.
3. **Given** a running environment and a git repository, **When** the user runs `cc-deck env push my-env --git`, **Then** the local git repository is pushed into the environment via `ext::kubectl exec`, and the remote workspace is updated to match.
4. **Given** an environment where the agent has made git commits, **When** the user runs `cc-deck env harvest my-env -b agent-work`, **Then** the agent's commits are fetched via `ext::kubectl exec` and placed on a local branch named `agent-work`.
5. **Given** a harvest with `--pr`, **When** the harvest completes, **Then** a pull request is created from the harvested branch.
6. **Given** sync exclude patterns configured in the environment definition, **When** files are pushed, **Then** excluded patterns (e.g., `node_modules`, `.git`, `target`) are not transferred.

---

### User Story 7 - Integration Tests with kind (Priority: P2)

A developer working on the k8s-deploy code runs integration tests locally against a kind cluster. The tests verify the full lifecycle (create, list, status, start, stop, delete) and resource correctness. The same tests run in GitHub Actions CI to catch regressions before merge.

**Why this priority**: Integration tests validate that the K8s API interactions work correctly, catching issues that unit tests against mock clients would miss.

**Independent Test**: Can be tested by running `go test -tags integration ./internal/integration/` against a kind cluster and verifying all tests pass.

**Acceptance Scenarios**:

1. **Given** a kind cluster is running and a test namespace exists, **When** the integration test suite runs, **Then** all core lifecycle tests (create, start, stop, delete) pass within 5 minutes.
2. **Given** a create test runs, **When** the test completes, **Then** a StatefulSet, headless Service, ConfigMap, PVC, and NetworkPolicy all exist in the test namespace with correct labels and configuration.
3. **Given** a deployed session exists, **When** the list function is called, **Then** the session appears in the results with correct name, namespace, and status.
4. **Given** a session name that is already deployed, **When** a second create with the same name is attempted, **Then** a duplicate conflict error is returned without modifying existing resources.
5. **Given** a push to any branch or a pull request is opened, **When** the CI workflow triggers, **Then** a kind cluster is created, a stub image is loaded, a test namespace is set up, and the integration tests run and report pass/fail status.

---

### User Story 8 - Environment Status and Listing (Priority: P3)

A developer wants to see all their environments across types (local, Podman, compose, k8s-deploy) in a single list. For k8s-deploy environments, the list shows cluster namespace, storage size, and connection state. A detailed status command reads session information from inside the running environment.

**Why this priority**: Listing and status are observability features that improve UX but are not required for core functionality.

**Independent Test**: Can be tested by creating a k8s-deploy environment, running `cc-deck env list`, and verifying it appears alongside other environment types with the expected columns.

**Acceptance Scenarios**:

1. **Given** one or more k8s-deploy environments exist, **When** the user runs `cc-deck env list`, **Then** k8s-deploy environments appear in the unified list alongside other types, showing name, type, state, namespace, and age.
2. **Given** a running k8s-deploy environment, **When** the user runs `cc-deck env status my-env`, **Then** the output shows detailed information including Pod status, PVC size, namespace, profile, uptime, and (via exec) the agent sessions running inside.
3. **Given** an environment whose Pod was deleted externally (e.g., by an admin), **When** `cc-deck env list` runs, **Then** the status is reconciled against the K8s API and shows the correct state (error or stopped), not stale cached state.

---

### Edge Cases

- What happens when the specified namespace does not exist? The system reports a clear error and does not create any resources.
- What happens when the kubeconfig is invalid or the cluster is unreachable? The system fails fast with a descriptive error before attempting resource creation.
- What happens when the PVC storage class does not exist on the cluster? The StatefulSet creation fails, and the system cleans up any partially created resources (Service, ConfigMap, NetworkPolicy) before reporting the error.
- What happens when the user tries to create an environment with a name that already exists in the state store? A name conflict error is returned.
- What happens when the Pod cannot reach Running state within the timeout? The system reports a timeout error with the Pod's current status and events for debugging.
- What happens when the cluster does not enforce NetworkPolicies? The NetworkPolicy resource is still created (it is a no-op on clusters without a CNI that enforces it), and the system does not attempt to verify enforcement.
- What happens when ESO is not installed but `--secret-store` is specified? The system reports an error indicating that the External Secrets Operator CRDs are not available on the cluster.
- What happens when an environment is deleted but some resources fail to clean up? The system logs warnings for resources it could not delete and removes the state record, noting the partially cleaned state.

## Clarifications

### Session 2026-03-27

- Q: Should `env delete` remove PVCs (permanent workspace data loss) or preserve them? → A: Delete PVC by default; add `--keep-volumes` flag for opt-in preservation (consistent with container environment's KeepVolumes pattern).
- Q: Should Pod readiness timeout be configurable via CLI? → A: Add `--timeout` flag with 5m default, following kubectl convention.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST implement the `Environment` interface (as defined in the environment interface contract) for the `k8s-deploy` type, satisfying all behavioral requirements including nested Zellij detection, session layout creation, auto-start on attach, timestamp updates, name validation, tool availability checks, cleanup on failure, and state reconciliation.
- **FR-002**: System MUST create a StatefulSet (replicas=1) with a headless Service, ConfigMap, and PVC via volumeClaimTemplates for each k8s-deploy environment.
- **FR-003**: System MUST support Start (scale to 1) and Stop (scale to 0) operations that preserve the PVC.
- **FR-004**: System MUST support credential input via inline key-value pairs (`--credential key=val`) and via reference to existing Kubernetes Secrets (`--existing-secret name`).
- **FR-005**: System MUST volume-mount credentials at `/run/secrets/cc-deck/` (not inject as environment variables).
- **FR-006**: System MUST support External Secrets Operator integration by generating ExternalSecret custom resources when `--secret-store` is specified.
- **FR-007**: System MUST detect ESO CRD availability on the cluster before generating ExternalSecret resources, and report a clear error if ESO is not installed.
- **FR-008**: System MUST generate a deny-all egress NetworkPolicy with allowlisted domains by default, using the same domain resolution logic as Podman and compose environments (from `internal/network/`).
- **FR-009**: System MUST support `--no-network-policy`, `--allow-domain`, and `--allow-group` flags consistent with other environment types.
- **FR-010**: System MUST detect OpenShift API groups (Route, EgressFirewall) via API discovery and generate platform-specific resources when available.
- **FR-011**: System MUST generate sidecar containers from the build manifest's MCP entries, placing them in the same Pod with shared network namespace.
- **FR-012**: System MUST support tar-over-exec file synchronization (push and pull) for the copy strategy.
- **FR-013**: System MUST support git harvesting via `ext::kubectl exec` for the git-harvest sync strategy.
- **FR-014**: System MUST connect to environments via `kubectl exec` by default, using the predictable Pod name (`cc-deck-<name>-0`).
- **FR-015**: System MUST register the `k8s-deploy` type in the environment factory.
- **FR-016**: System MUST update the CLI command handling to accept k8s-deploy-specific flags (`--namespace`, `--kubeconfig`, `--storage-size`, `--storage-class`, `--secret-store`, `--build-dir`, `--keep-volumes`, `--timeout`).
- **FR-017**: System MUST reconcile state against the K8s API when listing or querying status, updating stale records.
- **FR-018**: System MUST delete all generated K8s resources (including PVC) on environment deletion by default, and preserve user-managed resources (existing Secrets). When `--keep-volumes` is specified, the PVC is preserved.
- **FR-019**: System MUST clean up partially created resources when creation fails partway through.
- **FR-020**: System MUST include integration tests using kind that verify the full lifecycle (create, start, stop, delete) and resource correctness, runnable both locally and in GitHub Actions CI.

### Key Entities

- **K8s Deploy Environment**: A persistent remote execution environment backed by a StatefulSet, identified by name, associated with a namespace, kubeconfig, and optional credential profile.
- **Credential Source**: A provider of secrets for the environment. Can be inline (user-provided key-value), existing (pre-created K8s Secret), or external (ESO-backed from a vault).
- **MCP Sidecar**: An additional container within the same Pod, generated from a build manifest's MCP entry, sharing the Pod's network namespace for localhost communication.
- **Network Policy**: An egress filtering rule set derived from domain resolution, applied as a Kubernetes NetworkPolicy resource (and optionally an OpenShift EgressFirewall).

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A developer can create, attach to, stop, start, and delete a k8s-deploy environment through the same `cc-deck env` commands used for all other environment types.
- **SC-002**: Credentials are never exposed as environment variables; all secrets are volume-mounted and accessible only as files.
- **SC-003**: The full create-attach-stop-start-delete lifecycle completes successfully on both a kind cluster and a full OpenShift cluster.
- **SC-004**: Integration tests covering core lifecycle pass in under 5 minutes on both local kind clusters and GitHub Actions CI.
- **SC-005**: Network filtering flags (`--allow-domain`, `--allow-group`, `--no-network-policy`) produce identical UX across Podman, compose, and k8s-deploy environments.
- **SC-006**: Environments with MCP sidecars allow the main container to reach all MCP servers via localhost on their configured ports.
- **SC-007**: OpenShift-specific resources (Route, EgressFirewall) are generated automatically when the platform is detected, without requiring user flags.
- **SC-008**: External Secrets Operator integration works with any ESO-compatible SecretStore (Vault, AWS Secrets Manager, Azure Key Vault, GCP Secret Manager) without cc-deck being coupled to a specific backend.

## Assumptions

- The target cluster has a working kubeconfig accessible to the user.
- The container image used for the environment is pre-built and available in a registry accessible from the cluster. Image building is handled by the existing build pipeline (spec 018).
- The cluster has sufficient RBAC permissions for the user to create StatefulSets, Services, ConfigMaps, Secrets, PVCs, and NetworkPolicies in the target namespace.
- For ESO integration, the External Secrets Operator and at least one SecretStore are pre-configured by the cluster administrator. cc-deck generates ExternalSecret resources but does not install or configure ESO itself.
- For OpenShift features, the user has permissions to create Routes and EgressFirewalls.
- The stub container image for integration tests is a minimal Alpine image with `sleep infinity` as the entrypoint, sufficient for lifecycle testing without requiring the full cc-deck demo image.
- Pod readiness timeout defaults to 5 minutes (configurable via `--timeout`). This is sufficient for most clusters but may need adjustment for resource-constrained environments.

## Dependencies

- **023-env-interface** (completed): Provides the Environment interface, state store, factory pattern, and CLI command structure.
- **022-network-filtering** (nearly complete): Provides the domain resolution logic and domain group configuration in `internal/network/`.
- **018-build-manifest** (nearly complete): Provides the build manifest format (`cc-deck-build.yaml`) from which MCP sidecar definitions are read.
- **017-base-image** (completed): Provides the container image that runs inside the deployed environment.

## Contract Reference

This implementation MUST satisfy the behavioral contract defined in `specs/023-env-interface/contracts/environment-interface.md`. Key behavioral requirements:

- **Attach**: Nested Zellij detection, session creation with `--layout cc-deck`, auto-start if stopped, timestamp update.
- **Create**: Name validation via `ValidateEnvName()`, tool availability check (kubectl), state recording, cleanup on failure.
- **Delete**: Refuse if running unless force=true, best-effort cleanup with warnings, state removal.
- **Status**: Reconcile stored state against K8s API (Pod/StatefulSet status).
