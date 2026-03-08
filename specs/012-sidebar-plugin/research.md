# Research: cc-deck Sidebar Plugin

**Date**: 2026-03-07
**Feature**: 012-sidebar-plugin

## Zellij Plugin Architecture

**Decision**: Use zellij-tile 0.43.1 SDK with WASM (wasm32-wasip1) target
**Rationale**: This is the current stable plugin SDK matching our Cargo.toml. It provides all required APIs: pipe messages, tab/pane events, mouse events, run_command, set_selectable, reconfigure.
**Alternatives considered**:
- zellij-tile 0.42: Missing some pipe APIs we need
- Direct WASI without SDK: Too low-level, no event abstraction

## Multi-Instance State Sync

**Decision**: Use pipe broadcast messages for state synchronization between sidebar instances
**Rationale**: zellaude proves this pattern works in production. Each tab gets its own sidebar instance via `tab_template`. Instances broadcast state changes via `pipe_message_to_plugin(MessageToPlugin::new("cc-deck:sync"))`. New instances request current state via `cc-deck:request`.
**Alternatives considered**:
- WASI filesystem: Too slow for real-time sync, race conditions
- Single global instance via load_plugins: Cannot create visible panes, only background instances
- Shared memory: Not available in WASM sandbox

## Sidebar Rendering

**Decision**: ANSI escape codes rendered to stdout (same approach as zellaude and zellij-vertical-tabs)
**Rationale**: Zellij plugins render via stdout with ANSI control sequences. No widget library exists. Both zellaude (powerline status bar) and zellij-vertical-tabs (vertical tab list) prove this approach is effective.
**Alternatives considered**:
- zellij-tile-utils styling helpers: Limited, most plugins do raw ANSI anyway
- HTML/web rendering: Not supported by Zellij plugin system

## Layout Integration

**Decision**: Use `tab_template` in KDL layout to place sidebar on every tab
**Rationale**: This is how Zellij's built-in status bars work. The layout defines a vertical split with the sidebar plugin on the left and `children` (user content) on the right. Every new tab inherits this template.
**Alternatives considered**:
- load_plugins: Only creates background instances, no visible pane
- Manual pane creation per tab: Complex, fragile, doesn't work for initial tab

Layout structure:
```kdl
layout {
    tab_template name="default" {
        pane split_direction="vertical" {
            pane size=22 borderless=true {
                plugin location="file:~/.config/zellij/plugins/cc_deck.wasm" {
                    mode "sidebar"
                }
            }
            children
        }
        pane size=1 borderless=true {
            plugin location="compact-bar"
        }
    }
    default_tab_template {
        pane split_direction="vertical" {
            pane size=22 borderless=true {
                plugin location="file:~/.config/zellij/plugins/cc_deck.wasm" {
                    mode "sidebar"
                }
            }
            children
        }
        pane size=1 borderless=true {
            plugin location="compact-bar"
        }
    }
}
```

## Hook Command Architecture

**Decision**: Go binary (`cc-deck hook`) registered directly as Claude Code hook command in settings.json
**Rationale**: Compiled binary is fast (meets <100ms requirement), handles errors gracefully without shell script fragility, can detect Zellij presence by checking for `zellij` binary and ZELLIJ env var.
**Alternatives considered**:
- Shell script (like zellaude-hook.sh): Fragile, depends on jq, complex JSON manipulation
- Python script: Adds runtime dependency, slower startup

Hook flow:
1. Claude Code fires hook event, passes JSON on stdin
2. `cc-deck hook` reads stdin, parses JSON
3. Extracts: session_id, hook_event, tool_name, cwd from JSON body
4. Reads ZELLIJ_PANE_ID from environment
5. Constructs pipe payload JSON
6. Invokes: `zellij pipe --name "cc-deck:hook" --payload '<json>'`
7. Exits 0

## Settings.json Management

**Decision**: Go's encoding/json for safe JSON manipulation with timestamped backups
**Rationale**: zellaude's shell-based approach (jq pipelines) destroyed settings.json. Go's JSON marshal/unmarshal provides safe, atomic operations. Backup file naming: `settings.json.bak.YYYYMMDD-HHMMSS`.
**Alternatives considered**:
- jq-based shell script: Known to cause data loss (zellaude bug)
- Manual string manipulation: Error-prone

## Keybinding Registration

**Decision**: Use Zellij's `reconfigure()` API for dynamic keybinding registration at plugin load
**Rationale**: This avoids modifying config.kdl. The plugin registers attend and new-session keybindings at startup. Users can override via their config.kdl if desired.
**Alternatives considered**:
- Modify config.kdl during install: Invasive, hard to undo, fragile
- No keybindings (click-only): Poor UX for power users

## Git Repository Detection

**Decision**: Async via `run_command()` with `git rev-parse --show-toplevel` and `git rev-parse --abbrev-ref HEAD`
**Rationale**: WASM plugins cannot execute processes directly. `run_command()` runs async; results arrive via `RunCommandResult` event. This is the same pattern used in our v1 plugin (git.rs).
**Alternatives considered**:
- Synchronous execution: Not available in WASM sandbox
- Parsing filesystem for .git directory: run_command not needed, but misses worktree setups

## Existing Code Reuse

**Decision**: Start fresh but selectively port proven patterns from v1 and zellaude
**Rationale**: The v1 plugin had good test coverage for individual modules. Patterns worth porting:
- Pipe message parsing format (`cc-deck::EVENT_TYPE::PANE_ID` or JSON)
- Git detection async pattern (run_command + RunCommandResult)
- Session name deduplication logic (numeric suffix)
- MRU/elapsed time tracking

Code NOT to port:
- Dual-mode plugin architecture (replaced by sidebar + picker)
- Status bar rendering (replaced by sidebar rendering)
- Keybinding prefix model (replaced by direct shortcuts)
- Project group color system (simplified for v1 sidebar)
