# Quickstart: cc-deck Sidebar Plugin

**Feature**: 012-sidebar-plugin

## Prerequisites

- Zellij 0.42.0+ installed
- Go 1.22+ (for building cc-deck CLI)
- Rust stable with `wasm32-wasip1` target (`rustup target add wasm32-wasip1`)
- Claude Code installed

## Build

```bash
# Build everything (WASM plugin + Go CLI)
make build

# Or step by step:
make build-wasm    # Build Rust WASM plugin
make copy-wasm     # Copy WASM to Go embed location
make build-cli     # Build Go CLI
```

## Install

```bash
# Install plugin, layout, and hooks
./cc-deck/cc-deck install

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

## Development

```bash
# Build debug WASM and reload in running Zellij
make reload

# Run tests
make test

# Run linters
make lint
```

## Test Hook Manually

```bash
# Simulate a hook event (in a Zellij pane)
echo '{"session_id":"test","hook_event":"PreToolUse","tool_name":"Bash","cwd":"/tmp"}' | ZELLIJ_PANE_ID=$ZELLIJ_PANE_ID ./cc-deck/cc-deck hook
```

## Uninstall

```bash
./cc-deck/cc-deck uninstall
```
