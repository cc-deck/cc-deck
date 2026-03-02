# Review Brief: cc-deck

**Spec:** specs/001-cc-deck/spec.md
**Generated:** 2026-03-02

> Reviewer's guide to scope and key decisions. See full spec for details.

---

## Feature Overview

cc-deck is a Zellij plugin that manages multiple concurrent Claude Code sessions. It gives developers a visual control panel for their Claude sessions: auto-named from git repos, grouped by project with color coding, and switchable instantly via a Ctrl-T fuzzy picker. The core problem is that managing 3-5 Claude Code sessions across multiple terminal tabs is painful because sessions are indistinguishable. cc-deck solves this by adding identity, status awareness, and fast switching to sessions running inside Zellij.

## Scope Boundaries

- **In scope:** Session creation/switching, auto-naming from git repos, fuzzy picker overlay, three-state activity detection (working/waiting/idle), project grouping with colors, status bar, configurable keybindings, recent session persistence
- **Out of scope:** Container/remote backends (paude), split-pane layouts, output parsing, IDE integration, non-Claude sessions
- **Why these boundaries:** v1 focuses on the core pain point (finding and switching sessions). Zellij already handles terminal multiplexing, so cc-deck only adds the Claude-specific management layer. Container backends (paude) are noted as a v2 consideration.

## Critical Decisions

### Decision: Zellij Plugin (Not Standalone Multiplexer)
- **Choice:** Build as a Zellij WASM plugin in Rust rather than a standalone TUI application
- **Trade-off:** Depends on Zellij as a runtime requirement, but avoids building terminal emulation, PTY management, and screen buffering from scratch. Reduces scope from months to weeks.
- **Feedback:** Is the Zellij dependency acceptable for your use case?

### Decision: Claude Code Hooks for Status Detection
- **Choice:** Use Claude Code's hook system (pipe messages) as the primary mechanism for detecting working/waiting/idle states
- **Trade-off:** Requires users to configure Claude Code hooks for full functionality. Falls back to basic pane title monitoring if hooks are not set up.
- **Feedback:** Is the degraded fallback (basic idle detection only) acceptable when hooks aren't configured?

### Decision: Single Plugin Architecture
- **Choice:** One WASM plugin handles everything (status bar, fuzzy picker, session management, auto-naming)
- **Trade-off:** Simpler than multi-plugin split, but the plugin handles both persistent UI (status bar) and popup UI (picker), requiring more internal state management
- **Feedback:** Should we consider splitting the picker into a separate lightweight plugin?

## Areas of Potential Disagreement

### Zellij as a Hard Dependency
- **Decision:** cc-deck only works inside Zellij
- **Why this might be controversial:** Developers using tmux, iTerm2 tabs, or raw terminal windows cannot use cc-deck without switching to Zellij
- **Alternative view:** A standalone TUI in Go/bubbletea would work in any terminal but require building a terminal emulator from scratch
- **Seeking input on:** Is tying to Zellij too limiting for the target audience?

### Three-State Activity Detection via Hooks
- **Decision:** Smart status requires Claude Code hook configuration
- **Why this might be controversial:** Extra setup burden for users. Hooks API may change between Claude Code versions.
- **Alternative view:** Could rely solely on terminal output heuristics (output rate, cursor position) without hooks
- **Seeking input on:** How critical is working/waiting distinction vs. just active/idle?

## Naming Decisions

| Item | Name | Context |
|------|------|---------|
| Project | cc-deck | "Control deck" for Claude Code sessions |
| Prefix key | Ctrl-B (configurable) | tmux-compatible default |
| Picker key | Ctrl-T | Direct activation, no prefix needed |
| Activity states | working/waiting/idle | Three-tier status model |
| Session backend field | `backend` (future) | Extensibility for paude/container support in v2 |

## Open Questions

- [ ] Validate Claude Code hook event names and pipe message format against current API
- [ ] Test WASI filesystem sandbox restrictions for recent.json persistence
- [ ] Verify Ctrl-B prefix does not conflict with Claude Code keybindings

## Risk Areas

| Risk | Impact | Mitigation |
|------|--------|------------|
| Zellij plugin API changes | High | Target stable API (0.42.0+), pin zellij-tile version |
| Claude Code hook format changes | Medium | Abstract hook integration behind an adapter; fallback to basic detection |
| WASI sandbox blocks file persistence | Medium | Test early; fallback to in-memory-only recent list |
| Ctrl-T conflicts with terminal/app keybindings | Low | All keybindings are configurable |

---
*Share with reviewers before implementation.*
