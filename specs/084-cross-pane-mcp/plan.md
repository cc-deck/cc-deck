# Implementation Plan: Cross-Pane Agent Communication via MCP Server

**Branch**: `084-cross-pane-mcp` | **Date**: 2026-07-26 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `specs/084-cross-pane-mcp/spec.md`

## Summary

Add an MCP server (`cc-deck mcp serve`) that exposes 4 tools for cross-pane agent communication: session listing, scrollback reading, structured state queries, and ask/response handshake. The MCP server is a Go stdio process communicating with the Zellij WASM plugin via pipe messages. Session topics are tracked from UserPromptSubmit hook prompts.

## Technical Context

**Language/Version**: Go 1.25 (from go.mod) for CLI; Rust stable wasm32-wasip1 for plugin

**Primary Dependencies**: cobra v1.10.2 (CLI), zellij-tile 0.43.1 (plugin SDK), mcp-go (new, MCP server library), serde/serde_json 1.x (plugin serialization)

**Storage**: Temporary files in `os.TempDir()` for ask/response handshake; no persistent storage added

**Testing**: `make test` (Go + Rust), `make lint` (Go + Rust)

**Target Platform**: macOS, Linux (anywhere Zellij runs)

**Project Type**: CLI tool + WASM plugin (existing project, adding new subcommand and pipe handlers)

**Performance Goals**: Session listing < 2s, scrollback read < 1s, ask timeout configurable (default 120s)

**Constraints**: Must work within WASM sandbox (no socket listening); MCP server runs as native Go process

**Scale/Scope**: Single Zellij instance, typically 2-10 concurrent sessions

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Tests + docs | PASS | Tests required for MCP server, pipe handlers, topic tracking. CLI reference, Antora guide, and README updates planned. |
| II. Interface contracts | PASS | New pipe message types follow existing DumpState pattern. MCP tool contracts defined in contracts/mcp-tools.md. |
| III. Build/tool rules | PASS | Use `make test`/`make lint`, `internal/xdg` for paths, podman only. |
| IV. Plugin debug logging | PASS | New pipe handlers will use existing `debug_log` infrastructure. |
| V. Command files | N/A | No command files modified. |

## Project Structure

### Documentation (this feature)

```text
specs/084-cross-pane-mcp/
├── plan.md
├── research.md
├── data-model.md
├── contracts/
│   └── mcp-tools.md
└── tasks.md
```

### Source Code (repository root)

```text
cc-deck/                              # Go CLI
├── internal/
│   ├── cmd/
│   │   └── mcp.go                    # NEW: cc-deck mcp serve command
│   ├── mcp/
│   │   ├── server.go                 # NEW: MCP server setup, tool registration
│   │   ├── tools.go                  # NEW: Tool handlers (sessions, scrollback, state, ask)
│   │   ├── pipe.go                   # NEW: Zellij pipe communication helpers
│   │   ├── context.go                # NEW: Session context aggregation (CLAUDE.md, git, memory)
│   │   ├── ask.go                    # NEW: Ask flow orchestration (inject, watch, cleanup)
│   │   └── server_test.go            # NEW: Tests
│   ├── agent/
│   │   ├── agent.go                  # MODIFY: Add Prompt field to NormalizedPayload
│   │   ├── claude.go                 # MODIFY: Add Prompt field to claudeHookPayload
│   │   └── codex.go                  # MODIFY: Add Prompt field to codexHookPayload
│   └── cmd/
│       ├── hook.go                   # MODIFY: Pass prompt to plugin in NormalizedPayload
│       └── plugin.go                 # MODIFY: Add MCP config to .mcp.json during install
├── go.mod                            # MODIFY: Add mcp-go dependency

cc-zellij-plugin/                     # Rust WASM plugin
├── src/
│   ├── pipe_handler.rs               # MODIFY: Add MCP pipe actions to PipeAction enum
│   ├── session.rs                    # MODIFY: Add topic and recent_tools fields to Session
│   ├── controller/
│   │   ├── mod.rs                    # MODIFY: Handle new MCP pipe actions
│   │   ├── hooks.rs                  # MODIFY: Store topic from hook prompt field
│   │   └── mcp_handlers.rs           # NEW: MCP-specific pipe message handlers
│   └── lib.rs                        # MODIFY: Add mcp_handlers module

docs/                                 # Documentation
├── modules/
│   ├── guides/pages/
│   │   └── cross-pane-communication.adoc  # NEW: User guide
│   └── reference/pages/
│       └── cli.adoc                  # MODIFY: Add mcp serve reference
```

**Structure Decision**: Extends existing project structure. Go CLI gets a new `internal/mcp/` package for the MCP server. Rust plugin gets a new `mcp_handlers.rs` module. No new top-level directories.

## Global Constraints

These values are copied from the spec and apply to all tasks:

- **Scrollback line cap**: 500 lines maximum per request (FR-016)
- **Default ask timeout**: 120 seconds (FR-008, SC-003)
- **Topic truncation**: first ~100 characters of user prompt (FR-011)
- **Recent tools ring buffer**: 10 entries maximum
- **CLAUDE.md summary**: first ~500 characters for project summary
- **Supported agents**: Claude Code, Codex, OpenCode
- **Build commands**: `make test`, `make lint` only (never `go build` / `cargo build`)
- **XDG paths**: use `internal/xdg` package
- **Container runtime**: podman only
- **Interface contracts**: defined in `contracts/mcp-tools.md` and `data-model.md`

## Implementation Phases

### Phase 1: Foundation (Plugin State + Hook Prompt)

Extend the plugin Session struct with `topic` and `recent_tools` fields. Add `Prompt` field to Go hook payload structs and forward it to the plugin. Add new pipe message types for MCP queries.

**Files**: session.rs, pipe_handler.rs, hooks.rs, agent.go, claude.go, codex.go, hook.go

### Phase 2: MCP Server Core

Implement `cc-deck mcp serve` subcommand with the mcp-go library. Register 4 tools with JSON schemas. Implement pipe communication helpers that send messages to the plugin and read responses.

**Files**: mcp.go (cmd), server.go, tools.go, pipe.go, go.mod

### Phase 3: Session Listing + Context Aggregation

Implement the `cc_deck_sessions` tool handler. Add `cc-deck:mcp-sessions` pipe handler in the plugin. Implement context aggregation (CLAUDE.md, git state, memory index).

**Files**: tools.go, context.go, mcp_handlers.rs, controller/mod.rs

### Phase 4: Scrollback + State

Implement `cc_deck_read_scrollback` and `cc_deck_session_state` tools. Add scrollback reading to the plugin using `get_pane_scrollback`. Add `cc-deck:mcp-scrollback` and `cc-deck:mcp-state` pipe handlers.

**Files**: tools.go, mcp_handlers.rs, controller/mod.rs

### Phase 5: Ask Flow

Implement `cc_deck_ask` tool with file-based handshake. Add `cc-deck:mcp-inject` pipe handler for targeted text injection (similar to voice relay but with idle check). Implement query file creation, prompt injection, response polling, timeout handling, and cleanup.

**Files**: ask.go, tools.go, mcp_handlers.rs, controller/mod.rs

### Phase 6: Plugin Install Integration

Extend `cc-deck plugin install` to add MCP server config to each agent's configuration file. For Claude Code, add entry to `.mcp.json`. Handle idempotent add/update.

**Files**: plugin.go, claude.go (MCP config section)

### Phase 7: Documentation + Tests

Write unit tests for MCP server, pipe handlers, topic tracking, and context aggregation. Update CLI reference, add Antora guide page, update README.

**Files**: server_test.go, mcp_handlers tests, cross-pane-communication.adoc, cli.adoc, README.md

## Complexity Tracking

No constitution violations. All changes follow existing patterns (pipe messages, cobra commands, Session struct fields).
