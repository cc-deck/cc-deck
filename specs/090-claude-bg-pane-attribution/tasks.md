# Tasks: Claude Code Background Session Pane Attribution

**Input**: Design documents from `/specs/090-claude-bg-pane-attribution/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/claude-bg-attribution.md, quickstart.md

**Tests**: Included. Constitution principle I requires unit tests for new code; the contract (section 8) lists 22 contract tests, referenced below as "contract N".

**Organization**: Grouped by user story. Paths are repository-relative; Go code lives under `cc-deck/`, the plugin under `cc-zellij-plugin/`. Build and test only via `make test`, `make lint`, `make install` (constitution III). Never run `go build` or `cargo build` directly.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies on incomplete tasks)
- **[Story]**: US1 (parked session keeps its row), US2 (attach shows the job in the attaching pane), US3 (unviewed or ambiguous jobs never land in a wrong row), US4 (diagnosing background attribution)

## Shared definitions

These names are used across tasks; define them where the first task says.

- `claudeRecord`, `claudeHost` outcomes, `bgWorker`, `claudeViewer`, `bgAttribution`, reasons `viewer-env`, `parked-viewer`, `attach-viewer`, `headless`, `several-viewers`, `viewer-not-in-zellij`, `record-missing`, and `bgJobPane`: exactly as in `specs/090-claude-bg-pane-attribution/data-model.md`.
- Test seams: the existing `hookProcSource` (procinfo fake) and `sendHookPayload` (delivery double) in `cc-deck/internal/cmd/hook.go`. Records are read from `<config dir>/sessions`, so tests point `CLAUDE_CONFIG_DIR` and `HOME` at `t.TempDir()` directories (no extra seam). `bgJobPaneFile` is a package variable like `paneMapFile`, redirected by tests.

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Extend the process inspection interface that every story uses.

- [X] T001 Extend `cc-deck/internal/procinfo/procinfo.go`: add `StartUnix int64` (start time in seconds since the epoch) and `Comm string` (kernel process name) to `Process`, add `ListUserMatching(match func(comm string) bool) ([]Process, error)` to `Source` with a doc comment stating the spec 089 rule that a matching process which still runs but cannot be read fails the whole listing, and document that `ListUser()` is `ListUserMatching` with the `codex` filter (data-model.md "procinfo.Process (extended)")
- [X] T002 [P] Update the fake source `fakeProcs` in `cc-deck/internal/cmd/codex_attribution_test.go` to implement `ListUserMatching` (filter its table by `Process.Comm`, falling back to `Basename()` when `Comm` is empty so existing Codex fixtures keep working) and keep `ListUser` behavior identical; all existing Codex tests must pass unchanged

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Platform sources, payload field, state-file helpers, session records, host detection, and the `runHook` branch. Every story depends on these.

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [X] T003 Implement in `cc-deck/internal/procinfo/procinfo_darwin.go`: fill `Comm` from `P_comm` and `StartUnix` from `P_starttime.Sec` in `fromKinfo` and in `Ancestor`; add `ListUserMatching` with the loop of today's `ListUser` but calling `match(comm(kp))` instead of comparing with `CodexComm`; make `ListUser` return `ListUserMatching(func(c string) bool { return c == CodexComm })` (research R3, R4)
- [X] T004 [P] Implement in `cc-deck/internal/procinfo/procinfo_linux.go`: read `btime` once per source from `<root>/stat` (line `btime <seconds>`), set `StartUnix = btime + starttime/100` (USER_HZ, research R3) and `Comm` from the `stat` comm in `details` and `Ancestor`; add `ListUserMatching` with today's `ListUser` loop calling `match(st.comm)`; `ListUser` delegates as on Darwin; a missing or unparsable `btime` leaves `StartUnix` zero (which never verifies a record)
- [X] T005 [P] Add `ListUserMatching` returning `ErrUnsupported` to `cc-deck/internal/procinfo/procinfo_other.go`
- [X] T006 [P] Add tests in `cc-deck/internal/procinfo/procinfo_linux_test.go` (fixture `stat` with `btime`, expected `StartUnix`; missing `btime` gives zero; `ListUserMatching` with a predicate selecting `2.1.302` and `claude` comms but not `codex`) and a cross-platform smoke test in a new `cc-deck/internal/procinfo/procinfo_test.go` that `Default().Get(Default().Self())` has a nonempty `Comm` and a `StartUnix` within 120 s before `time.Now()` on darwin and linux
- [X] T007 [P] Add `Background bool` with tag `json:"background,omitempty"` to `NormalizedPayload` in `cc-deck/internal/agent/agent.go`, and a test in `cc-deck/internal/agent/claude_test.go` that a translated Claude payload marshals without a `background` key (SC-005)
- [X] T008 [P] Add `ClaudeViewerEnv(input []byte) map[string]string` to `cc-deck/internal/agent/claude.go`, mirroring `CodexClientEnv` in `cc-deck/internal/agent/codex.go`: return the payload's `viewer_env` object, else its `client_env` object, keeping only string values; nil on absence or malformed input (research R6); tests in `cc-deck/internal/agent/claude_test.go` for both spellings, precedence of `viewer_env`, non-string values dropped, malformed JSON
- [X] T009 Generalize the state-file helpers in `cc-deck/internal/cmd/hook.go` without changing pane-map behavior: extract `saveStateFileAtomic(path string, v any)` (temp file in the same directory, `Sync`, `Rename`, from today's `savePaneMap`) and `withStateLock(lockPath string, fn func()) bool` (from today's `withPaneMapLock`, same 2 s timeout and 10 ms poll); `savePaneMap` and `withPaneMapLock` become thin wrappers; `cc-deck/internal/cmd/hook_panemap_concurrency_test.go` and `cc-deck/internal/cmd/hook_test.go` pass unchanged
- [X] T010 Create `cc-deck/internal/cmd/claude_bg_attribution.go` with the file comment (link to `specs/090-claude-bg-pane-attribution/contracts/claude-bg-attribution.md`), the reason constants, `claudeRecord`, `parseProcStart(s string) (int64, bool)` (collapse repeated spaces, parse as UTC with layout `Mon Jan _2 15:04:05 2006`), `readClaudeRecord(dir string, pid int) (claudeRecord, error)` (reject a `pid` field that differs from the file name, `jobId`/`parkedJobId` validated against `^[0-9a-f]{8}$`, unknown `kind` values ignored), and `verifyRecord(r claudeRecord, p procinfo.Process) bool` (`|procStart - p.StartUnix| <= 1`, false when either is zero) (research R2, data-model `claudeRecord`); add table tests in a new `cc-deck/internal/cmd/claude_bg_attribution_test.go` with record fixtures copied from research R1 (interactive, bg with `jobId`, interactive with `parkedJobId`, missing `procStart`, mismatched pid, single-digit day `"Sun Oct  4 07:12:37 2026"`)
- [X] T011 Implement the ancestor walk `claudeHostOf(src procinfo.Source, recordsDir string, bgHint bool) (host claudeHost, worker claudeRecord, workerProc procinfo.Process, detail string)` in `cc-deck/internal/cmd/claude_bg_attribution.go` per contract section 2: at most `daemonAncestorDepthLimit` levels via `src.Ancestor`, stop at pid 1 or `procinfo.ErrForeign` (foreground), first verified record decides (`interactive` foreground, `bg` background), a `--bg-pty-host` argument without a verified record means background with an empty worker, inspection errors give foreground unless `bgHint` (then background with detail `host-unknown`); `recordsDir` is `<CLAUDE_CONFIG_DIR or $HOME/.claude>/sessions` of the hook environment; add tests for contract 1 (TUI ancestor, foreground), contract 12 (walk error with and without hint), a nested foreground TUI below a worker (foreground), a stale record whose start differs by 5 s (ignored, walk continues), a PTY host spelled `claude bg-pty-host --bg-pty-host ...` and `.../MacOS/claude --bg-pty-host ...`, a PTY host adopted by a newer daemon (its parent is pid 1; the worker is still found), and a hook-spawned helper running three levels below the worker (`worker → sh → helper → sh → cc-deck`, still background with the worker's record)
- [X] T012 Wire the branch into `runHook` in `cc-deck/internal/cmd/hook.go`: for `agentName == "claude"`, before profile enrichment and the `not-in-zellij` exit, call `claudeHostOf(hookProcSource, recordsDir, os.Getenv("CLAUDE_BG_BACKEND") != "")`; foreground continues on today's path byte for byte; background calls a new `runClaudeBackground(zellijPath string, input []byte, normalized *agent.NormalizedPayload, paneIDStr string, worker claudeRecord, workerProc procinfo.Process, detail string)` in `cc-deck/internal/cmd/claude_bg_attribution.go`, which in this task only drops the event (reason `record-missing`) and returns; extend `cc-deck/internal/cmd/claude_bg_attribution_test.go` with contract 1: with a TUI ancestor in the fake source, the existing hook test table produces identical `sendHookPayload` calls and `pane-map.json` content, and the Codex and OpenCode paths never call `claudeHostOf`

**Checkpoint**: `make test` passes; foreground Claude Code, Codex and OpenCode behavior unchanged; background events are dropped instead of reaching the `not-in-zellij` exit.

---

## Phase 3: User Story 1 - A session moved to the background keeps its row (Priority: P1) 🎯 MVP

**Goal**: Events of a parked job reach the pane of the TUI that parked it, in that TUI's Zellij session and profile, and the row shows the `⇣` marker until the next foreground event.

**Independent Test**: quickstart Q1; contract tests 2, 10, 11, 13, 16, 17, 19, 20, 22.

### Tests for User Story 1

- [X] T013 [P] [US1] Add Go contract tests to `cc-deck/internal/cmd/claude_bg_attribution_test.go` using a scenario helper (worker, PTY host, `sh`, hook process in the fake source; records in a temp `sessions` directory; `CLAUDE_CONFIG_DIR` set): contract 2 (parked viewer in pane A of session S, worker inherits pane B with `ZELLIJ=0`: delivered to A with `--session S` and `"background":true`, B never appears, `pane-map.json` unchanged; repeated with a worker and hook environment that carry no `ZELLIJ*` variables at all, same delivery), contract 10 (worker or parked record whose `procStart` is off by more than 1 s: `record-missing` or not a viewer), contract 11 (worker record without `jobId`, malformed record JSON, unreadable worker in `Get`: `record-missing`, nothing sent), contract 13 (`viewer_env` with valid pane and session wins over records; invalid pane id or session name falls through to records), contract 16 (background `SessionEnd`, delivered and dropped: the session's `pane-map.json` entry is removed), contract 17 (profile fields from the viewer's `CC_DECK_PROFILE*`, not the hook's; invalid profile name dropped); these tests fail until T014 to T018 are done

### Implementation for User Story 1

- [X] T014 [US1] Implement worker resolution in `cc-deck/internal/cmd/claude_bg_attribution.go`: `workerOf(src procinfo.Source, rec claudeRecord, p procinfo.Process) (bgWorker, error)` reads the worker environment with `src.Get`, takes `CLAUDE_CONFIG_DIR` else `$HOME/.claude` (with `os.UserHomeDir` standing in for a missing `HOME`), symlink-resolved with `resolvePath`; a `Get` error or a start time differing from `p.StartTime` is an error (reason `record-missing`)
- [X] T015 [US1] Implement parked viewer collection in `cc-deck/internal/cmd/claude_bg_attribution.go`: `parkedViewers(src procinfo.Source, w bgWorker) ([]claudeViewer, error)` reads every `<w.configDir>/sessions/<pid>.json` (a directory read error is an error; a file that vanished meanwhile is skipped), keeps `kind: "interactive"` records with `parkedJobId == w.jobID`, verifies each with `src.Get` plus `verifyRecord`, skips viewers whose own environment resolves to another configuration directory, and fills pane, Zellij session (outside Zellij when `ZELLIJ` is unset, the pane id does not parse, or `validZellijSessionName` fails) and `profileEnv` (data-model `claudeViewer`); a live verified viewer whose environment cannot be read is an error
- [X] T016 [US1] Implement `resolveClaudeBackground(w bgWorker, viewers func() ([]claudeViewer, error)) bgAttribution` in `cc-deck/internal/cmd/claude_bg_attribution.go` per contract section 3 steps 3 and 4 (step 1 runs in T017, step 2 in T014): viewer collection (any error: `record-missing` with the error text as detail); grouping into terminals by `(zellijSession, paneID)` with each outside-Zellij viewer its own terminal; 0 terminals `headless`, 2 or more `several-viewers`, one outside Zellij `viewer-not-in-zellij`, one inside Zellij `parked-viewer` if a parked viewer is in it else `attach-viewer`, profile from the parked viewer when both kinds share the terminal; always fill `bgAttribution.Terminals` (data-model `bgAttribution`) with the in-Zellij terminals found, which T025 needs for the `several-viewers` transitions; the `viewers` callback lets US2 add attach viewers without changing this function
- [X] T017 [US1] Complete `runClaudeBackground` in `cc-deck/internal/cmd/claude_bg_attribution.go`: first apply contract step 1 through `clientEnvTarget(agent.ClaudeViewerEnv(input), true, "")` (reason `viewer-env`, delivered even when the worker record is empty; the job id is filled when the worker record is known); otherwise an empty worker record gives `record-missing`; otherwise build the worker (T014), resolve (T016) with `parkedViewers` as the callback, and for delivering reasons set `normalized.Background = true`, `normalized.PaneID`, enrich the profile through `enrichProfileFrom` only for a name passing `config.ValidProfileName`, and call `deliverHookPayload(zellijPath, att.ZellijSession, normalized, false)` (auto-save never runs for background events, contract section 4); never call `recordPaneMapping` or read `pane-map.json`; for a `SessionEnd` that is dropped, call `removePaneMapping(normalized.SessionID)` (delivered ones already do it in `deliverHookPayload`)
- [X] T018 [US1] Add `logClaudeBackground(event, sessionID, jobID, resolution, detail, pane, targetSession, paneIDArg string)` to `cc-deck/internal/cmd/hook_debug.go` writing the line from contract section 7 (`background=true`, `job=`, `CLAUDE_BG_BACKEND=`), with `stripControl` on event, session and job, and call it once per background event from `runClaudeBackground`, including the `record-missing` drop from T012
- [X] T019 [P] [US1] Add `background: bool` with `#[serde(default)]` to `HookPayload` in `cc-zellij-plugin/src/pipe_handler.rs`, update every `HookPayload { ... }` literal in `cc-zellij-plugin/src/pipe_handler.rs`, `cc-zellij-plugin/src/controller/hooks.rs` (including `make_hook` and `make_agent_hook`) and `cc-zellij-plugin/src/controller/integration_tests.rs` with `background: false`, and add parse tests next to `test_parse_hook_payload_minimal` for payloads with `"background": true` and without the field (contract 19)
- [X] T020 [US1] Add `background: bool` with `#[serde(default)]` to `Session` in `cc-zellij-plugin/src/session.rs` (initialized `false` in `Session::new`) and set `session.background = hook.background` in `cc-zellij-plugin/src/controller/hooks.rs` for every processed hook event, including the paused-row early return and newly created rows; add tests in `cc-zellij-plugin/src/controller/hooks.rs`: background event sets the flag, the next event without it clears it, a paused row follows, and a session replacement (new `session_id`) with `background: true` keeps a manual rename (FR-009, contract 20); extend `test_session_profile_fields_round_trip` style coverage so a serialized session without the field deserializes with `false`
- [X] T021 [US1] Add `background: bool` with `#[serde(default)]` to `RenderSession` in `cc-zellij-plugin/src/lib.rs`, map it in `build_render_payload` in `cc-zellij-plugin/src/controller/render_broadcast.rs`, update every `RenderSession { ... }` literal (`cc-zellij-plugin/src/lib.rs`, `cc-zellij-plugin/src/sidebar_plugin/render.rs` tests, `cc-zellij-plugin/src/controller/render_broadcast.rs`, `make_session` in `cc-zellij-plugin/src/sidebar_plugin/test_helpers.rs`) with `background: false`, and add a render-payload test that a background session maps to `background: true`
- [X] T022 [US1] Draw the marker in `session_entry_lines` in `cc-zellij-plugin/src/sidebar_plugin/render.rs`: when `session.background`, insert `\x1b[2m⇣\x1b[0m ` (U+21E3 plus a space, dim style) after the agent prefix and before the name, add 2 to the name budget `prefix_len`, leave the edge bar, indicator and line 2 untouched; add a `⇣ background job` entry to the help overlay legend in `render_help_overlay`/`legend_line`; tests next to `test_edge_bar_on_both_lines_in_every_state` using `visible()`: marker present for background rows, absent otherwise, edge bar and indicator at the same columns, long names truncated two columns earlier; existing render tests pass unmodified (contract 22)

**Checkpoint**: contract tests 1, 2, 10, 11, 13, 16, 17, 19, 20, 22 pass; quickstart Q1 passes on a real machine (or documents the `parkedJobId` outcome, research R2).

---

## Phase 4: User Story 2 - Attaching to a job shows it in the attaching pane (Priority: P1)

**Goal**: `claude attach <jobId>` viewers receive the job's events, and the former viewer's row is released when the viewer moves or leaves.

**Independent Test**: quickstart Q2; contract tests 3, 14, 15, 21.

### Tests for User Story 2

- [X] T023 [P] [US2] Add Go contract tests to `cc-deck/internal/cmd/claude_bg_attribution_test.go`: attach viewers by job ID/session ID, native/npm forms, non-matching jobs, unreadable candidates, movement and headless release (contracts 3, 14, 15); deep-review regressions retain ownership after overnight idle, skip delivery on lock/write failure, and verify pending-release retry, revalidation, and bounded backoff

### Implementation for User Story 2

- [X] T024 [US2] Implement attach viewer discovery in `cc-deck/internal/cmd/claude_bg_attribution.go`: `isClaudeComm(c string) bool` (`claude`, `node`, or version shape `^\d+\.\d+\.\d+([.-].*)?$`), `claudeCLIArgs(p procinfo.Process) ([]string, bool)` (argv[0] basename `claude` or version shape gives `p.Args`; for `node`, `p.Args[0]` ending in `/@anthropic-ai/claude-code/cli.js` gives `p.Args[1:]`), and `attachViewers(src procinfo.Source, w bgWorker) ([]claudeViewer, error)` using `src.ListUserMatching(isClaudeComm)` (listing error is an error) and keeping processes whose first non-flag CLI argument is `attach` and whose next non-flag argument equals `w.jobID` or `w.sessionID`, with the viewer's own configuration directory equal to `w.configDir`; then make `runClaudeBackground` pass a callback that concatenates `parkedViewers` and `attachViewers` (research R4, R5)
- [X] T025 [US2] Implement `bg-job-panes.json` in `cc-deck/internal/cmd/claude_bg_attribution.go` with key `<configDir>|<jobId>`, the data-model fields and pure `jobPaneTransition` table (keep on incomplete discovery, preserve a remembered ambiguous viewer, hourly refresh, SessionEnd cleanup). Deep-review correction: retain ownership without age-only expiry, serialize fresh discovery per job, lock only shared write merges, and persist pending-release intent before sends; table and failure regressions cover each path
- [X] T026 [US2] Send ownership-guarded `cc-deck:release-row` payloads through context-aware `sendHookPayload` and `sendRowRelease(ctx, zellijPath, bgRowRelease)`, preserving foreground argv and timeout. Persist intent before delivery, retain failed removals alongside new ownership, revalidate pending panes on each event, share a 100 ms cleanup deadline with durable one-to-32-second backoff, and never deliver an untracked known job after lock/write failure
- [X] T027 [P] [US2] Add the release pipe to the plugin: parse `cc-deck:release-row` into a new `PipeAction::ReleaseRow { pane_id: u32, session_id: String }` in `cc-zellij-plugin/src/pipe_handler.rs` (malformed payload or empty session id gives `PipeAction::Unknown`), dispatch it in the controller in `cc-zellij-plugin/src/controller/mod.rs` (sidebar instances ignore it), and add `ControllerState::release_row(pane_id, session_id) -> bool` in `cc-zellij-plugin/src/controller/state.rs` that calls `evict_session(pane_id)` only when the row exists and its `session_id` equals the argument, followed by a render broadcast when a row was removed; tests in `cc-zellij-plugin/src/pipe_handler.rs` and `cc-zellij-plugin/src/controller/integration_tests.rs`: matching id evicts (also for a live pane), mismatched id, empty row id, unknown pane, and a row of another agent in the same pane stay untouched (contract 21)

**Checkpoint**: contract tests 3, 14, 15, 21 pass; quickstart Q2 passes.

---

## Phase 5: User Story 3 - Unviewed or ambiguous jobs never land in a wrong row (Priority: P2)

**Goal**: Headless jobs, several viewers, viewers outside Zellij, viewers of other configuration directories, and cross-session setups never update a row that does not show the job.

**Independent Test**: quickstart Q3; contract tests 4 to 9.

The rules are implemented by T015, T016, T024 and T025; this phase adds the guarding tests. Each task is done when its tests pass under `make test`; a failing case is fixed in `cc-deck/internal/cmd/claude_bg_attribution.go` within the same task, never by loosening the test.

- [X] T028 [P] [US3] Add contract tests 4 (no viewer: `headless`, nothing sent, no row created), 5 (attach viewers in panes C and D: `several-viewers`; after D's process disappears from the fake source the next event goes to C), 6 (parked TUI in pane A plus an attach process in pane A of the same session: one terminal, reason `parked-viewer`, profile of the TUI), 7 (single viewer without `ZELLIJ`: `viewer-not-in-zellij`), 8 (viewer in session S2 while the hook environment names S1: delivered with `--session S2`, nothing to S1) to `cc-deck/internal/cmd/claude_bg_attribution_test.go`
- [X] T029 [P] [US3] Add isolation tests to `cc-deck/internal/cmd/claude_bg_attribution_test.go`: contract 9 (a `sessions` directory shared through a symlink between two configuration directories; a parked TUI and an attach process of the other configuration directory naming the same job id are not viewers), a viewer with an invalid `ZELLIJ_SESSION_NAME` (leading `-`, contains `/`) counted as outside Zellij, a viewer that exits between listing and `Get` skipped without error, a viewer pid reused by a non-Claude process (record start mismatch) not counted, and the `several-viewers` transitions of T025 (remembered pane among the viewers keeps the entry; not among them releases once)

**Checkpoint**: contract tests 4 to 9 pass; SC-003 holds for the tested scenarios.

---

## Phase 6: User Story 4 - Diagnosing background attribution (Priority: P3)

**Goal**: Each resolution path writes a distinct, readable debug line, and releases are logged.

**Independent Test**: quickstart Q1 to Q3 with `hook-debug` enabled; tests below.

- [X] T030 [US4] Add `logRowRelease(event, sessionID, jobID string, r bgRowRelease)` to `cc-deck/internal/cmd/hook_debug.go` writing the successful release line of contract section 7 (`release_pane=`, `release_session=`, `release_sid=`, values through `stripControl`); log delivery/state errors separately so failed attempts are not reported as successful removals
- [X] T031 [US4] Add debug-log tests to `cc-deck/internal/cmd/claude_bg_attribution_test.go` using the existing `enableHookDebug` helper: one line per background event for each of the seven reasons with `event=`, `session=`, `job=`, `resolution=`, `pane=` (empty when dropped) and `target_session=`; `detail=` present for `record-missing` with the inspection error; a release produces its own line; a session id with a newline is written with `?`; foreground Claude Code events keep today's `logHookEnv` line without `background=` (FR-014)

**Checkpoint**: FR-014 covered; every quickstart path is distinguishable in `hook.log`.

---

## Phase 7: Polish & Cross-Cutting Concerns

- [X] T032 Add `BenchmarkClaudeBackgroundAttribution` to `cc-deck/internal/cmd/claude_bg_attribution_test.go` with sub-benchmarks: the real `procinfo.Default()` ancestor walk for a foreground hook (foreground overhead), the real `ListUserMatching(isClaudeComm)` on the host, and a full `runClaudeBackground` resolution over a fake table of 30 Claude Code processes and 30 records in a temp directory; record results in `specs/090-claude-bg-pane-attribution/VERIFY.md` against SC-004 (at most 150 ms)
- [X] T033 [P] Add a "Claude Code Background Sessions" section to `docs/modules/using/pages/multi-agent.adoc` after "Codex Shared Daemon": how sessions reach the background (`/background`, `claude --bg`, `claude attach`), the attribution rules (viewer pane, headless jobs hidden, several viewers hidden, viewer outside the workspace hidden, stale row release), the `⇣` marker, the observed `parkedJobId` behavior from quickstart Q1, the workaround of resuming the session in the foreground, diagnosis with `hook-debug`, and a link to https://github.com/anthropics/claude-code/issues/102949; only `cc-deck` and `claude` commands (constitution VI)
- [X] T034 [P] Document in `docs/modules/reference/pages/cli.adoc` under the `hook` command: a "Claude Code background attribution" subsection next to `[#codex-daemon-attribution]` covering detection, resolution order, the seven reasons, the `background` payload field, and the `cc-deck:release-row` pipe
- [X] T035 [P] Document `~/.local/state/cc-deck/bg-job-panes.json` (fields, retained cleanup ownership, pending retries, used only for row release), the short merge lock, and per-job serialization locks in the state file table of `docs/modules/reference/pages/configuration.adoc`, next to `pane-map.json` (deep-review correction replaces age-only expiry)
- [X] T036 [P] Describe the background marker in `docs/modules/using/pages/sidebar.adoc` ("What You See" and "How Sessions Are Tracked"), including that it disappears with the next foreground event
- [X] T037 [P] Add one sentence under Claude Code support in `README.md` stating that background sessions appear in the row of the pane that views them, linking to the multi-agent guide section
- [X] T038 Run `/prose:check` with the `cc-deck` voice on every documentation file touched in T033 to T037, fix findings, and confirm no em-dashes or en-dashes and one sentence per line in the AsciiDoc files
- [X] T039 Run `make lint` and `make test`; both must pass without weakening existing hook and sidebar assertions (SC-005); repair the Compose smoke-test prerequisites, stale CLI expectation, and session-deletion race as authorized by the user on 2026-10-18
- [x] T040 Run `make install`, restart the workspace, and execute quickstart Q1 to Q5 from `specs/090-claude-bg-pane-attribution/quickstart.md`; record outcomes in `specs/090-claude-bg-pane-attribution/VERIFY.md`, and update research R2 and the T033 documentation with the observed `parkedJobId` lifetime

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: none. T002 depends on T001.
- **Foundational (Phase 2)**: depends on Setup. T003 to T006 depend on T001; T010 to T012 depend on T003/T004 (for `StartUnix`) and run in order; T007, T008, T009 are independent of each other.
- **US1 (Phase 3)**: depends on Phase 2. Go tasks T014 → T015 → T016 → T017 → T018 in order (same file); plugin tasks T019 → T020 → T021 → T022 in order (shared literals), independent of the Go tasks.
- **US2 (Phase 4)**: depends on US1's T016 and T017 (resolver and `runClaudeBackground`). T024 → T025 → T026 in order; T027 (plugin) is independent of the Go tasks.
- **US3 (Phase 5)**: depends on US1 and US2 (it tests attach viewers and release transitions).
- **US4 (Phase 6)**: depends on US1 (T018) and US2 (T026).
- **Polish (Phase 7)**: T032 depends on US2; documentation T033 to T037 can start after US2 and are finalized after T040; T038 after T033 to T037; T039 after all code; T040 last.

### User Story Dependencies

- **US1 (P1)**: independent after Phase 2; the MVP.
- **US2 (P1)**: builds on the US1 resolver; independently testable through attach scenarios.
- **US3 (P2)**: verification of rules built in US1 and US2.
- **US4 (P3)**: diagnostics over US1 and US2 paths.

### Within Each User Story

- Tests are written first and fail before the implementation tasks of that story.
- Go and Rust tracks of a story run in parallel; within a track, tasks touching the same file run in order.

## Parallel Example: User Story 1

```text
# Go track and plugin track side by side:
Task: "T013 Go contract tests in cc-deck/internal/cmd/claude_bg_attribution_test.go"
Task: "T019 HookPayload.background in cc-zellij-plugin/src/pipe_handler.rs"

# Foundational tasks in parallel after T001:
Task: "T004 Linux source in cc-deck/internal/procinfo/procinfo_linux.go"
Task: "T007 NormalizedPayload.Background in cc-deck/internal/agent/agent.go"
Task: "T008 ClaudeViewerEnv in cc-deck/internal/agent/claude.go"
Task: "T009 state-file helpers in cc-deck/internal/cmd/hook.go"
```

## Parallel Example: User Story 2

```text
Task: "T023 Go contract tests for attach and release"
Task: "T027 cc-deck:release-row handler in cc-zellij-plugin/src/pipe_handler.rs, controller/mod.rs, controller/state.rs"
```

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Phase 1 and Phase 2: foreground unchanged, background events dropped safely.
2. Phase 3 (US1): parked sessions keep their row with the marker.
3. **STOP and VALIDATE**: quickstart Q1, which also settles the `parkedJobId` risk (research R2). If Claude Code clears `parkedJobId` when the terminal is freed, US2 becomes the main path for viewing jobs; continue with it before shipping.

### Incremental Delivery

1. Setup plus Foundational: no more misattribution risk from the inherited pane.
2. US1: parked sessions (MVP).
3. US2: attach viewers and stale row release.
4. US3: safety tests for every ambiguous case.
5. US4: diagnostics.
6. Polish: benchmark, documentation, quickstart run.

## Notes

- [P] tasks touch different files and have no dependency on incomplete tasks.
- Commit after each task or logical group; documentation lands on this branch, not as a follow-up (constitution I).
- Background events never write `pane-map.json`; any test that sees such a write is a defect, not a test to adjust.
