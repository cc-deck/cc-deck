# Verification: Claude Code background pane attribution

Updated: 2026-10-19. Host: macOS arm64, Apple M4 Pro. Claude Code: 2.1.302.

## Automated checks

| Check | Result |
|---|---|
| `make lint` | PASS: Go vet and Rust clippy with warnings denied |
| `make test` | PASS in all three deep-review fix rounds: all Go packages, all nine Compose smoke tests, and 483 Rust tests; final cmd run 222.999 s |
| `GOFLAGS='-skip=TestComposeSmoke' make test` | Earlier diagnostic PASS; superseded by the unfiltered suite above |
| `GOFLAGS='-race -run=TestClaude\|TestPaneMap\|TestWithPaneMapLock' make test-go` | PASS: attribution and state-lock tests under the race detector |
| `GOOS=linux GOARCH=amd64 CGO_ENABLED=0 GOFLAGS='-exec=/usr/bin/true' make test-go` | PASS: Linux source and test compilation only; Linux test binaries were not executed |
| `make install` | PASS: feature CLI built, WASM plugin and hooks installed |
| `git diff --check` | PASS |

### Compose smoke-test repair

The user authorized repairs to the failing Compose tests on 2026-10-18.
The suite now builds a dedicated Fedora image with Git, the official Zellij 0.45.1 binary, and a minimal `cc-deck` layout.
Podman caches its layers for later runs.
CLI subprocesses run from each temporary project directory, avoiding repository detection in the developer checkout.
Workspace, definition, and sharing state use temporary files, and inherited Zellij identity is cleared.
The lifecycle assertion follows the current `ws start` output (`ready`) and checks that the canonical session exists.

Real session creation also exposed a deletion race: Zellij could kill its server, then report an error while removing saved state.
Container and Compose teardown now accept that outcome only after confirming the target session is no longer active.
Seven regression cases cover successful deletion, confirmed absence, other or exited sessions, an active target, and inspection failures.
The full Go run passed with the Compose smoke tests enabled; `internal/cmd` completed in 226.410 seconds.
T039 is complete.

## Contract coverage

Go tests in `claude_bg_attribution_test.go` cover independent literal record parsing, UTC start-time tolerance on a non-UTC host, reused processes, eight ancestors, missing records, both PTY host spellings, nested foreground sessions, helper ancestry, explicit viewer identity, parked and attach viewers, native and npm forms, headless and ambiguous terminals, cross-session delivery, configuration isolation, profile validation, overnight releases, lock/write failure, serialized fresh lookup, pending-cleanup revalidation, aggregate timeout/backoff, SessionEnd cleanup, and debug logging.
The existing foreground hook cases also run against a verified interactive Claude ancestor.
Rust tests cover old/new payload parsing, background persistence defaults, paused and replacement rows, render projection and marker layout, ownership-checked release, and first-Notification ownership with manual naming, Waiting activity, and supplied identity/profile metadata.

## Latency (SC-004)

Command, through the required Make target:

```sh
GOCACHE=/private/tmp/cc-deck-090-go-cache GOFLAGS='-run=^$ -bench=BenchmarkClaudeBackgroundAttribution -benchtime=100x' make test-go
```

The host-list measurement ran outside the sandbox so it could inspect the real user process table.

| Sub-benchmark | Time per resolution |
|---|---:|
| Foreground ancestor walk | 0.207 ms |
| Real user process listing | 1.719 ms |
| Complete fake-source delivery, 30 processes and 30 records | 2.035 ms |

The complete fixture resolution is below the 150 ms budget.
The real process-list cost is measured separately; this is not a claim of a 30-process production workload.

Final deep-review measurements also exercised real no-op `cc-deck:release-row` pipes to a fresh isolated Zellij 0.45.1 controller.
Fifty sequential calls took 23.24–29.25 ms (median 24.39 ms); 60 calls from four concurrent senders took 27.76–75.84 ms (median 50.05 ms).
Those measurements raised a trial 40 ms cleanup deadline to the final 100 ms aggregate deadline.
Failed cleanup retains ownership and retry timing on disk, with backoff from one to 32 seconds.
These are bounded transport measurements, not new live Claude viewer-transition trials; T040 remains pending.
The isolated benchmark session was removed without restarting a user workspace.

## Deep-review outcome

PASS after three fix rounds, with all 18 functional requirements retained and no unresolved Critical or Important finding.
Six internal reviewers completed each round. CodeRabbit completed the initial and first two fix reviews, then rate-limited the final one-line rerun. Codex completed that final rerun after three earlier timeouts; Copilot was disabled.
See [review-findings.md](review-findings.md) for the complete findings and limitations.

## Live quickstart

A fresh isolated workspace, `v090-1d8d65`, loaded the installed plugin through `cc-deck ws new` and `cc-deck ws attach`.
Workspace state and definitions use `/private/tmp/cc-deck-090-live.1D8D65`.
An initial longer name exceeded macOS's 103-byte IPC socket limit; the shorter name worked.
The active user workspace was not restarted.
The test conversation supplied settings with the feature binary's absolute hook path because the user's `cc-deck` executable points at the main checkout.

Q1: OBSERVED RELEASE BEHAVIOR.
The foreground conversation answered a prompt and appeared in the sidebar.
`/background` printed `backgrounded`, gave a job ID, and exited to the shell.
Only the background worker record remained, with no interactive record naming its `parkedJobId`.
This installed default UI does not retain a parking viewer; the `parked-viewer` behavior is exercised by contract tests.
Research R2 and the user guide record this result.

Q2: PARTIAL LIVE PASS.
`claude attach` reopened the job in pane 0; a prompt executed `printf cc-deck-090-validation` and answered OK.
The job's real hook log recorded `resolution=attach-viewer`, the attaching pane, the isolated workspace session, and `background=true` for UserPromptSubmit and PostToolUse.
The sidebar visibly showed `⇣` before the row name.
After attaching the same job from pane 2 and submitting a prompt, the log recorded exactly one release for pane 0, then `resolution=attach-viewer` for pane 2 with `background=true`.
The live headless release after detaching pane 2 is still pending; automated release and headless contract tests pass.
The test job was stopped and both temporary workspace definitions were deleted after the run.

Q3: Pending full live multi-viewer and cross-workspace exercise; automated cases pass.
Q4: Foreground Claude live startup passed; existing Codex and OpenCode tests pass, but those two agents were not run interactively.
Q5: PASS, benchmark results above.

These pending checks were later observed in the live user-workspace run below (headless, ambiguous viewers, and the outside-Zellij viewer); see "Live user workspace (2026-10-19)".

## Walkthrough session (2026-10-19)

The full-mode runbook was started, and setup/cleanup steps passed. Steps 5–15 were skipped at the user's request so the live checks can be run later. No model usage occurred in this session. The two disposable `w090-a` and `w090-b` workspaces were deleted; the installed feature build was retained. This session adds no new live behavior evidence, and T040 remains unchecked.

## Live user workspace (2026-10-19)

The feature build `0.17.1-dev+8a247a83-dirty` was installed with `make install` from the worktree into the active user workspace `cc-deck-local` (Claude Code 2.1.303).
`~/bin/cc-deck` was repointed at the worktree binary, so the hooks reached the feature build through their normal `PATH` lookup rather than an absolute hook path.

The subject was a real daemon-hosted session that the user had reported as frozen in the sidebar.
A `claude daemon run --origin transient` instance had been spawned from pane 39 by an earlier session.
Its `bg-pty-host` worker ran `claude 2.1.303 --session-id 3efe15da... --fork-session` with `ZELLIJ` stripped and a stale `ZELLIJ_PANE_ID=39`, while the viewer `claude attach 3efe15da` ran in pane 4.
Under the main-branch binary (8b12664) every event of that job was logged as `resolution=not-in-zellij` and dropped.

From the first event under the feature build (11:57:14) the log recorded, for 17 consecutive events (13 PostToolUse, 4 SubagentStop):

```
background=true job=3efe15da resolution=attach-viewer pane=4 target_session="cc-deck-local" pane_id_arg="39" ZELLIJ=""
```

No event was dropped.
The job's events carried session `90691ed7...`, different from the job id, which is the FR-009 case of a job continuing under a new session id.
The user confirmed that the pane's sidebar row followed the job again, without restarting the workspace.
Foreground events from pane 0 in the same period logged `resolution=from-arg` and no `background=` field (Q4, foreground Claude, live).

A guided T040 run against this same job followed, driving the viewer through every resolution branch and reading the hook log after each transition.
All panes are in the `cc-deck-local` workspace; the job kept its session id `90691ed7...` throughout (FR-009).

| Transition | Observed log |
|---|---|
| Viewer moves pane 4 -> 3 | one `release_pane=4 release_sid=90691ed7...`, then `resolution=attach-viewer pane=3` (FR-007, FR-007a) |
| Two viewers (panes 3 and 7) | 11 events `resolution=several-viewers`, none delivered (Q3.1, FR-006) |
| Back to one viewer (detach 3) | `release_pane=3`, then `resolution=attach-viewer pane=7`; attribution resumed (Q3.1) |
| Last viewer detaches (pane 7) | `release_pane=7`, then `resolution=headless` on following events, none delivered (Q2.4) |
| Viewer outside Zellij (`ZELLIJ` unset) | 5 events `resolution=viewer-not-in-zellij`, none delivered (Q3.2) |

Totals for the job across the session: 155 `attach-viewer`, 11 `several-viewers`, plus the `headless` and `viewer-not-in-zellij` runs above, and the two row releases.
Foreground events from pane 0 logged `resolution=from-arg` with no `background=` field throughout (Q4, foreground Claude).

Still not observed live: the `parked-viewer` branch (Q1; this Claude build does not retain a parking TUI, as recorded above and in research R2), the cross-workspace case (Q3.3), and interactive Codex and OpenCode sessions (Q4; their hook handling is unchanged and covered by automated tests).

### T040 outcome

T040 is complete. The attribution, release, several-viewers, headless, and outside-Zellij branches were all observed live on a real daemon-hosted job in the user's workspace, with the hook log matching the contract section 7 format for each. The three remaining branches are accepted as documented limitations: Q1 `parked-viewer` is not reachable on this Claude build, and Q3.3 and interactive Codex/OpenCode are covered by automated tests.

## Documentation validation

Applied the installed prose plugin's `check` command and `pre-validator` instructions with `.style/voice.yaml` (`cc-deck`) and the project/global style and stoplist configuration.
Reviewed all five changed documentation files for style, terminology, flow, repetition, and voice.
The new passages contain no stoplist words, chatbot artifacts, curly quotes, em dashes, or en dashes.
AsciiDoc additions use one sentence per line.
New guide sentences average 16.3 words in the multi-agent guide and 16.2 in the sidebar guide; the reference entries use shorter sentences appropriate to a field/reason table.
Existing technical terminology elsewhere in these files was retained.
