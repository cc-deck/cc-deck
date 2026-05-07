# Research: Property-Based Fuzz Testing for Sidebar State Machine

## R1: proptest Dependency Compatibility

**Decision**: Add `proptest = "1"` as a `[dev-dependencies]` entry in `cc-zellij-plugin/Cargo.toml`.

**Rationale**: proptest is the standard Rust property-testing crate. It compiles only for tests (dev-dependency), so it does not affect the WASM binary size or production dependencies. The crate has no conflicts with zellij-tile or serde.

**Alternatives considered**:
- `quickcheck`: Simpler API but weaker shrinking. proptest's `prop_oneof!` and structured shrinking produce better minimal failing cases.
- `cargo-fuzz` (libfuzzer): Finds panics but not invariant violations. Requires a separate binary target and different CI integration. Out of scope per spec.

## R2: WASM Host Function Stubs

**Decision**: All WASM-gated functions in the sidebar module already have `#[cfg(not(target_family = "wasm"))]` no-op stubs. The fuzz test runs on the native target where these stubs are active. No additional stubbing needed.

**Rationale**: Verified in `input.rs`: `send_action_wasm` and `focus_self_wasm` both have no-op stubs. The `set_selectable_wasm` function in `wasm_compat` also has a stub. Tests compile and run natively without WASM host functions.

**Alternatives considered**: None needed; existing stubs are sufficient.

## R3: proptest Configuration

**Decision**: Use `ProptestConfig` with `cases = 2000` and default settings for shrinking and timeout.

**Rationale**: 2000 cases with sequence length 1-50 provides good coverage without excessive CI time. Local testing of similar proptest suites shows ~2-5 seconds for this case count. The default fork mode (`false`) is appropriate since we want direct stack traces on failure.

**Alternatives considered**:
- Higher case count (10000): Diminishing returns, increases CI time to 15-25s.
- Lower case count (500): Faster but reduces chance of finding rare multi-step bugs.

## R4: Regression Seed Path Resolution

**Decision**: proptest stores regression seeds at a path derived from the test function's module path. For `cc_deck::sidebar_plugin::fuzz_tests::test_sidebar_invariants`, proptest looks for `proptest-regressions/sidebar_plugin/fuzz_tests/test_sidebar_invariants.txt`.

**Rationale**: The existing `proptest-regressions/fuzz_tests.txt` was created by the old top-level `fuzz_tests` module with a different test function signature. The seed format encodes the `Arbitrary` impl shape, which will differ. These seeds are incompatible and should be left as historical artifacts.

**Alternatives considered**:
- Manual seed migration: Not feasible because the serialized format depends on the exact proptest `Strategy` tree shape, which has changed.
- Delete old seeds: They serve as documentation of previously-found bugs. Keep them.

## R5: Click Region Construction in Tests

**Decision**: Build click regions programmatically from the session list. Each session occupies 3 rows starting at row 2 (after header). Formula: session at index `i` gets click region `(2 + i*3, pane_id, tab_index)`.

**Rationale**: The render module uses a similar 3-row layout per session card. The fuzz test does not need pixel-perfect rendering, just valid click targets that the mouse handlers can resolve. The header click region at row 0 uses the sentinel `u32::MAX - 1`.

**Alternatives considered**:
- Call the actual render function: Not feasible in tests because render writes to the WASM terminal buffer.
- Skip mouse testing: Would miss click-related state transitions. Mouse actions are ~10% of real user input.
