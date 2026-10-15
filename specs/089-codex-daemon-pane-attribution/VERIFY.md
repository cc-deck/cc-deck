# Verification: Codex Shared Daemon Pane Attribution

**Feature**: 089-codex-daemon-pane-attribution | **Date**: 2026-10-15

Host: Apple M4 Pro (macOS 26.7.1), Go 1.27.2, Codex 0.162.1 with a running shared daemon, Zellij 0.45.1.

## Performance (T024)

Command:

```sh
cd cc-deck && go test ./internal/cmd/ -run '^$' -bench CodexAttribution -benchtime=2s
```

| Sub-benchmark | Time per op | What it measures |
|---------------|-------------|------------------|
| `baseline-pane-map-lookup` | 0.031 ms | Today's non-daemon pane-map lookup (30 entries) |
| `cache-hit-real-procinfo` | 0.226 ms | Real ancestor walk (`daemonHosted`, up to 8 levels) plus one real liveness `Get` plus the pane-map lookup |
| `list-user-real-procinfo` | 0.230 ms | Real `ListUser` on this host (about 716 processes of the user, 3 or 4 of them `codex` during the runs) |
| `discovery-fake-10-clients` | 0.253 ms | `resolveCodex` discovery over a synthetic table of 10 clients (includes symlink resolution and the pane-map read) |

- **SC-004** (cache hit adds at most 20 ms): 0.226 ms minus the 0.031 ms baseline is about 0.2 ms added. PASS.
- **SC-005** (first attributed event within 500 ms with up to 10 Codex clients): real listing 0.230 ms plus 10-client resolution 0.253 ms is about 0.5 ms. PASS.

The benchmark process is not hosted by the daemon, so the ancestor walk runs to its eight-level limit or to pid 1, which is the slower case for that step.

## Real process data (darwin source)

A throwaway listing through `procinfo.Default().ListUser()` on this host returned the TUI (`codex`, `PWD=/Users/rhuss/Work/projects/agentmeter`, `ZELLIJ_PANE_ID=25`) and the shared daemon (`codex app-server --listen unix:// --analytics-default-enabled --managed-daemon`) with its helper (`codex app-server daemon pid-update-loop`), both carrying the inherited `ZELLIJ_PANE_ID=19`. That is the reported defect, observed live. All 716 readable processes of the user parsed through `ParseProcArgs2` without truncation.

The Linux source was tested in `docker.io/library/golang:1.25` under podman: all `internal/procinfo` tests pass there, including the smoke test against the real `/proc`.

## Automated checks (T030, automated part)

What was actually run on 2026-10-15, on the host above:

| Check | Result |
|-------|--------|
| `make verify` | Exit 2: `make test` fails only on the 9 `TestComposeSmoke*` tests in `internal/cmd`, which also fail on unmodified `main` on this machine (the local podman compose image lacks zellij). Every other Go package passes (24 `ok`). `make verify` stops before lint because of that, so lint ran separately. |
| `make lint` | PASS (`go vet ./...` and `cargo clippy -- -D warnings`) |
| `make test-rust` | PASS (464 + 13 tests; the plugin is unchanged) |
| `go mod tidy -diff` | No diff (`golang.org/x/sys` is a direct dependency at the existing v0.47.0) |
| `go test -race` on the attribution, pane-map, hook, procinfo and agent tests | PASS |
| Linux (`golang:1.25` in podman): `internal/procinfo`, `internal/agent` and the attribution, pane-map and hook tests of `internal/cmd` | PASS |
| SC-006 concurrency test (20 goroutines, 20 processes) | PASS; with the lock disabled the same test reports lost updates |
| SC-007: the existing Claude Code and OpenCode hook tests | PASS, unchanged apart from the shared `sendHookPayload` test double, which now records the target session |

Live check against the real process table (a throwaway test, not committed): the host runs a shared daemon whose environment carries `ZELLIJ_PANE_ID=19` and a Codex TUI in pane 25 of `cc-deck-local` in `/Users/rhuss/Work/projects/agentmeter`. `resolveCodex` with the real `procinfo` source and that directory returned `resolution=discovered pane=25 session=cc-deck-local` for the TUI's pid. The daemon and its helper were recognized as the shared daemon and excluded from matching.

## Manual quickstart (T030, manual part): PENDING

The quickstart sections need a real Codex and Zellij session driven by hand with the installed binary (`make install`, Zellij restart, `touch ~/.local/state/cc-deck/hook-debug`). They were NOT run by the implementation agent, and T030 stays unticked until they are:

- [ ] Section 1: stale daemon identity (US1, SC-001)
- [ ] Section 2: three clients, one daemon (US2, SC-002)
- [ ] Section 3: same directory, `resolution=ambiguous`, attribution after one client quits (US2, SC-003)
- [ ] Section 4: plain-terminal client (`client-not-in-zellij`) and a second Zellij session (US3)
- [ ] Section 5: `codex --no-daemon` keeps `resolution=from-arg`
- [ ] Section 6: Claude Code next to the Codex clients, unchanged sidebar behavior (SC-007)
