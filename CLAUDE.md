# cc-mux Development Guidelines

Auto-generated from all feature plans. Last updated: 2026-03-03

## Active Technologies
- Go 1.22+ + cobra (CLI), viper (config), client-go (K8s API), adrg/xdg (XDG paths), serde/yaml (config parsing) (002-cc-deck-k8s)
- XDG config file (`~/.config/cc-deck/config.yaml`) for local state; K8s PVCs for remote persistent storage (002-cc-deck-k8s)
- Go 1.22+ (existing project uses Go 1.25 in go.mod) + cobra (CLI), go:embed (binary embedding), os/exec (Zellij detection) (009-plugin-lifecycle)
- Filesystem only (WASM binary, KDL layout files) (009-plugin-lifecycle)
- Rust (stable, wasm32-wasip1 target) with zellij-tile 0.43.1 + zellij-tile 0.43 (plugin SDK), serde/serde_json (serialization) (010-plugin-bugfixes)
- WASI `/cache/` directory for persistent state (recent.json) (010-plugin-bugfixes)
- Rust stable (edition 2021, wasm32-wasip1 target) for plugin; Go 1.22+ for CLI + zellij-tile 0.43.1 (plugin SDK), serde/serde_json 1.x; cobra (CLI), encoding/json (Go stdlib) (012-sidebar-plugin)
- WASI `/cache/` directory for plugin state; filesystem for installation artifacts (012-sidebar-plugin)

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
- 012-sidebar-plugin: Added Rust stable (edition 2021, wasm32-wasip1 target) for plugin; Go 1.22+ for CLI + zellij-tile 0.43.1 (plugin SDK), serde/serde_json 1.x; cobra (CLI), encoding/json (Go stdlib)
- 010-plugin-bugfixes: Added Rust (stable, wasm32-wasip1 target) with zellij-tile 0.43.1 + zellij-tile 0.43 (plugin SDK), serde/serde_json (serialization)
- 009-plugin-lifecycle: Added Go 1.22+ (existing project uses Go 1.25 in go.mod) + cobra (CLI), go:embed (binary embedding), os/exec (Zellij detection)


<!-- MANUAL ADDITIONS START -->
<!-- MANUAL ADDITIONS END -->
