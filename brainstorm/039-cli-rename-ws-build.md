# 039: CLI Rename - Workspace & Build

## Status: brainstorm

## Problem

The current CLI command structure uses "env" and "setup" as the two main command trees. These names are confusing for several reasons:

1. **"env" vs "setup" are near-synonyms** in casual speech, like brew's "upgrade" vs "update". Users have to memorize which is which.
2. **"env" is overloaded** with environment variables, `.env` files, shell environments.
3. **"setup" conflates two concerns**: artifact generation (building images, provisioning SSH hosts) and system configuration (profiles, domains, plugin management).
4. **For SSH, the two-step flow feels redundant**: "setup" provisions the host, then "env create" registers it. There is typically one environment per SSH host, so two commands feel like busywork.
5. **"session" (current group name) conflicts with Claude Code sessions** that run *inside* the workspace.

## Proposal

### Rename "env" to "ws" (workspace)

The thing cc-deck creates is a workspace, a place where you code. "Workspace" is widely understood (VS Code, Codespaces, Gitpod) and distinct from "session" (which Claude Code already uses internally).

- Primary command: `ws`
- Alias: `workspace` (for discoverability)
- Subcommands use tmux/zellij-inspired naming where possible

### Rename "setup" to "build"

"Build" has strong Docker/OCI precedent and is impossible to confuse with "create" or "new". The AI-assisted capture/generation workflow stays under the `/cc-deck.capture` and `/cc-deck.build` Claude Code commands; the CLI `build` tree manages the artifacts.

### Move config commands out of "setup"

`plugin`, `profile`, and `domains` are configuration management, not build operations. They move under a new `config` parent command.

### Hide plumbing

`hook` (called by Claude Code, never by users) becomes hidden from help output. `completion` moves under `config`.

## Command Tree

```
cc-deck - Manage Claude Code workspaces

Workspace:
  attach [name]     Connect to a workspace (auto-selects if only one running)
  ls                List workspaces
  exec <name> -- .. Run a command in a workspace
  ws|workspace      Full workspace management:
    new <name> [-t type]
    kill <name>
    start <name>
    stop <name>
    status [name]
    logs [name]
    exec <name> -- <cmd>
    push <name> <src> [dst]
    pull <name> <src> [dst]
    harvest <name>
    prune
    refresh-creds <name>

Session:
  snapshot          Save/restore Claude Code sessions (runs INSIDE a workspace)

Build:
  build             Prepare images and provision hosts:
    init [dir]
    run [dir]
    verify [dir]
    diff [dir]

Config:
  config            System configuration:
    plugin install/remove/status
    profile add/list/use/show
    domains init/list/show
    completion bash/zsh/fish

Additional:
  version           Print version information

Hidden:
  hook              Forward Claude Code hook events to Zellij plugin (internal)
```

## Design Decisions

### D1: Only three promoted top-level shortcuts

`attach`, `ls`, and `exec` are promoted because they are daily drivers (used multiple times a day). Everything else lives under `ws` to keep the top-level help clean.

Commands like `start`, `stop`, `status`, `logs` are important but used less frequently. The extra `ws` prefix is acceptable friction.

### D2: Subcommand naming follows tmux/zellij conventions

| Current | New | Why |
|---------|-----|-----|
| create | **new** | Shorter, matches `tmux new-session` |
| attach | **attach** | Already matches tmux |
| delete | **kill** | Matches `tmux kill-session`, signals destructiveness |
| list | **ls** | Matches `tmux ls`, shorter |

### D3: "snapshot" stays top-level, not under "ws"

`snapshot` operates *from inside* a running workspace (saves the Claude Code sessions within the current Zellij session). The promoted workspace commands (`attach`, `ls`, `exec`) operate *on* workspaces from outside. This inside/outside distinction justifies keeping `snapshot` separate.

### D4: "workspace" is a full alias, not just help text

`cc-deck workspace new mydev` works identically to `cc-deck ws new mydev`. Implemented via cobra's `Aliases` field. Users who prefer readability over brevity can use the long form.

### D5: Build stays separate from ws (not merged into "ws new")

The previous discussion considered merging build into `ws new` (auto-build on first create). We decided against this because:

- Build is AI-assisted (uses Claude Code commands for discovery and generation), while ws is pure orchestration.
- Build artifacts are reusable across multiple workspaces (one image, many containers).
- For SSH, build means running Ansible playbooks, which is a heavyweight operation that should be explicit.
- First-run penalty (slow `ws new` because it includes provisioning) would surprise users.

Build remains an optional pre-optimization. `ws new` uses existing artifacts or falls back to the base image.

## Mapping: Current to New

| Current command | New command |
|----------------|-------------|
| `cc-deck env create <name>` | `cc-deck ws new <name>` |
| `cc-deck env attach <name>` | `cc-deck attach <name>` (or `cc-deck ws attach <name>`) |
| `cc-deck env list` | `cc-deck ls` (or `cc-deck ws ls`) |
| `cc-deck env delete <name>` | `cc-deck ws delete <name>` |
| `cc-deck env start <name>` | `cc-deck ws start <name>` |
| `cc-deck env stop <name>` | `cc-deck ws stop <name>` |
| `cc-deck env status <name>` | `cc-deck ws status <name>` |
| `cc-deck env logs <name>` | `cc-deck ws logs <name>` |
| `cc-deck env exec <name> --` | `cc-deck exec <name> --` (or `cc-deck ws exec <name> --`) |
| `cc-deck env push/pull/harvest` | `cc-deck ws push/pull/harvest` |
| `cc-deck env prune` | `cc-deck ws prune` |
| `cc-deck env refresh-creds` | `cc-deck ws refresh-creds` |
| `cc-deck setup init` | `cc-deck build init` |
| `cc-deck setup run` | `cc-deck build run` |
| `cc-deck setup verify` | `cc-deck build verify` |
| `cc-deck setup diff` | `cc-deck build diff` |
| `cc-deck plugin` | `cc-deck config plugin` |
| `cc-deck profile` | `cc-deck config profile` |
| `cc-deck domains` | `cc-deck config domains` |
| `cc-deck completion` | `cc-deck config completion` |
| `cc-deck snapshot` | `cc-deck snapshot` (unchanged) |
| `cc-deck hook` | `cc-deck hook` (hidden from help) |
| `cc-deck version` | `cc-deck version` (unchanged) |

## Internal Impact

### Code changes

- `internal/cmd/env.go` renamed to `internal/cmd/ws.go`, command `Use` changed to `ws`, alias `workspace` added
- `internal/cmd/setup.go` renamed to `internal/cmd/build.go`, command `Use` changed to `build`
- New `internal/cmd/config.go` parent command grouping plugin, profile, domains, completion
- `main.go` groups and registrations updated
- Subcommand `create` renamed to `new`, `delete` renamed to `kill`

### Config/state files

No changes to file paths or YAML structure. The rename is CLI-only.

### Environment type names

`EnvironmentType` constants remain unchanged (`local`, `container`, `compose`, `ssh`, `k8s-deploy`, `k8s-sandbox`). These are internal, not user-facing command names.

## Open Questions

1. Should `cc-deck attach` without arguments auto-attach to the last-used or only-running workspace?
2. Should `snapshot` eventually support a workspace name argument for remote snapshot management?
3. Should the `/cc-deck.capture` and `/cc-deck.build` Claude Code commands be renamed to match (they already say "build")?
