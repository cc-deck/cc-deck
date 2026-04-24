# Brainstorm: cc-deck session save/restore

## Problem

Restarting Zellij (e.g., for plugin updates) destroys all Claude Code sessions and tab state.
No way to snapshot and restore the working environment.

## Commands

```
cc-deck session save [name]       # save current state (auto-generated name if omitted)
cc-deck session restore [name]    # restore latest (or named) snapshot
cc-deck session list              # show all saved snapshots
cc-deck session remove <name>     # delete a specific snapshot
cc-deck session remove --all      # delete all snapshots
```

## Key Decisions

| Topic | Decision |
|-------|----------|
| Command group | `cc-deck session` (top-level, future-proof for remote/pod sessions) |
| Restore scope | Tabs + working dirs + auto-start `claude --resume`, fall back to fresh if resume fails |
| Save triggers | Both: explicit `save` + auto-save as side-effect of `cc-deck hook` |
| History | Keep N (e.g., 5) most recent auto-saves + unlimited named saves |
| Restore behavior | Always create fresh tabs (no reuse of existing) |
| Restore feedback | Progress output: `Creating tab 1/5: cc-deck...` |
| State location | `~/.config/cc-deck/sessions/` (XDG-conformant) |
| Cleanup | `remove` and `remove --all` for managing saved snapshots |

## State File Format

```json
{
  "version": 1,
  "saved_at": "2026-03-10T14:30:00Z",
  "sessions": [
    {
      "tab_name": "cc-deck",
      "working_dir": "/Users/rhuss/Development/ai/mcp/cc-deck",
      "session_id": "afec0bb6-...",
      "display_name": "cc-deck",
      "paused": false,
      "git_branch": "main"
    }
  ]
}
```

## Auto-save Mechanism

The `cc-deck hook` command already fires on every Claude event.
Add a side-effect: after forwarding the event to the plugin via `zellij pipe`,
also query the plugin for current state and write to
`~/.config/cc-deck/sessions/auto-<N>.json` (rolling, keep latest 5).

## Restore Flow

1. Read state file (latest or named)
2. For each session in tab order:
   - `zellij action new-tab` (creates tab with cc-deck layout template)
   - `cd <working_dir>` via `zellij action write-chars`
   - `claude --resume <session_id>` (fall back to `claude` on failure)
   - Print progress: `Creating tab 2/5: llama-stack-k8s-operator...`
3. Switch to first restored tab
