# Feature Specification: Unified Setup Command

**Feature Branch**: `034-unified-setup-command`
**Created**: 2026-04-08
**Status**: Draft
**Input**: User description: "Unify cc-deck image and SSH remote provisioning into a single cc-deck setup command with a shared manifest, two Claude commands (capture + build), and Ansible playbooks as the SSH provisioning backend"

## Clarifications

### Session 2026-04-08

- Q: When `/cc-deck.build --target ssh` is re-run after playbooks already exist with user modifications, how should conflicts be handled? → A: Show diff of changed roles and ask user to choose (same as container path, consistent with US-2 AS-4).
- Q: What is the source of SSH authorized keys when `create_user: true`? → A: Use the public key matching `targets.ssh.identity_file` (append `.pub`). The Ansible `base` role reads the local public key file and installs it as the new user's authorized key.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Initialize a Setup Profile for a New Project (Priority: P1)

A developer starting a new project wants to create a developer environment profile that captures their local tools, shell configuration, and Claude Code plugins. They run a CLI command to scaffold the manifest and install Claude Code slash commands into their project. From there, they use the `/cc-deck.capture` slash command to discover and record their local setup into the manifest.

**Why this priority**: Without initialization, no other workflow is possible. The scaffold and capture flow is the entry point for both container and SSH targets.

**Independent Test**: Can be fully tested by running the CLI init command and verifying that the manifest template, Claude commands, and directory structure are created correctly.

**Acceptance Scenarios**:

1. **Given** a project directory without a setup profile, **When** the user runs the init command with `--target container`, **Then** a `.cc-deck/setup/` directory is created containing a manifest template with the container target section uncommented, and Claude commands are installed to `.claude/commands/`.
2. **Given** a project directory without a setup profile, **When** the user runs the init command with `--target ssh`, **Then** a `.cc-deck/setup/` directory is created containing a manifest template with the SSH target section uncommented, empty Ansible role skeletons in `roles/`, and Claude commands are installed to `.claude/commands/`.
3. **Given** a project directory without a setup profile, **When** the user runs the init command with `--target container,ssh`, **Then** both target sections are uncommented and Ansible role skeletons are scaffolded alongside the container build-context directory.
4. **Given** a scaffolded setup profile, **When** the user runs `/cc-deck.capture` in Claude Code, **Then** the command discovers local tools (from build files, CI configs, version files), shell configuration, Claude Code plugins, and MCP servers, and writes them into the shared sections of the manifest.

---

### User Story 2 - Build a Container Image from the Manifest (Priority: P1)

A developer who has captured their local setup wants to produce a container image that replicates their developer environment. They run the `/cc-deck.build` slash command targeting containers. Claude generates a Containerfile from the manifest, builds the image, and self-corrects on build failures. Optionally, the built image is pushed to a registry in the same step.

**Why this priority**: Container image building is the existing core functionality being preserved and unified. It must continue to work as before under the new command structure.

**Independent Test**: Can be fully tested by initializing a profile, populating the manifest, running the build command targeting containers, and verifying the resulting image contains the expected tools.

**Acceptance Scenarios**:

1. **Given** a manifest with tools, settings, and a container target section, **When** the user runs `/cc-deck.build --target container`, **Then** a Containerfile is generated from the manifest, the image is built using the container runtime, and the result is reported (image name, size).
2. **Given** a manifest with a container target and a registry configured, **When** the user runs `/cc-deck.build --target container --push`, **Then** the image is built and pushed to the configured registry.
3. **Given** a Containerfile that fails to build, **When** the build command encounters the failure, **Then** Claude reads the error output, fixes the Containerfile, and retries (up to 3 attempts).
4. **Given** a previously generated Containerfile, **When** the user runs the build command again after manifest changes, **Then** Claude shows the diff between the existing and newly generated Containerfile and asks the user to choose (use new, keep existing, or stop).

---

### User Story 3 - Provision an SSH Remote Machine via Ansible (Priority: P1)

A developer who has captured their local setup wants to provision a remote machine (e.g., a Hetzner VM) so that it mirrors their developer environment. They run the `/cc-deck.build` slash command targeting SSH. Claude generates Ansible playbooks from the manifest, runs `ansible-playbook` against the remote, and self-corrects on task failures. After convergence, the playbooks can be re-run standalone without Claude involvement.

**Why this priority**: SSH provisioning via Ansible is the primary new capability this feature adds. It is the core motivation for the unified design.

**Independent Test**: Can be fully tested by initializing a profile with an SSH target, populating the manifest, running the build command targeting SSH, and verifying the remote machine has the expected tools installed.

**Acceptance Scenarios**:

1. **Given** a manifest with tools, settings, and an SSH target section, **When** the user runs `/cc-deck.build --target ssh`, **Then** Ansible playbooks are generated as a roles directory structure, and `ansible-playbook` is executed against the remote host.
2. **Given** a generated Ansible playbook that fails on a task, **When** the build command encounters the failure, **Then** Claude reads the Ansible error output, fixes the relevant role, and re-runs (up to 3 attempts). Already-succeeded tasks are skipped on retry.
3. **Given** a converged set of Ansible playbooks, **When** a user runs `ansible-playbook -i inventory.ini site.yml` from the command line without Claude, **Then** the playbooks execute successfully and the remote machine state converges to the manifest definition.
4. **Given** an SSH target with `create_user: true`, **When** the playbooks run against a fresh VM, **Then** the specified user is created with sudo access and SSH authorized keys configured.
5. **Given** a manifest with tools, plugins, and shell settings, **When** the playbooks complete on the remote, **Then** the remote has Zellij, Claude Code (via official installer), cc-deck CLI (from GitHub Releases), the cc-deck plugin (via `cc-deck plugin install`), and the specified shell configuration with credential sourcing enabled.
6. **Given** previously generated Ansible role task files, **When** the user runs `/cc-deck.build --target ssh` again after manifest changes, **Then** Claude shows the diff between existing and newly generated role files and asks the user to choose (use new, keep existing, or stop).

---

### User Story 4 - Reuse a Single Capture for Both Targets (Priority: P2)

A developer maintains both a container image and an SSH-provisioned VM for the same project. They run `/cc-deck.capture` once to discover their local tools and configuration, then run `/cc-deck.build` twice with different `--target` flags to produce both a container image and a provisioned remote machine from the same manifest.

**Why this priority**: This is the key value proposition of the unified design, but it requires both individual targets to work first.

**Independent Test**: Can be fully tested by initializing with both targets, running capture once, then running build for each target separately and verifying both produce correct results from the same manifest.

**Acceptance Scenarios**:

1. **Given** a manifest with both container and SSH target sections, **When** the user runs `/cc-deck.capture`, **Then** the shared sections (tools, settings, plugins, MCP, github_tools) are populated and both target sections remain unchanged.
2. **Given** a manifest populated by a single capture run, **When** the user runs `/cc-deck.build --target container` followed by `/cc-deck.build --target ssh`, **Then** both produce correct artifacts that install the same set of tools and configuration.

---

### User Story 5 - Detect Manifest Drift (Priority: P3)

A developer has modified their manifest (added a tool, changed a setting) since the last build. They want to see what would change before regenerating. They run the CLI diff command to compare the current manifest against the last-generated artifacts.

**Why this priority**: Drift detection is a convenience feature that helps users understand what changed. It is valuable but not blocking for the core workflow.

**Independent Test**: Can be fully tested by generating artifacts, modifying the manifest, running the diff command, and verifying it reports the expected changes.

**Acceptance Scenarios**:

1. **Given** a manifest with a new tool added since the last Containerfile generation, **When** the user runs the diff command, **Then** the output shows the new tool as missing from the Containerfile.
2. **Given** a manifest with a new tool added since the last Ansible playbook generation, **When** the user runs the diff command, **Then** the output shows the new tool as missing from the roles.
3. **Given** a manifest that matches the last-generated artifacts, **When** the user runs the diff command, **Then** the output indicates no drift detected.

---

### User Story 6 - Verify a Provisioned Target (Priority: P3)

A developer wants to smoke-test whether a container image or SSH remote has the expected tools installed. They run the CLI verify command to check tool availability.

**Why this priority**: Verification is a quality assurance step, useful but not part of the core provisioning flow.

**Independent Test**: Can be fully tested by running verify against a provisioned target and checking the pass/fail report.

**Acceptance Scenarios**:

1. **Given** a built container image, **When** the user runs the verify command, **Then** the system runs checks inside the container (cc-deck version, Claude Code availability, language tools) and reports pass/fail per tool.
2. **Given** a provisioned SSH remote, **When** the user runs the verify command, **Then** the system runs the same checks via SSH against the remote host and reports pass/fail per tool.
3. **Given** a remote where a tool is missing, **When** the verify command runs, **Then** the failing tool is reported with a clear error message.

---

### Edge Cases

- What happens when the user runs `/cc-deck.build --target ssh` but Ansible is not installed locally? The command fails with a clear error message telling the user to install Ansible (e.g., `brew install ansible`).
- What happens when the SSH target host is unreachable during build? The `ansible-playbook` command fails with a connection error. Claude reports the error and does not retry (connectivity is not a fixable playbook issue).
- What happens when the manifest has no target section for the requested target type? The build command fails with a message telling the user to add the target section or re-run init with the appropriate `--target` flag.
- What happens when a user runs build for a target that was not scaffolded by init (e.g., SSH roles directory does not exist)? The build command creates the necessary directory structure on first run.
- What happens when the Ansible self-correction loop exhausts 3 retries without convergence? The command stops, reports the failing task and error, and leaves the playbooks in their last-edited state for manual inspection.
- What happens when both container and SSH artifacts exist and the user modifies only the shared manifest section? The diff command reports drift for both targets.
- What happens when Ansible role task files have been manually edited since the last build and the user re-runs `/cc-deck.build --target ssh`? Claude shows a diff of each changed role and asks the user to choose (use new generated content, keep existing, or stop). This mirrors the Containerfile conflict handling in US-2 AS-4.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST provide a CLI command (`cc-deck setup init`) that scaffolds a setup directory with a manifest template, Claude commands, and target-specific file structures.
- **FR-002**: The init command MUST accept a `--target` flag with values `container`, `ssh`, or a comma-separated combination of both. When `--target` is omitted, the full manifest template is generated with all target sections commented out.
- **FR-003**: System MUST install two Claude Code slash commands during init: `/cc-deck.capture` and `/cc-deck.build`.
- **FR-004**: The `/cc-deck.capture` command MUST discover local tools, shell configuration, Claude Code plugins, and MCP servers and write them into the manifest. It MUST be target-agnostic (not read or modify target-specific sections).
- **FR-005**: The `/cc-deck.build` command MUST accept a `--target` argument (`container` or `ssh`) to select the generation backend.
- **FR-006**: For `--target container`, the build command MUST generate a Containerfile from the manifest, build the image, and optionally push it when `--push` is specified.
- **FR-007**: For `--target ssh`, the build command MUST generate Ansible playbooks as a roles directory structure and execute `ansible-playbook` against the remote host defined in the manifest.
- **FR-008**: The Ansible playbooks MUST be idempotent and runnable standalone without Claude Code involvement after initial convergence.
- **FR-009**: The build command MUST implement a self-correction loop (up to 3 retries) for both container build failures and Ansible task failures.
- **FR-010**: System MUST provide a CLI command (`cc-deck setup verify`) that smoke-tests a provisioned target (container image or SSH remote) for expected tool availability.
- **FR-011**: System MUST provide a CLI command (`cc-deck setup diff`) that compares the current manifest against last-generated artifacts and reports drift.
- **FR-012**: The manifest MUST support both target sections (container and SSH) simultaneously, with shared tool/settings sections.
- **FR-013**: For SSH targets with `create_user: true`, the Ansible playbooks MUST create the specified user with sudo access and SSH authorized keys from the public key matching `targets.ssh.identity_file` (the `.pub` counterpart).
- **FR-014**: The Ansible playbooks MUST install credential sourcing in the shell configuration (source `~/.config/cc-deck/credentials.env` if it exists). Actual credentials are NOT stored in playbooks.
- **FR-015**: The build command MUST check that Ansible is installed locally before attempting SSH target builds and provide a clear error message if missing.
- **FR-016**: The Ansible playbooks MUST run `cc-deck plugin install` on the remote to set up the WASM plugin, layout files, controller config, and Claude Code hooks.
- **FR-017**: The existing `cc-deck image` command MUST be replaced by `cc-deck setup` (rename, no backwards compatibility needed).
- **FR-018**: The `cc-deck env create --type ssh` flow MUST be simplified to remove the pre-flight bootstrap. It MUST perform a lightweight probe (`which zellij && which cc-deck && which claude`) and fail with a clear message if the host appears unprovisioned.
- **FR-019**: Multiple environments MUST be allowed to target the same SSH host (different workspaces, different names).

### Key Entities

- **Setup Manifest**: The declarative profile describing what tools, configuration, and plugins to install. Contains shared sections (tools, settings, plugins, MCP, github_tools, sources) and per-target sections (container, SSH). File name: `cc-deck-setup.yaml`.
- **Setup Directory**: The `.cc-deck/setup/` directory containing the manifest, generated artifacts (Containerfile or Ansible playbooks), and supporting files (build-context, inventory, group_vars).
- **Ansible Role**: A self-contained provisioning unit handling one concern (base system, tools, Zellij, Claude Code, cc-deck, shell config, MCP). Each role has tasks, templates, and defaults.
- **Lightweight Probe**: A single SSH command run during `env create --type ssh` to verify the host has required tools. Not a full bootstrap, just a pass/fail check.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A developer can go from a bare VM to a fully provisioned cc-deck environment (with Zellij, Claude Code, cc-deck, shell config, and credential sourcing) in under 15 minutes using the setup command.
- **SC-002**: A single `/cc-deck.capture` run populates a manifest that successfully drives both container image building and SSH provisioning without manual editing of the shared sections.
- **SC-003**: Converged Ansible playbooks can be re-run standalone (without Claude Code) and complete successfully with no changes on an already-provisioned machine.
- **SC-004**: The self-correction loop resolves at least 80% of first-run Ansible failures (package name mismatches, wrong download URLs, missing dependencies) within 3 retries.
- **SC-005**: All 10 testing findings from the original brainstorm (F-001 through F-010) are resolved by the new provisioning approach.
- **SC-006**: The verify command correctly detects missing tools on both container and SSH targets and reports clear pass/fail results.

## Assumptions

- Ansible is available on the user's local machine. The system does not install Ansible; it documents `brew install ansible` (macOS) as a prerequisite.
- The SSH target runs a Linux distribution with a standard package manager. Initial support targets Fedora/RHEL (dnf). Other distributions (apt-based) can be added later.
- The user has SSH key-based access to the target machine. Password-based SSH is not supported for Ansible execution.
- Claude Code auto-updates itself on the remote, so version pinning for Claude Code is not needed.
- The existing `cc-deck env create --type ssh` is used for session lifecycle management. The setup command handles machine-level provisioning only.
- Credential forwarding continues to work through the existing `cc-deck env attach` flow. Ansible provisions the sourcing mechanism, not the actual secrets.
- The container build backend preserves all existing `cc-deck image` functionality (manifest schema, Containerfile generation, multi-arch builds, self-correction loop).
