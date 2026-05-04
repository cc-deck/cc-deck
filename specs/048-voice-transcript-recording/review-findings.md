# Deep Review Findings

**Date:** 2026-05-04
**Branch:** 048-voice-transcript-recording
**Rounds:** 0 (no fix loop needed)
**Gate Outcome:** PASS
**Invocation:** quality-gate

## Summary

| Severity | Found | Fixed | Remaining |
|----------|-------|-------|-----------|
| Critical | 0 | 0 | 0 |
| Important | 0 | 0 | 0 |
| Minor | 8 | - | 8 |
| **Total** | **8** | **0** | **8** |

**Agents completed:** 5/5 (+ 1 external tool: CodeRabbit)
**Agents failed:** none

## Findings

### FINDING-1
- **Severity:** Minor
- **Confidence:** 72
- **File:** cc-deck/internal/tui/voice/update.go:223-247
- **Category:** correctness
- **Source:** correctness-agent
- **Round found:** 1
- **Resolution:** remaining (minor, no action needed)

**What is wrong:**
The `updateFilenamePrompt` method consumes `relayEventMsg` events while the
filename prompt is visible, but only handles `level`, `transcription`, `muted`,
and `unmuted` event types. Events like `delivery`, `error`, and `target_changed`
are consumed by `waitForEvent` but silently dropped.

**Why this matters:**
If a transcription is in flight when the user presses `r` to start recording,
the delivery confirmation could arrive during the prompt phase and be lost from
the TUI history. The history entry would remain in "transcribed" status instead
of updating to "delivered". This is cosmetic since the text was still delivered.

### FINDING-2
- **Severity:** Minor
- **Confidence:** 90
- **File:** cc-deck/internal/tui/voice/model.go:33
- **Category:** architecture
- **Source:** architecture-agent
- **Round found:** 1
- **Resolution:** remaining (minor, no action needed)

**What is wrong:**
The `recPath` field in the `Model` struct is set when recording starts
(`update.go:200`) but never read anywhere in the codebase. It stores the
resolved file path but no code accesses it after assignment.

**Why this matters:**
This is dead state. It is harmless now but could cause confusion for future
maintainers who might assume `recPath` is used for display or logging. It may
be intended for future use (e.g., showing the filename in the header), in which
case it is acceptable forward-looking state.

### FINDING-3
- **Severity:** Minor
- **Confidence:** 78
- **File:** cc-deck/internal/tui/voice/transcript.go:30-43
- **Category:** security
- **Source:** security-agent
- **Round found:** 1
- **Resolution:** remaining (by design)

**What is wrong:**
`resolveTranscriptPath` does not sanitize path components like `..` in relative
filenames. A user could type `../../etc/foo` as a filename and the file would be
created outside the transcript directory.

**Why this matters:**
This is a local CLI tool where the user already has full filesystem access. The
user is deliberately choosing where to write their own transcript file. Path
traversal is only a vulnerability when untrusted input is involved. Here the
user is the input source. No action needed.

### FINDING-4
- **Severity:** Minor
- **Confidence:** 70
- **File:** cc-deck/internal/tui/voice/transcript.go:51-53
- **Category:** production-readiness
- **Source:** production-agent
- **Round found:** 1
- **Resolution:** remaining (acceptable trade-off)

**What is wrong:**
`writeTranscriptLine` uses `fmt.Fprintln` which writes to the OS file
descriptor but does not call `f.Sync()` to force a disk flush. If the process
crashes (not a clean exit), the last few lines could be lost.

**Why this matters:**
For a transcript recording feature, losing the last line or two in a crash
scenario is an acceptable trade-off. Users who want reliability can stop
recording cleanly with `R` or `q`. Adding `Sync()` on every line would add
I/O overhead for negligible benefit.

### FINDING-5
- **Severity:** Minor
- **Confidence:** 82
- **File:** cc-deck/internal/tui/voice/transcript_test.go:17-19
- **Category:** test-quality
- **Source:** test-agent
- **Round found:** 1
- **Resolution:** remaining (test still validates logic)

**What is wrong:**
`TestResolveTranscriptPath_Relative` sets `XDG_DATA_HOME` via `t.Setenv`, but
`xdg.DataHome` is a package-level variable that may be initialized at `init()`
time. If the xdg package reads the env var only once during init, the test's
`Setenv` call has no effect on the actual path used by `resolveTranscriptPath`.

**Why this matters:**
The test still validates that the function returns a path ending with the
filename and that the parent directory is created, so it is not a false pass.
However, it may be testing with the real XDG data directory rather than the temp
directory, which means the test creates a real directory as a side effect.

### FINDING-6
- **Severity:** Minor
- **Confidence:** 75
- **File:** cc-deck/internal/tui/voice/transcript_test.go
- **Category:** test-quality
- **Source:** test-agent
- **Round found:** 1
- **Resolution:** remaining (edge case covered by code review)

**What is wrong:**
There is no explicit test for the empty filename edge case. The spec says "when
the user enters an empty filename, the system uses a default name based on the
current timestamp." The code handles this at `update.go:181-183`, but no test
exercises pressing Enter with an empty text input.

**Why this matters:**
The code path is simple (three lines), so the risk of regression is low. A test
would add confidence but is not critical. The state machine tests cover the
prompt-to-recording transition, which implicitly exercises much of the same code
path.

### FINDING-7
- **Severity:** Minor
- **Confidence:** 80
- **File:** cc-deck/internal/tui/voice/transcript_test.go
- **Category:** test-quality
- **Source:** test-agent
- **Round found:** 1
- **Resolution:** remaining (simple code path)

**What is wrong:**
The spec requires that disk write errors stop recording gracefully. The code
was fixed during the compliance review to add `m.closeTranscript()` on write
error (`update.go:127`), but there is no test verifying this behavior.

**Why this matters:**
The fix is a single line addition to an existing error path. The risk of
regression is low, but a test would ensure the behavior is preserved if the
error handling is refactored. Given the simplicity, this is acceptable without
a dedicated test.

### FINDING-8
- **Severity:** Minor
- **Confidence:** 70
- **File:** cc-deck/internal/tui/voice/transcript_test.go:122-146
- **Category:** external
- **Source:** coderabbit
- **Round found:** 1
- **Resolution:** remaining (not a bug)

**What is wrong:**
`TestWriteTranscriptLine` reads the file using `os.ReadFile` while the file is
still open (closed via `defer f.Close()`). CodeRabbit suggests closing the file
explicitly before reading to avoid issues with buffered writes.

**Why this matters:**
`fmt.Fprintln` writes directly to the OS file descriptor without Go-level
buffering (it does not use `bufio.Writer`). The data is available for reading
immediately after the `Fprintln` call returns, even with the file still open.
The test is correct as written. Closing before reading would be slightly
cleaner but is not required for correctness.

**External tool analysis (CodeRabbit):**
> The test reads the temp file while it's still open (deferred f.Close()), which
> can hide buffered writes from writeTranscriptLine; fix by closing the file
> explicitly before calling os.ReadFile.

## Remaining Findings

All 8 findings are Minor severity. No Critical or Important findings remain.
No human action is required to unblock the review gate.
