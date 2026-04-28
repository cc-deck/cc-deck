# Code Review: 042-voice-relay

---

## Deep Review Report (Round 2)

> Automated multi-perspective code review results. This section summarizes
> what was checked, what was found, and what remains for human review.

**Date:** 2026-04-28 | **Rounds:** 1/3 | **Gate:** FAIL

### Review Agents

| Agent | Findings | Status |
|-------|----------|--------|
| Correctness | 6 | completed |
| Architecture & Idioms | 8 | completed |
| Security | 1 | completed |
| Production Readiness | 7 | completed |
| Test Quality | 6 | completed |
| CodeRabbit (external) | 0 | failed (152 files > 150 limit) |
| Copilot (external) | 0 | skipped (not installed) |

### Findings Summary

| Severity | Found | Fixed | Remaining |
|----------|-------|-------|-----------|
| Critical | 0 | 0 | 0 |
| Important | 9 | 5 | 4 |
| Minor | 13 | 2 | 11 |

### What was fixed automatically

Fixed PTT goroutine leak where the `SendReceive` long-poll in `collectPTTUtterance` blocked forever if the user quit without pressing F8 again (correctness/production agents). Added `stopCtx` with deferred cancel to interrupt the goroutine on all exit paths.

Fixed malgo audio source close-channel race where the data callback could send on a closed channel after device teardown timed out (correctness/production agents). Added `closing` atomic flag checked before channel sends.

Fixed unsanitized terminal text injection where transcribed text from whisper-server could contain ANSI escape sequences or control characters (security agent). Added `sanitizeTerminalText()` that strips non-printable characters and escape codes.

Removed unexplained `*2` multiplier on PTT `maxSamples` that silently doubled the configured max utterance duration (correctness agent).

Fixed device picker misleading UX by labeling it "Audio Devices (read-only)" since no device switching API exists (architecture agent). Strengthened delivery error test assertion and renamed misnamed test (test quality agent).

### What still needs human attention

- The `PipeSender`/`PipeSendReceiver` interface duplication is intentional to avoid `voice` -> `ws` package dependency. Is this boundary worth the duplication?
- No test coverage for FR-018 (auto-restart on backend crash up to 3 retries). The `WhisperServer.Restart()` boundary condition needs verification.
- No test coverage for FR-007 (permission prompt pauses text relay). Is pause behavior implemented at the TUI layer rather than relay?
- The WaitGroup reassignment in `Start()` after timed-out `stopInternal()` is a latent panic risk during rapid mode switching. Consider a done-channel pattern in a follow-up.

### Recommendation

4 Important findings remain (interface design decision, 2 test coverage gaps, WaitGroup fragility). Consider reviewing them during code review but the correctness and security fixes applied in this round address the most actionable issues. See [review-findings.md](review-findings.md) for details.
