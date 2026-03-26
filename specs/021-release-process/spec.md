# Feature Specification: Release Process

**Feature Branch**: `021-release-process`
**Created**: 2026-03-15
**Status**: Evolved (2026-03-26)
**Input**: User description: "Release process for cc-deck using GoReleaser with cross-platform binaries, Homebrew, RPM, DEB, Flatpak, container images, and registry migration to quay.io/cc-deck"

> **Evolution Note (2026-03-26)**: Flatpak packaging (User Story 4, FR-012)
> moved to Out of Scope. Never implemented, deferred to future work.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Install cc-deck via Homebrew on macOS (Priority: P1)

A macOS user discovers cc-deck and wants to install it without cloning the repository or building from source. They run a single Homebrew command and get a working installation with the CLI binary, the WASM plugin, and all layout files ready to use.

**Why this priority**: macOS is the primary development platform for Claude Code users. Homebrew is the expected installation method. Without it, adoption requires building from source, which is a significant barrier.

**Independent Test**: Run `brew install cc-deck/tap/cc-deck` on a clean macOS machine and verify `cc-deck plugin install` works, producing a functional Zellij + cc-deck setup.

**Acceptance Scenarios**:

1. **Given** Homebrew is installed on macOS, **When** the user runs `brew install cc-deck/tap/cc-deck`, **Then** the cc-deck CLI binary is installed and available in PATH.
2. **Given** cc-deck was installed via Homebrew, **When** the user runs `cc-deck plugin install`, **Then** the WASM plugin, layouts, and hooks are installed into Zellij's configuration directory.
3. **Given** a new version is released, **When** the user runs `brew upgrade cc-deck`, **Then** the latest version is installed.
4. **Given** the Homebrew formula is installed, **When** Zellij is not installed, **Then** the formula recommends Zellij but does not require it (Zellij is an optional dependency).

---

### User Story 2 - Download Binary from GitHub Release (Priority: P1)

A Linux or macOS user wants to install cc-deck by downloading a pre-built binary from GitHub Releases. They visit the releases page, download the archive for their platform and architecture, extract it, and run the installer.

**Why this priority**: GitHub Releases is the universal fallback for users who cannot use a package manager. Both arm64 and amd64 must be available for Linux and macOS.

**Independent Test**: Download the archive for the current platform from the GitHub Release page, extract it, and verify the binary runs and `cc-deck plugin install` completes successfully.

**Acceptance Scenarios**:

1. **Given** a new version tag is pushed, **When** the release workflow completes, **Then** GitHub Releases contains archives for linux/amd64, linux/arm64, darwin/amd64, and darwin/arm64.
2. **Given** the user downloads and extracts the archive, **When** they run the cc-deck binary, **Then** it reports the correct version and the WASM plugin is bundled.
3. **Given** the release is published, **When** the user views the release page, **Then** a SHA-256 checksums file is included for all artifacts.

---

### User Story 3 - Install via Linux Package Manager (Priority: P2)

A Fedora user installs cc-deck via an RPM package. A Debian/Ubuntu user installs via a DEB package. Both get proper system integration with package metadata, dependencies, and uninstall support.

**Why this priority**: Native package manager integration provides the best Linux experience (automatic updates, dependency management, clean uninstall). Fedora and Debian/Ubuntu cover the majority of Linux developer workstations.

**Independent Test**: Install the RPM on Fedora via `dnf install` and the DEB on Ubuntu via `apt install`, verify the binary is in PATH and `cc-deck plugin install` works.

**Acceptance Scenarios**:

1. **Given** a new version is released, **When** the release workflow completes, **Then** RPM packages for amd64 and arm64 are attached to the GitHub Release.
2. **Given** a new version is released, **When** the release workflow completes, **Then** DEB packages for amd64 and arm64 are attached to the GitHub Release.
3. **Given** the user installs the RPM or DEB package, **When** they run `cc-deck --version`, **Then** it reports the correct version.

---

### ~~User Story 4 - Install via Flatpak (Priority: P2)~~ DESCOPED

> **Descoped (2026-03-26)**: Flatpak packaging was never implemented.
> Deferred to future work. RPM/DEB and Homebrew cover primary distribution channels.

---

### User Story 5 - Automated Release on Version Tag (Priority: P1)

A maintainer pushes a version tag (`v0.3.0`) to trigger the full release pipeline. The pipeline builds all artifacts, creates a GitHub Release, publishes to Homebrew, and pushes container images, all without manual intervention.

**Why this priority**: Automation is the foundation. Every other user story depends on the release pipeline producing correct artifacts. Manual releases are error-prone and do not scale.

**Independent Test**: Push a version tag to the repository and verify the GitHub Release is created with all expected artifacts, the Homebrew tap is updated, and container images are pushed.

**Acceptance Scenarios**:

1. **Given** the main branch is ready for release, **When** a maintainer pushes tag `v0.3.0`, **Then** the release workflow triggers automatically.
2. **Given** the release workflow runs, **When** it completes, **Then** a GitHub Release is created with changelog, binaries for all platforms, RPM/DEB packages, and checksums.
3. **Given** the release workflow runs, **When** it completes, **Then** the Homebrew tap repository is updated with the new formula.
4. **Given** the release workflow runs, **When** it completes, **Then** container images are pushed to `quay.io/cc-deck` with the version tag and `latest`.

---

### User Story 6 - Container Images from New Registry (Priority: P1)

A user pulls the cc-deck demo image from the new `quay.io/cc-deck` organization. All documentation, Makefile references, and image labels reference the new registry. The old `quay.io/rhuss` images remain available but documentation points to the new location.

**Why this priority**: The registry migration from personal (`quay.io/rhuss`) to organizational (`quay.io/cc-deck`) establishes the project's independent identity and is a prerequisite for the release pipeline.

**Independent Test**: Pull `quay.io/cc-deck/cc-deck-demo:latest` and verify it works identically to the current `quay.io/rhuss/cc-deck-demo:latest`.

**Acceptance Scenarios**:

1. **Given** the registry migration is complete, **When** a user runs `podman pull quay.io/cc-deck/cc-deck-demo:latest`, **Then** the image is available for both arm64 and amd64.
2. **Given** the release workflow pushes images, **When** a user checks `quay.io/cc-deck`, **Then** both `cc-deck-base` and `cc-deck-demo` images are available with version tags.
3. **Given** the migration is complete, **When** searching the codebase for `quay.io/rhuss`, **Then** no references remain in documentation, Makefile, or configuration files.

---

### Edge Cases

- What happens when the WASM plugin build fails during release? The release workflow should fail fast and not publish partial artifacts.
- What happens when the Homebrew tap update fails? The GitHub Release should still be created, with a warning that the tap needs manual update.
- What happens when quay.io is unreachable during release? The release should complete for GitHub artifacts and report the image push failure separately.
- What happens when a user has the old `quay.io/rhuss` images? They continue to work but are no longer updated. Documentation points to the new location.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The release pipeline MUST build the Go CLI with the embedded WASM plugin for four platform/architecture combinations: darwin/amd64, darwin/arm64, linux/amd64, linux/arm64.
- **FR-002**: The release pipeline MUST produce `.tar.gz` archives containing the CLI binary, README, and LICENSE for each platform.
- **FR-003**: The release pipeline MUST generate a SHA-256 checksums file for all release artifacts.
- **FR-004**: The release pipeline MUST create and update a Homebrew formula in a dedicated tap repository.
- **FR-005**: The release pipeline MUST produce RPM packages for amd64 and arm64.
- **FR-006**: The release pipeline MUST produce DEB packages for amd64 and arm64.
- **FR-007**: The release pipeline MUST build and push multi-arch container images (arm64 + amd64) to `quay.io/cc-deck` with version tags and `latest`.
- **FR-008**: The release pipeline MUST be triggered by pushing a version tag matching `v*` to the repository.
- **FR-009**: The release pipeline MUST build the Rust WASM plugin before cross-compiling the Go CLI.
- **FR-010**: All references to `quay.io/rhuss` in documentation, Makefile, and configuration files MUST be updated to `quay.io/cc-deck`.
- **FR-011**: The release pipeline MUST generate a changelog from commit history for the GitHub Release.
- ~~**FR-012**: A Flatpak manifest MUST be created for submission to Flathub.~~ DESCOPED.
- **FR-013**: Version numbers MUST be derived from the git tag at release time. The Makefile `VERSION` and Cargo.toml `version` are updated post-release for development builds.

### Key Entities

- **Release Artifact**: A versioned binary, package, or container image produced by the release pipeline. Identified by platform, architecture, format, and version.
- **Homebrew Tap**: A separate repository (`cc-deck/homebrew-tap`) containing the Homebrew formula. Updated automatically on each release.
- **Container Image**: An OCI-compliant multi-arch image pushed to `quay.io/cc-deck`. Two variants: `cc-deck-base` (developer toolbox) and `cc-deck-demo` (ready-to-run with Claude Code).

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A maintainer can produce a complete release (all artifacts, all platforms) by pushing a single version tag, with no manual steps.
- **SC-002**: macOS users can install cc-deck via `brew install cc-deck/tap/cc-deck` within 5 minutes of the release being published.
- **SC-003**: All release artifacts are available for download within 10 minutes of tagging.
- **SC-004**: Container images are available at `quay.io/cc-deck` with both arm64 and amd64 support.
- **SC-005**: Zero references to `quay.io/rhuss` remain in the codebase after migration.
- **SC-006**: The release can be tested locally (dry run) before pushing the tag, producing the same artifacts without publishing.

## Clarifications

### Session 2026-03-15

- Q: Should the Homebrew formula declare Zellij as a dependency? → A: Zellij should be an optional (recommended) dependency, not a hard requirement. cc-deck can run without Zellij when operating inside a container.
- Q: How are version numbers managed across git tag, Makefile, and Cargo.toml? → A: Git tag is the source of truth. GoReleaser reads the version from the tag at release time. Makefile and Cargo.toml versions are for development builds and get updated post-release.

## Assumptions

- GoReleaser (open source edition) provides all needed features (cross-compile, nFPM for RPM/DEB, Homebrew tap).
- The `cc-deck` GitHub organization has permissions to create the `homebrew-tap` repository.
- The `quay.io/cc-deck` organization is set up with push credentials available as GitHub Actions secrets.
- Flatpak/Flathub submission is a separate follow-up process after the initial release pipeline is working.
- The WASM plugin can be built on GitHub Actions runners (Ubuntu) with the `wasm32-wasip1` Rust target.

## Scope Boundaries

### In Scope

- GoReleaser configuration for cross-platform binary distribution
- Homebrew tap repository and formula
- RPM and DEB packages via nFPM
- ~~Flatpak manifest for Flathub submission~~ DESCOPED
- GitHub Actions release workflow
- Container image build and push to quay.io/cc-deck
- Registry migration (quay.io/rhuss to quay.io/cc-deck)
- Documentation and Makefile updates for new registry
- Version synchronization (git tag, Makefile, Cargo.toml)

### Out of Scope

- Snap package (dropped per brainstorm decision)
- Windows binaries (no Windows support planned)
- Automatic version bumping (manual tag push)
- Signing binaries with GPG or cosign (future enhancement)
- COPR repository for Fedora (future, RPM available via GitHub Release for now)
- APT repository / PPA for Debian (future, DEB available via GitHub Release for now)
