# Research: Plugin Bugfixes

**Date**: 2026-03-05
**Feature**: 010-plugin-bugfixes

## Decision: Floating picker approach

**Chosen**: Use `open_command_pane_floating` with a helper script that pipes picker data back to the plugin, since `open_plugin_pane_floating` does not exist in zellij-tile 0.43.

**Rationale**: The Zellij plugin API does not support spawning floating instances of the same plugin. A floating command pane running a lightweight script can display the picker UI and communicate the selection back via `zellij pipe`. This avoids modifying the status bar pane size which would displace terminal content.

**Alternatives considered**:
- Temporarily expand status bar pane: Works but pushes terminal content up, jarring UX. Rejected.
- `open_plugin_pane_floating`: API doesn't exist in 0.43. Rejected.
- Render picker inline in single row: Too limited, can't show session list. Rejected.

**Fallback**: If the floating command pane approach proves too complex, temporarily expand the plugin pane via size manipulation and shrink back on dismissal.

## Decision: Auto-start Claude via open_command_pane

**Chosen**: After `new_tab()`, detect the new pane via `PaneUpdate`, then call `open_command_pane_in_place` with `command="claude"`.

**Rationale**: `open_command_pane_in_place` replaces the focused pane with a command pane. After `new_tab()` creates a tab with a shell, we wait for `PaneUpdate` to identify the new pane, focus it, then replace it with Claude.

**API**:
```rust
open_command_pane_in_place(
    CommandToRun { path: "claude".into(), args: vec![], cwd: Some(cwd) },
    context,  // BTreeMap with session_id
)
```

**Alternatives considered**:
- `new_tabs_with_layout` with `command="claude"`: Silently fails in Zellij 0.43 WASM plugins. Rejected.
- Write `claude\n` to terminal: Fragile, depends on shell prompt timing. Rejected.

## Decision: Session detection via PaneInfo.title

**Chosen**: On every `PaneUpdate` event, scan all terminal panes for titles containing "claude" (case-insensitive). Auto-register untracked panes as sessions.

**Rationale**: `PaneInfo` has a `title: String` field that reflects the terminal title set by Claude Code. Case-insensitive substring matching handles variations ("claude", "Claude Code", etc.).

**Detection flow**:
1. `PaneUpdate(PaneManifest)` fires
2. Iterate `manifest.panes` (keyed by tab index)
3. For each `PaneInfo` where `!is_plugin` and `title.to_lowercase().contains("claude")`
4. If pane ID not already tracked, create session with `tab_index` from manifest key

## Decision: Tab title updates via rename_tab

**Chosen**: Call `rename_tab(tab_position, &format!("{} {}", status_icon, display_name))` on every status change.

**API**: `rename_tab(position: u32, name: &str)` where position is 0-indexed.

**Rationale**: Direct API call, low complexity. Tab position obtained from `TabUpdate(Vec<TabInfo>)` where `TabInfo.position` matches.

**Status icons**: `⚡` (working), `⏳` (waiting), `✓` (done), `💤` (idle), `?` (unknown), `✗` (exited)

## Finding: PaneManifest structure

```rust
PaneManifest { panes: HashMap<usize, Vec<PaneInfo>> }
// Key: tab index (0-indexed)
// Value: all panes in that tab
```

`PaneInfo` has 23 fields including:
- `id: u32` (pane ID)
- `title: String` (terminal title, key for detection)
- `is_plugin: bool` (filter out plugin panes)
- `is_focused: bool`
- `exited: bool`
- `terminal_command: Option<String>`

## Finding: TabInfo structure

```rust
TabInfo {
    position: usize,  // 0-indexed tab position
    name: String,      // Current tab name
    active: bool,      // Whether focused
    // + 13 more fields (viewport, swap layout, etc.)
}
```

## Finding: CommandToRun structure

```rust
CommandToRun {
    path: PathBuf,         // e.g., PathBuf::from("claude")
    args: Vec<String>,     // CLI arguments
    cwd: Option<PathBuf>,  // Working directory
}
```

## Finding: open_plugin_pane_floating does NOT exist

Searched zellij-tile 0.43.1 docs. Functions with "floating" in name:
- `open_terminal_floating`
- `open_command_pane_floating`
- `open_command_pane_floating_near_plugin`
- `open_terminal_floating_near_plugin`
- `open_file_floating`

No `open_plugin_pane_floating`. Plugin panes cannot be spawned as floating overlays.
