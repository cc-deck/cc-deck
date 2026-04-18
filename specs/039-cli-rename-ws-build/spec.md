# Feature Specification: CLI Rename - Workspace & Build

**Feature Branch**: `039-cli-rename-ws-build`  
**Created**: 2026-04-18  
**Status**: Draft  
**Input**: User description: "Rename CLI command trees: 'env' becomes 'ws' (workspace), 'setup' becomes 'build', move config commands under a new 'config' parent, hide plumbing commands, and adopt tmux/zellij-inspired subcommand names"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Daily workspace operations with new command names (Priority: P1)

A developer manages their Claude Code workspaces using the renamed CLI commands. They use `cc-deck ws new mydev` to create a workspace, `cc-deck attach mydev` to connect, `cc-deck ls` to list workspaces, and `cc-deck ws delete mydev` to tear down. The commands feel familiar to anyone who has used tmux or Zellij.

**Why this priority**: The workspace commands are the primary user-facing interface. Every cc-deck user interacts with these commands daily, and the rename directly addresses the confusion between "env" and "setup."

**Independent Test**: Can be fully tested by creating, listing, attaching, and destroying workspaces using the new `ws` command tree. Delivers value by eliminating the "env" naming confusion.

**Acceptance Scenarios**:

1. **Given** cc-deck is installed, **When** a user runs `cc-deck ws new mydev`, **Then** a workspace named "mydev" is created with the same behavior as the former `cc-deck env create mydev`
2. **Given** a workspace exists, **When** a user runs `cc-deck attach mydev`, **Then** the user connects to the workspace (same behavior as former `cc-deck env attach mydev`)
3. **Given** one or more workspaces exist, **When** a user runs `cc-deck ls`, **Then** all workspaces are listed (same behavior as former `cc-deck env list`)
4. **Given** a workspace exists, **When** a user runs `cc-deck ws delete mydev`, **Then** the workspace is destroyed (same behavior as former `cc-deck env delete mydev`)
5. **Given** cc-deck is installed, **When** a user runs `cc-deck workspace new mydev`, **Then** the command works identically to `cc-deck ws new mydev` (full alias)

---

### User Story 2 - Build artifact management with renamed commands (Priority: P2)

A developer prepares container images or provisions SSH hosts using `cc-deck build init`, `cc-deck build run`, and `cc-deck build verify`. The "build" name clearly signals artifact generation, distinct from workspace creation.

**Why this priority**: Build commands are used less frequently than workspace commands but are essential for setting up custom development images. The rename from "setup" removes the ambiguity with system configuration.

**Independent Test**: Can be fully tested by running the build workflow (`init`, `run`, `verify`, `diff`) and confirming identical behavior to the former `setup` commands.

**Acceptance Scenarios**:

1. **Given** a project directory, **When** a user runs `cc-deck build init`, **Then** the build manifest is initialized (same behavior as former `cc-deck setup init`)
2. **Given** an initialized build directory, **When** a user runs `cc-deck build run`, **Then** artifacts are generated (same behavior as former `cc-deck setup run`)
3. **Given** build artifacts exist, **When** a user runs `cc-deck build verify`, **Then** the artifacts are validated (same behavior as former `cc-deck setup verify`)
4. **Given** build artifacts exist, **When** a user runs `cc-deck build diff`, **Then** the differences between the manifest and current state are shown (same behavior as former `cc-deck setup diff`)

---

### User Story 3 - System configuration under a unified parent (Priority: P2)

A developer manages plugins, profiles, domain groups, and shell completions through a single `cc-deck config` parent command. This groups all configuration management under one roof.

**Why this priority**: Configuration commands were previously scattered across the top level (`plugin`, `profile`, `domains`, `completion`) and mixed into the "Setup" help group. Grouping them under a dedicated `config` parent makes the CLI taxonomy self-documenting.

**Independent Test**: Can be fully tested by running each config subcommand (`plugin`, `profile`, `domains`, `completion`) and verifying identical behavior to their former locations.

**Acceptance Scenarios**:

1. **Given** cc-deck is installed, **When** a user runs `cc-deck config plugin install`, **Then** the plugin is installed (same behavior as former `cc-deck plugin install`)
2. **Given** cc-deck is installed, **When** a user runs `cc-deck config profile list`, **Then** profiles are listed (same behavior as former `cc-deck profile list`)
3. **Given** cc-deck is installed, **When** a user runs `cc-deck config domains list`, **Then** domain groups are listed (same behavior as former `cc-deck domains list`)
4. **Given** cc-deck is installed, **When** a user runs `cc-deck config completion zsh`, **Then** shell completion scripts are output (same behavior as former `cc-deck completion zsh`)

---

### User Story 4 - Promoted top-level shortcuts (Priority: P3)

Three daily-driver commands (`attach`, `ls`, `exec`) are available at the top level without the `ws` prefix for quick access. All other workspace commands require the `ws` prefix to keep the top-level help output clean.

**Why this priority**: This is a convenience optimization. The full `ws` subcommand tree already provides complete functionality; promotion reduces keystrokes for the most frequent operations.

**Independent Test**: Can be tested by verifying that `cc-deck attach`, `cc-deck ls`, and `cc-deck exec` work identically to their `cc-deck ws attach`, `cc-deck ws ls`, and `cc-deck ws exec` counterparts.

**Acceptance Scenarios**:

1. **Given** a running workspace, **When** a user runs `cc-deck attach mydev`, **Then** the user connects to the workspace
2. **Given** workspaces exist, **When** a user runs `cc-deck ls`, **Then** all workspaces are listed
3. **Given** a running workspace, **When** a user runs `cc-deck exec mydev -- ls`, **Then** the command runs inside the workspace

---

### User Story 5 - Hidden plumbing commands (Priority: P3)

The `hook` command (used internally by Claude Code, never by users) is hidden from help output. It remains functional when invoked directly but does not appear in `cc-deck --help` or `cc-deck help`.

**Why this priority**: Reducing help output clutter improves discoverability of user-facing commands. The hook command is an internal integration point.

**Independent Test**: Can be tested by verifying `cc-deck hook` still works but does not appear in help output.

**Acceptance Scenarios**:

1. **Given** cc-deck is installed, **When** a user runs `cc-deck --help`, **Then** the `hook` command does not appear in the output
2. **Given** cc-deck is installed, **When** Claude Code invokes `cc-deck hook <event>`, **Then** the command executes normally

---

### Edge Cases

- What happens when a user runs a removed command name (e.g., `cc-deck env create`)? No migration shims are registered. Cobra's default "unknown command" error (which includes "Did you mean...?" suggestions) handles this.
- What happens when a user runs a demoted top-level command (e.g., `cc-deck start mydev`)? Same as above: Cobra's default error with suggestions. No hidden aliases or shim commands.
- What happens when help text references old command names? All help text, usage strings, and error messages must use the new names.
- What happens when documentation or scripts reference old command names? Documentation must be updated as part of this feature (per constitution Principle IX).

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The CLI MUST rename the `env` command tree to `ws` with `workspace` as a full alias
- **FR-002**: The CLI MUST rename the `setup` command tree to `build`
- **FR-003**: The CLI MUST create a new `config` parent command containing `plugin`, `profile`, `domains`, and `completion` subcommands
- **FR-004**: The CLI MUST rename the `create` subcommand to `new` and the `delete` subcommand to `kill` under the `ws` tree
- **FR-005**: The CLI MUST add `ls` as a short alias for the `list` subcommand under the `ws` tree (both `ws list` and `ws ls` work)
- **FR-006**: The CLI MUST promote `attach`, `ls`, and `exec` as top-level commands that behave identically to their `ws` counterparts
- **FR-007**: The CLI MUST demote `status`, `start`, `stop`, and `logs` from top-level commands to `ws` subcommands only. These commands are used less frequently and the `ws` prefix is acceptable friction.
- **FR-008**: The CLI MUST hide the `hook` command from help output while keeping it functional
- **FR-009**: The CLI MUST leave `snapshot`, `version`, and `hook` behavior unchanged (only `hook` visibility changes)
- **FR-010**: The CLI MUST preserve all underlying operation behavior. Command names and discoverability change, but no operation produces different results than before.
- **FR-011**: The CLI MUST NOT change config file paths, state file paths, YAML structure, or internal type names (e.g., `EnvironmentType` constants remain unchanged)
- **FR-012**: All help text, usage strings, and error messages MUST reference the new command names
- **FR-013**: The `ws` subcommand tree MUST include: `new`, `kill`, `attach`, `list` (alias `ls`), `start`, `stop`, `status`, `logs`, `exec`, `push`, `pull`, `harvest`, `prune`, `refresh-creds`

### Key Entities

- **Command Group "ws"**: Parent command for all workspace operations, with `workspace` as a full alias
- **Command Group "build"**: Parent command for artifact generation operations (init, run, verify, diff)
- **Command Group "config"**: Parent command for system configuration (plugin, profile, domains, completion)
- **Top-level shortcuts**: `attach`, `ls`, `exec` promoted to root level as convenience duplicates

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: All existing workspace operations complete successfully using the new `ws` command names
- **SC-002**: All existing build operations complete successfully using the new `build` command names
- **SC-003**: All existing configuration operations complete successfully under the new `config` parent
- **SC-004**: The top-level help output shows exactly three command groups (Workspace, Build, Config) plus promoted shortcuts and standalone commands (snapshot, version)
- **SC-005**: Users discover the correct command on first attempt at least 90% of the time (measured by elimination of "env vs setup" confusion in help output)
- **SC-006**: All existing tests pass after the rename with updated command references
- **SC-007**: Documentation (README, CLI reference, Antora guides) reflects the new command structure

## Clarifications

### Session 2026-04-18

- Q: Should old command names (e.g., `env`, `setup`, demoted top-level `start`) have migration shims? → A: No shims. Rely on Cobra's default "unknown command" error with built-in suggestions.

## Assumptions

- Users have no automation scripts depending on the old command names. If they do, a clear error message pointing to the new names is sufficient migration guidance.
- The `workspace` alias is implemented via Cobra's `Aliases` field and does not require a separate command registration.
- The `hook` command is hidden via Cobra's `Hidden: true` field, which suppresses it from help output but allows direct invocation.
- The Claude Code commands (`/cc-deck.capture`, `/cc-deck.build`) are out of scope for this rename and will be addressed separately if needed.
- No deprecation period for old command names is needed since cc-deck is pre-1.0 and the user base is small.
