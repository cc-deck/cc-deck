# Brainstorm: Project-Local Environment Configuration

**Date:** 2026-03-21 (updated 2026-03-22)
**Status:** Brainstorm (consolidated)
**Depends on:** 023-env-interface, 024-container-env, 025-compose-env
**Discovered during:** Compose environment brainstorm

## Problem

Environment configuration is global: stored in `$XDG_CONFIG_HOME/cc-deck/environments.yaml`. This creates friction:

1. A new team member must manually run `cc-deck env create` with the right flags to match the team's setup.
2. Environment config is not version-controlled alongside the project.
3. There is no way to express "this project should use this image with these domain allowlists" declaratively.
4. Image build artifacts (`cc-deck-build.yaml`, Containerfile, extracted settings) live at the project root, separate from environment config.
5. Generated runtime artifacts (compose.yaml, .env) live in `.cc-deck/` with no clear separation from definition files.

## Core Design

Each project gets a `.cc-deck/` directory at the git root (same level as `.git/`) that contains both committed definitions and gitignored runtime artifacts. The global state file stores only a path reference, so `cc-deck env list` can discover all projects.

### Directory Layout

```
my-project/
  .cc-deck/
    environment.yaml          # Environment definition (COMMITTED)
    image/                     # Image build artifacts (COMMITTED)
      cc-deck-build.yaml       #   Build manifest
      Containerfile            #   Build recipe
      settings.json            #   Extracted Claude Code settings
      mcp-configs/             #   MCP server configurations
    .gitignore                 # Ignores: status.yaml, run/
    status.yaml                # Runtime state (GITIGNORED)
    run/                       # Generated runtime artifacts (GITIGNORED)
      compose.yaml             #   Generated compose file
      .env                     #   Generated credentials
      proxy/                   #   Generated proxy config (if filtering)
        tinyproxy.conf
        whitelist
  .git/
  src/
  go.mod
```

### Gitignore Boundary

`.cc-deck/.gitignore` contains:

```
status.yaml
run/
```

Everything else in `.cc-deck/` is committed. The `.gitignore` is created by `cc-deck env init` or `cc-deck env create`.

### Separation Principle

| Directory/File | Purpose | Committed | Who writes it |
|----------------|---------|-----------|---------------|
| `environment.yaml` | Declarative env definition | Yes | User via `env init` or `env create` |
| `image/` | Image build inputs | Yes | User via `cc-deck image init/extract` |
| `.gitignore` | Ignore boundary | Yes | `cc-deck env init/create` |
| `status.yaml` | Runtime state (container IDs, timestamps) | No | `cc-deck env create/start/stop` |
| `run/` | Generated compose, .env, proxy configs | No | `cc-deck env create` (regenerated) |

This follows established patterns: Terraform (`main.tf` vs `.terraform/`), Rust (`Cargo.toml` vs `target/`), Node.js (`package.json` vs `node_modules/`).

## Design Decisions

### D1: Single Environment Per Project

One project = one environment definition. The env name defaults to the directory basename. No need for named profiles or multi-env configs within a single project.

```bash
cd my-api
cc-deck env create --type compose    # name defaults to "my-api"
cc-deck env attach                    # finds .cc-deck/, uses the env
```

The user can still override the name: `cc-deck env create my-custom-name --type compose`.

Rationale: simplicity. Multiple environments per project can be added later if needed. The variant mechanism (D7) handles the common case of needing multiple instances of the same definition.

### D2: Git Boundary Walk

When an env name is omitted, cc-deck walks from cwd upward looking for `.cc-deck/environment.yaml`. The walk stops at the git root (the directory containing `.git/` or `.git` file for worktrees). If no `.git` is found, stops at filesystem root.

This matches git's own config resolution behavior.

`.cc-deck/` MUST live at the git root, never in subdirectories. A user running `cc-deck env attach` from `src/pkg/` walks up and finds `.cc-deck/` at the repo root.

Output always shows what was resolved:

```
Using environment "my-api" from /Users/rhuss/projects/my-api/.cc-deck/
```

### D3: Config Resolution Order

When creating or provisioning an environment, options resolve with this precedence:

1. **CLI flags** (highest priority, explicit user choice)
2. **Project config** (`.cc-deck/environment.yaml` found via walk)
3. **Global config** (`$XDG_CONFIG_HOME/cc-deck/config.yaml` defaults section)
4. **Hardcoded defaults** (lowest priority)

### D4: Global Project Registry

The global state file stores path references to project directories:

```yaml
# ~/.local/state/cc-deck/state.yaml
version: 2
instances: [...]     # existing v2 instances (unchanged)
projects:            # NEW section
  - path: /Users/rhuss/projects/my-api
    last_seen: 2026-03-22T10:00:00Z
  - path: /Users/rhuss/projects/other-project
    last_seen: 2026-03-20T08:00:00Z
```

Registration happens on:
- `cc-deck env create` (explicit action)
- Any command that discovers `.cc-deck/` via the walk (auto-registration)

This makes the team-sharing scenario seamless: `git clone` + `cc-deck env create` in a project with existing `.cc-deck/environment.yaml` just works.

### D5: Stale Path Handling

When `cc-deck env list` reads the registry:
- Stat each project path
- If the path is gone, show as `MISSING` in the output (do not auto-remove, the user might have an unmounted drive or renamed directory)

A `cc-deck env prune` command cleans up stale entries interactively:

```bash
cc-deck env prune
# "my-old-project" at /Users/rhuss/projects/my-old-project: MISSING. Remove? [Y/n]
```

Canonical (symlink-resolved) paths are stored to avoid duplicates from symlinks.

### D6: environment.yaml Schema

```yaml
# .cc-deck/environment.yaml
version: 1
name: my-api
type: compose
image: quay.io/cc-deck/cc-deck-demo:latest
auth: auto

# Network filtering (compose only).
# Domain groups are resolved via ~/.config/cc-deck/domains.yaml.
allowed-domains:
  - anthropic
  - github
  - npm

# Port mappings (host:container).
ports:
  - "8082:8082"

# Storage type (host-path is default for compose, named-volume for container).
storage:
  type: host-path

# Additional bind mounts as src:dst[:ro].
mounts:
  - "~/.ssh:/home/dev/.ssh:ro"

# Credentials resolved from host environment at create time.
# Only variable names are stored, never values.
credentials:
  - ANTHROPIC_API_KEY
  - GOOGLE_APPLICATION_CREDENTIALS

# Environment variables for the session container.
# Add your own variables here.
env:
  EDITOR: helix
  MY_CUSTOM_VAR: some-value
```

Users edit only `environment.yaml`. The generated `run/.env` is always recreated from this definition. It contains a header comment:

```
# AUTO-GENERATED from environment.yaml - do not edit
# Edit .cc-deck/environment.yaml instead
```

### D7: Variant Mechanism for Multiple Instances

When the same project needs multiple containers (e.g., per-worktree containers), the `--variant` flag creates a unique instance:

```bash
# Main worktree
cd ~/projects/my-api
cc-deck env create --type compose
# Container: cc-deck-my-api

# Feature worktree (separate container)
cd ~/projects/my-api-auth
cc-deck env create --type compose --variant auth
# Container: cc-deck-my-api-auth
```

The variant is stored in `status.yaml` (per-worktree, not committed):

```yaml
# .cc-deck/status.yaml
variant: auth
state: running
container_name: cc-deck-my-api-auth
created_at: 2026-03-22T10:00:00Z
```

Container naming: `cc-deck-<name>` (no variant) or `cc-deck-<name>-<variant>` (with variant).

### D8: Type Auto-Detection

When `environment.yaml` exists and specifies `type:`, running `cc-deck env create` without `--type` uses the definition's type. CLI `--type` overrides. This means most users just run `cc-deck env create` after cloning a project.

### D9: Init Command

`cc-deck env init` scaffolds `.cc-deck/` with an `environment.yaml`:

```bash
cd my-project
cc-deck env init
# Interactive: asks for type, image, domains
# Creates .cc-deck/environment.yaml, .cc-deck/.gitignore
# "Created .cc-deck/environment.yaml. Commit this directory to share with your team."
```

Flags for non-interactive use:

```bash
cc-deck env init --type compose --image quay.io/cc-deck/cc-deck-demo:latest
```

`cc-deck env init` ONLY creates the definition. It does not provision any containers. `cc-deck env create` does both (writes definition if missing, then provisions).

### D10: Build Manifest Relocation

`cc-deck-build.yaml`, `Containerfile`, and extracted settings move from the project root into `.cc-deck/image/`. The `cc-deck image` commands look in `.cc-deck/image/` for their files.

Since there has been no public release, no migration is needed. Existing local setups are cleaned up manually.

## Worktree Workflows

### Workflow A: Multiple Worktrees Inside One Container

One container with the project bind-mounted. Worktrees created inside the container for parallel feature work.

```bash
# On host
cd ~/projects/my-api
cc-deck env create --type compose
cc-deck env attach

# Inside the container (in /workspace = ~/projects/my-api)
git worktree add .worktrees/feature-auth origin/feature-auth
git worktree add .worktrees/bugfix-123 origin/bugfix-123

# Each worktree gets its own cc-deck tab
# Tab 1: /workspace (main)
# Tab 2: /workspace/.worktrees/feature-auth
# Tab 3: /workspace/.worktrees/bugfix-123
```

This needs no special cc-deck support. The worktrees are inside the bind-mounted project directory. Add `.worktrees/` to the project's `.gitignore`. The cc-deck sidebar naturally shows each tab.

Worktrees are discoverable via `git worktree list` on the project path.

### Workflow B: Separate Container Per Worktree

Each host-side worktree gets its own container. Useful for isolated containers per feature, or host-IDE integration.

```bash
# Create worktrees on host
cd ~/projects/my-api
git worktree add ~/projects/my-api-auth feature-auth

# Main worktree
cd ~/projects/my-api
cc-deck env create --type compose
# Container: cc-deck-my-api

# Feature worktree (separate container)
cd ~/projects/my-api-auth
cc-deck env create --type compose --variant auth
# Container: cc-deck-my-api-auth
```

Each worktree has its own `.cc-deck/status.yaml` and `.cc-deck/run/`. The committed `environment.yaml` is the same (inherited from the branch).

### Worktree Display in `cc-deck env list`

Normal view:

```
NAME       TYPE     STATUS    PATH                     BRANCHES
my-api     compose  running   ~/projects/my-api        main, feature-auth, bugfix-123
other      compose  stopped   ~/projects/other         main
```

Expanded view (`cc-deck env list --worktrees` or `-w`):

```
NAME       TYPE     STATUS    PATH                                        BRANCH
my-api     compose  running   ~/projects/my-api
  main                        ~/projects/my-api                           main
  worktree                    ~/projects/my-api/.worktrees/feature-auth   feature-auth
  worktree                    ~/projects/my-api/.worktrees/bugfix-123     bugfix-123
other      compose  stopped   ~/projects/other                            main
```

Variant view (Scenario B):

```
NAME       VARIANT   TYPE     STATUS    PATH                          BRANCH
my-api     -         compose  running   ~/projects/my-api             main
my-api     auth      compose  running   ~/projects/my-api-auth        feature-auth
```

Worktree data comes from `git worktree list --porcelain`, not from the registry. No additional registry entries are needed for in-container worktrees.

Targeted attach to a specific worktree:

```bash
cc-deck env attach --branch feature-auth
# Attaches and cds into the worktree directory inside the container
```

## Usage Scenarios

### Scenario 1: New Team Member Joins

```bash
git clone git@github.com:org/my-api.git
cd my-api
cc-deck env create
# Reads .cc-deck/environment.yaml (type: compose, image, domains, etc.)
# Provisions container, generates run/ artifacts
# "Created compose environment 'my-api' from .cc-deck/environment.yaml"
cc-deck env attach
```

### Scenario 2: Solo Developer Sets Up Project

```bash
cd my-project
cc-deck env init --type compose --image quay.io/cc-deck/cc-deck-demo:latest
# Creates .cc-deck/environment.yaml and .cc-deck/.gitignore
git add .cc-deck/
git commit -m "Add cc-deck environment config"
cc-deck env create
cc-deck env attach
```

### Scenario 3: Override Project Defaults

```bash
cd my-project
cc-deck env create --image my-custom-image:latest
# Uses environment.yaml for everything except image
```

### Scenario 4: Image Build Workflow

```bash
cd my-project
cc-deck image init          # Creates .cc-deck/image/cc-deck-build.yaml
cc-deck image extract       # Extracts settings into .cc-deck/image/
cc-deck image build         # Builds from .cc-deck/image/Containerfile
```

## Relationship to Existing Files

### Before (current state)

| File | Location | Purpose |
|------|----------|---------|
| `environments.yaml` | `$XDG_CONFIG_HOME/cc-deck/` | All environment definitions |
| `state.yaml` | `$XDG_STATE_HOME/cc-deck/` | Runtime instance state |
| `config.yaml` | `$XDG_CONFIG_HOME/cc-deck/` | Global user preferences |
| `cc-deck-build.yaml` | Project root | Image build manifest |
| `Containerfile` | Project root | Image build recipe |
| `.cc-deck/` | Project root | Generated compose/proxy artifacts |

### After (proposed)

| File | Location | Purpose | Committed |
|------|----------|---------|-----------|
| `.cc-deck/environment.yaml` | Project git root | Environment definition | Yes |
| `.cc-deck/image/*` | Project git root | Image build artifacts | Yes |
| `.cc-deck/.gitignore` | Project git root | Ignore boundary | Yes |
| `.cc-deck/status.yaml` | Project git root | Runtime state | No |
| `.cc-deck/run/*` | Project git root | Generated artifacts | No |
| `state.yaml` (projects section) | `$XDG_STATE_HOME/cc-deck/` | Global project registry | No |
| `config.yaml` | `$XDG_CONFIG_HOME/cc-deck/` | Global preferences (unchanged) | No |
| `environments.yaml` | `$XDG_CONFIG_HOME/cc-deck/` | Legacy global definitions (phased out) | No |

Global `environments.yaml` remains for non-project environments (e.g., standalone containers without a git repo). Project-local definitions take precedence when both exist.

## Edge Cases

### Renamed/Moved Project Directories

The global registry stores canonical (symlink-resolved) absolute paths. When a project is moved:
- `cc-deck env list` shows the entry as `MISSING`
- `cc-deck env prune` offers to remove stale entries
- Re-running `cc-deck env create` (or any walk-discovering command) in the new location auto-registers it

### Multiple Checkouts of the Same Repo

Two separate clones (not worktrees) of the same repo both have `.cc-deck/environment.yaml` with the same env name. The containers would collide.

Resolution: the variant mechanism. Or the user provides a different name at create time. The global registry tracks by path, so two entries with the same name but different paths coexist. Container names are scoped by name+variant.

### Concurrent Access

Two terminals in the same project running `cc-deck env create` simultaneously. The global registry and local `status.yaml` use the existing atomic-write pattern (write to `.tmp`, then rename).

### No Git Repository

If the project has no `.git/`, the walk stops at the filesystem root. `.cc-deck/` can still be created manually, it just will not benefit from worktree features. The gitignore inside `.cc-deck/` is inert without git.

### Containerized Development

If cc-deck itself runs inside a container (like the demo image), the paths in the global registry are container paths. When the container is recreated, paths still resolve since the workspace is bind-mounted. Not a problem, but worth documenting.

## Open Questions

1. **`cc-deck env init` interactivity**: How much should init ask interactively? Minimum viable: just `--type` is required, everything else has sensible defaults. Full interactive mode can be added later.

2. **Domains config location**: User-defined domain groups currently live in `$XDG_CONFIG_HOME/cc-deck/domains.yaml`. Should a project-local domains file (`.cc-deck/domains.yaml`) be supported for project-specific domain groups? Deferred for now; the global domains file is shared across all projects.

3. **`cc-deck env recreate` command**: When `environment.yaml` changes (e.g., new domain added), should there be a dedicated command to regenerate `run/` and restart the container? Or does `cc-deck env delete && cc-deck env create` suffice?

## Scope

This feature refactors the storage layout and adds project-local config. It does NOT change:
- The Environment interface contract
- The compose/container lifecycle logic
- The Zellij plugin behavior
- Network filtering mechanics

It DOES change:
- Where definitions and state are stored
- How environments are discovered and resolved
- Where image build artifacts are located
- The `cc-deck env list` output format
- CLI flags (`--variant`, init subcommand)
