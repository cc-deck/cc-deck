# Implementation Plan: Session Save and Restore

**Branch**: `015-session-save-restore` | **Date**: 2026-03-10 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/015-session-save-restore/spec.md`

## Summary

Add session save/restore commands to cc-deck, allowing users to snapshot their multi-tab Claude Code workspace and restore it after a Zellij restart. Includes explicit save/restore, auto-save via the hook system, named snapshots, and cleanup commands. Requires changes to both the Go CLI (new `session` command group) and the Rust plugin (new `cc-deck:dump-state` pipe message).

## Technical Context

**Language/Version**: Go 1.22+ (CLI), Rust stable wasm32-wasip1 (plugin)
**Primary Dependencies**: cobra (CLI), adrg/xdg (XDG paths), serde/serde_json (plugin serialization), zellij-tile 0.43.1 (plugin SDK)
**Storage**: JSON files in `$XDG_CONFIG_HOME/cc-deck/sessions/`
**Testing**: `go test` (CLI), `cargo test` (plugin)
**Target Platform**: macOS, Linux (inside Zellij terminal multiplexer)
**Project Type**: CLI tool + Zellij WASM plugin
**Performance Goals**: Save completes in < 1 second, restore creates tabs at ~1 tab/second
**Constraints**: Must run inside Zellij session (restore uses `zellij action`), auto-save cooldown of 5 minutes
**Scale/Scope**: Typical workspace: 3-10 tabs

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Two-Component Architecture | PASS | Go CLI handles commands + file I/O; Rust plugin provides state via pipe |
| II. Plugin Installation | N/A | No plugin install changes |
| III. WASM Filename Convention | N/A | No filename changes |
| IV. WASM Host Function Gating | PASS | New `cli_pipe_output` / `unblock_cli_pipe_input` calls will be `#[cfg(target_family = "wasm")]` gated |
| V. Zellij API Research | PASS | `cli_pipe_output` and `unblock_cli_pipe_input` verified in Zellij source (zellij-tile/src/shim.rs) |
| VI. Simplicity | PASS | Minimal new code; reuses existing pipe protocol, session serialization, and XDG config patterns |

## Project Structure

### Documentation (this feature)

```text
specs/015-session-save-restore/
├── spec.md
├── plan.md              # This file
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── pipe-protocol.md
└── tasks.md
```

### Source Code (repository root)

```text
cc-zellij-plugin/src/
├── pipe_handler.rs      # Add DumpState variant to PipeAction
└── main.rs              # Handle DumpState in pipe() method

cc-deck/internal/
├── cmd/
│   └── session.go       # New: cobra command group (save, restore, list, remove)
├── session/
│   ├── snapshot.go      # New: Snapshot struct, load/save/list/remove logic
│   ├── save.go          # New: Save (query plugin + write file)
│   ├── restore.go       # New: Restore (read file + create tabs + start Claude)
│   └── autosave.go      # New: Auto-save logic (cooldown, rotation)
└── cmd/
    └── hook.go          # Modified: add auto-save side-effect
```

**Structure Decision**: Follows the existing pattern of `internal/cmd/` for cobra constructors and dedicated packages (`internal/session/`) for business logic, matching `internal/plugin/` and `internal/config/`.

## Complexity Tracking

No constitution violations to justify.
