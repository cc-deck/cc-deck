# Quickstart: cc-deck Sidebar Plugin

**Feature**: 012-sidebar-plugin

## Prerequisites

- Zellij 0.42.0+ installed
- Go 1.22+ (for building cc-deck CLI)
- Rust stable with `wasm32-wasip1` target (`rustup target add wasm32-wasip1`)
- Claude Code installed

## Build

```bash
# Build WASM plugin (release)
cd cc-zellij-plugin
cargo build --target wasm32-wasip1 --release

# Build Go CLI
cd cc-deck
go build -o cc-deck ./cmd/cc-deck/
```

## Install

```bash
# Install plugin (WASM binary, layout, hooks, settings.json)
./cc-deck/cc-deck plugin install

# Verify installation
./cc-deck/cc-deck plugin status
```

## Run

```bash
# Start Zellij with cc-deck layout
zellij --layout cc-deck

# Or set as default layout in config.kdl:
# default_layout "cc-deck"
```

## Test Hook Manually

```bash
# Simulate a hook event (in a Zellij pane)
echo '{"session_id":"test","hook_event_name":"PreToolUse","tool_name":"Bash","cwd":"/tmp"}' | ZELLIJ_PANE_ID=$ZELLIJ_PANE_ID ./cc-deck/cc-deck hook --pane-id "$ZELLIJ_PANE_ID"
```

## Uninstall

```bash
./cc-deck/cc-deck plugin remove
```
