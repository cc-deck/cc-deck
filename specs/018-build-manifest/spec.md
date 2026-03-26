# Feature Specification: cc-deck Build Pipeline

**Feature Branch**: `018-build-manifest`
**Created**: 2026-03-12
**Status**: Evolved (2026-03-26)
**Input**: User description: "cc-deck build manifest schema and CLI commands for container image creation, combined with AI-driven build commands"

> **Evolution Note (2026-03-26)**: Updated to reflect implemented naming:
> manifest renamed from `cc-deck-build.yaml` to `cc-deck-image.yaml`,
> CLI commands moved under `cc-deck image` subcommand,
> AI commands renamed (`/cc-deck.capture`, `/cc-deck.build`, `/cc-deck.push`),
> `/cc-deck.plugin` and `/cc-deck.mcp` commands descoped (never implemented),
> base image registry updated to `quay.io/cc-deck/cc-deck-base`.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Initialize a Build Directory (Priority: P1)

A developer wants to create a containerized Claude Code environment for their projects.
They run `cc-deck build init my-cc-image` which scaffolds a build directory containing
the manifest file, AI-driven Claude Code commands, helper scripts, and a gitignore. The
developer can then use the embedded Claude Code commands to populate the manifest and
generate a Containerfile.

> **Note**: CLI command is `cc-deck image init`, not `cc-deck build init`.

**Why this priority**: Without initialization, no build directory exists. This is the
entry point for the entire build pipeline.

**Independent Test**: Run `cc-deck build init`, verify the directory structure is created
with all expected files, and confirm the manifest contains valid YAML with example entries.

**Acceptance Scenarios**:

1. **Given** an empty directory path, **When** the user runs `cc-deck image init my-image`,
   **Then** a directory is created with `cc-deck-image.yaml`, `.claude/commands/`, and `.gitignore`.
2. **Given** an initialized build directory, **When** the user opens it in Claude Code,
   **Then** the commands `cc-deck.capture`, `cc-deck.build`, and `cc-deck.push`
   are available as slash commands.
3. **Given** the scaffolded manifest, **When** the user opens `cc-deck-image.yaml`,
   **Then** it contains commented-out examples for each section (image, tools, sources,
   plugins, mcp, github_tools, settings).
4. **Given** a directory that already contains a `cc-deck-image.yaml`, **When** the user
   runs `cc-deck image init`, **Then** the command refuses to overwrite and reports an error.

---

### User Story 2 - Analyze Repositories for Tool Dependencies (Priority: P1)

A developer has several locally checked-out repositories and wants the AI to examine them
to discover required build tools, compilers, and runtime dependencies. They use the
`/cc-deck.capture` command in Claude Code, which analyzes build files, CI configs, and
tool version files, then updates the manifest with discovered tools.

**Why this priority**: Tool discovery is the primary use case for the AI-driven workflow.
Without it, users must manually list every tool, which is error-prone and tedious.

**Independent Test**: Run `/cc-deck.extract` on a repository with known dependencies (e.g.,
a Go project with `go.mod`), verify the manifest is updated with the correct tools and
source provenance.

**Acceptance Scenarios**:

1. **Given** a Go project at `/path/to/repo`, **When** the user runs `/cc-deck.capture`
   and provides the path, **Then** the AI detects Go version from `go.mod` and adds it to
   the tools section.
2. **Given** two repositories with overlapping dependencies, **When** both are analyzed,
   **Then** the tools list is deduplicated and version conflicts are resolved (highest
   compatible version).
3. **Given** an analyzed repository, **When** the user runs `/cc-deck.capture` on the
   same repo again, **Then** existing entries are updated (not duplicated) and changes
   are highlighted.
4. **Given** discovered tools, **When** the AI presents findings, **Then** the user can
   accept, reject, or modify individual entries before they are written to the manifest.

---

### User Story 3 - Generate a Containerfile from the Manifest (Priority: P1)

After populating the manifest with tools, plugins, and configuration, the developer uses
`/cc-deck.build` to generate a complete Containerfile. The AI resolves free-form
tool descriptions (like "Go compiler >= 1.22") to concrete install commands, optimizes
layer ordering, and includes cc-deck self-embedding. The Containerfile is always
regenerated from scratch (no manual edits).

**Why this priority**: The Containerfile is the build artifact that turns the manifest into
a usable container image. Without it, nothing can be built.

**Independent Test**: Populate a manifest with known tools, run `/cc-deck.build`,
verify the generated Containerfile is syntactically valid and includes all specified tools.

**Acceptance Scenarios**:

1. **Given** a manifest with tools listed as free-form text, **When** the user runs
   `/cc-deck.build`, **Then** a valid Containerfile is generated with concrete
   install commands for each tool.
2. **Given** a generated Containerfile, **Then** it includes a header comment stating
   it should not be manually edited and how to regenerate.
3. **Given** a manifest with the `zellij_config` setting set to `current`, **When** the
   Containerfile is generated, **Then** it includes instructions to copy the user's local
   Zellij configuration into the image.
4. **Given** a manifest with `github_tools` entries, **When** the Containerfile is generated,
   **Then** it includes multi-arch download instructions for each tool.

---

### User Story 4 - Build and Push the Container Image (Priority: P2)

After generating the Containerfile, the developer runs `cc-deck image build` to build the
container image and `cc-deck image push` to publish it to a registry. These are deterministic
CLI operations that don't require AI.

**Why this priority**: Building and pushing are the final steps. They depend on the
Containerfile existing (from Story 3).

**Independent Test**: Given a valid Containerfile and manifest, run `cc-deck build` and
verify a container image is produced with the correct name and tag.

**Acceptance Scenarios**:

1. **Given** a valid Containerfile and manifest, **When** the user runs `cc-deck image build <dir>`,
   **Then** the container image is built with the name and tag from the manifest.
2. **Given** a built image, **When** the user runs `cc-deck image push <dir>`, **Then** the image
   is pushed to the registry specified in the manifest.
3. **Given** no Containerfile in the build directory, **When** the user runs `cc-deck image build`,
   **Then** the command fails with a clear error message suggesting to run
   `/cc-deck.build` first.
4. **Given** the cc-deck binary, **When** `cc-deck image build` runs, **Then** the binary copies
   itself into the build context and the Containerfile installs it into the image via
   `cc-deck plugin install`.

---

### ~~User Story 5 - Add Plugins and MCP Servers (Priority: P2)~~ DESCOPED

> **Descoped (2026-03-26)**: `/cc-deck.plugin` and `/cc-deck.mcp` commands were never
> implemented. Plugin and MCP configuration is done manually in the manifest YAML.
> May be revisited in a future spec.

---

### User Story 6 - Verify and Compare Builds (Priority: P3)

A developer wants to verify that a built image works correctly or see what changed since
the last build. `cc-deck build verify` runs a smoke test, and `cc-deck build diff` shows
changes in tools, plugins, or MCP configuration.

**Why this priority**: Verification and diffing are quality-of-life features that improve
confidence but aren't required for basic operation.

**Independent Test**: Build an image, run `cc-deck build verify`, verify it reports tool
availability and Claude Code startup.

**Acceptance Scenarios**:

1. **Given** a built image, **When** the user runs `cc-deck build verify <dir>`, **Then**
   the command starts a container, checks tool availability, Claude Code startup, and
   reports pass/fail.
2. **Given** manifest changes since the last build, **When** the user runs
   `cc-deck build diff <dir>`, **Then** the command shows added/removed/changed tools,
   plugins, and MCP entries.

---

### Edge Cases

- What happens when a specified repository path doesn't exist during `/cc-deck.capture`?
  The AI reports the invalid path and asks for correction.
- What happens when the manifest has an invalid YAML syntax?
  `cc-deck image build` validates the manifest and reports parsing errors with line numbers.
- What happens when the container runtime (podman) is not installed?
  `cc-deck image build` detects the missing runtime and reports an error.
- What happens when a GitHub tool release doesn't have binaries for the target architecture?
  The Containerfile generation flags the issue and suggests alternatives.
- What happens when the user runs `/cc-deck.build` with an empty tools list?
  The AI generates a minimal Containerfile with only the base image, cc-deck, and Claude Code.

## Requirements *(mandatory)*

### Functional Requirements

#### Manifest & Schema

- **FR-001**: The manifest file MUST be named `cc-deck-image.yaml` and reside at the root
  of the build directory.
- **FR-002**: The manifest MUST support these top-level sections: `image` (name, tag, base),
  `tools` (free-form text list), `sources` (analyzed repository provenance), `plugins`
  (Claude Code plugins), `mcp` (MCP server sidecars), `github_tools` (GitHub release
  downloads), and `settings` (Claude Code configuration to bake into the image).
- **FR-003**: The `tools` section MUST accept human-readable, free-form text entries
  (e.g., "Go compiler >= 1.22") that are resolved to install commands during Containerfile
  generation.
- **FR-004**: The `sources` section MUST track which repositories were analyzed, including
  the detected tools, the files they were detected from, and the local checkout path.
- **FR-005**: The `settings.zellij_config` field MUST support three modes: `current` (copy
  from local `~/.config/zellij/`), `vanilla` (cc-deck defaults only), or a custom path.
- **FR-006**: The manifest MUST NOT contain any credentials or secret values. Auth sections
  describe required environment variable names only.

#### CLI Commands

- **FR-007**: `cc-deck image init <dir>` MUST create the build directory with: the manifest
  file (with commented examples), `.claude/commands/` with all AI-driven commands,
  `.claude/scripts/` with helper scripts, and a `.gitignore`.
- **FR-008**: `cc-deck image init` MUST refuse to overwrite an existing build directory
  that already contains a `cc-deck-image.yaml`.
- **FR-009**: All embedded commands and scripts MUST be stored inside the cc-deck binary
  and extracted at init time.
- **FR-010**: `cc-deck image build <dir>` MUST validate the manifest schema, verify the
  Containerfile exists, copy the cc-deck binary into the build context, and invoke
  podman to build the image.
- ~~**FR-011**: `cc-deck build` MUST auto-detect the available container runtime
  (podman or docker) and use it transparently.~~ Uses podman exclusively.
- **FR-012**: `cc-deck image push <dir>` MUST read the image name and tag from the manifest
  and push to the configured registry.
- **FR-013**: `cc-deck image verify <dir>` MUST build the image, start a container, verify
  that tools are available and Claude Code starts, then report pass/fail.
- **FR-014**: `cc-deck image diff <dir>` MUST compare the current manifest state against
  the last generated Containerfile and report added/removed/changed items.

#### AI-Driven Commands

- **FR-015**: `/cc-deck.capture` MUST analyze locally checked-out repositories by examining
  build files, CI configurations, and tool version files to discover required tools.
- **FR-016**: `/cc-deck.capture` MUST present discovered tools to the user for review before
  writing to the manifest. Users can accept, reject, or modify individual entries.
- **FR-017**: `/cc-deck.capture` MUST deduplicate tools across multiple repositories and
  resolve version conflicts by suggesting the highest compatible version.
- **FR-018**: `/cc-deck.build` MUST generate a complete, valid Containerfile from
  the manifest, resolving free-form tool descriptions to concrete install commands.
- **FR-019**: The generated Containerfile MUST include a "DO NOT EDIT" header and MUST
  always be regenerated from scratch (never patched).
- **FR-020**: `/cc-deck.build` MUST handle cc-deck self-embedding: the Containerfile
  copies the cc-deck binary from the build context and runs `cc-deck plugin install` to
  install the WASM plugin, layouts, and matching Zellij binary.
- ~~**FR-021**: `/cc-deck.plugin`~~ DESCOPED: Plugin management command not implemented.
- ~~**FR-022**: `/cc-deck.mcp`~~ DESCOPED: MCP server auto-detection command not implemented.
- **FR-023**: `/cc-deck.push` provides push functionality as a standalone command.

### Key Entities

- **Build Directory**: A self-contained directory created by `cc-deck build init` containing
  the manifest, Containerfile, AI commands, and helper scripts.
- **Manifest** (`cc-deck-image.yaml`): The YAML file describing the desired container image
  contents. Human-readable, declarative, and credential-free.
- **Containerfile**: The generated build instructions derived from the manifest by the AI.
  Never manually edited. Always regenerated from scratch.
- **Source Provenance**: Records of which repositories were analyzed and what tools were
  discovered from which files. Enables re-extraction and change detection.
- **MCP Label Schema**: Container image labels prefixed with `cc-deck.mcp/` that describe
  an MCP server's capabilities (transport, port, auth, description).

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A developer can go from zero to a built container image in under 30 minutes
  (init + extract 2-3 repos + generate containerfile + build).
- **SC-002**: The AI correctly identifies 90%+ of tool dependencies from standard build
  files (go.mod, package.json, Cargo.toml, pyproject.toml) without manual intervention.
- **SC-003**: Re-running `/cc-deck.extract` on already-analyzed repos completes in under
  2 minutes and correctly identifies changes.
- **SC-004**: The generated Containerfile builds successfully on first attempt for standard
  tool combinations (Go, Python, Node.js, Rust).
- **SC-005**: `cc-deck build init` completes in under 5 seconds and produces a valid,
  ready-to-use build directory.
- **SC-006**: The cc-deck binary self-embeds correctly in the built image, and
  `cc-deck plugin install` runs successfully inside the container.

## Assumptions

- The user has podman or docker installed and accessible from the command line.
- The user has Claude Code installed locally for the AI-driven Phase 2 commands.
- Repositories to be analyzed are locally checked out (not cloned during extraction).
- The cc-deck base image (`quay.io/cc-deck/cc-deck-base`) is available and accessible.
- Plugin marketplace APIs are accessible when using marketplace-based plugin discovery.
- MCP server images with `cc-deck.mcp/*` labels follow the documented label schema.
- The `go:embed` pattern used for WASM and layout files is extended to embed commands and scripts.
