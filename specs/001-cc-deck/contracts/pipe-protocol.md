# Contract: Pipe Message Protocol

cc-deck communicates with Claude Code hooks and between plugin instances via Zellij's pipe system.

## Inbound Messages (from Claude Code hooks)

Format: `cc-deck::EVENT_TYPE::PANE_ID`

| EVENT_TYPE | Source Hook | Meaning |
|------------|------------|---------|
| `working` | `UserPromptSubmit`, `PreToolUse` | Claude is actively working |
| `waiting` | `Notification`, `PermissionRequest` | Claude needs user input |
| `done` | `Stop` | Claude finished responding |

### Parsing Rules
1. Message must start with `cc-deck::` prefix (check both `pipe_message.name` and `pipe_message.payload`)
2. Split on `::` to get exactly 3 parts
3. EVENT_TYPE is case-insensitive
4. PANE_ID must parse as u32
5. Unknown EVENT_TYPEs are silently ignored (forward compatibility)
6. Malformed messages are silently ignored (no crash)

### Delivery
Messages are sent via `zellij pipe --name "cc-deck::EVENT::PANE_ID"` (broadcast pipe).
The plugin receives them in its `pipe()` method.

## Internal Messages (between plugin instances)

For communication between the status bar instance and picker instance:

| Message Name | Payload | Direction | Meaning |
|-------------|---------|-----------|---------|
| `cc-deck::session_list` | JSON session list | bar -> picker | Current sessions for picker display |
| `cc-deck::switch_to` | pane_id as string | picker -> bar | User selected a session |

## Zellij KDL Configuration Contract

```kdl
plugin location="file:~/.config/zellij/plugins/cc-deck.wasm" {
    // All optional, defaults shown
    idle_timeout "300"
    picker_key "Ctrl Shift t"
    new_session_key "Ctrl Shift n"
    rename_key "Ctrl Shift r"
    close_key "Ctrl Shift x"
    max_recent "20"
}
```

All values are strings (Zellij plugin config is `BTreeMap<String, String>`).
