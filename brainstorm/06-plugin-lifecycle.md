# Brainstorm 06: Plugin Lifecycle Management

Date: 2026-03-04
Status: complete

## Problem

The Zellij plugin (cc-zellij-plugin) and the Go CLI (cc-deck) are independently useful but have no integration path. Users must manually compile the Rust WASM binary, copy it to the right directory, and configure Zellij layouts. This friction kills adoption.

## Concept

The Go CLI embeds the compiled WASM binary at build time and provides `plugin install`, `plugin status`, and `plugin remove` commands that handle the full lifecycle: placing the binary, configuring Zellij layouts, and cleaning up on removal.

## Design Decisions

### Distribution: Go embed (build-time)

The WASM binary is compiled separately (Rust cross-compilation to wasm32-wasip1), then embedded into the Go binary via `//go:embed`. This produces a single self-contained binary.

Build pipeline: `cargo build --target wasm32-wasip1 --release` in cc-zellij-plugin, then `go build` in cc-deck with the WASM artifact available for embedding.

### Install location

WASM binary goes to `~/.config/zellij/plugins/cc_deck.wasm`, the standard Zellij plugin directory.

### Layout strategy

- **Default**: Install a minimal `cc-deck.kdl` layout file that adds only the status bar plugin pane. Users launch with `zellij --layout cc-deck`.
- **Optional**: `plugin install --layout full` installs an opinionated layout with sensible defaults for Claude Code sessions.
- **Inject mode**: `plugin install --inject-default` modifies the user's existing default Zellij layout to include the plugin. Reversible on `plugin remove`.

### Auto-start mechanism

The installed layout file references the plugin at `file:~/.config/zellij/plugins/cc_deck.wasm`. When `--inject-default` is used, the same plugin pane block is appended to the user's default layout.

### Version management

Simple overwrite-always strategy. `plugin install` writes the embedded version to disk. If a file already exists, prompt for confirmation (skip with `--force`). No side-by-side versioning.

### Scope: local only

Plugin commands manage the local Zellij installation only. K8s containers have the plugin baked into their container image.

## Command Design

### `cc-deck plugin install`

Flags:
- `--force`: Overwrite existing installation without prompting
- `--layout {minimal|full}`: Layout template to install (default: minimal)
- `--inject-default`: Also modify the default Zellij layout to include the plugin

Actions:
1. Write embedded WASM to `~/.config/zellij/plugins/cc_deck.wasm`
2. Write layout file to `~/.config/zellij/layouts/cc-deck.kdl`
3. If `--inject-default`: parse default layout, inject plugin pane, write back
4. Print summary of installed files and usage instructions

### `cc-deck plugin status`

Rich output including:
- Installed: yes/no, path, file size, embedded version
- Zellij version compatibility check (detect installed Zellij, compare against plugin SDK version)
- Layout files: which cc-deck layouts are installed, whether default layout is injected
- Running instances: detect if the plugin is currently loaded in any Zellij session

### `cc-deck plugin remove`

Actions:
1. Remove `~/.config/zellij/plugins/cc_deck.wasm`
2. Remove `~/.config/zellij/layouts/cc-deck.kdl`
3. If default layout was injected: parse it, remove the cc-deck plugin pane block, write back
4. Print summary of removed files

Full cleanup: undoes everything `plugin install` did, including default layout modifications.

## Build Pipeline Impact

The Go build now depends on a prior Rust compilation step:

```
# 1. Build WASM
cd cc-zellij-plugin
cargo build --target wasm32-wasip1 --release

# 2. Copy artifact to Go embed location
cp target/wasm32-wasip1/release/cc_deck.wasm ../cc-deck/internal/plugin/

# 3. Build Go CLI
cd ../cc-deck
go build ./cmd/cc-deck
```

A Makefile or Taskfile at the repo root should orchestrate this.

## File Structure (new in cc-deck/)

```
cc-deck/
  internal/
    plugin/
      embed.go          # //go:embed cc_deck.wasm
      cc_deck.wasm      # Compiled artifact (gitignored)
      install.go        # Install logic
      status.go         # Status detection
      remove.go         # Removal + cleanup
      layout.go         # Layout file generation/injection
    cmd/
      plugin.go         # Cobra command definitions
```

## Open Questions

- Should the Makefile/Taskfile live at repo root or in cc-deck/?
- Exact Zellij version compatibility matrix (which zellij-tile SDK versions work with which Zellij releases)
- Whether to add a `plugin upgrade` alias for `plugin install --force`
