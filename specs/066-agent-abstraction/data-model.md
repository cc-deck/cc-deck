# Data Model: Agent Abstraction Layer

## Go Entities

### Agent (interface)

The central abstraction. Each supported AI coding agent implements this interface.

| Method | Return | Description |
|--------|--------|-------------|
| `Name()` | `string` | Machine-readable identifier (e.g., "claude", "opencode") |
| `DisplayName()` | `string` | Human-readable name (e.g., "Claude Code", "OpenCode") |
| `Indicator()` | `string` | 1-3 char sidebar indicator (e.g., "CC", "OC"); must be unique |
| `IsInstalled()` | `bool` | Whether the agent binary is available in PATH |
| `DetectConfig()` | `string` | Filesystem path to agent's config directory |
| `InstallHooks()` | `error` | Write hook artifacts to agent's config |
| `UninstallHooks()` | `error` | Remove hook artifacts from agent's config |
| `HooksInstalled()` | `bool` | Whether cc-deck hooks are currently installed |
| `TranslateEvent([]byte)` | `(*NormalizedPayload, error)` | Parse agent-specific JSON, produce normalized payload |

### NormalizedPayload (struct)

The common format sent to the Zellij plugin. Extends the existing pipePayload.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `agent` | `string` | yes | Agent type identifier (e.g., "claude", "opencode") |
| `session_id` | `string` | no | Session identifier from the agent |
| `pane_id` | `uint32` | yes | Zellij pane ID (resolved by hook command) |
| `hook_event_name` | `string` | yes | Normalized event name (SessionStart, PreToolUse, etc.) |
| `tool_name` | `string` | no | Tool being used (for PreToolUse/PostToolUse) |
| `cwd` | `string` | no | Working directory |
| `agent_id` | `string` | no | Subagent identifier (for Claude Code subagent events) |
| `badges` | `[]string` | no | Evaluated badge rules |

### Registry (package-level)

| Field | Type | Description |
|-------|------|-------------|
| `agents` | `map[string]Agent` | Maps agent name to implementation |
| `indicators` | `map[string]string` | Maps indicator to agent name (uniqueness check) |

**Operations**:
- `Register(a Agent)`: Add agent; panic on duplicate name or indicator
- `Get(name string) Agent`: Look up by name; nil if not found
- `All() []Agent`: Return all registered agents (stable order)

## Rust Entities

### HookPayload (extended)

Add `agent` field to existing struct in `pipe_handler.rs`.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `agent` | `Option<String>` | no | Agent type (new field, `#[serde(default)]`) |
| `session_id` | `Option<String>` | no | Session identifier |
| `pane_id` | `u32` | yes | Zellij pane ID |
| `hook_event_name` | `String` | yes | Event name |
| `tool_name` | `Option<String>` | no | Tool name |
| `cwd` | `Option<String>` | no | Working directory |
| `agent_id` | `Option<String>` | no | Subagent ID (Claude-specific) |
| `badges` | `Vec<String>` | no | Badges |

### Session (extended)

Add `agent_name` field to existing struct in `session.rs`.

| Field | Type | Description |
|-------|------|-------------|
| `agent_name` | `Option<String>` | Agent type for this session (set from first hook event) |
| *(all existing fields unchanged)* | | |

### RenderSession (extended)

Add `agent_indicator` field to existing struct in `lib.rs`.

| Field | Type | Description |
|-------|------|-------------|
| `agent_indicator` | `Option<String>` | Display indicator (e.g., "CC", "OC"); None when all same type |
| *(all existing fields unchanged)* | | |

### RenderPayload (extended)

Add `show_agent_indicators` flag.

| Field | Type | Description |
|-------|------|-------------|
| `show_agent_indicators` | `bool` | Whether sidebar should display agent indicators (true when >1 agent type active) |
| *(all existing fields unchanged)* | | |

## Event Mapping

### Claude Code Events (passthrough)

Claude Code events are already the canonical event names used by the Rust plugin. `TranslateEvent()` adds the `agent: "claude"` field and passes through unchanged.

| Claude Code Event | Normalized Event | Activity |
|-------------------|-----------------|----------|
| SessionStart | SessionStart | Init |
| PreToolUse | PreToolUse | Working |
| PostToolUse | PostToolUse | Working |
| PostToolUseFailure | PostToolUseFailure | Working |
| UserPromptSubmit | UserPromptSubmit | Working |
| PermissionRequest | PermissionRequest | Waiting(Permission) |
| Notification | Notification | (timestamp refresh) |
| Stop | Stop | Done |
| SubagentStart | SubagentStart | Working |
| SubagentStop | SubagentStop | AgentDone |
| SessionEnd | SessionEnd | (session removal) |

### OpenCode Events (translated)

OpenCode events are translated to the canonical names by `TranslateEvent()`.

| OpenCode Event | Normalized Event | Source Hook | Key Data |
|----------------|-----------------|-------------|----------|
| `session.next.step.started` | SessionStart | `event` | agent, model info |
| `session.next.step.ended` | Stop | `event` | cost, token counts |
| `tool.execute.before` | PreToolUse | `tool.execute.before` | tool name from `input.tool` |
| `tool.execute.after` | PostToolUse | `tool.execute.after` | result from `output.title` |
| `permission.ask` | PermissionRequest | `permission.ask` | permission details |

## State Transitions

No changes to the existing `Activity` state machine. The agent abstraction adds metadata (agent name/indicator) but does not alter transition rules.

```
Init → Working → Idle → Working (cycle)
     → Waiting → Working
              → Done
     → Done
```
