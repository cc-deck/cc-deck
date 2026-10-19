# Quickstart: Validating Background Session Attribution

**Feature**: 090-claude-bg-pane-attribution

Manual end-to-end checks for the scenarios in [spec.md](spec.md). Expected log values are defined in [contracts/claude-bg-attribution.md](contracts/claude-bg-attribution.md) section 7.

## Prerequisites

- Claude Code 2.1.296 or later (native install), with cc-deck hooks installed (`cc-deck config plugin status`).
- The feature build installed: `make install`, then restart the cc-deck workspace so the new plugin loads.
- A local workspace with at least four panes: `cc-deck ws attach <name>` (or `cc-deck` for the default local session).
- Hook debugging on, log empty:

  ```bash
  touch ~/.local/state/cc-deck/hook-debug
  : > ~/.local/state/cc-deck/hook.log
  ```

- Automated checks green first: `make test` and `make lint`.

## Q1. Parked session keeps its row (US1)

1. Pane A: start `claude` in a scratch directory, send a prompt, and wait for the row to show done.
2. Move the session to the background with `/background` (or the keybinding).
3. Record what the TUI in pane A shows now. Then, in another pane, check the TUI's record: `jq '{kind, sessionId, parkedJobId}' ~/.claude/sessions/<tui pid>.json` (use the profile's configuration directory when the session runs under a cc-deck profile).
4. If the TUI keeps showing the job (`parkedJobId` set): send a prompt that runs a tool.

**Expected**:

- With `parkedJobId` set: pane A's row follows the job (working, then done), shows `⇣` before the name, and keeps a manual rename made before step 2. The log shows `background=true job=<id> resolution=parked-viewer pane=<A>`. No other row changes.
- With `parkedJobId` cleared (the TUI moved on): the job's next event logs `resolution=headless` and releases pane A's row once (`release_pane=<A>`), unless the TUI's new conversation already owns the row. Continue with Q2 to view the job.
- Record the observed `parkedJobId` lifetime in research.md R2; it confirms or corrects the assumption in the spec.

## Q2. Attach in another pane (US2)

1. Start a job directly: `claude --bg 'list the files here, then wait for me'` in pane B, note the job id.
2. Pane C: `claude attach <jobId>`, then send a prompt.
3. Detach in pane C, run `claude attach <jobId>` in pane D, send a prompt.
4. Detach in pane D and let the job run one more prompt headless (for example a prompt queued before detaching).

**Expected**: Step 2 updates pane C's row (`attach-viewer`, marker shown). Step 3 releases pane C's row and updates pane D's row. Step 4 releases pane D's row and changes no other row (`headless`). Pane B's row (the shell that started the job) never shows the job.

## Q3. Ambiguous and foreign viewers (US3)

1. Attach the same job in panes C and D at once, send a prompt: no row changes, log `several-viewers`. Detach D, send a prompt: pane C updates.
2. Attach the job only from a terminal outside the workspace: no sidebar changes, log `viewer-not-in-zellij`.
3. With two workspaces S1 and S2, start the daemon from S1 (first background job there), attach a job only from a pane in S2: only S2's sidebar shows it (`target_session="S2"`).

## Q4. Foreground and other agents unchanged (SC-005)

Run a foreground `claude`, a `codex`, and an `opencode` session in their own panes. Rows behave as before, show no `⇣`, and foreground log lines have no `background=` field.

## Q5. Latency (SC-004)

```bash
cd cc-deck && go test ./internal/cmd/ -run '^$' -bench ClaudeBackgroundAttribution
```

Expected: well under 150 ms per resolution with 30 processes and 30 records; results go to `VERIFY.md` as in spec 089.

## Cleanup

```bash
rm ~/.local/state/cc-deck/hook-debug
```
