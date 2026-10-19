# Code Review: 090 Claude background pane attribution

Date: 2026-10-19

## Result

Manual deep review PASS after three autonomous fix rounds, with external-tool limitations recorded in [review-findings.md](review-findings.md).
Evidence-completeness score remains 20/25 (80%): all 18 functional requirements are implemented and covered, SC-004 meets its latency budget, and SC-005 passes the full test suite.
Five other success criteria still lack all of the live evidence they specify.
The code-review flow gate is now passed; the live verification gate remains open for T040.
After the user authorized repairs to the Compose smoke tests, both `make lint` and the unfiltered `make test` passed.

## Functional requirements

| Requirement | Implementation and evidence | Status |
|---|---|---|
| FR-001 | `runClaudeBackground` resolves the current viewer; `resolveClaudeBackground` selects only its terminal. Attach and pane-movement tests plus live attach evidence. | Compliant |
| FR-002 | `claudeHostOf` detects verified background workers or PTY hosts; `runHook` diverts before foreground pane mapping. Host and inherited-identity tests. | Compliant |
| FR-003 | `runHook` retains the existing foreground routing after background detection. Existing foreground hook tests pass. | Compliant |
| FR-004 | `workerOf`, `parkedViewers`, and `attachViewers` use the worker job and same config directory. Record, CLI form, and config-isolation tests. | Compliant |
| FR-005 | `viewerFromProcess` and `resolveClaudeBackground` require one Zellij viewer and deliver to its session/pane. Outside-viewer and multi-viewer tests. | Compliant |
| FR-006 | `resolveClaudeBackground` drops headless and ambiguous events; each hook repeats resolution. Tests cover recovery when ambiguity clears. | Compliant |
| FR-007 | Fresh viewer discovery occurs inside per-job serialization; transitions and delivery cannot apply an older pre-lock snapshot. Unit regression and earlier live move from pane 0 to pane 2. | Compliant |
| FR-007a | Durable intent/pending cleanup never expires by age; failed releases retry with revalidated absence, bounded work and backoff. `ControllerState::release_row` guards ownership. Overnight, failure, rejoin and plugin regressions. | Compliant |
| FR-008 | `verifyRecord` checks PID start time; `attachViewers` rereads the live process and compares start time. Reused-PID tests. | Compliant |
| FR-009 | `process_hook` preserves manual naming across replacement, including a first background Notification; ownership and supplied metadata are adopted without changing Waiting activity. Rust regressions. | Compliant |
| FR-010 | `runClaudeBackground` checks explicit `client_env` identity before process discovery. Precedence and malformed-input tests. | Compliant |
| FR-011 | Worker, record, process-list, and viewer lookup errors fail closed. Missing, unreadable, foreign, stale, and exiting-process tests. | Compliant |
| FR-012 | Background delivery enriches from the selected viewer profile and validates its name. Profile-source and invalid-profile tests. | Compliant |
| FR-013 | Codex and OpenCode remain on their existing `runHook` branches. Existing tests pass in the unfiltered suite. | Compliant |
| FR-014 | `logClaudeBackground` logs each background event's resolution; `logRowRelease` records removed pane/session. Log-format tests. | Compliant |
| FR-014a | Background payload field is backward compatible, persisted in Rust session state, rendered as `⇣`, and cleared by foreground events. Go/Rust tests and live marker observation. | Compliant |
| FR-015 | Detection uses existing session records/process environments without requiring new user settings. Existing installed hooks were used in the isolated live workspace. | Compliant |
| FR-016 | README and multi-agent/sidebar/CLI/configuration guides cover behavior, marker, cleanup, upstream issue, and workaround. Documentation review recorded in VERIFY.md. | Compliant |

## Success criteria evidence

| Criterion | Status | Evidence / remaining gap |
|---|---|---|
| SC-001 | Partial | Parked-viewer contract tests pass. The installed Claude Code UI cleared `parkedJobId` when `/background` returned, so the live retained-TUI path was not available. |
| SC-002 | Partial | A live attach delivered events to the attaching pane; only one live trial was observed. |
| SC-002a | Partial | A live move from pane 0 to pane 2 released pane 0 exactly once. The live detach-to-headless case remains pending. |
| SC-003 | Partial | Automated tests cover headless, several viewers, outside-Zellij, missing-record, and stale-identity cases. The complete live Q3 exercise remains pending. |
| SC-004 | Pass | Final 30-process/30-record fixture: 2.035 ms; real process listing: 1.719 ms. Cleanup is separately bounded to 100 ms after measuring concurrent real Zellij pipes up to 75.84 ms. |
| SC-004a | Partial | Marker serialization, persistence, rendering, and foreground clearing pass tests; the marker was visible in one live attach. |
| SC-005 | Pass | Existing foreground Claude, Codex, and OpenCode hook tests pass in the unfiltered suite. Smoke-test fixture and lifecycle repairs did not weaken hook or sidebar assertions. Interactive Codex/OpenCode walkthrough checks remain part of T040. |

## Review findings

The Compose smoke tests exposed a session-deletion race that caused a completed stop to report failure.
The shared container teardown helper now confirms target absence before accepting a failed deletion; seven regression cases cover its outcomes.
The user explicitly invoked deep review despite the remaining live evidence gaps.
Six internal reviewers plus configured external tools reviewed the implementation and subsequent fixes.
Nineteen normalized findings were resolved: eighteen by code/tests and one by confirming the existing producer timezone.
CodeRabbit completed three reviews but rate-limited the final one-line rerun; Codex completed that final rerun after timing out on three earlier attempts.
All three full post-fix suites passed, including nine Compose smoke tests and 483 Rust tests; lint and targeted race tests also passed.
No Critical or Important finding remains. See [review-findings.md](review-findings.md) for findings, calibration, per-round results, and post-fix coverage of all 18 requirements.

T039 and the code-review gate are complete; live verification remains pending T040.
See [VERIFY.md](VERIFY.md) for the exact commands, live observations, and test limitations.
