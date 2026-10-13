# Contract: Hook payload, render payload, dump-state and snapshot additions

**Feature**: 087-harness-profiles

All additions are optional fields. Payloads without them keep their current meaning, so an older CLI with a newer plugin (or the reverse) keeps working.

## 1. Hook pipe `cc-deck:hook` (Go `NormalizedPayload` to Rust `HookPayload`)

```json
{
  "agent": "claude",
  "agent_indicator": "✳",
  "session_id": "…",
  "pane_id": 12,
  "hook_event_name": "SessionStart",
  "profile": "work",
  "profile_color": "#5AC0E2",
  "profile_icon": "W"
}
```

- `profile`: value of `CC_DECK_PROFILE` in the hook process environment. Absent when unset.
- `profile_color`: resolved in this order (FR-014a, research R17): `CC_DECK_PROFILE_COLOR` from the hook environment when it is a valid `#RRGGBB`, without reading `config.yaml`; otherwise the declared `color` of the profile in `config.yaml` when it is a valid `#RRGGBB`; otherwise the color derived from the name (this includes a profile that `config.yaml` does not define). Absent when `profile` is absent.
- `profile_icon`: the declared profile icon: `CC_DECK_PROFILE_ICON` on the env path, the profile `icon` on the config path. Only a single printable glyph (`config.ValidIcon`) is accepted; anything else is dropped. Absent when no icon is declared. The legend reads this field.
- `agent_indicator`: the profile icon when `profile_icon` is set (it replaces the harness glyph, FR-018); else the adapter indicator (unchanged).

Controller handling (`controller/hooks.rs`):
- First hook carrying `profile` for a session sets `Session.profile`, `Session.profile_color` (parsed to RGB) and `Session.profile_icon`; later hooks update `profile_color` only if it changed. With the env path, a running session keeps the color it was launched with; color edits apply to sessions started after the next sync (spec edge case).
- On session replacement in the same pane, profile fields follow `agent_name`: reset on same-agent replacement, overwritten on cross-agent replacement.

## 2. Render broadcast (`RenderPayload`)

```json
{
  "sessions": [{ "pane_id": 12, "agent_indicator": "✳", "profile_color": [90,192,226], "…": "…" }],
  "show_agent_indicators": false,
  "profile_legend": [
    { "icon": "", "color": [226,124,90], "name": "private" },
    { "icon": "W", "color": [90,192,226], "name": "work" }
  ]
}
```

- `show_agent_indicators` is true when the visible sessions contain more than one distinct `agent_name` (pre-feature rule; profiles do not affect it, research R13).
- `profile_color` (renamed from `agent_color`) is present only when the session has a profile. The sidebar draws the edge bar from it in column 1 of both text lines (research R12). It never colors the glyph; the glyph always uses `agent_indicator_color()`.
- `profile_legend` lists distinct profile names among visible sessions, sorted by `name`; `icon` is `Session.profile_icon` when declared, else empty. Empty when no profiled session is visible. The help overlay renders each entry as `▌ <name> [icon]` with the bar in the entry color.

## 3. `cc-deck:dump-state` response

`sessions` values are full `Session` structs; the two new fields appear as:

```json
{ "profile": "work", "profile_color": [79,193,233], "agent_name": "claude", "…": "…" }
```

Go `pluginSession` reads `profile` and `agent_name`.

## 4. Snapshot file (`~/.local/state/cc-deck/sessions/<name>.json`)

```json
{
  "version": 1,
  "name": "daily",
  "saved_at": "…",
  "sessions": [
    { "tab_name": "api", "working_dir": "/…", "session_id": "…", "display_name": "api",
      "paused": false, "git_branch": "main", "agent": "claude", "profile": "work" }
  ]
}
```

- `agent` and `profile` are `omitempty`. Files written before this feature have neither and restore as `claude` without profile (FR-023).

## 5. Restore launch command

| `agent` | `profile` | Command written to the pane |
|---------|-----------|-----------------------------|
| empty or `claude` | empty | `claude --resume <id>` (or `claude` without id) |
| `claude` | `work` (exists) | `claude-work --resume <id>` |
| `claude` | `gone` (missing) | `claude --resume <id>` plus warning `profile "gone" no longer exists; restored without profile` |
| `codex` | `team` | `codex-team resume <id>` |
| `opencode` | empty | `opencode --session <id>` |

The restore-meta pipe (`cc-deck:restore-meta`, `PendingOverride`) carries `profile` and `profile_color` so the entry is colored before the first hook.
