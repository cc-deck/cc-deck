# Deep Review Findings

**Date:** 2026-04-30
**Branch:** 046-voice-attend-stopword
**Rounds:** 0
**Gate Outcome:** PASS
**Invocation:** quality-gate

## Summary

| Severity | Found | Fixed | Remaining |
|----------|-------|-------|-----------|
| Critical | 0 | 0 | 0 |
| Important | 0 | 0 | 0 |
| Minor | 4 | - | 4 |
| **Total** | **4** | **0** | **4** |

**Agents completed:** 5/5 (+ 0 external tools; CodeRabbit rate-limited, Copilot not installed)
**Agents failed:** none

## Findings

### FINDING-1
- **Severity:** Minor
- **Confidence:** 75
- **File:** cc-zellij-plugin/src/controller/mod.rs:503-512
- **Category:** architecture
- **Source:** architecture-agent
- **Round found:** 1
- **Resolution:** remaining (minor, no fix needed)

**What is wrong:**
The `ActionMessage` construction for the "attend" voice command (lines 503-512) duplicates the identical struct creation at lines 200-211 (the `PipeAction::Attend` handler). Both create the same `ActionMessage { action: ActionType::Attend, pane_id: None, tab_index: None, value: None, sidebar_plugin_id: 0 }`.

**Why this matters:**
If the `ActionMessage` fields for attend ever change (e.g., adding a new required field), both locations must be updated. However, this pattern is consistent with how all other `PipeAction` variants are handled in the same file, so extracting a helper would be a broader refactor.

**How it was resolved:**
Not fixed. This follows the existing codebase convention. A helper function could be introduced in a future cleanup pass if more voice commands are added.

### FINDING-2
- **Severity:** Minor
- **Confidence:** 72
- **File:** cc-deck/internal/voice/relay.go:437
- **Category:** architecture
- **Source:** architecture-agent
- **Round found:** 1
- **Resolution:** remaining (minor, pre-existing)

**What is wrong:**
The `default` case in the command action switch sends `"[[enter]]"` for any unrecognized action name. With two actions now defined ("submit" and "attend"), this fallback could mask misconfigured action names by silently submitting a prompt.

**Why this matters:**
A user who misspells an action name in their config (e.g., "atend" instead of "attend") would have that action silently fall through to enter, which could cause unexpected prompt submissions. This is a pre-existing pattern from when only one action existed.

**How it was resolved:**
Not fixed. This is pre-existing behavior and not introduced by this feature. A follow-up improvement could log a warning or skip delivery for unknown actions.

### FINDING-3
- **Severity:** Minor
- **Confidence:** 70
- **File:** cc-deck/internal/voice/stopword_test.go
- **Category:** test-quality
- **Source:** test-quality-agent
- **Round found:** 1
- **Resolution:** remaining (minor)

**What is wrong:**
No test case covers "next" with trailing punctuation (e.g., "next." or "next!"). The `stripFillers` function strips trailing punctuation via `TrimRight(lower, ".,!?;:")`, so "next." should be treated as a standalone "next" command. This behavior is correct but not explicitly tested.

**Why this matters:**
If the punctuation stripping behavior changes in the future, there would be no test to catch a regression for the "attend" action specifically. The existing "send" tests also lack this case, so this is a general gap.

**How it was resolved:**
Not fixed. This is a minor test gap consistent with existing test patterns.

### FINDING-4
- **Severity:** Minor
- **Confidence:** 72
- **File:** cc-zellij-plugin/src/controller/mod.rs:503-512
- **Category:** test-quality
- **Source:** test-quality-agent
- **Round found:** 1
- **Resolution:** remaining (minor, pre-existing gap)

**What is wrong:**
No Rust unit test exists for the "attend" voice command arm in `handle_voice_command()`. The existing tests cover "voice:on", "voice:off", "voice:ping", "voice:mute", "voice:unmute", "enter" (no sessions case), and "unknown:cmd", but not "attend".

**Why this matters:**
The attend arm calls `actions::handle_action`, which requires state setup (sessions, pane IDs) to verify meaningful behavior. Testing it in isolation is less straightforward than the simpler voice protocol commands. The "enter" command also lacks a positive test (only the no-session case is tested), making this a pre-existing pattern.

**How it was resolved:**
Not fixed. Follows the pre-existing pattern. The Go-side relay test (`TestVoiceRelay_AttendCommandSendsAttend`) covers the end-to-end payload delivery, providing integration-level coverage.

## Remaining Findings

All 4 findings are Minor severity. No Critical or Important findings remain. The gate passes without requiring fixes.
