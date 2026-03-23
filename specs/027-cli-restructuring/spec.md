# Feature Specification: CLI Command Restructuring

**Feature Branch**: `027-cli-restructuring`
**Created**: 2026-03-22
**Status**: Draft
**Input**: Brainstorm session on optimizing cc-deck CLI UX by promoting high-frequency commands to top level, removing legacy K8s commands, and organizing help output with logical command groups.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Daily Commands at Top Level (Priority: P1)

A developer working with cc-deck environments throughout the day can run the most common commands (attach, list, status, start, stop, logs) directly at the top level without the `env` prefix. This reduces keystrokes and friction for the six most frequently used operations.

**Why this priority**: These six commands represent the core daily workflow. Every cc-deck user runs them repeatedly. Saving one word per invocation across dozens of daily uses adds up to a meaningful UX improvement.

**Independent Test**: Can be fully tested by running each promoted command at the top level (e.g., `cc-deck attach mydev`) and verifying identical behavior to the `env` subcommand path (`cc-deck env attach mydev`).

**Acceptance Scenarios**:

1. **Given** a user with an existing environment named "mydev", **When** they run `cc-deck attach mydev`, **Then** the tool attaches to the environment identically to `cc-deck env attach mydev`.
2. **Given** a user with multiple environments, **When** they run `cc-deck ls`, **Then** the tool lists all environments identically to `cc-deck env list`.
3. **Given** a user with a running environment, **When** they run `cc-deck status mydev`, **Then** the tool displays environment status identically to `cc-deck env status mydev`.
4. **Given** a user with a stopped environment, **When** they run `cc-deck start mydev`, **Then** the tool starts it identically to `cc-deck env start mydev`.
5. **Given** a user with a running environment, **When** they run `cc-deck stop mydev`, **Then** the tool stops it identically to `cc-deck env stop mydev`.
6. **Given** a user with an active environment, **When** they run `cc-deck logs mydev`, **Then** the tool shows logs identically to `cc-deck env logs mydev`.
7. **Given** a user runs a promoted command with all supported flags, **When** the flags are passed at top level (e.g., `cc-deck ls -o json`), **Then** all flags behave identically to the `env` subcommand path.

---

### User Story 2 - Organized Help Output (Priority: P2)

When a user runs `cc-deck --help`, commands are organized into logical groups that reflect usage frequency: daily commands first, then session management, environment lifecycle, and setup. This helps new users discover the most important commands quickly and helps experienced users scan for the right category.

**Why this priority**: Help output is the primary discoverability mechanism for CLI tools. Grouping commands by usage pattern reduces cognitive load and guides users toward the right commands.

**Independent Test**: Can be tested by running `cc-deck --help` and verifying that commands appear under the correct group headings in the expected order.

**Acceptance Scenarios**:

1. **Given** a user runs `cc-deck --help`, **When** the help output is displayed, **Then** commands are organized under group headings: "Daily", "Session", "Environment", and "Setup".
2. **Given** a user views help output, **When** they look at the "Daily" group, **Then** it contains: attach, list (ls), status, start, stop, logs.
3. **Given** a user views help output, **When** they look at the "Session" group, **Then** it contains: snapshot.
4. **Given** a user views help output, **When** they look at the "Environment" group, **Then** it contains: env.
5. **Given** a user views help output, **When** they look at the "Setup" group, **Then** it contains: plugin, profile, domains, image.

---

### User Story 3 - Backward Compatibility for env Subcommands (Priority: P2)

Users who have already learned the `cc-deck env attach`, `cc-deck env list`, and other `env` subcommand patterns can continue using them without any change in behavior. Both the top-level and `env`-prefixed paths work identically.

**Why this priority**: Avoiding disruption for existing users is essential. Dual-path support means no documentation, scripts, or muscle memory is broken.

**Independent Test**: Can be tested by running each promoted command through both paths and comparing output, exit codes, and side effects.

**Acceptance Scenarios**:

1. **Given** a user runs `cc-deck env attach mydev`, **When** the command executes, **Then** the behavior is identical to running `cc-deck attach mydev`.
2. **Given** a user has shell scripts that use `cc-deck env list`, **When** they run those scripts after the restructuring, **Then** the scripts continue to work without modification.
3. **Given** a user requests shell completions, **When** they tab-complete after `cc-deck`, **Then** both top-level commands (attach, ls, etc.) and the `env` subcommand appear as suggestions.

---

### User Story 4 - Legacy K8s Commands Removed (Priority: P3)

The six legacy Kubernetes-specific top-level commands (deploy, connect, list, delete, logs, sync) are removed from the CLI. These commands predate the unified environment system and will be replaced by `env` subcommands with appropriate `--type` flags when K8s environment types are implemented.

**Why this priority**: These commands are dead weight in the current CLI. Removing them cleans up the command surface and avoids confusion between the old K8s-specific commands and the new unified env system.

**Independent Test**: Can be tested by running each removed command and verifying that the CLI returns an "unknown command" error.

**Acceptance Scenarios**:

1. **Given** a user runs `cc-deck deploy myenv`, **When** the command is not found, **Then** the CLI returns an error indicating the command does not exist.
2. **Given** a user runs `cc-deck connect myenv`, **When** the command is not found, **Then** the CLI returns an error indicating the command does not exist.
3. **Given** a user runs `cc-deck sync mydir`, **When** the command is not found, **Then** the CLI returns an error indicating the command does not exist.
4. **Given** a user runs `cc-deck --help`, **When** the help output is displayed, **Then** none of the removed commands (deploy, connect, delete, sync) appear.

---

### Edge Cases

- What happens when a user runs `cc-deck list` with no environments configured? The behavior must match `cc-deck env list` exactly (empty list, not an error).
- What happens when shell completion scripts generated before the restructuring are used? Old completions referencing removed commands (deploy, connect, etc.) will simply not match, which is acceptable. Users should regenerate completions.
- What happens when `cc-deck` is run with no subcommand? It displays the help output (reserved for a future TUI session list).
- What happens when a promoted command name conflicts with a flag or alias? The six promoted commands (attach, list, ls, status, start, stop, logs) must not conflict with any existing top-level flags or aliases.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The CLI MUST provide the following commands at the top level: `attach`, `list` (with alias `ls`), `status`, `start`, `stop`, `logs`.
- **FR-002**: Each promoted top-level command MUST produce identical behavior (output, exit codes, side effects) to its corresponding `env` subcommand.
- **FR-003**: All flags and arguments supported by the `env` subcommands MUST work identically when the command is invoked at the top level.
- **FR-004**: The `env` subcommand namespace MUST continue to work for all commands, including the six promoted ones.
- **FR-005**: The help output MUST organize commands into four groups displayed in this order: "Daily", "Session", "Environment", "Setup".
- **FR-006**: The "Daily" group MUST contain: attach, list (ls), status, start, stop, logs.
- **FR-007**: The "Session" group MUST contain: snapshot.
- **FR-008**: The "Environment" group MUST contain: env.
- **FR-009**: The "Setup" group MUST contain: plugin, profile, domains, image.
- **FR-010**: The following legacy top-level commands MUST be removed: deploy, connect, list (K8s-specific), delete (K8s-specific), logs (K8s-specific), sync.
- **FR-011**: Running `cc-deck` with no subcommand MUST display the help output.
- **FR-012**: Shell completion MUST include both top-level promoted commands and the `env` subcommand with its full set of subcommands.
- **FR-013**: The `hook`, `version`, and `completion` commands MUST remain at the top level, outside of any named group (or in a default/utility group).
- **FR-014**: Commands that remain under `env` only (create, delete, exec, push, pull, harvest, prune) MUST NOT appear at the top level.

### Key Entities

- **Command Group**: A named category used to organize commands in help output (Daily, Session, Environment, Setup).
- **Promoted Command**: A command that exists both at the top level and under the `env` subcommand, with identical behavior in both locations.
- **Legacy Command**: A top-level command from the K8s-era CLI that is removed in this restructuring.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users can run the six daily commands (attach, list, status, start, stop, logs) with one fewer word than before, reducing keystrokes per invocation by 25-40%.
- **SC-002**: Help output displays commands in four named groups, with "Daily" commands appearing first, verified by visual inspection and automated test.
- **SC-003**: 100% of promoted commands produce byte-identical output through both the top-level and `env` subcommand paths, verified by automated comparison tests.
- **SC-004**: Zero legacy K8s commands (deploy, connect, delete, sync) appear in help output or are executable after the restructuring.
- **SC-005**: Shell completion works for all top-level commands and all `env` subcommands without conflicts or duplicates.
- **SC-006**: The bare `cc-deck` invocation (no subcommand) displays help output, preserving future extensibility for a TUI session list.

## Assumptions

- The six legacy K8s commands (deploy, connect, list, delete, logs, sync) have no active users who depend on them, since K8s environment types are not yet implemented in the unified env system.
- Shell completion regeneration is an acceptable migration step for users who had previously generated completions.
- The "hook" command is internal and does not need to appear in any named help group.
- The `version` and `completion` commands remain at top level and appear in the default (ungrouped) section of help output.

## Out of Scope

- Implementing K8s environment types (`--type k8s-deploy`, `--type k8s-sandbox`) within the `env` system. That is a separate feature.
- Building the TUI session list for the bare `cc-deck` invocation. The bare command shows help for now.
- Nesting setup commands (plugin, profile, domains, image) under a `setup` or `config` parent command. They remain at the top level, organized only by help output grouping.
- Deprecation warnings for removed commands. Since K8s commands are not yet in active use, clean removal is sufficient.
