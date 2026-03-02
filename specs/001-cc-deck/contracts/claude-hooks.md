# Contract: Claude Code Hook Configuration

Users must add the following to their Claude Code settings to enable smart status detection.

## Required Configuration

Add to `~/.claude/settings.json`:

```json
{
  "hooks": {
    "UserPromptSubmit": [
      {
        "matcher": "",
        "hooks": [
          {
            "type": "command",
            "command": "zellij pipe --name 'cc-deck::working::$ZELLIJ_PANE_ID'"
          }
        ]
      }
    ],
    "PreToolUse": [
      {
        "matcher": "",
        "hooks": [
          {
            "type": "command",
            "command": "zellij pipe --name 'cc-deck::working::$ZELLIJ_PANE_ID'"
          }
        ]
      }
    ],
    "Notification": [
      {
        "matcher": "",
        "hooks": [
          {
            "type": "command",
            "command": "zellij pipe --name 'cc-deck::waiting::$ZELLIJ_PANE_ID'"
          }
        ]
      }
    ],
    "PermissionRequest": [
      {
        "matcher": "",
        "hooks": [
          {
            "type": "command",
            "command": "zellij pipe --name 'cc-deck::waiting::$ZELLIJ_PANE_ID'"
          }
        ]
      }
    ],
    "Stop": [
      {
        "matcher": "",
        "hooks": [
          {
            "type": "command",
            "command": "zellij pipe --name 'cc-deck::done::$ZELLIJ_PANE_ID'"
          }
        ]
      }
    ]
  }
}
```

## Environment Variables

Available inside hooks when running in Zellij:
- `$ZELLIJ_PANE_ID`: Numeric pane identifier (required for pipe targeting)
- `$ZELLIJ_SESSION_NAME`: Current Zellij session name
- `$CLAUDE_PROJECT_DIR`: Absolute path to the Claude project root

## Fallback Behavior

When hooks are not configured, cc-deck operates in degraded mode:
- **Working/Waiting** distinction is not available
- All sessions show either "active" (recent `PaneUpdate` events) or "idle" (no updates for timeout period)
- The `PaneUpdate` event provides pane title changes as the only activity signal
