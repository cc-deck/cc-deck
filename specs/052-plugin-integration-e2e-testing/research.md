# Research: Plugin Integration and E2E Testing

## Decision 1: State Access Pattern for Integration Tests

**Decision**: Add `#[cfg(test)]` accessor methods on both SidebarRendererPlugin and ControllerPlugin to expose internal state for test assertions.

**Rationale**: Both plugins have private `state` fields. Without accessors, integration tests can only observe return values from `pipe()` and `update()` (both return `bool`), which is insufficient to verify internal state transitions like session creation, activity changes, or mode transitions. The brainstorm document explicitly proposed this pattern. The existing codebase already uses `#[cfg(test)]` module declarations for test helpers.

**Alternatives considered**:
- **Return-value-only testing**: Too limited. `pipe()` returns `bool` (should re-render), which cannot distinguish between successful payload processing and a no-op.
- **Mock/spy on zellij-tile output functions**: Would require a mock framework for WASM host functions. Overly complex for the goal of verifying state transitions.
- **Make state fields pub(crate)**: Too permissive. Breaks encapsulation beyond test scope.

## Decision 2: Test File Organization

**Decision**: Place integration tests in dedicated modules within the existing source tree: `sidebar_plugin/integration_tests.rs` and `controller/integration_tests.rs`, following the existing pattern of `sidebar_plugin/fuzz_tests.rs`.

**Rationale**: The codebase already uses this pattern (fuzz_tests.rs is a separate file within the sidebar_plugin module). Integration tests need access to `pub(crate)` items, which requires being within the same crate. A separate `tests/` directory would make integration tests external and lose access to internal helpers.

**Alternatives considered**:
- **Inline in mod.rs**: Would bloat already-large files (mod.rs is 300+ lines for sidebar, 400+ for controller).
- **External tests/ directory**: Would lose `pub(crate)` access to test helpers and state accessors.

## Decision 3: PipeMessage Construction Helpers

**Decision**: Create new helper functions that construct complete PipeMessage values for each protocol message type (hook events, render payloads, action messages, sidebar hello/init).

**Rationale**: Test readability requires reducing boilerplate. Each test should express intent (e.g., "send hook event for SessionStart") not mechanics (constructing PipeMessage with name, payload JSON, and source).

**Alternatives considered**:
- **Reuse existing test_helpers.rs**: The existing helpers build RenderPayload and SidebarState but not PipeMessage wrappers. Extending that file is appropriate for shared helpers; plugin-specific PipeMessage builders go in their respective integration test files.

## Decision 4: Timer-Dependent Behavior Testing

**Decision**: Skip timer-dependent scenarios in integration tests. Test state transitions that result from timer events by calling `update(Event::Timer(...))` directly, without real time delays.

**Rationale**: The spec explicitly states timer-dependent behavior is tested through direct state manipulation. Integration tests focus on the pipe-to-state pipeline, not time-based cleanup.

## Decision 5: Controller State Accessor Scope

**Decision**: Expose ControllerPlugin state via `#[cfg(test)] pub(crate) fn test_state(&self) -> &ControllerState` to allow integration tests to verify session creation, activity transitions, and sidebar registry state.

**Rationale**: ControllerPlugin integration tests need to verify that hook events create sessions, update activities, and register sidebars. The controller's `pipe()` always returns `false` (no UI), making return-value testing useless for state verification.
