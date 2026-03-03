# Quickstart: cc-deck Development

## Prerequisites

- Rust toolchain with WASM target: `rustup target add wasm32-wasip1`
- Zellij 0.42.0+: `cargo install zellij` or package manager
- Claude Code: `claude` binary on PATH

## Project Setup

```bash
# Clone and build
cargo build --target wasm32-wasip1 --release
```

Output: `target/wasm32-wasip1/release/cc_deck.wasm`

## Development Workflow

Use the dev layout for hot-reload:

```bash
zellij --layout zellij-dev.kdl
```

This opens a split with:
- Left: Your editor
- Right top: Build terminal
- Right bottom: cc-deck plugin (auto-loads on start)

Rebuild and reload:
```bash
cargo build --target wasm32-wasip1 && zellij action start-or-reload-plugin file:target/wasm32-wasip1/debug/cc_deck.wasm
```

## Installation

```bash
# Build release
cargo build --target wasm32-wasip1 --release

# Copy to Zellij plugin dir
mkdir -p ~/.config/zellij/plugins
cp target/wasm32-wasip1/release/cc_deck.wasm ~/.config/zellij/plugins/

# Start Zellij with the production layout
zellij --layout zellij-layout.kdl
```

See `specs/001-cc-deck/contracts/pipe-protocol.md` for configuration options.

## Testing

```bash
# Unit tests (native target)
cargo test

# Lint
cargo clippy
```

## Claude Code Hook Setup

See `specs/001-cc-deck/contracts/claude-hooks.md` for the hook configuration that enables smart status detection.
