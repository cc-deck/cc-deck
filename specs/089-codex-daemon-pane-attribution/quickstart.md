# Quickstart: Codex Shared Daemon Pane Attribution

**Feature**: 089-codex-daemon-pane-attribution

## Prerequisites

- `make install` from this branch, and `~/bin/cc-deck` (or the `cc-deck` on `PATH`) pointing at the new binary; restart the Zellij session so the plugin and hooks use it.
- Codex 0.157 or later with the shared daemon enabled (default). `cc-deck config plugin install` has registered the Codex hooks (`~/.codex/hooks.json`), and Codex lists them as trusted.
- Enable hook debugging: `touch ~/.local/state/cc-deck/hook-debug`; truncate `~/.local/state/cc-deck/hook.log`.

## 1. Stale daemon identity (US1, SC-001)

1. Pane A: `cd ~/tmp/a && codex`, send one prompt (this starts the daemon from pane A).
2. Close pane A. Confirm the daemon still runs: `pgrep -fl 'codex app-server'`.
3. Pane B: `cd ~/tmp/b && codex`, send a prompt that runs a tool (for example "run ls").

Expected: pane B's sidebar row shows the Codex session. `hook.log` shows `resolution=discovered` for the first event and `resolution=from-client-cache` afterwards, never pane A's id.

## 2. Three clients, one daemon (US2, SC-002)

Start Codex in three panes in `~/tmp/a`, `~/tmp/b`, `~/tmp/c`, drive each through a prompt.

Expected: three rows, each changing only with its own pane. `/new` inside one client creates a new session in the same row.

## 3. Same directory (US2 scenarios 2 and 3, SC-003)

Start Codex in two panes in `~/tmp/same`.

Expected: new sessions are not shown while both run; `hook.log` shows `resolution=ambiguous`. Quit one client: the next event of the remaining client is attributed.

## 4. Foreign clients (US3)

1. Plain terminal outside Zellij: `cd ~/tmp/outside && codex`, send a prompt. Expected: no sidebar change; `resolution=client-not-in-zellij`.
2. Second Zellij session S2: start Codex there. Expected: it appears only in S2's sidebar.

## 5. No daemon (US1 scenarios 3 and 4)

`codex --no-daemon` in any pane. Expected: attributed as before (`resolution=from-arg`).

## 6. Claude Code unaffected (SC-007)

Run a Claude Code session next to the Codex clients. Expected: unchanged sidebar behavior; `make test` passes the existing hook tests unchanged.

## Automated checks

```sh
make test      # unit tests incl. procinfo parsers, attribution table, pane-map concurrency
make lint
go test ./internal/cmd/ -run Benchmark -bench CodexAttribution   # SC-004, SC-005 budgets
```
