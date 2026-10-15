# Research: Codex Shared Daemon Pane Attribution

**Feature**: 089-codex-daemon-pane-attribution | **Date**: 2026-10-15

Facts below were verified on macOS (Apple Silicon) with Codex 0.162.1 and Zellij 0.45, unless marked otherwise.

## R1. Detecting a daemon-hosted hook (FR-002, FR-003)

**Finding**: Hooks run as children of the managed daemon. The daemon's argv is `<codex binary> app-server --listen unix:// ...` (a sibling `codex app-server daemon pid-update-loop` also exists). Since Codex re-parents the daemon to pid 1, the TUI is never an ancestor of the hook.

**Decision**: Walk the hook's ancestor chain (parent, grandparent, at most 8 levels, stopping at pid 1). The hook is daemon-hosted when an ancestor's executable basename is `codex` and its argv contains the `app-server` subcommand. The hook command itself runs under `sh -c` spawned by the daemon, so the daemon is usually the grandparent.

**Rationale**: Process ancestry is the only signal Codex does not control through environment inheritance. One or two process lookups cost microseconds.

**Alternatives considered**: environment markers set by Codex (none exist for hooks today); trusting `--pane-id` and validating it against Zellij's pane list (cannot tell a live wrong pane from the right one).

## R2. Process inspection without cgo (FR-004, FR-005)

**Finding**: Release builds use `CGO_ENABLED=0`. `golang.org/x/sys/unix` (already in the module graph, v0.47.0) provides `SysctlKinfoProc("kern.proc.pid", pid)` and `SysctlKinfoProcSlice("kern.proc.uid", uid)` on Darwin, exposing `Ppid`, `P_starttime` and the 16-byte `P_comm`. `SysctlRaw("kern.procargs2", pid)` returns argc, argv and the environment block for processes of the same user. Linux exposes the same data in `/proc/<pid>/stat`, `cmdline`, `environ` and the `cwd` symlink.

**Decision**: New package `internal/procinfo` with a small `Source` interface (`Self`, `Get(pid)`, `ListUser()`) and two build-tagged implementations: `procinfo_darwin.go` (sysctl) and `procinfo_linux.go` (`/proc`). Other platforms return `ErrUnsupported`, which makes discovery report "no client found" (events dropped, never misattributed). `golang.org/x/sys` moves from indirect to direct dependency; no new module.

**Rationale**: No cgo, no subprocess per lookup, testable through the interface.

**Alternatives considered**: shelling out to `ps eww` and `lsof` (tens of milliseconds per call and locale-sensitive output); cgo `libproc` (breaks `CGO_ENABLED=0` releases).

## R3. Working directory of a TUI (FR-004)

**Finding**: Darwin has no cgo-free API for another process's cwd. The TUI's environment contains `PWD` set by the launching shell (observed `PWD=/Users/rhuss/Work/projects/agentmeter`). Codex accepts `-C <dir>` and `--cd <dir>` (also `--cd=<dir>`), relative to the launch directory.

**Decision**: The client directory is the `-C`/`--cd` argument (resolved against `PWD` when relative) when present, else `PWD` from the client environment; on Linux, the `/proc/<pid>/cwd` link is used when neither is available. Both sides of the comparison pass through `filepath.EvalSymlinks` and `filepath.Clean` (macOS `/tmp` vs `/private/tmp`). A client without a determinable directory is not a candidate.

**Rationale**: `PWD` is exported by every POSIX shell and matches what the user typed `codex` in; the explicit flag wins, as it does in Codex.

**Alternatives considered**: `lsof -d cwd` fallback on Darwin (adds 30 to 100 ms; deferred unless field testing shows `PWD` missing).

## R4. Identifying interactive Codex clients (FR-004)

**Finding**: The TUI's executable basename is `codex` (standalone install under `~/.codex/packages/standalone/...`, Homebrew and npm native binaries alike; the npm Node shim has `comm=node` and is ignored). Non-interactive Codex processes carry subcommands: `app-server`, `exec`, `mcp`, `mcp-server`, `proto`, `login`, `logout`, `apply`, `completion`, `debug`, `cloud`, `agents`, `remote-control`.

**Decision**: Two sets, both limited to processes of the current user whose executable basename is `codex`:
- *Matching clients* (used for the ambiguity count): every such process in the hook's Codex home whose directory equals the payload `cwd`, except processes whose args contain `app-server` (the daemon and its helpers). This includes `codex exec`, unknown future subcommands, and clients outside Zellij.
- *Bindable clients*: matching clients whose first non-flag argument is absent, `resume` or `fork`.

An event is attributed only when exactly one matching client exists and it is bindable. Several matching clients mean `ambiguous`; a single non-bindable match means `no-client`. A plain-terminal client in the same directory therefore still makes the match ambiguous (FR-007) instead of letting a Zellij client win.

**Rationale**: Counting every Codex process for ambiguity keeps "never a wrong row" independent of how Codex names its subcommands; the allow-list only restricts which process may receive a row.

## R5. Codex home scoping (FR-004)

**Decision**: The daemon's Codex home is `CODEX_HOME` from the hook's environment, defaulting to `$HOME/.codex`. A client's Codex home is `CODEX_HOME` from its environment with the same default. Paths compare after `EvalSymlinks`.

## R6. Client identity and liveness (FR-005, FR-008, FR-009)

**Decision**: A client is identified by `(pid, start_time)`; start time is `P_starttime` (Darwin, seconds plus microseconds) or field 22 of `/proc/<pid>/stat` (Linux, clock ticks since boot). A cached mapping is live when `Get(pid)` succeeds, reports the same start time, is still an interactive Codex client, and its environment still names the same Zellij pane and session.

## R7. Delivery to the client's Zellij session (FR-010)

**Finding**: `zellij --session <name> pipe --name cc-deck:hook -- <payload>` targets a named session; without `--session`, `zellij pipe` uses the ambient `ZELLIJ_SESSION_NAME` of the caller, which for a daemon-hosted hook is the daemon's.

**Decision**: `sendHookPayload` gains a session argument. For daemon-hosted Codex events it passes the client's `ZELLIJ_SESSION_NAME` via `--session`; for all other events it passes the current session (today's behavior). The `not-in-zellij` early exit in `runHook` is skipped for daemon-hosted Codex events, because the daemon's own Zellij membership is irrelevant.

## R8. Pane-map concurrency (FR-012, inbox `pane-map-write-race`)

**Decision**: Every read-modify-write of `pane-map.json` runs under an exclusive `flock` on `pane-map.lock` in the same directory (blocking, with a 2 s timeout after which the update is skipped). Writes go to a temp file in the same directory and are renamed into place. Read-only lookups take no lock, because rename makes partial reads impossible.

**Rationale**: `flock` is advisory, cheap, and released automatically when the process dies; rename is atomic on the same filesystem.

## R9. Explicit client identity (FR-011)

**Finding**: openai/codex#44902 proposes a hook-scoped client environment allowlist; the payload shape is not final.

**Decision**: The Codex payload parser accepts an optional `client_env` object. When it carries `ZELLIJ_PANE_ID` (and optionally `ZELLIJ_SESSION_NAME`), that identity wins over every other path. The field name follows the proposal and is isolated in one place so it can follow the final upstream shape.

## R10. Profile enrichment (FR-017)

**Decision**: For daemon-hosted events, `enrichProfile` reads `CC_DECK_PROFILE`, `CC_DECK_PROFILE_COLOR` and `CC_DECK_PROFILE_ICON` from the resolved client's environment instead of `os.Getenv`. The cached mapping stores the profile name so cache hits need no environment read; color and icon fall back to the config lookup, as today for wrappers without exports.

## R11. Performance (SC-004, SC-005)

**Decision**: Cache hit path: one ancestor walk (1 to 3 sysctl calls) plus one liveness check (1 sysctl plus one `procargs2` read). Discovery path: one `kern.proc.uid` listing, filtered by `P_comm == "codex"` before reading `procargs2`. Both measured by a Go benchmark against the real `procinfo` source on the CI host and a synthetic table of 10 clients.
