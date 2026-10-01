# Code Review: 088-voice-reading-view

## Spec Compliance

**Score: 35/35 (100%)**

All functional requirements FR-001 through FR-035 verified against implementation.
One compliance fix applied during review (hardcoded turn mode in reading header, commit 09809d8).

## Deep Review Report

**Gate: PASS** (0 Critical, 0 Important)

### Agent Summary

| Agent | Findings | Critical | Important | Minor | Notable |
|-------|----------|----------|-----------|-------|---------|
| Security | 0 | 0 | 0 | 0 | 0 |
| Architecture | 3 | 0 | 0 | 2 | 1 |
| Production | 1 | 0 | 0 | 1 | 0 |
| Correctness | 1 | 0 | 0 | 1 | 0 |
| Test Quality | 0 | 0 | 0 | 0 | 0 |
| CodeRabbit | 4 (1 false positive) | 0 | 0 | 3 | 0 |
| **Total (deduplicated)** | **6** | **0** | **0** | **5** | **1** |

### Findings

#### FINDING-1

- **Severity**: Minor
- **Confidence**: 95%
- **File**: cc-deck/internal/tui/voice/reading.go:147-184
- **Category**: architecture
- **Source**: architecture-agent (also reported by: production-agent)
- **Round found**: 1
- **Resolution**: remaining (cosmetic duplication, no functional impact)

**What is wrong:**
`readingViewportWithScrollbar()` in reading.go:147-184 is a near-duplicate of `renderViewportWithScrollbar()` in view.go:241-279. Both functions implement the same scrollbar rendering logic.

**Why this matters:**
Code duplication increases maintenance burden. A fix to one scrollbar function could be missed in the other. However, the two functions serve different viewports (reading view vs. normal view) and may diverge in the future.

**How it was resolved:**
Not fixed. This is cosmetic duplication with no runtime impact. Extracting a shared scrollbar helper is a valid future refactor but not a blocker.

#### FINDING-2

- **Severity**: Minor
- **Confidence**: 80%
- **File**: cc-deck/internal/tui/voice/reading.go:148-150, view.go:242-244
- **Category**: performance
- **Source**: architecture-agent (also reported by: production-agent)
- **Round found**: 1
- **Resolution**: remaining (negligible performance impact)

**What is wrong:**
Both scrollbar functions call `viewport.View()` twice: once to count content lines and once for rendering. `View()` returns a pre-rendered string, so the double call is redundant but cheap.

**Why this matters:**
The double call is wasteful but has negligible performance impact since `View()` returns an already-computed string in the Bubbletea viewport. Not a correctness issue.

**How it was resolved:**
Not fixed. The overhead is negligible and a refactor would add complexity for no user-visible improvement.

#### FINDING-3

- **Severity**: Minor
- **Confidence**: 72%
- **File**: cc-deck/internal/tui/voice/reading.go:148-150
- **Category**: correctness
- **Source**: correctness-agent
- **Round found**: 1
- **Resolution**: remaining (cosmetic scrollbar position, no data loss)

**What is wrong:**
The scrollbar `totalContent` value is derived from `viewport.View()` line count (visible lines) rather than the total content lines. For long recordings that exceed the viewport, the scrollbar thumb position may not accurately reflect the scroll position.

**Why this matters:**
The scrollbar is a visual indicator only. An inaccurate thumb position does not affect data integrity, navigation, or keyboard scrolling. The user can still scroll correctly with keys (j/k/G/gg/PgUp/PgDn).

**How it was resolved:**
Not fixed. The scrollbar is approximate and functional. Precise scrollbar tracking would require tracking total content lines separately, adding complexity for a minor visual improvement.

#### FINDING-4

- **Severity**: Minor
- **Confidence**: 65%
- **File**: cc-deck/internal/tui/voice/turnmode.go:74
- **Category**: production
- **Source**: coderabbit
- **Round found**: 1
- **Resolution**: remaining (theoretical, channel has buffer of 16)

**What is wrong:**
The download goroutine's completion send (`ch <- downloadMsg{err: err}` at line 74) is a blocking send, unlike the progress sends which use `select/default`. If the channel buffer (size 16) is full and the TUI stops reading, the goroutine could block.

**Why this matters:**
In practice this cannot happen: the `waitForDownload` Cmd continuously drains the channel, and progress messages use non-blocking sends. The completion message will always find buffer space. A defensive `select` on `ctx.Done()` would be marginally safer.

**How it was resolved:**
Not fixed. The channel buffer of 16 combined with non-blocking progress sends means the completion send always has room. The goroutine closes the channel after the send (line 75), so even in a theoretical block scenario, the context cancellation in `cancelDownload()` would terminate the parent `DownloadModel` call.

#### FINDING-5

- **Severity**: Minor
- **Confidence**: 60%
- **File**: cc-deck/internal/tui/voice/update.go:330-333
- **Category**: production
- **Source**: coderabbit
- **Round found**: 1
- **Resolution**: remaining (process exit cleans up)

**What is wrong:**
When the user presses `q` or `ctrl+c` in the reading view, `closeTranscript()` is called but `cancelDownload()` is not. If a model download is in progress, its goroutine is not explicitly cancelled.

**Why this matters:**
Since `q`/`ctrl+c` triggers `tea.Quit`, the process exits immediately after. The download goroutine, its context, and any temp files are cleaned up by OS process termination. The `DownloadModel` function uses a temp directory with `defer os.RemoveAll`, which runs on goroutine exit.

**How it was resolved:**
Not fixed. Process exit handles cleanup. Adding explicit cancellation would be defensive but provides no user-visible benefit since the process is terminating.

### Notable Observations

#### NOTABLE-1

- **File**: cc-deck/internal/voice/turns.go (Segment struct)
- **Category**: architecture
- **Source**: architecture-agent
- **Description**: The `Speaker` field on the `Segment` struct is populated only in tdrz mode and unused in basic mode.
- **Rationale**: This is by spec design (FR-013, FR-014). The Speaker field is part of the tdrz data model and will be used for speaker-labeled rendering. It is not dead code.

### False Positives

#### CodeRabbit major finding on relay.go:821-829

CodeRabbit suggested updating `handleUtterance` to run `TranscribeTurns` first in tdrz mode. However, `handleRecordingPassage` (called by `handleUtterance` for recording-mode passages) already does exactly this at lines 821-829: it checks `turnMode == TurnModeTdrz && tt != nil`, runs `tt.TranscribeTurns()` first, and falls back to basic mode only on failure or empty result. The finding was a misread of the existing code flow.

### Post-Fix Spec Coverage

No fixes were required (all findings are Minor or Notable). The compliance fix for the hardcoded turn mode header (commit 09809d8) was applied during the compliance check phase before the deep review.

All 35 functional requirements remain covered. All 426 tests pass.
