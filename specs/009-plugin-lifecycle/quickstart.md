# Quickstart: Plugin Lifecycle Management

**Feature**: 009-plugin-lifecycle

## Prerequisites

- Go 1.22+ installed
- Rust toolchain with `wasm32-wasip1` target (`rustup target add wasm32-wasip1`)
- Zellij 0.40+ installed

## Build

```bash
# 1. Build the WASM plugin
cd cc-zellij-plugin
cargo build --target wasm32-wasip1 --release

# 2. Copy artifact to embed location
mkdir -p ../cc-deck/internal/plugin
cp target/wasm32-wasip1/release/cc_deck.wasm ../cc-deck/internal/plugin/

# 3. Build the Go CLI
cd ../cc-deck
go build -o cc-deck ./cmd/cc-deck
```

## Usage

```bash
# Install the plugin (minimal layout)
./cc-deck plugin install

# Install with full layout
./cc-deck plugin install --layout full

# Install and inject into default Zellij layout
./cc-deck plugin install --inject-default

# Check status
./cc-deck plugin status

# Check status as JSON
./cc-deck plugin status -o json

# Remove everything
./cc-deck plugin remove
```

## Development

```bash
# Run tests
cd cc-deck
go test ./internal/plugin/...

# Test install to a custom directory (avoids touching real Zellij config)
ZELLIJ_CONFIG_DIR=/tmp/test-zellij ./cc-deck plugin install
```
