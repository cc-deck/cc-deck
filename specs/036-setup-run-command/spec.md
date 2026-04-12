# Feature Specification: cc-deck setup run

**Feature Branch**: `036-setup-run-command`
**Created**: 2026-04-12
**Status**: Draft
**Input**: User description: "Add cc-deck setup run command to execute pre-generated build artifacts (Containerfile or Ansible playbooks) directly, without Claude Code involvement"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Run Container Build (Priority: P1)

A developer has already generated a Containerfile via `/cc-deck.build` and wants to rebuild the container image after making a small change (e.g., fixing a shell config line). They run `cc-deck setup run` from their project directory. The command auto-detects the container target from the existing Containerfile, loads the image name and tag from the manifest, and executes the container build with streaming output.

**Why this priority**: This is the most common use case. Developers iterate on container builds frequently after the initial artifact generation.

**Independent Test**: Can be tested by running `cc-deck setup run` in a directory with a Containerfile and `cc-deck-setup.yaml`, verifying the container runtime is invoked with the correct arguments.

**Acceptance Scenarios**:

1. **Given** a setup directory with a Containerfile and manifest, **When** the user runs `cc-deck setup run`, **Then** the container runtime builds the image with the correct name:tag from the manifest and streams output to the terminal.
2. **Given** a setup directory with a Containerfile, **When** the build fails, **Then** the command exits with the container runtime's exit code and the full error output is visible.
3. **Given** a setup directory with a Containerfile but no container runtime installed, **When** the user runs `cc-deck setup run`, **Then** the command prints "neither podman nor docker found in PATH" and exits with an error.

---

### User Story 2 - Run SSH Provisioning (Priority: P1)

A developer has generated Ansible playbooks via `/cc-deck.build` for an SSH target and wants to re-provision after updating a role (e.g., adding PATH export to shell_config). They run `cc-deck setup run` and the command auto-detects the SSH target from the existing `site.yml` and `inventory.ini`, then executes the Ansible playbook.

**Why this priority**: Equal to container builds. SSH provisioning is the other primary target type.

**Independent Test**: Can be tested by running `cc-deck setup run` in a directory with `site.yml` and `inventory.ini`, verifying `ansible-playbook` is invoked correctly.

**Acceptance Scenarios**:

1. **Given** a setup directory with `site.yml` and `inventory.ini`, **When** the user runs `cc-deck setup run`, **Then** `ansible-playbook -i inventory.ini site.yml` is executed with output streamed to the terminal.
2. **Given** a setup directory with Ansible artifacts but `ansible-playbook` not installed, **When** the user runs `cc-deck setup run`, **Then** the command prints an error with install instructions and exits.
3. **Given** a setup directory with Ansible artifacts, **When** the playbook fails, **Then** the command exits with the playbook's exit code.

---

### User Story 3 - Push Container Image (Priority: P2)

After a successful container build, a developer wants to push the image to the configured registry. They run `cc-deck setup run --push`, which builds and then pushes.

**Why this priority**: Pushing is a secondary action that depends on a successful build.

**Independent Test**: Can be tested by running `cc-deck setup run --push` with a manifest that has `targets.container.registry` set, verifying the push command is executed after the build.

**Acceptance Scenarios**:

1. **Given** a successful container build and a manifest with `targets.container.registry` set, **When** the user runs `cc-deck setup run --push`, **Then** the image is pushed to `<registry>/<name>:<tag>`.
2. **Given** a manifest without `targets.container.registry`, **When** the user runs `cc-deck setup run --push`, **Then** the command prints "targets.container.registry not set in manifest" and exits with an error.
3. **Given** `--push` with an SSH target, **When** the user runs `cc-deck setup run --target ssh --push`, **Then** the command prints an error that `--push` is only valid for container targets.

---

### User Story 4 - Explicit Target Selection (Priority: P2)

A developer has both container and SSH artifacts in the same setup directory and needs to specify which target to run. They use `--target container` or `--target ssh` to disambiguate.

**Why this priority**: Multi-target setups are less common but must be supported.

**Independent Test**: Can be tested with both Containerfile and Ansible artifacts present, verifying `--target` selects the correct backend.

**Acceptance Scenarios**:

1. **Given** a setup directory with both Containerfile and Ansible artifacts, **When** the user runs `cc-deck setup run` without `--target`, **Then** the command prints an error asking the user to specify `--target container` or `--target ssh`.
2. **Given** both artifact types exist, **When** the user runs `cc-deck setup run --target ssh`, **Then** only the Ansible playbook is executed.

---

### Edge Cases

- What happens when the setup directory has no artifacts at all? The command prints "no build artifacts found; run /cc-deck.build to generate them" and exits with an error.
- What happens when the manifest file is missing or malformed? The command uses existing `LoadManifest()` validation and reports the parse error.
- What happens when `--push` is used with SSH target? The command rejects the combination with a clear error message.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST auto-detect the target type from generated artifacts when `--target` is not specified.
- **FR-002**: System MUST execute `<runtime> build -t <name>:<tag> -f Containerfile .` for container targets, using the image reference from the manifest.
- **FR-003**: System MUST execute `ansible-playbook -i inventory.ini site.yml` for SSH targets.
- **FR-004**: System MUST stream stdout and stderr from the build tool to the terminal in real time.
- **FR-005**: System MUST pass through the exit code from the build tool as its own exit code.
- **FR-006**: System MUST support `--push` flag for container targets, executing `<runtime> push <registry>/<name>:<tag>` after a successful build.
- **FR-007**: System MUST validate that `--push` requires `targets.container.registry` in the manifest.
- **FR-008**: System MUST reject `--push` when used with SSH target.
- **FR-009**: System MUST use existing container runtime detection (prefers podman, falls back to docker).
- **FR-010**: System MUST resolve the setup directory using existing directory resolution logic.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users can rebuild a container image with a single command without invoking Claude Code.
- **SC-002**: Users can re-provision an SSH host with a single command without invoking Claude Code.
- **SC-003**: Build output is visible in real time, matching what users would see running podman or ansible-playbook directly.
- **SC-004**: Exit codes from build tools are preserved, enabling use in CI/CD pipelines and scripts.
- **SC-005**: Auto-detection correctly identifies the target in single-target setups without requiring `--target`.

## Assumptions

- Build artifacts (Containerfile, Ansible playbooks) have already been generated by `/cc-deck.build` before `cc-deck setup run` is invoked.
- The manifest file (`cc-deck-setup.yaml`) exists and is valid in the setup directory.
- For container targets, the container runtime (podman or docker) is installed on the local machine.
- For SSH targets, `ansible-playbook` is installed on the local machine and the remote host is reachable.
- No automatic retry or self-correction is needed; that remains Claude Code's responsibility during `/cc-deck.build`.
