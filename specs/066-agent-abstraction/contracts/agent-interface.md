# Contract: Agent Interface

## Overview

The `Agent` interface defines the behavioral contract for all AI coding agent integrations in cc-deck. Each adapter implements this interface to provide detection, hook management, and event translation for a specific agent.

## Interface Definition

```go
type Agent interface {
    // Identity
    Name() string        // Machine-readable, lowercase, alphanumeric (e.g., "claude", "opencode")
    DisplayName() string // Human-readable (e.g., "Claude Code", "OpenCode")
    Indicator() string   // 1-3 chars, unique across all agents (e.g., "CC", "OC")

    // Detection
    IsInstalled() bool   // True if agent binary is available
    DetectConfig() string // Path to agent's config directory, empty if not found

    // Hook Lifecycle
    InstallHooks() error    // Write hook artifacts; idempotent (update if exists)
    UninstallHooks() error  // Remove hook artifacts; no-op if not installed
    HooksInstalled() bool   // True if cc-deck hooks are currently active

    // Event Translation
    TranslateEvent(input []byte) (*NormalizedPayload, error) // Agent-specific → normalized
}
```

## Behavioral Requirements

### B1: Name Uniqueness

`Name()` MUST return a value unique across all registered agents. The registry panics at startup if duplicates are detected.

### B2: Indicator Uniqueness

`Indicator()` MUST return a value unique across all registered agents. The registry panics at startup if duplicates are detected. Indicators are case-sensitive.

### B3: IsInstalled Stability

`IsInstalled()` MUST be safe to call repeatedly without side effects. It MUST NOT modify the filesystem, network, or environment. It SHOULD complete in under 100ms.

### B4: DetectConfig Consistency

`DetectConfig()` MUST return the same path for the same system state. It returns an empty string if the agent's config directory does not exist.

### B5: InstallHooks Idempotency

`InstallHooks()` MUST be idempotent: calling it when hooks are already installed updates them to the current version without duplication. It MUST NOT remove non-cc-deck hook entries.

### B6: UninstallHooks Safety

`UninstallHooks()` MUST only remove cc-deck hook entries. It MUST NOT modify other hook configurations. It MUST be a no-op (return nil) if no cc-deck hooks are installed.

### B7: TranslateEvent Contract

`TranslateEvent(input)` MUST:
- Parse agent-specific JSON from `input`
- Return a `NormalizedPayload` with the `agent` field set to `Name()`
- Return an error for malformed input (never panic)
- Map agent-specific event names to the canonical set: SessionStart, PreToolUse, PostToolUse, PostToolUseFailure, UserPromptSubmit, PermissionRequest, Notification, Stop, SubagentStart, SubagentStop, SessionEnd
- Pass through unrecognized event names as-is (the Rust plugin ignores unknown events)

### B8: Error Handling

`InstallHooks()` and `UninstallHooks()` MUST return descriptive errors on failure (e.g., permission denied, directory not found). They MUST NOT panic. The caller (plugin install command) continues to other agents on error.

## Normalized Payload Schema

```go
type NormalizedPayload struct {
    Agent        string   `json:"agent"`
    SessionID    string   `json:"session_id,omitempty"`
    PaneID       uint32   `json:"pane_id"`
    HookEvent    string   `json:"hook_event_name"`
    ToolName     string   `json:"tool_name,omitempty"`
    Cwd          string   `json:"cwd,omitempty"`
    AgentID      string   `json:"agent_id,omitempty"`
    Badges       []string `json:"badges,omitempty"`
}
```

The `pane_id` and `badges` fields are populated by the hook command after `TranslateEvent()` returns, not by the adapter itself.

## Registration

Agents register via `init()` functions in their respective packages:

```go
func init() {
    agent.Register(&ClaudeAgent{})
}
```

The registry is populated at program startup. No runtime registration is supported.

## Testing Requirements

Each Agent implementation MUST have unit tests covering:
1. `Name()`, `DisplayName()`, `Indicator()` return expected values
2. `IsInstalled()` returns false when binary is not in PATH
3. `InstallHooks()` creates the expected artifacts (mock filesystem)
4. `UninstallHooks()` removes only cc-deck artifacts (mock filesystem)
5. `HooksInstalled()` correctly detects presence/absence
6. `TranslateEvent()` correctly maps all supported event types
7. `TranslateEvent()` returns error for malformed input
