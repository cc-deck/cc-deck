# Review Guide: Plugin Integration and E2E Testing

**Spec:** [spec.md](spec.md) | **Plan:** [plan.md](plan.md) | **Tasks:** [tasks.md](tasks.md)
**Generated:** 2026-05-07

---

## What This Spec Does

Adds integration tests that exercise the cc-deck Zellij plugin's two main components (SidebarRendererPlugin and ControllerPlugin) through their ZellijPlugin trait methods with synthetic events. This closes the gap between unit tests (which test state logic with direct function calls) and manual Zellij testing (which catches WASM-specific regressions). The integration tests run on native targets as part of `cargo test`.

**In scope:** Pipe-to-state event chains, controller-sidebar protocol verification, permission grant flow, mode transitions, error handling for malformed input, test helper infrastructure.

**Out of scope:** Real WASM host function effects (tab switching, pane focusing), terminal rendering verification (ANSI output), multi-instance coordination (multiple sidebars through a shared controller), timer-dependent behavior with real time delays.

## Bigger Picture

The plugin codebase has 228 existing tests, but they all test state logic in isolation or verify serialization roundtrips. The actual plugin lifecycle (load, grant permissions, receive pipe messages, update state, render) is only exercised manually inside Zellij. Multiple regressions in the pipe message protocol have been caught only through manual testing. This feature fills the most critical testing gap without the complexity of spinning up real Zellij instances (which would require solving terminal automation, CI environment setup, and flakiness from timing dependencies).

This is a natural successor to the [proptest fuzz testing](../051-proptest-fuzz-testing/) work, which added property-based tests for the sidebar state machine. Together, unit tests, fuzz tests, and these integration tests form a three-layer testing strategy. Real Zellij E2E tests (the "Approach 2" from the brainstorm) remain a future possibility but are explicitly deferred.

---

## Spec Review Guide (30 minutes)

> This guide helps you focus your 30 minutes on the parts of the spec and plan
> that need human judgment most. Each section points to specific locations and
> frames the review as questions.

### Understanding the approach (8 min)

Read the [implementation approach](plan.md#implementation-approach) for the overall strategy. The key design decision is adding `#[cfg(test)]` accessors on both plugin structs to expose private state for test assertions. As you read, consider:

- Is the `test_state()` accessor pattern the right trade-off between test coverage and encapsulation? The alternative (testing only through return values) would severely limit what can be verified, since `pipe()` returns only a boolean.
- Are 15+ integration tests the right target count? The spec [success criteria](spec.md#measurable-outcomes) set this as a minimum. Is this enough to catch the regressions that have historically required manual testing?

### Key decisions that need your eyes (12 min)

**Test file organization** ([plan.md Phase 2-3](plan.md#phase-2-sidebar-integration-tests-fr-001-fr-003-fr-006))

Integration tests go in separate files (`integration_tests.rs`) within each plugin module, following the existing `fuzz_tests.rs` pattern. This keeps them in the crate (for `pub(crate)` access) while avoiding bloating the main module files.
- Does this match the team's preference? Inline tests in `mod.rs` would keep everything in one place but those files are already 300-400 lines.

**Helper function scope** ([plan.md Phase 1](plan.md#phase-1-test-infrastructure-fr-007))

All PipeMessage construction helpers go in the shared `test_helpers.rs`. This file currently has 5 helpers and would grow to 11+.
- Should controller-specific helpers live in a separate `controller/test_helpers.rs` instead of the shared sidebar file? The current plan puts everything in one place for simplicity.

**Deferred event replay testing** ([spec.md User Story 5](spec.md#user-story-5---permission-grant-and-deferred-event-replay-priority-p3))

Only one test (T023) covers deferred event replay. The spec has two acceptance scenarios, but the second ("processed in order and matches expected state") is tested within the same test.
- Is single-test coverage for the permission deferral flow sufficient, given it is a P3 story and the mechanism rarely changes?

### Areas where I'm less certain (5 min)

- [spec.md Edge Cases](spec.md#edge-cases): The edge case "SidebarHello arrives for a plugin ID that is already registered" implies the controller should update the registration. I am not certain this is the current behavior, and the integration test would need to verify what actually happens rather than assert a specific outcome. The test may discover current behavior differs from the spec's assumption.

- [plan.md Phase 1b](plan.md#phase-1-test-infrastructure-fr-007): The `make_hook_pipe()` helper assumes `PipeSource::Cli` for hook events. In practice, hooks arrive from the CLI process, but I have not verified whether the controller's `pipe()` method checks the source field or ignores it. If it checks, the helper's source value matters for test correctness.

- [tasks.md Phase 9](tasks.md#phase-9-user-story-6---sidebar-mode-transitions-priority-p2): The spec lists sidebar mode transitions under User Story 1 (FR-006), but the tasks break it out as a separate Phase 9. This is a minor organizational inconsistency that does not affect implementation.

### Risks and open questions (5 min)

- The plan's [risk assessment](plan.md#risk-assessment) notes that WASM host function stubs may cause false positives. Are there specific regressions from project history that would NOT be caught by these integration tests because they involved host function behavior? If so, that gap should be documented as a known limitation.

- Test helpers in `test_helpers.rs` currently use `pub(crate)` visibility. Adding 6+ new helpers increases the surface area. Could any of these helpers be useful outside the test context (for example, in a future `cc-deck dump-state` diagnostic command)?

- The [success criterion SC-002](spec.md#measurable-outcomes) requires tests complete in under 5 seconds. Is this per-test or total? The spec says "total," but if individual tests create and tear down plugin instances, is there a risk of cumulative overhead?

---

*Full context in linked [spec](spec.md) and [plan](plan.md).*
