# Implementation Plan: Sidebar State Refresh on Reattach

**Branch**: `025-sidebar-state-refresh` | **Date**: 2026-03-21 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/025-sidebar-state-refresh/spec.md`

## Summary

After detaching and reattaching to a Zellij session, the sidebar shows "No Claude sessions" because early `PaneUpdate` events deliver incomplete pane manifests, causing `remove_dead_sessions()` to wipe restored cached sessions. The fix adds a startup grace period (3 seconds after permission grant) during which dead session cleanup is deferred. After the grace period, the next `PaneUpdate` reconciles cached state against the now-complete manifest.

## Technical Context

**Language/Version**: Rust stable (edition 2021, wasm32-wasip1 target)
**Primary Dependencies**: zellij-tile 0.43.1, serde/serde_json 1.x
**Storage**: WASI `/cache/sessions.json` (existing, unchanged)
**Testing**: `cargo test` (native target with WASM host function stubs)
**Target Platform**: WASM (wasm32-wasip1, runs inside Zellij)
**Project Type**: Plugin (Zellij WASM sidebar plugin)
**Performance Goals**: N/A (single timestamp comparison per PaneUpdate, negligible)
**Constraints**: Grace period must be short enough for prompt stale entry removal (< 5s)
**Scale/Scope**: Single new field in PluginState, guard in one code path

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Two-Component Architecture | PASS | Plugin-only change, no CLI modifications |
| II. Plugin Installation | PASS | No build changes; `make install` workflow unchanged |
| III. WASM Filename Convention | PASS | No file renames |
| IV. WASM Host Function Gating | PASS | No new host functions; `unix_now_ms()` is pure Rust |
| V. Zellij API Research Order | PASS | No new Zellij API usage |
| VI. Build via Makefile Only | PASS | Use `make install` / `make test` |
| VII. Interface Behavioral Contracts | N/A | No new interface implementations |
| VIII. Simplicity | PASS | Single field, single guard; no abstractions |
| IX. Documentation Freshness | PASS | Will update README if user-facing behavior changes (it does not; this is an internal fix) |
| X. Spec Tracking in README | PASS | Will add spec entry |
| XI. Release Process | N/A | No release-related changes |
| XII. Prose Plugin | PASS | No documentation content changes |
| XIII. XDG Paths | N/A | Plugin uses WASI `/cache/`, not XDG |

All gates pass. No violations to justify.

## Project Structure

### Documentation (this feature)

```text
specs/025-sidebar-state-refresh/
├── spec.md              # Feature specification
├── plan.md              # This file
├── research.md          # Phase 0: research findings
├── data-model.md        # Phase 1: data model changes
├── quickstart.md        # Phase 1: test instructions
└── tasks.md             # Phase 2: task breakdown (next step)
```

### Source Code (files to modify)

```text
cc-zellij-plugin/src/
├── state.rs             # Add startup_grace_until field to PluginState
└── main.rs              # Set grace timestamp at permission grant;
                         # guard remove_dead_sessions() in PaneUpdate handler
```

No new files. No changes to session.rs, sync.rs, sidebar.rs, config.rs, or any other module.

**Structure Decision**: Minimal modification to two existing files. The change is a single timestamp field and a single conditional guard.

## Complexity Tracking

No constitution violations. Table not needed.
