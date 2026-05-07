# Implementation Plan: Plugin Integration and E2E Testing

**Branch**: `052-plugin-integration-e2e-testing` | **Date**: 2026-05-07 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/052-plugin-integration-e2e-testing/spec.md`

## Summary

Add plugin-level integration tests that exercise SidebarRendererPlugin and ControllerPlugin through their ZellijPlugin trait methods (load, update, pipe, render) with synthetic events. This closes the gap between unit tests (which test state logic in isolation) and manual Zellij testing (which tests the real WASM plugin). Integration tests verify the full event dispatch chain on native targets without requiring a running Zellij instance.

## Technical Context

**Language/Version**: Rust stable, edition 2021 (wasm32-wasip1 target for build, native for tests)
**Primary Dependencies**: zellij-tile 0.43.1 (plugin SDK), serde/serde_json 1.x (serialization)
**Storage**: N/A (test-only feature, no persistent storage)
**Testing**: cargo test (native target), existing test helpers in sidebar_plugin/test_helpers.rs
**Target Platform**: Native (tests run on host, not WASM)
**Project Type**: Zellij plugin (Rust WASM)
**Performance Goals**: All integration tests complete in under 5 seconds
**Constraints**: No access to real WASM host functions (stubbed as no-ops in test environment)
**Scale/Scope**: 15+ integration tests across two plugin types

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Tests + Documentation | PASS | This feature IS tests. README update included in scope. |
| II. Interface contracts | N/A | No new interface implementations. |
| III. Build/tool rules | PASS | Tests run via `make test` (wraps `cargo test`). No direct `cargo build`. |

## Project Structure

### Documentation (this feature)

```text
specs/052-plugin-integration-e2e-testing/
├── spec.md
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── REVIEW-SPEC.md       # Spec review output
└── checklists/
    └── requirements.md
```

### Source Code (repository root)

```text
cc-zellij-plugin/src/
├── sidebar_plugin/
│   ├── mod.rs                  # Add #[cfg(test)] test_state() accessor
│   ├── test_helpers.rs         # Extend with PipeMessage construction helpers
│   ├── integration_tests.rs    # NEW: Sidebar integration tests
│   ├── state.rs                # Existing (unchanged)
│   └── fuzz_tests.rs           # Existing (unchanged)
├── controller/
│   ├── mod.rs                  # Add #[cfg(test)] test_state() accessor
│   ├── integration_tests.rs    # NEW: Controller integration tests
│   └── state.rs                # Existing (unchanged)
├── main.rs                     # Existing (unchanged)
└── lib.rs                      # Existing (unchanged)
```

**Structure Decision**: Integration test files follow the established pattern of `fuzz_tests.rs` as separate modules within the plugin directories. This maintains `pub(crate)` access to internal types and test helpers.

## Implementation Approach

### Phase 1: Test Infrastructure (FR-007)

Add test state accessors and pipe message construction helpers.

**1a. Test State Accessors**

Add `#[cfg(test)]` accessor methods to both plugin structs:

```rust
// In sidebar_plugin/mod.rs
#[cfg(test)]
impl SidebarRendererPlugin {
    pub(crate) fn test_state(&self) -> &SidebarState {
        &self.state
    }
}

// In controller/mod.rs
#[cfg(test)]
impl ControllerPlugin {
    pub(crate) fn test_state(&self) -> &ControllerState {
        &self.state
    }
}
```

**1b. PipeMessage Construction Helpers**

Extend `test_helpers.rs` with functions that construct PipeMessage values:

- `make_pipe(name: &str, payload: &str) -> PipeMessage`: Generic pipe message from plugin source
- `make_hook_pipe(hook_event: &str, pane_id: u32) -> PipeMessage`: Hook event from CLI source
- `make_hook_pipe_with_cwd(hook_event: &str, pane_id: u32, cwd: &str) -> PipeMessage`: Hook event with CWD
- `make_action_pipe(action: ActionType, pane_id: u32, sidebar_plugin_id: u32) -> PipeMessage`: Action message from sidebar
- `make_hello_pipe(plugin_id: u32) -> PipeMessage`: SidebarHello from sidebar plugin
- `make_init_pipe(tab_index: usize, controller_plugin_id: u32) -> PipeMessage`: SidebarInit from controller

**1c. Plugin Setup Helpers**

Add convenience functions that create and initialize plugins in a ready-to-test state:

- `setup_sidebar() -> SidebarRendererPlugin`: Creates, loads, and grants permissions
- `setup_controller() -> ControllerPlugin`: Creates, loads, and grants permissions
- `setup_sidebar_with_tab(tab_index: usize) -> SidebarRendererPlugin`: Setup + SidebarInit

### Phase 2: Sidebar Integration Tests (FR-001, FR-003, FR-006)

File: `sidebar_plugin/integration_tests.rs`

Tests covering User Stories 1, 3 (sidebar side), 4 (sidebar side), 5 (sidebar):

1. **test_sidebar_load_and_permission_grant**: Load plugin, verify not initialized, grant permissions, verify initialized state.
2. **test_sidebar_receives_render_payload**: Send render payload with 3 sessions via pipe, verify state contains correct sessions.
3. **test_sidebar_payload_replacement**: Send two consecutive render payloads, verify second replaces first.
4. **test_sidebar_render_before_permissions**: Send render payload before granting permissions, verify state is unchanged.
5. **test_sidebar_init_assigns_tab**: Send SidebarInit pipe message, verify tab_index is stored.
6. **test_sidebar_navigate_mode_via_pipe**: Send navigate pipe message, verify mode transitions to Navigate.
7. **test_sidebar_empty_payload**: Send render payload with zero sessions, verify empty state without errors.
8. **test_sidebar_malformed_pipe_message**: Send pipe message with invalid JSON, verify no panic and state unchanged.

### Phase 3: Controller Integration Tests (FR-002, FR-003, FR-005)

File: `controller/integration_tests.rs`

Tests covering User Stories 2, 3 (controller side), 4 (controller side), 5 (controller):

1. **test_controller_load_and_permission_grant**: Load controller, verify not initialized, grant permissions, verify initialized state.
2. **test_controller_hook_session_start**: Send SessionStart hook event, verify new session created with Init activity.
3. **test_controller_hook_pre_tool_use**: Create session, send PreToolUse hook, verify activity transitions to Working.
4. **test_controller_hook_stop**: Create session, send Stop hook, verify activity transitions to Done.
5. **test_controller_sidebar_hello_registration**: Send SidebarHello pipe message, verify sidebar registered in registry.
6. **test_controller_action_pause**: Create session, send Pause action, verify paused state toggles.
7. **test_controller_action_attend**: Create done session, send Attend action, verify done_attended flag set.
8. **test_controller_deferred_events**: Send hook events before permissions, grant permissions, verify all events replayed.
9. **test_controller_malformed_hook_payload**: Send hook pipe with invalid JSON, verify no panic.

### Phase 4: Protocol Roundtrip Tests (FR-004)

Add to the appropriate integration test file:

1. **test_render_payload_roundtrip_through_pipe**: Construct RenderPayload, serialize to JSON, send through sidebar pipe, verify deserialized state matches.
2. **test_action_message_roundtrip_through_pipe**: Construct ActionMessage, serialize to JSON, send through controller pipe, verify action processed correctly.

### Phase 5: Documentation (Constitution I)

Update README.md to document the integration test suite:
- What integration tests cover
- How to run them (`make test`)
- Distinction from unit tests and fuzz tests

## Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| WASM host function stubs cause false positives | Medium | Low | Tests verify state transitions, not host function effects. This limitation is documented. |
| PipeMessage struct changes in zellij-tile updates | Low | Medium | Tests use helper functions that centralize PipeMessage construction. |
| Private state accessor pattern couples tests to internals | Low | Low | Accessors are `#[cfg(test)]` only and read-only. State struct is already `pub(crate)`. |
