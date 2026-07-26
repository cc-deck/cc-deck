# Data Model: Cross-Pane MCP Server

## Entities

### MCPSession (returned by cc_deck_sessions)

Represents a tracked agent session as seen by MCP tool consumers.

| Field | Type | Source | Description |
|-------|------|--------|-------------|
| name | string | plugin Session.display_name | User-visible session name |
| pane_id | uint32 | plugin Session.pane_id | Zellij pane identifier |
| agent | string | plugin Session.agent_name | Agent type: "claude", "codex", "opencode" |
| state | string | plugin Session.activity | Activity state: "idle", "working", "waiting", "done" |
| cwd | string | plugin Session.working_dir | Current working directory |
| branch | string | git -C {cwd} branch | Git branch name |
| topic | string | plugin Session.topic (new) | First ~100 chars of most recent user prompt |
| project_summary | string | {cwd}/CLAUDE.md | First ~500 chars of project CLAUDE.md |
| recent_tools | []string | plugin Session.recent_tools (new) | Last 10 tool names from hook events |
| modified_files | []string | git -C {cwd} diff --name-only | Files with uncommitted changes |
| paused | bool | plugin Session.paused | Whether session is paused |

### SessionState (returned by cc_deck_session_state)

Detailed state for a single session.

| Field | Type | Source | Description |
|-------|------|--------|-------------|
| name | string | plugin Session.display_name | Session name |
| pane_id | uint32 | plugin Session.pane_id | Pane identifier |
| cwd | string | plugin Session.working_dir | Working directory |
| branch | string | git branch | Current branch |
| git_diff | string | git diff --stat | Summary of changes |
| modified_files | []string | git diff --name-only | Changed file list |
| recent_commits | []string | git log --oneline -5 | Last 5 commit messages |
| recent_tools | []string | plugin state | Recent tool names with timestamps |
| topic | string | plugin Session.topic | Current topic |
| activity_timeline | []ActivityEvent | plugin state | Recent state transitions |

### ActivityEvent

| Field | Type | Description |
|-------|------|-------------|
| timestamp | uint64 | Unix timestamp in milliseconds |
| activity | string | Activity state at that time |
| tool_name | string | Tool name if applicable |

### QueryRequest (file-based handshake)

| Field | Type | Description |
|-------|------|-------------|
| id | string (UUID) | Unique request identifier |
| source_session | string | Display name of asking session |
| question | string | The question text |
| response_path | string | Path where target should write response |
| timestamp | string | ISO 8601 creation time |

### QueryResponse (file written by target agent)

Free-form markdown text written by the target agent to the response file path. No structured format imposed, since the response is natural language from the agent.

## State Transitions

### Ask Flow States

```
Created -> Injected -> WaitingForResponse -> Completed
                    -> TimedOut -> CleanedUp
         -> TargetBusy (immediate error)
```

### Session Topic Updates

```
(any state) --UserPromptSubmit--> topic updated to first ~100 chars of prompt
```

## New Plugin State Fields

Two new fields on the existing `Session` struct:

- `topic: Option<String>` - set from UserPromptSubmit hook's prompt field
- `recent_tools: Vec<String>` - ring buffer of last 10 tool names from PreToolUse/PostToolUse events
