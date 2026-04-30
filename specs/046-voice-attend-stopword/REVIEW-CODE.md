# Code Review: Voice Attend Stop Word

**Spec:** [spec.md](spec.md)
**Date:** 2026-04-30
**Reviewer:** Claude (speckit.spex-gates.review-code)

## Compliance Summary

**Overall Score: 100%**

- Functional Requirements: 7/7 (100%)
- Error Handling: N/A (no new error paths)
- Edge Cases: 4/4 (100%)
- Non-Functional: N/A

---

## Code Review Guide (30 minutes)

> This section guides a code reviewer through the implementation changes,
> focusing on high-level questions that need human judgment.

**Changed files:** 7 files changed (2 Go source, 2 Go test, 1 Rust source, 2 AsciiDoc docs)

### Understanding the changes (8 min)

- Start with `cc-deck/internal/voice/stopword.go`: This is the simplest and most
  important change, a single line adding `"attend": {"next"}` to `DefaultCommands`.
  Once you understand this, the rest follows mechanically.
- Then `cc-deck/internal/voice/relay.go` (line 434): The switch case that maps the
  `"attend"` action to the `"[[attend]]"` wire payload. Note the `default` fallback
  to `"[[enter]]"` for unknown actions.
- Question: Is the `default: payload = "[[enter]]"` fallback at line 437 the right
  behavior for future unknown actions, or should unknown actions be logged/dropped
  instead of silently becoming Enter keypresses?

### Key decisions that need your eyes (12 min)

**Default fallback for unknown command actions** (`cc-deck/internal/voice/relay.go:437`, relates to [FR-002](spec.md#fr-002))

The switch statement has a `default` case that maps any unrecognized command action
to `"[[enter]]"`. This was pre-existing behavior from the submit-only era when only
one action existed. Now that two actions exist and more could follow, an unknown
action silently submitting a prompt could be surprising.
- Question: Should the default case log a warning and skip delivery instead of
  falling back to enter?

**Plugin reuse of existing ActionType::Attend** (`cc-zellij-plugin/src/controller/mod.rs:503-512`, relates to [FR-003](spec.md#fr-003) and [FR-004](spec.md#fr-004))

The voice `"attend"` command reuses the exact same `ActionType::Attend` dispatch
path as the keybinding and pipe-based attend. This is correct and ensures behavioral
parity with Alt+a. The implementation is 8 lines of boilerplate creating an
`ActionMessage`.
- Question: Is the boilerplate acceptable, or should a helper function reduce the
  repetition with the `PipeAction::Attend` handler at line 200?

**Documentation completeness** (`docs/modules/using/pages/voice.adoc:240-278`, `docs/modules/reference/pages/configuration.adoc:32-61`)

Both the voice guide and configuration reference now document the "attend"/"next"
command word. The voice guide includes a command words table, an example config
snippet showing both actions, and explains filler stripping behavior.
- Question: Does the Quick Start section (line 66-68) give enough context for a
  first-time user to discover the "next" command?

### Areas where I'm less certain (5 min)

- `cc-deck/internal/voice/relay.go:437` ([FR-002](spec.md#fr-002)): The default
  fallback to `[[enter]]` for unknown actions predates this feature. It is not wrong
  for the current two-action set, but it could mask configuration errors if a user
  misspells an action name. This is an existing concern, not introduced by this
  feature.
- `cc-deck/internal/voice/relay_test.go:551`: The `isProtocolMessage` helper now
  checks for `[[attend]]` as a protocol message. If more commands are added, this
  helper needs to grow. It is a test-only concern but could become a maintenance
  burden.

### Deviations and risks (5 min)

No deviations from [plan.md](plan.md) were identified. The implementation follows
the plan exactly: Phase 1 (Go side stop word + relay), Phase 2 (Rust plugin
handler), Phase 3 (documentation). All task file paths match actual changes.

- Risk: No integration test verifies the full pipeline from "next" utterance through
  the plugin to session cycling. Unit tests cover each layer independently. This is
  consistent with the existing "send" command, which also lacks an integration test.
  Question: Is this acceptable given the voice relay's architecture?

---

## Deep Review Report

> Automated multi-perspective code review results. This section summarizes
> what was checked, what was found, and what remains for human review.

**Date:** 2026-04-30 | **Rounds:** 0/3 | **Gate:** PASS

### Review Agents

| Agent | Findings | Status |
|-------|----------|--------|
| Correctness | 0 | completed |
| Architecture & Idioms | 2 | completed |
| Security | 0 | completed |
| Production Readiness | 0 | completed |
| Test Quality | 2 | completed |
| CodeRabbit (external) | 0 | rate-limited (hourly cap exceeded) |
| Copilot (external) | 0 | skipped (not installed) |

### Findings Summary

| Severity | Found | Fixed | Remaining |
|----------|-------|-------|-----------|
| Critical | 0 | 0 | 0 |
| Important | 0 | 0 | 0 |
| Minor | 4 | - | 4 |

### What was fixed automatically

No fixes were needed. All findings are Minor severity and do not require automatic correction.

### What still needs human attention

All Critical and Important findings were resolved (none existed). 4 Minor findings remain (see [review-findings.md](review-findings.md) for details). No further review action is needed, but reviewers may want to consider these during code review:

- The `ActionMessage` boilerplate at `cc-zellij-plugin/src/controller/mod.rs:503-512` duplicates the `PipeAction::Attend` handler at line 200. Is a helper function warranted if more voice commands are added?
- The `default` fallback to `[[enter]]` at `cc-deck/internal/voice/relay.go:437` could mask misconfigured action names. Should it log a warning instead?
- No test covers "next" with trailing punctuation (e.g., "next."). Is this edge case worth testing explicitly?
- No Rust unit test exists for the "attend" voice command arm, following the same gap as the "enter" command. Should this be addressed?

### Recommendation

All findings addressed. Code is ready for human review with no known blockers. The 4 Minor findings are consistent with pre-existing patterns in the codebase and do not require changes before merging.
