# Deep Review Findings

**Date:** 2026-05-14
**Branch:** 053-render-pipeline-stability
**Rounds:** 1
**Gate Outcome:** PASS
**Invocation:** quality-gate

## Summary

| Severity | Found | Fixed | Remaining |
|----------|-------|-------|-----------|
| Critical | 0 | 0 | 0 |
| Important | 2 | 2 | 0 |
| Minor | 9 | 2 | 7 |
| **Total** | **11** | **4** | **7** |

**Agents completed:** 5/5 (+ 1 external tool)
**Agents failed:** none

## Findings

### FINDING-1
- **Severity:** Important
- **Confidence:** 90
- **File:** cc-zellij-plugin/src/sidebar_plugin/mod.rs:88-101
- **Category:** correctness
- **Source:** correctness-agent
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**
The one-shot render request fallback (FR-009) required `controller_plugin_id` to be `Some(...)` in order to send the request. In the exact scenario this fallback targets (no render payload received within 3 ticks), `controller_plugin_id` would be `None` since it is only learned from render payloads or sidebar-init messages. The fallback was silently skipped, defeating the safety net.

**Why this matters:**
FR-009 requires the sidebar to send a render request after 3 ticks if no payload is received. Without this working, sidebars could stay in "Connecting..." state indefinitely if push-on-discovery also failed.

**How it was resolved:**
Changed `send_render_request` to accept `Option<u32>` for controller_plugin_id. When `Some`, sends a targeted message. When `None`, sends a broadcast that any controller can receive. Also removed the redundant `!self.state.initialized` check (always true due to early return above).

### FINDING-2
- **Severity:** Important
- **Confidence:** 92
- **File:** cc-zellij-plugin/src/controller/events.rs:220-229
- **Category:** correctness
- **Source:** correctness-agent
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**
The fade throttle marked `render_dirty` every 5 ticks if ANY session had `Activity::Done`, `Activity::AgentDone`, or `Activity::Idle`. The `Activity::Idle` state is terminal, and once fading completes (`elapsed >= idle_fade_secs`), `faded_color()` returns a clamped value that never changes. Yet the code continued to broadcast identical payloads every 5 seconds as long as any Idle session existed.

**Why this matters:**
This violated SC-003 ("Render broadcast frequency drops to zero per second when no state changes are occurring and fade animations have completed"). With idle sessions present (the steady-state norm), the controller never stopped broadcasting, undermining the CPU optimization goal.

**How it was resolved:**
Added elapsed time checks to the fade throttle condition. Done/AgentDone sessions only trigger re-render when `elapsed < done_timeout`. Idle sessions only trigger when `elapsed < idle_fade_secs`. Once fade completes, the condition is false and broadcasts stop.

### FINDING-3
- **Severity:** Minor
- **Confidence:** 75
- **File:** cc-zellij-plugin/src/controller/mod.rs:433-444
- **Category:** correctness
- **Source:** correctness-agent
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**
`broadcast_controller_ping` sends to ALL plugin instances including itself. The self-ping/pong creates unnecessary round-trips on every startup.

**How it was resolved:**
Added `if sender_id == self.state.plugin_id { return false; }` guard at the top of the `ControllerPing` handler.

### FINDING-4
- **Severity:** Minor
- **Confidence:** 78
- **File:** cc-zellij-plugin/src/controller/sidebar_registry.rs:124-140
- **Category:** architecture
- **Source:** correctness-agent, architecture-agent, prod-readiness-agent
- **Round found:** 1
- **Resolution:** remaining

**What is wrong:**
`discover_sidebars_from_manifest` registers every plugin pane that is not the controller as a sidebar, including third-party plugins that are not cc-deck sidebars.

**Why this matters:**
Sends unnecessary render payloads to non-sidebar plugins. Low practical impact since typical cc-deck layouts only contain cc-deck plugins, but adds overhead proportional to the number of non-cc-deck plugins.

### FINDING-5
- **Severity:** Minor
- **Confidence:** 90
- **File:** cc-zellij-plugin/src/controller/render_broadcast.rs:79,90
- **Category:** architecture
- **Source:** architecture-agent, prod-readiness-agent
- **Round found:** 1
- **Resolution:** remaining

**What is wrong:**
Serialization timing uses `unix_now_ms().saturating_mul(1000)` to fake microseconds. The metric can only measure at millisecond boundaries, making sub-ms serializations report as 0.

### FINDING-6
- **Severity:** Minor
- **Confidence:** 85
- **File:** cc-zellij-plugin/src/debug.rs:11
- **Category:** security
- **Source:** security-agent
- **Round found:** 1
- **Resolution:** remaining (pre-existing)

**What is wrong:**
`static mut DEBUG_ENABLED` uses `unsafe` blocks. Safe in single-threaded WASI but could use `AtomicBool` to eliminate unsafe.

### FINDING-7
- **Severity:** Minor
- **Confidence:** 95
- **File:** cc-zellij-plugin/src/debug.rs:67-79
- **Category:** production-readiness
- **Source:** prod-readiness-agent
- **Round found:** 1
- **Resolution:** remaining (pre-existing)

**What is wrong:**
Debug log file grows without bound when debug is enabled. No rotation or size cap.

### FINDING-8
- **Severity:** Minor
- **Confidence:** 70
- **File:** cc-zellij-plugin/src/sidebar_plugin/mod.rs:104
- **Category:** production-readiness
- **Source:** prod-readiness-agent
- **Round found:** 1
- **Resolution:** remaining

**What is wrong:**
Sidebar timer fires indefinitely if the controller never responds. Each sidebar instance fires 1-second timers doing nothing after the render request is sent.

### FINDING-9
- **Severity:** Minor
- **Confidence:** 80
- **File:** cc-zellij-plugin/src/controller/integration_tests.rs
- **Category:** test-quality
- **Source:** test-quality-agent
- **Round found:** 1
- **Resolution:** remaining

**What is wrong:**
Missing integration test for the controller's `RenderRequest` handler and missing edge case test for equal plugin_id in startup probe.

### FINDING-10
- **Severity:** Minor
- **Confidence:** 82
- **File:** cc-zellij-plugin/src/controller/sidebar_registry.rs
- **Category:** test-quality
- **Source:** test-quality-agent
- **Round found:** 1
- **Resolution:** remaining

**What is wrong:**
`discover_sidebars_from_manifest` tests only cover single-tab scenarios. Multi-tab test would catch tab-position mapping errors.

### FINDING-11
- **Severity:** Minor
- **Confidence:** 90
- **File:** controller/state.rs, controller/events.rs, controller/sidebar_registry.rs
- **Category:** architecture
- **Source:** architecture-agent
- **Round found:** 1
- **Resolution:** remaining

**What is wrong:**
`make_pane_info` test helper is duplicated across three test modules with minor field value differences.
