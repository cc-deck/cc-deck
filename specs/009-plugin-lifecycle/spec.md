# Feature Specification: Plugin Lifecycle Management

**Feature Branch**: `009-plugin-lifecycle`
**Created**: 2026-03-04
**Status**: Draft
**Input**: User description: "plugin lifecycle management"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Install Plugin (Priority: P1)

A user has installed the cc-deck CLI and wants to start using the Zellij session management plugin. They run a single command to install the plugin binary and a layout file so they can launch Zellij with the plugin active.

**Why this priority**: Without installation, nothing else works. This is the entry point for the entire plugin experience.

**Independent Test**: Can be fully tested by running the install command and verifying the plugin binary and layout file exist at the expected locations. Delivers immediate value because the user can launch Zellij with the plugin right away.

**Acceptance Scenarios**:

1. **Given** cc-deck is installed and Zellij config directory exists, **When** the user runs the install command, **Then** the plugin binary is written to the standard Zellij plugins directory and a layout file is created in the Zellij layouts directory.
2. **Given** the plugin is already installed, **When** the user runs install again without a force flag, **Then** the system prompts for confirmation before overwriting.
3. **Given** the plugin is already installed, **When** the user runs install with a force flag, **Then** the system overwrites without prompting.
4. **Given** the install completes, **When** the user reviews the output, **Then** a summary shows installed file paths and instructions for launching Zellij with the plugin.
5. **Given** the Zellij config directory does not exist, **When** the user runs install, **Then** the system creates the necessary directories before writing files.

---

### User Story 2 - Choose Layout Template (Priority: P1)

A user wants control over how the plugin integrates with their Zellij setup. By default a minimal layout is installed (just the status bar plugin). Optionally, a full layout with sensible Claude Code session defaults is available.

**Why this priority**: Directly tied to install; the layout determines the user's first experience with the plugin.

**Independent Test**: Can be tested by running install with different layout options and verifying the generated layout file content matches the expected template.

**Acceptance Scenarios**:

1. **Given** the user runs install with no layout option specified, **When** installation completes, **Then** a minimal layout file containing only the plugin status bar pane is created.
2. **Given** the user runs install requesting the full layout, **When** installation completes, **Then** an opinionated layout file with defaults for Claude Code sessions is created.
3. **Given** a layout file already exists, **When** the user installs with a different layout option and force flag, **Then** the existing layout file is replaced with the newly selected template.

---

### User Story 3 - Inject into Default Layout (Priority: P2)

A user already has a customized Zellij default layout and wants the plugin to load automatically without switching to a cc-deck-specific layout. They use an inject option during install to add the plugin pane to their existing default layout.

**Why this priority**: Important for users with existing Zellij configurations, but not required for first-time users. The dedicated layout from P1 covers the common case.

**Independent Test**: Can be tested by creating a sample default layout, running install with the inject option, and verifying the plugin pane block was appended without corrupting the existing layout content.

**Acceptance Scenarios**:

1. **Given** a default Zellij layout exists, **When** the user runs install with the inject option, **Then** the plugin pane block is appended to the default layout while preserving all existing content.
2. **Given** the default layout already contains the plugin pane, **When** the user runs install with the inject option, **Then** the system detects the existing injection and skips duplicate insertion.
3. **Given** no default Zellij layout exists, **When** the user runs install with the inject option, **Then** the system reports that no default layout was found and suggests using the dedicated layout instead.

---

### User Story 4 - Check Plugin Status (Priority: P2)

A user wants to verify their plugin installation state: whether the plugin is installed, which version, whether Zellij is compatible, which layouts reference it, and whether any running Zellij sessions currently have it loaded.

**Why this priority**: Useful for troubleshooting and verification, but not blocking for the core install/remove workflow.

**Independent Test**: Can be tested by checking status in different states (not installed, installed, running) and verifying the output accurately reflects each state.

**Acceptance Scenarios**:

1. **Given** the plugin is not installed, **When** the user runs the status command, **Then** the output shows "not installed" with the expected install location.
2. **Given** the plugin is installed, **When** the user runs the status command, **Then** the output shows the installed path, file size, and embedded version.
3. **Given** Zellij is installed on the system, **When** the user runs the status command, **Then** the output includes a compatibility check between the installed Zellij version and the plugin's SDK version.
4. **Given** layout files reference the plugin, **When** the user runs the status command, **Then** the output lists which layout files contain the plugin and whether the default layout was injected.
5. **Given** Zellij is not installed, **When** the user runs the status command, **Then** the output warns that Zellij was not found on the system.

---

### User Story 5 - Remove Plugin (Priority: P2)

A user wants to fully uninstall the plugin, removing the binary, layout files, and undoing any modifications to the default layout.

**Why this priority**: Important for a clean user experience but only relevant after installation.

**Independent Test**: Can be tested by installing the plugin (with and without default layout injection), running remove, and verifying all artifacts are cleaned up.

**Acceptance Scenarios**:

1. **Given** the plugin is installed with a dedicated layout, **When** the user runs the remove command, **Then** the plugin binary and cc-deck layout file are deleted.
2. **Given** the default layout was injected, **When** the user runs the remove command, **Then** the plugin pane block is removed from the default layout while preserving all other content.
3. **Given** the plugin is not installed, **When** the user runs the remove command, **Then** the system reports that nothing was found to remove.
4. **Given** removal completes, **When** the user reviews the output, **Then** a summary lists all files that were removed or modified.

---

### Edge Cases

- **Non-standard config directory**: When ZELLIJ_CONFIG_DIR is set, the system uses that path instead of the default. Covered by FR-019.
- **File permission errors**: When the system cannot write to the plugins or layouts directory, it reports the exact path and required permissions so the user can fix it. Covered by FR-021.
- **Unparseable default layout**: When the inject option is used but the default layout cannot be parsed, the system warns the user and skips injection rather than corrupting the file. The plugin binary and dedicated layout are still installed. Covered by FR-022.
- **Remove while Zellij is running**: The system proceeds with file removal and warns that running Zellij sessions may need to be restarted. Zellij will not crash (it loads the WASM into memory at startup). Covered by FR-023.
- **Corrupted binary from failed install**: The install command uses atomic writes (write to temp file, then rename) to prevent partial binaries. If a corrupted file exists from an external cause, the force flag allows overwriting it. Covered by FR-024.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST embed the compiled plugin binary within the CLI binary at build time, producing a single distributable artifact.
- **FR-002**: System MUST write the embedded plugin binary to the standard Zellij plugins directory during installation.
- **FR-003**: System MUST create necessary parent directories if they do not exist during installation.
- **FR-004**: System MUST install a layout file to the Zellij layouts directory during installation.
- **FR-005**: System MUST support two layout templates: a minimal template (a single terminal pane plus the plugin status bar) and a full template (terminal pane, plugin status bar, preconfigured keybindings for session management, and tab bar disabled to avoid conflict with the plugin's own session switching).
- **FR-006**: System MUST default to the minimal layout template when no layout option is specified.
- **FR-007**: System MUST prompt for confirmation before overwriting an existing installation unless a force flag is provided.
- **FR-008**: System MUST support injecting the plugin pane into an existing default Zellij layout file when requested.
- **FR-009**: System MUST detect and skip duplicate injection if the plugin pane already exists in the default layout.
- **FR-010**: System MUST display a summary of installed files and usage instructions after successful installation.
- **FR-011**: System MUST report installation state including: installed (yes/no), file path, file size, and embedded version.
- **FR-012**: System MUST check compatibility between the installed Zellij version and the plugin's SDK version, reporting "compatible," "untested," or "incompatible" based on a known compatibility range embedded in the CLI. The plugin requires Zellij 0.40 or later (matching the zellij-tile SDK major version).
- **FR-013**: System MUST list layout files that reference the plugin and indicate whether the default layout was injected.
- **FR-014**: System MUST detect whether Zellij is installed on the system and warn if not found.
- **FR-015**: System MUST remove the plugin binary, cc-deck layout file, and undo default layout injection during removal.
- **FR-016**: System MUST preserve all non-cc-deck content when modifying or reverting the default layout file.
- **FR-017**: System MUST report what was removed or modified after successful removal.
- **FR-018**: System MUST handle the case where the plugin is not installed when remove is run, reporting that nothing was found.
- **FR-019**: System MUST respect the ZELLIJ_CONFIG_DIR environment variable if set, falling back to the standard config location when unset.
- **FR-020**: System MUST scope plugin commands to local Zellij installations only; remote/K8s sessions are out of scope.
- **FR-021**: System MUST report actionable error messages on file permission failures, including the exact path and required permissions.
- **FR-022**: System MUST skip default layout injection and warn the user when the default layout file cannot be parsed, without aborting the rest of the installation.
- **FR-023**: System MUST warn the user during removal that running Zellij sessions may need to be restarted to fully unload the plugin.
- **FR-024**: System MUST use atomic file writes (write to temporary file, then rename) for the plugin binary to prevent corrupted partial files from interrupted installs.

### Key Entities

- **Plugin Binary**: The compiled WASM file embedded in the CLI. Has a version, file size, and target install path.
- **Layout Template**: A Zellij layout definition (KDL format) that references the plugin. Comes in minimal and full variants.
- **Default Layout**: The user's existing Zellij default layout file, which may be modified by injection and reverted on removal.
- **Installation State**: The aggregate of plugin binary presence, layout file presence, default layout injection status, and Zellij compatibility.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users can go from a fresh cc-deck CLI install to a working Zellij plugin session in under 2 minutes using a single command.
- **SC-002**: The install command works without network access (offline installation).
- **SC-003**: The remove command fully reverses all changes made by install, leaving no artifacts behind.
- **SC-004**: The status command provides enough information for a user to self-diagnose installation issues without consulting documentation.
- **SC-005**: Default layout injection preserves 100% of existing user layout content (no corruption or data loss).
- **SC-006**: Plugin commands handle missing directories, permission errors, and absent Zellij installations with clear, actionable error messages.

## Constraints

- **Single binary distribution**: The plugin binary is embedded in the CLI at build time. The CLI is the sole distribution artifact. No network access or separate downloads are needed during installation. The build pipeline compiles the Rust WASM binary before the Go CLI build, making the artifact available for embedding.

## Assumptions

- Zellij is already installed on the user's system (the CLI does not install Zellij itself, but warns if missing).
- Minimum supported Zellij version is 0.40 (matching the zellij-tile SDK major version used by the plugin).
- The Zellij config directory follows the standard XDG convention or is specified via ZELLIJ_CONFIG_DIR.
- The plugin binary is compiled for wasm32-wasip1 and compatible with Zellij's plugin loading mechanism.
- KDL layout files follow Zellij's documented layout format. Default layout injection uses string-level append rather than full KDL parsing, so it works with any valid layout file but cannot validate structural correctness.

## Scope Boundaries

**In scope**:
- Plugin install, status, and remove commands
- Layout template installation (minimal and full)
- Default layout injection and reversal
- Build-time embedding of the WASM binary
- Local Zellij installations only

**Out of scope**:
- Installing or managing Zellij itself
- Remote/K8s session plugin management (handled by container images)
- Plugin version upgrade from network sources (download-based updates)
- Zellij configuration beyond layout files (keybindings, themes)
