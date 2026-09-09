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
  "profile_color": "#4FC1E9"
}
```

- `profile`: value of `CC_DECK_PROFILE` in the hook process environment. Absent when unset.
- `profile_color`: declared `color` of that profile, or the derived palette color. Absent when `profile` is absent. If the profile is not found in `config.yaml`, `profile` is still sent and `profile_color` is derived from the name.
- `agent_indicator`: the profile `icon` when declared, else the adapter indicator (unchanged).

Controller handling (`controller/hooks.rs`):
- First hook carrying `profile` for a session sets `Session.profile` and `Session.profile_color` (parsed to RGB); later hooks update `profile_color` only if it changed (color edits become visible on the next event, per spec edge case).
- On session replacement in the same pane, profile fields follow `agent_name`: reset on same-agent replacement, overwritten on cross-agent replacement.

## 2. Render broadcast (`RenderPayload`)

```json
{
  "sessions": [{ "pane_id": 12, "agent_indicator": "✳", "agent_color": [79,193,233], "…": "…" }],
  "show_agent_indicators": true,
  "profile_legend": [
    { "indicator": "✳", "color": [79,193,233], "name": "work" },
    { "indicator": "✳", "color": [236,135,192], "name": "private" }
  ]
}
```

- `show_agent_indicators` is true when the visible sessions contain more than one distinct `(agent_name, profile)` pair; `profile` is compared as `Option<String>` so "no profile" is its own value.
- `agent_color` is present only when the session has a profile; the sidebar uses it instead of `agent_indicator_color()`.
- `profile_legend` lists distinct pairs with a profile, sorted by `name`. Empty when no profiled session is visible. The sidebar help overlay renders a `Profiles` section from it.

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
