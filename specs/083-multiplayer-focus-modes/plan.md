# Implementation Plan: Multiplayer Focus Modes

**Branch**: `083-multiplayer-focus-modes` | **Date**: 2026-07-22 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `specs/083-multiplayer-focus-modes/spec.md`

## Summary

Fix the multiplayer focus bug where sidebar clicks on client 2 control client 1's terminal, and add presence indicators showing which sessions other clients are focused on. The sidebar will call `focus_terminal_pane()` and `switch_tab_to()` directly from its own plugin instance (per-client by design), send focus-reports to the controller for tracking, maintain per-client activation order, and render colored presence indicators using Zellij's `multiplayer_user_colors` palette.

## Technical Context

**Language/Version**: Rust stable (edition 2021, wasm32-wasip1 target)

**Primary Dependencies**: zellij-tile 0.43.1 (plugin SDK), serde/serde_json 1.x (serialization)

**Storage**: WASI `/cache/` directory (existing, no new persistence needed; local activation order is transient sidebar state)

**Testing**: `cargo test` via `make test` (unit tests for Rust plugin), manual observation for multiplayer integration tests

**Target Platform**: WASM (wasm32-wasip1) running inside Zellij terminal multiplexer

**Project Type**: Terminal multiplexer plugin (Zellij WASM plugin + Go CLI)

**Performance Goals**: Render cycle < 10ms (existing target), presence indicators add negligible overhead (a few ANSI escape sequences per session line)

**Constraints**: Pipe messages don't carry client_id (Zellij PR #4094 unmerged), limiting keyboard shortcuts to primary client only

**Scale/Scope**: Up to 10 multiplayer clients (matches Zellij's 10 multiplayer colors), typically 2-3

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Tests + documentation | PASS | Unit tests for new logic, README update, Antora guide page planned |
| II. Interface contracts | PASS | Extends existing pipe protocol (new message type), backward compatible |
| III. Build/tool rules | PASS | Uses `make test`/`make lint`/`make install`, no direct cargo/go build |
| IV. Plugin debug logging | PASS | New debug_log calls follow existing pattern |
| V. Command files as code | N/A | No command file changes |

## Project Structure

### Documentation (this feature)

```text
specs/083-multiplayer-focus-modes/
├── plan.md              # This file
├── spec.md              # Feature specification
├── research.md          # Phase 0: research findings
├── data-model.md        # Phase 1: entity model
├── contracts/           # Phase 1: pipe message contracts
│   └── focus-report.md  # New pipe message format
├── quickstart.md        # Phase 1: validation guide
├── checklists/          # Quality checklists
│   └── requirements.md
└── tasks.md             # Phase 2 output (via /speckit-tasks)
```

### Source Code (repository root)

```text
cc-zellij-plugin/
├── src/
│   ├── lib.rs                          # Shared types (RenderPayload, ActionMessage)
│   ├── pipe_handler.rs                 # Pipe message parsing (add FocusReport)
│   ├── controller/
│   │   ├── state.rs                    # ControllerState (add client_focus, multiplayer_colors)
│   │   ├── actions.rs                  # Action handlers (modify handle_switch, add handle_focus_report)
│   │   ├── events.rs                   # Event subscriptions (add ModeUpdate)
│   │   ├── render_broadcast.rs         # Render payload construction (add other_client_focus)
│   │   └── mod.rs                      # Controller pipe routing (add focus-report handling)
│   └── sidebar_plugin/
│       ├── state.rs                    # SidebarState (add local_activation_order)
│       ├── input.rs                    # Click/keyboard handlers (local focus + focus-report)
│       ├── render.rs                   # Rendering (add presence indicators)
│       └── mod.rs                      # Sidebar pipe/event handling
└── tests/                              # Unit tests for new logic
```

**Structure Decision**: All changes are within the existing `cc-zellij-plugin/` crate. No new crates or packages. The Go CLI (`cc-deck/`) is unaffected.
