# Brainstorm: Project-Local Environment Configuration

**Date:** 2026-03-21
**Status:** Brainstorm
**Depends on:** 023-env-interface, compose environment (next spec)
**Discovered during:** Compose environment brainstorm

## Problem

Currently, environment configuration is global: stored in `$XDG_CONFIG_HOME/cc-deck/environments.yaml`. This means:

1. A new team member must manually run `cc-deck env create` with the right flags to match the team's setup.
2. Environment config is not version-controlled alongside the project.
3. There is no way to express "this project should use this image with these domain allowlists" declaratively.

## Proposal: Project-Local Config File

A project-level config file defines environment defaults. When a user runs `cc-deck env create mydev --type compose` from within the project directory, the CLI picks up these defaults automatically.

## Directory Separation (Key Design Principle)

Generated/runtime artifacts and user-authored config MUST live in separate locations so that the gitignore boundary is clear:

```
my-project/
  cc-deck.yaml              # User-authored config (COMMITTED to git)
  .cc-deck/                  # Generated/runtime artifacts (GITIGNORED)
    compose.yaml             # Generated compose file
    .env                     # Generated credentials (never committed)
    proxy/
      tinyproxy.conf         # Generated proxy config
      whitelist              # Generated allowlist
  src/
  go.mod
```

- **`cc-deck.yaml`** at project root: Declarative, checked into version control. Defines the desired environment shape.
- **`.cc-deck/`** directory: All generated artifacts. Entirely gitignored. Recreatable from `cc-deck.yaml` at any time.

This follows the same pattern as other tools:
- Terraform: `main.tf` (committed) vs `.terraform/` (gitignored)
- Node.js: `package.json` (committed) vs `node_modules/` (gitignored)
- Rust: `Cargo.toml` (committed) vs `target/` (gitignored)

## Config File Schema

```yaml
# cc-deck.yaml
version: 1

# Default environment type for this project
type: compose

# Container image
image: quay.io/cc-deck/cc-deck-demo:latest

# Authentication mode (auto, none, api, vertex, bedrock)
auth: auto

# Network filtering (compose only)
allowed-domains:
  - anthropic
  - github
  - npm

# Port mappings
ports:
  - "8082:8082"

# Storage type (host-path is default for project-local compose)
storage:
  type: host-path

# Additional bind mounts
mounts:
  - "~/.ssh:/home/dev/.ssh:ro"

# Credentials to inject (resolved from host env vars)
credentials:
  - ANTHROPIC_API_KEY
```

## Usage Scenarios

### Scenario 1: New team member joins

```bash
git clone git@github.com:org/my-api.git
cd my-api
cc-deck env create mydev
# Reads cc-deck.yaml, creates compose environment with all project defaults
# Prints: "Created compose environment 'mydev' from project config"
```

### Scenario 2: Solo developer sets up project

```bash
cd my-project
cc-deck env init
# Interactive: asks for type, image, domains
# Writes cc-deck.yaml
# Prints: "Created cc-deck.yaml. Commit this file to share with your team."
```

### Scenario 3: Override project defaults

```bash
cd my-project
cc-deck env create mydev --image my-custom-image:latest
# Uses cc-deck.yaml for everything except image
```

## Config Resolution Order

When creating an environment, options are resolved with this precedence:

1. **CLI flags** (highest priority, explicit user choice)
2. **Project config** (`cc-deck.yaml` in cwd or parent directories)
3. **Global config** (`$XDG_CONFIG_HOME/cc-deck/config.yaml` defaults section)
4. **Hardcoded defaults** (lowest priority)

The CLI walks up from cwd to find `cc-deck.yaml` (like `.gitignore` resolution). The first one found wins.

## Relationship to Existing Config

| File | Purpose | Location | Committed |
|------|---------|----------|-----------|
| `cc-deck.yaml` | Project environment defaults | Project root | Yes |
| `environments.yaml` | All environment definitions | `$XDG_CONFIG_HOME/cc-deck/` | No |
| `state.yaml` | Runtime instance state | `$XDG_STATE_HOME/cc-deck/` | No |
| `config.yaml` | Global user preferences | `$XDG_CONFIG_HOME/cc-deck/` | No |
| `cc-deck-build.yaml` | Image build manifest | Project root | Yes |

`cc-deck.yaml` and `cc-deck-build.yaml` are complementary:
- `cc-deck-build.yaml` defines how to BUILD the image
- `cc-deck.yaml` defines how to RUN the environment (which image, what config)

## Intersection with cc-deck-build.yaml

Some fields overlap (image name, for example). Resolution:
- `cc-deck.yaml` takes precedence for runtime config
- `cc-deck-build.yaml` is only consulted by `cc-deck image` commands
- A future `cc-deck env init` could auto-populate `cc-deck.yaml` from `cc-deck-build.yaml` if present

## Open Questions

1. **File name**: `cc-deck.yaml` vs `.cc-deck.yaml` (dotfile)? Visible config files are more discoverable. Lean toward `cc-deck.yaml` (no dot).

2. **Multiple environments per project**: Should the config support named environment profiles?
   ```yaml
   environments:
     dev:
       type: compose
       allowed-domains: [anthropic, github, npm]
     ci:
       type: compose
       allowed-domains: [anthropic]
       storage: { type: named-volume }
   ```
   Or keep it simple with one default config per project?

3. **`cc-deck env init` command**: Should this be a new command that scaffolds `cc-deck.yaml` interactively? Or just document that users create the file manually?

4. **Parent directory search**: How far up should cc-deck walk to find `cc-deck.yaml`? Until `/` (like git)? Until a `.git` directory (project boundary)? Fixed depth?

## Scope

This brainstorm is intentionally deferred from the compose spec. The compose environment will work without project-local config (all flags on CLI or in `environments.yaml`). Project config is an ergonomic enhancement that layers on top.
