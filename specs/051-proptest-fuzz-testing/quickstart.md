# Quickstart: Property-Based Fuzz Testing

## What This Feature Does

Adds proptest-based property testing for the sidebar mode state machine. Generates random sequences of user actions (keyboard, mouse, session mutations) and verifies that state invariants hold after every action.

## Files Changed

1. `cc-zellij-plugin/Cargo.toml` - Add `proptest = "1"` to `[dev-dependencies]`
2. `cc-zellij-plugin/src/sidebar_plugin/mod.rs` - Add `mod fuzz_tests;` declaration
3. `cc-zellij-plugin/src/sidebar_plugin/fuzz_tests.rs` - New file with fuzz test suite

## How to Run

```bash
# Run all tests including fuzz tests
make test

# Run only the fuzz tests
cargo test -p cc-deck fuzz

# Run with verbose output to see proptest progress
cargo test -p cc-deck fuzz -- --nocapture
```

## How It Works

1. proptest generates a random initial state (0-5 sessions)
2. It generates a random sequence of 1-50 `FuzzAction` values
3. Each action is applied to the `SidebarState` via the corresponding handler
4. After every action, 5 invariants are checked
5. If an invariant fails, proptest shrinks the input to find the minimal failing case
6. Failing cases are saved as regression seeds for future runs
