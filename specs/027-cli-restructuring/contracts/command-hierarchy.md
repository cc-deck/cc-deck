# Contract: Command Hierarchy

**Feature**: 027-cli-restructuring
**Date**: 2026-03-22

## Top-Level Command Surface

### Daily Commands (promoted from env)

Each command exists at both `cc-deck <cmd>` and `cc-deck env <cmd>` with identical behavior.

| Command | Use | Aliases | Args | Key Flags |
|---------|-----|---------|------|-----------|
| `attach` | `attach [name]` | | 0-1 | `--branch`, `--create-background` |
| `list` | `list` | `ls` | 0 | `--type`, `--worktrees`, `-o` |
| `status` | `status [name]` | | 0-1 | `-o` |
| `start` | `start [name]` | | 0-1 | |
| `stop` | `stop [name]` | | 0-1 | |
| `logs` | `logs <name>` | | 1 | `--follow`, `--tail` |

### Session Commands

| Command | Use | Subcommands |
|---------|-----|-------------|
| `snapshot` | `snapshot <sub>` | `save`, `restore`, `list`, `remove` |

### Environment Commands

| Command | Use | Subcommands |
|---------|-----|-------------|
| `env` | `env <sub>` | `create`, `delete`, `attach`, `list`, `status`, `start`, `stop`, `logs`, `exec`, `push`, `pull`, `harvest`, `prune` |

### Setup Commands

| Command | Use | Subcommands |
|---------|-----|-------------|
| `plugin` | `plugin <sub>` | `install`, `status`, `remove` |
| `profile` | `profile <sub>` | `add`, `list`, `use`, `show` |
| `domains` | `domains <sub>` | `init`, `list`, `show`, `add`, `remove`, `blocked` |
| `image` | `image <sub>` | `init`, `verify`, `diff` |

### Utility Commands (ungrouped)

| Command | Use |
|---------|-----|
| `hook` | `hook` (internal) |
| `version` | `version` |
| `completion` | `completion [bash\|zsh\|fish]` |

## Behavioral Contract: Promoted Commands

For each promoted command P with env counterpart E:

1. **Output equivalence**: `cc-deck P [args] [flags]` produces the same stdout, stderr, and exit code as `cc-deck env P [args] [flags]` for all valid inputs.
2. **Flag parity**: Every flag available on E is available on P with identical name, shorthand, default, and behavior.
3. **Argument parity**: P accepts the same positional arguments as E with identical validation.
4. **Completion parity**: Tab completion for P and E suggest the same values for arguments and flags.

## Removed Commands

The following commands MUST NOT exist after this feature:

`deploy`, `connect`, `list` (K8s), `delete` (K8s), `logs` (K8s), `sync`

## Help Output Contract

```
cc-deck --help

Daily:
  attach      Attach to an environment
  list        List environments
  logs        View environment logs
  start       Start a stopped environment
  status      Show environment status
  stop        Stop a running environment

Session:
  snapshot    Manage session snapshots

Environment:
  env         Manage environments

Setup:
  domains     Manage domain groups for network filtering
  image       Container image lifecycle
  plugin      Manage the Zellij plugin
  profile     Manage credential profiles

Additional Commands:
  completion  Generate shell completion scripts
  hook        Forward Claude Code hook events to the Zellij plugin
  version     Print version information
```

Note: Commands within each group are sorted alphabetically by Cobra's default behavior.
