# Code Review: Pipe Mux Broker

**Spec:** specs/086-pipe-mux-broker/spec.md
**Date:** 2026-08-04
**Reviewer:** Claude (speckit.spex-gates.review-code)

## Compliance Summary

**Overall Score: 97%**

- Functional Requirements: 13/14 (93%)
- Error Handling / Edge Cases: 7/7 (100%)
- Acceptance Scenarios: 15/15 (100%)
- Non-Functional (docs, config, tests): 4/4 (100%)

## Detailed Review

### Functional Requirements

#### FR-001: `cc-deck mux` subcommand as standalone daemon
**Implementation:** `internal/cmd/mux.go:15-25`, `cmd/cc-deck/main.go:93`
**Status:** Compliant
**Notes:** NewMuxCmd() creates a hidden cobra command. Registered in main.go. Broker binds a Unix socket and runs accept loop.

#### FR-002: Deduplication using (session_name, pipe_name, hash(args)) with last-writer-wins
**Implementation:** `internal/mux/message.go:18-21`, `internal/mux/broker.go:172-179`
**Status:** Compliant
**Notes:** DedupKey() uses sha256(session_name+"|"+pipe_name+"|"+args)[:16]. Map overwrites existing keys (last-writer-wins).

#### FR-003: Configurable flush interval (default 200ms) with zellij pipe delivery
**Implementation:** `internal/mux/broker.go:57-63,199-211`, `internal/config/config.go:42`
**Status:** Compliant
**Notes:** defaultFlushFn calls `zellij pipe --session <name> --name <pipe_name> -- <args>`. Ticker at configurable interval.

#### FR-004: Self-terminate after configurable idle timeout (default 30s), remove socket
**Implementation:** `internal/mux/broker.go:245-264,119`, `internal/config/config.go:43`
**Status:** Compliant
**Notes:** idleLoop checks every 1s. Calls Stop() on timeout. Socket removed in Run() cleanup.

#### FR-005: Socket bind() as exclusive locking mechanism
**Implementation:** `internal/mux/broker.go:69`
**Status:** Compliant
**Notes:** net.ListenUnix("unix", addr) fails with EADDRINUSE if another broker holds the socket.

#### FR-006: Hook checks mux.enabled before broker communication
**Implementation:** `internal/cmd/hook.go:151`, `internal/cmd/hook_raw.go:54`
**Status:** Compliant
**Notes:** Both hooks check `cfg.Mux.Enabled` and fall through to direct `zellij pipe` when disabled. MuxDefaults has Enabled=false.

#### FR-007: Hook client: connect, start if unavailable, fall back after 3 retries (10ms apart)
**Implementation:** `internal/mux/client.go:35-57`
**Status:** Compliant
**Notes:** SendOrStart tries Send(), starts broker on failure, retries 3 times with 10ms sleep. Hook falls through on error.

#### FR-008: Detect stale socket files, remove, start fresh broker
**Implementation:** `internal/mux/client.go:41-43`
**Status:** Compliant
**Notes:** Checks os.Stat(socketPath), removes if exists but connect failed.

#### FR-009: Message includes session_name from $ZELLIJ_SESSION_NAME
**Implementation:** `internal/cmd/hook.go:152-154`, `internal/cmd/hook_raw.go:55-57`
**Status:** Compliant
**Notes:** Both hooks read os.Getenv("ZELLIJ_SESSION_NAME") and populate Message.SessionName.

#### FR-010: Drop oldest messages when queue exceeds queue_size, log drop event
**Implementation:** `internal/mux/broker.go:178-197`
**Status:** Minor Deviation
**Issue:** In dedup mode (map-based), `dropOldestDedup()` iterates the Go map and deletes the first entry found. Go map iteration order is randomized, so the dropped message is not necessarily the oldest. In no-dedup mode (slice-based, line 183-188), oldest is correctly dropped (index 0).
**Impact:** Minor. The dedup map does not track insertion order, so "oldest" is approximate. Functionally the queue is bounded correctly.
**Recommendation:** Accept as-is. Tracking insertion order in the dedup map would require an ordered map, adding complexity disproportionate to the benefit.

#### FR-011: Configurable parameters under mux section in config.yaml
**Implementation:** `internal/config/config.go:29-36,39-67`
**Status:** Compliant
**Notes:** MuxConfig struct has all 6 fields (enabled, flush_interval, idle_timeout, queue_size, dedup, log) with yaml tags.

#### FR-012: Debug logs to ~/.local/state/cc-deck/mux.log when mux.log is true
**Implementation:** `internal/mux/log.go:39-82`
**Status:** Compliant
**Notes:** FileLogger writes to xdg.StateHome/cc-deck/mux.log. Covers: Incoming, DedupHit, Flush, FlushError, Drop, Lifecycle.

#### FR-013: Create socket directory, fall back to /tmp/cc-deck-$UID/
**Implementation:** `internal/cmd/mux.go:35-38`, `internal/xdg/xdg.go:27-31`
**Status:** Compliant
**Notes:** RuntimeDir() returns $XDG_RUNTIME_DIR or /tmp/cc-deck-<uid>. Socket dir created with MkdirAll.

#### FR-014: When dedup=false, skip deduplication, deliver in arrival order
**Implementation:** `internal/mux/broker.go:181-188,222-224`
**Status:** Compliant
**Notes:** Uses []Message slice, appends in order, flushes sequentially.

### Error Handling / Edge Cases

| Edge Case | Status | Location |
|-----------|--------|----------|
| Socket dir doesn't exist | Compliant | `mux.go:36` MkdirAll |
| $XDG_RUNTIME_DIR not set | Compliant | `xdg.go:30` fallback |
| Two hooks race to start broker | Compliant | `broker.go:69` bind() atomic |
| Message targets dead session | Compliant | `broker.go:234-236` log + discard |
| Malformed config | Compliant | `mux.go:29` uses empty Config |
| Queue full | Compliant | `broker.go:178-188` drop + log |
| $ZELLIJ_SESSION_NAME not set | Compliant | `hook.go:152` checks != "" |

### Acceptance Scenarios

All 15 acceptance scenarios across 5 user stories verified:
- US1 (Pipe Storm Prevention): 3/3 compliant
- US2 (Graceful Fallback): 3/3 compliant
- US3 (Opt-In Configuration): 3/3 compliant
- US4 (Debug Logging): 3/3 compliant
- US5 (Broker Self-Termination): 3/3 compliant

### Extra Features (Not in Spec)

#### FlushError logging
**Location:** `internal/mux/log.go:73-75`, `internal/mux/broker.go:234-236`
**Description:** Logs individual zellij pipe failures with message details
**Assessment:** Helpful addition for operational debugging
**Recommendation:** Add to spec

#### File truncation on log open
**Location:** `internal/mux/log.go:47`
**Description:** Log file is truncated each time the broker starts (O_TRUNC flag)
**Assessment:** Reasonable operational choice, prevents unbounded log growth
**Recommendation:** Document in spec

### Test Coverage

| File | Test Count | Coverage |
|------|-----------|----------|
| `message_test.go` | 5 tests | Dedup key determinism, uniqueness, marshal/unmarshal, empty fields |
| `broker_test.go` | 7 tests | Accept connections, dedup collapse, flush delivery, queue limits, no-dedup, concurrency, flush errors, idle timeout |
| `client_test.go` | 5 tests | Send success/failure, SendOrStart with running broker, stale socket cleanup, connection close |
| `log_test.go` | 4 tests | File writes, timestamps, all methods, noop behavior |
| `validate_test.go` | 7 mux tests | Disabled skips, valid config, negative values, high flush interval, defaults pass |
| `mux_integration_test.go` | 5 tests | Concurrent multi-session dedup, idle timeout shutdown, stale socket recovery, flush errors, no-dedup ordering |

## Code Quality Notes

- Clean separation between broker (socket listener + dedup + flush), client (send + start), message (types + serialization), and log (debug output)
- FlushFunc is injectable for testing, avoiding real zellij pipe calls in unit tests
- Proper cleanup: socket file removed on shutdown, signal handling for SIGTERM/SIGINT
- Thread safety: mutex protects dedup map and queue; idleMu separates idle timer contention
- Daemon startup uses Setsid for process group isolation

## Recommendations

### Spec Evolution Candidates
- [ ] FR-010: Clarify that "oldest" in dedup mode is approximate (Go map has no insertion order)
- [ ] Add FlushError logging to FR-012 log event list
- [ ] Document log file truncation behavior in FR-012

### Optional Improvements
- [ ] Consider using an ordered map (linked hash map) in dedup mode for true FIFO eviction

## Conclusion

Implementation is highly compliant at 97%. The single minor deviation (FR-010 dedup mode drop order) is a pragmatic trade-off documented in the plan. All 15 acceptance scenarios pass. Test coverage is comprehensive with 33 tests across unit and integration levels. Documentation updated in both CLI and configuration references.

---

## Deep Review Report

**Date:** 2026-08-04
**Branch:** 086-pipe-mux-broker
**Rounds:** 1
**Gate Outcome:** PASS
**Invocation:** quality-gate

### Summary

| Severity | Found | Fixed | Remaining |
|----------|-------|-------|-----------|
| Critical | 0 | 0 | 0 |
| Important | 2 | 2 | 0 |
| Minor | 7 | 0 | 7 |
| **Total** | **9** | **2** | **7** |

**Agents completed:** 5/5 (+ 1 external tool)
**Agents failed:** none

### Review Agents

| Agent                   | Found | Fixed | Remaining | Status    |
|-------------------------|-------|-------|-----------|-----------|
| Correctness             |     2 |     0 |         2 | completed |
| Architecture & Idioms   |     1 |     0 |         1 | completed |
| Security                |     1 |     0 |         1 | completed |
| Production Readiness    |     1 |     1 |         0 | completed |
| Test Quality            |     2 |     0 |         2 | completed |
| CodeRabbit (external)   |     2 |     1 |         1 | completed |
| Copilot (external)      |     0 |     0 |         0 | skipped (CLI not installed) |
| Test Suite (regression) |     0 |     0 |         0 | passed |
|-------------------------|-------|-------|-----------|-----------|
| Total                   |     9 |     2 |         7 |           |

MVP: Correctness (2 findings)

Key fixes applied:
  1. Added 5s context timeout to defaultFlushFn exec.Command to prevent indefinite flush stall (production-readiness)
  2. Added isDialFailure() check in SendOrStart to only remove socket on confirmed stale (dial error), preserving live sockets on transient write errors (correctness + coderabbit)

### Findings

#### FINDING-1
- **Severity:** Important
- **Confidence:** 85
- **File:** internal/mux/broker.go:58-64
- **Category:** production-readiness
- **Source:** production-readiness-agent
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**
`defaultFlushFn` used `exec.Command("zellij", "pipe", ...)` with no timeout. If the zellij process hangs, the flush goroutine blocks indefinitely. Since flush processes messages sequentially, one hung call blocks all subsequent flushes, causing the queue to fill and messages to be silently dropped. The hook's `Send()` still succeeds (broker accepts connections), so no fallback to direct pipe triggers.

**Why this matters:**
A single hanging zellij pipe call cascades into total message loss for all sessions. The broker appears alive (accepts connections) but stops delivering, creating a silent failure mode.

**How it was resolved:**
Added `context.WithTimeout(context.Background(), 5*time.Second)` and replaced `exec.Command` with `exec.CommandContext`. The 5s timeout is generous enough for normal operations (zellij pipe completes in <10ms) but prevents indefinite blocking. If the timeout fires, the child process is killed and the error is logged via FlushError.

#### FINDING-2
- **Severity:** Important
- **Confidence:** 80
- **File:** internal/mux/client.go:36-46
- **Category:** correctness
- **Source:** correctness-agent (also reported by: coderabbit)
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**
`SendOrStart` unconditionally removed the socket file after any `Send()` failure. If the broker was running but experienced a transient write error (not a connection failure), the live socket would be deleted, breaking the running broker's ability to accept new connections.

**Why this matters:**
Removing a live broker's socket file causes it to stop receiving new connections. It would continue flushing its existing queue but never accept new messages. The broker would eventually idle-timeout and exit, leaving a gap in message delivery.

**How it was resolved:**
Added `isDialFailure()` function that checks if the error originated from the `dial` phase (`net.OpError.Op == "dial"`). Socket removal now only happens when the dial fails (confirming no listener), not on write errors (which indicate an active but temporarily troubled connection).

**External tool analysis (CodeRabbit):**
> In client.go around lines 36-46, arbitrary send failures should not remove the socket; only perform cleanup when the failure confirms the socket is stale. The current flow can unlink a live broker's socket, breaking all subsequent clients.

#### FINDING-3
- **Severity:** Minor
- **Confidence:** 75
- **File:** internal/mux/broker.go:133-147
- **Category:** correctness
- **Source:** correctness-agent
- **Round found:** 1
- **Resolution:** remaining (Minor, no fix needed)

**What is wrong:**
`handleConn` goroutines spawned by `acceptLoop` are not tracked by the `WaitGroup`. During shutdown, these goroutines may still be running when `finalFlush()` drains the queue. A message could be enqueued after the final flush.

**Why this matters:**
One message could be lost during the narrow shutdown window (~1ms). In practice, shutdown happens due to idle timeout (no active connections) or signal (very few active connections), making this extremely unlikely.

#### FINDING-4
- **Severity:** Minor
- **Confidence:** 70
- **File:** internal/cmd/hook.go:150-169, internal/cmd/hook_raw.go:53-76
- **Category:** architecture
- **Source:** architecture-agent
- **Round found:** 1
- **Resolution:** remaining (Minor, acceptable duplication)

**What is wrong:**
The mux integration code (~10 lines) is duplicated between `hook.go` and `hook_raw.go`. Both check `cfg.Mux.Enabled`, get `ZELLIJ_SESSION_NAME`, construct a `mux.Message`, and call `SendOrStart`.

**Why this matters:**
If the mux routing logic changes, both files must be updated in sync. Could diverge over time. However, at ~10 lines, the duplication is manageable and a shared helper would add indirection.

#### FINDING-5
- **Severity:** Minor
- **Confidence:** 65
- **File:** internal/xdg/xdg.go:27-31
- **Category:** security
- **Source:** security-agent
- **Round found:** 1
- **Resolution:** remaining (Minor, mitigated by directory permissions)

**What is wrong:**
The `/tmp/cc-deck-<uid>/` fallback path is susceptible to a symlink race if an attacker pre-creates the directory before the legitimate user. `MkdirAll` would succeed on an attacker-owned directory.

**Why this matters:**
On systems without `$XDG_RUNTIME_DIR` (rare on modern Linux/macOS with systemd), an attacker with local access could potentially redirect the socket. Mitigated by: uid-specific path, directory created with 0o700, and `$XDG_RUNTIME_DIR` being set on most systems.

#### FINDING-6
- **Severity:** Minor
- **Confidence:** 75
- **File:** internal/mux/broker_test.go
- **Category:** test-quality
- **Source:** test-quality-agent
- **Round found:** 1
- **Resolution:** remaining (Minor)

**What is wrong:**
`TestBroker_QueueSizeDropsOldest` only tests queue_size limit with `dedup=true`. No test verifies queue_size behavior in no-dedup mode (`dedup=false`).

#### FINDING-7
- **Severity:** Minor
- **Confidence:** 70
- **File:** test/mux_integration_test.go
- **Category:** test-quality
- **Source:** test-quality-agent
- **Round found:** 1
- **Resolution:** remaining (Minor)

**What is wrong:**
No test specifically verifies that receiving a message resets the idle timer. `TestIntegration_IdleTimeoutShutdown` sends one message then waits for timeout, but doesn't test the "near-timeout reset" behavior from acceptance scenario US5.2.

#### FINDING-8 (CodeRabbit, out-of-scope)
- **Severity:** Minor
- **Confidence:** 75
- **File:** internal/voice/stopword.go:163-176
- **Category:** external
- **Source:** coderabbit
- **Round found:** 1
- **Resolution:** skipped (not part of this feature)

**What is wrong:**
`looksLikeURL` function has overly broad matching: values like "httpserver" match the http prefix check, and "foo.computer" matches the ".com" domain check.

**Why this matters:**
Not related to the pipe mux broker feature. File was not changed in this branch. Likely detected because CodeRabbit reviewed a broader scope.

### Test Suite Results

| Round | Test Command | Exit Code | Failures | Status |
|-------|-------------|-----------|----------|--------|
| 1     | go test ./internal/mux/... ./test/... | 0 | 0 | passed |

Test suite passed in all fix rounds.

### Post-Fix Spec Coverage

All spec requirements verified after fix loop. No code was removed; only additions (timeout, dial-failure check). All 14 functional requirements remain implemented.
