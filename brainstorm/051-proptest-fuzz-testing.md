# Brainstorm: Property-Based Fuzz Testing for Sidebar State Machine

**Date:** 2026-05-06
**Status:** proposed

## Problem Framing

The sidebar plugin has a 7-state mode machine with ~25 transitions driven by keyboard and mouse input.
Unit tests (added in spec 049) verify known transitions, but cannot discover unknown edge cases like:
- Mode transitions that leave the cursor out of bounds after a session is removed
- Filter state leaking between mode transitions
- Key sequences that reach impossible states (e.g., NavigateDeleteConfirm with an invalid pane_id)

The project previously had proptest fuzz tests (324 lines in `fuzz_tests.rs`), but they tested the dead `PluginState` type and were removed during spec 049.
Two regression seeds survive in `proptest-regressions/fuzz_tests.txt`, proving that fuzzing previously found real bugs (navigation key sequences that violated cursor invariants).

### Why fuzz testing specifically

The sidebar mode machine is a classic fuzz target:
- Small state space (7 modes, bounded session list)
- Deterministic transitions (same input always produces same output)
- Clear invariants that must hold after every action
- History of real bugs found by the previous proptest suite

## Proposed Approach

### Action strategy

Define a `FuzzAction` enum covering all user inputs:
- All keyboard keys (j, k, Enter, Esc, d, r, p, /, ?, m, n, y, Y, Backspace, arbitrary chars)
- Toggle navigate and toggle navigate prev (keybinding entry points)
- Left click and right click at arbitrary rows
- Session mutations (add/remove sessions from payload)

Use `proptest::prop_oneof!` with uniform weighting across ~21 action variants.
Sequences of 1-50 actions, starting with 0-5 initial sessions.

### Invariants to verify after every action

1. **No panic** (implicit, the most basic property)
2. **Cursor in bounds**: `cursor_index < max(1, filtered_sessions.len())`
3. **Mode accessor consistency**: if mode is NavigateFilter, `filter_state()` returns Some
4. **Filter text empty in Passive**: `filter_text` is always empty when mode is Passive
5. **Selectable matches mode**: Passive is not selectable, all others are

### Configuration

- 2000 test cases per run (balance between thoroughness and CI speed)
- Sequence length 1-50 (long enough to find multi-step bugs)
- Regression seeds preserved in `proptest-regressions/`

### File organization

New file `cc-zellij-plugin/src/sidebar_plugin/fuzz_tests.rs` with `#[cfg(test)] mod fuzz_tests` declaration in mod.rs.
This keeps fuzz tests separate from unit tests for clarity.

### Open Questions

- Should we also fuzz the controller event handling (hooks.rs, events.rs)?
  The controller has more complex state (sessions map, pane manifest, tab list) and might benefit from fuzzing session mutation sequences.
- Should we use `cargo-fuzz` (libfuzzer-based) in addition to proptest?
  `cargo-fuzz` finds different kinds of bugs (crash/panic paths) while proptest finds invariant violations.
- How many proptest cases are worth the CI time? 2000 cases runs in ~2-5 seconds locally, but might be slower in CI.
- Should the old regression seeds in `proptest-regressions/fuzz_tests.txt` be migrated to the new test module path, or left as historical artifacts?
