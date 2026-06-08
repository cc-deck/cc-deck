# Data Model: Sidebar Session Sort by Activity

## Entities

### SortTier (derived, not persisted)

A classification computed from session state for sorting purposes.

| Tier | Priority | Condition | Description |
|------|----------|-----------|-------------|
| Active | 0 (highest) | `!paused && (activity == Working \|\| activity == Waiting)` | Sessions actively doing work or waiting for user input |
| Inactive | 1 | `!paused && activity in {Idle, Done, AgentDone, Init}` | Sessions not currently active |
| Paused | 2 (lowest) | `paused == true` | Sessions explicitly paused by user, regardless of activity |

### ActionType Extension

New variant added to the `ActionType` enum in `cc-zellij-plugin/src/lib.rs`:

```
Sort  // Triggers activity-based tab reorder
```

No payload fields needed (pane_id, tab_index, value are all None). The controller uses its own session state to compute the sort.

## State Transitions

```
User in Navigate mode
  -> presses S (Shift+s)
  -> sidebar sends ActionType::Sort to controller
  -> controller computes target order (stable partition by tier)
  -> controller executes swap sequence via move_focus_or_tab
  -> controller broadcasts render update
  -> sidebar re-renders with new session order
  -> sidebar updates cursor_index to follow the same pane_id
```

No persistent state changes. Tab positions are updated in Zellij (which is the source of truth for tab order via TabInfo events).
