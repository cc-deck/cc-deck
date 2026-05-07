# Implementation Plan: WASM Plugin Dead Code Removal and Code Health

**Branch**: `049-wasm-dead-code-cleanup` | **Date**: 2026-05-06 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/049-wasm-dead-code-cleanup/spec.md`

## Summary

Remove ~4,000+ lines of dead legacy code from the monolithic `PluginState` architecture that coexists alongside the live controller/sidebar split architecture. The `UnifiedPlugin` dispatcher routes to either `ControllerPlugin` or `SidebarRendererPlugin` at runtime, making the entire `PluginState` code path unreachable. After dead code removal, reorganize `main.rs` into focused modules and remove the global `#![allow(dead_code, unused_imports)]` suppression so the compiler catches real issues going forward.

## Technical Context

**Language/Version**: Rust stable, edition 2021
**Primary Dependencies**: zellij-tile 0.44, serde/serde_json 1.x
**Storage**: WASI `/cache/` directory for persistent state
**Testing**: `make test` (cargo test on native target)
**Target Platform**: WASM (wasm32-wasip1) compiled to run in Zellij's wasmi interpreter
**Project Type**: Zellij plugin (WASM binary)
**Performance Goals**: Minimize WASM binary size (LTO fat, opt-level z, strip, panic abort)
**Constraints**: Must compile for both wasm32-wasip1 (production) and native (testing)
**Scale/Scope**: ~14,400 LOC total, targeting ~4,000+ line reduction

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| Tests and documentation | PASS | This is a refactoring feature. No new user-facing behavior. Existing tests must pass. Legacy test files audited before deletion. No doc updates needed (no new commands, flags, or config). |
| Interface contracts | N/A | No new interface implementations. Existing controller/sidebar protocol unchanged. |
| Build and tool rules | PASS | Will use `make test` and `make lint` exclusively. No direct cargo build. |

## Project Structure

### Documentation (this feature)

```text
specs/049-wasm-dead-code-cleanup/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output (minimal for refactoring)
└── tasks.md             # Phase 2 output (/speckit.tasks command)
```

### Source Code (repository root)

```text
cc-zellij-plugin/src/
├── main.rs              # UnifiedPlugin dispatcher + mod declarations (target: <500 lines)
├── lib.rs               # Shared protocol types (RenderPayload, ActionMessage, etc.)
├── config.rs            # PluginConfig loading (ACTIVE, unchanged)
├── git.rs               # Git detection/branch discovery (ACTIVE, unchanged)
├── session.rs           # Session/Activity types and helpers (ACTIVE, unchanged)
├── pipe_handler.rs      # Pipe message parsing (ACTIVE, gains sync message utils)
├── perf.rs              # Performance tracking (ACTIVE, unchanged)
├── debug.rs             # NEW: debug_log, debug_init, install_panic_hook (extracted from main.rs)
├── keybindings.rs       # NEW: register_keybindings, shift_variant (extracted from main.rs)
├── wasm_compat.rs       # EXISTING+EXPANDED: all WASM/native conditional function pairs
├── controller/          # Controller plugin (ACTIVE, unchanged)
│   ├── mod.rs
│   ├── actions.rs
│   ├── events.rs        # Has its own register_keybindings/shift_variant (duplicate of main.rs)
│   ├── hooks.rs
│   ├── render_broadcast.rs
│   ├── sidebar_registry.rs
│   └── state.rs         # Gains cleanup_orphaned_state_files() from sync.rs
└── sidebar_plugin/      # Sidebar renderer plugin (ACTIVE, unchanged)
    ├── mod.rs
    ├── input.rs
    ├── modes.rs
    ├── rename.rs
    ├── render.rs         # Gains HELP_LINES from sidebar.rs
    └── state.rs

DELETED (legacy dead code):
├── state.rs             # PluginState, PluginMode, SidebarMode (dead)
├── sidebar.rs           # Monolithic sidebar rendering (dead, except HELP_LINES relocated)
├── rename.rs            # Monolithic rename logic (dead, superseded by sidebar_plugin/rename.rs)
├── attend.rs            # Monolithic attend logic (dead, superseded by controller/actions.rs)
├── sync.rs              # Multi-instance sync (dead, except utilities relocated)
├── notification.rs      # Notification helpers (dead, superseded by sidebar_plugin/state.rs)
├── state_machine_tests.rs  # Tests for dead SidebarMode (dead)
└── fuzz_tests.rs        # Fuzz tests for dead PluginState (dead)
```

**Structure Decision**: The existing controller/sidebar_plugin split architecture is the live code. All root-level modules that only serve the dead `PluginState` path are deleted. Shared utilities used by active code are relocated before deletion.

## Research Findings

### Dead vs Active Module Classification

Based on codebase analysis, here is the definitive classification:

**Purely Dead Modules** (only referenced from dead `PluginState` code in main.rs):
- `state.rs` (789 lines) - `PluginState`, `PluginMode`, `SidebarMode`, `NavigateContext`, etc.
- `rename.rs` (342 lines) - superseded by `sidebar_plugin/rename.rs`
- `attend.rs` (479 lines) - superseded by attend logic in `controller/actions.rs`
- `notification.rs` - superseded by `Notification` type in `sidebar_plugin/state.rs`

**Mostly Dead Modules** (contain relocatable active code):
- `sidebar.rs` (601 lines) - only `HELP_LINES` constant is used by active code (`sidebar_plugin/render.rs`). Relocate to `sidebar_plugin/render.rs`, then delete.
- `sync.rs` (825 lines) - three categories:
  1. `cleanup_orphaned_state_files()` + `extract_pid_from_filename()` + process-checking helpers: used by controller. Relocate to `controller/state.rs`.
  2. `is_sync_message()`, `is_request_message()`, `extract_pid_from_message_name()`: used by `pipe_handler.rs`. Relocate to `pipe_handler.rs`.
  3. Everything else (broadcast_state, sync_now, handle_sync, etc.): dead, operates on `PluginState`.

**Dead Test Files**:
- `state_machine_tests.rs` (1,080 lines) - tests `SidebarMode` from dead `state.rs`. The sidebar_plugin has its own `modes.rs` with 10 tests.
- `fuzz_tests.rs` (324 lines) - fuzz tests for dead `PluginState`.

**Active Shared Modules** (unchanged):
- `config.rs`, `git.rs`, `session.rs`, `pipe_handler.rs`, `perf.rs`, `wasm_compat.rs`, `lib.rs`

### main.rs Code Classification

Current main.rs: 2,084 lines. After cleanup, target: <500 lines.

| Line Range | Content | Status |
|------------|---------|--------|
| 1 | `#![allow(dead_code, unused_imports)]` | DELETE |
| 3-28 | mod declarations | EDIT (remove dead mods) |
| 35-69 | debug_log, debug_init, DEBUG_ENABLED | EXTRACT to debug.rs |
| 76-99 | install_panic_hook | EXTRACT to debug.rs |
| 102-121 | sanitize_voice_text | KEEP in main.rs or move to a utility |
| 125-182 | PerfTimer, PerfTimerPipe | Already in perf.rs? Verify. |
| 185-190 | set_selectable_wasm | Already in wasm_compat.rs |
| 215-274 | shift_variant, register_keybindings | DEAD (controller/events.rs has own copies) |
| 283-361 | broadcast_action through auto_rename_tab WASM shims | AUDIT: check if used by active code |
| 374-483 | UnifiedPlugin enum + ZellijPlugin impl | KEEP |
| 484-511 | Tests for UnifiedPlugin/shift_variant | KEEP UnifiedPlugin tests, DELETE shift_variant tests |
| 512-2061 | PluginState impl blocks (ZellijPlugin, handle_event, key handlers) | DELETE |

### WASM Function Pair Audit

Functions in main.rs with `#[cfg(target_family = "wasm")]` / `#[cfg(not(...))]` pairs:

| Function | Used by Active Code? | Action |
|----------|---------------------|--------|
| `debug_init()` | Yes (controller, sidebar load) | Extract to debug.rs |
| `debug_log()` | Yes (extensively) | Extract to debug.rs |
| `install_panic_hook()` | Yes (controller, sidebar load) | Extract to debug.rs |
| `set_selectable_wasm()` | Already in wasm_compat.rs | No action |
| `register_keybindings()` | No (controller has own copy) | DELETE |
| `shift_variant()` | No (controller has own copy) | DELETE |
| `broadcast_action()` | Verify | Move to wasm_compat.rs if active |
| `focus_plugin()` | Verify | Move to wasm_compat.rs if active |
| `focus_terminal()` | Verify | Move to wasm_compat.rs if active |
| `switch_to_tab()` | Verify | Move to wasm_compat.rs if active |
| `create_new_session_tab()` | Verify | Move to wasm_compat.rs if active |
| `create_new_session_pane()` | Verify | Move to wasm_compat.rs if active |
| `close_session_pane()` | Verify | Move to wasm_compat.rs if active |
| `auto_rename_tab()` | Verify | Move to wasm_compat.rs if active |

> Note: The WASM shim functions (broadcast_action through auto_rename_tab) need a final usage check during implementation. If used by controller/sidebar, relocate to wasm_compat.rs. If only used by dead PluginState code, delete.

### Keybinding Deduplication

`register_keybindings()` and `shift_variant()` exist in TWO places:
1. `main.rs` lines 215-274 (dead, only called from PluginState path)
2. `controller/events.rs` (active, with its own tests)

The main.rs versions are dead code. Delete them (and the associated tests in main.rs). The controller's copies are the canonical versions.

The spec says to extract these to a `keybindings.rs` module, but since the controller already has working copies with tests, and the sidebar doesn't need keybinding registration, extracting to a shared module adds complexity with no benefit. Recommendation: keep them in controller/events.rs and skip creating keybindings.rs, unless the spec mandates it.

### HELP_LINES Relocation

`sidebar.rs` defines `HELP_LINES: &[&str]` (a constant array of help overlay text). It's used by `sidebar_plugin/render.rs`. Move the constant directly into `sidebar_plugin/render.rs` before deleting `sidebar.rs`.

## Data Model

This is a refactoring feature with no new data model. The existing types are:

**Preserved types** (in lib.rs, used by controller/sidebar protocol):
- `RenderSession`, `RenderPayload`, `ActionType`, `ActionMessage`, `SidebarHello`, `SidebarInit`

**Preserved types** (in session.rs, used by controller and sidebar):
- `Session`, `Activity`, `WaitReason`

**Deleted types** (in state.rs, only used by dead PluginState):
- `PluginState`, `PluginMode`, `SidebarMode`, `NavigateContext`, `RenameState`, `FilterState`, `PendingOverride` (if only in state.rs)

## Implementation Phases

### Phase 1: Audit and Relocate (prerequisites for deletion)

Before deleting any files, relocate the small amount of active code that lives in otherwise-dead modules:

1. **Audit WASM shim functions** in main.rs (lines 283-361): grep for each function name in controller/ and sidebar_plugin/ to determine if active. Move active ones to wasm_compat.rs, mark dead ones for deletion.

2. **Relocate `HELP_LINES`** from `sidebar.rs` to `sidebar_plugin/render.rs`.

3. **Relocate sync utilities to pipe_handler.rs**: Move `is_sync_message()`, `is_request_message()`, `extract_pid_from_message_name()` from `sync.rs` to `pipe_handler.rs`. Update the two call sites in `pipe_handler.rs` to use local functions.

4. **Relocate `cleanup_orphaned_state_files()`** and its helpers (`extract_pid_from_filename()`, process-alive checks) from `sync.rs` to `controller/state.rs`. Update call sites in `controller/mod.rs` and `controller/events.rs`.

5. **Audit legacy test files**: Compare test scenarios in `state_machine_tests.rs` and `fuzz_tests.rs` against existing tests in `sidebar_plugin/modes.rs`, `sidebar_plugin/rename.rs`, `controller/` test modules. Document any unique coverage gaps. Port unique scenarios before deletion.

### Phase 2: Delete Dead Code

6. **Delete legacy module files**: `state.rs`, `sidebar.rs`, `rename.rs`, `attend.rs`, `sync.rs`, `notification.rs`

7. **Delete legacy test files**: `state_machine_tests.rs`, `fuzz_tests.rs`

8. **Delete PluginState code from main.rs**: Remove all `PluginState` impl blocks (lines ~512-2061), dead WASM shim functions, dead `register_keybindings`/`shift_variant`, dead imports and `use` statements.

9. **Remove dead mod declarations** from main.rs: `mod state`, `mod sidebar`, `mod rename`, `mod attend`, `mod sync`, `mod notification`, `mod state_machine_tests`, `mod fuzz_tests`.

10. **Verify**: `make test` and `make lint` pass.

### Phase 3: Extract and Reorganize main.rs

11. **Extract debug module**: Move `DEBUG_ENABLED`, `debug_init()`, `debug_log()`, `install_panic_hook()` from main.rs to new `src/debug.rs`. Update all call sites (`crate::debug_log` becomes `crate::debug::debug_log`, etc., or re-export from main.rs).

12. **Consolidate WASM shims**: Move any remaining WASM/native function pairs from main.rs to `wasm_compat.rs`. The goal is that main.rs contains only: mod declarations, `sanitize_voice_text()` (if not better placed elsewhere), `UnifiedPlugin` enum + impl, and the `register_plugin!` macro.

13. **Verify**: `make test` and `make lint` pass. Confirm main.rs is under 500 lines.

### Phase 4: Remove Suppression and Consolidate Types

14. **Remove `#![allow(dead_code, unused_imports)]`** from main.rs line 1.

15. **Fix warnings**: Add targeted `#[allow(dead_code)]` only to WASM/native conditional code that legitimately triggers the lint (the `#[cfg(not(target_family = "wasm"))]` no-op stubs).

16. **Consolidate types**: Check for any redundant type definitions between lib.rs and the module tree. Ensure single canonical definitions.

17. **Final verification**: `make test` passes, `make lint` passes with zero warnings.

18. **Measure binary size**: Build WASM binary before and after (on a separate measurement branch or by comparing with main). Document any reduction.

## Complexity Tracking

No constitution violations. This is a straightforward deletion and reorganization with no new abstractions, no new dependencies, and no architectural changes.

## Risks and Mitigations

| Risk | Likelihood | Mitigation |
|------|-----------|------------|
| Deleting code that's actually used | Low | Thorough grep-based audit before each deletion. Compiler will catch missing references. |
| Test coverage gap after deleting legacy tests | Medium | Explicit audit step (Phase 1, step 5) before deletion. Port unique scenarios. |
| WASM binary breaks after changes | Low | `make test` runs native tests. Manual WASM build verification with `make build-wasm`. |
| Type consolidation causes import churn | Low | Only consolidate if duplicates actually exist. Research found zero duplicates currently. |
