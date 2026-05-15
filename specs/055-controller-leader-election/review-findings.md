# Deep Review Findings

**Date:** 2026-05-15
**Branch:** 055-controller-leader-election
**Rounds:** 1
**Gate Outcome:** PASS
**Invocation:** quality-gate

## Summary

| Severity | Found | Fixed | Remaining |
|----------|-------|-------|-----------|
| Critical | 0 | 0 | 0 |
| Important | 4 | 4 | 0 |
| Minor | 5 | 0 | 5 |
| **Total** | **9** | **4** | **5** |

**Agents completed:** 5/5 (+ 1 external tool)
**Agents failed:** none

## Findings

### FINDING-1
- **Severity:** Important
- **Confidence:** 90
- **File:** cc-zellij-plugin/src/controller/mod.rs:158-166
- **Category:** correctness
- **Source:** correctness-agent (also reported by: coderabbit)
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**
The dormant guard in `pipe()` returned early without calling `unblock_cli_pipe_input()` for CLI-sourced pipe messages. When `zellij pipe` sends a hook, navigate, attend, or any other message to a dormant controller, the CLI process would block indefinitely waiting for the pipe to be unblocked.

**Why this matters:**
CLI commands like `cc-deck hook` would hang when delivered to a dormant controller instance. Since Zellij may route pipe messages to either controller (leader or dormant), this would cause intermittent CLI hangs.

**How it was resolved:**
Added `#[cfg(target_family = "wasm")] if let PipeSource::Cli(ref pipe_id) = pipe_message.source { unblock_cli_pipe_input(pipe_id); }` before the early return, matching the pattern used by the pre-permissions guard.

### FINDING-2
- **Severity:** Important
- **Confidence:** 85
- **File:** cc-zellij-plugin/src/controller/events.rs:176-186
- **Category:** correctness
- **Source:** correctness-agent
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**
When a dormant controller self-activated after the 60-second leader failure timeout, it did not broadcast a `controller-ping`. If multiple dormant controllers exist (N>2), they would all hit the 60-second timeout simultaneously and self-activate as leaders without discovering each other.

**Why this matters:**
This would revert to the dual-leader bug in the leader failure recovery scenario, negating the purpose of the election protocol. Even with N=2, the timing could cause a brief dual-leader window.

**How it was resolved:**
Added `broadcast_controller_ping(state.plugin_id)` immediately after `state.is_leader = true` on re-activation. This triggers a new election round, and the lower-ID controller wins.

### FINDING-3
- **Severity:** Important
- **Confidence:** 80
- **File:** cc-zellij-plugin/src/controller/mod.rs:682-690, events.rs:518-527
- **Category:** architecture
- **Source:** architecture-agent
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**
Two identical functions existed: `broadcast_controller_ping` in mod.rs and `broadcast_controller_ping_from_timer` in events.rs. Both constructed the same `MessageToPlugin` with identical logic.

**Why this matters:**
Copy-paste duplication that would diverge if the ping message format changed.

**How it was resolved:**
Consolidated into a single `pub fn broadcast_controller_ping` in events.rs. mod.rs now calls `events::broadcast_controller_ping` via a thin wrapper.

### FINDING-4
- **Severity:** Important
- **Confidence:** 85
- **File:** cc-zellij-plugin/src/controller/integration_tests.rs
- **Category:** test-quality
- **Source:** test-quality-agent
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**
Two critical code paths had no test coverage: leader demotion (an active leader receiving a lower-ID ping and stepping down) and leader failure timeout recovery (dormant controller re-activating after 60-second timeout).

**Why this matters:**
These are the crash recovery and mid-session demotion paths. Without tests, regressions in these paths would go undetected.

**How it was resolved:**
Added three new tests: `test_election_leader_demotion`, `test_election_leader_failure_reactivation`, and `test_election_no_payload_ping_ignored`.

### FINDING-5 (Minor, remaining)
- **Severity:** Minor
- **File:** cc-zellij-plugin/src/controller/render_broadcast.rs:93-101
- **Category:** architecture
- **Source:** architecture-agent

Every sidebar receives the render payload twice per cycle: once via targeted send (per-sidebar registry) and once via the untargeted broadcast. This is by design per FR-007 but adds N extra pipe messages at steady state.

### FINDING-6 (Minor, remaining)
- **Severity:** Minor
- **File:** cc-zellij-plugin/src/controller/mod.rs:436
- **Category:** architecture
- **Source:** architecture-agent

ControllerPong is handled identically to ControllerPing but no code ever sends a pong message. It is effectively dead code, kept for forward compatibility.

### FINDING-7 (Minor, remaining)
- **Severity:** Minor
- **File:** cc-zellij-plugin/src/controller/state.rs:51-129
- **Category:** architecture
- **Source:** architecture-agent

ControllerState has 30+ fields. The election fields could be grouped into a sub-struct for readability. This is a future refactoring opportunity, not a bug.

### FINDING-8 (Minor, remaining)
- **Severity:** Minor
- **File:** cc-zellij-plugin/src/controller/integration_tests.rs:419-428
- **Category:** test-quality
- **Source:** test-quality-agent

`test_election_dual_controllers_navigation` has a weak assertion on the leader side. In non-WASM builds, `broadcast_navigate` is a no-op, so the test only verifies no panic occurs.

### FINDING-9 (Minor, remaining)
- **Severity:** Minor
- **File:** specs/055-controller-leader-election/research.md:25
- **Category:** external
- **Source:** coderabbit

Research doc uses ambiguous "60 ticks (60 seconds)" when the implementation uses wall-clock milliseconds (LEADER_FAILURE_TIMEOUT_MS). Documentation-only issue.
