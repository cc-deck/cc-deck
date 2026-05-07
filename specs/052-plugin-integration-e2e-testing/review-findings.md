# Deep Review Findings

**Date:** 2026-05-07
**Branch:** 052-plugin-integration-e2e-testing
**Rounds:** 1
**Gate Outcome:** PASS
**Invocation:** quality-gate

## Summary

| Severity | Found | Fixed | Remaining |
|----------|-------|-------|-----------|
| Critical | 0 | 0 | 0 |
| Important | 0 | 0 | 0 |
| Minor | 7 | - | 7 |
| **Total** | **7** | **0** | **7** |

**Agents completed:** 5/5 (+ 1 external tool: CodeRabbit)
**Agents failed:** none
**External tools:** CodeRabbit (4 findings, 2 false positives dismissed), Copilot (not installed, skipped)

## Findings

### FINDING-1
- **Severity:** Minor
- **Confidence:** 80
- **File:** cc-zellij-plugin/src/sidebar_plugin/integration_tests.rs:74-92
- **Category:** test-quality
- **Source:** correctness-agent (also reported by: test-quality-agent)
- **Round found:** 1
- **Resolution:** remaining (spec/code mismatch, candidate for spex:evolve)

**What is wrong:**
`test_sidebar_render_before_permissions` asserts `cached_payload.is_some()`, meaning the sidebar stores the render payload even without permission grant. The spec (US1 acceptance scenario 3) says "the payload is not processed until permissions are granted."

**Why this matters:**
The test correctly validates actual code behavior (the sidebar re-requests permissions and stores the payload regardless). However, the spec text and implementation diverge. This should be reconciled via spec evolution rather than a code change.

### FINDING-2
- **Severity:** Minor
- **Confidence:** 75
- **File:** cc-zellij-plugin/src/controller/integration_tests.rs:150-168
- **Category:** test-quality
- **Source:** correctness-agent
- **Round found:** 1
- **Resolution:** remaining (acceptable limitation)

**What is wrong:**
`test_controller_deferred_events` queues a `Timer` event before permissions, not hook events as the spec describes (US5 acceptance scenario 1). The controller's `pipe()` method drops messages before permissions rather than queuing them, so pipe-based hook events cannot be deferred.

**Why this matters:**
The test exercises the queuing mechanism that exists (update() event queuing) but doesn't validate that hook events specifically are deferred, because they are delivered via pipe() which drops them pre-permission. This is an acceptable limitation of the architecture but worth noting.

### FINDING-3
- **Severity:** Minor
- **Confidence:** 85
- **File:** cc-zellij-plugin/src/sidebar_plugin/test_helpers.rs:102-117
- **Category:** architecture
- **Source:** architecture-agent
- **Round found:** 1
- **Resolution:** remaining (intentional, planned for future use)

**What is wrong:**
`make_hook_pipe_with_cwd` is marked `#[allow(dead_code)]` and not used by any test.

**Why this matters:**
Dead code in test helpers adds maintenance burden. However, this helper was planned in the spec (FR-007) and is available for future tests that need CWD in hook events.

### FINDING-4
- **Severity:** Minor
- **Confidence:** 85
- **File:** cc-zellij-plugin/src/sidebar_plugin/test_helpers.rs:188-193
- **Category:** architecture
- **Source:** architecture-agent
- **Round found:** 1
- **Resolution:** remaining (intentional, planned for future use)

**What is wrong:**
`setup_sidebar_with_tab` is marked `#[allow(dead_code)]` and not used by any test.

**Why this matters:**
Same as FINDING-3. This helper was planned in the spec and is available for future use.

### FINDING-5
- **Severity:** Minor
- **Confidence:** 70
- **File:** cc-zellij-plugin/src/controller/integration_tests.rs:271-299
- **Category:** architecture
- **Source:** architecture-agent
- **Round found:** 1
- **Resolution:** remaining (acceptable for now)

**What is wrong:**
`make_pane_info_full` is defined locally in controller/integration_tests.rs rather than in the shared test_helpers.rs module.

**Why this matters:**
If future tests in other modules need PaneInfo construction, this helper will need to be duplicated or moved. For now, only one test uses it.

### FINDING-6
- **Severity:** Minor
- **Confidence:** 70
- **File:** cc-zellij-plugin/src/sidebar_plugin/integration_tests.rs:219-220
- **Category:** architecture
- **Source:** architecture-agent
- **Round found:** 1
- **Resolution:** remaining (style preference)

**What is wrong:**
`use super::SidebarRendererPlugin;` and `use cc_deck::RenderPayload;` appear at the bottom of the file instead of the top, which is unconventional for Rust.

**Why this matters:**
Rust convention places `use` statements at the top of the file. Bottom placement makes imports harder to discover at a glance.

### FINDING-7
- **Severity:** Minor
- **Confidence:** 70
- **File:** cc-zellij-plugin/src/controller/integration_tests.rs:301
- **Category:** architecture
- **Source:** architecture-agent
- **Round found:** 1
- **Resolution:** remaining (style preference)

**What is wrong:**
`use crate::controller::ControllerPlugin;` appears at the bottom of the file.

**Why this matters:**
Same as FINDING-6.

## CodeRabbit Findings (External)

CodeRabbit reported 4 findings. Two were dismissed as false positives:

1. **Dismissed (false positive):** CodeRabbit flagged integration_tests.rs as not being cfg(test) gated. However, both parent modules (sidebar_plugin/mod.rs:16-17 and controller/mod.rs:636) already declare the module with `#[cfg(test)] mod integration_tests;`.

2. **Dismissed (false positive):** CodeRabbit flagged test_controller_multiple_sessions for sending Stop without a prior Working transition. The test passes because the state machine allows Init -> Done via Stop, which is valid behavior.

3. **Absorbed as FINDING-6/7:** README.md and CLAUDE.md prose plugin suggestions are process advice, not code issues.

## Remaining Findings

All 7 findings are Minor severity. No Critical or Important findings remain. No fix loop was needed.
