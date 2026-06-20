# Implementation Plan: Build Skill Iteration Reduction

**Branch**: `071-build-skill-fixes` | **Date**: 2026-06-20 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/071-build-skill-fixes/spec.md`

## Summary

Eliminate 8-9 build iterations in OpenShell builds by fixing 13 instruction gaps across the build skill (`cc-deck.build.md`), capture skill (`cc-deck.capture.md`), and Go template files (`03-mandatory-stack.tmpl`, `05-shell-finalize.tmpl`). All changes are to skill markdown and template content, not compiled Go code.

## Technical Context

**Language/Version**: Markdown (skill instructions), Go templates (`.tmpl` files with Go template syntax)
**Primary Dependencies**: N/A (editing existing skill files, no new dependencies)
**Storage**: N/A
**Testing**: Manual validation via `/cc-deck.capture --all` + `/cc-deck.build --target openshell` on a test workspace
**Target Platform**: macOS (capture runs locally), Linux containers (build output)
**Project Type**: CLI tool skill documentation
**Performance Goals**: First-try build success (0 self-correction iterations)
**Constraints**: Changes must not break existing container builds (Section A) or SSH provisioning (Section B)
**Scale/Scope**: 4 files to modify, 13 distinct changes

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Tests and documentation | PASS | Changes ARE documentation (skill instructions). No new CLI commands or config options. README unchanged (no user-facing behavior change, only build reliability improvement). |
| II. Interface contracts | N/A | No new interface implementations |
| III. Build and tool rules | PASS | No `go build` or `cargo build` needed. Using `podman` for validation builds. |
| IV. Plugin debug logging | N/A | No plugin changes |

## Project Structure

### Documentation (this feature)

```text
specs/071-build-skill-fixes/
├── plan.md              # This file
├── research.md          # Phase 0: current state analysis
├── quickstart.md        # Quick reference for changes
└── tasks.md             # Implementation tasks (from /speckit.tasks)
```

### Source Code (files to modify)

```text
cc-deck/internal/build/
├── commands/
│   ├── cc-deck.build.md           # Build skill (Sections A2, C2, Key Rules)
│   └── cc-deck.capture.md         # Capture skill (Steps 5, 11)
└── templates/containerfile/
    ├── 03-mandatory-stack.tmpl    # Cache ownership, marketplace setup
    └── 05-shell-finalize.tmpl    # Starship TERM=dumb guard
```

**Structure Decision**: No new files created. All changes are edits to existing skill markdowns and Go template files.
