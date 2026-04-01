# Research: Single Binary Merge (031)

**Date**: 2026-04-01

## Decision 1: Plugin Registration Strategy

**Decision**: Create a `UnifiedPlugin` enum wrapping `ControllerPlugin` and `SidebarRendererPlugin`, dispatching based on `configuration.get("mode")` at `load()` time.

**Rationale**: Both plugin structs already exist as separate modules with clean `ZellijPlugin` implementations. An enum wrapper delegates all trait methods (`load`, `update`, `pipe`, `render`) to the active variant without merging internal state. This is ~50 lines of new code with zero changes to existing controller/sidebar logic.

**Alternatives considered**:
- Merging into a single monolithic struct: Would require ~500+ LOC refactoring, risk regressions, and lose the clean separation.
- Runtime trait object dispatch (`Box<dyn ZellijPlugin>`): `register_plugin!` macro expects a concrete type, not a trait object. Would require forking the macro.

## Decision 2: Cargo.toml Structure

**Decision**: Remove feature flags and `[[bin]]` targets entirely. Use a single `[[bin]]` target named `cc_deck` with `path = "src/main.rs"`.

**Rationale**: Feature flags only served to select which `register_plugin!` call was compiled. With runtime mode selection, both modules are always compiled. No `required-features` needed.

**Alternatives considered**:
- Keeping feature flags as optional optimizations: Adds build complexity for negligible binary size savings (most code is shared).

## Decision 3: Event Subscription

**Decision**: The `UnifiedPlugin::load()` method calls `subscribe()` with different event sets based on mode. Controller subscribes to heavy events (PaneUpdate, TabUpdate, Timer, etc.), sidebar subscribes to Mouse/Key only.

**Rationale**: This is the core performance benefit of the architecture split. Subscribing sidebars to heavy events would reintroduce the N-instance scaling problem.

**Alternatives considered**:
- Subscribe to all events, ignore in handler: Wastes WASM function calls per event per instance. Defeats the purpose of the split.

## Decision 4: Go CLI Embed Simplification

**Decision**: Single `//go:embed cc_deck.wasm` replacing the three current embeds. `PluginInfo` struct simplified to `Binary []byte` and `BinarySize int64`.

**Rationale**: YAGNI. The controller/sidebar-specific fields and the legacy binary are artifacts of the two-binary architecture. No backward compatibility needed per clarification.

**Alternatives considered**:
- Keeping separate fields pointing at same binary: Unnecessary indirection.

## Decision 5: Permission Handling

**Decision**: Remove `EnsureControllerPermissions()` entirely. The sidebar instance (in a tab) gets the permission dialog. Zellij's permission cache, keyed by plugin URL, covers the controller instance using the same `cc_deck.wasm` URL.

**Rationale**: This is the primary motivation for the merge. The permission pre-population hack in `zellij.go` exists solely because background plugins cannot show permission dialogs. With a single binary, the sidebar's dialog covers both.

**Alternatives considered**: None viable within the two-binary architecture.

## Decision 6: Layout and Config Generation

**Decision**: Both `load_plugins` (config.kdl) and `default_tab_template` (layout.kdl) reference `cc_deck.wasm` with different `mode` config values. The `sidebarPluginBlock()` and `controllerConfigBlock()` functions update their paths from `cc_deck_sidebar.wasm`/`cc_deck_controller.wasm` to `cc_deck.wasm`.

**Rationale**: Minimal change to existing generation logic. Just update the filename references.

## Decision 7: Hook Routing

**Decision**: No change to CLI hook command. Keep broadcast via `zellij pipe --name cc-deck:hook`. Sidebars receive and ignore hook messages.

**Rationale**: The current broadcast approach is already in use and working. Targeted routing would require the controller to persist its plugin_id, adding a new failure mode for negligible benefit.

## Decision 8: Makefile Cleanup

**Decision**: Single `build-wasm` target replacing `build-wasm-controller` and `build-wasm-sidebar`. Single `copy-wasm` target. Add cleanup step to remove old `cc_deck_controller.wasm` and `cc_deck_sidebar.wasm` from the plugins directory during `make install`.

**Rationale**: Simplifies the build pipeline. Legacy file cleanup belongs in the Makefile (per clarification), not in Go CLI runtime detection.

## Key Files to Modify

### Rust Plugin (cc-zellij-plugin/)
| File | Change |
|------|--------|
| `Cargo.toml` | Remove features section, replace two `[[bin]]` with single `cc_deck` target |
| `src/main.rs` | Remove `#[cfg(feature)]` gates, add `UnifiedPlugin` enum, `register_plugin!(UnifiedPlugin)` |
| `src/controller/mod.rs` | No changes (module always compiled) |
| `src/sidebar_plugin/mod.rs` | No changes (module always compiled) |

### Go CLI (cc-deck/)
| File | Change |
|------|--------|
| `internal/plugin/embed.go` | Single embed, simplified `PluginInfo` |
| `internal/plugin/install.go` | Remove two-binary atomic write, remove `EnsureControllerPermissions` call, remove legacy cleanup |
| `internal/plugin/zellij.go` | Remove `EnsureControllerPermissions()` function |
| `internal/plugin/layout.go` | Update `sidebarPluginBlock()` and `controllerConfigBlock()` to reference `cc_deck.wasm` |
| `internal/plugin/remove.go` | Simplify to remove single `cc_deck.wasm` |
| `internal/plugin/state.go` | Update `InstallState` to check single binary |
| `internal/plugin/layout_test.go` | Update test expectations for new paths |

### Build System
| File | Change |
|------|--------|
| `Makefile` | Single `build-wasm` target, single `copy-wasm`, cleanup of old binaries in install |
