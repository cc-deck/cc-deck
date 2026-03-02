# Implementation Plan: cc-deck

**Branch**: `001-cc-deck` | **Date**: 2026-03-02 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `specs/001-cc-deck/spec.md`

## Summary

cc-deck is a Zellij WASM plugin (Rust) for managing multiple Claude Code sessions. It provides auto-naming from git repositories, a fuzzy picker overlay for instant session switching, three-state activity detection via Claude Code hooks, and project-based session grouping with color coding. The plugin leverages Zellij's pane management, floating panes, and pipe system rather than building terminal multiplexing from scratch.

## Technical Context

**Language/Version**: Rust (stable, latest edition 2021+)
**Primary Dependencies**: `zellij-tile` (plugin SDK), `serde`/`serde_json` (serialization)
**Storage**: WASI `/cache` directory for persistent recent sessions (JSON file)
**Testing**: `cargo test` for unit tests (native target), manual integration testing in Zellij
**Target Platform**: WASM (wasm32-wasip1), runs inside Zellij 0.42.0+
**Project Type**: Zellij plugin (WASM library)
**Performance Goals**: Plugin load < 500ms, picker open < 100ms, status update < 2s
**Constraints**: WASM sandbox (no PTY output reading, no host env vars, limited filesystem)
**Scale/Scope**: 3-10 concurrent sessions, single developer use

## Constitution Check

*Constitution is a template (not yet populated). No gates to evaluate.*

## Project Structure

### Documentation (this feature)

```text
specs/001-cc-deck/
├── spec.md
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── review_brief.md
├── checklists/
│   └── requirements.md
├── contracts/
│   ├── pipe-protocol.md
│   └── claude-hooks.md
└── tasks.md
```

### Source Code (repository root)

```text
src/
├── main.rs              # Plugin entry point, ZellijPlugin trait impl
├── state.rs             # PluginState struct, session/group management
├── session.rs           # Session, SessionStatus, state transitions
├── group.rs             # ProjectGroup, color assignment
├── recent.rs            # RecentEntries, persistence to /cache
├── picker.rs            # Fuzzy picker UI rendering and input handling
├── status_bar.rs        # Status bar UI rendering
├── pipe_handler.rs      # Pipe message parsing (Claude hooks + internal)
├── config.rs            # PluginConfig parsing from KDL
├── git.rs               # Git repo detection via run_command
└── keybindings.rs       # Keybinding registration via reconfigure

Cargo.toml               # Dependencies, WASM target config
zellij-dev.kdl            # Development layout for hot-reload
zellij-layout.kdl         # Production layout with cc-deck
README.md                 # User documentation (generated later)
```

**Structure Decision**: Single Rust crate compiled to WASM. Flat module structure (no deep nesting). Each module maps to a clear responsibility from the spec.

## Complexity Tracking

No constitution violations to justify. Structure is intentionally simple.
