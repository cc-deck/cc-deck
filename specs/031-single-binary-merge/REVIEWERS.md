# Review Guide: 031 Single Binary Merge

## Summary

Merges the two WASM plugin binaries (`cc_deck_controller.wasm` + `cc_deck_sidebar.wasm`) into a single `cc_deck.wasm` with runtime mode dispatch. Adopts named pipe channels (`cc-deck:ctrl:*` / `cc-deck:side:*`) and removes legacy code.

## Key Review Areas

### 1. Runtime Mode Dispatch (spec.md US1, plan.md Project Structure)

The core architectural change. The unified `CcDeckPlugin` enum in `main.rs` reads `config.get("mode")` during `load()` and delegates to either `ControllerPlugin` or `SidebarRendererPlugin`.

**What to verify:**
- The `CcDeckPlugin` enum correctly delegates all `ZellijPlugin` trait methods (load, update, pipe, render)
- Default-to-sidebar behavior for missing or unrecognized mode values
- No performance overhead from the delegation layer

**Files:** `cc-zellij-plugin/src/main.rs`, `cc-zellij-plugin/Cargo.toml`

### 2. Pipe Channel Protocol (spec.md US2, plan.md Pipe Channel Rename Map)

All 10 pipe messages renamed to use `cc-deck:ctrl:*` / `cc-deck:side:*` convention. Constants defined in `lib.rs::channels`.

**What to verify:**
- All pipe name string literals replaced with constants (no stale hardcoded names)
- Controller pipe handler ignores `cc-deck:side:*` messages
- Sidebar pipe handler ignores `cc-deck:ctrl:*` messages
- CLI hook uses broadcast pipe (no `--plugin` flag) per FR-005

**Files:** `cc-zellij-plugin/src/lib.rs`, `cc-zellij-plugin/src/controller/mod.rs`, `cc-zellij-plugin/src/sidebar_plugin/mod.rs`, `cc-zellij-plugin/src/pipe_handler.rs`, `cc-deck/internal/cmd/hook.go`

### 3. Legacy Code Removal (spec.md US3, plan.md Files to Delete)

8 Rust source files and 2 embedded WASM binaries deleted. Verify no dangling references.

**What to verify:**
- No `mod sync`, `mod state`, `mod sidebar`, `mod attend`, `mod notification`, `mod rename` declarations remain in main.rs
- No `#[cfg(feature = "controller")]` or `#[cfg(feature = "sidebar")]` anywhere in codebase
- `cargo test` passes on native target
- Types previously in `state.rs` that sidebar_plugin still needs are defined in sidebar_plugin or lib.rs

**Files:** All deleted files listed in plan.md "Files to Delete" section

### 4. Build Pipeline and Go CLI (spec.md US4, plan.md Project Structure)

Single build target, single embedded binary, single install path.

**What to verify:**
- Makefile `build-wasm` produces one file, `copy-wasm` copies one file
- `embed.go` has exactly one `//go:embed cc_deck.wasm`
- `SDKVersion` in `embed.go` matches Cargo.toml's `zellij-tile` version
- `install.go` removes old two-binary files during migration
- `layout.go` references `cc_deck.wasm` for both sidebar and controller roles
- No `EnsureControllerPermissions` in `zellij.go`
- No `permissions.kdl` writes

**Files:** `Makefile`, `cc-deck/internal/plugin/embed.go`, `cc-deck/internal/plugin/install.go`, `cc-deck/internal/plugin/layout.go`, `cc-deck/internal/plugin/zellij.go`

### 5. Constitution Compliance

| Principle | Relevant | Notes for reviewer |
|-----------|----------|-------------------|
| II. Plugin Installation | Yes | Verify `make install` is used, not direct cargo/go build |
| III. WASM Filename | Yes | Must be `cc_deck.wasm` (underscore) everywhere |
| IV. WASM Host Function Gating | Yes | All Zellij host calls must have `#[cfg(target_family = "wasm")]` |
| VI. Build via Makefile Only | Yes | No raw `cargo build` or `go build` in tasks/checkpoints |
| IX. Documentation Freshness | Yes | README update in T026 |
| X. Spec Tracking | Yes | Feature spec table update in T026 |
| XII. Prose Plugin | Yes | T026 uses prose plugin with cc-deck voice |

## Review Checklist

- [ ] Runtime mode dispatch works correctly for both controller and sidebar
- [ ] All 10 pipe channel names use new `ctrl:`/`side:` convention
- [ ] No stale pipe name string literals remain
- [ ] All legacy files are deleted with no dangling references
- [ ] Single WASM binary builds, embeds, and installs correctly
- [ ] Migration cleanup removes old two-binary files
- [ ] `cargo test` and `go test` pass
- [ ] `cargo clippy` reports no warnings
- [ ] README documents first-tab permission behavior
