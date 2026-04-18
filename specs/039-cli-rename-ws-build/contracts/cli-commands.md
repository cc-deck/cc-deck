# CLI Command Schema: Post-Rename

## Top-Level Help Output

```
cc-deck - Manage Claude Code workspaces

Workspace:
  attach [name]     Connect to a workspace
  ls                List workspaces
  exec <name> -- .. Run a command in a workspace
  ws|workspace      Full workspace management

Session:
  snapshot          Save/restore Claude Code sessions

Build:
  build             Prepare images and provision hosts

Config:
  config            System configuration

Additional Commands:
  version           Print version information
```

## Command Tree

### ws (alias: workspace)

| Subcommand | Use | Aliases | Former Name |
|------------|-----|---------|-------------|
| new | `new [name] [-t type]` | | create |
| kill | `kill [name] [--force]` | | delete |
| attach | `attach [name]` | | attach |
| list | `list` | ls | list |
| start | `start [name]` | | start |
| stop | `stop [name]` | | stop |
| status | `status [name]` | | status |
| logs | `logs [name]` | | logs |
| exec | `exec <name> -- <cmd>` | | exec |
| push | `push <name> <src> [dst]` | | push |
| pull | `pull <name> <src> [dst]` | | pull |
| harvest | `harvest <name>` | | harvest |
| prune | `prune` | | prune |
| refresh-creds | `refresh-creds <name>` | | refresh-creds |

### build

| Subcommand | Use | Former Name |
|------------|-----|-------------|
| init | `init [dir]` | setup init |
| run | `run [dir]` | setup run |
| verify | `verify [dir]` | setup verify |
| diff | `diff [dir]` | setup diff |

### config

| Subcommand | Use | Former Location |
|------------|-----|-----------------|
| plugin | `plugin install\|remove\|status` | top-level `plugin` |
| profile | `profile add\|list\|use\|show` | top-level `profile` |
| domains | `domains init\|list\|show\|add\|remove\|blocked` | top-level `domains` |
| completion | `completion bash\|zsh\|fish` | top-level `completion` |

### Promoted top-level shortcuts

| Command | Delegates to |
|---------|-------------|
| `cc-deck attach [name]` | `cc-deck ws attach [name]` |
| `cc-deck ls` | `cc-deck ws list` |
| `cc-deck exec <name> -- <cmd>` | `cc-deck ws exec <name> -- <cmd>` |

### Hidden commands

| Command | Visibility |
|---------|-----------|
| hook | Hidden from help, callable directly |
