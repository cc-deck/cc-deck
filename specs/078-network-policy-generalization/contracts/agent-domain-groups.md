# Contract: Agent Domain Group Declaration

## Interface

```go
// RequiredDomainGroups returns the builtin domain group names
// this agent needs for its API communication.
// Returns group name references only (not raw domain lists).
// Groups are resolved from builtinGroups or user-defined groups.
// Empty return means the agent has no domain requirements.
RequiredDomainGroups() []string
```

## Behavioral Requirements

1. **Deterministic**: `RequiredDomainGroups()` MUST return the same values on every call. No dynamic resolution.

2. **Group names only**: Return values MUST be builtin group names (e.g., `"anthropic"`) or names that will be defined as user groups. MUST NOT return raw domain patterns.

3. **Minimal set**: Return only groups required for the agent's own API communication. Do NOT include shared ecosystem groups (github, python, nodejs, etc.) as those are handled as shared builtin components with `always: true` or tool-based matching.

4. **No side effects**: The method MUST NOT modify state, perform I/O, or depend on runtime context.

## Implementor Checklist

When adding a new agent adapter:

1. Implement `RequiredDomainGroups()` returning the agent's API domain group names
2. Add the corresponding domain group to `builtin.go` if it does not exist
3. Create a policy component YAML file under `internal/build/policies/` with `match: agents: [<agent-name>]`
4. Add tests verifying the domain group declaration
5. Verify backward compatibility: builds without the new agent produce unchanged policies

## Existing Implementations

| Agent | Groups | Policy Component |
|-------|--------|-----------------|
| ClaudeAgent | `["anthropic"]` | `claude-code.yaml` |
| OpenCodeAgent | `["openai"]` | (new: `opencode.yaml`) |
