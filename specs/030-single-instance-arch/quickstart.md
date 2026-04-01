# Quickstart: Single-Instance Architecture

## Build

From project root (never from subdirectories):

```bash
make install
```

This builds both WASM binaries (controller + sidebar), copies them to the Go embed location, builds the CLI, and runs `cc-deck plugin install --force`.

## Verify Installation

```bash
cc-deck plugin status
```

Should show both `cc_deck_controller.wasm` and `cc_deck_sidebar.wasm` installed in `~/.config/zellij/plugins/`.

## Test

```bash
# Rust unit tests (native target, WASM stubs)
make test-rust

# Go CLI tests
make test-go

# Full test suite
make test
```

## Launch

```bash
# Kill existing sessions (they cache old plugin binaries)
zellij kill-all-sessions -y 2>/dev/null

# Start with cc-deck layout
zellij --layout cc-deck
```

## Development Cycle

```bash
# 1. Edit Rust code in cc-zellij-plugin/src/
# 2. Build and install
make install

# 3. Kill sessions and relaunch
zellij kill-all-sessions -y 2>/dev/null
zellij --layout cc-deck
```

## Debugging

Enable debug logging for the controller:
```bash
touch ~/Library/Caches/org.Zellij-Contributors.Zellij/plugins/cc_deck_controller/cache/debug_enabled
```

View controller logs:
```bash
tail -f ~/Library/Caches/org.Zellij-Contributors.Zellij/plugins/cc_deck_controller/cache/debug.log
```

## Key Architecture Points

- **Controller** (`cc_deck_controller.wasm`): Background plugin, single instance, processes all events and state
- **Sidebar** (`cc_deck_sidebar.wasm`): One per tab, renders cached payloads, handles local interaction
- **Communication**: Pipe messages with broadcast+filter pattern
- **No sync**: Single source of truth in controller, no merge conflicts
