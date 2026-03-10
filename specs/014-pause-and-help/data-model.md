# Data Model: 014-pause-and-help

## Entities

### Session (Modified)

Add one new field:

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| paused | bool | false | When true, session is excluded from attend cycling and shown with ⏸ icon + dimmed name |

### PluginState (Modified)

Add one new field:

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| show_help | bool | false | When true, sidebar renders the help overlay instead of session list |
