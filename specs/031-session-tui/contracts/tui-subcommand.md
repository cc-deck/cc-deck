# Contract: `cc-deck tui` Subcommand

**Feature**: 031-session-tui
**Date**: 2026-03-30

## Command Interface

```
cc-deck tui [flags]
```

### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--poll-local` | duration | `2s` | Polling interval for local environments |
| `--poll-container` | duration | `5s` | Polling interval for container environments |
| `--no-color` | bool | `false` | Disable color output |

### Behavior

1. Launches a full-screen terminal UI using bubbletea
2. Loads all environments from `FileStateStore` and `DefinitionStore`
3. Begins polling at configured intervals
4. Displays the environment list view
5. Accepts keyboard input for navigation and actions
6. On attach: suspends TUI, spawns attach process, resumes on exit
7. On quit (`q` or `Ctrl+C`): restores terminal and exits cleanly

### Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Normal exit |
| 1 | Initialization error (terminal, state store) |

## Key Bindings Contract

All key bindings are defined in the spec under "Key Bindings" sections. The TUI MUST implement all P1-scoped bindings:

### Global
- `q` / `Ctrl+C`: Quit
- `?` / `F1`: Help overlay
- `Esc`: Back / close
- `R`: Force refresh

### List View
- `j`/`k`/`Up`/`Down`: Navigate
- `g`/`G`: Top/bottom
- `Enter`: Attach
- `n`: Create wizard
- `S`: Start
- `X`: Stop
- `d`: Delete (with confirmation)

### Create Wizard
- `Tab`/`Down`: Next field
- `Shift+Tab`/`Up`: Previous field
- `Enter`: Create
- `Esc`: Cancel

### Help Overlay
- `?`/`Esc`/`q`: Close

## Plugin Session Data Contract

The TUI reads session data from the Zellij plugin's cache file. This is a read-only contract.

**Path**: `~/.config/zellij/plugins/cc_deck.wasm/cache/sessions.json`

**Format**: JSON object where keys are pane ID strings and values are session objects.

**Activity field**: Serde-tagged enum. Parse as:
- String value: `"Init"`, `"Working"`, `"Idle"`, `"Done"`, `"AgentDone"`
- Object value: `{"Waiting":"Permission"}`, `{"Waiting":"Notification"}`

The TUI MUST handle both forms and MUST NOT write to this file.
