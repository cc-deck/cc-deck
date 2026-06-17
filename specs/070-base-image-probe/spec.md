# Feature Specification: Base Image Probe

**Feature Branch**: `070-base-image-probe`
**Created**: 2026-06-17
**Status**: Draft
**Input**: User description: "Inspect the selected base image during the build phase to discover what's pre-installed (OS, package manager, tools, user setup), diff against manifest requirements, and generate only the missing install steps. Supports switching between base images (Fedora, UBI, OpenShell Debian) without manual Containerfile adjustment."

## User Scenarios & Testing

### User Story 1 - Build adapts to a new base image automatically (Priority: P1)

A developer switches their manifest's base image from Fedora 41 to a UBI 9 image. When they run `/cc-deck.build`, the build command inspects the new base image, discovers it uses `dnf` (same as Fedora) but lacks some tools that Fedora included. The generated Containerfile installs only the missing tools instead of duplicating what the base already provides.

**Why this priority**: This is the core value. Without it, switching base images requires manual Containerfile editing and trial-and-error builds.

**Independent Test**: Can be tested by running `/cc-deck.build` against two different base images with the same manifest and verifying both produce working images with the correct tools installed.

**Acceptance Scenarios**:

1. **Given** a manifest requiring Go 1.25 and Python 3, **When** the base image already has Python 3 installed, **Then** the generated Containerfile does NOT include a Python install step
2. **Given** a manifest requiring Go 1.25 and Python 3, **When** the base image has neither, **Then** the generated Containerfile installs both using the base image's package manager
3. **Given** a Fedora-based base image, **When** the build runs, **Then** install commands use `dnf`
4. **Given** a Debian-based base image (OpenShell), **When** the build runs, **Then** install commands use `apt-get`

---

### User Story 2 - Probe results inform the learnings file (Priority: P2)

After probing a base image, the discovered capabilities (OS family, package manager, pre-installed tools) are recorded in the build learnings file. On subsequent builds against the same base image, the probe step is skipped and cached results are used instead.

**Why this priority**: Probe operations require pulling and running the base image, which takes time. Caching makes repeat builds faster.

**Independent Test**: Run a build twice against the same base image. The first build probes and records. The second build skips the probe and uses cached results.

**Acceptance Scenarios**:

1. **Given** no cached probe results exist for a base image, **When** the build runs, **Then** a full probe is executed and results are saved to the learnings file
2. **Given** cached probe results exist for a base image, **When** the build runs and the base image digest has not changed, **Then** the cached results are used without running the probe
3. **Given** cached probe results exist but the base image digest has changed, **When** the build runs, **Then** a fresh probe is executed and the cache is updated

---

### User Story 3 - Probe report shows base image capabilities (Priority: P3)

Before generating the Containerfile, the build command prints a summary of what the probe discovered: OS name/version, package manager, pre-installed tools with versions, default user, shell availability. This helps the developer understand what their base image provides.

**Why this priority**: Visibility into what the base image contains aids debugging and builds trust in the automated tool selection.

**Independent Test**: Run a build and verify the probe summary appears in the output before Containerfile generation begins.

**Acceptance Scenarios**:

1. **Given** any base image, **When** the probe runs, **Then** a summary table is printed showing: OS name, package manager, user/home, shell, and discovered tools with versions
2. **Given** a manifest with 5 required tools where 3 are pre-installed, **When** the probe summary is shown, **Then** it clearly indicates which tools need installation and which are already present

---

### Edge Cases

- What happens when the base image cannot be pulled (network error, authentication required)?
- What happens when `podman run` fails on the base image (incompatible architecture, entrypoint failure)?
- What happens when a tool exists in the base image but at an incompatible version (e.g., Go 1.21 when 1.25 is required)?
- What happens when the base image has no package manager at all (distroless images)?

## Requirements

### Functional Requirements

- **FR-001**: The build command MUST inspect the base image before generating Containerfile instructions, by running probe commands inside a temporary container
- **FR-002**: The probe MUST detect the OS family (Fedora/RHEL, Debian/Ubuntu, Alpine) and available package manager (dnf, apt-get, apk)
- **FR-003**: The probe MUST discover pre-installed tools by checking common binary locations and running version commands
- **FR-004**: The probe MUST detect the default non-root user, home directory, and available shells
- **FR-005**: The Containerfile generator MUST use the probed package manager for install commands instead of assuming a fixed one
- **FR-006**: The Containerfile generator MUST skip installation of tools that the probe confirmed are already present at a compatible version
- **FR-007**: The probe results MUST be cached in a structured file (`probe-cache.json`) in the setup directory, keyed by base image reference and digest, so repeat builds skip the probe
- **FR-008**: The probe cache MUST be invalidated when the base image digest changes
- **FR-009**: The probe MUST complete within 30 seconds per base image (timeout and fall back to defaults if exceeded)
- **FR-010**: If the probe fails entirely (image pull failure, runtime error), the build MUST fall back to the current behavior (assume Fedora/dnf for container target, Debian/apt for OpenShell target)
- **FR-011**: When a tool is present in the base image but at an incompatible version (older than what the manifest requires), the Containerfile MUST include an install step for the required version, overriding or shadowing the pre-installed one
- **FR-012**: When the base image has no recognized package manager (distroless or minimal images), the probe MUST report this and the build MUST fall back to binary-only install methods (GitHub releases, curl downloads) or fail with a clear error if package-manager-only tools are required

### Key Entities

- **ProbeResult**: What was discovered about a base image (OS family, package manager, installed tools with versions, user setup, shell availability)
- **ProbeCache**: Stored probe results in `probe-cache.json`, keyed by image reference + digest, with a timestamp for staleness detection

## Success Criteria

### Measurable Outcomes

- **SC-001**: A developer can switch base images in the manifest and get a working build on the first attempt for supported OS families (Fedora, Debian, UBI)
- **SC-002**: Repeat builds against the same unchanged base image skip the probe step, adding negligible overhead (under 1 second for cache lookup)
- **SC-003**: The build command correctly detects and uses the right package manager for at least 3 OS families (Fedora/dnf, Debian/apt-get, RHEL/dnf)
- **SC-004**: Tools already present in the base image are not reinstalled, reducing image layer count and build time

## Assumptions

- The base image can be run with `podman run --rm` for probing (it has a working entrypoint or can be overridden with `--entrypoint`)
- The probe runs on the build host architecture (no cross-architecture probing needed)
- Tool version detection uses standard `<tool> --version` or `<tool> version` patterns
- The probe does not need to detect language-specific package managers (pip, npm, cargo) since those are handled by language-specific install steps in the Containerfile
- The existing two-pass probe for OpenShell policy binary paths is a separate mechanism and continues to function independently
