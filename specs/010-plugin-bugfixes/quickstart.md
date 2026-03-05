# Quickstart: Plugin Bugfixes

**Feature**: 010-plugin-bugfixes

## Prerequisites

- Rust toolchain with `wasm32-wasip1` target
- Zellij 0.40+ installed
- cc-deck CLI built (from 009-plugin-lifecycle)

## Build and Test

```bash
# Build the WASM plugin
cd cc-zellij-plugin
cargo build --target wasm32-wasip1 --release

# Install plugin with full layout
cd ..
make build
./cc-deck/cc-deck plugin install --force --layout full

# Start Zellij
zellij --layout cc-deck
```

## Testing Each Fix

### Tab Titles
```bash
# Create a session, watch tab title in tab bar
zellij pipe --name new_session
# Tab should show "? cc-0" initially, then update to project name

# Simulate status change
zellij pipe --name "cc-deck::working::PANE_ID"
# Tab title should update to "⚡ project-name"
```

### Session Detection
```bash
# Start claude manually in a new tab
zellij action new-tab
claude
# Plugin status bar should detect and show the session within 5 seconds
```

### Auto-Start
```bash
# Create a session via plugin
zellij pipe --name new_session
# Claude should start automatically in the new tab
```

### Floating Picker
```bash
# Open picker
zellij pipe --name open_picker
# A floating overlay should appear with session list
# Type to filter, Enter to select, Escape to dismiss
```

### Automated Tests
```bash
# Run the full test suite
./smoke_test.sh
# Should complete without manual intervention
```

## Development Workflow

```bash
# Hot-reload cycle
cd cc-zellij-plugin
cargo build --target wasm32-wasip1
# Copy to plugins dir
cp target/wasm32-wasip1/debug/cc_deck.wasm ~/.config/zellij/plugins/
# Reload in running Zellij
zellij action start-or-reload-plugin file:~/.config/zellij/plugins/cc_deck.wasm
```
