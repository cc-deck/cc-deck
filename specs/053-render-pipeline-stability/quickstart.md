# Quickstart: Render Pipeline Stability

## What this feature does

Fixes sidebar flickering, activity indicator blinking, and high CPU usage caused by a phantom second controller instance and excessive render broadcasts. Adds profiling instrumentation for ongoing performance monitoring.

## Key files to modify

### Controller side
- `cc-zellij-plugin/src/controller/mod.rs` - Add disabled guard, startup probe handling
- `cc-zellij-plugin/src/controller/events.rs` - Conditional mark_render_dirty in handle_tab_update
- `cc-zellij-plugin/src/controller/render_broadcast.rs` - Add profiling instrumentation
- `cc-zellij-plugin/src/controller/sidebar_registry.rs` - Push-on-discovery: send render to newly registered sidebars
- `cc-zellij-plugin/src/controller/state.rs` - Add `disabled` field

### Sidebar side
- `cc-zellij-plugin/src/sidebar_plugin/state.rs` - Add `render_request_sent`, `ticks_since_init` fields
- `cc-zellij-plugin/src/sidebar_plugin/mod.rs` - Handle timer tick for render request fallback, "Connecting..." display

### Shared
- `cc-zellij-plugin/src/pipe_handler.rs` - Add ControllerPing, ControllerPong, RenderRequest variants
- `cc-zellij-plugin/src/debug.rs` - Buffered logging
- `cc-zellij-plugin/src/perf.rs` - New render event labels

## Build and test

```bash
make test          # Run all tests (Rust + Go)
make lint          # Clippy + go vet
make install       # Build and install plugin
```

## Manual verification

```bash
# Enable debug logging before starting Zellij
CACHE="$HOME/Library/Caches/org.Zellij-Contributors.Zellij/file:$HOME/.config/zellij/plugins/cc_deck.wasm/plugin_cache"
touch "$CACHE/debug_enabled"

# Start Zellij, open 14 tabs
# Check for single controller:
rg "CTRL PIPE" "$CACHE/debug.log" | rg -o 'plugin_id=\d+' | sort -u
# Should show exactly one plugin_id

# Check CPU:
top -pid $(pgrep -f 'zellij.*server') -stats pid,cpu,threads
# Should be under 30%
```
