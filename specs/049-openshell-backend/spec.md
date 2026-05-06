# Feature Specification: OpenShell Backend for cc-deck

**Feature Branch**: `049-openshell-backend`
**Created**: 2026-04-30
**Status**: Draft
**Input**: Brainstorm document `brainstorm/01-cc-deck-openshell-backend.md`

## Clarifications

### Session 2026-04-30

- Q: How should OpenShell sandbox states map to cc-deck's InfraState model? → A: Map to existing InfraState values (creating/error -> "starting", running -> "running", suspended -> "stopped", deleted -> "not found"). Do not add new state values to cc-deck's core model.
- Q: How should concurrent attach from two cc-deck instances be handled? → A: Single attach only. Second instance receives a "workspace already attached" error. Matches existing SSH and container backend behavior.
- Q: Should TLS be required for gateway connections? → A: TLS optional with warning. Plaintext allowed for localhost connections. Warning emitted for non-localhost connections without TLS.

### Session 2026-05-04

- Q: How should cc-deck handle OpenShell proto versioning? → A: Pin proto files to a specific gateway release tag and document the minimum compatible gateway version. Update proto files explicitly when upgrading gateway compatibility.
- Q: How should cc-deck communicate sandbox operation failures to the user? → A: Print structured error messages to stderr with gateway error context and remediation hints (e.g., "sandbox image not found locally, the gateway may be pulling it"). Matches existing cc-deck CLI error patterns.
- Q: Should cc-deck add logging for OpenShell operations? → A: Debug-level logging of gRPC call outcomes (connect, create, attach, delete) in cc-deck's existing log output. No new metrics infrastructure for the MVP.
- Q: How should credentials (e.g., Anthropic API key) be handled for the sandbox? → A: cc-deck passes the provider name from the workspace definition only. The OpenShell gateway's provider mechanism handles all credential injection. cc-deck does not touch secrets.

## User Scenarios & Testing

### User Story 1 - Create a Sandboxed Workspace (Priority: P1)

A developer wants to start a Claude Code session inside an OpenShell sandbox so that the agent's network access, filesystem access, and credentials are restricted by policy. They run a single cc-deck command to create the workspace, and cc-deck provisions the sandbox, starts Zellij inside it, and reports that the workspace is ready.

**Why this priority**: Without sandbox creation, no other feature works. This is the foundational capability that makes OpenShell workspaces possible.

**Independent Test**: Create an OpenShell workspace with `cc-deck create --type openshell my-workspace`, verify that the sandbox is running and Zellij is active inside it.

**Acceptance Scenarios**:

1. **Given** the OpenShell gateway is running locally with the Podman driver, **When** the user runs `cc-deck create --type openshell my-workspace`, **Then** a sandbox is created via gRPC, Zellij starts as the agent process, and the workspace status shows as "running".
2. **Given** the OpenShell gateway is not reachable, **When** the user runs `cc-deck create --type openshell my-workspace`, **Then** the command fails with a clear error message indicating the gateway is unavailable.
3. **Given** a workspace definition YAML specifies a custom network policy, **When** the workspace is created, **Then** the sandbox applies that policy (verified by checking denied connections in OCSF logs).

---

### User Story 2 - Attach to a Sandboxed Workspace (Priority: P1)

A developer wants to open their terminal and connect to an existing sandboxed workspace. They run the attach command and land inside a Zellij session where Claude Code is running. The session survives disconnects; if the SSH tunnel drops, the developer can reattach without losing context.

**Why this priority**: Attach is the primary interaction model. Developers create once and attach many times. Without attach, workspaces are not usable.

**Independent Test**: Create a workspace, detach, then reattach and verify the Zellij session and Claude Code context are preserved.

**Acceptance Scenarios**:

1. **Given** a running OpenShell workspace, **When** the user runs `cc-deck attach my-workspace`, **Then** an SSH tunnel is established to the sandbox and the user lands in the Zellij session.
2. **Given** the user is attached to a workspace, **When** the SSH tunnel drops (network interruption), **Then** the Zellij session inside the sandbox continues running.
3. **Given** the SSH tunnel was dropped, **When** the user runs `cc-deck attach my-workspace` again, **Then** a new SSH tunnel is established and the user reconnects to the same Zellij session with all panes and Claude Code context intact.

---

### User Story 3 - Sync Files Into and Out of the Sandbox (Priority: P2)

A developer wants to push project files from their local machine into the sandbox before Claude Code starts working, and pull results (generated code, test outputs) back to the host after the agent finishes. File sync works over the SSH tunnel and respects exclusion lists (node_modules, .git, build artifacts).

**Why this priority**: File sync enables the "develop locally, execute in sandbox" workflow. Without it, developers must clone repos inside the sandbox manually, which is slower and doesn't integrate with cc-deck's harvest workflow.

**Independent Test**: Push a project directory into the sandbox, verify files appear at the expected path. Run Claude Code. Pull results back and verify they match what was generated.

**Acceptance Scenarios**:

1. **Given** a running workspace and a local project directory, **When** the user runs `cc-deck push my-workspace`, **Then** the project files appear inside the sandbox at the configured workspace path, excluding patterns in the exclusion list.
2. **Given** Claude Code has generated files inside the sandbox, **When** the user runs `cc-deck pull my-workspace`, **Then** the generated files are transferred to the local project directory.
3. **Given** a workspace with git initialized inside the sandbox, **When** the user runs `cc-deck harvest my-workspace`, **Then** git commits from the sandbox are fetched to the local repository.

---

### User Story 4 - Delete a Workspace and Clean Up (Priority: P2)

A developer wants to remove a workspace and all its associated resources (sandbox container, SSH tunnel, stored state). The delete command tears everything down and confirms cleanup.

**Why this priority**: Clean lifecycle management prevents resource leaks (orphaned containers, stale state entries).

**Independent Test**: Create a workspace, attach, do some work, then delete. Verify no Podman containers, SSH tunnels, or cc-deck state entries remain.

**Acceptance Scenarios**:

1. **Given** a running workspace, **When** the user runs `cc-deck delete my-workspace`, **Then** the Zellij session is killed, the sandbox is destroyed via gRPC, and stored state is removed.
2. **Given** a workspace whose gateway is unreachable, **When** the user runs `cc-deck delete my-workspace --force`, **Then** cc-deck removes its local state and warns that the sandbox may still be running.

---

### User Story 5 - Execute Commands Inside the Sandbox (Priority: P3)

A developer wants to run one-off commands inside the sandbox without attaching to the full Zellij session. They use the exec command and see the output in their terminal.

**Why this priority**: Useful for scripting, automation, and quick inspections, but not required for the primary interactive workflow.

**Independent Test**: Create a workspace, run `cc-deck exec my-workspace -- ls /sandbox`, verify output shows the sandbox filesystem.

**Acceptance Scenarios**:

1. **Given** a running workspace, **When** the user runs `cc-deck exec my-workspace -- echo hello`, **Then** the command executes inside the sandbox and "hello" is printed to the terminal.
2. **Given** a workspace that is stopped, **When** the user runs exec, **Then** the command fails with a clear error that the sandbox is not running.

---

### Edge Cases

- What happens when the gateway restarts while a workspace is running? The sandbox (Podman container) survives, but the gateway loses its session lease. cc-deck detects the stale connection and re-registers with the gateway on next attach. Error message: "gateway connection lost, reconnecting" with remediation hint.
- What happens when the user creates a workspace but the sandbox image is not available locally? The gateway pulls the image, but this may take time. cc-deck prints "sandbox image not found locally, the gateway may be pulling it" to stderr and continues polling until the sandbox reaches "running" state or times out.
- What happens when two cc-deck instances try to attach to the same workspace? Only one SSH session is active at a time. The second instance receives a "workspace already attached" error with a hint to detach the other session first.
- What happens when the sandbox runs out of disk space? Claude Code may fail silently. cc-deck surfaces the gRPC error from the gateway with context (e.g., "sandbox exec failed: no space left on device") and a remediation hint to clean up or resize.

## Requirements

### Functional Requirements

- **FR-001**: The system MUST add a new workspace type `openshell` that creates and manages workspaces inside OpenShell sandboxes.
- **FR-002**: The system MUST communicate with the OpenShell gateway via the `openshell` CLI binary, using correct CLI syntax for all sandbox operations (create, delete, get, exec, upload, download).
- **FR-003**: The system MUST provision sandboxes with a container image that includes Zellij and Claude Code, with Zellij as the agent command.
- **FR-004**: The system MUST attach to sandboxes interactively via `openshell sandbox exec --tty`, running Zellij attach inside the sandbox.
- **FR-005**: The system MUST detect attach session disconnections and allow re-attach on the next attempt without losing the Zellij session inside the sandbox. Zellij runs independently inside the sandbox and survives client disconnections.
- **FR-006**: The system MUST support file synchronization (push/pull) via the `openshell sandbox upload` and `openshell sandbox download` CLI commands.
- **FR-007**: The system MUST support git commit harvesting from the sandbox using the git ext:: protocol over `openshell sandbox exec`.
- **FR-008**: The system MUST implement the InfraManager interface, mapping Start to CreateSandbox and Stop to DeleteSandbox.
- **FR-009**: The system MUST reconcile stored workspace state with actual sandbox state (via GetSandbox) to handle gateway restarts and out-of-band changes. OpenShell states map to cc-deck's existing InfraState: creating/error to "starting", running to "running", suspended to "stopped", deleted to "not found".
- **FR-010**: The system MUST accept gateway connection configuration from workspace definition YAML, with environment variable fallback (`OPENSHELL_GATEWAY_URL`). TLS is optional: plaintext is allowed for localhost connections, but the system MUST emit a warning for non-localhost connections without TLS.
- **FR-013**: The system MUST enforce single-attach semantics. If a workspace already has an active attach session, a second attach attempt MUST fail with a clear "workspace already attached" error.
- **FR-011**: The system MUST apply network policy to sandboxes, using a default policy that allows inference APIs, package registries, and git hosting, with user customization via workspace definitions.
- **FR-012**: The system MUST register the OpenShell workspace type in the factory so that `cc-deck create --type openshell` works.
- **FR-014**: The system MUST log all CLI invocation outcomes (create, attach, delete, exec, upload, download) at debug level using cc-deck's existing log output. No new metrics infrastructure is required for the initial implementation.

### Key Entities

- **OpenShellWorkspace**: A cc-deck workspace backed by an OpenShell sandbox. Holds sandbox name, gateway connection info, attach session state, and workspace definition reference.
- **Sandbox Image**: A container image containing Zellij, Claude Code, and development tools. Built from a Dockerfile provided by cc-deck or the user.
- **Network Policy**: An OPA/Rego policy file defining which hosts, ports, and HTTP methods the sandbox can access. Stored in workspace definition or as a standalone YAML file.
- **Gateway Connection**: Configuration for reaching the OpenShell gateway (address, port, optional TLS settings). TLS is not required for localhost but recommended for remote gateways. Resolved from workspace definition or environment variable.
- **Provider**: An optional name referencing an OpenShell credential provider. cc-deck passes this name to CreateSandbox; the gateway injects the corresponding credentials (e.g., API keys) into the sandbox environment. cc-deck never handles secrets directly.

## Success Criteria

### Measurable Outcomes

- **SC-001**: A developer can create a sandboxed Claude Code workspace with a single command in under 30 seconds (Podman driver, image already pulled).
- **SC-002**: Attaching to an existing workspace takes under 5 seconds (exec-based attach + Zellij).
- **SC-003**: File sync (push/pull) transfers a 100MB project in under 30 seconds via `openshell sandbox upload/download`.
- **SC-004**: Workspace survives gateway restarts: after gateway comes back, the developer can reattach to the same Zellij session without data loss.
- **SC-005**: The OpenShell backend passes all cc-deck workspace behavioral contract tests (as defined in `specs/043-workspace-lifecycle/contracts/workspace-interface.md`).
- **SC-006**: All Claude Code network activity inside the sandbox is subject to policy enforcement (verified by OCSF deny logs for blocked destinations).

## Assumptions

- OpenShell gateway with the Podman compute driver is available and running on the developer's machine.
- The `openshell` CLI is installed and available in PATH. The CLI version should be compatible with the running gateway.
- The sandbox image (with Zellij + Claude Code) can be built from a Containerfile that cc-deck provides or the user customizes.
- The `openshell sandbox exec --tty` command provides proper interactive terminal support for Zellij attach.
- The cc-deck Zellij sidebar plugin will not work inside the sandbox in the initial version (it requires host-side pipe communication). This is an accepted limitation.
- Kubernetes/OpenShift driver support is out of scope for the initial implementation but the interface should not preclude it.
