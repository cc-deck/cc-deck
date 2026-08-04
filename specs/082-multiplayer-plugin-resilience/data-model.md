# Data Model: Multiplayer Plugin Resilience

## Modified Entities

### SidebarHello (protocol message)

Current:
```
plugin_id: u32
```

New:
```
plugin_id: u32
client_id: u16 (optional, default 0, #[serde(default)])
```

### SidebarRegistry (controller state)

Current:
```
sidebar_registry: HashMap<u32, usize>  // plugin_id -> tab_index
```

New:
```
sidebar_registry: HashMap<u32, (usize, u16)>  // plugin_id -> (tab_index, client_id)
```

### ControllerState (new field)

```
client_id: u16  // controller's own client_id, set during permission grant
```

### Election Ping (pipe message payload)

Current payload format:
```
"{plugin_id}"       // e.g., "42"
```

New payload format:
```
"{client_id}:{plugin_id}"   // e.g., "1:42"
```

Backward compatibility: if parsing fails to find `:`, treat the entire payload as `plugin_id` and assume `client_id = 0`.

## Invariants

1. Each `(tab_index, client_id)` pair has at most one entry in the registry
2. The controller's own `client_id` is set once during permission grant and never changes
3. Render broadcasts only target entries where `client_id == state.client_id`
4. The election uses `(client_id, plugin_id)` lexicographic ordering (lowest wins)
5. `client_id = 0` is a valid value (not a sentinel for "unknown" or "orphan")
