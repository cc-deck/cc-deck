---
runbook_version: 1
feature: 090-claude-bg-pane-attribution
status: passed
mode: full
steps_total: 16
step_results: passed,passed,passed,passed,skipped,skipped,skipped,skipped,skipped,skipped,skipped,skipped,skipped,skipped,skipped,passed
drafted_at: 8a247a8
drafted_fingerprint: 559025347d9a8abaa2f9a71869a36258fd70ecd848473c3a5e43b3181fb72795
tested_at: 8a247a8
tested_fingerprint: 559025347d9a8abaa2f9a71869a36258fd70ecd848473c3a5e43b3181fb72795
updated: 2026-10-19
---

# Try it: Claude Code background jobs follow their viewer

<!-- runbook-status -->
> **Status:** passed · 16 of 16 steps (5 passed, 11 skipped) · Mode: full · Tested at 8a247a8 · 2026-10-19
<!-- /runbook-status -->

















## What you're testing

Follow one real conversation from a foreground pane into the background, attach elsewhere, move it, then leave it headless.
Next try competing viewers, a viewer outside the workspace, and a viewer in a second workspace.
Finish by checking foreground agents and latency, then clean up only the resources created here.
Watch both the sidebar and real hook diagnostics: neither synthetic hook payloads nor unit-test output substitutes for these live observations.

This is full mode: building and installing updates the shared cc-deck plugin and agent hook installation, and live agent prompts use your existing subscription or API account.
Allow roughly 1–3 minutes for a cached build (longer on a cold cache) and 20–40 minutes for the interactive journey.
No cloud infrastructure or new credentials are required.
Approve the build/install and model usage separately before starting them; optional Codex/OpenCode checks need their own existing login and usage approval.
Interactive terminal actions and visual sidebar observations are user-run, even when the agent runs the surrounding diagnostics.

Run command blocks from the same control terminal unless a step explicitly names a workspace pane or a separate plain terminal.
Keep that control terminal available while using two disposable workspaces; do not reset your current workspace.
The short names `w090-a` and `w090-b` are reserved for this walkthrough. If either already belongs to another activity, stop and choose unused names before continuing.
Retain existing logs, permissions, credentials, conversations, and unrelated jobs.

## Test environment

The feature build/install is complete. The fixture helper currently reports:

```json
{
  "fixtures": [
    {"name":"workspace-a","kind":"external","tier":"quick","owner":"user","started":"2026-10-19T06:13:00Z","teardown":"\"/Users/rhuss/Development/ai/cc-deck/cc-deck/.claude/worktrees/090-claude-bg-pane-attribution/cc-deck/cc-deck\" ws delete --force w090-a","note":"Private workspace stores; fresh feature plugin","state":"unknown"},
    {"name":"workspace-b","kind":"external","tier":"quick","owner":"user","started":"2026-10-19T06:13:00Z","teardown":"\"/Users/rhuss/Development/ai/cc-deck/cc-deck/.claude/worktrees/090-claude-bg-pane-attribution/cc-deck/cc-deck\" ws delete --force w090-b","note":"Private workspace stores; cross-workspace viewer","state":"unknown"}
  ]
}
```

The installed build is retained by default. Workspace teardown targets only `w090-a` and `w090-b` through the private stores below.
At close, both workspace deletions succeeded and the private workspace list was empty. The fixture helper reports external fixture state as `unknown` because it does not probe external resources; the actual CLI listing confirmed teardown. The existing Zellij sessions `obo-stage` and `cc-deck-local` remained.
- Claude conversation/background job: none started in this run; no paid model usage occurred. If a later live session starts one, capture its real ID and record an exact `claude stop` teardown command; preserve its transcript by default. Never stop the shared daemon or other jobs.
- Codex/OpenCode: both CLIs were detected during setup but not invoked; no model requests were made. Their live checks remain deferred.

## Steps

### Step 1 of 16: Locate the checkout and available tools (US4)

**Why this step:** Keep this checkout's CLI distinct from any other cc-deck installation, and establish shared paths for later commands.

**Needs:** A shell in this feature worktree. No model request is made here.

**Run:**

```sh
export CC090_ROOT="$(git rev-parse --show-toplevel)"
cd "$CC090_ROOT"
export CC090_SCRATCH="$CC090_ROOT/$(.specify/extensions/spex/scripts/spex-runbook-status.sh locate specs/090-claude-bg-pane-attribution --create | jq -r .scratch)"
export CC090_CLI="$CC090_ROOT/cc-deck/cc-deck"
export CC090_CONFIG="${CLAUDE_CONFIG_DIR:-$HOME/.claude}"
export CC090_STATE="${XDG_STATE_HOME:-$HOME/.local/state}/cc-deck"
git branch --show-current
command -v claude
command -v jq
claude --version
command -v codex || true
command -v opencode || true
```

**What you should see:** Branch `090-claude-bg-pane-attribution`, an installed Claude Code version supporting background sessions, and jq. Missing optional agents are noted, not silently counted as tested.

**If you see something else:** A different branch or unsupported Claude release means this journey needs adjustment before building. Do not print credential files or whole environment dumps.

**Result:**
Passed by Codex in the feature worktree. Branch: `090-claude-bg-pane-attribution`; feature CLI is executable at `/Users/rhuss/Development/ai/cc-deck/cc-deck/.claude/worktrees/090-claude-bg-pane-attribution/cc-deck/cc-deck`; Claude Code `2.1.303`; `jq` is available at `/opt/homebrew/bin/jq`. Optional Codex and OpenCode CLIs are installed. No model session or workspace was started.

### Step 2 of 16: Build and install the reviewed feature (US1)

**Why this step:** The previous live checks predate the deep-review fixes; new workspaces must load the current plugin.

**Needs:** Explicit slow-local approval. Estimate 1–3 minutes with caches; installation updates shared plugin files and agent hooks, but does not restart an existing workspace.

**Run:**

```sh
cd "$CC090_ROOT"
make install
"$CC090_CLI" config plugin status
```

**What you should see:** Successful WASM/CLI build and installation, with the plugin and available agent hooks reported installed. Existing user workspaces stay running.

**If you see something else:** Stop at the actual build/install failure. Inspect its real output before proposing a fix; an old installed binary is not evidence for the reviewed build.

**Result:**
Passed by Codex. `make install` built the WASM plugin and CLI (`0.17.1-dev+8a247a83-dirty`), installed the reviewed plugin, and reported plugin/Zellij/hooks installed. Agent hooks were detected for Claude Code, Codex CLI, and OpenCode. No existing workspace was restarted and no model session was started.

### Step 3 of 16: Prepare private workspace state and exact hook commands (US4)

**Why this step:** Avoid modifying your workspace definitions or clearing your diagnostic log. Absolute hook commands ensure a daemon with an older PATH still invokes this checkout.

**Needs:** Step 1 of 16 and Step 2 of 16. This setup does not start a model session.

**Run:**

```sh
export CC_DECK_STATE_FILE="$CC090_SCRATCH/workspace-state.yaml"
export CC_DECK_WORKSPACES_FILE="$CC090_SCRATCH/workspaces.yaml"
export CC_DECK_SHARE_STATE_FILE="$CC090_SCRATCH/shares.yaml"
export PATH="$CC090_ROOT/cc-deck:$PATH"
mkdir -p "$CC090_SCRATCH/project" "$CC090_STATE"
if test -e "$CC090_STATE/hook-debug"; then export CC090_DEBUG_WAS_ON=1; else export CC090_DEBUG_WAS_ON=0; fi
touch "$CC090_STATE/hook-debug"
jq -n --arg cli "$CC090_CLI" '{hooks: (["SessionStart","UserPromptSubmit","PreToolUse","PostToolUse","PermissionRequest","Notification","Stop","SubagentStop","SubagentStart","SessionEnd"] | map({key: ., value: [{hooks: [{type: "command", command: ("\"" + $cli + "\" hook --agent claude --pane-id \"$ZELLIJ_PANE_ID\"")}]}]}) | from_entries)}' > "$CC090_SCRATCH/claude-hooks.json"
command -v cc-deck
jq -r '.hooks.PostToolUse[0].hooks[0].command' "$CC090_SCRATCH/claude-hooks.json"
```

**What you should see:** Both printed paths point at this worktree's `cc-deck/cc-deck`. The private project and stores live under `.walkthrough`. Existing hook-log contents remain intact.

**If you see something else:** Fix the checkout/path selection before continuing. The local settings file is test isolation for multiple checkouts, not a new configuration requirement for normal users.

**Result:**
Passed by Codex. Private workspace/state/share YAML paths and the project directory were created under `.walkthrough`; the generated Claude settings use the absolute feature CLI path. The shared `hook-debug` flag already existed before this run (`CC090_DEBUG_WAS_ON=1`), so it was preserved. No log was truncated.

### Step 4 of 16: Create two disposable workspaces (US2)

**Why this step:** Fresh sessions load the new plugin without restarting your active workspace, and a second workspace makes cross-session routing observable.

**Needs:** Quick-fixture approval, private stores from Step 3 of 16, and confirmation that the short names are unused or owned by this runbook. Reuse this runbook's existing healthy fixtures on resume.

**Run:**

```sh
cd "$CC090_SCRATCH/project"
env -u ZELLIJ -u ZELLIJ_SESSION_NAME -u ZELLIJ_PANE_ID "$CC090_CLI" ws new w090-a --type local
env -u ZELLIJ -u ZELLIJ_SESSION_NAME -u ZELLIJ_PANE_ID "$CC090_CLI" ws new w090-b --type local
"$CC090_ROOT/.specify/extensions/spex/scripts/spex-runbook-fixture.sh" record "$CC090_SCRATCH" --name workspace-a --tier quick --owner user --teardown "\"$CC090_CLI\" ws delete --force w090-a" --note "Private workspace stores; fresh feature plugin"
"$CC090_ROOT/.specify/extensions/spex/scripts/spex-runbook-fixture.sh" record "$CC090_SCRATCH" --name workspace-b --tier quick --owner user --teardown "\"$CC090_CLI\" ws delete --force w090-b" --note "Private workspace stores; cross-workspace viewer"
"$CC090_CLI" ws list
```

**What you should see:** Both walkthrough workspaces exist and are ready. Only their private definitions are listed; your ordinary definitions remain unchanged. Record the actual starter and teardown environment in Test environment.

**If you see something else:** Do not use `--reset` or delete an existing session to force a name collision. Investigate creation failures before attaching.

**Result:**
Passed. Codex created `w090-a` and `w090-b` from the private stores in `.walkthrough/project`; both report `type=local`, `session=active`, and project `project`. Fixture helper records `workspace-a` and `workspace-b` as quick external fixtures owned by the user, with teardown commands targeting only those exact private workspace names. Existing Zellij sessions were `obo-stage` and `cc-deck-local`; neither name collided. No model request was started.

### Step 5 of 16: Observe a foreground row and rename it (US1)

**Why this step:** Establish the row's baseline and a manual name whose preservation you can recognize later.

**Needs:** An interactive terminal, an existing Claude login, and explicit approval for normal model usage. This is user-run.

**Do:**

1. From the prepared control terminal, run `env -u ZELLIJ -u ZELLIJ_SESSION_NAME -u ZELLIJ_PANE_ID "$CC090_CLI" ws attach w090-a` in a separate terminal carrying the prepared environment. If a newly opened terminal lacks those variables, copy the exports from Step 1 of 16 and Step 3 of 16 first.
2. In workspace A, create at least four ordinary terminal panes using the workspace UI. Label their roles mentally: original, viewer C, viewer D, and observer. Do not use panes from your everyday workspace.
3. In the original pane, run `cd "$CC090_SCRATCH/project"` and `claude --settings "$CC090_SCRATCH/claude-hooks.json"`.
4. Send: `Run printf cc090-foreground in Bash, then answer READY. Do not edit any files.` Approve only that requested harmless tool command if prompted.
5. Rename this row to `Walkthrough job` using the sidebar's rename action. Record its pane ID and visible activity transitions.

**What you should see:** The original pane has one foreground row, transitions through activity, and ends without a `⇣` marker. Its manual name is `Walkthrough job`.

**If you see something else:** Check the actual binary path and plugin/hook installation before testing background behavior. A missing foreground row is not a background-attribution result.

**Result:**
Skipped by user request: the live walkthrough is deferred until a later session. No Claude foreground conversation was opened and no paid model request was made.

### Step 6 of 16: Move the conversation to the background (US1)

**Why this step:** Discover what this installed Claude version actually does with the parking TUI, rather than assuming it remains open.

**Needs:** The foreground conversation from Step 5 of 16. This is user-run.

**Do:**

1. Enter `/background` in that Claude conversation.
2. Record whether its TUI stays open or exits to the shell, and the job identifier printed by Claude.
3. In the observer pane, run `claude agents --json --cwd "$CC090_SCRATCH/project" | jq '[.[] | {pid,kind,sessionId,status}]'`.
4. If the original TUI still views the job, inspect its matching session record using the actual PID from the listing; record only `kind`, `sessionId`, and `parkedJobId`. Send the harmless printf prompt again through that TUI and observe its row/name/marker.

**What you should see:** A real background job exists. If the TUI remains parked, its row follows events, keeps the manual name, and shows `⇣`; diagnostics say `parked-viewer`. If the TUI exits and no parked viewer remains, record that producer behavior and continue through attach. Do not count an unavailable retained-TUI path as a pass.

**If you see something else:** Preserve the actual producer output. A disabled/unsupported `/background` command or missing job is a mismatch requiring investigation, not a reason to fabricate a job record.

**Result:**
Skipped by user request: deferred until the live walkthrough. Step 5 of 16 was not run, so there is no conversation to background and no observed parking behavior.

### Step 7 of 16: Capture the real job ID for later panes (US4)

**Why this step:** Later commands should read the actual ID instead of relying on copied placeholders or matching another user's running job.

**Needs:** Exactly one background worker in the scratch project, the same Claude configuration directory, and the prepared control/observer environment.

**Run:**

```sh
CC090_WORKER_PID=$(claude agents --json --cwd "$CC090_SCRATCH/project" | jq -er '[.[] | select(.kind == "bg")] | if length == 1 then .[0].pid else error("Expected exactly one walkthrough background worker") end')
jq -er '.jobId | select(test("^[0-9a-f]{8}$"))' "$CC090_CONFIG/sessions/$CC090_WORKER_PID.json" > "$CC090_SCRATCH/job-id"
cat "$CC090_SCRATCH/job-id"
jq '{pid,kind,sessionId,jobId,procStart}' "$CC090_CONFIG/sessions/$CC090_WORKER_PID.json"
"$CC090_ROOT/.specify/extensions/spex/scripts/spex-runbook-fixture.sh" record "$CC090_SCRATCH" --name claude-job --tier remote --owner user --teardown "claude stop $(cat "$CC090_SCRATCH/job-id")" --note "Existing Claude account/config; scratch-project job only; preserve transcript"
```

**What you should see:** An eight-character job ID and its real verified worker record. The recorded teardown names only this job; no token or credential is printed.

**If you see something else:** Resolve an ambiguous listing or a changed producer schema before selecting a job. Never stop or adopt an unrelated worker merely because it is the newest process.

**Result:**
Skipped by user request: deferred until the live walkthrough. No Claude background job or job ID was created.

### Step 8 of 16: Attach in viewer C (US2)

**Why this step:** Check that background events follow a terminal that views the job, not the original pane or daemon identity.

**Needs:** The captured ID and no other active viewer. Interactive work is user-run.

**Do:**

1. In viewer C, run `claude attach "$(cat "$CC090_SCRATCH/job-id")"`.
2. Send `Run printf cc090-viewer-c in Bash, then answer READY. Do not edit files.`
3. Observe C's sidebar row and marker. In the observer pane run `rg -F "job=$(cat "$CC090_SCRATCH/job-id") " "$CC090_STATE/hook.log" | tail -20`.
4. Record the real pane ID, target session, activity transitions, marker, and any release of the former row. Check that another row is not created for the daemon's inherited pane.

**What you should see:** Only viewer C receives the job's new events, with `background=true`, `resolution=attach-viewer`, and `⇣` before its row name. An already-owned previous pane is released only when its row still belongs to the job.

**If you see something else:** Check whether a parked or second attach viewer still exists. Multiple viewers intentionally suppress delivery; do not treat that as a single-viewer failure.

**Result:**
Skipped by user request: deferred until the live walkthrough. No Claude background job was created, so viewer attribution was not observed.

### Step 9 of 16: Move the viewer from C to D (US2)

**Why this step:** Exercise destination changes and conditional removal of the old row.

**Needs:** Viewer C from Step 8 of 16 and another pane D. This is user-run.

**Do:**

1. Detach C using Claude attach's documented `Ctrl+Z` shell-return action; confirm it no longer views the job. If the installed UI uses another detach action, record and use that actual action.
2. In D, run `claude attach "$(cat "$CC090_SCRATCH/job-id")"` and send `Run printf cc090-viewer-d in Bash, then answer READY. Do not edit files.`
3. Inspect the sidebar and the same job-filtered hook-log command from Step 8 of 16.
4. Send one more harmless printf prompt in D and check that C's successful removal is not repeated.

**What you should see:** The job's next event updates D and removes C's job-owned row once. No unrelated row disappears. Further events stay on D, with `⇣` visible.

**If you see something else:** Inspect `release_error`, `state_error`, live viewer processes, and the old row's session ownership. A release rejected because another session owns C is correct protection, not a failed deletion.

**Result:**
Skipped by user request: deferred until the live walkthrough. No viewer was attached; row movement and release behavior were not observed.

### Step 10 of 16: Leave the running job headless (US2)

**Why this step:** Confirm stale-row cleanup happens on a genuine event after detachment, without injecting a fake hook.

**Needs:** Viewer D is the sole viewer. This is user-run and may require approving the harmless sleep/printf tool command.

**Do:**

1. In D, send `Run sleep 20; printf cc090-headless in one Bash command, then answer READY. Do not edit files.`
2. Once the tool is genuinely running, detach D before the 20 seconds finish. Ensure no other terminal still views this job.
3. Wait for the tool's real PostToolUse/Stop events, then inspect the sidebar and the job-filtered hook log.
4. Observe a subsequent event, if available, to check that a successful release is not repeated.

**What you should see:** The first event after all viewers leave logs `resolution=headless`, releases D's still-job-owned row, and creates no new sidebar row. Unrelated rows remain unchanged.

**If you see something else:** First establish whether the tool actually completed after detach and whether a suspended/remaining viewer still exists. No post-detach event means insufficient evidence, not a pass.

**Result:**
Skipped by user request: deferred until the live walkthrough. No background job or post-detach event was observed.

### Step 11 of 16: Try two viewers, then recover to one (US3)

**Why this step:** Verify that ambiguous ownership cannot update an arbitrary pane, and that attribution recovers without restarting the job.

**Needs:** The same background job. Both terminal attaches are user-run.

**Do:**

1. Attach the job in both C and D using `claude attach "$(cat "$CC090_SCRATCH/job-id")"`.
2. Record the sidebar baseline, then submit the harmless printf prompt through one viewer.
3. Check for `resolution=several-viewers` and no row updates caused by those ambiguous events.
4. Detach D completely, submit another printf prompt in C, and inspect the next events and marker.

**What you should see:** Two simultaneous viewer terminals suppress background row updates. Once only C remains, the next event resolves C and its row resumes normally.

**If you see something else:** If Claude itself refuses a second concurrent attach, record the producer restriction and skip this live case rather than claiming the ambiguity rule was observed. Automated ambiguity tests remain separate evidence.

**Result:**
Skipped by user request: deferred until the live walkthrough. Concurrent viewers and recovery behavior were not observed.

### Step 12 of 16: View the job outside the workspace (US3)

**Why this step:** A plain-terminal viewer must not borrow a daemon's inherited workspace identity.

**Needs:** Detach all workspace viewers first. A real separate plain terminal is required; the agent cannot substitute a synthetic process for it.

**Do:**

1. Open a plain terminal outside the workspace, copy the path exports from Step 1 of 16, and run `env -u ZELLIJ -u ZELLIJ_SESSION_NAME -u ZELLIJ_PANE_ID claude attach "$(cat "$CC090_SCRATCH/job-id")"`.
2. Submit the harmless printf prompt there while observing workspace A from another terminal.
3. Inspect the job-filtered log, then detach the plain-terminal viewer before continuing.

**What you should see:** Diagnostics say `viewer-not-in-zellij`, and no workspace sidebar row receives the plain-terminal events. A former job-owned workspace row may be conditionally released.

**If you see something else:** Verify that this is genuinely outside the workspace and that no other viewer remains. Do not count clearing environment variables in an attached interactive workspace as the whole visual test.

**Result:**
Skipped by user request: deferred until the live walkthrough. No plain-terminal viewer was opened.

### Step 13 of 16: Attach only from workspace B (US3)

**Why this step:** Confirm routing follows the viewer's session even when the worker/daemon originated elsewhere.

**Needs:** No other job viewer; the second disposable workspace. Interactive attachment is user-run.

**Do:**

1. In a separate prepared terminal, run `env -u ZELLIJ -u ZELLIJ_SESSION_NAME -u ZELLIJ_PANE_ID "$CC090_CLI" ws attach w090-b`.
2. In B, run `claude attach "$(cat "$CC090_SCRATCH/job-id")"` and submit the harmless printf prompt.
3. Observe both sidebars and the job-filtered hook log. Record the actual target session and B's pane ID.
4. Detach B after the observation.

**What you should see:** Only B's viewer row updates and shows `⇣`; diagnostics name B's target session. A receives no new row from the job, even if its pane numbers overlap B's.

**If you see something else:** Inspect the actual viewer process environment and configuration directory. Do not restart the shared daemon to manufacture an inherited identity; record the identity actually present.

**Result:**
Skipped by user request: deferred until the live walkthrough. No cross-workspace viewer was opened.

### Step 14 of 16: Check foreground agents still behave normally (US1)

**Why this step:** Confirm the background marker and cleanup do not leak into ordinary foreground agent rows.

**Needs:** User-run interactive sessions and existing credentials. Approve any Codex/OpenCode model usage separately; missing or declined clients are explicitly skipped.

**Do:**

1. In an ordinary pane of A, run `claude --settings "$CC090_SCRATCH/claude-hooks.json"` from the scratch project and submit the harmless printf prompt. If reusing a background-marked pane, verify its marker clears on the next foreground event.
2. In separate panes, run `codex` and `opencode` if installed and approved. Ask each to print its working directory without editing files.
3. Observe each agent's ordinary activity transitions and row ownership. Inspect only diagnostics for those sessions, without copying credentials or full account state.
4. Exit the walkthrough foreground clients normally.

**What you should see:** Foreground rows behave as before, have no `⇣`, and are not removed by another job's stale cleanup. Foreground Claude log lines have no background flag.

**If you see something else:** Record results per client, including an unavailable client as skipped. Passing automated tests does not turn a skipped live client check into a pass.

**Result:**
Skipped by user request: deferred until the live walkthrough. No foreground Claude, Codex, or OpenCode model request was made in this run.

### Step 15 of 16: Measure attribution overhead (US4)

**Why this step:** Check the reviewed implementation's measured resolution time against the 150 ms target.

**Needs:** The built checkout. This command makes no model request and may be agent-run after approval.

**Run:**

```sh
cd "$CC090_ROOT"
GOFLAGS='-run=^$ -bench=BenchmarkClaudeBackgroundAttribution -benchtime=100x' make test-go
```

**What you should see:** The complete 30-process/30-record fixture stays below 150,000,000 ns/op. Record the actual foreground, real process-list, and complete-fixture measurements separately; this is not a claim that 30 real Claude clients were launched.

**If you see something else:** Capture the actual failure or slow measurement. A sandbox-denied host inspection is skipped/failed evidence, not a zero-cost result.

**Result:**
Skipped by user request: deferred with the rest of the walkthrough. No benchmark was run in this session.

### Step 16 of 16: Record evidence and choose cleanup (US4)

**Why this step:** Leave an honest verification record and stop only this walkthrough's resources.

**Needs:** Actual observations from the preceding steps. Full-mode slow/paid fixtures default to keep unless you explicitly choose teardown.

**Do:**

1. Review `.specify/extensions/spex/scripts/spex-runbook-fixture.sh status "$CC090_SCRATCH"` and decide keep or teardown for the installed build and the captured Claude job. The installation remains by default; keeping a job must be explicitly recorded with the fixture helper.
2. If stopping the job, check its captured record/ID still names the walkthrough conversation, then run `claude stop "$(cat "$CC090_SCRATCH/job-id")"`. Do not run daemon-wide stop commands or remove transcripts.
3. After leaving the walkthrough terminals, run `"$CC090_CLI" ws delete --force w090-a` and `"$CC090_CLI" ws delete --force w090-b` with the private store variables still exported. These remove only the disposable workspace definitions/sessions, not the scratch project or your regular definitions.
4. If `CC090_DEBUG_WAS_ON` was `0`, remove only the debug flag created here using the platform's recoverable file-removal mechanism; otherwise leave it enabled. Keep the existing hook log intact.
5. Stop helper-managed quick fixtures using `.specify/extensions/spex/scripts/spex-runbook-fixture.sh stop "$CC090_SCRATCH" --quick`; external fixture teardown is the explicit action above, not an automatic consequence of that command.
6. Record real, credential-redacted evidence in each Result and in `VERIFY.md`. Update research R2 only with newly observed parking behavior. Leave T040 unchecked if any required live scenario remains skipped, failed, or unobserved.

**What you should see:** Only the walkthrough's disposable sessions are gone or explicitly kept; your existing workspaces, unrelated jobs, transcripts, and logs remain. The runbook tally reflects actual passes, failures, and skips.

**If you see something else:** Stop cleanup if names, configuration directories, or ownership differ from the recorded fixtures. Do not broaden deletion targets to make cleanup succeed.

**Result:**
Passed by Codex at the user's request to stop and finish the runbook session. Deleted only `w090-a` and `w090-b` using the private workspace stores; the CLI confirmed both deletions and then reported no workspaces. Zellij still listed the pre-existing `obo-stage` and `cc-deck-local` sessions. The installed feature build was retained. No Claude job existed to stop. The pre-existing `hook-debug` flag and hook log were left intact. The fixture helper's `stop --quick` had no helper-managed quick fixtures to stop.

## Not covered here

- A retained parking TUI may not exist in the installed Claude release; prior observations showed `/background` returning to the shell. The retained-TUI branch is only a live pass if it genuinely occurs.
- PID reuse, unreadable/malformed records, overnight ownership retention, injected delivery failures, and retry backoff use deterministic automated regressions. Deliberately corrupting a live user's records or waiting overnight is not part of this journey.
- Future explicit-viewer fields cannot be produced by a Claude release that does not send them.
- A missing/declined Codex or OpenCode client, an upstream restriction on concurrent attaches, or an inaccessible plain terminal is recorded as a live skip with its reason.
- A fresh daemon with a deliberately different inherited identity is not guaranteed when reusing the existing per-configuration-directory daemon. Cross-workspace behavior is tested against the real identity present; no shared daemon is stopped for this exercise.
- The exact producer `record-missing` and explicit-viewer debug reasons remain covered by contract tests unless encountered naturally. This runbook does not manufacture external events.

## Coverage

- US1: foreground baseline, manual naming, observed parking behavior, marker appearance/clearing, and foreground-agent checks. Retained-TUI coverage is conditional on the actual producer behavior.
- US2: real attach, movement, headless continuation, and conditional release on post-detach events.
- US3: multiple viewers and recovery, a real plain-terminal viewer, and cross-workspace routing, subject to producer/terminal availability.
- US4: real job/worker capture, event/release diagnostics, recorded observations, and latency measurements. Unproducible error/future-field cases remain automated-only evidence.
- T040 is complete only after its required live checks have actually been observed. Drafting this runbook or passing its structure check does not complete T040.

## Results

Session closed at the user's request: 16 steps total, 5 passed (setup and cleanup), 0 failed, and 11 skipped for later live verification. The feature build remains installed; both disposable workspaces were removed. No model usage occurred. Live background attribution, viewer movement, headless cleanup, multi-viewer handling, outside/cross-workspace behavior, foreground regressions, and the benchmark remain unverified here. Resume this runbook later for those checks; T040 remains incomplete until the required live observations are recorded.
