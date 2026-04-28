# 034: Unified Setup Command (Container Images + SSH Provisioning)

**Date:** 2026-04-08
**Context:** Hands-on testing of spec 033 (SSH environment) against a Hetzner CAX11 VM revealed that the pre-flight bootstrap is insufficient. Simultaneously, the existing `cc-deck image` command (specs 017-018) solves the same problem for containers. This brainstorm unifies both approaches into a single command with a shared manifest.
**Status:** active
**Depends on:** 033-ssh-environment (transport layer, implemented), 017-base-image, 018-build-manifest
**Supersedes:** the "Design: Automated Remote Provisioning" section of the original 034 brainstorm

## Problem

Two parallel systems exist for preparing a machine to run cc-deck:

1. **Container images** (`cc-deck image`): a manifest-driven pipeline that uses Claude Code slash commands to discover local tools, generate a Containerfile, build the image, and push it to a registry.
2. **SSH remote machines**: a pre-flight bootstrap in `Create()` that checks for Zellij, Claude Code, and cc-deck, with interactive remediation prompts.

These solve the same problem ("make a target machine ready for cc-deck") with different mechanisms but identical intent. The container pipeline is mature, declarative, and idempotent. The SSH bootstrap is procedural, fragile, and riddled with bugs discovered during real-world testing (see "Findings from Testing" below).

The goal is to replace both with a unified `cc-deck setup` command that uses a single manifest and two generation backends: Containerfile for container images, Ansible playbooks for SSH targets.

## Findings from Testing (spec 033)

Testing against a bare Hetzner VM (hostname: marovo, Fedora 43, aarch64) uncovered ten issues with the current SSH bootstrap approach. These findings motivated the redesign.

### F-001: Workspace directory not created automatically

The `Create()` flow validates SSH connectivity but does not create the workspace directory on the remote. When the workspace does not exist, `Attach()` fails with "No such file or directory."

### F-002: Tilde expansion happens locally instead of remotely

When the user passes `--workspace ~/workspace`, the shell expands `~` to the local home directory (e.g., `/Users/rhuss/workspace`). The stored path is wrong for the remote machine.

### F-003: Layout file contains absolute local paths

Copying the local `cc-deck.kdl` layout to the remote preserves absolute paths like `/Users/rhuss/.config/zellij/plugins/cc_deck.wasm`. Zellij on the remote cannot find the plugin at that path. Running `cc-deck plugin install` on the remote generates the layout with correct paths.

### F-004: Controller plugin not loaded

The cc-deck layout only defines the sidebar plugin instance. The controller plugin is loaded via `load_plugins` in the Zellij config, which is set up by `cc-deck plugin install`. Without the controller, the sidebar shows "Waiting for controller."

### F-005: Claude Code hooks not registered

Even with the controller running, the sidebar shows "No Claude sessions" because the Claude Code hooks (in `~/.claude/settings.json` on the remote) are not configured. Running `cc-deck plugin install` on the remote is the only correct approach.

### F-006: Credential file not sourced by shell

Credentials are written to `~/.config/cc-deck/credentials.env` on the remote, but new Zellij panes do not source this file automatically. The provisioning system needs to add a sourcing snippet to the shell config.

### F-007: Zellij remedy uses wrong URL format

The Zellij install remedy constructs URLs using Go-normalized architecture names (`arm64`, `amd64`) but the GitHub release uses raw uname names (`aarch64`, `x86_64`).

### F-008: Claude Code remedy uses npm instead of official installer

The `ClaudeCodeCheck` remedy runs `npm install -g @anthropic-ai/claude-code`. The official installation method is `curl -fsSL https://claude.ai/install.sh | bash`.

### F-009: cc-deck remedy references nonexistent install script

The `CcDeckCheck` remedy runs `curl | bash` from a `main` branch install script that does not exist. It should download the release binary from GitHub Releases.

### F-010: Zellij `attach --layout` flag does not exist

The attach flow used `zellij attach --create-background <name> --layout cc-deck`, but `--layout` is a top-level flag, not a subcommand flag. Fixed during testing.

## Design: Unified `cc-deck setup`

### Core Principle

The "what to install" (tools, shell config, plugins, MCP servers) is orthogonal to "where to install it" (container image vs SSH remote). A single manifest captures the developer profile. Two backends generate target-specific artifacts from that profile.

### Architecture Overview

```
                    cc-deck-setup.yaml
                    (single manifest)
                          │
              ┌───────────┴───────────┐
              │                       │
     /cc-deck.capture          /cc-deck.capture
     (populates manifest,       (same command,
      target-agnostic)           reusable)
              │                       │
              v                       v
     /cc-deck.build            /cc-deck.build
     --target container        --target ssh
              │                       │
              v                       v
        Containerfile           Ansible playbooks
              │                (roles directory)
              v                       │
        podman build                  v
        [--push]             ansible-playbook
              │                       │
              v                       v
     Container image          Provisioned remote
```

### CLI Commands

```
cc-deck setup init [dir]          # scaffold manifest + install Claude commands
cc-deck setup verify [dir]        # smoke-test target (container or remote)
cc-deck setup diff [dir]          # manifest vs last-generated artifacts
```

The `cc-deck image` command is renamed to `cc-deck setup`. No backwards compatibility needed (no external users).

### Claude Code Commands

Two commands replace the current three:

| Command | Purpose |
|---------|---------|
| `/cc-deck.capture` | Discover tools, shell config, plugins, MCP servers from local machine. Target-agnostic. Populates the manifest. |
| `/cc-deck.build --target container\|ssh` | Generate artifacts from manifest, then apply. For containers: generate Containerfile, build image, optionally push (`--push`). For SSH: generate Ansible playbooks, run `ansible-playbook`. |

The current `/cc-deck.push` is folded into `/cc-deck.build --target container --push`. Users who need to re-push without rebuilding can run `podman push` directly.

### Manifest Schema

```yaml
version: 1

# ---- WHAT (populated by /cc-deck.capture, target-agnostic) ----
tools:
  - go 1.25
  - ripgrep
  - jq
  - yq
  - fzf
  - starship

settings:
  shell: zsh
  shell_rc: ./build-context/zshrc
  zellij_config: current
  claude_md: ~/.claude/CLAUDE.md
  claude_settings: ~/.claude/settings.json
  hooks: ~/.claude/hooks.json
  mcp_settings: ~/.config/cc-setup/mcp.json

plugins:
  - name: superpowers
    source: superpowers@claude-plugins-official

mcp:
  - name: filesystem
    command: npx
    args: ["-y", "@anthropic-ai/mcp-filesystem"]

github_tools:
  - name: starship
    repo: starship/starship
    asset_pattern: "starship-{arch}-unknown-linux-gnu.tar.gz"

sources:
  - url: https://github.com/user/project
    tools: [go 1.25]

# ---- WHERE (target-specific configuration) ----
targets:
  container:
    name: my-dev
    tag: latest
    base: quay.io/cc-deck/cc-deck-base:latest
    registry: quay.io/cc-deck

  ssh:
    host: dev@marovo
    port: 22
    identity_file: ~/.ssh/id_ed25519
    create_user: true
    user: dev
    workspace: ~/workspace
```

Both target sections can coexist. A single `/cc-deck.capture` run populates the shared sections. Then `/cc-deck.build --target container` and `/cc-deck.build --target ssh` each generate from the same manifest.

### Generated Directory Structure

For `--target container` (same as current `cc-deck image`):

```
.cc-deck/setup/
├── cc-deck-setup.yaml              # manifest (source of truth)
├── Containerfile                    # generated (DO NOT EDIT)
├── build-context/                   # cc-deck binaries, config files
│   ├── cc-deck-linux-amd64
│   ├── cc-deck-linux-arm64
│   └── zshrc
├── build.sh                         # standalone rebuild script
└── .gitignore
```

For `--target ssh`:

```
.cc-deck/setup/
├── cc-deck-setup.yaml              # manifest (source of truth)
├── inventory.ini                    # generated from targets.ssh
├── site.yml                         # main playbook entry point
├── roles/
│   ├── base/                        # user creation, SSH keys, shell setup
│   │   ├── tasks/main.yml
│   │   ├── templates/
│   │   └── defaults/main.yml
│   ├── tools/                       # system packages + github releases
│   ├── cc-deck/                     # cc-deck CLI + plugin install + hooks
│   ├── claude/                      # Claude Code via official installer
│   ├── zellij/                      # Zellij binary download
│   ├── shell-config/               # zshrc, starship, aliases, credential sourcing
│   └── mcp/                         # MCP server setup
├── group_vars/
│   └── all.yml                      # variables extracted from manifest
├── README.md                        # standalone usage instructions
└── .gitignore
```

### Ansible Design Principles

**Idempotent and standalone.** The generated playbooks must run correctly without an LLM. After the initial convergence loop with Claude (where playbook errors are fixed interactively), anyone can re-run `ansible-playbook -i inventory.ini site.yml` to update the target machine. No Claude Code involvement needed.

**Local control node.** Ansible runs on the user's local machine, connecting to the remote via SSH. Ansible is a required prerequisite (`brew install ansible` on macOS). The `/cc-deck.build` command checks for Ansible and provides a clear error message if missing.

**Role-per-concern.** Each Ansible role handles one logical concern (tools, cc-deck, Claude, shell config). Roles are small, testable independently, and can be skipped selectively.

**Self-correction loop.** When `/cc-deck.build --target ssh` runs `ansible-playbook` and a task fails, Claude reads the error output, fixes the relevant role, and re-runs. Ansible's idempotency means already-succeeded tasks are skipped on retry. The loop runs up to 3 iterations before stopping.

### Ansible Roles Detail

**`base` role:**
- Detects OS and package manager (dnf, apt, etc.)
- Creates user if `create_user: true` (with sudo access, SSH authorized keys)
- Sets default shell (zsh or bash)
- Installs core packages (git, curl, tar, unzip)

**`tools` role:**
- Installs system packages from `tools` list (maps free-form descriptions to package names)
- Downloads GitHub release binaries from `github_tools` list
- Handles architecture mapping (aarch64/x86_64 for download URLs)

**`zellij` role:**
- Downloads Zellij release binary for the target architecture
- Installs to `/usr/local/bin/zellij`
- Supports version pinning (pin to the version used during development)

**`claude` role:**
- Installs Claude Code via official installer (`curl -fsSL https://claude.ai/install.sh | bash`)
- Claude Code auto-updates itself, so version pinning is not relevant

**`cc-deck` role:**
- Downloads cc-deck release binary from GitHub Releases for the target OS and architecture
- Runs `cc-deck plugin install` on the remote (installs WASM plugin, layout files, controller config, Claude Code hooks)
- This single command resolves F-003, F-004, F-005 from the testing findings

**`shell-config` role:**
- Installs curated base shell config (generated from captured local config, filtered for portability)
- Adds user overlay file for personal aliases and functions
- Adds credential sourcing snippet to shell RC: `[ -f ~/.config/cc-deck/credentials.env ] && source ~/.config/cc-deck/credentials.env`
- Installs starship prompt config if starship is in the tools list

**`mcp` role:**
- Configures MCP servers from the manifest `mcp` section
- Sets up config files (not credentials; those flow through `cc-deck env attach`)

### Credential Handling

Ansible provisions the **mechanism** (shell sourcing, credential file path, directory permissions). The actual secrets flow through the existing `cc-deck env attach` credential forwarding at attach time. This keeps secrets out of playbooks entirely and makes the playbooks safe to commit to git.

The `shell-config` role adds a sourcing line to the shell RC file:
```bash
# cc-deck credential sourcing
[ -f ~/.config/cc-deck/credentials.env ] && source ~/.config/cc-deck/credentials.env
```

This resolves F-006 from the testing findings.

### Relationship to `cc-deck env create --type ssh`

The lifecycle is cleanly separated:

- **`cc-deck setup`** provisions the machine (one-time or update). Makes a bare VM into a ready dev environment.
- **`cc-deck env create --type ssh`** registers a session environment (many times per machine). Assumes the machine is already provisioned.

The current `internal/ssh/bootstrap.go` (pre-flight checks with interactive remediation) is deleted. The `Create()` flow becomes:

1. Validate SSH connectivity
2. Lightweight probe: `ssh user@host 'which zellij && which cc-deck && which claude'`
3. If probe fails: "Host appears unprovisioned. Run `cc-deck setup` first."
4. If probe passes: register the environment, create workspace directory, done.

Multiple environments can target the same SSH host (different workspaces, different names). The provisioning is host-level; the environment is session-level.

### Diff Command

`cc-deck setup diff` compares the **current manifest** against the **last generated artifacts** (Containerfile or Ansible playbooks). If the manifest changed since the last `/cc-deck.build` run, diff shows what would change before regenerating.

For container targets: same as the current `cc-deck image diff` behavior (manifest entries not reflected in Containerfile).

For SSH targets: manifest entries not reflected in the current playbook roles (e.g., a new tool added to the manifest but no corresponding task in `roles/tools/tasks/main.yml`).

### Verify Command

`cc-deck setup verify` smoke-tests the target:

- **Container target**: runs checks inside the container (same as current `cc-deck image verify`). Verifies cc-deck version, Claude Code availability, language tools.
- **SSH target**: runs the same checks via SSH against the remote host. Reuses the probe logic but with detailed per-tool reporting.

### Init Command

`cc-deck setup init [dir] --target container,ssh` scaffolds the setup directory:

1. Creates `.cc-deck/setup/` directory
2. Generates manifest template with the requested target sections uncommented
3. Installs Claude commands to `.claude/commands/` (capture and build)
4. For SSH target: scaffolds empty Ansible role skeletons in `roles/`
5. Creates `.gitignore`

The `--target` flag accepts `container`, `ssh`, or both (comma-separated). If omitted, the full template is generated with all sections commented out.

## Migration from `cc-deck image`

The existing `cc-deck image` command (init, verify, diff) and its Claude commands (capture, build, push) are replaced by the unified `cc-deck setup` command. Since there are no external users, this is a clean rename with no backwards compatibility concerns.

The internal `build` package is renamed to `setup`. The manifest file changes from `cc-deck-image.yaml` to `cc-deck-setup.yaml`. The Claude commands change from `cc-deck.capture`/`cc-deck.build`/`cc-deck.push` to `cc-deck.capture`/`cc-deck.build`.

## Open Questions

- Q: Should the Ansible roles support both dnf and apt, or should we start with one package manager and add others later? Starting with dnf (Fedora/RHEL) matches the current testing target (Hetzner VM with Fedora 43).
- Q: Should `cc-deck setup verify --target ssh` run an Ansible `verify.yml` playbook (declarative, reusable) or direct SSH commands (simpler, no Ansible dependency for verification)?
- Q: How should version pinning work? Pin Zellij and cc-deck to specific versions in the manifest, or always install latest? Pinning cc-deck to the local version ensures remote and local match.
- Q: Should the `init` command detect that the project already has an old-style `.cc-deck/image/` directory and offer to migrate?

## Next Steps

1. Create a spec (034) for the unified setup command
2. Implement the CLI command (`cc-deck setup init/verify/diff`)
3. Refactor `internal/build` to `internal/setup` with target abstraction
4. Create the Ansible generation backend
5. Update the Claude commands (capture, build) for dual-target support
6. Remove `internal/ssh/bootstrap.go` and simplify `Create()` to lightweight probe
7. Test against marovo (Hetzner CAX11, Fedora 43, aarch64)
