# Brainstorm: Codex CLI Agent Adapter

**Date:** 2026-07-12
**Status:** active

## Problem Framing

cc-deck supports Claude Code and OpenCode as sidebar agents, but not OpenAI's Codex CLI despite it being installed locally (`codex-cli 0.46.0`). Codex is the third major AI coding agent and its absence limits cc-deck's value as a multi-agent multiplexer.

Initial research suggested Codex (Rust rewrite, v0.46+) had no hooks system, making integration difficult. Deeper investigation of the [source code](https://github.com/openai/codex) (`codex-rs/hooks/`, `codex-rs/config/src/hook_config.rs`) reveals a full hooks system at GA with 10 lifecycle events, the same pattern as Claude Code.

## Research Findings

### Codex Hook System (confirmed from source)

**Events** (from `HookEventsToml` in `config/src/hook_config.rs`):
- `SessionStart`, `SubagentStart`, `PreToolUse`, `PermissionRequest`, `PostToolUse`
- `PreCompact`, `PostCompact`, `UserPromptSubmit`, `SubagentStop`, `Stop`

**Config location**: `~/.codex/hooks.json` (user-level), `<repo>/.codex/hooks.json` (project-level)

**Format** (from test fixtures in `core/tests/suite/hooks.rs`):
```json
{
  "hooks": {
    "SessionStart": [{ "hooks": [{ "type": "command", "command": "cc-deck hook --agent codex" }] }],
    "Stop": [{ "hooks": [{ "type": "command", "command": "cc-deck hook --agent codex" }] }]
  }
}
```

**Handler config fields**: `type` ("command"), `command`, `commandWindows`, `timeout_sec`, `async`, `statusMessage`

**Tool name mapping**: Codex uses `apply_patch` (with `Write`/`Edit` as matcher aliases), `spawn_agent` (with `Agent` alias), and `Bash` as tool names. These differ from Claude Code's tool names but map to the same concepts.

**Stdin JSON payload**: Structured with `session_id`, `cwd`, `model`, and event-specific fields (same pattern as Claude Code).

### Competitor Integration Patterns

All major multiplexers (cmux, Superterm, Agent-Deck) use Codex's hooks system for real-time state detection. Nobody polls session JSONL files. The pattern is: install hook scripts at setup time, receive structured JSON callbacks during runtime.

### Version Note

Local install is v0.46.0, latest is v0.143.0. The hooks system may require a newer version. The adapter should gracefully handle older versions that lack hooks support.

## Approaches Considered

### A: Full hooks-based adapter (chosen)

Implement `codex.go` following the exact same pattern as `claude.go` and `opencode.go`: `InstallHooks()` writes `~/.codex/hooks.json`, `TranslateEvent()` parses the Codex JSON payload into `NormalizedPayload`, `CredentialSpecs()` declares OpenAI credentials.

- Pros: Identical pattern to existing adapters. Real-time event-driven state tracking. Proven by cmux, Superterm, Agent-Deck. All 10 events give full activity state coverage.
- Cons: Requires Codex v0.143+ for hooks. The hooks.json format uses the wrapper `{"hooks": {...}}` structure, slightly different from Claude Code's flat `settings.json` approach.

### B: Session log polling (rejected)

Poll `~/.codex/sessions/YYYY/MM/DD/*.jsonl` files for activity state inference.

- Pros: Works without hooks support. No config file modification needed.
- Cons: No real-time signaling (polling delay). Difficult pane-to-session correlation. No multiplexer uses this approach. Complex file watching logic.

### C: Wrapper script (rejected)

Create a `codex-wrapper` that runs `cc-deck hook` at startup, then exec's `codex`.

- Pros: Simple. Works on any Codex version.
- Cons: Only provides SessionStart/Stop events. No mid-session activity tracking. Users must use the wrapper instead of `codex` directly.

## Decision

Approach A: Full hooks-based adapter. The hooks system is confirmed in Codex source code, used by all major competitors, and follows the same pattern as the existing Claude Code adapter. The implementation is structurally identical to `claude.go` with different config paths and event field names.

## Key Requirements

- New `codex.go` agent adapter implementing the full Agent interface
- `InstallHooks()` writes to `~/.codex/hooks.json`, preserving existing hooks (same merge strategy as Claude Code)
- `UninstallHooks()` removes cc-deck entries from `~/.codex/hooks.json`
- `TranslateEvent()` parses Codex hook JSON payload (session_id, cwd, tool_name) into NormalizedPayload
- `CredentialSpecs()` declares `OPENAI_API_KEY` (required) for the "api" auth mode
- `RequiredDomainGroups()` returns `["openai"]`
- `IsInstalled()` checks for `codex` in PATH
- `DetectConfig()` checks for `~/.codex/` directory
- Indicator: unique 1-3 char symbol (e.g., "CX" or a unicode glyph) distinct from Claude ("✳") and OpenCode ("❯")
- Hook events to register: `SessionStart`, `PreToolUse`, `PostToolUse`, `PermissionRequest`, `UserPromptSubmit`, `Stop`, `SubagentStart`, `SubagentStop`
- Graceful handling when Codex version lacks hooks support

## Open Questions

- What indicator symbol to use for Codex in the sidebar? Should be visually distinct from Claude (✳) and OpenCode (❯).
- Should `InstallHooks()` check the Codex version and warn if hooks are not supported?
- Codex uses `apply_patch` instead of `Write`/`Edit` as tool names. Should `TranslateEvent()` normalize these to match Claude Code's tool names, or should the sidebar/plugin handle both?
