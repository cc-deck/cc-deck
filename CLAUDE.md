# cc-mux Development Guidelines

Auto-generated from all feature plans. Last updated: 2026-03-03

## Active Technologies
- Go 1.22+ + cobra (CLI), viper (config), client-go (K8s API), adrg/xdg (XDG paths), serde/yaml (config parsing) (002-cc-deck-k8s)
- XDG config file (`~/.config/cc-deck/config.yaml`) for local state; K8s PVCs for remote persistent storage (002-cc-deck-k8s)
- Go 1.22+ (existing project uses Go 1.25 in go.mod) + cobra (CLI), go:embed (binary embedding), os/exec (Zellij detection) (009-plugin-lifecycle)
- Filesystem only (WASM binary, KDL layout files) (009-plugin-lifecycle)

- Rust (stable, latest edition 2021+) + `zellij-tile` (plugin SDK), `serde`/`serde_json` (serialization) (001-cc-deck)

## Project Structure

```text
cc-zellij-plugin/   # Zellij plugin (Rust)
cc-deck/            # CLI tool (Go)
specs/              # Feature specifications
brainstorm/         # Design notes
```

## Commands

cargo test [ONLY COMMANDS FOR ACTIVE TECHNOLOGIES][ONLY COMMANDS FOR ACTIVE TECHNOLOGIES] cargo clippy

## Code Style

Rust (stable, latest edition 2021+): Follow standard conventions

## Recent Changes
- 009-plugin-lifecycle: Added Go 1.22+ (existing project uses Go 1.25 in go.mod) + cobra (CLI), go:embed (binary embedding), os/exec (Zellij detection)
- 002-cc-deck-k8s: Added Go 1.22+ + cobra (CLI), viper (config), client-go (K8s API), adrg/xdg (XDG paths), serde/yaml (config parsing)

- 001-cc-deck: Added Rust (stable, latest edition 2021+) + `zellij-tile` (plugin SDK), `serde`/`serde_json` (serialization)

<!-- MANUAL ADDITIONS START -->
<!-- MANUAL ADDITIONS END -->
