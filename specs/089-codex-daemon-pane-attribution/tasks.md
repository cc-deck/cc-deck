# Tasks: Codex Shared Daemon Pane Attribution

**Input**: Design documents from `/specs/089-codex-daemon-pane-attribution/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/codex-attribution.md, quickstart.md

**Tests**: Included. Constitution principle I requires unit tests for new code; the contract (section 6) lists eight contract tests.

**Organization**: Grouped by user story. Paths are repository-relative; Go code lives under `cc-deck/`. Build and test only via `make test`, `make lint`, `make install` (constitution III).

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies on incomplete tasks)
- **[Story]**: US1 (hand-started session in its own row), US2 (several sessions, one daemon), US3 (foreign clients stay out), US4 (diagnosing attribution)

## Phase 1: Setup (Shared Infrastructure)

- [x] T002 [P] Create `cc-deck/internal/procinfo/procinfo.go` with `Process` (PID, PPID, StartTime int64, Exe, Args, Env map[string]string, Cwd), the `Source` interface (`Self() int`, `Get(pid int) (Process, error)`, `ListUser() ([]Process, error)` returning only same-uid processes whose executable basename is `codex`), `ErrUnsupported`, and `Default() Source` (data-model.md)

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Process inspection and a race-free pane-map cache. Every story depends on these.

- [x] T003 Implement the Darwin source in `cc-deck/internal/procinfo/procinfo_darwin.go` (build tag `darwin`): `Get` via `unix.SysctlKinfoProc("kern.proc.pid", pid)` for PPID, `P_starttime` (seconds*1e6+microseconds) and `P_comm`, plus `unix.SysctlRaw("kern.procargs2", pid)` for argv and environment; `ListUser` via `unix.SysctlKinfoProcSlice("kern.proc.uid", uid)` filtered by `P_comm == "codex"` before reading procargs (research R2); this first import of `golang.org/x/sys/unix` promotes `golang.org/x/sys` from indirect to direct in `cc-deck/go.mod` (same version v0.47.0, no new module), and `make lint` must show that `go mod tidy` leaves no diff (formerly T001, folded here because it has no deliverable of its own)
- [x] T004 [P] Implement the procargs2 parser as a pure function in `cc-deck/internal/procinfo/procargs.go` (no build tag): input is the raw buffer (int32 argc, exec path, NUL padding, argv, env), output is exe, args, env map; tolerate truncated buffers by returning what parsed so far plus an error
- [x] T005 [P] Implement the Linux source in `cc-deck/internal/procinfo/procinfo_linux.go` (build tag `linux`) reading `/proc/<pid>/stat` (ppid field 4, starttime field 22, comm in parentheses which may contain spaces), `cmdline`, `environ` and the `cwd` link, with the proc root injectable for tests; `ListUser` scans the numeric entries of the proc root, keeps those owned by the current uid (owner of `/proc/<pid>`) whose `stat` comm is `codex`, and only then reads `cmdline`, `environ` and `cwd`
- [x] T006 [P] Add `cc-deck/internal/procinfo/procinfo_other.go` (build tag `!darwin && !linux`) returning `ErrUnsupported` from every method
- [x] T007 [P] Write `cc-deck/internal/procinfo/procargs_test.go` (byte fixtures: normal, empty env, truncated) and `cc-deck/internal/procinfo/procinfo_linux_test.go` (temp proc root fixtures: comm with spaces, missing environ, exited process) plus a smoke test that `Default().Get(Default().Self())` returns the test binary with a nonzero start time on darwin and linux
- [x] T008 Make `pane-map.json` writes atomic and serialized in `cc-deck/internal/cmd/hook.go`: `savePaneMap` writes a temp file created with `os.CreateTemp(hookStateDir, "pane-map-*.json")`, syncs and renames it over `pane-map.json`; every read-modify-write (`recordPaneMapping`, the SessionEnd removal, and the new Codex paths) runs inside a `withPaneMapLock(fn func()) bool` helper holding an exclusive `syscall.Flock` on `pane-map.lock`, polled with `LOCK_NB` every 10 ms up to a 2 s deadline (same pattern as `cc-deck/internal/share/lock.go`), skipping the write on timeout; read-only lookups take no lock (research R8, contract section 4)
- [x] T009 Add `client_pid`, `client_start` and `profile` (all `omitempty`) to `paneMapEntry` in `cc-deck/internal/cmd/hook.go` and a `recordClientMapping(sessionID string, e paneMapEntry)` helper that writes them inside `withPaneMapLock` with the existing hourly refresh rule (data-model.md)
- [x] T010 [P] Write a concurrency test in `cc-deck/internal/cmd/hook_test.go`: 20 goroutines (and a second variant with 20 `exec` subprocesses of the test binary via a helper flag) each record a distinct session; afterwards the file parses and contains all 20 sessions, and no reader in a parallel loop ever fails to decode (SC-006)

**Checkpoint**: `make test` passes; Claude Code and OpenCode hook tests unchanged.

---

## Phase 3: User Story 1 - A hand-started Codex session shows up in its own row (Priority: P1) 🎯 MVP

**Goal**: A Codex event hosted by the shared daemon reaches the pane of the single matching client, never the daemon's inherited pane.

**Independent Test**: quickstart section 1, and contract tests 1, 5 and 6.

### Tests for User Story 1

- [x] T011 [P] [US1] Create `cc-deck/internal/cmd/codex_attribution_test.go` with a fake `procinfo.Source` (process table, settable errors) and contract tests 1 (inherited pane exists, one matching client elsewhere: delivered to the client's pane, inherited pane never used), 5 (cached pid reused by a process with another start time: cache ignored, discovery runs) and 6 (no app-server ancestor: identical behavior to today for the existing hook table); plus cases for `-C`/`--cd`/`--cd=` relative to `PWD`, symlinked directories, `CODEX_HOME` mismatch excluding a client, non-interactive subcommands (`exec`, `mcp-server`, `app-server`) never being candidates, and `resume`/`fork` being candidates; and these FR-002, FR-009, FR-010 and FR-017 cases: a legacy cache entry without `client_pid` is ignored for a daemon-hosted event; after daemon-hosted events of every outcome, `pane-map.json` holds no entry with the inherited pane; a cached client that has exited (`Get` returns an error) is not used and discovery runs; a cached entry whose Zellij session differs from the client's environment is not used; `ListUser` or `Get` errors drop the event as `no-client` without touching the inherited pane; a hook environment without `ZELLIJ`/`ZELLIJ_SESSION_NAME` (daemon started from a plain terminal) still delivers to the single client's pane and session; and a daemon-hosted event carries the client's `CC_DECK_PROFILE*` values, not the hook's. Tests inject the fake through the `hookProcSource` package variable defined in T015 and observe delivery through the `sendHookPayload` double; and the ambiguity-count rule (research R4): a `codex exec` in the same directory and Codex home as one TUI makes the TUI's new session `ambiguous`, a lone `codex exec` match yields `no-client`, and the daemon's own `app-server` processes never count even when their `PWD` matches

### Implementation for User Story 1

- [x] T012 [US1] Implement daemon detection in `cc-deck/internal/cmd/codex_attribution.go`: `daemonHosted(src procinfo.Source) bool` walks at most 8 ancestors from `src.Self()` stopping at pid 1, true when an ancestor's executable basename is `codex` and its args contain `app-server` (research R1)
- [x] T013 [US1] Implement client derivation in `cc-deck/internal/cmd/codex_attribution.go`: `toClient(p procinfo.Process, home string) (codexClient, bool)` applying the interactive rule (first non-flag arg absent, `resume` or `fork`), directory from `-C`/`--cd`/`--cd=` (relative to `PWD`) else `PWD` else `Cwd`, `CODEX_HOME` defaulting to `$HOME/.codex`, both via `filepath.EvalSymlinks`+`Clean`, Zellij identity and `CC_DECK_PROFILE*` from `Env` (research R3 to R5, data-model.md) Return both whether the process is a matching client (any `codex` process without `app-server` in its args, same Codex home and directory) and whether it is bindable (interactive rule), so `resolveCodex` counts all matching clients for ambiguity and binds only bindable ones.
- [x] T014 [US1] Implement `resolveCodex(in codexInput, src procinfo.Source, paneMap map[string]paneMapEntry) attribution` in `cc-deck/internal/cmd/codex_attribution.go`, where `codexInput` is `struct { SessionID, Cwd, CodexHome string }` (`CodexHome` is the hook's `CODEX_HOME` or `$HOME/.codex`, symlink-resolved) and `attribution` is the type in data-model.md, following contract section 2 steps 3 to 5: live cached client mapping (same pid, start time, still interactive, same pane and session in its env), else discovery with exactly-one, ambiguous, no-client and client-not-in-zellij outcomes; inspection errors never fall back to the inherited pane
- [x] T015 [US1] Wire into `runHook` in `cc-deck/internal/cmd/hook.go`: add `var hookProcSource procinfo.Source = procinfo.Default()` as the test seam; for `--agent codex`, compute `daemonHosted(hookProcSource)` before the `not-in-zellij` exit; when daemon-hosted, skip that exit, ignore `--pane-id`, call `resolveCodex`, record the mapping on `discovered` (T009), refresh it on cache hits, and drop the event on the non-delivering reasons; never call `recordPaneMapping` with the inherited pane (FR-002)
- [x] T016 [US1] Use the client's profile for daemon-hosted events in `cc-deck/internal/cmd/hook.go`: pass the client's `CC_DECK_PROFILE`, `CC_DECK_PROFILE_COLOR`, `CC_DECK_PROFILE_ICON` (from the client env, or the profile name stored in the mapping on cache hits) to `enrichProfile` instead of `os.Getenv` (FR-017, research R10)

**Checkpoint**: contract tests 1, 5 and 6 pass; quickstart section 1 passes on a real machine.

---

## Phase 4: User Story 2 - Several Codex sessions share one daemon (Priority: P1)

**Goal**: Each of several clients gets only its own sessions; a shared directory drops new sessions instead of guessing.

**Independent Test**: quickstart sections 2 and 3, contract test 2.

- [x] T017 [P] [US2] Add contract test 2 and multi-client cases to `cc-deck/internal/cmd/codex_attribution_test.go`: three clients in distinct directories each receive their own sessions; one client owning several sessions (`/new`) keeps all of them in its pane; two clients in one directory with one already mapped drop a new session as `ambiguous` while the mapped session keeps resolving through `from-client-cache`; after one of the two exits, the next event resolves as `discovered`; a cache hit performs no `ListUser` call on the fake source (FR-008)
- [x] T018 [US2] Ensure `resolveCodex` in `cc-deck/internal/cmd/codex_attribution.go` treats two or more matching clients as `ambiguous` regardless of existing mappings, and that a client may own several mappings (FR-005 to FR-007); adjust only if T017 exposes a gap

**Checkpoint**: contract test 2 passes; quickstart sections 2 and 3 pass.

---

## Phase 5: User Story 3 - Codex clients outside this Zellij session stay out (Priority: P2)

**Goal**: Events go to the originating client's own Zellij session; clients outside Zellij are dropped; an explicit client identity wins.

**Independent Test**: quickstart section 4, contract tests 3, 4 and 7.

- [x] T019 [P] [US3] Add contract tests 3 (client in another Zellij session: delivered with that session), 4 (single client outside Zellij: dropped as `client-not-in-zellij`, also when the daemon itself has no Zellij env) and 7 (`client_env.ZELLIJ_PANE_ID` wins over cache and discovery) to `cc-deck/internal/cmd/codex_attribution_test.go`, asserting the session argument the fake `sendHookPayload` receives
- [x] T020 [US3] Change `sendHookPayload` in `cc-deck/internal/cmd/hook.go` to `func(zellijPath, targetSession string, payload []byte)`: a non-empty `targetSession` runs `zellij --session <name> pipe --name cc-deck:hook -- <payload>`, an empty one runs today's argv unchanged; non-daemon events pass `""` (Claude Code, OpenCode and non-daemon Codex keep the exact current command, FR-013), daemon-hosted Codex events pass the attributed session; update only the shared test double in `cc-deck/internal/cmd/hook_test.go` to record the session, leaving every existing test case unchanged (research R7, FR-010, SC-007)
- [x] T021 [US3] Parse an optional `client_env` object in the Codex payload in `cc-deck/internal/agent/codex.go` and expose it as `func CodexClientEnv(input []byte) map[string]string` (nil when absent or malformed), so `NormalizedPayload`'s JSON on the pipe is unchanged; implement contract step 1 as `clientEnvTarget(clientEnv map[string]string, daemonHosted bool, hookSession string) (attribution, bool)` in `cc-deck/internal/cmd/codex_attribution.go` and call it in `runHook` for every Codex event before the daemon branch: a parsable `ZELLIJ_PANE_ID` wins; the target session is `client_env.ZELLIJ_SESSION_NAME`, else `hookSession` for non-daemon hooks only, while a daemon-hosted hook without that session continues with the cache and discovery steps (research R9, FR-011, contract section 2)

**Checkpoint**: contract tests 3, 4 and 7 pass; quickstart section 4 passes.

---

## Phase 6: User Story 4 - Diagnosing attribution (Priority: P3)

**Goal**: The hook debug log states how each Codex event was attributed or why it was dropped.

**Independent Test**: enable `hook-debug`, reproduce each path, check one line per event with the reason.

- [x] T022 [US4] Extend `logHookEnv` (or add `logCodexAttribution`) in `cc-deck/internal/cmd/hook_debug.go` to write `event=`, `session=`, `resolution=`, `pane=`, `target_session=` plus the existing env fields for Codex events, using the reason values from data-model.md; call it from every Codex branch in `runHook`
- [x] T023 [P] [US4] Add a test in `cc-deck/internal/cmd/codex_attribution_test.go` that enables hook debugging in a temp state dir and asserts one log line per path (`from-arg`, `client-env`, `from-client-cache`, `discovered`, `ambiguous`, `no-client`, `client-not-in-zellij`)

**Checkpoint**: all contract tests pass; `make test` and `make lint` pass.

---

## Phase 7: Polish & Cross-Cutting Concerns

- [x] T024 [P] Add `BenchmarkCodexAttribution` in `cc-deck/internal/cmd/codex_attribution_test.go` with sub-benchmarks: the real `procinfo.Default()` cost of `daemonHosted` plus one `Get` liveness check (cache-hit path, compared against a baseline sub-benchmark of today's non-daemon pane-map lookup for the SC-004 delta), the real `ListUser` on the host, and `resolveCodex` discovery over a fake table of 10 clients (SC-005 is the real `ListUser` cost plus the fake-table time); record the results in `specs/089-codex-daemon-pane-attribution/VERIFY.md` against SC-004 (at most 20 ms added) and SC-005 (at most 500 ms)
- [x] T025 [P] Document the Codex shared daemon in `docs/modules/using/pages/multi-agent.adoc`: why hooks run in the daemon, how cc-deck attributes events (single matching client by working directory and Codex home, cached per session), that new sessions in a directory shared by several Codex clients stay hidden, and the alternatives (distinct directories, `codex --no-daemon`); prose plugin with the `cc-deck` voice, one sentence per line
- [x] T026 [P] Update the `hook` section of `docs/modules/reference/pages/cli.adoc` with Codex daemon attribution, delivery to the client's Zellij session, and the new debug-log resolution values
- [x] T027 [P] Update `docs/modules/reference/pages/configuration.adoc`: new optional `pane-map.json` fields (`client_pid`, `client_start`, `profile`) and the `pane-map.lock` file in the state-file table
- [x] T028 [P] Add one sentence on Codex shared-daemon support to the agent support part of `README.md`
- [x] T029 Run `/prose:check` on the changed AsciiDoc pages and the README sentence and fix findings
- [ ] T030 Run `make verify`, then quickstart sections 1 to 6 on a real machine with the shared daemon; record results in `specs/089-codex-daemon-pane-attribution/VERIFY.md`

---

## Dependencies & Execution Order

- **Phase 1**: T002 first (the former T001 go.mod step is part of T003).
- **Phase 2**: T003 after T002 and uses T004; T004, T005, T006 parallel after T002; T007 after T003 to T006; T008 then T009 (same file); T010 after T009.
- **US1 (Phase 3)**: after Phase 2. T011 first (tests before code); T012 then T013 then T014 (same file); T015 and T016 after T014 (both in `hook.go`, sequential).
- **US2 (Phase 4)**: after T014; T017 then T018.
- **US3 (Phase 5)**: after T015; T019 first; then T020, then T021 (both change `runHook` in `hook.go`, so they are sequential).
- **US4 (Phase 6)**: after T015 and T020; T022 then T023.
- **Polish (Phase 7)**: after all stories; T025 to T028 parallel; T029 after them; T030 last.

### Parallel Opportunities

- T004, T005, T006 (different platform files) after T002
- T007 and T010 test files in parallel once their code exists
- T011, T017, T019 test additions are in the same file and therefore sequential, but each can be written while the corresponding implementation task in another file proceeds
- Documentation tasks T025 to T028

## Parallel Example: Phase 2

```bash
Task: "Implement the procargs2 parser in cc-deck/internal/procinfo/procargs.go"
Task: "Implement the Linux source in cc-deck/internal/procinfo/procinfo_linux.go"
Task: "Add cc-deck/internal/procinfo/procinfo_other.go returning ErrUnsupported"
```

## Implementation Strategy

### MVP (User Story 1)

1. Phases 1 and 2 (process inspection, safe cache).
2. Phase 3: the reported defect is fixed for the single-client case.
3. Validate with quickstart section 1.

### Incremental Delivery

1. US2 (several clients, strict same-directory rule).
2. US3 (session targeting, explicit client identity).
3. US4 (diagnostics).
4. Phase 7 documentation and verification in the same branch (constitution I).

## Notes

- Never run `go build` or `cargo build` directly; use `make test`, `make lint`, `make install`.
- Use `internal/xdg` for every state path.
- The hook must stay silent on every error and must never block Codex for more than the 2 s lock deadline.
- Commit after each task or logical group with the required attribution trailers.
