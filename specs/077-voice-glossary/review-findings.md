# Deep Review Findings

**Date:** 2026-07-03
**Branch:** 077-voice-glossary
**Rounds:** 1
**Gate Outcome:** PASS
**Invocation:** quality-gate

## Summary

| Severity | Found | Fixed | Remaining |
|----------|-------|-------|-----------|
| Critical | 0 | 0 | 0 |
| Important | 1 | 1 | 0 |
| Minor | 1 | 1 | 0 |
| Notable | 0 | - | 0 |
| **Total** | **2** | **2** | **0** |

**Agents completed:** 5/5 (+ 1 external tool)
**Agents failed:** none

## Findings

### FINDING-1
- **Severity:** Important
- **Confidence:** 85
- **File:** cc-deck/internal/voice/relay.go:347
- **Category:** correctness
- **Source:** coderabbit (also reported by: spec-compliance-review)
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**
The glossary prompt update in `statePoll` compared `state.workingDir` against `prevDir` without checking if `state.workingDir` was empty. When `parseDumpStateResponse` returns a zero-value result (e.g., during a transient ambiguous poll where no attended/focused pane is resolved), `state.workingDir` is `""`. If `prevDir` was previously set to a valid directory, the comparison `"" != "/some/project"` evaluates to true, triggering `ResolvePrompt("")` which returns only global terms, effectively clearing the project-specific prompt.

**Why this matters:**
In production, poll responses can transiently return empty results when the Zellij session is between states. This would cause the glossary to flicker between project-specific and global-only on every ambiguous poll tick, degrading Whisper recognition accuracy for the active project's terms.

**How it was resolved:**
Added `state.workingDir != ""` guard before the comparison, mirroring the existing `targetName` guard pattern at line 337. Empty working directories are now ignored, preserving the last known project glossary.

**External tool analysis (CodeRabbit):**
> The glossary prompt update in relay.go should not treat an empty state.workingDir as a real directory change, since transient ambiguous polls can clear the active prompt. Mirror the existing targetName guard logic by checking that state.workingDir is non-empty before comparing or assigning r.lastWorkingDir.

### FINDING-2
- **Severity:** Minor
- **Confidence:** 90
- **File:** specs/077-voice-glossary/spec.md:67
- **Category:** external
- **Source:** coderabbit
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**
FR-001 in the spec referenced the config path as `voice.glossary`, but the actual YAML path (matching the implementation in config.go and the documentation) is `defaults.voice.glossary`.

**Why this matters:**
Inconsistent spec text could mislead future implementers or spec reviewers about the correct config location.

**How it was resolved:**
Updated spec FR-001 text to use the canonical path `defaults.voice.glossary`.

## Test Suite Results

| Round | Test Command | Exit Code | Failures | Status |
|-------|-------------|-----------|----------|--------|
| 1     | go test ./internal/voice/... ./internal/config/... ./internal/tui/voice/... | 0 | 0 | passed |

Test suite passed in all fix rounds.

## Post-Fix Spec Coverage

All spec requirements verified after fix loop.

| Requirement | Implementation | Status |
|-------------|---------------|--------|
| FR-001 | config.go:62 VoiceDefaults.Glossary, ws_voice.go:168 | Verified |
| FR-002 | glossary.go:75 filepath.Join(workingDir, ".cc-deck", "voice-glossary.txt") | Verified |
| FR-003 | glossary.go:50-53 skip blank/comment lines | Verified |
| FR-004 | relay.go:419-420 sessionFields.WorkingDir | Verified |
| FR-005 | relay.go:347-362 state.workingDir change detection | Verified |
| FR-006 | glossary.go:22,73-83 projectCache map | Verified |
| FR-007 | glossary.go:86 dedup(globalTerms, projectTerms) | Verified |
| FR-008 | glossary.go:102-124 case-insensitive dedup | Verified |
| FR-009 | transcriber_http.go:26-28 SetPrompt, relay.go:352-354 type assertion | Verified |
| FR-010 | transcriber_http.go:52-56 omit when empty | Verified |
| FR-011 | glossary.go:92-94 warning at 800 chars | Verified |
