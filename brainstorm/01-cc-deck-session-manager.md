# Brainstorm: cc-deck Session Manager

**Date:** 2026-03-02
**Status:** spec-created
**Spec:** specs/001-cc-deck/

## Problem Framing

Managing 3-5 concurrent Claude Code sessions in Ghostty terminal tabs is painful. Sessions are visually indistinguishable, and finding the right one requires manually scanning through tabs. The developer loses context and time switching between sessions working on different projects or tasks.

The core need is a purpose-built management layer for Claude Code sessions that provides identity (auto-naming), awareness (activity status), organization (project grouping), and speed (fuzzy search switching).

## Approaches Considered

### A: Standalone Go TUI (bubbletea)

Build a full terminal multiplexer from scratch using Go with the Charm stack (bubbletea + lipgloss + bubbles).

- Pros: Full control, no runtime dependencies, familiar Go ecosystem, excellent TUI component library
- Cons: Requires building terminal emulation, PTY management, and screen buffering from scratch. Months of work before the first useful feature ships.

### B: Zellij WASM Plugin (Rust) - CHOSEN

Build cc-deck as a Zellij plugin in Rust, compiled to WASM. Zellij handles all terminal multiplexing; cc-deck focuses on Claude Code session management.

- Pros: Leverages battle-tested terminal emulation. Floating panes for fuzzy picker. Rich plugin API for pane management, keybinding configuration, and event handling. Proven pattern (zellij-attention plugin already integrates Claude Code hooks). Ships in weeks.
- Cons: Runtime dependency on Zellij 0.42.0+. Must use Rust. Constrained by WASM sandbox (no direct PTY output reading). Plugin API still evolving.

### C: Hybrid (Go orchestrator + Zellij plugin)

Go binary manages session metadata and orchestrates Zellij via CLI/IPC. Small Rust WASM plugin handles the in-Zellij UI.

- Pros: CLI testable outside Zellij. Best of both languages.
- Cons: Two codebases, two build targets. Over-engineered for v1.

## Decision

Chose **Approach B: Zellij WASM Plugin** because:
1. Terminal emulation is a solved problem; building it from scratch wastes effort on the wrong layer
2. The Zellij plugin API is rich enough for all cc-deck requirements (validated by research)
3. The zellij-attention plugin proves Claude Code hook integration via pipes works
4. Shipping time drops from months to weeks
5. The plugin stays focused on the value-add (session management) rather than infrastructure

## Key Design Decisions

- **Name:** cc-deck ("control deck" for Claude Code sessions)
- **Interaction model:** Full-screen multiplexer (cc-deck owns the terminal via Zellij)
- **Session lifecycle:** cc-deck launches all sessions (no adoption of external processes)
- **Auto-naming:** Git repo detection with manual override via keybinding
- **Status detection:** Three states (working/waiting/idle) via Claude Code hooks + fallback
- **Keybindings:** Configurable prefix (default Ctrl-B), Ctrl-T for fuzzy picker
- **Persistence:** Recent sessions stored as JSON via WASI filesystem, 20-entry LRU

## Future Considerations

- **paude integration:** Container/remote backends for autonomous Claude sessions (v2). Session model includes extensible `backend` field.
- **Plugin split:** If the single plugin grows too complex, consider splitting status bar and picker into separate plugins communicating via pipes.

## Open Threads

- Claude Code hook API format needs validation against current implementation
- WASI filesystem sandbox restrictions for persistence need testing
- Ctrl-B prefix key conflict with Claude Code keybindings needs verification
