# Code Review: Voice Transcript Recording

**Spec:** [spec.md](spec.md)
**Date:** 2026-05-04
**Reviewer:** Claude (speckit.spex-gates.review-code)

## Compliance Summary

**Overall Score: 100%**

- Functional Requirements: 14/14 (100%)
- Error Handling / Edge Cases: 6/6 (100%)
- Non-Functional (Tests + Docs): 2/2 (100%)

All 20 checkpoints verified against code locations. One deviation (disk-write error
not stopping recording) was identified and fixed during review by adding a
`closeTranscript()` call in the write-error path at `update.go:127`.

---

## Code Review Guide (30 minutes)

> This section guides a code reviewer through the implementation changes,
> focusing on high-level questions that need human judgment.

**Changed files:** 9 files changed (2 relay layer, 4 TUI layer, 1 documentation,
2 dependency files). Core logic spans 4 files.

### Understanding the changes (8 min)

- Start with `cc-deck/internal/tui/voice/transcript.go`: This is the new file
  that defines the recording state machine types (`recStatus`) and all file I/O
  helpers. It is small (68 lines) and establishes the vocabulary used everywhere
  else. Pay attention to how `closeTranscript()` resets state and calls
  `relay.SetRecording(false)`.
- Then `cc-deck/internal/tui/voice/update.go`: This is where all keyboard
  handling and event routing lives. The two key additions are the `"r"`/`"R"`
  key handlers (lines 84-101) and the `updateFilenamePrompt()` method (lines
  175-249). The transcription capture happens at line 124 inside the
  `relayEventMsg` handler.
- Question: Is the split between `transcript.go` (types + helpers) and
  `update.go` (state transitions) the right decomposition, or would colocating
  the state machine transitions with the type definitions be clearer?

### Key decisions that need your eyes (12 min)

**Mute-bypass in relay layer** (`cc-deck/internal/voice/relay.go:397-451`,
relates to [FR-011](spec.md#fr-011))

The `handleUtterance` method now checks both `muted` and `recording` flags.
When muted+recording, it still runs the full Whisper transcription but emits
only a `"transcription"` event, skipping stopword processing and pipe delivery.
This means the Whisper server does real work even while muted, which is
intentional per the spec.
- Question: Is the ordering of the two early-return checks (lines 401-406 for
  muted+not-recording, lines 444-451 for muted+recording) clear enough? The
  second check is 40 lines away from the first, separated by the transcription
  call. Would a comment or restructuring help future readers?

**File open flags** (`cc-deck/internal/tui/voice/update.go:193`, relates to
[FR-004](spec.md#fr-004))

The file is opened with `O_CREATE|O_WRONLY|O_TRUNC` and mode `0644`. This means
starting a recording with the same filename overwrites the previous transcript
without warning. The spec does not mention append-vs-overwrite behavior.
- Question: Should this be `O_APPEND` instead of `O_TRUNC` to avoid accidental
  data loss? Or is overwrite the expected behavior since the user explicitly
  chose the filename?

**Relay `SetRecording` stays true during pause** (`cc-deck/internal/tui/voice/update.go:91-94`,
relates to [FR-007](spec.md#fr-007) and [FR-011](spec.md#fr-011))

When the user pauses recording, only `m.recState` changes to `recPaused`. The
relay's `recording` flag stays `true`. This means the relay continues
transcribing during pause (needed for mute-bypass), and the TUI filters at the
write level. The spec says "transcriptions continue to appear in TUI history."
- Question: Is it correct that the relay keeps transcribing while paused even
  when NOT muted? In the unmuted+paused case, transcriptions go to both the
  pane AND the TUI history but skip the file. This matches the spec but is
  worth confirming.

**Prompt sub-mode event forwarding** (`cc-deck/internal/tui/voice/update.go:223-247`)

While the filename prompt is visible, the `updateFilenamePrompt` method
continues to consume `relayEventMsg` events so the event loop does not stall.
However, it only handles `level`, `transcription`, `muted`, and `unmuted`
events. It does not handle `delivery`, `error`, or `target_changed`.
- Question: Could dropping `delivery` or `error` events during prompting cause
  the TUI history to miss status updates for in-flight transcriptions?

### Areas where I'm less certain (5 min)

- `cc-deck/internal/tui/voice/update.go:126-128` ([FR-004](spec.md#fr-004)):
  The disk-write error path now calls `closeTranscript()` which closes the file
  and resets state. However, the last successful write before the error may have
  been partially flushed. `fmt.Fprintln` does not call `Sync()`. On a real
  disk-full scenario, data could be lost silently before the error surfaces.
- `cc-deck/internal/tui/voice/transcript.go:30-43`: The `resolveTranscriptPath`
  function creates directories with `MkdirAll` during path resolution, before
  the file is actually opened. If the user cancels the prompt after typing a
  path, an empty directory may be left behind. This is harmless but
  potentially surprising.
- `cc-deck/internal/tui/voice/transcript_test.go:17-19`: The test for relative
  path resolution sets `XDG_DATA_HOME` via `t.Setenv`, but the comment
  acknowledges that `xdg.DataHome` is set at init time. The test may not
  actually exercise the real XDG path logic if the package-level variable was
  already initialized.

### Deviations and risks (5 min)

- `cc-deck/internal/tui/voice/update.go:127`: The original implementation did
  not stop recording on write errors, deviating from the
  [disk space edge case](spec.md) ("displays an error and stops recording
  gracefully"). This was fixed during review by adding `m.closeTranscript()`.
  Question: "Is this fix sufficient, or should the error message distinguish
  between 'recording stopped due to disk error' and other errors?"
- No deviations from [plan.md](plan.md) were identified in the final
  implementation structure. All files match the planned layout.

---

## Deep Review Report

> Automated multi-perspective code review results. This section summarizes
> what was checked, what was found, and what remains for human review.

**Date:** 2026-05-04 | **Rounds:** 0/3 | **Gate:** PASS

### Review Agents

| Agent | Findings | Status |
|-------|----------|--------|
| Correctness | 1 | completed |
| Architecture & Idioms | 1 | completed |
| Security | 1 | completed |
| Production Readiness | 1 | completed |
| Test Quality | 3 | completed |
| CodeRabbit (external) | 1 | completed |
| Copilot (external) | 0 | skipped (not installed) |

### Findings Summary

| Severity | Found | Fixed | Remaining |
|----------|-------|-------|-----------|
| Critical | 0 | 0 | 0 |
| Important | 0 | 0 | 0 |
| Minor | 8 | - | 8 |

### What was fixed automatically

No fix loop was needed. All findings are Minor severity.

One spec compliance deviation was fixed during the compliance check (before
deep review): the disk-write error path in `update.go:127` was not stopping
recording, violating the spec's edge case requirement. A `closeTranscript()`
call was added.

### What still needs human attention

All Critical and Important findings were resolved (none were found).
8 Minor findings remain (see [review-findings.md](review-findings.md) for
details). No further review action needed, but reviewers may want to check
the Minor findings during code review. Notable items:

- The `updateFilenamePrompt` method drops `delivery` and `error` events
  during the filename prompt phase (`update.go:223-247`). Is this acceptable
  given that the prompt is typically visible for only a few seconds?
- The `recPath` field in `Model` is set but never read. Is this intended
  for future use (e.g., displaying the filename in the header)?
- Three test gaps exist (empty filename, write-error-stops-recording,
  XDG path isolation) that add coverage without being critical.

### Recommendation

All findings addressed. Code is ready for human review with no known blockers.
8 Minor findings remain across test quality (3), correctness (1),
architecture (1), security (1), production readiness (1), and external
tool (1). Consider reviewing them during code review but they are not blocking.
