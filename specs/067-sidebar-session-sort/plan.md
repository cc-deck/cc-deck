# Implementation Plan: Sidebar Session Sort by Activity

**Branch**: `067-sidebar-session-sort` | **Date**: 2026-06-08 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/067-sidebar-session-sort/spec.md`

## Summary

Add a `S` (Shift+s) keybinding in navigate (amber) mode that physically reorders Zellij tabs to cluster active sessions at the top. Sessions sort into three tiers: Active (Working, Waiting) > Inactive (Idle, Done, AgentDone, Init) > Paused. Stable sort within tiers preserves relative tab order. The sidebar sends a single `Sort` action to the controller, which computes the target order and executes tab swaps via `move_focus_or_tab(Direction)`.

## Technical Context

**Language/Version**: Rust stable (edition 2021, wasm32-wasip1 target) for plugin; shared lib  
**Primary Dependencies**: zellij-tile 0.44 (plugin SDK), serde/serde_json 1.x  
**Storage**: N/A (no persistent state changes)  
**Testing**: `cargo test` (unit tests, no WASM host functions in test)  
**Target Platform**: WASM (wasm32-wasip1)  
**Project Type**: Zellij plugin (WASM)  
**Performance Goals**: Sort completes without visible delay for up to 15 sessions  
**Constraints**: Zellij has no direct tab-reorder API; must use `move_focus_or_tab(Direction)` which moves one tab position at a time  
**Scale/Scope**: Typically 3-15 sessions; worst case ~20

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Tests & documentation | PLANNED | Unit tests for sort logic, help overlay update, README/docs updates in tasks |
| II. Interface contracts | N/A | No new interface implementations |
| III. Build and tool rules | PASS | Using `make install`/`make test`/`make lint`; no direct cargo build |

## Project Structure

### Documentation (this feature)

```text
specs/067-sidebar-session-sort/
├── spec.md
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
└── tasks.md             # Phase 2 output (via /speckit-tasks)
```

### Source Code (repository root)

```text
cc-zellij-plugin/src/
├── lib.rs                          # ActionType enum (add Sort variant)
├── wasm_compat.rs                  # Add move_focus_or_tab_wasm wrapper
├── session.rs                      # Session struct (read-only: activity, paused, tab_index)
├── controller/
│   ├── actions.rs                  # Add handle_sort() with swap algorithm
│   └── state.rs                    # sessions_by_tab_order() (existing, read-only)
└── sidebar_plugin/
    ├── input.rs                    # Add S keybinding in handle_navigate_key()
    └── render.rs                   # Add S to HELP_LINES constant
```

**Structure Decision**: All changes are within the existing cc-zellij-plugin crate. No new files needed; only additions to existing modules.

## Complexity Tracking

No constitution violations. No complexity justifications needed.

## Known Risks

### Navigate Mode Exit During Sort Swaps

The sort swap sequence calls `switch_tab_to` on multiple tabs, which changes `active_tab_index`. When the render broadcast arrives after the sort, the sidebar detects the tab change (mod.rs lines 186-201) and exits navigate mode. The grace period (1500ms) may or may not cover the swap sequence duration.

**Mitigation**: The controller must restore the original active tab index after the swap sequence completes, so the sidebar does not see a tab change in the render broadcast.

### Cursor Tracking Limitation

`preserve_cursor()` in state.rs only clamps the cursor index to bounds. It does NOT track by pane_id. The sidebar must explicitly find the new index of the previously-highlighted session's pane_id in the updated session list after the sort completes.
