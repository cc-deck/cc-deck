# Implementation Plan: Virtual Sort Fix

**Branch**: `074-virtual-sort-fix` | **Date**: 2026-06-28 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/074-virtual-sort-fix/spec.md`

## Summary

Replace the broken physical tab reorder sort (which races with Zellij's async `MoveTab` API) with a virtual display-only sort. When the user presses S in navigate mode, the controller toggles a `sort_active` flag and re-broadcasts. The render payload sorts sessions by `(tier, tab_index)` instead of just `tab_index`, grouping Active sessions at the top, Inactive in the middle, and Paused at the bottom. Zellij tab positions remain unchanged.

## Technical Context

**Language/Version**: Rust stable (edition 2021, wasm32-wasip1 target)
**Primary Dependencies**: zellij-tile 0.43.1 (plugin SDK), serde/serde_json 1.x
**Storage**: WASI `/cache/` directory (existing, not modified by this feature)
**Testing**: `make test` (cargo test for plugin crate)
**Target Platform**: WASM (wasm32-wasip1)
**Project Type**: Zellij plugin (controller + sidebar architecture)
**Performance Goals**: Sort activation must be instant (no Zellij API calls)
**Constraints**: No physical tab reorder; display-only sort in sidebar
**Scale/Scope**: Handles up to 15+ sessions

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Tests & Documentation | PASS | Unit tests for sort toggle, render sort, auto-clear. Help overlay already documents S keybinding. |
| II. Interface contracts | N/A | No new interfaces; modifying existing controller-sidebar payload. |
| III. Build and tool rules | PASS | Uses `make test`, `make lint`. No direct `cargo build`. |
| IV. Plugin debug logging | PASS | Existing debug logging infrastructure reused. |

## Project Structure

### Documentation (this feature)

```text
specs/074-virtual-sort-fix/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── checklists/
│   └── requirements.md  # Spec quality checklist
└── tasks.md             # Phase 2 output (created by /speckit.tasks)
```

### Source Code (files modified)

```text
cc-zellij-plugin/src/
├── lib.rs                              # Add sort_active to RenderPayload
├── controller/
│   ├── state.rs                        # Add sort_active to ControllerState
│   ├── actions.rs                      # Simplify handle_sort() to toggle
│   ├── render_broadcast.rs             # Conditional sort key
│   └── events.rs                       # Clear sort_active on tab change
└── sidebar_plugin/
    ├── input.rs                        # Remove grace period reset (no longer needed)
    └── render.rs                       # Sort indicator in header
```
