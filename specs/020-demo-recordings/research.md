# Research: 020-demo-recordings

**Date**: 2026-03-14
**Method**: Parallel agent team research (4 agents)
**Status**: Complete

## R1: Zellij Pipe Message API

**Decision**: Use existing `zellij pipe` CLI with `--name` flag to send commands to the plugin.

**Rationale**: cc-deck already has a mature pipe handler infrastructure (`pipe_handler.rs`) with 9 existing actions. The `PipeMessage` struct provides `source`, `name`, `payload`, `args`, and `is_private` fields. Adding new pipe actions follows established patterns.

**Key findings**:
- CLI syntax: `zellij pipe --name "cc-deck:action" -- "payload"`
- Plugin method: `fn pipe(&mut self, pipe_message: PipeMessage) -> bool` (return true to re-render)
- Response via `cli_pipe_output(pipe_id, output)` for bidirectional communication
- Existing actions parsed in `pipe_handler.rs:41-67` via `parse_pipe_message()`
- PipeMessage struct fields: `source: PipeSource`, `name: String`, `payload: Option<String>`, `args: BTreeMap<String, String>`, `is_private: bool`

**Alternatives considered**:
- OS-level key simulation (xdotool/ydotool): Rejected, fragile and platform-dependent
- Zellij action write (sending raw key bytes): Rejected, keybindings are intercepted at Zellij level

## R2: Plugin Architecture for Pipe Support

**Decision**: Extend existing `PipeAction` enum and `parse_pipe_message()` with new demo-control actions.

**Rationale**: The plugin already routes pipe messages through a clean enum-based dispatch. Navigation, attend, pause, and help all have internal methods that can be called directly from new pipe action handlers.

**Key findings**:
- `PipeAction` enum in `pipe_handler.rs:17-38` already has: HookEvent, SyncState, RequestState, Attend, Rename, NewSession, Navigate, DumpState, RestoreMeta, Unknown
- `PluginState` struct in `state.rs:41-95` holds: sessions, navigation_mode, cursor_index, show_help, last_attended_pane_id, tabs, pane_manifest
- Action methods available: `enter_navigation_mode()`, cursor index manipulation, `attend::perform_attend()`, session pause toggle
- Navigation mode is instance-local (not synced); pipe must target active-tab instance
- `is_on_active_tab()` check determines which instance handles user actions
- Keybinding path: key event -> match in handle_event -> call action method. Pipe handler can call same methods.

**New pipe actions needed**:
- `cc-deck:nav-toggle` - toggle navigation mode
- `cc-deck:nav-up` / `cc-deck:nav-down` - move cursor
- `cc-deck:nav-select` - select current session (Enter equivalent)
- `cc-deck:attend` - already exists
- `cc-deck:pause` - toggle pause on selected session
- `cc-deck:help` - toggle help overlay

## R3: Zellij CLI Actions for Demo Scripts

**Decision**: Use `zellij action` commands for tab/pane management and `zellij pipe` for plugin control.

**Rationale**: Zellij provides 60+ CLI actions. The key ones for demo scripting are all functional and tested.

**Key actions for demo scripts**:
- `zellij action new-tab --name "tab-name"` - create named tabs
- `zellij action go-to-tab-name "name"` - switch to tab by name
- `zellij action write-chars "text"` - type into focused pane
- `zellij action rename-tab "name"` - rename current tab
- `zellij action query-tab-names` - list tabs (useful for checkpoint timing)
- `zellij action dump-layout` - inspect layout state
- `zellij run -- command` - run a command in a new pane

**Layout structure**: cc-deck sidebar mode uses a 22-char wide plugin pane alongside the main working pane per tab.

**Limitations**: Global keybindings (Alt+s, Alt+a) cannot be triggered via CLI. Must use pipe messages instead, which produce the identical visual result.

## R4: Recording Toolchain

**Decision**: Use asciinema for recording, agg for GIF conversion, ffmpeg for video/audio mixing.

**Rationale**: All tools are already installed and tested on the development machine.

**Tools available**:

| Tool      | Version | Status |
|-----------|---------|--------|
| asciinema | 3.2.0   | Installed, tested |
| agg       | 1.7.0   | Installed, tested (produces production GIFs) |
| ffmpeg    | 8.0.1   | Installed, supports audio mixing |
| macOS say | native  | Available as TTS fallback |

**Cast format**: JSONL header + `[timestamp, event_type, data]` events. Supports v1/v2/v3 with bidirectional conversion.

**Makefile integration**: Current Makefile (165 lines) has clean sections. Demo targets fit as a new "Demo" section. Proposed directory: `demos/` at project root with `scripts/`, `projects/`, `recordings/` subdirectories.

**Alternatives considered**:
- VHS (charmbracelet): Good for scripted recordings but adds a dependency and uses its own DSL
- Screen recording (OBS): Not terminal-native, larger files, harder to automate
