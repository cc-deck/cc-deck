# Implementation Plan: Sidebar Session Isolation

**Branch**: `044-sidebar-session-isolation` | **Date**: 2026-04-25 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `specs/044-sidebar-session-isolation/spec.md`

## Summary

Scope all plugin state persistence and inter-instance sync by Zellij PID so each sidebar instance only sees sessions from its own Zellij session. The change is confined to `cc-zellij-plugin/src/sync.rs` (state file paths, pipe message names, orphan cleanup) with minor touchpoints in `lib.rs` (pipe message routing) and test files.

## Technical Context

**Language/Version**: Rust stable (edition 2021), compiled to wasm32-wasip1
**Primary Dependencies**: zellij-tile 0.44, serde/serde_json 1.x
**Storage**: WASI `/cache/` directory for persistent state (sessions.json, session-meta.json, zellij_pid)
**Testing**: cargo test (unit tests in sync.rs, state.rs)
**Target Platform**: WASM (wasm32-wasip1) running inside Zellij
**Project Type**: Zellij plugin (WASM)
**Performance Goals**: No regression from current timer-based sync (<10ms per cycle)
**Constraints**: WASI sandbox limits filesystem access to `/cache/`; `/proc/` may not be available
**Scale/Scope**: Typically 1-5 concurrent Zellij sessions on a single machine

## Constitution Check

*GATE: Constitution is a template with no project-specific rules. No violations possible.*

## Project Structure

### Documentation (this feature)

```text
specs/044-sidebar-session-isolation/
├── plan.md              # This file
├── research.md          # Phase 0: WASI capabilities research
├── data-model.md        # Phase 1: state file schema changes
├── checklists/
│   └── requirements.md  # Spec quality checklist
└── tasks.md             # Phase 2 output
```

### Source Code (repository root)

```text
cc-zellij-plugin/src/
├── sync.rs              # PRIMARY: state file paths, pipe names, orphan cleanup
├── lib.rs               # Pipe message routing (name extraction)
└── sync tests           # Unit tests for PID-scoped behavior
```

**Structure Decision**: Changes are confined to the plugin crate. No CLI (Go) changes needed. The sync module is the single point of change for file paths, pipe names, and cleanup logic.

## Complexity Tracking

No complexity violations. The change is a straightforward scoping of existing mechanisms.
