# Brainstorm: Plugin Integration and E2E Testing

**Date:** 2026-05-06
**Status:** proposed

## Problem Framing

The cc-deck WASM plugin tests only run on native, with all Zellij host functions stubbed as no-ops.
This means the actual plugin behavior inside Zellij is never automatically verified:
- Pipe message serialization/deserialization between controller and sidebar
- The full load -> permission grant -> subscribe -> render lifecycle
- WASM host function calls (focus_plugin_pane, pipe_message_to_plugin, rename_tab)
- Visual rendering output and click region accuracy

Multiple regressions have been caught only through manual testing inside Zellij.
The gap between "tests pass" and "plugin works" is the primary quality risk.

### Testing layers

| Layer | Coverage | Gap |
|-------|----------|-----|
| Unit tests (native) | Mode transitions, state logic, parsing | WASM host calls are no-ops |
| Proptest (native) | Invariant verification across random sequences | Same WASM limitation |
| **Plugin integration (native)** | **Full event flow through SidebarRendererPlugin** | **New, proposed** |
| **Zellij E2E (real WASM)** | **Plugin running in actual Zellij** | **New, proposed** |

## Approach 1: Plugin-Level Integration Tests (recommended first)

Test `SidebarRendererPlugin` and `ControllerPlugin` directly on native by calling their `ZellijPlugin` trait methods (`load`, `update`, `pipe`, `render`) with synthetic events.

### What this tests

- The full event dispatch chain: `pipe()` -> parse message -> update state -> `render()` output
- Controller-sidebar protocol: render payload serialization, action message deserialization
- Permission grant flow and deferred event replay
- Session lifecycle: hook event -> session creation -> CWD detection -> display name
- Mode transitions through the real pipe interface (not just direct function calls)

### What this does NOT test

- Actual WASM host function effects (tab switching, pane focusing, keybinding registration)
- Terminal rendering correctness (ANSI output is just written to stdout)
- Multi-instance coordination (would need multiple plugin instances communicating)

### Implementation sketch

Add `#[cfg(test)]` accessor to `SidebarRendererPlugin`:
```rust
#[cfg(test)]
impl SidebarRendererPlugin {
    pub(crate) fn test_state(&self) -> &SidebarState { &self.state }
}
```

Test example:
```rust
#[test]
fn test_sidebar_receives_payload_and_renders() {
    let mut plugin = SidebarRendererPlugin::default();
    plugin.load(BTreeMap::new());
    plugin.update(Event::PermissionRequestResult(PermissionStatus::Granted));
    
    // Send render payload via pipe
    let payload = make_render_payload(vec![session("api", 10, 0)]);
    plugin.pipe(make_pipe("cc-deck:render", &serde_json::to_string(&payload).unwrap()));
    
    assert!(plugin.test_state().initialized);
    assert_eq!(plugin.test_state().filtered_sessions().len(), 1);
}
```

### Effort estimate

- Add test accessors: ~10 lines
- Pipe message construction helpers: ~30 lines
- 10-15 integration tests: ~200-300 lines
- Could be done in a single session

## Approach 2: Real Zellij E2E Tests (deferred)

Run the actual WASM plugin inside a real Zellij instance and verify behavior through the CLI pipe interface.

### How it would work

1. Start Zellij with a test layout loading the cc-deck plugin
2. Send pipe commands via `zellij pipe` (e.g., `zellij pipe cc-deck:hook --payload '...'`)
3. Query state via `zellij pipe cc-deck:dump-state` and parse the JSON response
4. Verify session list, mode state, and focus via the dump output
5. Send keyboard inputs via `zellij action write-chars` or `zellij action write`

### Challenges

- **Zellij startup time**: ~2-3 seconds per test, making the suite slow
- **Terminal automation**: no official expect-style framework for Zellij plugins
- **Rendering verification**: terminal output is hard to assert on (ANSI codes, variable widths)
- **CI environment**: needs Zellij installed, a terminal emulator, and possibly a virtual display
- **Flakiness**: timing-dependent behavior (timer events, grace periods) causes intermittent failures

### What Zellij provides

Zellij does not have an official plugin testing framework.
Their own tests (in the Zellij repo) use integration tests that spawn Zellij processes and interact via CLI, similar to what we would build.

The Go CLI already has e2e tests in `cc-deck/internal/e2e/` that build the binary once and test as subprocess. This pattern could be extended to include Zellij plugin scenarios.

### Possible middle ground: state dump verification

Rather than full rendering verification, test through the `cc-deck dump-state` pipe:
1. Start Zellij with the plugin
2. Trigger hook events via `cc-deck hook` CLI commands
3. Dump state and verify session count, names, activities
4. This avoids rendering assertions entirely and tests the real WASM plugin

## Open Questions

- Should integration tests live in `sidebar_plugin/mod.rs` (inline) or a separate `tests/` directory?
- For Zellij E2E, should we use the existing Go e2e framework or build a Rust/shell-based one?
- Is the `cc-deck dump-state` pipe reliable enough to serve as the primary E2E assertion mechanism?
- How do we handle timer-dependent behavior in tests (grace periods, stale session cleanup)?
- Should we invest in screenshot/golden-file tests for rendering, or is state verification sufficient?
- Would a "headless Zellij" mode be worth requesting upstream? (Zellij running without a terminal, exposing state via IPC)
