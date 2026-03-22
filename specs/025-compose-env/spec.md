# Feature Specification: Compose Environment

**Feature Branch**: `025-compose-env`
**Created**: 2026-03-21
**Status**: Evolved (post-walkthrough)
**Input**: User description: "Compose environment with multi-container orchestration and optional network filtering"

## Context: Existing Infrastructure

The project already has a working `container` environment type (single container via `podman run`) and a compose file generator with tinyproxy sidecar support. The compose environment builds on both, adding multi-container orchestration as a project-local environment type. It reuses the existing container interaction layer for secrets, volumes, exec, and auth detection, while adding compose CLI lifecycle management and optional network filtering via a proxy sidecar.

The compose environment implements the `Environment` interface defined in `specs/023-env-interface/contracts/environment-interface.md`. All behavioral requirements from that contract (nested Zellij detection, session creation with layout, auto-start, timestamp updates, name validation, cleanup on failure, running check on delete, state reconciliation) apply to this implementation without deviation.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Create and Use a Compose Environment (Priority: P1)

A user navigates to their project directory and creates a compose environment. The system generates all necessary orchestration files in a subdirectory, starts the environment, and the user attaches to an interactive session with the sidebar plugin visible and their project files accessible.

**Why this priority**: This is the core lifecycle (create, attach, use). Without this, compose environments do not exist.

**Independent Test**: Can be fully tested by running `cc-deck env create mydev --type compose` in any project directory, then `cc-deck env attach mydev`, and verifying the sidebar loads and `/workspace` contains the project files.

**Acceptance Scenarios**:

1. **Given** a project directory with source files, **When** the user runs `cc-deck env create mydev --type compose`, **Then** the system generates orchestration files in a `.cc-deck/` subdirectory, starts a container with the project directory mounted at `/workspace`, and records the environment in both the definition store and state store.
2. **Given** a running compose environment, **When** the user runs `cc-deck env attach mydev`, **Then** the user enters an interactive session with the cc-deck sidebar plugin loaded.
3. **Given** a running compose environment with the project directory bind-mounted, **When** the user edits a file on the host, **Then** the change is immediately visible inside the container at `/workspace`, and vice versa.

---

### User Story 2 - Network Filtering via Proxy Sidecar (Priority: P2)

A user creates a compose environment with network filtering by specifying allowed domain groups. The system generates a proxy sidecar alongside the session container. The session container can only reach the internet through the proxy, which enforces the domain allowlist.

**Why this priority**: Network filtering is the primary differentiator between compose and the simpler container type. However, a compose environment is useful even without filtering (as a foundation for future MCP sidecars), so filtering is P2 rather than P1.

**Independent Test**: Can be tested by creating a compose environment with `--allowed-domains anthropic`, attaching, and verifying that requests to `api.anthropic.com` succeed while requests to an unlisted domain are blocked.

**Acceptance Scenarios**:

1. **Given** a user creates a compose environment with `--allowed-domains anthropic,github`, **When** the environment starts, **Then** a proxy sidecar container is running alongside the session container, configured to allow only the specified domains and their subdomains.
2. **Given** a running compose environment with network filtering, **When** the session container makes an HTTPS request to an allowed domain, **Then** the request succeeds through the proxy.
3. **Given** a running compose environment with network filtering, **When** the session container makes an HTTPS request to a domain not in the allowlist, **Then** the request is blocked by the proxy.
4. **Given** a user specifies domain group names (e.g., "anthropic", "github"), **When** the environment is created, **Then** the groups are expanded to their full domain lists using the existing domain resolver.

---

### User Story 3 - Full Lifecycle Management (Priority: P2)

A user manages the lifecycle of a compose environment: stopping it to free resources, restarting it later with state preserved, and eventually deleting it with full cleanup.

**Why this priority**: Lifecycle management ensures compose environments are practical for daily use. Without stop/start, users must delete and recreate environments.

**Independent Test**: Can be tested by creating a compose environment, writing a file inside it, stopping it, starting it, and verifying the file persists. Then deleting and verifying all artifacts are removed.

**Acceptance Scenarios**:

1. **Given** a running compose environment, **When** the user runs `cc-deck env stop mydev`, **Then** all containers in the compose project are stopped and the environment state is updated to "stopped".
2. **Given** a stopped compose environment, **When** the user runs `cc-deck env start mydev`, **Then** the containers resume and the environment state is updated to "running".
3. **Given** a stopped compose environment with a bind-mounted project directory, **When** the user starts the environment, **Then** the project files are still accessible at `/workspace`.
4. **Given** a compose environment (running or stopped), **When** the user runs `cc-deck env delete mydev`, **Then** all containers are removed, secrets are cleaned up, the generated files directory is deleted, and the environment is removed from both the definition and state stores.

---

### User Story 4 - Credential Passthrough (Priority: P2)

A user creates a compose environment and their host authentication credentials (API keys, cloud provider credentials) are automatically detected and injected into the session container, matching the behavior of the existing container environment type.

**Why this priority**: Credential passthrough is essential for the session container to run AI coding agents, but the mechanism is already proven in the container type.

**Independent Test**: Can be tested by setting an API key environment variable on the host, creating a compose environment, and verifying the key is available inside the session container.

**Acceptance Scenarios**:

1. **Given** a host environment with authentication credentials set, **When** the user creates a compose environment with `--auth auto`, **Then** the appropriate credentials are detected and injected into the session container as secrets.
2. **Given** explicit credential flags (`--credential KEY=VALUE`), **When** the environment is created, **Then** the specified credentials are available inside the session container.
3. **Given** file-based credentials (e.g., a cloud provider credentials file), **When** the environment is created, **Then** the file is mounted as a secret and the corresponding environment variable points to the mounted path.

---

### User Story 5 - Gitignore and Project Hygiene (Priority: P3)

When a user creates a compose environment in a git-tracked project, they are warned about adding the generated files directory to `.gitignore` and can optionally have it done automatically.

**Why this priority**: Important for project hygiene but not blocking. Users can always add the gitignore entry manually.

**Independent Test**: Can be tested by creating a compose environment in a git repo without the generated directory in `.gitignore`, and verifying a warning is printed. Then testing with `--gitignore` to verify auto-addition.

**Acceptance Scenarios**:

1. **Given** a git-tracked project where the generated files directory is not in `.gitignore`, **When** the user creates a compose environment, **Then** a warning message is printed advising the user to add the directory to `.gitignore`.
2. **Given** a git-tracked project, **When** the user creates a compose environment with the `--gitignore` flag, **Then** the generated files directory is automatically appended to `.gitignore`.
3. **Given** a project that already has the generated files directory in `.gitignore`, **When** the user creates a compose environment, **Then** no warning is printed and no duplicate entry is added.

---

### Edge Cases

- What happens when the compose runtime is not installed? The system reports a clear error: "podman-compose not found. Install it or configure an alternative compose runtime."
- What happens when the project directory is not writable? The system reports an error before attempting to generate files.
- What happens when the user creates a compose environment in a directory that already has generated files? The system regenerates all files from the current definition and prints a warning.
- What happens when `cc-deck env attach` is run from inside a Zellij session? The system warns the user to detach first, matching the container type behavior.
- What happens when the proxy sidecar fails to start? The session container should not start either, and the system reports the proxy error.
- What happens when the user runs `cc-deck env list` with both container and compose environments? Both types appear in the list with their respective type labels.
- What happens when an auto-started compose environment has stopped containers? Attach auto-starts the environment before connecting, matching container type behavior.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST support creating compose environments via `cc-deck env create <name> --type compose`.
- **FR-002**: On create, the system MUST generate orchestration files in a `.cc-deck/` subdirectory within the project directory. Generated files inside `.cc-deck/` MUST NOT use dot prefixes (e.g., `env` not `.env`) per constitution principle XIV (no dotfile nesting).
- **FR-003**: The default storage for compose environments MUST be a host-path bind mount of the project directory, mounted at `/workspace` inside the session container. The mount MUST use the `:U` volume flag to map file ownership to the container user, ensuring non-root container images (e.g., images with `USER dev`) can write to the workspace on hosts with UID mismatch (common on macOS with Podman Machine).
- **FR-004**: Users MUST be able to specify `--storage named-volume` to use an isolated volume instead of a bind mount. Named volumes MUST be pre-created via `podman volume create` and declared as `external: true` in the top-level `volumes:` section of the generated compose file (required by the compose specification).
- **FR-005**: The system MUST support optional network filtering via `--allowed-domains`, which adds a proxy sidecar that enforces a domain allowlist.
- **FR-006**: Domain group names specified in `--allowed-domains` MUST be expanded using the existing domain resolver, supporting built-in groups, user-defined groups, and literal domain patterns.
- **FR-007**: When network filtering is active, the session container MUST NOT have direct internet access. All outbound traffic MUST route through the proxy sidecar.
- **FR-008**: The system MUST support the full lifecycle: create, attach, start, stop, delete. Start and stop MUST fall back to direct `podman start`/`podman stop` on the session container when the `.cc-deck/` directory or compose file is missing, ensuring graceful operation even if generated files were removed externally.
- **FR-009**: Delete MUST remove all compose containers, clean up secrets, and remove the generated files directory from the project.
- **FR-010**: Credential detection and injection MUST match the behavior of the existing container environment type (auto-detection of API keys, cloud provider credentials, file-based credentials). File-based credentials (e.g., gcloud ADC) MUST be mounted as read-only volume mounts with `:ro,U` from the original host file, keeping the host file as the single source of truth (no copying, no drift after re-authentication). The `:U` flag ensures the container user can read the file regardless of host UID. The `ANTHROPIC_API_KEY` MUST always be included in the credential set when present in the host environment, regardless of the detected auth mode (useful as fallback alongside Vertex or Bedrock).
- **FR-011**: Attach MUST open an interactive session identical to the container type (sidebar plugin loaded, Zellij session with cc-deck layout).
- **FR-012**: Attach MUST auto-start a stopped compose environment before connecting.
- **FR-013**: The system MUST detect and warn when the generated files directory is not in `.gitignore`, and offer automatic addition via `--gitignore`.
- **FR-014**: If the generated files directory already exists on create, the system MUST regenerate all files and print a warning.
- **FR-015**: The system MUST auto-detect the compose runtime, with `podman-compose` as the preferred default, and allow override via configuration.
- **FR-016**: The compose environment MUST appear correctly in `cc-deck env list` and `cc-deck env status` output, including reconciliation of actual container state.
- **FR-017**: Exec and push/pull operations MUST work identically to the container type, targeting the session container by name.
- **FR-018**: The `--path` flag MUST allow specifying the project directory explicitly, defaulting to the current working directory.
- **FR-019**: Create MUST validate the environment name, check for duplicate names in the state store, and check that the compose runtime is available before proceeding. The duplicate name check MUST happen before any resource creation (fail-fast) to prevent orphaned containers or generated files.
- **FR-020**: Create MUST clean up partially created resources (generated files, secrets, containers) if creation fails partway through.
- **FR-021**: Attach MUST update the last-attached timestamp in the state store.
- **FR-022**: Delete MUST refuse to delete a running environment unless `--force` is specified.
- **FR-023**: Harvest MUST return a clear error indicating that compose environments do not support harvest, matching the container type behavior.

### Key Entities

- **Compose Project**: The set of generated orchestration files (compose definition, credentials file, proxy config) stored in the `.cc-deck/` subdirectory of the project directory. Fully managed by the system, never hand-edited.
- **Session Container**: The primary container in the compose project, running the cc-deck image with Zellij and the sidebar plugin. Named `cc-deck-<env-name>`.
- **Proxy Sidecar**: An optional container that filters outbound network traffic. Only present when `--allowed-domains` is specified. Named `cc-deck-<env-name>-proxy`.
- **Domain Group**: A named collection of domain patterns (e.g., "anthropic", "github") that resolves to a list of allowed domains and subdomains. Groups can be built-in, user-defined, or extended.
- **Compose Fields**: Runtime state specific to compose environments, including the project directory path and container names.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A user can create a compose environment and attach to a working session within 30 seconds (excluding image pull time).
- **SC-002**: When network filtering is active, 100% of requests to unlisted domains are blocked by the proxy sidecar.
- **SC-003**: All lifecycle operations (create, attach, start, stop, delete) complete without leaving orphaned containers, volumes, or secrets.
- **SC-004**: File changes in the project directory are visible inside the container within one second (bind mount mode).
- **SC-005**: Credential detection produces the same results as the container environment type for identical host configurations.
- **SC-006**: The compose environment appears correctly in `cc-deck env list` with accurate state reconciliation.

## Evolution Log

**2026-03-22: Post-walkthrough fixes** (commits d27f763, a2a446d)

Spec updated to reflect implementation improvements discovered during manual walkthrough testing:

- FR-002: Added no-dotfile-nesting rule (`env` not `.env` inside `.cc-deck/`)
- FR-003: Added `:U` volume flag requirement for non-root container UID mapping
- FR-004: Added `external: true` requirement for named volumes in compose spec
- FR-008: Added graceful podman fallback when `.cc-deck/` directory is missing
- FR-010: File credentials use `:ro,U` volume mounts (not copies or compose secrets) for live reads without permission issues. API key always included alongside other auth modes.
- FR-019: Added fail-fast duplicate name check before resource creation

## Assumptions

- The compose runtime (`podman-compose` or alternative) is installed and available in the user's PATH.
- The project directory exists and is writable by the user.
- The existing compose YAML generator and proxy config generator produce valid output for the compose runtime.
- The existing domain resolver correctly expands group names to domain lists.
- Bind mount performance is acceptable for the user's workflow (no large monorepo performance concerns).
- The session container uses the same image as the container environment type (`quay.io/cc-deck/cc-deck-demo:latest` by default).
