# Review Guide: Claude Code Background Session Pane Attribution

**Generated**: 2026-10-18 | **Spec**: [spec.md](spec.md)

## Why This Change

Claude Code can move a session into a background job that runs inside a per-configuration-directory daemon. The job's hooks inherit the daemon's environment, which names whichever pane started the daemon and lacks `ZELLIJ`. As a result, cc-deck drops every background event as "not in Zellij", and the row of the pane that actually shows the job freezes. On 2026-10-16 about 1,350 events were lost this way. If a future Claude Code release keeps `ZELLIJ`, the same events would land in an unrelated row instead. The upstream report is [anthropics/claude-code#102949](https://github.com/anthropics/claude-code/issues/102949).

## What Changes

Background job events now reach the row of the pane that currently views the job. That pane is either the TUI that parked the session or a pane running `claude attach <jobId>`. Rows driven by a background job show a small dim `⇣` marker, which goes away with the next foreground event. Jobs with no viewer, with several viewers, or viewed only from outside Zellij update no row. When the viewer moves or leaves, the former viewer's row is removed, but only if that row still belongs to the job's session. Users change nothing about how they start, park, or attach sessions. There are no breaking changes. Foreground Claude Code, Codex, and OpenCode behave as before, and their payloads stay byte-identical.

## How It Works

- **Detection**: `cc-deck hook --agent claude` walks up to eight ancestors. The first ancestor with a verified session record (`<config dir>/sessions/<pid>.json`, pid plus `procStart` within one second of the live start time) decides the outcome. `kind: "bg"` means background, with the job id taken from the record. `kind: "interactive"` means foreground, which takes today's path unchanged. A `--bg-pty-host` ancestor without a record means background with a missing record, so the event is dropped.
- **Viewer discovery** (exact matching only, no working-directory heuristics):
  - Parked TUIs: records with `parkedJobId == jobId`.
  - `claude attach` processes: matched by their arguments, found through a new `procinfo.ListUserMatching` filter. The kernel names native Claude processes by version number, for example `2.1.302`.
  - Both kinds must use the worker's `CLAUDE_CONFIG_DIR`, read from their own environment. The cc-deck profiles share one `sessions` directory through symlinks, so the directory alone does not tell configurations apart.
  - Viewers are grouped by Zellij session and pane, so two processes in one pane count as one terminal.
- **Delivery**: The event goes out with `zellij --session <viewer session> pipe` and carries `background: true` and the viewer's profile. Background events never read or write `pane-map.json`.
- **Stale row release**: A new state file, `bg-job-panes.json`, stores the pane that last received each job's events. It follows the same flock and atomic-write pattern as `pane-map.json`. When that pane stops viewing the job, the hook sends a new `cc-deck:release-row` pipe message. The plugin evicts the row only if its `session_id` still matches. Older plugin builds ignore the unknown pipe.
- **Plugin**:
  - `background` travels from `HookPayload` through `Session` and `RenderSession` to the renderer. Every field uses `#[serde(default)]`.
  - The marker sits between the agent prefix and the name on line 1.
  - FR-009 (one row when the job continues under a new session id) needs no plugin change. The existing session-replacement path already keeps manual renames.
- **Diagnostics**: Each background event writes one debug line. The line names the job and one of seven reasons: `viewer-env`, `parked-viewer`, `attach-viewer`, `headless`, `several-viewers`, `viewer-not-in-zellij`, `record-missing`.

## When It Applies

**Applies when**:
- A Claude Code hook runs inside a background job (Claude Code 2.1.296 or later, macOS or Linux, including SSH and OpenShell workspaces).
- The job is shown by a parked TUI or a `claude attach` process.
- A future Claude Code release passes the viewer's identity explicitly (`viewer_env` or `client_env` in the payload). That identity then wins over record lookup.

**Does not apply when**:
- The hook runs in a foreground session, including a foreground Claude Code started inside a background job's shell. Today's attribution is kept.
- The agent is Codex or OpenCode, or `--raw` is used (no change).
- The job has no viewer, several viewer terminals, or only a viewer outside Zellij. The event is dropped on purpose ("never a wrong row").
- The viewer is idle: the row only updates with the job's next hook event, because cc-deck learns about jobs through hooks alone.
- Windows (`procStartFt`) is out of scope.

## Key Decisions

1. **Detect background jobs by process ancestry and session records, not by environment.** `CLAUDE_BG_BACKEND` is cheap, but it is undocumented and inherited by nested processes. It only serves as a hint when the ancestor walk fails (research R1, R9).
2. **Verify every record by pid plus `procStart` (within one second).** This matches Claude Code's own check and guards against pid reuse. Running `ps` per process was rejected for latency, and `startedAt` was rejected because it is not the process start time (R2, R3).
3. **Find viewers through records and attach arguments, grouped by pane.** Rejected alternatives: the `jobs/<id>/state.json` files (they hold no viewer data), the daemon control socket (undocumented protocol), and working-directory matching (ruled out in clarification) (R5).
4. **Ownership checks for row removal happen in the plugin.** The new `cc-deck:release-row` pipe was chosen over a synthetic `SessionEnd`. Older plugins would apply a `SessionEnd` unconditionally and could evict another agent's row (R11).
5. **Keep a separate `bg-job-panes.json` file instead of new keys in `pane-map.json`.** The separate file makes the rule "used only for removal, never for attribution" structural (R7).
6. **Use `⇣` in dim style for the marker.** Circle glyphs read as the idle indicator `○`. The other candidate positions collide with the presence markers, line 2, or the profile edge bar (R12).
7. **On ancestor inspection errors, fall back to the foreground path.** Codex drops in that case, but doing the same here would break foreground sessions on transient errors (FR-003). The `CLAUDE_BG_BACKEND` hint covers the background side (R9).

## Areas Needing Attention

- **`parkedJobId` lifetime (open risk, research R2).** `/background` describes itself as "free the terminal", and in-process viewing sits behind a feature flag. If Claude Code clears `parkedJobId` right away, parked sessions count as headless: their row is released and reappears only after `claude attach`. Safety holds either way, but User Story 1 depends on the outcome. Quickstart Q1 settles it, and T040 records the result.
- **Undocumented Claude Code internals.** The design reads the session record fields, the `--bg-pty-host` argument, and the `attach <jobId>` argument shape, none of which are a supported interface. A field rename breaks attribution, but in the safe direction: events are dropped, not misrouted. Check that every unexpected shape ends in `record-missing`.
- **Process classification.** Native installs show the version or `claude` as the process name, while npm installs show `node` with a `cli.js` argument. Any classification gap only hides `claude attach` viewers. A process that matches but cannot be read fails the whole listing, the same strict rule as spec 089.
- **Linux start time.** `btime + starttime/100` assumes USER_HZ is 100. That holds on every release target, but it is an assumption.
- **Removal timing.** If the job sends an event before a parked TUI's new conversation sends its first foreground event, the row is released. The TUI's next event then recreates it, and a manual rename made earlier is lost. This is accepted as rare.
- **Auto-save never runs for background events**, the same as for daemon-hosted Codex events (contract section 4).
- **Plugin literal churn.** About 29 `HookPayload` and `RenderSession` literals in tests gain a `background: false` field. These edits are mechanical, but they make the diff larger.

## Open Questions

- Does the installed Claude Code keep `parkedJobId` while the TUI shows the job after `/background` or the keybinding? This is resolved by quickstart Q1 (task T040), and the documentation (T033) states the observed behavior.

## Review Checklist

- [ ] Key decisions are justified
- [ ] Breaking changes are documented with migration guidance (none expected; foreground payloads unchanged)
- [ ] Scope matches the stated boundaries
- [ ] Success criteria are achievable (SC-004 measured by `BenchmarkClaudeBackgroundAttribution`)
- [ ] No unstated assumptions
- [ ] Every unexpected or unreadable input path ends in a dropped event, never in the inherited pane
- [ ] `cc-deck:release-row` cannot remove a row owned by another session or agent
- [ ] Background events never write `pane-map.json`

---

<!-- Code phase sections are appended below this line by the phase-manager command -->
