# Code Review: 087-harness-profiles

## Spec Compliance

**Score: 100% (27/27 FRs)**

All functional requirements verified against the implementation:

| FR | Status | Notes |
|----|--------|-------|
| FR-001 profiles map in config.yaml | PASS | config.Profile struct, validation |
| FR-002 harness field | PASS | HarnessName() with default detection |
| FR-003 backend field | PASS | EffectiveBackend() with defaults |
| FR-004 model field | PASS | Model in ResolvedProfile |
| FR-005 auth block | PASS | EffectiveAuth(), login, api-key, credentials |
| FR-006 env map | PASS | Env in ResolvedProfile, sorted export |
| FR-007 profile sync | PASS | Sync() with SyncResult |
| FR-008 wrapper scripts | PASS | renderWrapper, cc-deck marker, shebang |
| FR-009 per-profile config dir | PASS | PrepareConfigDir, symlinks, isolated entries |
| FR-010 remote provisioning | PASS | Provision(), prepare.sh, stale removal |
| FR-011 stale cleanup | PASS | removeStaleWrappers, marker check |
| FR-012 no bare binary names | PASS | Guard in Sync and Provision |
| FR-013 shell rc PATH | PASS | shellrc.EnsureAll, marker blocks |
| FR-014 credential transport | PASS | env-check, file-check blocks in wrapper |
| FR-015 file credential copy | PASS | copyFileCredentials into CredDir |
| FR-016 Vertex backend | PASS | Claude translator, project/region env |
| FR-017 OpenAI backend | PASS | Codex translator, OPENAI_API_KEY |
| FR-018 Anthropic backend | PASS | Claude/OpenCode translators |
| FR-019 login mode | PASS | Claude SupportsLogin, codex/opencode reject |
| FR-020 deterministic render | PASS | Contract test, sorted env keys |
| FR-021 idempotent PrepareConfigDir | PASS | Contract test (twice succeeds) |
| FR-022 profile color | PASS | Deterministic hash, 8-color palette |
| FR-023 sidebar identity | PASS | HookPayload profile/profile_color fields |
| FR-024 snapshot restore | PASS | CC_DECK_PROFILE detection |
| FR-025 CLI subcommands | PASS | profile sync, profile list |
| FR-026 hook integration | PASS | Profile-aware hook dispatch |
| FR-027 10 profiles < 2s | PASS | Performance test |

## Deep Review Report

### Review Agents

Five specialized review agents executed in parallel:

1. **Correctness Agent**: Focused on logic errors, edge cases, data flow
2. **Architecture Agent**: Evaluated interface design, coupling, extensibility
3. **Security Agent**: Audited credential handling, file permissions, injection risks
4. **Production Agent**: Checked error handling, logging, resource management
5. **Tests Agent**: Assessed test coverage, assertion quality, edge case gaps

### Findings Summary

| Severity | Count | Fixed | Deferred |
|----------|-------|-------|----------|
| Critical | 2 | 2 | 0 |
| Important | 7 | 7 | 0 |
| Notable | 3 | 0 | 3 |

### Critical Findings (all fixed)

1. **Shell RC atomic writes** (Security/Production): `os.WriteFile` in shellrc/block.go could leave truncated files on crash. Fixed with temp-file + rename pattern (`atomicWriteFile`).

2. **Contract test env var ordering** (Tests): `t.Setenv("CONTRACT_TEST_KEY", ...)` was called AFTER `tr.Render(rp)`, so the "no credential value in wrapper" assertion tested against an empty string. Fixed by moving `t.Setenv` before `Render`.

### Important Findings (all fixed)

3. **PrepareConfigDir warnings lost** (Architecture): `PrepareConfigDir` returned only `error`; warnings from `PrepareSharedDir` were silently discarded. Changed signature to `(warnings []string, err error)` and propagated through `SyncResult.Warnings`.

4. **Login rejection missing for codex/opencode** (Correctness): Contract 2.2 requires translators that do not support login to reject `rp.Login=true`. Added explicit error return in codex and opencode `Render` methods, with tests.

5. **Stale wrapper removal missing from Provision** (Correctness/FR-010): `Provision` uploaded wrappers but never cleaned stale ones on the remote. Added stale-removal loop to `prepare.sh` that checks for the cc-deck marker on line 2. Added `TestProvision_StaleWrapperRemoval`.

6. **Duplicated Resolve/resolveRemote** (Architecture): `Resolve` and `resolveRemote` shared nearly identical logic. Extracted a shared `resolve()` helper parameterized on `dataHome`/`configHome`, removed `resolveRemote`.

7. **Harness config subdir hardcoded in switch** (Architecture/SC-007): `defaultConfigSubdir()` used a switch on harness name. Moved this knowledge to a new `DefaultConfigSubdir() string` method on the `Translator` interface. Removed the standalone function.

8. **Dead code** (Architecture): Removed unused `registerPluginInConfig` in agent/opencode.go, unused `isolated` map and `checks` slice in Provision loop, and the now-replaced `resolveRemote` and `defaultConfigSubdir` functions.

9. **Weak symlink assertions in contract test** (Tests): Assertion only checked that link "contains" the string `shared.json`. Strengthened to resolve the symlink target via `filepath.EvalSymlinks` and compare to the actual default dir entry path.

### Notable Findings (deferred, not blocking)

10. **`harnessDefaultBackend` remains in config package** (SC-007 partial): Listed for migration to the Translator interface, but moving it would create a circular dependency (config cannot import profile). Kept in config package; the function is small and stable.

11. **opencode.json written with os.WriteFile, not atomic** (Production): The opencode translator writes `opencode.json` directly. This is lower risk than shell RC files (crash during profile sync is recoverable by re-running sync) and does not warrant the added complexity of atomic writes.

12. **Provision stale removal depends on shell sed** (Production): The remote stale-cleanup uses `sed -n '2p'` which is POSIX but could differ on exotic systems. Accepted as reasonable given the target environments (Linux containers, macOS).

### External Tool Reviews

| Tool | Status | Findings |
|------|--------|----------|
| CodeRabbit CLI | Complete (87 files) | 3 findings (1 major, 2 minor); see below |
| Codex CLI | Usage limit | `gpt-5.6-terra` quota exceeded; non-blocking |

**CodeRabbit findings**:

1. **(major) contract_test.go**: `t.Setenv` should be called before `Render`. Already fixed in hardening commit `bc4cc09`.
2. **(minor) tasks.md**: Stale compilation references. Already resolved by fix commit.
3. **(minor) harness-profiles.adoc**: Sample output does not match actual warning format from `runProfileSyncWorkspace`. Non-blocking doc polish, not a code defect.

### Fix Commit

```
bc4cc09 fix(profile): deep-review hardening round
```

9 files changed, 186 insertions(+), 126 deletions(-)

### make verify

All tests pass, all linters clean:
- Go: 25 packages, all OK (profile package 0.8s fresh)
- Rust: 436 plugin tests + 13 protocol tests, all passed
- Clippy: clean
- golangci-lint: clean

### Gate Outcome

**PASS**: All Critical and Important findings fixed. 3 Notable findings deferred (none blocking). 100% spec compliance. `make verify` green.
