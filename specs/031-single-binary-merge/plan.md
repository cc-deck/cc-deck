# Implementation Plan: Single Binary Merge

**Branch**: `031-single-binary-merge` | **Date**: 2026-04-01 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/031-single-binary-merge/spec.md`

## Summary

Merge the two WASM binaries (`cc_deck_controller.wasm` + `cc_deck_sidebar.wasm`) into a single `cc_deck.wasm` that determines its role from a `mode` configuration parameter at runtime. Adopt named pipe channels (`cc-deck:ctrl:*` / `cc-deck:side:*`) for self-documenting message routing. Remove legacy code (sync.rs, PluginState, old tests). Simplify the build pipeline and Go CLI to handle a single binary.

## Technical Context

**Language/Version**: Rust stable (edition 2021, wasm32-wasip1 target) + Go 1.25
**Primary Dependencies**: zellij-tile 0.44 (Cargo.toml), serde/serde_json 1.x, cobra (CLI). Note: Go CLI's `EmbeddedPlugin().SDKVersion` should be updated from "0.43" to "0.44" to match the actual dependency
**Storage**: WASI `/cache/` for plugin state
**Testing**: `cargo test` (Rust, native target), `go test` (Go)
**Target Platform**: WASM (wasm32-wasip1) running inside Zellij 0.44
**Project Type**: Plugin (WASM binary) + CLI tool (Go binary)

## Constitution Check

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Two-Component Architecture | PASS | Both Rust plugin and Go CLI are modified |
| II. Plugin Installation | PASS | Will use `make install` exclusively |
| III. WASM Filename Convention | PASS | Output remains `cc_deck.wasm` (underscore) |
| IV. WASM Host Function Gating | PASS | All `#[cfg(target_family = "wasm")]` guards retained |
| V. Zellij API Research Order | PASS | Pipe targeting verified against Zellij 0.44 source in brainstorm |
| VI. Build via Makefile Only | PASS | Single `build-wasm` target via Makefile |
| VII. Interface Behavioral Contracts | N/A | No new interface implementations |
| VIII. Simplicity | PASS | Removing abstractions (feature flags, dual builds) |
| IX. Documentation Freshness | PASS | README update in scope (T026) |
| X. Spec Tracking in README | PASS | Feature spec table update in T026 |
| XII. Prose Plugin | PASS | T026 mandates prose plugin with cc-deck voice |
| XIII. XDG Paths | N/A | No path changes in this feature |
| XIV. No Dotfile Nesting | N/A | No dotfile changes |

## Project Structure

### Source Code (changes)

```text
cc-zellij-plugin/
├── Cargo.toml                   # Remove feature flags, single [[bin]] target
├── src/
│   ├── main.rs                  # Runtime mode dispatch, remove legacy fallback + PluginState impl
│   ├── lib.rs                   # Shared types (pipe channel name constants)
│   ├── controller/
│   │   ├── mod.rs               # Update pipe names to cc-deck:ctrl:*
│   │   ├── render_broadcast.rs  # Update pipe names to cc-deck:side:*
│   │   ├── sidebar_registry.rs  # Update pipe names to cc-deck:side:*
│   │   ├── events.rs            # Update keybinding pipe names
│   │   └── state.rs             # Fix test fixtures (new PaneInfo fields, dead session removal logic)
│   ├── sidebar_plugin/
│   │   ├── mod.rs               # Update pipe names, remove cfg gates
│   │   └── input.rs             # Update pipe names to cc-deck:ctrl:*
│   ├── pipe_handler.rs          # Update pipe names (legacy handler)
│   ├── sync.rs                  # DELETE
│   ├── state.rs                 # DELETE (old PluginState)
│   ├── sidebar.rs               # DELETE (old sidebar renderer)
│   ├── state_machine_tests.rs   # DELETE
│   ├── attend.rs               # DELETE (old attend logic, absorbed into controller)
│   ├── notification.rs          # DELETE (old notification logic, absorbed into controller)
│   ├── rename.rs                # DELETE (old rename logic, absorbed into sidebar_plugin)
│   ├── fuzz_tests.rs            # DELETE (proptest tests for removed PluginState)
│   ├── config.rs                # Minor test fix (timer_interval default assertion)
│   └── [git, session, perf]     # Keep as-is

cc-deck/
├── internal/
│   ├── plugin/
│   │   ├── embed.go             # Single go:embed, simplified PluginInfo
│   │   ├── install.go           # Single binary install, remove permission pre-population
│   │   ├── layout.go            # Update sidebar/controller references to cc_deck.wasm
│   │   ├── layout_test.go       # Update test assertions
│   │   ├── remove.go            # Simplified removal (single binary)
│   │   └── zellij.go            # Remove EnsureControllerPermissions
│   └── cmd/
│       └── hook.go              # Update pipe name to cc-deck:ctrl:hook

Makefile                         # Single build-wasm target, simplified copy-wasm
```

### Files to Delete

- `cc-zellij-plugin/src/sync.rs` - Entire sync subsystem (pipe-based, file-based, peer request)
- `cc-zellij-plugin/src/state.rs` - Old PluginState struct (used by legacy no-feature-flag path)
- `cc-zellij-plugin/src/sidebar.rs` - Old sidebar renderer (replaced by sidebar_plugin/)
- `cc-zellij-plugin/src/state_machine_tests.rs` - Tests for removed PluginState
- `cc-zellij-plugin/src/attend.rs` - Old attend logic (absorbed into controller)
- `cc-zellij-plugin/src/notification.rs` - Old notification logic (absorbed into controller)
- `cc-zellij-plugin/src/rename.rs` - Old rename logic (absorbed into sidebar_plugin)
- `cc-zellij-plugin/src/fuzz_tests.rs` - Proptest fuzz tests for removed PluginState
- `cc-deck/internal/plugin/cc_deck_controller.wasm` - Old controller binary (embedded)
- `cc-deck/internal/plugin/cc_deck_sidebar.wasm` - Old sidebar binary (embedded)

### Pipe Channel Rename Map

| Old Name | New Name | Files |
|----------|----------|-------|
| `cc-deck:hook` | `cc-deck:ctrl:hook` | pipe_handler.rs, controller/mod.rs, hook.go |
| `cc-deck:action` | `cc-deck:ctrl:action` | lib.rs, controller/mod.rs, sidebar_plugin/input.rs |
| `cc-deck:sidebar-hello` | `cc-deck:ctrl:hello` | controller/mod.rs, controller/sidebar_registry.rs, sidebar_plugin/mod.rs |
| `cc-deck:render` | `cc-deck:side:render` | lib.rs, controller/mod.rs, controller/render_broadcast.rs, sidebar_plugin/mod.rs |
| `cc-deck:sidebar-init` | `cc-deck:side:init` | controller/mod.rs, controller/sidebar_registry.rs, sidebar_plugin/mod.rs |
| `cc-deck:sidebar-reindex` | `cc-deck:side:reindex` | controller/mod.rs, controller/sidebar_registry.rs, sidebar_plugin/mod.rs |
| `cc-deck:navigate` | `cc-deck:side:navigate` | controller/mod.rs, controller/events.rs, sidebar_plugin/mod.rs, main.rs |
| `cc-deck:navigate-prev` | `cc-deck:side:navigate-prev` | main.rs, pipe_handler.rs |
| `cc-deck:attend` | `cc-deck:ctrl:attend` | main.rs, pipe_handler.rs |
| `cc-deck:attend-prev` | `cc-deck:ctrl:attend-prev` | main.rs, pipe_handler.rs |

## Complexity Tracking

No constitution violations. This feature reduces complexity by eliminating feature flags, dual builds, and the sync subsystem.
