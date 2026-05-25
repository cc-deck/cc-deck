# Feature Specification: Tool PATH Restoration in Container Builds

**Feature Branch**: `064-tool-path-restoration`
**Created**: 2026-05-25
**Status**: Draft
**Input**: User description: "Ensure tool install paths survive shell initialization in container builds"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Tools Available in Interactive Shell (Priority: P1)

A developer builds a container image with Go and Rust tools via the cc-deck build pipeline. After launching the container and opening an interactive shell session, all installed tools are available on the PATH without manual configuration. Running `go version` or `cargo --version` works immediately.

**Why this priority**: This is the core problem. Without it, users get `command not found` errors for tools that are visibly installed in the image.

**Independent Test**: Build a container image with Go and Rust in the manifest, start a shell session, and verify both `go` and `cargo` are on the PATH.

**Acceptance Scenarios**:

1. **Given** a manifest with Go as a detected tool, **When** a user opens an interactive zsh session in the container, **Then** `go version` executes successfully.
2. **Given** a manifest with Rust/Cargo as a detected tool, **When** a user opens an interactive zsh session in the container, **Then** `cargo --version` executes successfully.
3. **Given** a manifest with both Go and Rust, **When** a user opens an interactive bash session in the container, **Then** both `go` and `cargo` are available on the PATH.
4. **Given** a manifest with no tools that require non-standard PATH entries (e.g., only Node.js from a distro package), **When** a user opens a shell session, **Then** no extra PATH entries are prepended and the shell starts normally.

---

### User Story 2 - PATH Restoration Survives Shell Initialization (Priority: P2)

The tool PATH entries are prepended at the top of the shell rc files so they survive any PATH reset that occurs during shell initialization (e.g., `/etc/environment` overwriting the Docker `ENV PATH`). The restoration works regardless of the base OS or shell initialization sequence.

**Why this priority**: This is the specific mechanism that fixes the root cause. Without it, the tool paths may be added but then overwritten by login shell initialization.

**Independent Test**: Build a container from a base image that resets PATH during login (e.g., Debian with `/etc/environment`), start a login shell, and verify tool paths are present.

**Acceptance Scenarios**:

1. **Given** a container built from a Debian-based image that sets PATH in `/etc/environment`, **When** a login shell starts, **Then** tool install paths are present in the PATH.
2. **Given** a container built from an Ubuntu-based image, **When** the user's `.zshrc` runs, **Then** it sees tool paths already prepended before any user PATH modifications.

---

### User Story 3 - Extensible Tool Registry (Priority: P3)

When a new tool with a non-standard install path is added to the build pipeline, a developer can add one entry to a tool path registry to ensure the path is restored in shell sessions. No template or Containerfile changes are needed.

**Why this priority**: This ensures the solution scales as new tools are added, without requiring changes to multiple files.

**Independent Test**: Add a hypothetical tool with a custom install path to the registry, build a container, and verify the path appears in the shell PATH.

**Acceptance Scenarios**:

1. **Given** a new tool entry in the registry mapping "mytool" to "/opt/mytool/bin", **When** a manifest includes "mytool" and a container is built, **Then** "/opt/mytool/bin" appears in the shell PATH.
2. **Given** a tool that is already in the standard PATH (e.g., installed to /usr/local/bin), **When** the registry has no entry for it, **Then** no duplicate PATH entry is added.

---

### Edge Cases

- What happens when no manifest tools match any registry entries? No extra PATH entries are generated and the shell starts normally.
- What happens when multiple tools map to the same install path? The path appears only once (deduplication).
- What happens when the home directory varies between targets (e.g., `/sandbox` for OpenShell, `/home/dev` for container)? The registry uses a placeholder that is resolved to the actual home directory at build time.
- What happens when the base image's `.zshrc` or `.bashrc` does not exist yet? The build step creates it before prepending.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The build system MUST maintain a registry that maps tool name patterns to their non-standard install paths.
- **FR-002**: During Containerfile generation, the build system MUST resolve the registry against the manifest's tool list to produce a list of paths that need restoration.
- **FR-003**: The build system MUST prepend the resolved tool paths to both `.zshrc` and `.bashrc` in the container image.
- **FR-004**: The PATH prepend MUST occur after all tool installations but before the user's curated shell configuration is appended.
- **FR-005**: Duplicate paths MUST be deduplicated (each path appears at most once).
- **FR-006**: Paths with home directory references MUST be resolved to the correct home directory for the target (e.g., `/sandbox` for OpenShell, `/home/dev` for container).
- **FR-007**: Tools installed to standard system paths (e.g., `/usr/local/bin`, `/usr/bin`) MUST NOT generate registry entries, since these paths are already in the default PATH.
- **FR-008**: The existing `ENV PATH` lines in the Containerfile MUST remain unchanged as defense-in-depth for non-interactive commands (e.g., `RUN` steps).
- **FR-009**: The feature MUST work for both OpenShell and container build targets.
- **FR-010**: The feature MUST NOT affect SSH targets.

### Key Entities

- **Tool Path Registry**: A mapping of tool name patterns to their install paths. Used at build time to determine which paths need restoration. A tool name pattern matches case-insensitively against manifest tool names.
- **ContainerfileData**: The data structure passed to Containerfile templates during rendering. Extended with a list of resolved tool paths.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: All tools detected in the manifest are accessible via their command name in an interactive shell session without manual PATH configuration.
- **SC-002**: Adding a new tool to the registry requires changing exactly one location (the registry), with no template or Containerfile modifications.
- **SC-003**: Existing container images built without this feature continue to build and work identically (zero regression for manifests with no matching registry entries).
- **SC-004**: The PATH restoration adds no more than one additional Containerfile layer (a single `RUN` step in the shell finalize template).

## Assumptions

- Tools installed via distro packages (e.g., `apt install nodejs`) are placed in standard PATH directories and do not need registry entries.
- Tools installed via GitHub releases (using `install_path: /usr/local/bin/...`) are placed in standard PATH directories and do not need registry entries.
- The shell finalize template (`05-shell-finalize`) runs after all tool installation layers and before the footer layer.
- The base image provides at least one of `.zshrc` or `.bashrc` in the user's home directory, or the build process creates them.
