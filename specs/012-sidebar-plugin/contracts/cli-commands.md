# CLI Commands Contract: cc-deck

**Date**: 2026-03-07
**Feature**: 012-sidebar-plugin

## cc-deck install

Installs the cc-deck plugin, layout, and hooks.

```
cc-deck install [--force] [--skip-backup] [--layout <name>]
```

| Flag | Default | Description |
|------|---------|-------------|
| --force | false | Overwrite existing files without prompting |
| --skip-backup | false | Skip creating backup of settings.json |
| --layout | cc-deck | Layout name to install |

**Artifacts created:**
- `~/.config/zellij/plugins/cc_deck.wasm`
- `~/.config/zellij/layouts/cc-deck.kdl`
- `~/.claude/settings.json` (modified, backup created first)

**Exit codes:**
- 0: Success
- 1: Error (with message to stderr)

## cc-deck uninstall

Removes cc-deck plugin, layout, and hooks.

```
cc-deck uninstall [--skip-backup]
```

| Flag | Default | Description |
|------|---------|-------------|
| --skip-backup | false | Skip creating backup of settings.json |

**Artifacts removed:**
- `~/.config/zellij/plugins/cc_deck.wasm`
- `~/.config/zellij/layouts/cc-deck.kdl`
- cc-deck hook entries from `~/.claude/settings.json`

**Exit codes:**
- 0: Success (including "nothing to remove")
- 1: Error (with message to stderr)

## cc-deck hook

Receives Claude Code hook events and forwards to the Zellij plugin.

```
cc-deck hook
```

Reads JSON from stdin (Claude Code hook payload). Reads `ZELLIJ_PANE_ID` from environment.

**stdin format** (Claude Code hook JSON):
```json
{
  "session_id": "abc123",
  "hook_event": "PreToolUse",
  "tool_name": "Bash",
  "cwd": "/home/user/project"
}
```

**Pipe message sent:**
```
zellij pipe --name "cc-deck:hook" --payload '{"session_id":"abc123","pane_id":42,"hook_event":"PreToolUse","tool_name":"Bash","cwd":"/home/user/project"}'
```

**Exit codes:**
- 0: Always (never fails, never disrupts Claude Code)

**Silent failure conditions:**
- Zellij not running (ZELLIJ env var unset or `zellij` not on PATH)
- Malformed JSON on stdin
- `zellij pipe` command fails

## cc-deck plugin status

Shows current plugin installation status.

```
cc-deck plugin status
```

**Output:**
```
Plugin:  installed (v0.2.0) at ~/.config/zellij/plugins/cc_deck.wasm
Layout:  installed at ~/.config/zellij/layouts/cc-deck.kdl
Hooks:   registered (10 event types)
```

**Exit codes:**
- 0: Installed
- 1: Not installed or partially installed
