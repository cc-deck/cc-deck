# Deep Review Findings

**Date:** 2026-05-14
**Branch:** 054-voice-sync-reliability
**Rounds:** 1
**Gate Outcome:** PASS
**Invocation:** quality-gate

## Summary

| Severity | Found | Fixed | Remaining |
|----------|-------|-------|-----------|
| Critical | 0 | 0 | 0 |
| Important | 4 | 4 | 0 |
| Minor | 14 | 0 | 14 |
| **Total** | **18** | **4** | **14** |

**Agents completed:** 5/5 (+ 1 external tool)
**Agents failed:** none

## Findings

### FINDING-1
- **Severity:** Important
- **Confidence:** 95
- **File:** cc-zellij-plugin/src/controller/mod.rs:419-428, render_broadcast.rs:168-171
- **Category:** architecture
- **Source:** architecture-agent
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**
The `RenderRequest` handler duplicated the same three steps as the new `targeted_render()` function: build payload, serialize JSON, send to plugin. Two code paths doing the same thing will diverge.

**Why this matters:**
Future changes to render payload construction would need to be applied in two places, risking inconsistency.

**How it was resolved:**
Replaced the inline code in the `RenderRequest` handler with a call to `render_broadcast::targeted_render()`.

### FINDING-2
- **Severity:** Important
- **Confidence:** 95
- **File:** cc-zellij-plugin/src/controller/render_broadcast.rs (test)
- **Category:** test-quality
- **Source:** test-quality-agent
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**
Test `test_targeted_render_builds_and_sends` claimed to verify sending but `send_render_to_plugin` is a no-op in non-WASM test mode. The test only verified no-panic and payload construction.

**Why this matters:**
Misleading test name gives false confidence about what is actually verified.

**How it was resolved:**
Renamed to `test_targeted_render_builds_payload_without_panic` with a comment explaining the WASM test limitation.

### FINDING-3
- **Severity:** Important
- **Confidence:** 95
- **File:** cc-zellij-plugin/src/controller/sidebar_registry.rs (test)
- **Category:** test-quality
- **Source:** test-quality-agent, coderabbit
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**
Test `test_handle_sidebar_hello_sends_targeted_render` name overstated what it verified. Only checked registration, not render delivery (which is a no-op in test mode).

**Why this matters:**
Same issue as FINDING-2: misleading test name.

**How it was resolved:**
Renamed to `test_handle_sidebar_hello_registers_and_renders` with a comment about the WASM test limitation. Removed unused voice state setup.

### FINDING-4
- **Severity:** Important
- **Confidence:** 75
- **File:** cc-zellij-plugin/src/controller/mod.rs (test)
- **Category:** test-quality
- **Source:** test-quality-agent
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**
Test `test_voice_on_already_enabled_preserves_mute` had a misleading name (behavior changed: bare `voice:on` now clears mute) and dropped the `render_dirty` assertion.

**Why this matters:**
Missing assertion for FR-006 (muted->unmuted transition should mark dirty).

**How it was resolved:**
Renamed to `test_voice_on_bare_clears_mute_and_marks_dirty`, added `render_dirty` assertion.

### FINDING-5 (Minor, remaining)
- **Severity:** Minor
- **Confidence:** 85
- **File:** cc-zellij-plugin/src/controller/mod.rs:497-500
- **Category:** correctness
- **Source:** correctness-agent

`voice_mute_requested` is not cleared on subsequent heartbeats when voice is already enabled. The stale value persists in dump-state responses. Currently benign due to idempotent relay handling.

### FINDING-6 (Minor, remaining)
- **Severity:** Minor
- **Confidence:** 90
- **File:** cc-zellij-plugin/src/controller/render_broadcast.rs:168-171
- **Category:** architecture
- **Source:** architecture-agent

`send_render_to_plugin_pub` remains as a thin wrapper. Still used by `events.rs` for multi-sidebar batch sends (serializes once, sends to many). Kept intentionally.

### FINDING-7 (Minor, remaining)
- **Severity:** Minor
- **Confidence:** 95
- **File:** cc-deck/internal/voice/relay_test.go:777-788
- **Category:** architecture
- **Source:** architecture-agent

Dead code: `contains` and `searchString` helper functions are never called.

### FINDING-8 (Minor, remaining)
- **Severity:** Minor
- **Confidence:** 85
- **File:** cc-deck/internal/voice/relay.go:327-332
- **Category:** architecture
- **Source:** architecture-agent

`hasFocusedPane` and `hasAttendedPane` fields on `dumpStateResult` are only used in test assertions, not production code.

### FINDING-9 (Minor, remaining)
- **Severity:** Minor
- **Confidence:** 90
- **File:** cc-zellij-plugin/src/controller/mod.rs (test)
- **Category:** test-quality
- **Source:** test-quality-agent

`test_dump_state_includes_focused_pane_id` tests a local struct, not the actual `dump_state()` method. If the real struct diverges, the test would still pass.

### FINDING-10 (Minor, remaining)
- **Severity:** Minor
- **Confidence:** 80
- **File:** cc-deck/internal/voice/relay_test.go
- **Category:** test-quality
- **Source:** test-quality-agent

No integration-level test for end-to-end session targeting via `focused_pane_id`.

### FINDING-11 through FINDING-18 (Minor, remaining)
- **Severity:** Minor
- **File:** Various spec artifacts (research.md, REVIEW-PLAN.md, REVIEW-CODE.md, tasks.md)
- **Category:** external
- **Source:** coderabbit

Style issues: contractions in spec documents ("doesn't", "I'm") that violate the cc-deck no-contractions voice guideline. Also a dependency statement inconsistency in tasks.md.
