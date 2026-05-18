# Implementation Plan: Configurable Sidebar Badges

**Branch**: `056-sidebar-badges` | **Date**: 2026-05-18 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/057-sidebar-badges/spec.md`

## Summary

Add configurable badge indicators to the sidebar's line 2 (before the git branch icon). Badge rules are defined in `~/.config/cc-deck/config.yaml` under a `badges:` section. The Go CLI hook command evaluates each rule by checking for a JSON file relative to the session's `working_dir`, extracting a value via a dot-path expression, and mapping it to an emoji. Resolved badges are sent in the hook payload to the Rust WASM plugin, which renders them.

## Technical Context

**Language/Version**: Go 1.25 (CLI), Rust stable wasm32-wasip1 (plugin)
**Primary Dependencies**: cobra v1.10.2 (CLI), gopkg.in/yaml.v3 (config), zellij-tile 0.43.1 (plugin SDK), serde/serde_json 1.x
**Storage**: `~/.config/cc-deck/config.yaml` (badge rule definitions)
**Testing**: `go test` (CLI), `cargo test` (plugin)
**Target Platform**: WASM (plugin), macOS/Linux (CLI)
**Project Type**: CLI + Zellij WASM plugin
**Performance Goals**: Badge evaluation adds <50ms per hook event
**Constraints**: No external dependencies for JSON path extraction. Silent failure on all errors.
**Scale/Scope**: Typically 1-5 badge rules, 1-10 active sessions

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Tests + Documentation | WILL COMPLY | Unit tests for badge evaluation (Go), badge rendering (Rust). Config reference and docs updated. |
| II. Interface contracts | N/A | No new interface implementations |
| III. Build and tool rules | WILL COMPLY | Use `make test`, `make lint`. XDG paths via `internal/xdg`. |

## Project Structure

### Documentation (this feature)

```text
specs/057-sidebar-badges/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
└── tasks.md             # Phase 2 output (from /speckit.tasks)
```

### Source Code (repository root)

```text
cc-deck/                          # Go CLI
├── internal/
│   ├── cmd/
│   │   └── hook.go               # MODIFY: add badge evaluation before payload send
│   ├── config/
│   │   └── config.go             # MODIFY: add Badges field to Config struct
│   └── badge/
│       ├── badge.go              # NEW: badge evaluation logic (rule matching, JSON extraction)
│       └── badge_test.go         # NEW: unit tests for badge evaluation

cc-zellij-plugin/                 # Rust WASM plugin
├── src/
│   ├── lib.rs                    # MODIFY: add badges field to RenderSession
│   ├── pipe_handler.rs           # MODIFY: add badges field to HookPayload
│   ├── session.rs                # MODIFY: add badges field to Session
│   ├── controller/
│   │   ├── hooks.rs              # MODIFY: store badges from hook payload into session
│   │   └── render_broadcast.rs   # MODIFY: copy badges from Session to RenderSession
│   └── sidebar_plugin/
│       └── render.rs             # MODIFY: render badges on line 2 before branch icon
```

**Structure Decision**: No new directories except `cc-deck/internal/badge/` for the badge evaluation package. All other changes modify existing files following established patterns.

## Data Flow

```
Hook event fires
  → cc-deck hook (Go CLI)
    → reads config.yaml badges section
    → for each badge rule:
      → check if file exists at working_dir/rule.file
      → read JSON, extract value via dot-path
      → map value to emoji via rule.values (or rule.default)
    → add resolved badges []string to pipePayload
  → zellij pipe → plugin
    → HookPayload.badges stored on Session
    → Session.badges copied to RenderSession.badges
    → sidebar renders badges on line 2 before branch icon
```

## Complexity Tracking

No constitution violations. No complexity justification needed.
