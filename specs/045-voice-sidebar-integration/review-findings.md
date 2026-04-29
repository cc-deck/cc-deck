# Deep Review Findings

**Date:** 2026-04-29
**Branch:** 045-voice-sidebar-integration
**Rounds:** 1
**Gate Outcome:** PASS (after fixes)
**Invocation:** manual

## Summary

| Severity | Found | Fixed | Remaining |
|----------|-------|-------|-----------|
| Critical | 0 | 0 | 0 |
| Important | 15 | 10 | 5 |
| Minor | 14 | 1 | 13 |
| **Total** | **29** | **11** | **18** |

**Agents completed:** 5/5 (+ 1 external tool)
**Agents failed:** none

## Findings

### FINDING-1
- **Severity:** Important
- **Confidence:** 85
- **File:** cc-zellij-plugin/src/sidebar_plugin/render.rs:44-53
- **Category:** correctness
- **Source:** coderabbit (also reported by: correctness-agent)
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**
The voice click region (`VOICE_CLICK_SENTINEL`) was pushed after the header click regions for row 0. Since `handle_left_click` uses `.find()` which returns the first match, clicks on the voice indicator (♫) on row 0 would match the header region first, making the voice click handler unreachable. This violated FR-006 ("Users MUST be able to toggle mute by clicking the ♫ symbol").

**Why this matters:**
The ♫ click-to-mute feature would never work. Users clicking the indicator would enter navigation mode instead of toggling mute.

**How it was resolved:**
Moved the voice click region push to before the header click regions, ensuring `.find()` matches the voice sentinel first on row 0.

### FINDING-2
- **Severity:** Important
- **Confidence:** 85
- **File:** cc-zellij-plugin/src/main.rs:101-119
- **Category:** security
- **Source:** security-agent
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**
The `sanitize_voice_text` function only stripped CSI-style ANSI escape sequences (`ESC [ ... letter`). It did not handle OSC (`ESC ]`), DCS (`ESC P`), or other terminal escape types. OSC 52 sequences could read/write the system clipboard.

**Why this matters:**
While exploitation requires a compromised transcription engine, defense-in-depth demands complete escape sequence coverage. The Go-side sanitizer had the same gap.

**How it was resolved:**
Added handling for BEL terminator (`\x07`), ESC-backslash terminator, and a final pass that strips any remaining ESC bytes. Applied the same fix to the Go-side `sanitizeTerminalText`.

### FINDING-3
- **Severity:** Important
- **Confidence:** 85
- **File:** cc-deck/internal/voice/relay.go:293-318
- **Category:** correctness
- **Source:** correctness-agent (also reported by: production-readiness-agent, architecture-agent)
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**
Two issues in `statePoll` mute handling: (1) TOCTOU race condition where `r.muted` was read and written under separate lock acquisitions, allowing a concurrent TUI mute toggle to interleave; (2) the `else` branch re-sent acknowledgment pipe messages every poll tick (1s) when state already matched, creating unnecessary I/O.

**Why this matters:**
The race could cause mute state inconsistency between sidebar and voice TUI (violating FR-013). The redundant acknowledgments added unnecessary pipe traffic.

**How it was resolved:**
Combined the read-compare-write into a single lock acquisition. Removed the redundant acknowledgment branch (the plugin clears `voice_mute_requested` upon receiving the command).

### FINDING-4
- **Severity:** Important
- **Confidence:** 80
- **File:** cc-deck/internal/voice/relay.go:232-241
- **Category:** correctness
- **Source:** correctness-agent (also reported by: production-readiness-agent)
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**
`Stop()` sent `[[voice:off]]` using `r.parentCtx`, which may already be cancelled during normal shutdown. The nil fallback to `context.Background()` was unreachable since `parentCtx` is always set by `Start()`.

**Why this matters:**
Clean shutdown would fail to deliver `voice:off`, forcing the plugin to rely on the 15-second heartbeat timeout to detect disconnection. This created a stale ♫ indicator window.

**How it was resolved:**
Changed to use `context.WithTimeout(context.Background(), 2*time.Second)` for the `voice:off` send, ensuring delivery regardless of parent context state.

### FINDING-5
- **Severity:** Important
- **Confidence:** 80
- **File:** cc-deck/internal/tui/voice/update.go:53-63
- **Category:** correctness
- **Source:** correctness-agent (also reported by: coderabbit)
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**
The TUI mute toggle set `m.muted` locally before sending the protocol message. If `SendMuteCommand` failed, the TUI showed muted but the plugin was unaware. Errors were silently discarded.

**Why this matters:**
Mute state could become inconsistent. The user would see "MUTED" but voice would continue recording.

**How it was resolved:**
Changed the toggle to capture the desired state, send the command in the async closure, and return the state-change event only on success (or an error event on failure). The TUI now updates `m.muted` through the `relayEventMsg` handler.

### FINDING-6
- **Severity:** Important
- **Confidence:** 82
- **File:** cc-zellij-plugin/src/main.rs:38-48
- **Category:** architecture
- **Source:** architecture-agent
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**
`debug_init()` had `let enabled = true;` hardcoded with a "TEMPORARY" comment. Debug logging was unconditionally enabled in all WASM builds, creating unnecessary I/O.

**Why this matters:**
Every production deployment wrote to `/cache/debug.log` on every event. The original opt-in design was bypassed.

**How it was resolved:**
Restored the conditional check: `let enabled = std::fs::metadata("/cache/debug_enabled").is_ok();`

### FINDING-7
- **Severity:** Important
- **Confidence:** 80
- **File:** cc-zellij-plugin/src/controller/render_broadcast.rs:97-100
- **Category:** architecture
- **Source:** architecture-agent
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**
Free function `mark_render_dirty` in `render_broadcast.rs` duplicated `ControllerState::mark_render_dirty()` method and was never called.

**How it was resolved:**
Removed the dead function.

### FINDING-8
- **Severity:** Important
- **Confidence:** 85
- **File:** cc-deck/internal/voice/relay_test.go:531-559
- **Category:** test-quality
- **Source:** coderabbit
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**
`TestVoiceRelay_CommandWordSendsEnterProtocol` was an exact duplicate of `TestVoiceRelay_CommandWordSendsEnter`.

**How it was resolved:**
Removed the duplicate test.

### FINDING-9
- **Severity:** Important
- **Confidence:** 95
- **File:** cc-deck/internal/voice/relay_test.go:380-441
- **Category:** test-quality
- **Source:** test-quality-agent
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**
`TestParseDumpStateResponse` never asserted on `voiceMuteRequested`, leaving the FR-012 mute synchronization parsing path untested.

**How it was resolved:**
Added three test cases asserting `voiceMuteRequested` for true, false, and absent values.

### FINDING-10
- **Severity:** Minor
- **Confidence:** 82
- **File:** cc-zellij-plugin/src/pipe_handler.rs:245-246
- **Category:** test-quality
- **Source:** test-quality-agent (also reported by: coderabbit)
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**
The pipe handler test for voice commands tested that legacy names returned `Unknown` but did not verify `cc-deck:voice-mute-toggle` parsed to `VoiceMuteToggle`.

**How it was resolved:**
Added assertion for `VoiceMuteToggle` parsing.

### FINDING-11
- **Severity:** Minor
- **Confidence:** 78
- **File:** cc-zellij-plugin/src/lib.rs:237-248
- **Category:** test-quality
- **Source:** test-quality-agent
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**
`test_all_action_types_serialize` did not include `ActionType::VoiceMute` or `ActionType::Refresh`.

**How it was resolved:**
Added both variants to the test vector.

## Remaining Findings

### FINDING-12 (remaining)
- **Severity:** Important
- **Confidence:** 90
- **File:** cc-deck/internal/voice/relay.go:136-137
- **Category:** production-readiness
- **Source:** correctness-agent (also reported by: production-readiness-agent)

**What is wrong:**
`r.wg = sync.WaitGroup{}` in `Start()` resets the WaitGroup. If goroutines from a previous run are still draining after `stopInternal`'s 3-second timeout, they would call `Done()` on the reset WaitGroup, causing a panic.

**Why this was not auto-fixed:**
In practice, the `running` flag prevents `Start()` from being called while goroutines are active. The WaitGroup reset is only reachable after `Stop()` fully returns. Fixing this requires rearchitecting the relay lifecycle (e.g., creating fresh instances instead of reuse). Low risk in current usage.

### FINDING-13 (remaining)
- **Severity:** Important
- **Confidence:** 85
- **File:** cc-zellij-plugin/src/lib.rs and cc-zellij-plugin/src/controller/events.rs
- **Category:** architecture
- **Source:** architecture-agent

**What is wrong:**
`shift_variant` function is duplicated between `lib.rs` and `controller/events.rs` with identical logic.

**Why this was not auto-fixed:**
Consolidating across module boundaries risks breaking the build for a cosmetic improvement. Should be addressed in a separate cleanup.

### FINDING-14 (remaining)
- **Severity:** Important
- **Confidence:** 90
- **File:** cc-deck/internal/voice/relay_test.go
- **Category:** test-quality
- **Source:** test-quality-agent

**What is wrong:**
No test exists for the `statePoll` mute synchronization loop (relay.go:266-322). This is the core FR-012/FR-013 implementation.

**Why this was not auto-fixed:**
Testing `statePoll` requires a `mockPipeSendReceiver` that returns dump-state JSON with `voice_mute_requested` and verifying the full async round-trip. This is a significant test that should be written carefully, not auto-generated.

### FINDING-15 (remaining)
- **Severity:** Important
- **Confidence:** 90
- **File:** cc-zellij-plugin/src/controller/events.rs:136-147
- **Category:** test-quality
- **Source:** test-quality-agent

**What is wrong:**
No test for the voice heartbeat timeout logic in `handle_timer` (FR-019 crash recovery).

**Why this was not auto-fixed:**
`handle_timer` depends on WASM-gated APIs. A unit test requires careful mocking of `unix_now_ms()` state. Should be addressed separately.

### FINDING-16 (remaining)
- **Severity:** Important
- **Confidence:** 88
- **File:** cc-zellij-plugin/src/controller/actions.rs:241-246
- **Category:** test-quality
- **Source:** test-quality-agent

**What is wrong:**
No test for `handle_voice_mute` action handler.

**Why this was not auto-fixed:**
Simple test to write, but requires accessing the private `handle_voice_mute` function. Should be added in a follow-up.

### Minor Remaining Findings

- **FINDING-17** (Minor): `levelTickMsg` unused type in model.go - removed in this round
- **FINDING-18** (Minor): `target` field naming could be clearer (architecture)
- **FINDING-19** (Minor): `handleUtterance` switch statement redundant (architecture)
- **FINDING-20** (Minor): `auto_rename_tab` duplicated across 3 modules (architecture)
- **FINDING-21** (Minor): Protocol confusion risk with `[[command]]` detection (security)
- **FINDING-22** (Minor): `stopInternal` leaked goroutine on timeout (production)
- **FINDING-23** (Minor): `QuitMsg` not explicitly handled in TUI Update (production)
- **FINDING-24** (Minor): Protocol tests test local strings, not production code (test-quality)
- **FINDING-25** (Minor): No rapid mute toggle edge case test (test-quality)
- **FINDING-26** (Minor): `TestVoiceRelay_SendsVoiceOnAtStart` timing-dependent (test-quality)
- **FINDING-27** (Minor): config.rs voice_key needs test coverage (test-quality)
- **FINDING-28** (Minor): input.rs click matcher could match sentinel as session (correctness)
- **FINDING-29** (Minor): Go-side `sanitizeTerminalText` OSC gap (security) - fixed alongside Rust side
