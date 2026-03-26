# Implementation Plan: Keyboard Navigation & Global Shortcuts

**Branch**: `013-keyboard-navigation` | **Date**: 2026-03-09 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/013-keyboard-navigation/spec.md`

## Summary

Add keyboard-driven session navigation to the cc-deck sidebar plugin with two global shortcuts: `Alt+s` to toggle navigation mode (cursor-based session selection) and `Alt+a` for smart attend (priority-based session cycling). Navigation mode supports `j`/`k`/arrow keys for cursor movement, `Enter` to switch, `r` to rename, `d` to delete, `/` to search, and `n` to create. Smart attend uses a tiered priority algorithm distinguishing PermissionRequest (critical) from Notification (soft) waiting states.

Global shortcuts are registered via `reconfigure()` with KDL `MessagePlugin` directives after plugin permissions are granted.

## Technical Context

**Language/Version**: Rust stable (edition 2021, wasm32-wasip1 target) for plugin; Go 1.22+ for CLI
**Primary Dependencies**: zellij-tile 0.43.1 (plugin SDK), serde/serde_json 1.x
**Storage**: WASI `/cache/` for plugin state
**Testing**: `cargo test` (native target with WASM host function stubs)
**Target Platform**: WASM (wasm32-wasip1) running inside Zellij 0.43.1+
**Project Type**: Zellij WASM plugin + Go CLI
**Performance Goals**: Key events processed within one render frame (~16ms)
**Constraints**: Plugin API limitations (no direct pane focus detection event)
**Scale/Scope**: Sidebar rendering with up to ~50 sessions, keyboard input handling

## Constitution Check

Constitution is default template (not customized). No gates to enforce.

**Post-design re-check**: No violations. All changes are within the existing two-component structure (Rust plugin + Go CLI). No new dependencies added.

## Project Structure

### Documentation (this feature)

```text
specs/013-keyboard-navigation/
├── plan.md
├── spec.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── pipe-protocol.md
├── checklists/
│   └── requirements.md
└── tasks.md
```

### Source Code (repository root)

```text
cc-zellij-plugin/src/
├── main.rs          # Key handler expansion, keybinding registration, navigation pipe action
├── state.rs         # New fields: navigation_mode, cursor_index, filter_state, delete_confirm
├── session.rs       # Activity::Waiting(WaitReason) enum change
├── attend.rs        # Smart attend with tiered priority algorithm
├── pipe_handler.rs  # New cc-deck:navigate action, updated hook-to-activity mapping
├── config.rs        # navigate_key, attend_key config fields
├── sidebar.rs       # Cursor rendering (▶ marker), filter input rendering
├── rename.rs        # Minor: accept cursor pane_id instead of active tab pane_id
├── notification.rs  # (unchanged)
├── git.rs           # (unchanged)
└── sync.rs          # (unchanged)

cc-deck/             # Go CLI (no changes needed for this feature)
```

**Structure Decision**: All changes are in the existing Rust plugin. No new files needed, only modifications to existing modules. The Go CLI is unaffected.

## Research Basis

Research conducted with 4 parallel agents exploring:
1. Zellij `rebind_keys()`/`reconfigure()` API and `MessagePlugin` action routing
2. Claude Code hook event names and PermissionRequest vs Notification distinction
3. Plugin focus/selectability independence and event delivery mechanics
4. Navigation patterns from harpoon/room/zbuffers plugins + cc-deck state inventory

See [research.md](research.md) for detailed findings.
