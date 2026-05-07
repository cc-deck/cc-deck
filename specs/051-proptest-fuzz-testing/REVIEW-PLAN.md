# Review Guide: Property-Based Fuzz Testing for Sidebar State Machine

**Spec:** [spec.md](spec.md) | **Plan:** [plan.md](plan.md) | **Tasks:** [tasks.md](tasks.md)
**Generated:** 2026-05-07

---

## What This Spec Does

The sidebar plugin has a 7-state mode machine with roughly 25 transitions driven by keyboard and mouse input. Existing unit tests verify known paths, but they cannot explore the combinatorial space of multi-step input sequences. This feature adds proptest-based property testing that generates random action sequences and checks state invariants after every action. The project previously had proptest fuzz tests that found real cursor-invariant bugs, but those tests targeted a since-removed type and were deleted during cleanup.

**In scope:** FuzzAction enum covering all sidebar inputs, 5 state invariants, proptest integration with 2000 cases, regression seed management.

**Out of scope:** Controller-level event handling (hooks.rs, events.rs), cargo-fuzz/libfuzzer integration, CI pipeline changes.

## Bigger Picture

This feature is part of an ongoing test infrastructure investment following spec 049 (dead code cleanup) and spec 050 (test coverage measurement). The sidebar state machine is the most interaction-heavy component in the plugin, and it already has ~80 hand-written unit tests covering individual transitions. Fuzz testing fills the gap between "each transition works" and "arbitrary sequences of transitions maintain invariants." If the fuzz suite finds bugs (as the previous one did), those fixes ship alongside the tests rather than as follow-up work.

The proptest crate is new to this project. If it proves valuable here, it could later be applied to the controller state machine (sessions map, pane manifest, tab list), which has even more complex state interactions.

---

## Spec Review Guide (30 minutes)

> This guide helps you focus your 30 minutes on the parts that need human judgment most.

### Understanding the approach (8 min)

Read [User Story 1](spec.md#user-story-1---discover-unknown-state-machine-bugs-via-random-input-sequences-priority-p1) and the [FuzzAction design decision](plan.md#d1-fuzzaction-enum-design) for the core approach. As you read, consider:

- Are the 5 invariants in [FR-003](spec.md#functional-requirements) sufficient, or are there state properties we care about that they miss? For example, should we verify that `NavigateDeleteConfirm` always stores a pane_id that exists in the current session list?
- Does uniform weighting across all 22 action variants in [D1](plan.md#d1-fuzzaction-enum-design) make sense, or should session mutations be rarer to reflect realistic usage patterns?
- Is 2000 cases the right balance for the [performance target](spec.md#measurable-outcomes) of <10 seconds?

### Key decisions that need your eyes (12 min)

**Regression seed incompatibility** ([D4](plan.md#d4-regression-seed-migration))

The plan concludes that existing seeds in `proptest-regressions/fuzz_tests.txt` are incompatible with the new module path and FuzzAction shape. The decision is to keep them as historical artifacts rather than attempting migration.
- Question for reviewer: Should we delete the old seeds to avoid confusion, or does their documentary value (showing what bugs were previously found) outweigh the clutter?

**Click region construction** ([D5](plan.md#d5-click-region-synchronization))

Mouse actions need valid click regions. The plan synthesizes them with a simple 3-row-per-session formula rather than calling the actual render function.
- Question for reviewer: Is the 3-row assumption stable enough, or could future render changes silently invalidate the fuzz test's mouse coverage?

**Single-file implementation** ([tasks.md Phase 2](tasks.md#phase-2-user-story-1---discover-unknown-state-machine-bugs-priority-p1))

All core fuzz test code goes in one file (fuzz_tests.rs). This means no parallelism within the main implementation phase.
- Question for reviewer: At ~300-400 lines, is this the right granularity, or should the FuzzAction strategy and invariant checks be in separate files?

### Areas where I'm less certain (5 min)

- [Invariant 5 (Help consistency)](plan.md#d3-invariant-verification): The "Help round-trip preserves state" check is the weakest invariant. Help wraps any mode via `Box<SidebarMode>`, and verifying the round-trip via `toggle_help` only checks that the toggle is involutory, not that Help interacts correctly with all inner modes.

- [Session mutation realism](plan.md#d2-action-application-strategy): The plan adds/removes sessions by directly mutating `cached_payload`. In production, session changes arrive via pipe messages with additional state updates. The fuzz test may miss bugs that only manifest during the full payload update path.

- [Filter state invariant scope](spec.md#functional-requirements): FR-003 checks `filter_text.is_empty()` in Passive mode, but there is also a mode-level `FilterState` in `NavigateFilter`. Should we verify that `filter_text` and the mode-level filter never conflict?

### Risks and open questions (5 min)

- If the fuzz suite discovers real bugs in the state machine ([T010](tasks.md#phase-2-user-story-1---discover-unknown-state-machine-bugs-priority-p1)), should fixes be made inline or does each bug merit its own commit for traceability?
- The [performance target](spec.md#measurable-outcomes) of <10s is based on local developer machine benchmarks. Is there a CI environment where proptest's default RNG might be slower, and should we set a `PROPTEST_CASES` environment variable override for CI?
- proptest is a new dependency for this project. Does adding it have any implications for the WASM build size, given it is dev-only? (It should not, but worth confirming the `[dev-dependencies]` section is not included in the WASM target.)

---

*Full context in linked [spec](spec.md) and [plan](plan.md).*
