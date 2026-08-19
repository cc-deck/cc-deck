# Deep Review Findings

**Date:** 2026-08-04
**Branch:** 086-pipe-mux-broker
**Rounds:** 1
**Gate Outcome:** PASS
**Invocation:** quality-gate

## Summary

| Severity | Found | Fixed | Remaining |
|----------|-------|-------|-----------|
| Critical | 0 | 0 | 0 |
| Important | 2 | 2 | 0 |
| Minor | 7 | 0 | 7 |
| **Total** | **9** | **2** | **7** |

**Agents completed:** 5/5 (+ 1 external tool)
**Agents failed:** none

## Findings

### FINDING-1
- **Severity:** Important
- **Confidence:** 85
- **File:** internal/mux/broker.go:58-64
- **Category:** production-readiness
- **Source:** production-readiness-agent
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**
`defaultFlushFn` used `exec.Command("zellij", "pipe", ...)` with no timeout. If the zellij process hangs, the flush goroutine blocks indefinitely. Since flush processes messages sequentially, one hung call blocks all subsequent flushes, causing the queue to fill and messages to be silently dropped.

**Why this matters:**
A single hanging zellij pipe call cascades into total message loss for all sessions. The broker appears alive (accepts connections) but stops delivering.

**How it was resolved:**
Added `context.WithTimeout(context.Background(), 5*time.Second)` and replaced `exec.Command` with `exec.CommandContext`. The 5s timeout prevents indefinite blocking while being generous enough for normal operations.

### FINDING-2
- **Severity:** Important
- **Confidence:** 80
- **File:** internal/mux/client.go:36-46
- **Category:** correctness
- **Source:** correctness-agent (also reported by: coderabbit)
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**
`SendOrStart` unconditionally removed the socket file after any `Send()` failure. If the broker was running but experienced a transient write error, the live socket would be deleted, breaking the running broker.

**Why this matters:**
Removing a live broker's socket file causes it to stop receiving new connections, creating a gap in message delivery.

**How it was resolved:**
Added `isDialFailure()` function that checks `net.OpError.Op == "dial"`. Socket removal now only happens on confirmed stale sockets (dial failure), not on write errors from active connections.

**External tool analysis (CodeRabbit):**
> In client.go around lines 36-46, arbitrary send failures should not remove the socket; only perform cleanup when the failure confirms the socket is stale.

### FINDING-3
- **Severity:** Minor
- **Confidence:** 75
- **File:** internal/mux/broker.go:133-147
- **Category:** correctness
- **Source:** correctness-agent
- **Round found:** 1
- **Resolution:** remaining

**What is wrong:**
`handleConn` goroutines spawned by `acceptLoop` are not tracked by the WaitGroup. During shutdown, a message could be enqueued after the final flush.

**Why this matters:**
One message could be lost during the narrow shutdown window (~1ms). In practice extremely unlikely since shutdown occurs due to idle timeout or signal.

### FINDING-4
- **Severity:** Minor
- **Confidence:** 70
- **File:** internal/cmd/hook.go:150-169, internal/cmd/hook_raw.go:53-76
- **Category:** architecture
- **Source:** architecture-agent
- **Round found:** 1
- **Resolution:** remaining

**What is wrong:**
The mux integration code (~10 lines) is duplicated between `hook.go` and `hook_raw.go`.

### FINDING-5
- **Severity:** Minor
- **Confidence:** 65
- **File:** internal/xdg/xdg.go:27-31
- **Category:** security
- **Source:** security-agent
- **Round found:** 1
- **Resolution:** remaining

**What is wrong:**
The `/tmp/cc-deck-<uid>/` fallback path is susceptible to a symlink race. Mitigated by uid-specific path and $XDG_RUNTIME_DIR being set on most systems.

### FINDING-6
- **Severity:** Minor
- **Confidence:** 75
- **File:** internal/mux/broker_test.go
- **Category:** test-quality
- **Source:** test-quality-agent
- **Round found:** 1
- **Resolution:** remaining

**What is wrong:**
No test for queue_size limit in no-dedup mode (`dedup=false`).

### FINDING-7
- **Severity:** Minor
- **Confidence:** 70
- **File:** test/mux_integration_test.go
- **Category:** test-quality
- **Source:** test-quality-agent
- **Round found:** 1
- **Resolution:** remaining

**What is wrong:**
No test specifically verifies that receiving a message resets the idle timer (US5.2).

### FINDING-8 (out-of-scope)
- **Severity:** Minor
- **Confidence:** 75
- **File:** internal/voice/stopword.go:163-176
- **Category:** external
- **Source:** coderabbit
- **Round found:** 1
- **Resolution:** skipped (not part of this feature)

**What is wrong:**
`looksLikeURL` has overly broad matching. Not related to the pipe mux broker feature.

## Test Suite Results

| Round | Test Command | Exit Code | Failures | Status |
|-------|-------------|-----------|----------|--------|
| 1     | go test ./internal/mux/... ./test/... | 0 | 0 | passed |

Test suite passed in all fix rounds. Pre-existing failures in unrelated packages (compose, voice relay) are not caused by this feature.

## Post-Fix Spec Coverage

All spec requirements verified after fix loop. No code was removed; only additions (timeout, dial-failure check). All 14 functional requirements remain implemented.
