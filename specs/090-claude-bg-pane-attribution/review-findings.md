# Deep Review Findings

Date: 2026-10-19
Branch: `090-claude-bg-pane-attribution`
Invocation: explicit `$speckit-spex-deep-review-run`, delegated to installed cc-review 1.2.0
Fix rounds: 3
Gate outcome: PASS (with external-review limitations below)

## Scope and method

Reviewed all 34 changed source/build/fixture files, including untracked feature files, against `spec.md` and their callers.
Six internal perspectives ran initially and after every fix round: architecture, correctness, production readiness, security, test quality, and the enabled Gator reviewer.
Goal alignment was skipped because this branch has no PR.
CodeRabbit completed the initial review and the first two re-reviews; the final one-line budget re-review was attempted but rate-limited. Codex timed out on the first three attempts, then completed the final one-line re-review without an actionable finding. Copilot is disabled in project configuration.
Tests use `make test` with the configured 300-second bound.

The installed delegation detector did not recognize Codex's plugin-cache location; the installed review workflow was located and used directly.
Structured `ReportFindings` is unavailable in this harness, so agents returned the same JSON in their final messages.
External tools reviewed disposable detached snapshots containing staged new files, avoiding accidental omission of untracked implementation files. Re-review snapshots contain only that round's fix diff.
No PR comments, pushes, or commits to the feature branch were made. Temporary snapshot commits are not part of its history.

## Summary

Counts retain independent categories, including Gator's isolated findings; overlapping production findings F2 and F3 are consolidated as F3.
There were 20 raw reports and 19 normalized findings. Eighteen are fixed in code/tests; F11 is resolved by confirming the existing UTC behavior.
The lifecycle-GC observation below is an informational follow-up, outside the defect counts.

| Severity | Found | Fixed | Verified without change | Remaining |
|---|---:|---:|---:|---:|
| Critical | 0 | 0 | 0 | 0 |
| Important | 14 | 13 | 1 | 0 |
| Minor | 5 | 5 | 0 | 0 |
| Total | 19 | 18 | 1 | 0 |

| Reviewer | Found | Fixed | Remaining | Status |
|---|---:|---:|---:|---|
| Architecture & Idioms | 1 | 1 | 0 | Completed |
| Correctness | 4 | 4 | 0 | Completed |
| Production Readiness | 2 | 2 | 0 | Completed |
| Security | 0 | 0 | 0 | Completed |
| Test Quality | 2 | 2 | 0 | Completed |
| Gator (Principal Engineer) | 4 | 4 | 0 | Completed |
| Codex | 0 | 0 | 0 | Final one-line re-review completed; three earlier timeouts |
| Copilot | - | - | - | Disabled |
| CodeRabbit | 6 | 5 | 0 | Three reviews completed; final rerun rate-limited; one UTC verification required no patch |
| Goal Alignment | - | - | - | Skipped: no PR |
| Test Suite (regression) | 0 | 0 | 0 | Passed in all three fix rounds |

MVP: Correctness (4 findings; alphabetical tie-break with Gator).

## Findings and resolutions

Locations below refer to the reviewed revisions, before line shifts from fixes.
All verdicts are confirmed. External findings pass through meta-review without calibration, as required by the delegated workflow.

### F1 and F12: Untracked delivery after lock failure

- Severity: Important (F1), Minor (F12)
- Sources: architecture (98% confidence), CodeRabbit (75%)
- Category: architecture / external
- Location: `cc-deck/internal/cmd/claude_bg_attribution.go:514–543`
- Resolution: fixed in round 1

What was wrong: A known worker fell through to event delivery when the ownership lock failed. Its first row could therefore be created without a removal record.
Why it matters: The next detached or moved event had no remembered destination to release, leaving a stale sidebar row under plausible contention.
Resolution: Known jobs never deliver outside their serialization path. Required intent writes also must succeed before a row can be created; lock/write failures log diagnostics and the next event retries. Tests cover job-lock, ledger-lock, write-failure, and recovery paths.

### F3 (including F2): Shared lock and lost failed releases

- Severity: Important
- Source: production readiness (99%; F2 98%)
- Category: production-readiness
- Location: `cc-deck/internal/cmd/claude_bg_attribution.go:515–536`
- Resolution: fixed in round 1

What was wrong: The global ledger lock covered synchronous Zellij commands, while the sender discarded command errors. Old ownership was deleted or replaced even when removal failed, and a successful release was logged regardless.
Why it matters: An unavailable destination delayed unrelated jobs, and a transient timeout could leave an old row permanently without retry memory.
Resolution: Per-job locks isolate delivery; the shared lock protects only atomic merges. Delivery errors are returned. Transition intent persists before sends, and failed old-row removals remain pending alongside the new owner. Success is logged only after a successful command. Tests prove both owners survive a failed move cleanup and are later removed, and that pipe I/O does not hold the global lock.

### F4 and F8: Overnight expiry loses visible-row ownership

- Severity: Important
- Sources: correctness (100%), Gator (99%)
- Category: correctness / gator (isolated)
- Location: `cc-deck/internal/cmd/claude_bg_attribution.go:403–407`
- Resolution: fixed in round 1

What was wrong: Loading the ledger discarded entries older than twelve hours, independently of whether their sidebar rows still existed.
Why it matters: After an overnight idle period, detaching or moving left the former pane's row behind. Controller stale-session handling does not evict a live pane's row. The planned TTL contradicted FR-007a.
Resolution: Outstanding removal ownership no longer expires by age. Overnight detach and move regressions assert the old release. Contracts, research, plan, tasks, and state-file documentation record this correction.

### F5 and F9: Notification ownership mismatch

- Severity: Important
- Sources: correctness (95%), Gator (98%)
- Category: correctness / gator (isolated)
- Location: `cc-zellij-plugin/src/controller/hooks.rs:23–74`
- Resolution: fixed in round 1

What was wrong: A background Notification set the marker but returned before adopting the worker's session ID. The ledger then released with the new ID while the row still held its foreground ID.
Why it matters: The ownership guard correctly rejected removal, leaving a stale row after detach or move. A different worker session ID is explicitly allowed by FR-009.
Resolution: Background non-activity hooks adopt ownership before returning. The regression covers an old or empty ID, manual naming, unchanged Waiting activity, identity/profile metadata, and subsequent conditional release. Foreground Notifications retain their existing path.

### F6: Self-generated external-schema fixtures

- Severity: Minor, calibrated from Important
- Source: test quality (99%)
- Category: test-quality
- Location: `cc-deck/internal/cmd/claude_bg_attribution_test.go:63–73`
- Resolution: fixed in round 1

What was wrong: Fixtures marshaled the same production struct used for decoding, so an incorrect JSON tag could change both sides and evade tests.
Why it matters: The current tags were correct; this was a future-regression coverage weakness, not an existing runtime failure. Meta-review therefore reduced severity to Minor.
Resolution: Independent literal worker/viewer JSON checks producer field names, decoded values, UTC identity verification on a non-UTC host, and actual Notification attribution.

### F7: Untested release argv

- Severity: Minor
- Source: test quality (99%)
- Category: test-quality
- Location: `cc-deck/internal/cmd/hook_test.go:342–347`
- Resolution: fixed in round 1

What was wrong: Argument-builder tests only exercised `cc-deck:hook`; background scenario tests mocked the sender.
Why it matters: Accidentally hardcoding the old pipe name could pass all cleanup-transition tests while breaking real removal.
Resolution: The complete targeted `cc-deck:release-row` argv and unchanged payload are asserted independently.

### F10: Help legend precedes its title

- Severity: Minor
- Source: CodeRabbit (75%)
- Category: external
- Location: `cc-zellij-plugin/src/sidebar_plugin/render.rs:799`
- Resolution: fixed in round 1

What was wrong: The background marker legend was placed before the help heading.
Resolution: It now follows the title and rule under a Legend subheading.

### F11: Verify producer timestamp timezone

- Severity: Important (external severity preserved)
- Source: CodeRabbit (75%)
- Category: external
- Location: `cc-deck/internal/cmd/claude_bg_attribution.go:44–48`
- Resolution: verified without code change

CodeRabbit requested confirmation of the producer timezone, suggesting local parsing only if records use local time.
Research R2 already identifies the producer as `LC_ALL=C TZ=UTC ps -o lstart=`. Three live Claude records were independently compared with that command on this Europe/Berlin host and matched UTC exactly.
The existing UTC parser is correct; changing it to local time would introduce a bug. The literal-schema regression now explicitly exercises a non-UTC local zone.
This verification is not presented as an implementation fix or a meta-review removal.

### F13: Unbounded synchronous pending cleanup

- Severity: Important
- Source: production readiness (97%)
- Category: production-readiness
- Location: `cc-deck/internal/cmd/claude_bg_attribution.go:592–610` (round 1)
- Resolution: fixed in round 2; transport budget refined in round 3

What was wrong: Each pending removal could spend three seconds before the healthy current viewer received its event; multiple failed destinations multiplied the delay. Competing same-job hooks could time out on their lock.
Why it matters: An old unreachable Zellij session repeatedly delayed or suppressed updates to the healthy destination.
Resolution: All cleanup attempts share one 100 ms context deadline, passed through to the actual subprocess. Unfinished releases remain durable; failed destinations back off from one to 32 seconds. The budget/backoff regression uses three unavailable destinations and confirms continued current-viewer delivery.

### F14 and F16: Viewer snapshots captured before serialization

- Severity: Important
- Sources: correctness (95%), Gator (98%)
- Category: correctness / gator (isolated)
- Location: `cc-deck/internal/cmd/claude_bg_attribution.go:516–562` (round 1)
- Resolution: fixed in round 2

What was wrong: A delayed hook could resolve former viewer C, wait outside the lock while a newer hook moved ownership to D, then acquire the lock and apply its stale C snapshot.
Why it matters: That ordering released the still-viewing D row and recreated the departed C row, violating FR-001, FR-007, and FR-007a.
Resolution: Viewer discovery now occurs inside the per-job critical section. A deterministic source wrapper verifies that the job lock is held during enumeration, rather than relying on race timing.

### F15 and F17: Pending releases remove returning viewers

- Severity: Important
- Sources: correctness (98%), Gator (99%)
- Category: correctness / gator (isolated)
- Location: `cc-deck/internal/cmd/claude_bg_attribution.go:574–579` (round 1)
- Resolution: fixed in round 2

What was wrong: Pending cleanup protected only the next unique owner. A formerly departed pane could rejoin among several viewers and still receive its old pending release; incomplete lookup also triggered cleanup without proving absence.
Why it matters: A legitimate viewer could lose its row and manual name during ambiguity, contrary to FR-006 and FR-007a.
Resolution: Each pending release is revalidated against all current terminals. Incomplete discovery defers all pending cleanup, and a unique return to the owned row cancels that pending release. Regressions cover ambiguity, incomplete lookup, later verified absence, and a unique return.

### F18: Unnecessary shared read lock

- Severity: Important (external severity preserved)
- Source: CodeRabbit (75%)
- Category: external
- Location: `cc-deck/internal/cmd/claude_bg_attribution.go:550–557` (round 1)
- Resolution: fixed in round 2

What was wrong: Reads acquired the shared ledger lock even though atomic replacement yields complete snapshots and the per-job lock already excludes same-entry writers.
Why it matters: Read-only access added avoidable contention and another lock-failure drop path.
Resolution: Reads use the atomic snapshot directly. Read-modify-write merges retain shared locking and write-error handling, preventing lost updates between different jobs.

### F19: Restore Notification metadata after replacement

- Severity: Minor
- Source: CodeRabbit (75%)
- Category: external
- Location: `cc-zellij-plugin/src/controller/hooks.rs:74–82` (round 1)
- Resolution: fixed in round 2

What was wrong: Replacement cleared identity/profile fields and Notification returned before reapplying supplied metadata.
Resolution: A shared identity helper applies supplied fields on that path, preserving absent-field behavior. The same-agent Waiting-row regression verifies name, session ownership, agent indicator, profile, color, and icon.

### F20: Cleanup deadline too short under normal concurrent load

- Severity: Important
- Source: CodeRabbit (75%)
- Category: external
- Location: `cc-deck/internal/cmd/claude_bg_attribution.go:611–612` (round 2)
- Resolution: fixed in round 3

What was wrong: The first bounded-cleanup fix allowed only 40 ms for process startup, IPC, plugin delivery, and command completion.
Why it matters: A real isolated Zellij 0.45.1 controller completed 50 sequential no-op releases in 23.24–29.25 ms, but 60 calls with four concurrent senders reached 75.84 ms; 45 exceeded 40 ms. The short limit could repeatedly interrupt healthy cleanup under plausible load.
Resolution: The aggregate deadline is now 100 ms, providing measured transport headroom while retaining bounded work, persistent retry, and backoff. The existing deadline regression derives its expected limit from the constant.

## Meta-review

The Skeptic and Calibrator independently evaluated the initial findings and the seven round-1 findings. The last single external finding was below the five-finding threshold.
No false positives were removed. One severity was calibrated (F6 Important to Minor). No contradictions were detected. Intentional TTL/fallback design choices did not refute their concrete stale-row consequences.

| Agent | Initial raw findings | Precision | Signal/noise | Round-1 findings | Precision | Signal/noise |
|---|---:|---:|---:|---:|---:|---:|
| Architecture | 1 | 1.00 | 1.0 | 0 | - | - |
| Correctness | 2 | 1.00 | 2.0 | 2 | 1.00 | 2.0 |
| Production | 2 | 1.00 | 2.0 | 1 | 1.00 | 1.0 |
| Test quality | 2 | 0.50 | 2.0 | 0 | - | - |
| Gator | 2 | 1.00 | 2.0 | 2 | 1.00 | 2.0 |
| CodeRabbit | 3 | 0.67 | 0.5 | 2 | 0.50 | 1.0 |

## Notable observation

N1: Jobs killed without SessionEnd can retain cleanup ownership and empty per-job lock files. A lifecycle-aware collector should verify retirement and conditionally release rows before forgetting entries; lock cleanup must avoid inode races with waiting hooks. This is a future state-hygiene enhancement, not a reason to restore unsafe age-only expiry.
Captured in `brainstorm/idea-inbox.md` as `background-cleanup-lifecycle-gc`.

## Post-fix spec coverage

No functional requirement was dropped by the fixes. Removal of age-only pruning and the untracked fallback corrects design artifacts to satisfy primary requirements, rather than removing feature behavior.

| Requirement | Final implementation / evidence | Status |
|---|---|---|
| FR-001 | Fresh viewer resolution inside the per-job lock; delivery regression | Covered |
| FR-002 | Background branch never uses inherited/cache attribution or writes pane-map destinations | Covered |
| FR-003 | Foreground routing and three-second sender deadline retained; existing tests | Covered |
| FR-004 | Worker record and same-directory parked/attach discovery; independent JSON fixtures | Covered |
| FR-005 | One valid Zellij viewer receives delivery; outside viewer drops | Covered |
| FR-006 | Headless/ambiguous events drop; pending cleanup respects present/unknown viewers | Covered |
| FR-007 | Fresh serialized lookup follows moves; stale-snapshot regression | Covered |
| FR-007a | Durable ownership/pending cleanup, no TTL, guarded release and retries | Covered |
| FR-008 | Process identity/start-time verification retained | Covered |
| FR-009 | Non-activity ownership adoption preserves manual names and activity | Covered |
| FR-010 | Explicit viewer identity precedence retained | Covered |
| FR-011 | Unreadable/inconsistent lookup fails closed | Covered |
| FR-012 | Viewer profile enrichment; Notification metadata regression | Covered |
| FR-013 | Codex/OpenCode paths unchanged; unfiltered suite | Covered |
| FR-014 | Resolution logs plus truthful release/error diagnostics | Covered |
| FR-014a | Background marker propagation, clearing, rendering and persistence retained | Covered |
| FR-015 | Existing hook installation/configuration remains sufficient | Covered |
| FR-016 | README and all four reference/guide additions updated | Covered |

All 18 functional requirements checked after the fix loop. This does not replace the remaining live success-criterion trials in T040.

## Test suite results

| Fix round | Command | Exit | Result |
|---|---|---:|---|
| 1 | `make test` (300 s bound) | 0 | All Go, nine Compose smoke cases, 483 Rust tests; cmd 226.847 s |
| 2 | `make test` (300 s bound) | 0 | All Go, nine Compose smoke cases, 483 Rust tests; cmd 226.152 s |
| 3 | `make test` (300 s bound) | 0 | All Go, nine Compose smoke cases, 483 Rust tests; cmd 222.999 s |

Additional checks: targeted attribution tests, race-detector attribution/state tests, Linux amd64 compile-only tests, `make lint`, and staged/unstaged diff whitespace checks pass.
Final 100-iteration benchmarks measured 0.207 ms foreground ancestry, 1.719 ms real process listing, and 2.035 ms for the complete 30-process/30-record fake-source path, below SC-004's 150 ms limit. Real Zellij transport was measured separately as described under F20.
An initial ad-hoc Rust test compile caught a raw-string delimiter typo during round 2; it was repaired before the full round-2 suite, which passed.

The final Codex review independently found no actionable regression in the timeout adjustment. Its optional snapshot test attempts could not run because that source-only snapshot omitted the ignored embedded WASM artifact and its default cache was sandbox-restricted; the root workspace's full suites above ran successfully.
CodeRabbit's final attempt returned a provider rate-limit error (27-minute retry guidance). No account, billing, or installation changes were attempted. The final change was reviewed by all six internal agents and Codex, and its motivation was verified with real transport measurements.
The isolated benchmark session and disposable review snapshots were removed; existing user workspaces were preserved.

## Remaining work

No unresolved Critical or Important finding remains. All six internal reviewers returned clean results on the final fixes, and tests passed within the configured bound. External tool failures are reported rather than treated as a clean review of the full feature.
T040's live headless, multi-viewer, outside-Zellij, cross-workspace, and Codex/OpenCode walkthrough evidence remains pending. See `VERIFY.md`.
