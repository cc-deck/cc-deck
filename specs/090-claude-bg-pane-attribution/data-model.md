# Data Model: Claude Code Background Session Pane Attribution

**Feature**: 090-claude-bg-pane-attribution

All Go types live in `cc-deck/internal/cmd` unless stated otherwise; they are unexported.

## procinfo.Process (extended, `internal/procinfo`)

| Field | Type | Change | Notes |
|-------|------|--------|-------|
| `StartUnix` | `int64` | new | Start time in seconds since the epoch (R3). Darwin: `P_starttime.Sec`. Linux: `btime + starttime/100`. Zero when unknown, which never matches a record. |
| `Comm` | `string` | new | Kernel process name (`P_comm` on Darwin, `stat` comm on Linux), the value `ListUserMatching` filters on (R4). |

`procinfo.Source` gains `ListUserMatching(match func(comm string) bool) ([]Process, error)`.
`ListUser()` keeps its contract (Codex processes only) and is implemented through it.

## claudeRecord

A parsed `<config dir>/sessions/<pid>.json` (R2).

| Field | JSON | Notes |
|-------|------|-------|
| `PID` | `pid` | Must equal the file name's pid; a mismatch makes the record invalid. |
| `SessionID` | `sessionId` | |
| `Kind` | `kind` | `interactive` or `bg`; other values are ignored. |
| `JobID` | `jobId` | `bg` only. Valid when it matches `^[0-9a-f]{8}$`. |
| `ParkedJobID` | `parkedJobId` | `interactive` only, optional, same shape as `JobID`. |
| `ProcStart` | `procStart` | UTC `ps -o lstart` text; parsed to seconds. Missing or unparsable: record invalid. |

**Validation**: A record is *verified* when its pid is live, the process belongs to the current user, and `|ProcStart - StartUnix| <= 1 s`. Unverified records are ignored, never trusted.

## claudeHost

Outcome of the ancestor walk (contract section 2).

| Value | Meaning |
|-------|---------|
| `claudeForeground` | First verified record on the way up is `interactive`, or no record and no PTY host within eight levels. Today's path. |
| `claudeBackground` | First verified record is `bg` (`worker` set), or a `--bg-pty-host` ancestor was reached without a verified record (`worker` empty, reason `record-missing`). |
| `claudeHostUnknown` | An ancestor could not be inspected. Treated as foreground unless the hook environment has `CLAUDE_BG_BACKEND`, in which case the event is dropped (`record-missing`, detail `host-unknown`). |

## bgWorker

| Field | Source |
|-------|--------|
| `pid`, `start` | procinfo |
| `jobID`, `sessionID` | worker record |
| `configDir` | worker's `CLAUDE_CONFIG_DIR`, else `$HOME/.claude`, symlink-resolved |

## claudeViewer

| Field | Source |
|-------|--------|
| `pid`, `start` | procinfo (`StartTime` and `StartUnix`) |
| `via` | `parked` or `attach` |
| `configDir` | viewer's own environment, resolved as for `bgWorker` |
| `paneID`, `zellijSession` | viewer's `ZELLIJ_PANE_ID`, `ZELLIJ_SESSION_NAME` (outside Zellij when `ZELLIJ` is unset, the pane id does not parse, or the session name fails `validZellijSessionName`) |
| `profile` | viewer's `CC_DECK_PROFILE*` (`profileEnv`) |

Viewers are grouped into terminals by `(zellijSession, paneID)`; every viewer outside Zellij is a terminal of its own.

## bgAttribution

Result of resolving one background event; mirrors the Codex `attribution` type.

| Field | Notes |
|-------|-------|
| `Reason` | One of the reasons below. |
| `Detail` | Optional refinement for the log (inspection error text, `host-unknown`). |
| `JobID` | Empty when unknown. |
| `PaneID`, `ZellijSession`, `Profile` | Set when delivered. |
| `Terminals` | `[]terminalKey` (`{ZellijSession, PaneID}`): the in-Zellij viewer terminals found; set for every reason that collected viewers, so the `several-viewers` transitions can tell whether the remembered pane is among them. |

### Resolution reasons (hook debug log, FR-014)

| Reason | Delivered | Meaning |
|--------|-----------|---------|
| `viewer-env` | yes | Payload carried an explicit viewer identity (R6). |
| `parked-viewer` | yes | Single terminal, from a parked TUI record. |
| `attach-viewer` | yes | Single terminal, from a `claude attach` process. |
| `headless` | no | No viewer. |
| `several-viewers` | no | Two or more terminals. |
| `viewer-not-in-zellij` | no | Single terminal outside Zellij. |
| `record-missing` | no | Worker record missing, unverified, or without job id; a viewer or candidate could not be read; listing failed. |

When parked and attach viewers share the single terminal, the reason is `parked-viewer`.

## bgJobPane (`bg-job-panes.json`)

Key: `<resolved config dir>|<jobId>`.

| Field | JSON | Notes |
|-------|------|-------|
| `PaneID` | `pane_id` | Pane that last received an event of the job. |
| `ZellijSession` | `zellij_session` | Its Zellij session. |
| `SessionID` | `session_id` | Session id of that event (the job's session). |
| `UpdatedAt` | `updated_at` | Unix seconds; refresh unchanged ownership after one hour. No age-only expiry. |
| `Released` | `released,omitempty` | No current row remains owned; entry exists only for pending cleanup. |
| `Pending` | `pending,omitempty` | Unfinished `{pane_id,zellij_session,session_id}` removals, with `attempts` and `retry_after` (Unix milliseconds). |

### State transitions

| Current entry | Event outcome | Action |
|---------------|---------------|--------|
| none | delivered to pane P | write `{P, S, sid}` |
| `{P, S, sid}` | delivered to P in S | refresh only when older than 1 h |
| `{P, S, sid}` | delivered to P' (P' or S' differs) | send removal for `{P, S, sid}` (plugin checks ownership), then write `{P', S', sid'}` |
| `{P, S, sid}` | `headless`, or `viewer-not-in-zellij` | send removal for `{P, S, sid}`, delete entry |
| `{P, S, sid}` | `several-viewers` and P is not among the viewer terminals | send removal, delete entry |
| `{P, S, sid}` | `several-viewers` and P is among them | keep entry, no removal |
| `{P, S, sid}` | `record-missing` | keep entry, no removal (nothing is known) |
| any | `SessionEnd` delivered | clear current ownership; retain unfinished pending cleanup until successful release |

Per-job locks serialize viewer discovery and transitions.
The shared ledger lock protects only atomic read-modify-write merges, never Zellij I/O.
Transition intent, including pending cleanup, is persisted before delivery; write failure drops the event.
Successful removal consumes its pending record; failed or interrupted delivery is retried idempotently under the plugin's ownership guard.
A release is deferred when its pane appears among current viewers or discovery is incomplete.
A unique viewer returning to the same owned row cancels that pending release.
All cleanup attempts share a 100 ms deadline per event; failures back off from one to 32 seconds.
No age-only expiry removes outstanding ownership, because rows can persist longer than any idle interval.

## NormalizedPayload (extended, `internal/agent`)

| Field | JSON | Notes |
|-------|------|-------|
| `Background` | `background,omitempty` | `true` for every delivered background event; absent otherwise, so foreground payloads stay byte-identical (FR-014a, SC-005). |

The plugin side of the payload, the row state and the removal message are specified in [contracts/claude-bg-attribution.md](contracts/claude-bg-attribution.md) sections 5 and 6.

## Files

| Path | Change |
|------|--------|
| `~/.local/state/cc-deck/bg-job-panes.json` | new |
| `~/.local/state/cc-deck/bg-job-panes.lock` | new |
| `~/.local/state/cc-deck/bg-job-<hash>.lock` | per-job serialization; empty files retained to avoid lock-inode races |
| `~/.local/state/cc-deck/pane-map.json` | not written by background events; `SessionEnd` still removes entries |
| `~/.local/state/cc-deck/hook.log` | new line shape for background Claude Code events |
