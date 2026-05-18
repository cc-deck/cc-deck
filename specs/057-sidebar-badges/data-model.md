# Data Model: Configurable Sidebar Badges

## Entities

### BadgeRule (Go config)

Defined in `~/.config/cc-deck/config.yaml` under the `badges:` key.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| name | string | yes | Unique identifier for the badge rule |
| file | string | yes | Path to state file, relative to session's working_dir |
| format | string | yes | File format, must be "json" |
| extract | string | yes | Dot-path expression (e.g., `.mode`, `.result.outcome`) |
| values | map[string]string | yes | Mapping of extracted values to emoji strings |
| default | string | no | Fallback emoji when extracted value has no mapping |

### Resolved Badge (payload transport)

Transported as a simple `[]string` (list of emoji strings) in the hook payload. No structured object needed since all evaluation happens on the CLI side.

### Session Badge State (Rust plugin)

Stored as `Vec<String>` on the `Session` struct. Updated on each hook event. Copied to `RenderSession` for rendering.

## Data Flow

```
config.yaml          hook.go              HookPayload         Session           RenderSession
┌──────────┐    ┌──────────────┐    ┌──────────────┐    ┌────────────┐    ┌──────────────┐
│ badges:  │───>│ evaluate()   │───>│ badges: []   │───>│ badges: [] │───>│ badges: []   │
│  - name  │    │ read file    │    │ ["🚢","✅"]  │    │ ["🚢","✅"]│    │ ["🚢","✅"]  │
│    file   │    │ extract path │    └──────────────┘    └────────────┘    └──────────────┘
│    values │    │ map to emoji │
└──────────┘    └──────────────┘
```

## JSON Dot-Path Specification

The `extract` field uses a simple dot-path notation:

- Must start with `.`
- Segments separated by `.`
- Each segment is a JSON object key
- No array indexing, wildcards, or filters

Examples:
- `.mode` extracts `"ship"` from `{"mode": "ship"}`
- `.result.outcome` extracts `"pass"` from `{"result": {"outcome": "pass"}}`
- `.a.b.c` extracts `"deep"` from `{"a": {"b": {"c": "deep"}}}`

Non-string leaf values are converted to their string representation for matching against the `values` map.
