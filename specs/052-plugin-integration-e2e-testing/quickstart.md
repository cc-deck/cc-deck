# Quickstart: Plugin Integration and E2E Testing

## Run Integration Tests

```bash
make test
```

Integration tests run as part of the standard `cargo test` suite. No special setup needed.

## Run Only Integration Tests

```bash
cargo test --lib integration_tests
```

## Test Structure

Integration tests live alongside the code they test:

- `cc-zellij-plugin/src/sidebar_plugin/integration_tests.rs` - Sidebar plugin tests
- `cc-zellij-plugin/src/controller/integration_tests.rs` - Controller plugin tests

## Writing New Integration Tests

Use the setup helpers:

```rust
use crate::sidebar_plugin::test_helpers::*;

#[test]
fn test_my_scenario() {
    let mut plugin = setup_sidebar();
    let payload = make_payload(vec![make_session(1, "my-session", 0)]);
    plugin.pipe(make_pipe("cc-deck:render", &serde_json::to_string(&payload).unwrap()));
    assert_eq!(plugin.test_state().filtered_sessions().len(), 1);
}
```
