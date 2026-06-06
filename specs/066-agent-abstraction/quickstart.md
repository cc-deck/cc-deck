# Quickstart: Agent Abstraction Layer

## What This Feature Does

Introduces a Go `Agent` interface so cc-deck can support multiple AI coding agents (Claude Code, OpenCode, and future agents) instead of being hardcoded to Claude Code. Each agent gets an adapter that handles detection, hook installation, and event translation.

## Key Files to Understand First

1. **`cc-deck/internal/agent/agent.go`** (NEW) - The Agent interface and registry. Start here.
2. **`cc-deck/internal/agent/claude.go`** (NEW) - ClaudeAgent adapter. Extracted from existing code in `internal/plugin/hooks.go` and `internal/cmd/hook.go`.
3. **`cc-deck/internal/agent/opencode.go`** (NEW) - OpenCodeAgent adapter. Generates a TypeScript plugin for OpenCode's plugin API.
4. **`cc-zellij-plugin/src/pipe_handler.rs`** (MODIFIED) - HookPayload gets an `agent` field.
5. **`cc-zellij-plugin/src/sidebar_plugin/render.rs`** (MODIFIED) - "Claude Code" strings replaced with agent-agnostic text; agent indicators added.

## How to Test

```bash
# Run Go tests (agent adapters)
cd cc-deck && make test

# Run Rust tests (plugin changes)
cd cc-zellij-plugin && cargo test

# Build and install
make install

# Verify plugin install detects agents
cc-deck plugin status

# Verify hooks work (in a Zellij session)
echo '{"hook_event_name":"SessionStart","session_id":"test"}' | cc-deck hook --agent claude
echo '{"event":"SessionStart","agent":"test","session_id":"s1","pane_id":1}' | cc-deck hook --raw
```

## Architecture Summary

```
User runs agent → Agent emits lifecycle events → cc-deck hook --agent <name>
                                                        ↓
                                               TranslateEvent() (adapter)
                                                        ↓
                                               NormalizedPayload + pane_id
                                                        ↓
                                               zellij pipe --name "cc-deck:hook"
                                                        ↓
                                               Rust plugin updates session
                                                        ↓
                                               Sidebar shows [CC]/[OC] + status
```

## Common Patterns

### Adding a New Agent

1. Create `cc-deck/internal/agent/<name>.go`
2. Implement the `Agent` interface
3. Add `func init() { agent.Register(&MyAgent{}) }` 
4. Add unit tests in `<name>_test.go`
5. Import the package in `cmd/cc-deck/main.go`
6. Run `make test && make lint`

### OpenCode Plugin Template

The generated TypeScript plugin (`cc-deck.ts`) lives in `~/.config/opencode/plugins/` and uses OpenCode's plugin API to call `cc-deck hook --agent opencode` on lifecycle events. The template is embedded in the Go binary via `go:embed`.
