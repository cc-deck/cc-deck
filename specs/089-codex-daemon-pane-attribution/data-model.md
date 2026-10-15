# Data Model: Codex Shared Daemon Pane Attribution

**Feature**: 089-codex-daemon-pane-attribution | **Date**: 2026-10-15

## Go: `procinfo.Process` (new, `internal/procinfo`)

```go
type Process struct {
    PID       int
    PPID      int
    StartTime int64             // platform start time, opaque but comparable
    Exe       string            // argv[0]
    Args      []string          // argv[1:]
    Env       map[string]string // environment block (same-user processes only)
    Cwd       string            // Linux /proc cwd link; empty on Darwin
}

type Source interface {
    Self() int                       // pid of the current process
    Get(pid int) (Process, error)    // single process
    ListUser() ([]Process, error)    // processes of the current uid whose executable basename is "codex"
}
```

`ListUser` filters by executable basename before reading arguments and environment, so it never reads the environment of unrelated processes.

## Go: `codexClient` (derived, `internal/cmd/codex_attribution.go`)

| Field | Source | Rule |
|-------|--------|------|
| `pid`, `start` | `Process` | identity (R6) |
| `dir` | `-C`/`--cd` arg, else `Env["PWD"]`, else `Cwd` | `EvalSymlinks` + `Clean`; empty disqualifies |
| `codexHome` | `Env["CODEX_HOME"]` or `$HOME/.codex` | `EvalSymlinks` |
| `paneID`, `zellijSession` | `Env["ZELLIJ_PANE_ID"]`, `Env["ZELLIJ_SESSION_NAME"]` | both empty means outside Zellij |
| `profile` | `Env["CC_DECK_PROFILE"]`, `..._COLOR`, `..._ICON` | FR-017 |

Interactive when the executable basename is `codex` and the first non-flag argument is absent, `resume` or `fork` (R4).

## Go: `paneMapEntry` (extended, `internal/cmd/hook.go`)

```go
type paneMapEntry struct {
    PaneID        uint32 `json:"pane_id"`
    ZellijSession string `json:"zellij_session"`
    UpdatedAt     int64  `json:"updated_at"`
    // New, set only for daemon-hosted Codex attribution:
    ClientPID     int    `json:"client_pid,omitempty"`
    ClientStart   int64  `json:"client_start,omitempty"`
    Profile       string `json:"profile,omitempty"`
}
```

- Entries without `ClientPID` (written by Claude Code, OpenCode, non-daemon Codex, or older versions) remain valid for today's paths and are never used for daemon-hosted Codex events (FR-002).
- The file format stays backward compatible: older binaries ignore the new fields.
- Expiry (`paneMapTTL`, 12 h) and refresh (`paneMapRefresh`, 1 h) are unchanged.

## Go: attribution result

```go
type attribution struct {
    PaneID        uint32
    ZellijSession string
    Profile       profileEnv // name, color, icon from the client
    Reason        string     // one of the values below
}
```

| Reason (log value) | Meaning | Event delivered |
|--------------------|---------|-----------------|
| `from-arg` | not daemon-hosted; own `--pane-id` (today) | yes |
| `client-env` | explicit client identity in payload (FR-011) | yes |
| `from-client-cache` | live cached mapping (FR-008) | yes |
| `discovered` | exactly one matching client (FR-006) | yes |
| `ambiguous` | several matching clients (FR-007) | no |
| `no-client` | no matching client, or inspection failed | no |
| `client-not-in-zellij` | matching client outside Zellij (FR-010) | no |

Existing reasons (`from-cache`, `no-cache-entry`, `cache-session-mismatch`, `not-in-zellij`, `bad-pane-id-arg`, `no-pane-id-no-session-id`) keep their meaning for non-daemon events.

## State transitions: session mapping

```
(none) --discovered--> bound(pid,start,pane,session)
bound --cache hit, client live--> bound (UpdatedAt refreshed hourly)
bound --client exited / pid reused / pane changed--> (none) [entry ignored, overwritten on next discovery]
bound --SessionEnd--> (none) [entry removed, as today]
bound --TTL 12h--> (none)
```

## Files

| Path | Change |
|------|--------|
| `~/.local/state/cc-deck/pane-map.json` | new optional fields; atomic rename writes |
| `~/.local/state/cc-deck/pane-map.lock` | new, empty lock file (flock target) |
| `~/.local/state/cc-deck/hook.log` | new reason values and a `session=` field for Codex events |
