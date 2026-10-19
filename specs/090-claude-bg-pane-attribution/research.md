# Research: Claude Code Background Session Pane Attribution

**Feature**: 090-claude-bg-pane-attribution | **Date**: 2026-10-18

Facts below were observed on the development machine (macOS, Claude Code 2.1.296 to 2.1.302, cc-deck profiles `private` and `work` with `CLAUDE_CONFIG_DIR`) and in the Claude Code 2.1.302 binary.
Process ids are the ones seen on 2026-10-18.

## R1. Process tree of a background job

**Observed**:

| pid | parent | process | `ZELLIJ` | `ZELLIJ_PANE_ID` |
|-----|--------|---------|----------|------------------|
| 52675 | 1 | `claude daemon run --json-path <config dir>/daemon.json ...` | `0` | `9` |
| 52691 | 52675 | `claude bg-pty-host --bg-pty-host /tmp/cc-daemon-501/<hash>/spare/<id>.pty.sock ... -- <versions>/2.1.302 --bg-spare ...` (spare) | unset | `9` |
| 93641 | 1 | `<install>/ClaudeCode.app/Contents/MacOS/claude --bg-pty-host /tmp/cc-daemon-501/bf7c1594/pty/2402e991.sock 123 46 -- <versions>/2.1.296 ...` (adopted from an older daemon) | unset | `3` |
| 93856 | 93641 | `<versions>/2.1.296 --session-id 2402e991-d639-... --fork-session --resume <transcript>` (worker) | unset | `3` |
| 53462 | 53314 | `<home>/.local/bin/claude attach 2402e991` (viewer) | `0` | `9` |

- The PTY host is recognizable by the argument `--bg-pty-host` in every observed spelling (`claude bg-pty-host --bg-pty-host ...` and `.../MacOS/claude --bg-pty-host ...`). Its parent is the daemon or, after adoption, pid 1.
- The worker is the direct child of the PTY host. Its session record `<config dir>/sessions/93856.json` has `kind: "bg"`, `jobId: "2402e991"` and `sessionId: "2402e991-d639-..."`: the job id is the first eight hex digits of the job's session id.
- The worker environment adds `CLAUDE_BG_BACKEND=daemon`, `CLAUDE_BG_SOURCE`, and `CLAUDE_BG_RENDEZVOUS_SOCK=/tmp/cc-daemon-501/<hash>/rv/<jobId>.sock`. `CLAUDE_CONFIG_DIR` and `CC_DECK_PROFILE*` are those of the process that started the daemon, not of any viewer.
- The `claude attach` viewer has no session record. Its arguments are `attach <jobId>`, and its environment carries its own pane (`ZELLIJ_PANE_ID=9`, `ZELLIJ_SESSION_NAME=cc-deck-local`) and its own `CLAUDE_CONFIG_DIR`.
- A Claude Code hook runs as `/bin/sh -c 'cc-deck hook --agent claude --pane-id "$ZELLIJ_PANE_ID"'`, a child of the worker (background) or of the TUI (foreground), so the session process is the grandparent of `cc-deck hook`.

**Decision**: A hook runs inside a background job when the ancestor walk (at most eight levels, as in spec 089) reaches either a process whose verified session record has `kind: "bg"` (the worker) or a process with the `--bg-pty-host` argument, before it reaches a process with a verified `kind: "interactive"` record.
The first verified record on the way up decides; the PTY host only matters when the worker record is missing.

**Rationale**: The walk is exact and independent of the daemon instance (adoption by a newer daemon re-parents the PTY host to pid 1 but keeps the worker below it). Stopping at the first verified record makes a foreground Claude Code started inside a background job's shell count as foreground, which keeps today's behavior for it.

**Alternatives considered**:

- `CLAUDE_BG_BACKEND` in the hook's environment: cheap, but undocumented, inherited by any process the job starts (including a nested foreground Claude Code), and not mentioned in the spec. It is logged as a diagnostic field only.
- The `jobs/<jobId>/state.json` files: they describe jobs, not which process the hook belongs to.

## R2. Session records and their lifetime

**Observed** (fields used by this feature, from 2.1.296 to 2.1.302):

- File: `<config dir>/sessions/<pid>.json`, one per live Claude Code process with a session (`kind` is `interactive` or `bg`). `claude attach` and the daemon have none.
- `pid`, `sessionId`, `kind`, `jobId` (bg only), `parkedJobId` (interactive only, optional), `procStart`, `cwd`, `version`, `updatedAt`.
- `procStart` is the output of `LC_ALL=C TZ=UTC ps -o lstart= -p <pid>`, for example `"Sun Oct 18 07:12:37 2026"`: a UTC wall-clock time with one-second precision. Claude Code itself uses it to tell a live process from a reused pid (`isSameProcess(pid, procStart)`). On Windows it writes `procStartFt` instead, which is out of scope.
- In the binary, `parkedJobId` is set (`{parkedJobId: <jobId>}`) when the TUI moves its session to the background and keeps showing it, cleared (`{parkedJobId: undefined}`) when the background spawn fails, and cleared together with the `sessionId` change when the TUI switches to another conversation (`/clear`, `/resume`, new session). The record disappears when the TUI exits. Claude Code's own listings treat `kind == "interactive" && parkedJobId !== undefined` as "a session moved to the background from this window".
- The `sessions` and `jobs` directories of the cc-deck profiles are symbolic links to `~/.claude/sessions` and `~/.claude/jobs`, so one directory holds records of processes with different `CLAUDE_CONFIG_DIR` values.

**Decision**:

- Records are read from `<config dir>/sessions/`, where the config dir is the worker's `CLAUDE_CONFIG_DIR` (else `$HOME/.claude`), symlink-resolved.
- A record counts only when its process is live and its `procStart`, parsed as UTC with layout `Mon Jan _2 15:04:05 2006` after collapsing repeated spaces, is within one second of the process start time from `procinfo` (FR-008). A record without `procStart` does not count.
- The configuration directory of a process is taken from its own environment, never from the directory the record lives in.
- The exact lifetime of `parkedJobId` while the TUI shows the job is checked manually in quickstart.md (scenario Q1); the design does not depend on more than "set while parked, cleared or removed when the TUI stops showing the job".

**Live validation, 2026-10-18, Claude Code 2.1.302**: In an isolated cc-deck workspace, a foreground conversation answered a smoke-test prompt, then `/background` printed `backgrounded` and returned to the shell. Its new worker record had `kind: "bg"` and a job id, and no interactive record named that job in `parkedJobId`. `claude attach <jobId>` reopened the conversation. This installed release frees the original terminal instead of retaining a parking viewer, so the parked-viewer form of US1 does not apply to that default UI. A headless job releases its former row on its next event, and an attach viewer is the supported way to show the job again. The in-process view behind `tengu_bg_leftarrow_inprocess` was not enabled or tested.

**Rationale**: One-second precision on Claude Code's side makes the tolerance necessary on Linux, where `procinfo` derives the start from boot time plus clock ticks and `ps` rounds the same way. A reused pid within one second that also writes a record of the same configuration directory is not a realistic case.

**Alternatives considered**: Comparing `startedAt` (milliseconds) was rejected: it is the time Claude Code registered, not the process start, and differs by up to seconds.

## R3. Process start time in seconds

**Decision**: `procinfo.Process` gains `StartUnix int64` (seconds since the epoch). Darwin: `P_starttime.Sec`. Linux: `btime` from `/proc/stat` plus `starttime / 100` (USER_HZ is 100 on every Linux ABI the release targets; reading `sysconf(_SC_CLK_TCK)` would need cgo). `StartTime` keeps its meaning for pid-reuse checks inside cc-deck.

**Alternatives considered**: Running `ps -o lstart=` per process, as Claude Code does, was rejected: a subprocess per candidate breaks the latency budget (SC-004) and the "no subprocess on the hot path" rule of spec 089.

## R4. Finding Claude Code processes

**Observed**: The kernel name (`P_comm` on Darwin, `comm` on Linux) of a native Claude Code process is the executed file name, which is the version (`2.1.302`) for processes started from `~/.local/share/claude/versions/<version>`, and `claude` for the macOS app bundle binary. `ps -o comm` shows the symlink path instead, which `procinfo` does not see.

**Decision**: `procinfo.Source` gets `ListUserMatching(match func(comm string) bool) ([]Process, error)`; `ListUser()` becomes `ListUserMatching(isCodexComm)` with unchanged behavior. Claude Code candidates have a kernel name of `claude`, a version shape (`^\d+\.\d+\.\d+([.-].*)?$`), or `node` (npm installs run `node .../@anthropic-ai/claude-code/cli.js`). After reading the arguments, a candidate is a Claude Code process when `argv[0]` has basename `claude` or a version shape, or, for `node`, when `argv[1]` ends in `/@anthropic-ai/claude-code/cli.js`; its CLI arguments are what follows.

The "an unreadable matching process fails the listing" rule of spec 089 applies: a candidate that still runs but cannot be read drops the event (FR-011).

**Alternatives considered**: Filtering by argument content before reading arguments is impossible; reading arguments of every user process costs one `kern.procargs2` call each and was rejected for SC-004.

## R5. Which processes view a job

**Decision**: Viewers of job `J` with configuration directory `C` are:

1. **Parked viewers**: records in `C/sessions/` with `kind: "interactive"` and `parkedJobId == J`, whose process is live and verified (R2) and whose own environment names configuration directory `C`.
2. **Attach viewers**: Claude Code processes (R4) whose CLI arguments have `attach` as the first non-flag argument and `J` (or the job's full session id) as the next non-flag argument, and whose environment names configuration directory `C`.

Viewers are grouped by terminal: viewers inside Zellij with the same `ZELLIJ_SESSION_NAME` and `ZELLIJ_PANE_ID` count as one terminal (a parked TUI that runs an attach child in its own pane is one viewer), and each viewer outside Zellij counts on its own.
One terminal: delivered to it (or dropped as `viewer-not-in-zellij`). None: `headless`. Several: `several-viewers`.

**Rationale**: Exact matching on Claude Code's own data, as decided in the clarifications. Grouping by pane avoids a false "several viewers" when two processes in one terminal show the job.

**Alternatives considered**:

- `jobs/<jobId>/state.json`: holds state, intent and output, but no viewer data (fields checked: no `viewers`, `attached`, or similar).
- The daemon control socket (`/tmp/cc-daemon-<uid>/<hash>/control.sock`): undocumented protocol, and the upstream issue asks for exactly this as a supported interface; rejected until it exists.
- Matching viewers by working directory: rejected in the clarifications.

## R6. Explicit viewer identity (FR-010)

**Decision**: The Claude Code payload is parsed for an optional object `viewer_env` (string values, same keys as Codex `client_env`: `ZELLIJ_PANE_ID`, `ZELLIJ_SESSION_NAME`, `CC_DECK_PROFILE`, `CC_DECK_PROFILE_COLOR`, `CC_DECK_PROFILE_ICON`). When it names a valid pane and session, it wins over record lookup (reason `viewer-env`). Because the upstream issue proposes viewer context "similar to" Codex `client_env` without a field name, `client_env` is accepted as an alias. Validation reuses `clientEnvTarget` with `sessionInherited = true`.

**Rationale**: Mirrors spec 089, so whichever spelling Claude Code ships first works without another cc-deck release in the common case. Should the upstream field differ, only the parser changes.

## R7. Remembering the last viewer pane (FR-007a)

**Decision (revised by deep review, 2026-10-19)**: A new state file `~/.local/state/cc-deck/bg-job-panes.json` maps `<config dir>|<jobId>` to `{pane_id, zellij_session, session_id, updated_at}`, plus unfinished releases and their retry deadlines. Per-job hashed lock files serialize fresh viewer discovery and delivery with a 100 ms acquisition limit. The shared `bg-job-panes.lock` protects only atomic merges with a 20 ms wait; no Zellij I/O holds it. Intent is recorded before delivery, and locking or required write failure drops the tracked event. Ownership does not expire by age: a twelve-hour TTL can forget a still-visible overnight row and violate FR-007a. Failed releases persist for retry, subject to revalidated viewer absence, one 100 ms deadline per event, and one-to-32-second failure backoff. Unchanged ownership refreshes after one hour. `pane-map.json` remains unchanged in behavior: background events never record attribution there (FR-002), but background `SessionEnd` removes a former foreground mapping.

**Transport measurement**: Fifty real no-op release pipes to an isolated Zellij 0.45.1 controller completed in 23.24–29.25 ms (median 24.39 ms). With four concurrent senders, 60 calls took 27.76–75.84 ms (median 50.05 ms; 45 exceeded 40 ms). The final 100 ms aggregate cleanup budget replaces a too-small 40 ms trial while retaining a bounded retry cost below SC-004's 150 ms budget at the measured load.

**Rationale**: Keeping the file separate from `pane-map.json` keeps its rule ("used only for removal") structurally obvious: no attribution code reads it.

**Alternatives considered**: An extra key namespace inside `pane-map.json` was rejected because every reader of that file would have to skip the new keys.

## R8. Performance (SC-004)

**Estimate**: Foreground path: two to three `Ancestor` calls plus one `stat` and one small read of the TUI's record, well under 5 ms. Background path: the same walk, one listing of the user's processes (one `kern.proc.uid` sysctl or one `/proc` scan), argument reads for up to about 30 Claude Code candidates, and reading up to about 30 records of a few hundred bytes each. Spec 089 measured comparable discovery at a few milliseconds per process.

**Decision**: A benchmark with 30 fake Claude Code processes and 30 records guards SC-004 (budget 150 ms; expected below 20 ms).

## R9. Foreground fallback on inspection errors

**Decision**: When the ancestor walk fails (an ancestor of this user cannot be inspected), a Claude Code event takes today's path, unless the hook environment contains `CLAUDE_BG_BACKEND`; then it is dropped as `record-missing` (detail `host-unknown`).

**Rationale**: Codex drops on an unknown host because its daemon keeps `ZELLIJ`. For Claude Code, dropping foreground events on a transient inspection error would break FR-003 and SC-005, while today's path for a background event ends in `not-in-zellij` as long as Claude Code strips `ZELLIJ`. The environment hint closes the remaining gap (a future release that keeps `ZELLIJ`) without making the hint part of normal detection.

## R10. Plugin payload decoding and row identity

**Observed** (`cc-zellij-plugin/src/`):

- `pipe_handler.rs:7-29`: `HookPayload` derives `Deserialize` without `deny_unknown_fields`; unknown fields are ignored, so older plugin builds ignore `background` (FR-014a). A new field needs `#[serde(default)]`.
- `controller/state.rs:91`: rows are `BTreeMap<u32, Session>`, keyed by pane id; one pane has at most one row.
- `controller/hooks.rs:89-147`: an event of the same agent with a different `session_id` on an existing row resets the derived fields (`display_name` only when not `manually_renamed`, `working_dir`, profile fields, agent fields, pending prompts) and adopts the new id; agent, profile and directory name are filled again from the same event. Tests: `test_process_hook_session_replacement`, `test_session_replacement_preserves_manual_rename`.

**Decision**: FR-009 needs no plugin change: the job's first event (new session id) reuses the parked pane's row through the existing replacement path, and a manual rename survives.

## R11. Conditional row removal

**Observed**: Every removal path is keyed by pane id only: `SessionEnd` (`hooks.rs:23-51`, keeps a live pane's row as idle), `evict_session` (`state.rs:297-309`), `PaneClosed`, `remove_dead_sessions`, quarantine sweep, and `cc-deck:action` delete (which also closes the pane). None compares the session id.

**Decision**: A new pipe `cc-deck:release-row` with payload `{"pane_id": <u32>, "session_id": "<id>"}`. The controller evicts the row of `pane_id` (through `evict_session`) only when the row exists and its `session_id` equals the payload's; otherwise it does nothing. The hook sends it to the former viewer's Zellij session with `zellij --session <S> pipe --name cc-deck:release-row -- <json>`.

**Rationale**: The plugin is the only place that knows whom a row belongs to, so the ownership check (FR-007a) must run there. A separate pipe name is ignored by older plugin builds (`PipeAction::Unknown`), whereas a synthetic `SessionEnd` would be applied unconditionally by them, and an unknown `hook_event_name` would be treated as an ordinary event.

**Alternatives considered**: Synthetic `SessionEnd` with a session guard field: older plugins ignore the guard and evict or idle any row of the pane, including another agent's. Rejected.

## R12. Background marker

**Observed**: Line 1 of a row is `edge bar + status indicator + " " + agent prefix + name` (`sidebar_plugin/render.rs:578-698`, name budget `prefix_len` at 635); presence markers overwrite the right end of line 1; line 2 holds badges and branch. Per-row flags such as `badges`, `paused`, `in_worktree` travel `HookPayload` → `Session` → `RenderSession` (`render_broadcast.rs:93-112`) → render; `RenderSession` and `Session` fields use `#[serde(default)]`.

**Decision**: `background: bool` travels the same path: `HookPayload.background` (`#[serde(default)]`), set on the row from every hook event (`session.background = hook.background`, so the next foreground event clears it), `Session.background` (`#[serde(default)]`, persisted), `RenderSession.background` (`#[serde(default)]`). The sidebar draws `⇣` (U+21E3, downwards dashed arrow, one column) followed by a space between the agent prefix and the name, in the dim text style (`\x1b[2m`) rather than the profile or status color, and counts both columns in `prefix_len`. Edge bar, status indicator and line 2 stay where they are.

**Rationale**: The name area is the only place on line 1 that can absorb two columns without moving the indicator or the bar, and the dim style stays legible against every profile color because it does not depend on the profile color. U+21E3 has width 1 in the `unicode-width` 0.1 tables the sidebar uses for truncation, and it does not resemble any activity indicator (`○ ● ⚠ ✓`) or agent glyph (`✳ ◆ ❯`); circle glyphs such as `◌` were rejected because they read as the idle indicator `○`.

**Alternatives considered**: Replacing the status indicator (violates FR-014a), coloring the edge bar differently (collides with profile colors), a marker on line 2 (collides with badges and branch, and the spec keeps line 2 unchanged), a marker at the right end of line 1 (overwritten by presence markers).
