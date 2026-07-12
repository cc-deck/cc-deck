# Implementation Plan: Codex CLI Agent Adapter

**Branch**: `081-codex-adapter` | **Date**: 2026-07-12 | **Spec**: [spec.md](spec.md)

## Summary

Add a Codex CLI agent adapter (`codex.go`) implementing the full Agent interface. Follows the identical pattern established by `claude.go` and `opencode.go`: hook installation via `~/.codex/hooks.json`, event translation from Codex JSON payloads to `NormalizedPayload`, and credential declaration for `OPENAI_API_KEY`.

## Technical Context

**Language/Version**: Go 1.25 (from go.mod)
**Primary Dependencies**: encoding/json (stdlib), os/exec (stdlib), internal/agent, internal/fileutil
**Testing**: `make test`, `make verify`
**Target Platform**: macOS/Linux CLI

## Constitution Check

- **Tests**: Required. Unit tests for all Agent interface methods.
- **Documentation**: Required. README.md agent list, configuration reference for hooks.json.
- **Build rules**: Use `make test`, `make verify`. Never `go build` directly.

## Research Findings

### Codex hooks.json Format (from source `codex-rs/config/src/hook_config.rs`)

```json
{
  "hooks": {
    "SessionStart": [{"hooks": [{"type": "command", "command": "cc-deck hook --agent codex --pane-id \"$ZELLIJ_PANE_ID\""}]}],
    "Stop": [{"hooks": [{"type": "command", "command": "cc-deck hook --agent codex --pane-id \"$ZELLIJ_PANE_ID\""}]}]
  }
}
```

Key differences from Claude Code's `settings.json`:
- Codex uses a dedicated `hooks.json` file, not a general settings file
- The top-level has a `hooks` wrapper object (Claude Code stores hooks directly in settings)
- Same `MatcherGroup` structure: `{"matcher": optional, "hooks": [{"type": "command", "command": "..."}]}`
- Events support the same `matcher` field for tool-specific filtering

### Existing Pattern (from claude.go and opencode.go)

| Method | Claude Code | OpenCode | Codex (planned) |
|--------|-------------|----------|-----------------|
| Config file | `~/.claude/settings.json` | `~/.config/opencode/opencode.json` | `~/.codex/hooks.json` |
| Hook format | JSON in `hooks` key | TypeScript plugin file | JSON in `hooks` key |
| Event names | Same 11 events | N/A (plugin-based) | 8 events (no PostToolUseFailure, Notification, SessionEnd) |
| Identity check | `exec.LookPath("claude")` | `exec.LookPath("opencode")` | `exec.LookPath("codex")` |

The Codex adapter is closest to the Claude Code adapter in structure (both use JSON hook config), just with a different file path and wrapper object.

### Key Files

| File | Role |
|------|------|
| `cc-deck/internal/agent/codex.go` | New: CodexAgent struct + all Agent methods |
| `cc-deck/internal/agent/codex_test.go` | New: Unit tests |
| `README.md` | Update: Add Codex to agent list |
| `docs/modules/reference/pages/configuration.adoc` | Update: Codex hook config docs |

## Implementation Approach

Single phase: implement `codex.go` with all Agent methods, add tests, update docs. The file is self-contained (no cross-file refactoring needed) and follows the established pattern exactly.
