# Brainstorm: WASM Plugin Dead Code Removal and Code Health

**Date:** 2026-05-06
**Status:** active

## Problem Framing

The WASM plugin has accumulated ~4,500 lines of dead code (31% of the 14,436-line codebase) from the legacy monolithic `PluginState` architecture.
This architecture was superseded by the controller/sidebar split in spec 030 (single-instance architecture), but the old code was never removed.

The dead code is not just clutter.
It caused a real bug during spec 042: a permission fix was applied to the dead `PluginState::load()` instead of the live `SidebarRendererPlugin::load()`.
The stale code path silently accepted the change while the real path remained broken.

### Dead code inventory

| Module | Lines | Status |
|--------|-------|--------|
| `src/state.rs` | 789 | Superseded by `controller/state.rs` + `sidebar_plugin/modes.rs` |
| `src/sync.rs` | 825 | Superseded by single-instance arch; only `cleanup_orphaned_state_files()` still used |
| `src/sidebar.rs` | 601 | Superseded by `sidebar_plugin/render.rs` |
| `src/attend.rs` | 479 | Superseded by `controller/actions.rs` |
| `src/rename.rs` | 342 | Superseded by `sidebar_plugin/rename.rs` |
| `src/state_machine_tests.rs` | 1,080 | Tests for dead `PluginState` |
| `src/fuzz_tests.rs` | 324 | Tests for dead `PluginState` |
| Legacy code in `main.rs` | ~1,600 | `PluginState` impl blocks, lines ~485-2083 |
| **Total** | **~4,500** | **31% of codebase** |

### Code health issues beyond dead code

- `main.rs` is 2,083 lines and mixes the `UnifiedPlugin` dispatcher, WASM shims, debug logging, panic hooks, keybinding registration, and the entire legacy `PluginState`
- Global `#![allow(dead_code, unused_imports)]` suppresses real warnings
- Current WASM binary: 898 KB (LTO may already strip some dead code, but not the compilation cost)

## Approaches Considered

### A: Big Bang Cleanup (Chosen)

Single spec covering all cleanup in one pass: delete dead modules, reorganize main.rs, consolidate types.

- Pros: Single coherent cleanup, avoids intermediate broken states, easy to review (mostly deletions)
- Cons: Larger diff, but review is straightforward since it is almost entirely deletions

### B: Incremental (Three Phases)

Three separate specs: (1) delete dead modules, (2) reorganize main.rs, (3) consolidate types.

- Pros: Smaller diffs per PR, easier bisection
- Cons: More overhead, intermediate states may need temporary workarounds, three review cycles

### C: Conservative (Dead Code Only)

Only delete dead modules. No reorganization, no type consolidation.

- Pros: Minimal risk, fast to execute
- Cons: Misses the code health improvements, main.rs stays bloated at ~500 lines of mixed concerns

## Decision

Chose **Approach A: Big Bang Cleanup**. Rationale: the work is almost entirely deletions with some function moves. The compiler catches any accidental breakage immediately. One spec, one implementation pass, one review.

### Agreed scope

1. **Delete legacy modules entirely:** `state.rs`, `sidebar.rs`, `rename.rs`, `attend.rs`, `sync.rs`
2. **Extract before deleting:** Move `cleanup_orphaned_state_files()` from `sync.rs` to `controller/state.rs`
3. **Audit legacy tests before deleting:** Scan `state_machine_tests.rs` and `fuzz_tests.rs` for coverage gaps against controller/sidebar tests; port any unique scenarios, then delete both files
4. **Strip legacy code from main.rs:** Remove `PluginState` impl blocks (~1,600 lines), legacy imports, legacy mod declarations
5. **Reorganize main.rs** by extracting into focused modules:
   - `src/debug.rs` for debug logging, panic hook, `DEBUG_ENABLED`
   - Expand `src/wasm_compat.rs` with all WASM/native function pairs (set_selectable, focus_plugin, focus_terminal, switch_to_tab, etc.)
   - `src/keybindings.rs` for `register_keybindings()` and `shift_variant()`
6. **Consolidate redundant types** between `lib.rs` and the module tree
7. **Remove global `#![allow(dead_code, unused_imports)]`** from main.rs; apply targeted `#[allow(...)]` only where needed

### Success criteria

- `cargo test` passes, `cargo clippy` passes with zero warnings
- `main.rs` under 500 lines (down from 2,083)
- No references to `PluginState`, `PluginMode`, or legacy `SidebarMode`
- WASM binary size decrease measured (may be modest if LTO was already stripping)
- Total line count drops by ~4,000+

## Open Threads

- Exact binary size reduction is unpredictable since LTO may already strip dead code at link time. Needs before/after measurement.
- Some shared helper functions in `sync.rs` beyond `cleanup_orphaned_state_files()` may be worth keeping. Needs audit during implementation.
