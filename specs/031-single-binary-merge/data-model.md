# Data Model: Single Binary Merge (031)

This feature is a build/architecture refactoring. No new data entities are introduced. The existing data model is preserved unchanged.

## Existing Entities (Unchanged)

### Plugin Mode (modified scope)

Currently a compile-time concept (feature flags). Becomes a runtime concept.

- **Values**: `"controller"`, `"sidebar"` (default if absent)
- **Source**: KDL configuration `mode` key, read via `configuration.get("mode")` in `load()`
- **Lifecycle**: Set once at plugin load, immutable for the instance's lifetime

### UnifiedPlugin (new, wraps existing)

An enum that delegates to the existing `ControllerPlugin` or `SidebarRendererPlugin` based on the runtime mode.

- **Variants**: `Controller(ControllerPlugin)`, `Sidebar(SidebarRendererPlugin)`
- **Determined by**: `configuration.get("mode")` at `load()` time
- **Default**: `Sidebar` when mode is absent or unrecognized

### Existing Entities (no changes)

- **Session**: `BTreeMap<u32, Session>` owned by controller
- **RenderPayload**: JSON broadcast from controller to sidebars
- **ActionMessage**: JSON from sidebar to controller
- **SidebarHello/SidebarInit**: Discovery protocol messages
- **PluginConfig**: Parsed from KDL configuration, shared across modes

## State Transitions

```
Plugin loads
    |
    v
Read configuration.get("mode")
    |
    +-- "controller" --> Initialize ControllerPlugin
    |                     Subscribe: PaneUpdate, TabUpdate, Timer, ...
    |                     set_selectable(false)
    |
    +-- "sidebar" (or absent) --> Initialize SidebarRendererPlugin
                                   Subscribe: Mouse, Key, PermissionRequestResult
```

No state transitions change within the controller or sidebar after initialization. The mode is fixed for the lifetime of the instance.
