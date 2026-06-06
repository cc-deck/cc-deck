# Research: Agent Abstraction Layer

## R1: Claude Code Hook Architecture (Current State)

**Decision**: Extract all Claude Code-specific logic into a `ClaudeAgent` adapter.

**Findings**:
- Hook registration lives in `internal/plugin/hooks.go` with `RegisterHooks()`, `RemoveHooks()`, `HasHooks()`
- Settings path is hardcoded via `ClaudeSettingsPath()` → `~/.claude/settings.json`
- Hook command (`internal/cmd/hook.go`) parses a `hookPayload` struct with fields: `session_id`, `hook_event_name`, `tool_name`, `cwd`, `agent_id`
- Output is a `pipePayload` adding `pane_id` and `badges`
- 11 event types: SessionStart, PreToolUse, PostToolUse, PostToolUseFailure, UserPromptSubmit, PermissionRequest, Notification, Stop, SubagentStop, SubagentStart, SessionEnd
- Pane ID resolution has a two-stage cache mechanism (flag → pane-map.json)
- Settings.json uses Claude Code 2.1+ matcher-based hook format

**Rationale**: Clean extraction path exists. The hookPayload → pipePayload transformation is the natural `TranslateEvent()` boundary. Pane ID resolution and pipe forwarding are agent-agnostic and stay in the hook command.

## R2: OpenCode Plugin System

**Decision**: Generate a TypeScript plugin that calls `cc-deck hook --agent opencode` via Bun shell.

**Findings**:
- OpenCode plugin API defined in `@opencode-ai/plugin` package
- Plugin function signature: `(input: PluginInput) => Promise<Hooks>`
- `PluginInput` provides `$` (BunShell) for executing shell commands
- Key hooks for cc-deck integration:
  - `event`: Catches all lifecycle events including `session.next.step.started` and `session.next.step.ended`
  - `tool.execute.before`: Tool name in `input.tool`, session in `input.sessionID`
  - `tool.execute.after`: Tool result in `output.title`/`output.output`
  - `permission.ask`: Permission prompt in `input`, status in `output.status`
- Plugin file location: `~/.config/opencode/plugins/` (global) or `.opencode/plugins/` (project)
- Plugins are TypeScript files loaded by Bun

**Rationale**: Global plugin location (`~/.config/opencode/plugins/cc-deck.ts`) ensures the integration works across all OpenCode sessions without per-project setup. The `$` shell access makes calling `cc-deck hook --agent opencode` straightforward.

**Alternatives considered**:
- MCP server: Would work but requires separate process management; plugin approach is simpler
- Project-local plugin: Requires copying per project; global is one-time setup

## R3: Agent Detection Patterns

**Decision**: Use PATH-based binary detection for `IsInstalled()`.

**Findings**:
- Claude Code: `claude` binary in PATH (also check for `~/.claude/` directory existence)
- OpenCode: `opencode` binary in PATH (also check for `~/.config/opencode/` directory)
- Go stdlib `exec.LookPath()` handles PATH resolution

**Rationale**: Binary presence is the minimal reliable signal. Config directory existence is a secondary check for `DetectConfig()`. API key validity is explicitly out of scope per spec edge case: "missing credentials are not a hook concern."

## R4: Rust Plugin Session Model Extension

**Decision**: Add `agent_name: Option<String>` to Session and HookPayload.

**Findings**:
- `HookPayload` in `pipe_handler.rs` already has `agent_id: Option<String>` (used for subagent tracking, distinct from agent type)
- `Session` in `session.rs` has no agent type field
- `RenderSession` in `lib.rs` has no agent type field
- Sidebar rendering in `render.rs` hardcodes "Claude Code" in 3 locations (header/loading/permission prompt)
- Agent indicator rendering requires counting distinct agent types across all sessions

**Rationale**: The existing `agent_id` field is for Claude Code's internal subagent concept (distinguishing main agent from subagent tool calls). The new `agent_name` field represents the external agent type (claude, opencode). These are orthogonal concepts and both needed.

## R5: Normalized Payload Format

**Decision**: Extend existing `pipePayload` with an `agent` field.

**Findings**:
- Current pipePayload (Go → Rust): `{ session_id, pane_id, hook_event_name, tool_name, cwd, agent_id, badges }`
- Adding `agent` field: `{ ..., agent: "claude" | "opencode" | ... }`
- The `agent` field is set by `TranslateEvent()` for each adapter, or passed through directly for `--raw`
- Rust plugin parses with serde; new optional field is backward-compatible (`#[serde(default)]`)

**Rationale**: Minimal change to existing wire format. Optional field means old payloads (from pre-upgrade hooks) still parse correctly, defaulting to no agent indicator shown.

## R6: Agent Indicator Display Logic

**Decision**: Show `[CC]`/`[OC]` indicators only when multiple agent types are active.

**Findings**:
- Sidebar renders 3 lines per session; indicator goes before display_name on line 1
- Current `indicator` field in `RenderSession` holds the activity indicator (colored dot)
- Need a separate `agent_indicator` field or prepend to display_name
- Controller can compute `show_agent_indicators: bool` as `distinct_agent_count > 1` when building `RenderPayload`

**Rationale**: Adding a `show_agent_indicators` flag to `RenderPayload` and an `agent_indicator` string to `RenderSession` keeps the decision in the controller (authoritative) and the rendering simple (just check the flag). This avoids the sidebar having to count agent types from the session list.
