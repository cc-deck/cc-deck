# Research: Sidebar Auto-Sort with Pause Zone

## Codebase Analysis

### Session Ordering Path

`build_render_payload()` in `controller/render_broadcast.rs` is the single point where session order is determined. It collects all sessions, sorts them (by `sort_order` if present, else by `tab_index`), then maps to `RenderSession` structs. The resulting `Vec<RenderSession>` is sent to the sidebar as JSON via pipe.

### Existing Sort Mechanism

The manual S keybinding calls `handle_sort()` in `controller/actions.rs`, which:
1. Collects sessions with tab indices
2. Sorts by `(sort_tier, last_event_ts)` (tier 0=Working/Waiting, 1=Idle/Done/Init, 2=Paused)
3. Stores the resulting pane ID order in `state.sort_order`
4. `build_render_payload()` picks up `sort_order` and uses it instead of tab_index

The manual sort is a frozen snapshot. It does not auto-update when session states change. The S key can be pressed again to re-sort, or to toggle off (second press clears `sort_order`).

### Config System

`PluginConfig` in `config.rs` reads KDL layout parameters via `from_configuration(&BTreeMap<String, String>)`. The pattern for adding a boolean config: parse string value, default to a sensible value if missing. `sidebar_width` is the existing example for integer config. Boolean parsing follows the same pattern: `config.get("auto_sort").map(|v| v == "true").unwrap_or(true)`.

### RenderPayload Structure

```rust
pub struct RenderPayload {
    pub sessions: Vec<RenderSession>,
    // ... summary counts, config values
}

pub struct RenderSession {
    pub pane_id: u32,
    pub display_name: String,
    pub activity_label: String,
    pub indicator: String,
    pub color: String,
    pub git_branch: Option<String>,
    pub tab_index: usize,
    pub paused: bool,         // already present
    pub done_attended: bool,
    pub badges: Vec<String>,
    pub agent_indicator: Option<String>,
    pub in_worktree: bool,
}
```

### Sidebar Rendering

The sidebar (`sidebar/mod.rs` or `sidebar/render.rs`) iterates over `payload.sessions` and renders each entry. Adding a separator means checking if `separator_after_index` is set and rendering a horizontal line after the corresponding session entry.

### Key Files to Modify

| File | Change |
|------|--------|
| `cc-zellij-plugin/src/config.rs` | Add `auto_sort: bool` field, parsing, default, tests |
| `cc-zellij-plugin/src/lib.rs` | Add `separator_after_index: Option<usize>` to `RenderPayload` |
| `cc-zellij-plugin/src/controller/render_broadcast.rs` | Apply active/paused partition in `build_render_payload()` |
| `cc-zellij-plugin/src/controller/actions.rs` | Update `handle_sort()` to only sort within active zone when auto-sort is on |
| `cc-zellij-plugin/src/sidebar/` | Render separator line at the indicated index |
