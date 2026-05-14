# Research: Render Pipeline Stability and CPU Optimization

**Date**: 2026-05-14
**Branch**: 053-render-pipeline-stability

## R1: Why does a second controller instance appear?

**Investigation**: The `UnifiedPlugin` in `main.rs` determines its role based on the `mode` configuration key. Instances loaded via `load_plugins` with `mode "controller"` become controllers; instances loaded via layout template (without explicit mode or with `mode "sidebar"`) become sidebars.

**Finding**: The second "controller" (plugin_id=0) is most likely a sidebar instance that receives broadcast pipe messages intended for the controller. When Zellij sends untargeted pipe messages (e.g., `cc-deck:hook` from CLI), ALL plugin instances receive them. Sidebars parse these messages in their `pipe()` handler. If a sidebar processes controller-specific messages, it could exhibit controller-like behavior in the logs.

However, debug logs show `CTRL PIPE` and `CTRL SIDEBAR` prefixes from plugin_id=0, which are controller-specific log lines. This means plugin_id=0 IS running controller code. The most likely cause: `load_plugins` in `config.kdl` plus a layout entry for the same WASM URL without explicit mode creates two instances where one gets `mode "controller"` and the other gets no mode (defaulting to sidebar), BUT receives permission grant before the actual controller.

**Decision**: Implement defensive guard (FR-007) as the primary fix. Add diagnostic logging to capture plugin_id at load time to confirm the root cause. The guard is correct regardless of the exact cause.

**Alternatives considered**:
- Zellij bug report: Out of scope per spec, and the guard is needed regardless
- Separate WASM binary: Overkill; single-binary architecture is a design goal

## R2: WASI cache directory interactions

**Finding**: All plugin instances from the same WASM URL share `/cache/`. This is by Zellij design. The `debug.log`, `perf.csv`, and `sessions-{pid}.json` files are all in this shared directory. Two controllers writing to `debug.log` simultaneously explains interleaved log lines but does not cause state corruption (append-only).

**Decision**: Cache sharing is not a root cause of rendering issues. It only affects log readability. No changes needed beyond adding plugin_id to log prefix for disambiguation.

**Alternatives considered**:
- Instance-scoped log files: Would complicate debugging by splitting logs

## R3: Debug logging optimization

**Finding**: Current `debug_log()` opens the file, writes, and closes on every call. With 14 tabs generating events, this creates significant I/O. The `debug_init()` checks the flag file once at load, which is correct.

**Decision**: Buffer debug writes. Accumulate log lines in memory and flush periodically (on timer tick or when buffer exceeds a threshold). This reduces file I/O from N writes/second to 1 write/second.

**Alternatives considered**:
- Reduce log verbosity: Would lose diagnostic value
- Async I/O: Not available in WASI

## R4: Profiling instrumentation gaps

**Finding**: Current `PerfTracker` records `gauge:sessions`, `gauge:tabs`, `gauge:sidebars`. It does not track render broadcast count, serialization time, or pipe delivery count. These are the metrics needed to detect render pipeline regressions.

**Decision**: Add three new perf events:
- `render:broadcast` (count + serialization time)
- `render:pipe_send` (count per sidebar)
- `render:skipped` (count of flush_render calls where dirty was false)

**Alternatives considered**:
- Separate profiling system: Unnecessary; PerfTracker is well-suited

## R5: Sidebar bootstrapping with push-on-discovery

**Finding**: `discover_sidebars_from_manifest()` runs on every `handle_pane_update()`. It registers new sidebars but does not send them an initial render payload. Adding a targeted render on first registration is straightforward: check if the plugin_id was newly inserted, and if so, call `send_render_to_plugin`.

**Decision**: In `discover_sidebars_from_manifest`, track newly registered sidebar IDs and return them. The caller (handle_pane_update) triggers a targeted render for each new sidebar.

**Alternatives considered**:
- Immediate broadcast_render: Would re-serialize for all sidebars, not just new ones
- Separate "initial render" flag: More complex, same result

## R6: Conditional handle_tab_update

**Finding**: Currently `handle_tab_update` calls `mark_render_dirty()` unconditionally at line 51 of events.rs. The function already tracks `tab_count_changed` and runs `remove_dead_sessions` and `cleanup_stale_sessions`. The only missing comparison is active tab (focus) change.

**Decision**: Replace unconditional `mark_render_dirty()` with conditional: mark dirty only when active tab changed, tab count changed, sessions were removed, or stale sessions transitioned. The rebuild_pane_map already handles focus tracking.

**Alternatives considered**:
- Remove mark_render_dirty entirely from handle_tab_update: Too aggressive; tab changes do affect sidebar display
