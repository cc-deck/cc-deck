# Data Model: Virtual Sort Fix

## Entities

### ControllerState (modified)

New field added to existing struct at `cc-zellij-plugin/src/controller/state.rs`:

| Field | Type | Description |
|-------|------|-------------|
| `sort_active` | `bool` | Whether virtual sort is currently active. Default `false`. |

### RenderPayload (modified)

New field added to existing struct at `cc-zellij-plugin/src/lib.rs`:

| Field | Type | Description |
|-------|------|-------------|
| `sort_active` | `bool` | Whether the sessions list is sorted by activity tier. Sidebars use this to show a sort indicator. |

### Sort Tiers (unchanged)

Existing tier classification from `sort_tier()` at `actions.rs:265`:

| Tier | Value | Activity States |
|------|-------|----------------|
| Active | 0 | Working, Waiting |
| Inactive | 1 | Idle, Done, AgentDone, Init |
| Paused | 2 | Any activity with `session.paused == true` |

## State Transitions

```
sort_active = false  --[S key pressed]--> sort_active = true
sort_active = true   --[S key pressed]--> sort_active = false
sort_active = true   --[tab count changes]--> sort_active = false
```

## Data Flow

```
Sidebar: S key → ActionType::Sort → Controller
Controller: toggle sort_active → mark_render_dirty()
Controller: build_render_payload() → sort by (tier, tab_index) if sort_active, else tab_index
Controller: broadcast RenderPayload (with sort_active flag) → Sidebars
Sidebar: render with sort indicator if payload.sort_active
```
