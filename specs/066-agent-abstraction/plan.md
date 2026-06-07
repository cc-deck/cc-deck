# Implementation Plan: Agent Abstraction Layer

**Branch**: `066-agent-abstraction` | **Date**: 2026-06-06 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `specs/066-agent-abstraction/spec.md`

## Summary

Define a Go `Agent` interface with a central registry, refactor all Claude Code-specific logic from `internal/plugin/hooks.go` and `internal/cmd/hook.go` into a `ClaudeAgent` adapter, implement an `OpenCodeAgent` adapter that generates a TypeScript plugin for OpenCode's native plugin API, add `cc-deck hook --raw` for pre-normalized payloads, generalize the Rust Zellij plugin to display per-agent indicators, and update all CLI/UI strings to be agent-agnostic.

## Technical Context

**Language/Version**: Go 1.25 (CLI), Rust stable edition 2021 wasm32-wasip1 (Zellij plugin)
**Primary Dependencies**: cobra v1.10.2 (CLI), zellij-tile 0.44 (plugin SDK), serde/serde_json 1.x
**Storage**: `~/.claude/settings.json` (Claude Code hooks), `~/.config/opencode/plugins/` (OpenCode plugin), WASI `/cache/` (plugin state)
**Testing**: `go test` (Go), `cargo test` (Rust), existing integration tests in `cc-deck/internal/plugin/`
**Target Platform**: Linux/macOS (CLI), WASM wasm32-wasip1 (plugin)
**Project Type**: CLI tool + Zellij plugin (dual-language monorepo)
**Performance Goals**: Hook event forwarding < 50ms (SC-006), agent detection < 2s (SC-004), session appearance < 1s (SC-002)
**Constraints**: Zero regression on existing Claude Code hooks (FR-016/SC-001); build via `make install`/`make test`/`make lint` only
**Scale/Scope**: 2 agent adapters (Claude Code, OpenCode), 16 functional requirements, ~15 files modified, ~5 new files

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Tests + Documentation | PASS | Spec includes SC-001/SC-005/SC-008 for tests; Documentation Requirements section lists README, CLI ref, Antora guide, config ref |
| II. Interface contracts | PASS | New Agent interface; contract document will be created in Phase 1. No existing interface is being re-implemented |
| III. Build and tool rules | PASS | Plan uses `make install`/`make test`/`make lint`; uses `internal/xdg` for paths; no Docker references |

## Project Structure

### Documentation (this feature)

```text
specs/066-agent-abstraction/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output
│   └── agent-interface.md
└── tasks.md             # Phase 2 output (via /speckit-tasks)
```

### Source Code (repository root)

```text
cc-deck/                           # Go CLI
├── internal/
│   ├── agent/                     # NEW: Agent abstraction package
│   │   ├── agent.go               # Agent interface + Registry
│   │   ├── claude.go              # ClaudeAgent adapter
│   │   ├── opencode.go            # OpenCodeAgent adapter
│   │   ├── opencode_plugin.go     # Embedded TypeScript plugin template
│   │   ├── claude_test.go         # ClaudeAgent unit tests
│   │   ├── opencode_test.go       # OpenCodeAgent unit tests
│   │   └── registry_test.go       # Registry unit tests
│   ├── cmd/
│   │   ├── hook.go                # MODIFY: Add --agent flag, delegate to TranslateEvent()
│   │   ├── hook_raw.go            # NEW: --raw subcommand handler
│   │   └── plugin.go              # MODIFY: Add --agents flag, plugin status/uninstall
│   └── plugin/
│       ├── hooks.go               # MODIFY: Extract Claude-specific logic → agent/claude.go
│       └── install.go             # MODIFY: Iterate registry instead of hardcoded Claude logic
├── cmd/cc-deck/
│   └── main.go                    # MODIFY: Register agent package init
└── go.mod                         # No new dependencies expected

cc-zellij-plugin/                  # Rust Zellij plugin
└── src/
    ├── pipe_handler.rs            # MODIFY: Add agent field to HookPayload
    ├── session.rs                 # MODIFY: Add agent_name field to Session
    ├── lib.rs                     # MODIFY: Add agent_name to RenderSession
    ├── controller/
    │   ├── hooks.rs               # MODIFY: Store agent_name from payload
    │   └── render_broadcast.rs    # MODIFY: Include agent_name in RenderSession
    └── sidebar_plugin/
        └── render.rs              # MODIFY: Replace "Claude Code" strings, add agent indicators
```

**Structure Decision**: The new `internal/agent/` package centralizes all agent abstraction logic. Existing files in `internal/plugin/` and `internal/cmd/` are modified to delegate through the Agent interface rather than containing Claude-specific logic directly. This minimizes file moves while achieving clean separation (SC-007).

## Complexity Tracking

No constitution violations to justify.

## Implementation Strategy

### Layer 1: Go Agent Interface + Registry (FR-001, FR-002)

Create `internal/agent/agent.go` with the `Agent` interface and a package-level registry map. The interface methods map directly to the spec:

- **Identity**: `Name() string`, `DisplayName() string`, `Indicator() string`
- **Detection**: `IsInstalled() bool`, `DetectConfig() string`
- **Hooks**: `InstallHooks() error`, `UninstallHooks() error`, `HooksInstalled() bool`
- **Translation**: `TranslateEvent(input []byte) (*NormalizedPayload, error)`

Registry: `var registry = map[string]Agent{}` with `Register(a Agent)` that panics on duplicate name or indicator.

### Layer 2: ClaudeAgent Adapter (FR-003)

Extract from `internal/plugin/hooks.go`:
- `ClaudeSettingsPath()` → `DetectConfig()`
- `RegisterHooks()` → `InstallHooks()`
- `RemoveHooks()` → `UninstallHooks()`
- `HasHooks()` → `HooksInstalled()`
- Hook payload parsing from `internal/cmd/hook.go` → `TranslateEvent()`

The existing hookPayload struct becomes the Claude-specific input format. `TranslateEvent()` parses it and produces a `NormalizedPayload` with the `agent` field set to `"claude"`.

Key: The 11 Claude Code event names (SessionStart, PreToolUse, PostToolUse, PostToolUseFailure, UserPromptSubmit, PermissionRequest, Notification, Stop, SubagentStop, SubagentStart, SessionEnd) remain unchanged in the normalized payload since the Rust plugin already handles them.

### Layer 3: OpenCodeAgent Adapter (FR-004)

- `IsInstalled()`: Check if `opencode` binary is in PATH
- `DetectConfig()`: Return `~/.config/opencode/` (via XDG resolution)
- `InstallHooks()`: Write embedded TypeScript plugin to `~/.config/opencode/plugins/cc-deck.ts`
- `UninstallHooks()`: Delete the plugin file
- `HooksInstalled()`: Check if plugin file exists
- `TranslateEvent()`: Map OpenCode event JSON to NormalizedPayload

The TypeScript plugin template is embedded via `go:embed` and uses OpenCode's plugin API:
- `event` hook → filter for `session.next.step.started` (→SessionStart), `session.next.step.ended` (→Stop)
- `tool.execute.before` → PreToolUse
- `tool.execute.after` → PostToolUse
- `permission.ask` → PermissionRequest

Each handler calls `cc-deck hook --agent opencode` via Bun `$` shell with JSON on stdin.

### Layer 4: Hook Command Refactoring (FR-005, FR-009)

- Add `--agent <name>` flag to `cc-deck hook` (default: "claude" for backward compat)
- Add `--raw` flag that skips `TranslateEvent()` and forwards stdin directly
- Look up agent in registry, call `TranslateEvent()`, add `agent` field to output
- Pane ID resolution and pipe forwarding remain unchanged

### Layer 5: Plugin Install Refactoring (FR-006, FR-007, FR-008)

- `plugin install`: Iterate `agent.Registry()`, call `IsInstalled()` on each, call `InstallHooks()` on detected agents
- Add `--agents` flag to filter by name
- `plugin uninstall`: Iterate and call `UninstallHooks()`
- `plugin status`: Show detection state + hook state for each agent

### Layer 6: Rust Plugin Generalization (FR-010, FR-011, FR-012, FR-013, FR-014)

- Add `agent` field (optional string) to `HookPayload` in `pipe_handler.rs`
- Add `agent_name` field to `Session` in `session.rs`
- Add `agent_name` to `RenderSession` in `lib.rs`
- Controller stores agent name from payload into session
- Sidebar renders `[CC]`/`[OC]` prefix when multiple agent types are active (count distinct agent names across sessions; hide indicators when count == 1)
- Replace 3 hardcoded "Claude Code" strings in `render.rs` with agent-agnostic text (e.g., "cc-deck" or the product name)

### Layer 7: CLI Text Cleanup (FR-015)

Grep for "Claude Code" in Go source and update help text, command descriptions, and log messages to use agent-agnostic language.

### Layer 8: Documentation

Per constitution Principle I and spec Documentation Requirements section:
- Update README.md
- Update CLI reference (`docs/modules/reference/pages/cli.adoc`)
- Add multi-agent setup guide (Antora)
- Update configuration reference

### Dependency Order

```
Layer 1 (interface) → Layer 2 (claude adapter) → Layer 4 (hook cmd) → Layer 5 (plugin install)
                    → Layer 3 (opencode adapter) ↗
Layer 1 → Layer 6 (rust plugin)
Layer 7 (text cleanup) - independent
Layer 8 (docs) - after all code layers
```

Layers 2 and 3 can be developed in parallel after Layer 1. Layer 6 (Rust) is independent of Go layers 2-5 and can be developed in parallel.
