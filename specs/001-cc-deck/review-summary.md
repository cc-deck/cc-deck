# Review Summary: cc-deck

**Spec:** specs/001-cc-deck/spec.md | **Plan:** specs/001-cc-deck/plan.md
**Generated:** 2026-03-02

---

## Executive Summary

Developers working with Claude Code frequently run 3-5 sessions simultaneously across different projects. In a standard terminal setup (e.g., Ghostty tabs), these sessions look identical, making it frustrating and slow to find the right one. cc-deck solves this by providing a purpose-built management layer for Claude Code sessions inside the Zellij terminal multiplexer.

cc-deck is a Zellij plugin written in Rust and compiled to WebAssembly. It does not build terminal multiplexing from scratch. Instead, it relies on Zellij for all terminal emulation, PTY management, and pane handling, and focuses entirely on the Claude-specific features: automatically naming sessions from git repositories, showing whether Claude is actively working, waiting for input, or idle, grouping sessions by project with color coding, and providing a fuzzy search picker (Ctrl+Shift+T) for instant switching.

The plugin integrates with Claude Code's hook system. When hooks are configured, cc-deck receives real-time notifications about Claude's state (working, waiting for permission, done) via Zellij's pipe messaging system. When hooks are not configured, the plugin falls back to basic activity detection. All keybindings use Ctrl+Shift modifiers to avoid conflicts with both Zellij's mode system and Claude Code's own shortcuts.

The implementation is organized into 46 tasks across 8 phases, with the MVP (session creation and fuzzy switching) deliverable after Phase 3 (19 tasks). Each subsequent user story adds incremental value (auto-naming, status tracking, grouping, recent session memory) without breaking previous functionality.

## PR Contents

| Artifact | Description |
|----------|-------------|
| `spec.md` | Feature requirements: 19 functional requirements, 5 user stories, error handling, edge cases |
| `plan.md` | Technical approach: Rust/WASM plugin architecture, project structure, build pipeline |
| `research.md` | Five research decisions: hook integration, WASI storage, keybinding strategy, git detection, rendering model |
| `data-model.md` | Entity definitions: Session, SessionStatus, ProjectGroup, RecentEntry, PluginState |
| `contracts/pipe-protocol.md` | Pipe message format for Claude hook integration and inter-plugin communication |
| `contracts/claude-hooks.md` | Claude Code hook configuration for activity detection |
| `quickstart.md` | Development setup and build instructions |
| `tasks.md` | 46 tasks across 8 phases with dependency tracking |
| `review_brief.md` | Reviewer's guide to scope and key decisions |
| `review-summary.md` | This file |

## Technical Decisions

### Decision: Zellij Plugin vs Standalone TUI
- **Chosen approach:** Build as a Zellij WASM plugin in Rust
- **Alternatives considered:**
  - Standalone Go TUI (bubbletea): Rejected because building terminal emulation from scratch adds months of work for zero user-facing value
  - Hybrid (Go orchestrator + Zellij plugin): Rejected as over-engineered for v1 (two codebases, IPC complexity)
- **Trade-off:** Hard dependency on Zellij 0.42.0+, but shipping time drops from months to weeks
- **Reviewer question:** Is the Zellij dependency acceptable for the target audience?

### Decision: Ctrl+Shift Keybindings (No Prefix Key)
- **Chosen approach:** Direct Ctrl+Shift+key bindings registered via Zellij's `reconfigure` API
- **Alternatives considered:**
  - Ctrl-B prefix (tmux-style): Rejected because Ctrl-B conflicts with both Zellij tmux mode AND Claude Code's "background task" shortcut
  - Ctrl-T for picker: Rejected because Ctrl-T conflicts with both Zellij tab mode AND Claude Code's task list toggle
- **Trade-off:** Requires Kitty Keyboard Protocol support (Ghostty, Kitty, WezTerm, Alacritty all support this)
- **Reviewer question:** Are Ctrl+Shift bindings ergonomic enough for frequent use?

### Decision: Claude Code Hooks via Pipes for Status Detection
- **Chosen approach:** Claude Code hooks send pipe messages (`cc-deck::EVENT::PANE_ID`) to the plugin
- **Alternatives considered:**
  - PTY output screen-scraping: Not possible (WASM sandbox isolates plugin memory)
  - File-based state sharing: Viable but pipes are more natural in the Zellij plugin model
- **Trade-off:** Requires user to configure Claude Code hooks for full status detection. Fallback to basic idle detection without hooks.

## Critical References

| Reference | Why it needs attention |
|-----------|----------------------|
| `spec.md` FR-009/FR-010: Activity status detection | Core differentiator. Three-state detection (working/waiting/idle) depends on correct hook integration. |
| `research.md` Decision 3: Keybinding strategy | Major spec change from brainstorm. Original Ctrl-T/Ctrl-B defaults replaced with Ctrl+Shift to avoid conflicts. |
| `research.md` Decision 5: Plugin rendering model | Architectural subtlety. Plugin serves dual roles (status bar + picker) using different rendering modes. |
| `contracts/claude-hooks.md` | Defines the external integration contract. Hook format must match Claude Code's current API exactly. |
| `data-model.md` SessionStatus state machine | State transitions must be correct. Invalid transitions could show wrong status indicators. |

## Reviewer Checklist

### Verify
- [ ] All 19 functional requirements have implementing tasks (coverage matrix in review)
- [ ] Keybinding defaults (Ctrl+Shift+T/N/R/X) do not conflict with Ghostty shortcuts
- [ ] Claude Code hook event names match current Claude Code API documentation
- [ ] WASI `/cache` directory is writable by plugins without special permissions
- [ ] Session status state machine handles all valid transitions and edge cases

### Question
- [ ] Is the three-state model (working/waiting/idle) sufficient, or should "done" be a visible fourth state?
- [ ] Should cc-deck auto-configure Claude Code hooks on first run, or require manual setup?
- [ ] Is 20 entries the right cap for recent sessions, or should it be configurable?

### Watch out for
- [ ] Zellij plugin API may change between versions. zellij-tile version pinning is critical.
- [ ] Kitty Keyboard Protocol support varies across terminals. Ctrl+Shift bindings may not work in older terminal emulators.
- [ ] WASM performance under wasmi interpreter. CPU-intensive fuzzy matching on 10+ sessions could lag.

## Scope Boundaries
- **In scope:** Session lifecycle (create/switch/close/rename), auto-naming, fuzzy picker, three-state activity detection, project grouping, recent session persistence
- **Out of scope:** Container backends (paude), split panes, output parsing, IDE integration, non-Claude sessions
- **Why these boundaries:** v1 focuses on the core pain point (finding and switching sessions). Container/remote support deferred to v2 with extensible backend field in the data model.

## Naming & Schema Decisions

| Item | Name | Context |
|------|------|---------|
| Project | cc-deck | "Control deck" for Claude Code sessions |
| Plugin binary | cc_deck.wasm | Rust crate naming convention (underscores) |
| Pipe prefix | cc-deck:: | Namespace for pipe messages (hyphens for external) |
| Status states | Working/Waiting/Done/Idle/Exited/Unknown | Six-variant enum for session lifecycle |
| Config keys | idle_timeout, picker_key, etc. | snake_case strings in KDL config |
| Persistence file | /cache/recent.json | WASI virtual path, maps to Zellij cache dir |

## Risk Areas

| Risk | Impact | Mitigation |
|------|--------|------------|
| Zellij plugin API breaking changes | High | Pin zellij-tile version, target stable API (0.42.0+) |
| Claude Code hook format changes | Medium | Abstract hook integration behind pipe parser; fallback to basic detection |
| Ctrl+Shift keys not supported by terminal | Medium | All keybindings configurable; document terminal requirements |
| wasmi performance for fuzzy matching | Low | Session counts are small (3-10); pre-compute match scores |
| WASI /cache path changes | Low | Test persistence early (T037); fail gracefully to empty state |

---
*Share this with reviewers. Full context in linked spec and plan.*
