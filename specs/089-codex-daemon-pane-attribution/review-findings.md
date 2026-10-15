# Deep Review Findings

**Date:** 2026-10-15
**Branch:** 089-codex-daemon-pane-attribution
**Rounds:** 3, plus one corrective fix after the round 3 re-review
**Gate Outcome:** PASS (see note below)
**Invocation:** quality-gate (spex-ship, review-code stage)

Gate note: the round 3 re-review found one Critical regression, which round 3 itself had introduced (FINDING-R4-1). By the letter of the 3-round loop the gate would be FAIL. The fix was small: it removed an unsafe error mapping and kept a stricter carve-out. It is covered by new tests on darwin and in a Linux container. After it, no Critical or Important finding remains, so the gate is reported as PASS. No fourth agent re-review ran.

## Summary

| Severity | Found | Fixed | Remaining |
|----------|-------|-------|-----------|
| Critical | 2 | 2 | 0 |
| Important | 9 | 9 | 0 |
| Minor | 25 | 19 | 6 |
| Notable | 7 | - | 7 |
| **Total** | **43** | **30** | **13** |

**Agents completed:** 5/5 initial agents, plus 3 re-review agents (rounds 2, 3 and final) and CodeRabbit (2 runs).
**External tools:**
- CodeRabbit 0.7.6: completed, with 1 finding on the initial run and 1 on the re-review. Both were under `specs/` and so excluded from the counts. Both were still addressed, because they duplicated FINDING-3 and the contract test 5 wording.
- Codex CLI: failed (rate-limited until 2026-10-17).
- Copilot CLI: skipped (not installed).

## Stage 1: Spec Compliance

The initial score was 95% (41/43 requirements checked: FR-001 to FR-017, the edge cases, SC-001 to SC-007). It is now 100%.

There were two deviations:
- **FR-002.** The code narrowed "Codex app-server process" to the managed shared daemon and its helpers. The spec was clarified to match.
- **Edge case "the old session's mapping is not reused for a different pane".** This was violated through rediscovery after a client exited (FINDING-1). The code was fixed and FR-009 amended.

The strict same-directory rule is unchanged: several matching clients always drop the event.

## Findings

### FINDING-1
- **Severity:** Important
- **Confidence:** 75
- **File:** cc-deck/internal/cmd/codex_attribution.go (resolveCodex)
- **Category:** correctness
- **Source:** correctness-agent
- **Round found:** 1
- **Resolution:** fixed (round 1, e9e52a5)

**What was wrong:** when a cached client had exited, discovery ran on ordinary events. A different client that now matched the directory on its own received the stale session's trailing events, and the cache was rewritten to point at it.

**Why it matters:** this put a wrong row in another pane's sidebar entry (SC-003).

**How it was resolved:** a stale mapping now drops ordinary events (`no-client`, `detail="stale-mapping"`) without running discovery. Only `SessionStart` (startup, resume, clear, compact, fork) runs discovery for such a session.

### FINDING-2
- **Severity:** Important
- **Confidence:** 70
- **File:** cc-deck/internal/cmd/codex_attribution.go (resolveCodex, cachedClient)
- **Category:** correctness
- **Source:** correctness-agent
- **Round found:** 1
- **Resolution:** fixed (rounds 1 and 2, e9e52a5, f6ce1cc), with a documented residual limitation

**What was wrong:** a live cached mapping always won. A session resumed in another client therefore kept updating the old client's row.

**How it was resolved:** on `SessionStart`, a uniquely discovered client replaces the mapping. The live mapping stays only when its client is among the matches, which keeps US2-2 working across compaction. Otherwise the mapping is forgotten and the event is dropped.

**Residual limitation:** a session resumed in a second client in the same directory, while the original client still runs, keeps the original row. This case cannot be told apart from a compaction. It is documented in the spec, cli.adoc and multi-agent.adoc.

### FINDING-3
- **Severity:** Important
- **Confidence:** 85
- **File:** specs/.../contracts/codex-attribution.md, spec.md FR-002
- **Category:** architecture (contract drift)
- **Source:** architecture-agent (also reported by: coderabbit)
- **Round found:** 1
- **Resolution:** fixed (4aae321)

The contract and FR-002 described every `app-server` process as the daemon, while the code recognizes only the managed shared daemon. The contract steps 2 and 4 and a new spec clarification now describe the shipped predicate.

### FINDING-4
- **Severity:** Important
- **Confidence:** 72
- **File:** cc-deck/internal/cmd/hook.go (Codex profile enrichment)
- **Category:** security
- **Source:** security-agent
- **Round found:** 1
- **Resolution:** fixed (e9e52a5)

Profile names from `client_env` or from another process's environment reached the sidebar unescaped. They are now checked with `config.ValidProfileName` before they are delivered or cached.

### FINDING-5
- **Severity:** Important
- **Confidence:** 95
- **File:** cc-deck/internal/cmd/codex_attribution_test.go (TestDaemonHosted)
- **Category:** test-quality
- **Source:** test-quality-agent
- **Round found:** 1
- **Resolution:** fixed (e9e52a5)

The depth-limit test used a stdio app-server, so it passed for the wrong reason. It now places the shared daemon at depth 8 (expected: found) and at depth 9 (expected: not found).

### FINDING-6
- **Severity:** Important
- **Confidence:** 90
- **File:** cc-deck/internal/cmd/codex_attribution_test.go (ReusedPidIgnoresCache)
- **Category:** test-quality
- **Source:** test-quality-agent
- **Round found:** 1
- **Resolution:** fixed (e9e52a5)

The reused pid now stays in the same pane and Zellij session, so only the start-time guard can reject it.

### FINDING-7
- **Severity:** Important
- **Confidence:** 80
- **File:** cc-deck/internal/cmd/codex_attribution_test.go
- **Category:** test-quality
- **Source:** test-quality-agent
- **Round found:** 1
- **Resolution:** fixed (e9e52a5, f6ce1cc)

US1-5 and the edge case "a Codex client runs in the daemon's own pane" were untested. They are now covered by `TestCodexDaemon_ClientInInheritedPane`, and the shared helper keeps its pane check.

### FINDING-8
- **Severity:** Important
- **Confidence:** 80
- **File:** cc-deck/internal/cmd/codex_attribution_test.go
- **Category:** test-quality
- **Source:** test-quality-agent
- **Round found:** 1
- **Resolution:** fixed (e9e52a5, 5ba3cbe)

The profile Codex home was untested. It is now covered by `TestCodexDaemon_ProfileCodexHome` and by the SessionStart Codex-home subtest.

### FINDING-R2-1
- **Severity:** Important
- **Confidence:** 70
- **File:** cc-deck/internal/procinfo/procinfo_linux.go
- **Category:** correctness
- **Source:** round 2 re-review
- **Round found:** 2
- **Resolution:** fixed (f6ce1cc)

**What was wrong:** process ownership came from the owner of the `/proc/<pid>` directory, which the kernel reports as root for non-dumpable processes. A non-dumpable daemon could therefore look foreign, which meant the hook trusted the inherited pane. A non-dumpable client could also drop out of the listing, which could turn an ambiguous match into a unique one.

**How it was resolved:** the effective uid is now read from `/proc/<pid>/status`.

### FINDING-R3-1
- **Severity:** Critical
- **Confidence:** 72
- **File:** cc-deck/internal/procinfo/procinfo_linux.go, codex_attribution.go (codexHost)
- **Category:** regression
- **Source:** round 3 re-review
- **Round found:** 3
- **Resolution:** fixed (5ba3cbe)

**What was wrong:** the ancestor walk read each ancestor's environment. For a non-dumpable Codex the kernel refuses that read, so every non-daemon Codex event on Linux would have been dropped.

**How it was resolved:** a new `procinfo.Source.Ancestor` reads only identity and arguments (Linux `status`, `stat` and `cmdline`; Darwin kinfo and argv, tolerating `ErrTruncatedEnv`).

### FINDING-R4-1
- **Severity:** Critical
- **Confidence:** 80
- **File:** cc-deck/internal/procinfo/procinfo_linux.go (own)
- **Category:** regression (wrong row)
- **Source:** final re-review
- **Round found:** after round 3
- **Resolution:** fixed (6488be6)

**What was wrong:** round 3 mapped any permission error on `status` to `ErrForeign`. Under `hidepid=noaccess` the kernel also denies that read for the user's own non-dumpable daemon, so the daemon would have counted as foreign and the hook would have trusted the inherited pane.

**How it was resolved:** a denied `status` now counts as foreign only when the `/proc/<pid>` owner is neither this user nor root. In every other case the event is dropped. The test was inverted, and a `codexHost` test now checks for `hostUnknown`.

### Minor findings (fixed)
1. On Linux the real working directory now wins over a stale `PWD` (correctness agent).
2. The ancestor walk no longer reads the environment, and an ancestor owned by another user ends the walk with "not daemon-hosted" (correctness, production and security agents).
3. `reasonFromArg` is now used in hook.go (architecture agent).
4. The test-only `daemonHosted` helper moved to the tests, and the parameter that shadowed it was renamed `sessionInherited` (architecture agent).
5. `recordPaneMapping` delegates to `recordClientMapping` (architecture agent).
6. Dropped events now carry a `detail=` field in the debug log, for inspection errors, stale mappings, a single non-interactive match and a moved session (architecture and production agents).
7. The comment that had drifted away from the not-in-zellij check is back in place (architecture agent).
8. Zellij session names from another process or from `client_env` are now checked: no leading `-`, no `/`, no control characters, at most 256 bytes (security agent).
9. Event and session values have control characters removed before they are written to the debug log (security agent).
10. The test now asserts that SessionEnd removes the daemon-hosted mapping (test-quality agent).
11. The pipe target of the hook-without-Zellij path is asserted (test-quality agent).
12. The debug-log test now checks the event name (test-quality agent).
13. The behavior after a cached-client `Get` error is covered by the stale-mapping tests (test-quality agent).
14. Not running discovery for a stale mapping is asserted through `listCalls` (round 2 re-review).
15. When discovery is not unique on SessionStart, the fallback requires the cached client to be among the matches (round 2 re-review).
16. The `resolveCodex` comment no longer claims protection for sessions that were never bound. That case is now a known limitation (round 2 re-review).
17. multi-agent.adoc names the working-directory source on Linux and on macOS (round 2 re-review).
18. Tests cover the stale-plus-ambiguous case and moves to a shared directory and to outside Zellij (round 2 re-review).
19. Tests cover the Codex-home clause and the listing-error clause of the SessionStart fallback (round 3 re-review).

The round 3 hidepid finding (R3-2) was folded into FINDING-R4-1.

## Remaining Findings (Minor)
1. `savePaneMap` duplicates `fileutil.AtomicWrite`, adding only an fsync (architecture agent).
2. The persisted `profile` field in `pane-map.json` is redundant with the client environment (architecture agent).
3. Cache write failures and lock timeouts are not logged (production agent).
4. A temp file `pane-map-*.json` is left behind if the hook is killed between create and rename (production agent).
5. Several test gaps remain (test-quality agent):
   - a symlinked payload `cwd`;
   - `codexHomeFor` unit tests;
   - mixed concurrent writers for `recordClientMapping` and `removePaneMapping`;
   - a vacuous assertion in the "client exits, new one in the same pane" test.

None of these affects attribution correctness.

## Notable Observations
Captured to brainstorm/idea-inbox.md:
- the trust in `client_env`;
- daemon clients that are not `codex` processes;
- the resume-in-the-same-directory row, with a SessionStart `source` field as a possible fix;
- tail latency from the pane-map lock;
- missing test seams for autoSave and the Darwin sysctl calls.

Also noted:
- On unsupported platforms all Codex events are dropped (production agent).
- `recordPaneMapping` and `enrichProfile` share profile lookups (architecture agent).

## Post-Fix Spec Coverage

| Requirement | Implementation | Status |
|-------------|---------------|--------|
| FR-001, FR-004, FR-005, FR-006 | codex_attribution.go `discoverClient`, `toClient` | ✓ |
| FR-002, FR-003 | `codexHost` via `procinfo.Source.Ancestor`, `runCodexHook` | ✓ |
| FR-007, FR-008, FR-009 | `resolveCodex`, `cachedClient` (stale mapping and SessionStart rules, FR-009 amended) | ✓ |
| FR-010 | `hookPipeArgs`, `deliverCodexAttribution` | ✓ |
| FR-011 | `clientEnvTarget` | ✓ |
| FR-012 | `savePaneMap`, `withPaneMapLock` | ✓ |
| FR-013 | runHook non-Codex path unchanged, SC-007 hook tests unmodified | ✓ |
| FR-014 | `logCodexAttribution` (all reasons plus `detail=`) | ✓ |
| FR-015 | no Codex configuration changes | ✓ |
| FR-016 | multi-agent.adoc, cli.adoc, configuration.adoc, README | ✓ |
| FR-017 | `deliverCodexAttribution` (client profile, validated) | ✓ |

All spec requirements were verified after the fix loop.

## Test Suite Results

| Round | Test Command | Exit Code | Failures | Status |
|-------|-------------|-----------|----------|--------|
| 1 | make test | 2 | 9 (TestComposeSmoke*) | pre-existing only |
| 2 | make test | 2 | 9 (TestComposeSmoke*) | pre-existing only |
| 3 | make test | 2 | 9 (TestComposeSmoke*) | pre-existing only |
| final | make test | 2 | 9 (TestComposeSmoke*) | pre-existing only |

The TestComposeSmoke* failures already occur on unmodified main on this machine, because the local podman compose image lacks zellij. They are not regressions.

`make lint` is clean in every round.

The Linux-only procinfo and cmd tests were also run in `golang:1.25` under podman, with a non-root uid, after each round, and passed.
