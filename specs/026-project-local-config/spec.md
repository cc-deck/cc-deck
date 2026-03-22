# Feature Specification: Project-Local Environment Configuration

**Feature Branch**: `026-project-local-config`
**Created**: 2026-03-22
**Status**: Draft
**Input**: User description: "Project-local environment configuration with .cc-deck directory"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Clone and Create (Priority: P1)

A new team member clones a repository that already has a `.cc-deck/environment.yaml` committed. They run `cc-deck env create` without any flags and get a fully configured environment matching the team's setup.

**Why this priority**: This is the core value proposition. Declarative, shareable environment definitions eliminate manual flag passing and reduce onboarding friction from minutes of configuration to a single command.

**Independent Test**: Can be fully tested by cloning a repo with `.cc-deck/environment.yaml`, running `cc-deck env create`, and verifying the environment matches the definition.

**Acceptance Scenarios**:

1. **Given** a cloned repo with `.cc-deck/environment.yaml` specifying type, image, and domains, **When** the user runs `cc-deck env create` from the project root, **Then** the environment is created with all settings from the definition, and the output shows `Using environment "my-api" from /path/to/.cc-deck/`.
2. **Given** a cloned repo with `.cc-deck/environment.yaml`, **When** the user runs `cc-deck env create --image custom:latest`, **Then** the CLI flag overrides the definition's image for this instance only (stored in `status.yaml`, not persisted to `environment.yaml`), while using all other definition settings.
3. **Given** a cloned repo with `.cc-deck/environment.yaml`, **When** the user runs `cc-deck env create`, **Then** the project is auto-registered in the global registry so `cc-deck env list` shows it.

---

### User Story 2 - Initialize a Project (Priority: P1)

A developer sets up a new project with `cc-deck env init`, which scaffolds `.cc-deck/environment.yaml` and `.cc-deck/.gitignore`. The developer commits `.cc-deck/` to share the environment definition with the team.

**Why this priority**: Equal to P1 because project initialization is the prerequisite for the clone-and-create workflow. Without init, there is nothing to clone.

**Independent Test**: Can be fully tested by running `cc-deck env init` in a git repo and verifying the scaffolded files.

**Acceptance Scenarios**:

1. **Given** a git repository without `.cc-deck/`, **When** the user runs `cc-deck env init --type compose --image quay.io/cc-deck/cc-deck-demo:latest`, **Then** `.cc-deck/environment.yaml` is created with the specified type and image, and `.cc-deck/.gitignore` is created with `status.yaml` and `run/` entries.
2. **Given** a git repository that already has `.cc-deck/environment.yaml`, **When** the user runs `cc-deck env init`, **Then** the command fails with a clear error: "`.cc-deck/environment.yaml` already exists".
3. **Given** a directory that is not a git repository, **When** the user runs `cc-deck env init`, **Then** the command succeeds with a warning: "Not a git repository; `.cc-deck/.gitignore` will have no effect until git is initialized."

---

### User Story 3 - Implicit Name Resolution (Priority: P2)

A developer working inside a project directory runs `cc-deck env attach` (or other env commands) without specifying an environment name. The CLI walks up from cwd to find `.cc-deck/environment.yaml` at the git root and uses that environment.

**Why this priority**: This is an ergonomic enhancement that makes the common case (working inside a project) frictionless. P1 stories deliver value without this, but this story significantly improves daily workflow.

**Independent Test**: Can be tested by cd-ing into a subdirectory of a project with `.cc-deck/`, running commands without a name argument, and verifying the correct environment is resolved.

**Acceptance Scenarios**:

1. **Given** a project at `~/projects/my-api` with `.cc-deck/environment.yaml`, **When** the user runs `cc-deck env attach` from `~/projects/my-api/src/pkg/`, **Then** the CLI resolves the environment from `~/projects/my-api/.cc-deck/` and attaches.
2. **Given** a project with `.cc-deck/environment.yaml`, **When** the user runs any env command without a name, **Then** the output includes `Using environment "my-api" from ~/projects/my-api/.cc-deck/`.
3. **Given** a directory with no `.cc-deck/environment.yaml` in any parent up to the git root, **When** the user runs `cc-deck env attach` without a name, **Then** the command fails with: "No environment name specified and no `.cc-deck/environment.yaml` found in project hierarchy."

---

### User Story 4 - Global List Shows All Projects (Priority: P2)

A developer runs `cc-deck env list` and sees all environments across all registered projects, including their paths. Projects that have been moved or deleted show as MISSING.

**Why this priority**: Visibility across projects is essential for managing multiple environments but does not block creating or using individual environments.

**Independent Test**: Can be tested by registering multiple projects, renaming one, and verifying list output shows correct status.

**Acceptance Scenarios**:

1. **Given** three registered projects (two existing, one moved), **When** the user runs `cc-deck env list`, **Then** the output shows all three with NAME, TYPE, STATUS, and PATH columns, with the moved project showing `MISSING` status.
2. **Given** a project registered in the global registry, **When** the user runs `cc-deck env list --worktrees`, **Then** git worktrees within the project are shown as sub-entries with their branch names.

---

### User Story 5 - Variant for Worktree Isolation (Priority: P3)

A developer with multiple git worktrees on the host needs separate containers per worktree. They use `--variant` to create uniquely named containers from the same environment definition.

**Why this priority**: This is a power-user feature. Most users will use worktrees inside a single container (which needs no special support). Variants address the less common case of host-side worktree isolation.

**Independent Test**: Can be tested by creating two worktrees, running `cc-deck env create --variant <name>` in each, and verifying separate containers with unique names.

**Acceptance Scenarios**:

1. **Given** two worktrees of the same repo, **When** the user runs `cc-deck env create --variant auth` in the second worktree, **Then** a container named `cc-deck-my-api-auth` is created (distinct from `cc-deck-my-api` in the main worktree).
2. **Given** a project with variant `auth` stored in `status.yaml`, **When** the user runs `cc-deck env list`, **Then** the variant is shown as a separate column in the output.

---

### User Story 6 - Image Build Artifacts in .cc-deck/image/ (Priority: P3)

A developer uses `cc-deck image init` and `cc-deck image extract` to create image build artifacts. These artifacts are stored in `.cc-deck/image/` instead of the project root.

**Why this priority**: This is a structural improvement that reduces root directory clutter. The image build workflow already functions; this moves its artifacts into the `.cc-deck/` directory for organizational consistency.

**Independent Test**: Can be tested by running `cc-deck image init` and verifying artifacts appear in `.cc-deck/image/`.

**Acceptance Scenarios**:

1. **Given** a project with `.cc-deck/`, **When** the user runs `cc-deck image init`, **Then** `cc-deck-build.yaml` is created at `.cc-deck/image/cc-deck-build.yaml`.
2. **Given** a project with `.cc-deck/image/cc-deck-build.yaml`, **When** the user runs `cc-deck image extract`, **Then** extracted settings are written to `.cc-deck/image/` (e.g., `.cc-deck/image/settings.json`).

---

### User Story 7 - Prune Stale Entries (Priority: P3)

A developer who has moved or deleted project directories runs `cc-deck env prune` to clean up stale entries from the global registry.

**Why this priority**: Housekeeping feature. Does not block any core workflow but prevents registry clutter over time.

**Independent Test**: Can be tested by registering a project, removing its directory, and running prune.

**Acceptance Scenarios**:

1. **Given** a global registry with two entries (one existing, one moved), **When** the user runs `cc-deck env prune`, **Then** the stale entry is removed and the user sees: `Removed 1 stale project(s)`.

---

### Edge Cases

- What happens when a user runs `cc-deck env create` in a project that has both a project-local `.cc-deck/environment.yaml` AND a global definition in `environments.yaml` with the same name? Project-local takes precedence; the global definition is ignored with a warning (FR-026).
- What happens when the `.cc-deck/.gitignore` is accidentally deleted? Regenerated on the next environment operation (FR-030). The `run/` and `status.yaml` entries are idempotently ensured.
- What happens when two users on different machines create environments from the same cloned `.cc-deck/environment.yaml`? Each gets an independent container. No conflict because `status.yaml` is local and gitignored.
- What happens when a project directory is a symlink? The canonical (symlink-resolved) path is stored in the global registry to prevent duplicate entries.
- What happens when `cc-deck env init` is run outside a git repository? It succeeds with a warning. The `.cc-deck/.gitignore` has no effect but is still created for when git is initialized later.
- What happens when `environment.yaml` changes after the environment was created (new domain, updated image)? The user runs `cc-deck env delete` followed by `cc-deck env create` to apply changes. A dedicated `env recreate` command is out of scope for this feature.
- What happens when a project "my-api" with variant "auth" and a separate project "my-api-auth" both produce containers named `cc-deck-my-api-auth`? The second `env create` fails with an `ErrNameConflict` because the container name already exists. The user must choose a different variant name or project name to resolve the collision.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST support a `.cc-deck/` directory at the git root that contains committed environment definitions and gitignored runtime artifacts.
- **FR-002**: The system MUST provide a `cc-deck env init` command that scaffolds `.cc-deck/environment.yaml` and `.cc-deck/.gitignore` from CLI flags.
- **FR-003**: The system MUST resolve environment configuration with this precedence: CLI flags > project-local definition > global config defaults > hardcoded defaults.
- **FR-004**: The system MUST walk from cwd upward to the git root to find `.cc-deck/environment.yaml` when no environment name is provided.
- **FR-005**: The system MUST stop the upward walk at the git boundary (the directory containing `.git/` or a `.git` file for worktrees).
- **FR-006**: The system MUST maintain a global project registry (in `$XDG_STATE_HOME/cc-deck/state.yaml`) storing path references to project directories.
- **FR-007**: The system MUST auto-register projects in the global registry both on explicit `cc-deck env create` and on walk-based discovery.
- **FR-008**: The system MUST show `MISSING` status for registry entries whose paths no longer exist, without auto-removing them.
- **FR-009**: The system MUST provide a `cc-deck env prune` command that removes stale registry entries.
- **FR-010**: The system MUST support a `--variant` flag on `cc-deck env create` that stores a variant identifier in `status.yaml` and appends it to the container name.
- **FR-011**: The system MUST display the variant column in `cc-deck env list` output when variants are present.
- **FR-012**: The system MUST display the project path in `cc-deck env list` output for project-local environments.
- **FR-013**: The system MUST auto-detect the environment type from `environment.yaml` when `--type` is omitted and a definition exists.
- **FR-014**: The system MUST store generated runtime artifacts (compose.yaml, .env, proxy configs) in `.cc-deck/run/` instead of directly in `.cc-deck/`.
- **FR-015**: The system MUST store runtime state in `.cc-deck/status.yaml` for project-local environments.
- **FR-016**: The system MUST create a `.cc-deck/.gitignore` containing `status.yaml` and `run/` when scaffolding the directory.
- **FR-017**: The system MUST store image build artifacts (`cc-deck-build.yaml`, `Containerfile`, extracted settings) in `.cc-deck/image/`.
- **FR-018**: The system MUST always display which environment definition is being used when resolving via the walk (e.g., `Using environment "my-api" from /path/.cc-deck/`).
- **FR-019**: When an existing `.cc-deck/environment.yaml` is found during `cc-deck env create`, the system MUST use it as the source of truth and provision runtime resources without rewriting the definition. CLI flag overrides are runtime-only and stored in `status.yaml`, not persisted to `environment.yaml`.
- **FR-020**: The system MUST support a `--worktrees` flag on `cc-deck env list` that shows git worktrees within each project (discovered via `git worktree list`).
- **FR-021**: The system MUST store canonical (symlink-resolved) paths in the global registry to prevent duplicate entries from symlinks.
- **FR-022**: The system MUST support `cc-deck env attach --branch <name>` to attach and land in a specific worktree directory inside the container. If no worktree matches the given branch name, the command MUST fail with a clear error listing the available worktrees.
- **FR-023**: All environment operations (Create, Attach, Delete, Status, Start, Stop) MUST satisfy the behavioral requirements defined in `specs/023-env-interface/contracts/environment-interface.md`, including nested Zellij detection, session creation with layout, auto-start on attach, timestamp updates, name validation, cleanup on failure, and state reconciliation.
- **FR-024**: The `.cc-deck/.gitignore` file is an explicit exception to Principle XIV (No Dotfile Nesting) because `.gitignore` is a git-recognized filename that requires the dot prefix to function. No other files inside `.cc-deck/` may use a dot prefix.
- **FR-025**: When `cc-deck env create` is run in a git repository with no `.cc-deck/environment.yaml`, the system MUST scaffold `.cc-deck/environment.yaml` and `.cc-deck/.gitignore` from CLI flags before provisioning, equivalent to an implicit `cc-deck env init`. When run outside a git repository with no definition, the system MUST require an explicit environment name.
- **FR-026**: When both a project-local `.cc-deck/environment.yaml` and a global definition in `environments.yaml` exist with the same name, the system MUST use the project-local definition and emit a warning identifying the shadowed global definition.
- **FR-027**: When deleting a project-local environment, the system MUST remove `.cc-deck/status.yaml` and `.cc-deck/run/`, MUST NOT remove `.cc-deck/environment.yaml` or `.cc-deck/image/`, and MUST deregister the project from the global registry. The committed definition remains intact for future `env create`.
- **FR-028**: The environment definition (`environment.yaml`) MUST support an `env` section for declaring arbitrary environment variables to pass into the session container. These are additive with any variables set by the system (auth, credentials).
- **FR-029**: The `environment.yaml` file MUST include a `version` field (starting at `1`) to support future schema evolution.
- **FR-030**: When `.cc-deck/.gitignore` is missing or incomplete during any environment operation (`create`, `start`, `attach`), the system MUST idempotently regenerate it with `status.yaml` and `run/` entries.

### Key Entities

- **Project Registry Entry**: A reference to a project directory in the global state file, with path and last-seen timestamp.
- **Environment Definition** (`environment.yaml`): Declarative, user-editable description of an environment's desired state (type, image, domains, credentials, env vars, storage).
- **Runtime Status** (`status.yaml`): Per-project, gitignored state tracking container name, variant, lifecycle state, and timestamps.
- **Generated Artifacts** (`run/`): Compose files, .env, proxy configs that are fully regenerable from the environment definition.
- **Image Build Artifacts** (`image/`): Build manifest, Containerfile, and extracted settings committed alongside the project.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A new team member can go from `git clone` to a running environment in two commands (`cc-deck env create` + `cc-deck env attach`) without specifying any flags.
- **SC-002**: All environment commands (attach, status, start, stop, delete) work without specifying an environment name when run from within a project directory.
- **SC-003**: `cc-deck env list` shows all registered project environments with their paths, types, status, and variants in a single view.
- **SC-004**: Moved or deleted project directories are visibly marked as MISSING in list output without data loss or crashes.
- **SC-005**: The `.cc-deck/` directory structure clearly separates committed artifacts (definition, image build files) from gitignored runtime state (status, generated files), requiring zero manual gitignore configuration.
- **SC-006**: Environment definitions survive git operations (branch, merge, rebase, worktree add) without requiring re-creation or manual intervention.
- **SC-007**: Image build artifacts (`cc-deck-build.yaml`, `Containerfile`, extracted settings) are stored in `.cc-deck/image/` and referenced by image commands without additional flags.
- **SC-008**: The variant mechanism allows multiple isolated container instances from the same environment definition, each with a unique container name.
