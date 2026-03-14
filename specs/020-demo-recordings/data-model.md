# Data Model: 020-demo-recordings

## Entities

### PipeAction (extended enum)

Existing enum in `pipe_handler.rs` extended with navigation control variants:

| Variant | Pipe Name | Payload | Description |
|---------|-----------|---------|-------------|
| NavToggle | `cc-deck:nav-toggle` | None | Toggle navigation mode on/off |
| NavUp | `cc-deck:nav-up` | None | Move cursor up in session list |
| NavDown | `cc-deck:nav-down` | None | Move cursor down in session list |
| NavSelect | `cc-deck:nav-select` | None | Select session at cursor (Enter) |
| Pause | `cc-deck:pause` | None | Toggle pause on selected session |
| Help | `cc-deck:help` | None | Toggle help overlay |

Existing variants (unchanged): HookEvent, SyncState, RequestState, Attend, Rename, NewSession, Navigate, DumpState, RestoreMeta, Unknown

### Demo Project

Template structure copied to `/tmp/cc-deck-demo/` during setup:

| Field | Type | Description |
|-------|------|-------------|
| name | string | Directory and tab name (e.g., "todo-api") |
| language | string | Primary language (Python, Go, HTML) |
| task | string | Pre-staged task description in CLAUDE.md |
| files | list | Source files included in template |
| git_history | int | Number of pre-staged commits (minimum 2) |

### Demo Scene

Logical unit in a demo script:

| Field | Type | Description |
|-------|------|-------------|
| name | string | Scene identifier and chapter marker |
| actions | list | Sequence of commands, pipe messages, waits |
| narration | string | Corresponding voiceover text (optional) |

### Narration Script

Plain text file with chapter markers:

```
## scene:install
Welcome to cc-deck. Let me show you how to install the plugin.

## scene:launch
Now we launch Zellij with the cc-deck layout.

## scene:navigate
Watch the sidebar as I navigate between sessions.
```

## State Transitions

### Navigation Mode (via pipe)

```
Inactive --[cc-deck:nav-toggle]--> Active (cursor visible)
Active --[cc-deck:nav-up/down]--> Active (cursor moved)
Active --[cc-deck:nav-select]--> Inactive (session focused)
Active --[cc-deck:nav-toggle]--> Inactive (cancelled)
```

### Recording Pipeline

```
Setup --> Recording --> Post-processing --> Output
  |          |              |                |
  |  asciinema record  agg/ffmpeg     GIF/MP4/embed
  |                        |
  setup.sh           voiceover.sh (optional)
```
