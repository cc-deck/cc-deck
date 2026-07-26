# Brainstorm: Cross-Pane Agent Communication via MCP Server

**Date:** 2026-07-26
**Status:** active
**Issue:** [#13](https://github.com/cc-deck/cc-deck/issues/13)

## Problem Framing

CC Deck manages multiple AI coding agent sessions (Claude Code, Codex, OpenCode) running in parallel Zellij panes. Each session operates in isolation with its own context window, loaded codebase knowledge, conversation history, and project-specific memory. There is no mechanism for sessions to communicate with each other.

This isolation is a missed opportunity. When a user runs five sessions in parallel, each working on different aspects of a project (or different projects entirely), those sessions accumulate distinct contextual understanding. An agent working on plugin resilience may have deep knowledge about the hook system that would benefit the agent working on the sidebar. An agent in a cc-spex session may have opinions about an extension being developed in another session.

The core insight: **different contexts produce different results.** Cross-pane communication lets agents consult each other, leveraging the diversity of their loaded contexts rather than duplicating the same analysis in every session.

Use cases span both consultation and state observation:
- "Ask the cc-spex session what they think about our extension design"
- "What files has the server-setup session modified?"
- "Read the last 30 lines of the cc-deck session's scrollback"
- An agent autonomously checking with another session before modifying a shared config schema

## Approaches Considered

### A: Minimal MCP - Query Only (no injection)

Expose read-only MCP tools: list sessions, read scrollback, read structured state. No `ask` tool. The calling agent reads the target's scrollback and reasons about it locally.

- Pros: No text injection, no response capture, no disruption. Ships fast.
- Cons: No true agent-to-agent dialogue. The asking agent reasons from raw scrollback, not from the target's contextual understanding. Misses the "different contexts, different results" value.

### B: Full MCP with Ask and File-Based Response (Chosen)

All query tools from A, plus an `ask` tool that injects a question into an idle target session and captures the response via a file-based handshake. The target agent writes its response to a temp file; the MCP server watches for the file and returns it as the tool result.

- Pros: True agent-to-agent consultation. Each agent reasons from its own full context. Supports both human-initiated and autonomous communication.
- Cons: Depends on target being idle. File-based handshake needs timeout handling and cleanup. The injected prompt format must work across agent types.

### C: Full MCP with Persistent Channels

Everything from B plus named communication channels with message queuing. Supports ongoing collaboration and notifications ("tell me when you're done with the config schema").

- Pros: Most flexible, supports long-running collaboration patterns.
- Cons: Significant complexity (channel lifecycle, message ordering, persistence). Overengineered for a first version.

## Decision

**Approach B: Full MCP with Ask and File-Based Response.** It delivers the core value of cross-context consultation without the infrastructure burden of channels. Approach C (channels) can be added as a future enhancement if the one-shot `ask` pattern proves insufficient.

## Key Requirements

### Architecture

The MCP server is implemented as a `cc-deck mcp serve` Go CLI subcommand. The WASM plugin cannot serve as an MCP server (no socket listening in the WASM sandbox), but it handles all Zellij-side operations (scrollback reads, text injection, state tracking). Communication between the MCP server and plugin uses the existing `zellij pipe` infrastructure.

```
Claude Code <-stdio-> cc-deck mcp serve (Go) <-zellij pipe-> cc-deck plugin (WASM)
```

Each Claude Code session spawns its own `cc-deck mcp serve` process (standard MCP stdio lifecycle). Auto-configured during `cc-deck plugin install`.

### MCP Tools (MVP: 4 tools)

1. **`cc_deck_sessions()`** - List all sessions with rich metadata: name, agent type, activity state, CWD, git branch, topic (current prompt summary), project summary (from CLAUDE.md), recent activity (from hook history), modified files (from git).

2. **`cc_deck_read_scrollback(session, lines)`** - Read N lines from a target pane's scrollback buffer. Uses the Zellij plugin API's `get_pane_scrollback`.

3. **`cc_deck_session_state(session)`** - Structured state for a specific session: full git diff summary, recent tool calls from hooks, working directory, detailed activity timeline.

4. **`cc_deck_ask(session, question)`** - Inject a question into an idle target session, wait for the response via file-based handshake. Returns "session busy" error if the target is not idle. Includes configurable timeout.

### Session Discovery (Topic Tracking)

The `prompt` field in the UserPromptSubmit hook payload (already available from Claude Code but currently not parsed by cc-deck) provides a natural, zero-cost session topic. The first ~100 characters of the user's most recent prompt are stored as `session.topic` and returned in `cc_deck_sessions()`.

No LLM calls needed. The user's own words are the best summary of what the session is about. UserPromptSubmit is self-throttling (fires only when the human types a new prompt).

### Session Context Aggregation

For rich session discovery, the MCP server aggregates existing file-based signals without new storage infrastructure:

| Source | What it provides |
|--------|-----------------|
| `{cwd}/CLAUDE.md` | Project description, technologies, commands |
| `~/.claude/projects/{path}/memory/MEMORY.md` | Project knowledge, decisions, patterns |
| Git state (`{cwd}/.git`) | Branch, recent commits, modified files |
| UserPromptSubmit `prompt` field | Current task/question (session topic) |
| Hook event history (plugin state) | Recent tools used, CWD changes, activity timeline |

### Ask Flow (File-Based Handshake)

1. Source agent calls `cc_deck_ask("cc-spex", "what do you think about our extension?")`
2. MCP server writes question to a temp file (e.g., `/tmp/cc-deck-query-{uuid}.md`)
3. MCP server sends pipe message to plugin to inject a structured prompt into the target pane
4. Plugin verifies target is idle, then injects prompt via `write_chars_to_pane_id`
5. Target agent reads the question, writes its response to `/tmp/cc-deck-response-{uuid}.md`
6. MCP server watches for the response file (with timeout)
7. Response content is returned as the tool result
8. Temp files are cleaned up

### Scope

**In scope (MVP):**
- MCP server Go implementation with stdio transport
- Auto-configuration during `cc-deck plugin install` (add to `.mcp.json`)
- Topic extraction from UserPromptSubmit hooks (parse `prompt` field)
- Session context aggregation from existing files
- Plugin-side scrollback reading and state serialization via pipe protocol
- File-based ask/response protocol with timeout handling
- Same-Zellij-instance communication only

**Out of scope (future):**
- Persistent communication channels (Approach C)
- Cross-machine/remote session queries
- LLM-generated session summaries
- `cc_deck_broadcast()` (ask all sessions simultaneously)
- Vector DB / semantic session search
- Structured activity log persistence (SQLite)

## Open Questions

- What is the exact prompt injection format for `ask` that works across Claude Code, Codex, and OpenCode?
- How should timeout and error handling work for busy/unresponsive targets?
- What is the temp file cleanup strategy (immediate after read, periodic sweep, or on MCP server shutdown)?
- Should `cc-deck plugin install` auto-add MCP config to all agents or require opt-in?
- How does this interact with the existing voice relay text injection (shared infrastructure or separate paths)?
- Should the plugin expose a new pipe message type for MCP queries, or reuse/extend existing ones?

## Inspiration

- [herdr](https://github.com/ogulcancelik/herdr) uses a socket API and SKILL.md protocol for inter-agent communication, but as a standalone multiplexer rather than a plugin
- herdr's peer-to-peer model (agents discover each other via `herdr agent list`, read output via `herdr agent read`) validates the session discovery + query pattern
- CC Deck's existing workspace channels abstraction (brainstorm 040) established the transport pattern for bridging local/remote, which the MCP server extends to agent-to-agent communication
