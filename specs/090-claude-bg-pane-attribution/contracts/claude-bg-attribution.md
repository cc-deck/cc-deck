# Contract: Claude Code background hook attribution

**Feature**: 090-claude-bg-pane-attribution

Binding behavior of `cc-deck hook --agent claude` and of the sidebar plugin after this feature.
`--agent codex`, `--agent opencode` and `--raw` are unchanged (FR-013).
Types and reasons are defined in [data-model.md](../data-model.md); background on each decision is in [research.md](../research.md).

## 1. Inputs

- stdin: Claude Code hook payload with `hook_event_name`, `session_id`, `cwd`, optional `tool_name`, `agent_id`, and optional `viewer_env` or `client_env` (object of string values, R6).
- `--pane-id`: shell-expanded `$ZELLIJ_PANE_ID` of the hook process (inherited from the daemon for background events).
- Hook environment: `ZELLIJ`, `ZELLIJ_SESSION_NAME`, `CLAUDE_CONFIG_DIR`, `HOME`, `CC_DECK_PROFILE*`, and `CLAUDE_BG_BACKEND` (diagnostic hint only, R9).
- Session records in `<config dir>/sessions/*.json` and the current user's process table (through `procinfo.Source`).

## 2. Host detection (FR-002, FR-003)

The hook walks at most eight ancestors (stopping at pid 1 or at a process of another user, as in spec 089). The records directory for the walk is the hook's `CLAUDE_CONFIG_DIR`, else `$HOME/.claude`.
For each ancestor, in order:

1. If `<records dir>/<pid>.json` exists and is verified (data-model `claudeRecord`): `kind: "interactive"` ends the walk as **foreground**; `kind: "bg"` ends it as **background** with that record as the worker record.
2. Else, if the ancestor's arguments contain `--bg-pty-host`: **background** without a worker record.

No decision within eight levels: **foreground**.
An inspection error: **foreground**, unless `CLAUDE_BG_BACKEND` is set in the hook environment, then **background** with reason `record-missing`, detail `host-unknown`.

Foreground events take today's path unchanged, including the `not-in-zellij` exit, `--pane-id`, the `pane-map.json` fallback and recording, and payloads without the `background` field.

## 3. Background resolution (first match wins)

For background events the inherited `--pane-id`, the hook's `ZELLIJ` and `ZELLIJ_SESSION_NAME`, and `pane-map.json` are never used to choose a target.

1. **Explicit viewer identity** (`viewer-env`): `viewer_env` (else `client_env`) names a pane id that parses as `u32` and a `ZELLIJ_SESSION_NAME` that passes `validZellijSessionName`. Profile from the same object. Delivered. A partial or invalid object is ignored and resolution continues.
2. **Worker**: without a verified worker record whose `jobId` is valid, the event is dropped as `record-missing`. The worker's configuration directory comes from its environment (`procinfo.Source.Get`); an unreadable worker drops as `record-missing`.
3. **Viewers**: collect parked and attach viewers of the job with the worker's configuration directory (R5). Reading the records directory, the listing, or any matching candidate failing drops the event as `record-missing`. Records and candidates that belong to other configuration directories or are unverified are skipped silently.
4. **Terminals**: group viewers into terminals (data-model `claudeViewer`).
   - 0 terminals: `headless`, dropped.
   - 2 or more: `several-viewers`, dropped.
   - 1 terminal outside Zellij: `viewer-not-in-zellij`, dropped.
   - 1 terminal inside Zellij: `parked-viewer` when a parked viewer is in it, else `attach-viewer`. Delivered to that pane and Zellij session, with the profile of that viewer (the parked one when both kinds are present).

## 4. Delivery and bookkeeping

- The payload keeps the `NormalizedPayload` schema plus `"background": true`. `pane_id` is the viewer's pane. Profile fields come from the viewer (FR-012); a profile name that fails `config.ValidProfileName` is dropped, and the icon passes `config.ValidIcon`, as in spec 089. Badges are evaluated from the payload `cwd`.
- Sent with `zellij --session <viewer session> pipe --name cc-deck:hook -- <payload>`.
- `session.AutoSave()` runs only when the hook's own Zellij session equals the target, which for background events (no `ZELLIJ`) means not at all.
- `pane-map.json` is never written by a background event. A background `SessionEnd`, delivered or dropped, removes the session's `pane-map.json` entry if one exists.
- `bg-job-panes.json` follows data-model.md. A per-job lock (100 ms wait) covers fresh viewer discovery and delivery; the shared ledger lock (20 ms wait) covers only atomic write merges. Tracked events drop if locking or the required intent write fails, so no row is created without removal ownership. The next event retries.
- Failed row releases remain pending alongside the new destination. Current viewers are revalidated before retry; incomplete discovery defers cleanup. Retry work shares a 100 ms deadline per event and uses persistent one-to-32-second failure backoff.
- Ownership does not expire solely by age. Successful conditional release consumes cleanup memory; interrupted delivery can repeat an idempotent release.

## 5. Row release (FR-007a)

When a transition in data-model.md calls for removal of `{P, S, sid}`, the hook sends, before delivering the current event:

```text
zellij --session <S> pipe --name cc-deck:release-row -- {"pane_id":<P>,"session_id":"<sid>"}
```

The controller handles `cc-deck:release-row` as follows:

- Malformed payload, unknown pane, or a row whose `session_id` differs from `sid` (including an empty one): no change.
- Otherwise: `evict_session(P)`, then a render broadcast. A live pane does not keep the row (unlike `SessionEnd`), because the pane no longer shows the job.
- Sidebar instances ignore the pipe. Plugin builds without the handler ignore it as an unknown pipe.

## 6. Background marker (FR-014a)

- `HookPayload.background: bool`, `#[serde(default)]`.
- Every processed hook event sets `Session.background = hook.background`, including events on paused rows (which otherwise only update `session_id`) and events that create the row.
- `Session.background` and `RenderSession.background` carry `#[serde(default)]`; restored sessions from older builds start without the marker.
- Rendering: line 1 becomes `edge bar + status indicator + " " + agent prefix + [dim "⇣ "] + name` when `background` is true; the name budget shrinks by two columns. Line 2, the edge bar and the indicator do not change. Rows without `background` render byte-identically to today.
- The help overlay legend lists the marker.

## 7. Debug log (FR-014)

When `~/.local/state/cc-deck/hook-debug` exists, each background Claude Code event appends one line:

```text
event=<name> session=<id> background=true job=<jobId or empty> resolution=<reason> [detail="..."] pane=<pane or empty> target_session="<S>" pane_id_arg="..." ZELLIJ="..." ZELLIJ_SESSION_NAME="..." ZELLIJ_PANE_ID="..." CLAUDE_BG_BACKEND="..."
```

A release appends `event=<name> session=<id> job=<jobId> release_pane=<P> release_session="<S>" release_sid=<sid>`. Event, session and job values pass `stripControl`. Foreground events keep today's line.

## 8. Contract tests

Go (`internal/cmd`, fake `procinfo.Source` and temporary records directory):

1. Foreground TUI ancestor with verified record: output identical to the existing hook test table (SC-005), no `background` field.
2. Parked viewer in pane A, worker inherits pane B with `ZELLIJ` set: delivered to A in A's session, never B; `pane-map.json` unchanged.
3. Attach viewer in pane C, worker inherits pane E: delivered to C; E never appears.
4. No viewer: `headless`, nothing sent.
5. Attach viewers in panes C and D: `several-viewers`; after D's process is gone, the next event goes to C.
6. Parked TUI and its attach child in the same pane: one terminal, `parked-viewer`.
7. Single viewer outside Zellij: `viewer-not-in-zellij`.
8. Viewer in Zellij session S2, daemon inherits S1: delivered with `--session S2`.
9. Viewer with another `CLAUDE_CONFIG_DIR`, records directory shared through a symlink: not a viewer.
10. Record whose `procStart` differs from the live process by more than one second (pid reuse): not a viewer; worker record likewise yields `record-missing`.
11. Unreadable candidate, malformed record, missing `jobId`: `record-missing`, nothing sent.
12. Ancestor walk error with and without `CLAUDE_BG_BACKEND`: dropped and today's path, respectively.
13. `viewer_env` present: wins over records; invalid `viewer_env` falls through.
14. Viewer moves from C to D: event delivered to D, `cc-deck:release-row` sent to C's session first with the job's session id; next event sends no release.
15. Viewer detaches (headless): release sent once, entry deleted, no event delivered.
16. Background `SessionEnd`: `pane-map.json` entry of the session removed; current background ownership cleared, with any failed cleanup retained for retry.
17. Profile enrichment uses the viewer's `CC_DECK_PROFILE*`, not the hook's.
18. Benchmark: 30 Claude Code processes and 30 records resolve within the SC-004 budget.

Rust (plugin):

19. `HookPayload` parses with and without `background`; old payloads parse unchanged.
20. `background: true` sets the row flag; a following event without it clears it; paused rows follow too.
21. `cc-deck:release-row` evicts only when the session id matches; mismatched, empty and unknown panes are untouched, including a row of another agent in the same pane.
22. Render: marker present on line 1 for background rows, absent otherwise; edge bar and indicator columns unchanged; existing render tests pass unmodified.

Deep-review regression additions:

23. Literal external worker and parked-viewer JSON decodes independently of production struct tags, including UTC parsing on a non-UTC host.
24. Overnight detach/move releases the former row rather than losing age-expired ownership.
25. Lock/write failure cannot create an untracked row; the next event recovers. Zellij I/O holds no global ledger lock, and viewer enumeration holds the per-job lock.
26. Failed old-row release persists alongside a new owner; later detach cleans both. Rejoined ambiguous viewers and incomplete lookup defer pending cleanup safely.
27. Several pending destinations share one short context deadline; failed retries persist their backoff and do not suppress healthy current-viewer delivery.
28. `hookPipeArgs` preserves the complete release-row argv and payload.
29. A first background Notification adopts a changed or empty row session ID, preserves manual naming and Waiting activity, applies supplied identity/profile, and permits conditional release.
