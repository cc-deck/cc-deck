# MCP Tool Contracts

## cc_deck_sessions

Lists all active agent sessions with metadata.

**Parameters**: None

**Returns**: JSON array of MCPSession objects (see data-model.md)

**Errors**:
- Plugin not reachable: `{"error": "cc-deck plugin not running"}`

**Example response**:
```json
[
  {
    "name": "cc-spex",
    "pane_id": 42,
    "agent": "claude",
    "state": "idle",
    "cwd": "/home/user/dev/cc-deck",
    "branch": "047-codex-plugin",
    "topic": "fix the agent indicator bug when sessions replace each other",
    "project_summary": "cc-deck: Zellij plugin + CLI for managing AI coding sessions...",
    "recent_tools": ["Edit", "Bash", "Read"],
    "modified_files": ["cc-zellij-plugin/src/controller/hooks.rs"],
    "paused": false
  }
]
```

## cc_deck_read_scrollback

Reads terminal scrollback from a target session's pane.

**Parameters**:
| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| session | string | yes | - | Session display name or pane ID |
| lines | integer | no | 50 | Number of lines to read (1-500) |

**Returns**: Plain text string containing the last N lines of terminal output.

**Errors**:
- Session not found: `{"error": "session not found: <name>"}`
- Ambiguous name: `{"error": "ambiguous session name '<name>', matches: [pane 42: 'cc-spex', pane 55: 'cc-spex']"}`
- Invalid line count: `{"error": "lines must be between 1 and 500"}`
- Plugin not reachable: `{"error": "cc-deck plugin not running"}`

## cc_deck_session_state

Returns structured state for a single session.

**Parameters**:
| Name | Type | Required | Description |
|------|------|----------|-------------|
| session | string | yes | Session display name or pane ID |

**Returns**: JSON SessionState object (see data-model.md)

**Errors**:
- Session not found: `{"error": "session not found: <name>"}`
- Ambiguous name: `{"error": "ambiguous session name '<name>', matches: [...]"}`
- Plugin not reachable: `{"error": "cc-deck plugin not running"}`

## cc_deck_ask

Injects a question into an idle target session and waits for a file-based response.

**Parameters**:
| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| session | string | yes | - | Target session display name or pane ID |
| question | string | yes | - | Question text to ask the target agent |
| timeout | integer | no | 120 | Timeout in seconds (10-600) |

**Returns**: Plain text string containing the target agent's response.

**Errors**:
- Session not found: `{"error": "session not found: <name>"}`
- Session busy: `{"error": "session '<name>' is not idle (current state: working)"}`
- Timeout: `{"error": "ask timed out after 120s waiting for response from '<name>'"}`
- Plugin not reachable: `{"error": "cc-deck plugin not running"}`
- Temp file error: `{"error": "cannot create temporary files: <reason>"}`

## Pipe Message Contracts (CLI to Plugin)

### cc-deck:mcp-sessions

**Direction**: CLI -> Plugin -> CLI response
**Payload**: None
**Response**: JSON array of session objects with fields: pane_id, display_name, activity, working_dir, agent_name, topic, recent_tools, paused, badges

### cc-deck:mcp-scrollback

**Direction**: CLI -> Plugin -> CLI response
**Payload**: `{"pane_id": 42, "lines": 50}`
**Response**: Plain text scrollback content

### cc-deck:mcp-state

**Direction**: CLI -> Plugin -> CLI response
**Payload**: `{"pane_id": 42}`
**Response**: JSON session state object

### cc-deck:mcp-inject

**Direction**: CLI -> Plugin (fire and forget)
**Payload**: `{"pane_id": 42, "text": "..."}`
**Response**: `{"ok": true}` or `{"error": "session not idle"}`
