# Deep Review Findings

**Date:** 2026-04-28
**Branch:** 042-voice-relay
**Rounds:** 1 (round 2)
**Gate Outcome:** FAIL (4 remaining Important, all test gaps or design decisions)
**Invocation:** manual

## Summary

| Severity | Found | Fixed | Remaining |
|----------|-------|-------|-----------|
| Critical | 0 | 0 | 0 |
| Important | 9 | 5 | 4 |
| Minor | 13 | 2 | 11 |
| **Total** | **22** | **7** | **15** |

**Agents completed:** 5/5 (+ 0 external tools)
**Agents failed:** CodeRabbit (152 files exceeded 150-file limit)

## Findings

### FINDING-1
- **Severity:** Critical
- **Confidence:** 90
- **File:** cc-deck/internal/voice/server.go:52
- **Category:** correctness
- **Source:** coderabbit (also reported by: correctness-agent)
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**
The WhisperServer subprocess context was derived from the caller's context (`context.WithCancel(ctx)`), which caused the whisper-server process to be killed when the caller's context was cancelled. After `SwitchMode()` loses the parent context (FINDING-3), a `Restart()` call would use a detached context, making the server lifecycle unmanageable.

**Why this matters:**
If the caller's context is cancelled (e.g., SIGTERM during graceful shutdown), the whisper-server subprocess would be killed immediately. Combined with `SwitchMode()` using `context.Background()`, the server could become unmanageable since the cancel function would not be connected to anything meaningful.

**How it was resolved:**
Changed `context.WithCancel(ctx)` to `context.WithCancel(context.Background())` so the subprocess lifecycle is managed exclusively through `Stop()`/`stopLocked()` rather than through context inheritance. The server's `cancel` field controls termination.

**External tool analysis (CodeRabbit):**
> The subprocess context is currently derived from the passed ctx which causes the whisper-server process to be killed when the caller's request-scoped context is canceled. Change this to create the command with a background-based context so the subprocess can outlive Start.

---

### FINDING-2
- **Severity:** Critical
- **Confidence:** 85
- **File:** cc-deck/internal/voice/audio_ffmpeg.go:52-63
- **Category:** correctness
- **Source:** coderabbit
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**
The goroutine launched in `Start()` closed `s.stopped` via `s.closeOnce`, but both fields were accessed through the receiver pointer `s`. After a `Stop()`/`Start()` cycle, `s.stopped` and `s.closeOnce` were reset to new values. If the old goroutine's deferred `close(s.stopped)` executed after the new `Start()`, it would close the NEW stopped channel, prematurely signaling the new goroutine to exit.

**Why this matters:**
A Stop/Start race could cause the new audio source to appear stopped immediately after starting, silently breaking audio capture until the next restart.

**How it was resolved:**
Captured `stopped` and `closeOnce` into local variables at goroutine creation time so the deferred close operates on the correct channel instance, not whatever `s.stopped` points to at execution time.

---

### FINDING-3
- **Severity:** Important
- **Confidence:** 88
- **File:** cc-deck/internal/voice/relay.go:307-319
- **Category:** correctness
- **Source:** correctness-agent (also reported by: production-agent)
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**
`SwitchMode` called `r.Start(context.Background())` instead of reusing the parent context from the original `Start()` call. This disconnected the relay from the caller's signal-aware context, preventing graceful shutdown via Ctrl+C or SIGTERM after a mode toggle.

**Why this matters:**
After switching between VAD and PTT mode, pressing Ctrl+C or sending SIGTERM would not stop the relay, requiring a force-kill. The `cmd_context()` in `ws_voice.go` creates a context tied to OS signals, but `SwitchMode` replaced it with a non-cancellable background context.

**How it was resolved:**
Added `parentCtx` field to `VoiceRelay` struct, stored at `Start()` time, and reused in `SwitchMode()`.

---

### FINDING-4
- **Severity:** Important
- **Confidence:** 85
- **File:** cc-deck/internal/voice/relay.go:557-574
- **Category:** correctness
- **Source:** correctness-agent (also reported by: production-agent)
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**
`sendEvent` read `r.ctx` under mutex but `r.ctx` could be nil if `sendEvent` was called before `Start()` was ever invoked. The `NewVoiceRelay` constructor did not initialize `ctx`, so a call to `sendEvent` before `Start()` would cause a nil pointer dereference on `ctx.Done()`.

**Why this matters:**
Any goroutine that outlived `stopInternal()` (see FINDING-6) or called `sendEvent` during construction would panic.

**How it was resolved:**
Added nil guard: when `ctx` is nil, fall back to a select without `ctx.Done()`.

---

### FINDING-5
- **Severity:** Important
- **Confidence:** 85
- **File:** cc-deck/internal/voice/relay.go:249-304
- **Category:** production-readiness
- **Source:** production-agent (also reported by: security-agent, correctness-agent, coderabbit)
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**
Two issues in `collectPTTUtterance`: (1) The `allSamples` slice grew without limit during PTT recording, with no cap corresponding to `MaxUtteranceDuration` enforced in VAD mode. A stuck PTT key would consume unbounded memory at ~1.9 MB/minute. (2) The goroutine calling `sr.SendReceive` was not tracked in the WaitGroup, so `wg.Wait()` in `stopInternal()` would not wait for it.

**Why this matters:**
(1) Directly threatens the NFR-002 200 MB memory budget. A stuck key or forgotten session could exhaust memory. (2) Goroutine leak: after `stopInternal()` timed out, the untracked goroutine would continue running.

**How it was resolved:**
(1) Added `maxSamples` cap based on `MaxUtteranceDuration * 2 * sampleRate`. (2) Added `r.wg.Add(1)` and `defer r.wg.Done()` to the SendReceive goroutine.

---

### FINDING-6
- **Severity:** Important
- **Confidence:** 85
- **File:** cc-deck/internal/voice/relay.go:337-346
- **Category:** production-readiness
- **Source:** production-agent
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**
`stopInternal()` gave goroutines only 3 seconds via `r.wg.Wait()`, then proceeded without terminating them. The WaitGroup was never reset, so subsequent `Start()` calls (via `SwitchMode`) would inherit the abandoned count.

**Why this matters:**
Old goroutines continued running with references to audio and transcriber resources, potentially racing with newly started goroutines on the same relay state.

**How it was resolved:**
`Start()` now reinitializes `r.wg = sync.WaitGroup{}` before spawning new goroutines, ensuring each lifecycle has a fresh WaitGroup.

---

### FINDING-7
- **Severity:** Important
- **Confidence:** 88
- **File:** cc-deck/internal/voice/audio_malgo.go:99-108
- **Category:** correctness
- **Source:** correctness-agent (also reported by: production-agent, coderabbit)
- **Round found:** 1
- **Resolution:** fixed (round 2)

**What is wrong:**
The lifecycle goroutine at line 99 called `defer close(out)`. The `Stop()` method closed `s.stopped`, which unblocked the goroutine. However, the Data callback (running in miniaudio's audio thread) might still be trying to send to `out` between `s.stopped` closing and `close(out)` executing. Sending to a closed channel panics regardless of `default` case in select.

**Why this matters:**
Under high audio throughput or during rapid Stop/Start cycles, a panic in the audio callback thread would crash the entire process.

**How it was resolved:**
Removed `close(out)` from the lifecycle goroutine. Instead, `out` is stored as `s.out` and closed in `Stop()` after `dev.Uninit()` completes (or times out), guaranteeing the callback is no longer firing.

---

### FINDING-8
- **Severity:** Important
- **Confidence:** 82
- **File:** cc-deck/internal/voice/server.go:34-46
- **Category:** correctness
- **Source:** correctness-agent (also reported by: production-agent, coderabbit)
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**
`Start()` checked `s.cmd != nil` under lock (line 36), released the lock, called `Healthy()` without the lock (line 42), then re-acquired the lock for the rest of the function. Between the unlock and re-lock, another goroutine could start the server concurrently, leading to duplicate whisper-server processes.

**Why this matters:**
While concurrent `Start()` calls are unlikely in normal usage, `Restart()` calls `stopLocked()` then `Start()`, and `SwitchMode` could trigger concurrent restarts.

**How it was resolved:**
Added a second `s.cmd != nil` check after re-acquiring the lock (double-check locking pattern).

**External tool analysis (CodeRabbit):**
> The code releases s.mu, calls s.Healthy(ctx), then reacquires s.mu without re-checking s.cmd, allowing a race where another goroutine may have set s.cmd. After reacquiring the lock you must re-check s.cmd and return if non-nil.

---

### FINDING-9
- **Severity:** Important
- **Confidence:** 85
- **File:** cc-deck/internal/cmd/ws_voice.go:73-74
- **Category:** security
- **Source:** security-agent
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**
The verbose log file was created with mode `0644` (world-readable) at a deterministic path (`$XDG_STATE_HOME/cc-deck/voice.log`). Voice transcriptions logged in verbose mode may contain sensitive dictated content. NFR-005 states "transcribed text must not be logged by default," but even in opt-in verbose mode, the file was accessible to any user on the system.

**Why this matters:**
Voice transcriptions may contain passwords, PII, or confidential instructions. The deterministic path makes it trivially discoverable by other processes.

**How it was resolved:**
Changed directory permissions from `0755` to `0700` and file permissions from `0644` to `0600`.

---

### FINDING-10
- **Severity:** Important
- **Confidence:** 90
- **File:** cc-deck/internal/ws/channel_pipe.go:20-64
- **Category:** security
- **Source:** security-agent (also reported by: coderabbit)
- **Round found:** 1
- **Resolution:** remaining

**What is wrong:**
`execPipeChannel` passes `pipeName` and `payload` as arguments to `execFn`/`execOutputFn`. If the exec function implementation joins args into a shell string (common for SSH/kubectl exec), then transcribed voice text (user-controlled input) containing shell metacharacters could be interpreted.

**Why this matters:**
Transcribed voice text flows directly into command arguments. While `localPipeChannel` uses `exec.Command` (no shell), the `execPipeChannel` path depends on how the workspace backend joins arguments.

**What needs to happen:**
Audit all `execFn`/`execOutputFn` implementations to verify they use argument arrays (not shell strings). Consider adding payload sanitization at the `Send`/`SendReceive` boundary.

---

### FINDING-11
- **Severity:** Important
- **Confidence:** 85
- **File:** cc-zellij-plugin/src/controller/actions.rs:265-490
- **Category:** architecture
- **Source:** architecture-agent
- **Round found:** 1
- **Resolution:** remaining

**What is wrong:**
`perform_attend_directed` and `perform_working_directed` share ~120 lines of nearly identical control flow: candidate tier building, rapid-cycle visited-set logic, direction-based iteration with wrap-around. The only difference is filtering criteria.

**Why this matters:**
Changes to the direction iteration or visited-set logic must be duplicated in both functions, creating divergence risk.

**What needs to happen:**
Extract a generic `perform_directed_cycle` function that takes candidate tiers as input and handles iteration, visited-set, and wrap-around. Both functions would build tiers then delegate.

---

### FINDING-12
- **Severity:** Important
- **Confidence:** 75
- **File:** cc-deck/internal/cmd/ws_voice.go:69,119-128
- **Category:** architecture
- **Source:** architecture-agent
- **Round found:** 1
- **Resolution:** remaining

**What is wrong:**
The `--device` flag is accepted and the device name is resolved for TUI display, but the device ID is never passed to `AudioSource.Start()`. The audio source always uses the system default device regardless of the flag value.

**Why this matters:**
The CLI flag exists but has no effect, which is misleading to users who rely on it.

**What needs to happen:**
Either pass the device ID through to `AudioSource.Start()` (requiring an API change), or remove the `--device` flag until device selection is implemented.

---

### FINDING-13
- **Severity:** Important
- **Confidence:** 95
- **File:** cc-deck/internal/voice/relay_test.go
- **Category:** test-quality
- **Source:** test-agent
- **Round found:** 1
- **Resolution:** remaining

**What is wrong:**
No test coverage for `parseDumpStateResponse`, which contains JSON parsing with multiple branches (nil sessions, missing attended pane, activity detection, target name resolution). The function is pure and trivially testable.

**What needs to happen:**
Add `TestParseDumpStateResponse` table-driven test covering: valid JSON with paused state, non-paused state, missing `attended_pane_id`, empty sessions, and malformed JSON.

---

### FINDING-14
- **Severity:** Important
- **Confidence:** 90
- **File:** cc-deck/internal/voice/relay_test.go
- **Category:** test-quality
- **Source:** test-agent
- **Round found:** 1
- **Resolution:** remaining

**What is wrong:**
No test coverage for PTT mode (`startPTT`, `pttLoop`, `collectPTTUtterance`, `SwitchMode`). The existing mock infrastructure only implements `PipeSender`, not `PipeSendReceiver`.

**What needs to happen:**
Create a `mockPipeSendReceiver` and add tests for: PTT mode requiring PipeSendReceiver, PTT recording cycle, SwitchMode transitions.

---

### FINDING-15
- **Severity:** Important
- **Confidence:** 92
- **File:** cc-deck/internal/voice/relay_test.go
- **Category:** test-quality
- **Source:** test-agent
- **Round found:** 1
- **Resolution:** remaining

**What is wrong:**
No test for paused-state text discard behavior (FR-007/FR-016). The `handleUtterance` method does not check `r.paused` before sending, which is either a spec gap or missing feature.

**What needs to happen:**
Add a test that sets the relay into paused state and verifies utterances are discarded or queued per the spec.

## Remaining Findings

5 Important findings could not be auto-fixed:
- FINDING-10: Shell injection risk requires design decision on payload sanitization
- FINDING-11: Rust code duplication requires architectural refactor
- FINDING-12: `--device` flag is a feature gap
- FINDING-13-15: Test coverage gaps require writing new tests

16 Minor findings were identified but are not gate-blocking. These include dead code (`DroppedFrames`, `levelTickMsg`, `pttStateMsg`, legacy pipe variants), triplicated `rename_tab_wasm` helper, `TestInject` in production builds, Restart error message clarity, whisper-server crash detection, concurrent pipe subprocess spawning, test assertion weaknesses, and platform-specific install instructions. See agent reports for details.
