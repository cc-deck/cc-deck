# Implementation Plan: Property-Based Fuzz Testing for Sidebar State Machine

**Branch**: `051-proptest-fuzz-testing` | **Date**: 2026-05-07 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `specs/051-proptest-fuzz-testing/spec.md`

## Summary

Add proptest-based property testing for the sidebar mode state machine (7 modes, ~25 transitions). Define a `FuzzAction` enum covering all user inputs (keyboard, mouse, session mutations), generate random action sequences, and verify state invariants after every action. Reuse existing test helpers and migrate historical regression seeds.

## Technical Context

**Language/Version**: Rust 2021 edition, compiled to WASM (wasm32-wasip1) for production, native target for tests
**Primary Dependencies**: zellij-tile 0.44, serde/serde_json 1.x, proptest (new dev-dependency)
**Storage**: N/A
**Testing**: `cargo test` (via `make test`), proptest for property-based testing
**Target Platform**: Native (tests run on host, not WASM)
**Project Type**: Zellij plugin (WASM) with native test harness
**Performance Goals**: Fuzz test suite completes in <10 seconds for 2000 cases
**Constraints**: Tests must not depend on WASM host functions (all gated behind `#[cfg(target_family = "wasm")]`)
**Scale/Scope**: Single test file, ~300-400 lines of test code

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Tests + docs | PASS | This IS a test feature. No user-facing changes requiring README/CLI/Antora updates. |
| II. Interface contracts | N/A | No new interface implementations. |
| III. Build and tool rules | PASS | Will use `make test` / `make lint`. No direct `cargo build`. |

## Project Structure

### Documentation (this feature)

```text
specs/051-proptest-fuzz-testing/
├── plan.md              # This file
├── spec.md              # Feature specification
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── checklists/          # Quality checklists
│   └── requirements.md
├── REVIEW-SPEC.md       # Spec review
└── tasks.md             # Phase 2 output (via /speckit-tasks)
```

### Source Code (repository root)

```text
cc-zellij-plugin/
├── Cargo.toml                          # Add proptest dev-dependency
├── src/sidebar_plugin/
│   ├── mod.rs                          # Add fuzz_tests module declaration
│   ├── fuzz_tests.rs                   # NEW: property-based fuzz tests
│   ├── input.rs                        # Existing (functions under test)
│   ├── modes.rs                        # Existing (SidebarMode enum)
│   ├── state.rs                        # Existing (SidebarState)
│   └── test_helpers.rs                 # Existing (reused by fuzz tests)
└── proptest-regressions/
    └── fuzz_tests.txt                  # Existing seeds (evaluate migration)
```

**Structure Decision**: Single new file `fuzz_tests.rs` in the existing `sidebar_plugin` module. No new directories or modules beyond the test file itself.

## Design Decisions

### D1: FuzzAction Enum Design

The `FuzzAction` enum must cover all input paths in `input.rs`. Based on code analysis:

**Keyboard actions (from `handle_navigate_key`, `handle_filter_key`, `handle_delete_confirm_key`, `handle_rename_key`):**
- KeyJ, KeyK (navigation)
- KeyEnter, KeyEsc (confirm/cancel)
- KeyD (delete), KeyR (rename), KeyP (pause)
- KeySlash (filter), KeyQuestion (help), KeyF1 (help)
- KeyM (mute), KeyN (new session), KeyBigR (refresh)
- KeyY, KeyBigY (delete confirm)
- KeyBackspace (filter/rename editing)
- ArbitraryChar(char) (filter/rename typing)

**Navigation entry points (from `toggle_navigate`, `toggle_navigate_prev`):**
- ToggleNavigate, ToggleNavigatePrev

**Mouse actions (from `handle_mouse`):**
- LeftClick(usize), RightClick(usize)

**Session mutations (simulated payload changes):**
- AddSession, RemoveSession

Total: ~21 distinct variants, exceeding the SC-003 target of 18.

### D2: Action Application Strategy

Each `FuzzAction` maps to a specific function call:
- Keyboard actions: `input::handle_key(state, bare(BareKey::...))`
- Toggle navigate: `input::toggle_navigate(state)` / `input::toggle_navigate_prev(state)`
- Mouse: `input::handle_mouse(state, Mouse::LeftClick(row, 0))` / `Mouse::RightClick(row, 0)`
- AddSession: Append a new `RenderSession` to `cached_payload.sessions`, update click regions, call `preserve_cursor()`
- RemoveSession: Remove a random session from `cached_payload.sessions`, update click regions, call `preserve_cursor()`

### D3: Invariant Verification

Five invariants checked after every action:

1. **Cursor in bounds**: If `mode.nav_ctx()` returns `Some(ctx)`, then `ctx.cursor_index < max(1, filtered_sessions.len())`
2. **Filter state consistency**: If mode is `NavigateFilter`, then `mode.filter_state()` returns `Some`
3. **Passive filter clean**: If mode is `Passive`, then `filter_text.is_empty()`
4. **Selectable matches mode**: `mode.is_selectable() == !matches!(mode, Passive)`
5. **Help consistency**: If `mode.is_help()`, inner mode via `toggle_help` round-trip preserves state

### D4: Regression Seed Migration

The existing seeds in `proptest-regressions/fuzz_tests.txt` reference a different test module path and likely a different `FuzzAction` enum shape. The seeds will NOT be directly compatible because:
- The old test was in a top-level `fuzz_tests` module, the new one is in `sidebar_plugin::fuzz_tests`
- The `FuzzAction` variants may have different names/ordering

**Decision**: Keep the old seeds file as-is (historical reference). Create a new regression file at the path proptest expects for the new module. If the new fuzz suite finds bugs, proptest will automatically create seeds at the correct path.

### D5: Click Region Synchronization

Mouse actions (LeftClick, RightClick) reference row positions. The fuzz test must maintain valid click regions that match the current session list. After any session mutation, rebuild click regions with 3-row spacing per session (matching the render layout).

## Complexity Tracking

No constitution violations to justify.
