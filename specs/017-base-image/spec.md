# Feature Specification: cc-deck Base Container Image

**Feature Branch**: `017-base-image`
**Created**: 2026-03-12
**Status**: Draft
**Input**: User description: "cc-deck base container image for Claude Code development environments"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Build a Project-Specific Claude Code Image (Priority: P1)

A developer wants to create a containerized Claude Code environment for their team.
They run `cc-deck build` which generates a Containerfile that uses the cc-deck base
image as the foundation. The base image provides all essential developer tools, runtimes,
and a configured shell environment out of the box, so the project-specific image only
needs to add project tools, Claude Code, and cc-deck itself.

**Why this priority**: The base image exists solely to be consumed by the user image
build pipeline. Without a working base image, no user images can be built.

**Independent Test**: Pull the base image, start a container, and verify that all
expected tools are available, the non-root user works correctly, and the shell
environment is properly configured.

**Acceptance Scenarios**:

1. **Given** the base image is pulled, **When** a container is started, **Then** the
   user lands in a zsh shell with starship prompt, as the `coder` user in `/home/coder`.
2. **Given** a running container, **When** the user runs `git`, `gh`, `rg`, `fd`, `jq`,
   `bat`, `lsd`, `fzf`, `hx`, `node`, `python3`, `uv`, **Then** all commands are found
   and functional.
3. **Given** a running container, **When** the user runs `sudo dnf install <package>`,
   **Then** the installation succeeds without a password prompt.
4. **Given** a running container, **When** the user runs `npm install -g <package>`,
   **Then** the package installs to `~/.local/lib/npm` without requiring root.

---

### User Story 2 - Multi-Architecture Support (Priority: P1)

A team uses both Intel/AMD workstations and Apple Silicon Macs for local development,
and deploys to amd64 Kubernetes clusters. The base image must work on both architectures
so that `podman pull` fetches the correct variant automatically.

**Why this priority**: Without multi-arch support, the image is unusable for a significant
portion of the target audience (Mac developers using Apple Silicon).

**Independent Test**: Pull the base image on both an amd64 and arm64 machine, run a
container, and verify all tools work correctly on both.

**Acceptance Scenarios**:

1. **Given** the image is published as a multi-arch manifest, **When** pulled on an
   arm64 Mac, **Then** the arm64 variant is used and all tools work.
2. **Given** the image is published as a multi-arch manifest, **When** pulled on an
   amd64 Linux host, **Then** the amd64 variant is used and all tools work.

---

### User Story 3 - Reproducible Shell Environment (Priority: P2)

A developer starts a container and expects a consistent, productive shell experience
with modern tools and sensible defaults, regardless of which team member built the image.
The shell includes a starship prompt showing git status and directory context, smart
directory jumping via zoxide, fuzzy finding via fzf, and aliases for common tools.

**Why this priority**: Developer experience matters for adoption. A bare shell with
no configuration discourages interactive use.

**Independent Test**: Start a container, verify the prompt renders correctly, test that
aliases work (`ls` shows lsd output, `cat` shows bat output), and confirm fzf and
zoxide are initialized.

**Acceptance Scenarios**:

1. **Given** a new container, **When** the shell starts, **Then** the starship prompt
   displays the current directory and git branch (if in a repo).
2. **Given** a new container, **When** the user types `ls`, **Then** lsd output is shown
   (with colors and icons).
3. **Given** a new container, **When** the user types `cat <file>`, **Then** bat output
   is shown (with syntax highlighting).
4. **Given** a new container with git history, **When** the user runs `git diff`,
   **Then** delta renders the diff with syntax highlighting.

---

### User Story 4 - Base Image Maintenance (Priority: P3)

A maintainer needs to update the base image when Fedora releases a new version or when
a tool needs updating. The Containerfile and build scripts are organized so that updates
are straightforward and the build process is reproducible.

**Why this priority**: Long-term maintenance is important but doesn't block initial use.

**Independent Test**: Modify the Fedora version in the Containerfile, rebuild, and verify
the image works correctly with the updated base.

**Acceptance Scenarios**:

1. **Given** the base image source files, **When** a maintainer changes the Fedora version
   ARG, **Then** the image rebuilds successfully with updated packages.
2. **Given** the CI pipeline, **When** triggered manually, **Then** multi-arch images are
   built and pushed to the registry with proper tags.

---

### Edge Cases

- What happens when a tool is unavailable in Fedora's repos for a specific architecture?
  Fallback to GitHub release binary with architecture detection.
- What happens when npm global installs fail due to permissions?
  The npm prefix is configured to `~/.local/lib/npm` to avoid root requirements.
- What happens when a user needs to install additional system packages at runtime?
  The `coder` user has passwordless sudo access.
- What happens when the Fedora base image has a critical CVE?
  Rebuild and push an updated image. Tags include dates for traceability.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The base image MUST be built from the latest stable Fedora release.
- **FR-002**: The base image MUST include Node.js (current stable from the OS
  package manager) and npm for Claude Code installation during user image build.
- **FR-003**: The base image MUST include Python 3 and the `uv` package manager
  for MCP server tooling and development.
- **FR-004**: The base image MUST include these version control tools: `git`, `gh`
  (GitHub CLI), `glab` (GitLab CLI).
- **FR-005**: The base image MUST include these search and file tools: `ripgrep` (rg),
  `fd-find` (fd), `fzf`, `jq`, `yq`, `less`, `tree`.
- **FR-006**: The base image MUST include these modern CLI replacements: `bat` (cat),
  `lsd` (ls), `delta` (diff), `zoxide` (cd).
- **FR-007**: The base image MUST include `starship` prompt with a default configuration
  showing git branch, directory, python venv, and kubernetes context.
- **FR-008**: The base image MUST include text editors: `helix` (hx), `vim`, `nano`.
- **FR-009**: The base image MUST include network and system tools: `curl`, `wget`,
  `htop`, `netcat` (nc), `dig`/`nslookup`, `ssh`/`scp`, `make`, `sudo`.
- **FR-010**: The base image MUST include `ca-certificates` for TLS trust.
- **FR-011**: The base image MUST create a non-root user named `coder` (UID 1000)
  with home directory at `/home/coder`.
- **FR-012**: The `coder` user MUST have passwordless sudo access.
- **FR-013**: The `coder` user MUST have proper XDG directory structure
  (`~/.config/`, `~/.local/`, `~/.cache/`).
- **FR-014**: The `coder` user's default shell MUST be zsh with starship prompt
  initialization, zoxide initialization, fzf integration, and standard aliases
  (`cat`→`bat`, `ls`→`lsd`, `ll`→`lsd -l`, `la`→`lsd -a`).
- **FR-015**: The `coder` user's npm global prefix MUST be set to `~/.local/lib/npm`
  so that `npm install -g` does not require root.
- **FR-016**: The base image MUST be published as a multi-arch manifest supporting
  both `amd64` and `arm64` architectures.
- **FR-017**: The base image MUST be published to `ghcr.io/rhuss/cc-deck-base` with
  tags: `latest`, version-based (`vX.Y.Z`), and Fedora-version-based (`fedora-NN`).
- **FR-018**: The base image MUST NOT include Zellij, Claude Code, or cc-deck.
  These are added during user image build to ensure version consistency.
- **FR-019**: The image Containerfile and build scripts MUST reside in a top-level
  `base-image/` directory in the cc-deck repository.
- **FR-020**: The git diff pager MUST be configured to use `delta` by default.
- **FR-021**: The build pipeline MUST run a vulnerability scan on the image and
  publish scan results alongside the image. Scan results are informational and
  MUST NOT block image publication.

### Key Entities

- **Base Image**: The published container image at `ghcr.io/rhuss/cc-deck-base`. Provides
  a Fedora-based developer toolbox with runtimes, CLI tools, and a configured zsh
  environment. Does not contain any cc-deck-specific components.
- **Coder User**: The non-root user (`coder`, UID 1000) that runs inside the container.
  Has sudo access, a configured zsh shell, and proper XDG directories.
- **Shell Environment**: The zsh configuration including starship prompt, zoxide, fzf
  integration, bat/lsd/delta aliases, and npm prefix configuration.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A developer can start a container from the base image and have all 20+
  listed tools available within 5 seconds of shell startup.
- **SC-002**: The base image works identically on amd64 and arm64 architectures with
  no tool failures or missing binaries on either platform.
- **SC-003**: The base image can be pulled on a typical broadband connection in
  under 2 minutes.
- **SC-004**: The base image size stays under 1.5 GB (compressed) to keep pull times
  reasonable.
- **SC-005**: The `coder` user can install npm global packages and Python packages
  without root access or permission errors.
- **SC-006**: The shell environment renders the starship prompt correctly on first
  container start with no additional setup required.

## Clarifications

### Session 2026-03-12

- Q: Should automated vulnerability scanning be part of the build pipeline, and should it block publication? → A: Scan runs and results are published, but does not block image publication.

## Assumptions

- Fedora's package repositories include all required tools for both amd64 and arm64.
  Where a tool is not available via dnf, GitHub release binaries are used as fallback.
- The `ghcr.io` registry is accessible to all target users without special authentication
  for public image pulls.
- The starship prompt configuration is shipped as a default that works for common
  workflows (git, Python, Kubernetes) without requiring user customization.
- zsh-autosuggestions and zsh-syntax-highlighting plugins are not included in the base
  image to minimize dependencies. Users who want them can install via their Zellij
  config or user image customization.
