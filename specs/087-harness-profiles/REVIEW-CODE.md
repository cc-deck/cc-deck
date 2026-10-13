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

## Orchestrator fix round (after Deep Review Report)

The pipeline orchestrator verified the deep-review findings against the code and found these still open after the first hardening round. All are fixed in the commit following this note; `make verify` passes.

| Severity | Finding | Fix |
|----------|---------|-----|
| Critical | Vertex wrapper reused the `_f` shell variable for both file sources, so `ANTHROPIC_API_KEY` read the ADC file | Distinct `_f_apikey` and `_f_creds` variables in the Claude translator; golden and regression tests |
| Important | `ResolveProfile` did not expand `~/` in `file:` sources, so documented paths failed before the copy step | `expandHome` plus a stat check instead of a full read; test |
| Important (security) | Credential `env` names were not validated as shell identifiers before being interpolated into generated shell | `validateCredentialSource` applies `shellIdentRegex`; test |
| Important (security) | `Sync` and `Provision` rendered profiles from a hand-edited config without validation; profile name unquoted in the template | Both loops skip profiles with error-level validation findings (`invalidProfiles`), iterate in sorted order, and the template single-quotes `CC_DECK_PROFILE`; test |
| Important | Codex `config.toml` was written model-only and only when a model was set, dropping user settings and keeping stale models | `overlayCodexModel` carries the default file over, replaces the top-level model, and always rewrites; test |
| Minor | Duplicate `shellQuote` next to `shQuote` | Single `shQuote` |
| Minor | Profile icon changes did not propagate to a running session while color did | Indicator follows the hook like the color |

Deferred (recorded as Notable, not addressed here): loading `config.yaml` on every hook event to resolve profile color and icon (could move to `CC_DECK_PROFILE_COLOR` and `CC_DECK_PROFILE_ICON` exported by the wrapper); the pane-map write race and the OpenShell temp-file cleanup, both pre-existing outside this feature; `WrapperName()` helper unused while three call sites inline the pattern.

---

# Code Review: Revision 2026-10-12 (T067 to T105)

**Date:** 2026-10-12
**Reviewer:** Claude (speckit.spex-gates.review-code)
**Commits:** 207794c, cfd11f3, 8e787c0, 480afcc, d728af4

## Compliance Summary

**Overall Score: 100%** (31 of 31 requirements compliant; one spec wording candidate)

Unchanged requirements (FR-001, FR-003, FR-004, FR-006 to FR-013, FR-015, FR-020 to FR-024) keep their evidence from the 2026-09-09 review; their tests still pass after the revision (`make verify`, exit 0).

| Requirement | Implementation | Test evidence | Status |
|-------------|----------------|---------------|--------|
| FR-002 oauth_token source (env, file) | `config/profile.go` AuthConfig.OAuthToken; `credential/profile.go` | `TestValidateProfiles_OAuthToken`, `TestResolveProfile_OAuthToken*` | Compliant |
| FR-002a CLAUDE_CODE_OAUTH_TOKEN, remote transport | `profile/claude.go` Render; `profile/sync.go` EnvCredentials, Provision upload; `ws/ssh.go` Attach | `TestClaudeRenderGolden_OAuthToken*`, `TestProvision_OAuthTokenFileUploaded`, `TestEnvCredentials_*` | Compliant |
| FR-005 three-way exclusivity, claude-only token | `config/validate.go`, `Profile.Validate()` | `TestValidateProfiles_OAuthToken`, `TestProfileAdd_OAuthTokenForCodexFailsValidation` | Compliant |
| FR-007 CC_DECK_PROFILE_COLOR/ICON exports | `templates/wrapper.sh.tmpl`, `wrapper.go` | contract test 7, goldens | Compliant |
| FR-007a clear other auth kinds, env after clearing, warning | `wrapper.go` unsetLine, translators' AuthVars, `sync.go` envOverrideWarnings | contract test 7, `TestSync_EnvOverrideOfOtherAuthKindWarns`, quickstart bogus-key check | Compliant |
| FR-013 missing credential exit | unchanged check blocks, now also for tokens | goldens, `TestSync_MissingOAuthTokenReportedAsWarning`, quickstart | Compliant |
| FR-014a env-first hook, fallback, derived for unknown | `cmd/hook.go` enrichProfile | `TestRunHook_EnvColor*`, `TestRunHook_InvalidEnvColorFallsBackToConfig`, `TestRunHook_UnknownProfileWithoutEnvColorDerives` | Compliant |
| FR-016 edge bar on both lines, blank column for unprofiled | `sidebar_plugin/render.rs` session_entry_lines | SC-008 render tests | Compliant (see spec candidate 1) |
| FR-016a same color in every state, no highlight on bar | `render.rs` lead/edge_bar, row_highlight | state matrix tests (plain, active, cursor, rename, faded, paused, paused-active) | Compliant |
| FR-017 add picks farthest hue; derived otherwise; no profile color on glyph or status | `cmd/profile.go` runProfileAdd + `profile.Pick`; `render.rs` agent_indicator_color | `TestProfileAdd_PicksColorWhenUnset`, `TestProfileAdd_SequentialColorsAreDistinct`, glyph brand color test | Compliant |
| FR-017a glyph show rule on distinct harnesses | `controller/render_broadcast.rs` | controller integration tests | Compliant |
| FR-017b sync warns on similar colors, never writes config | `profile/sync.go` colorCollisionWarnings | `TestSync_SimilarColorsWarnOnce`, `TestSync_DistinctColorsNoWarningAndConfigUntouched`, quickstart | Compliant |
| FR-018 icon in place of glyph only when glyph shown | controller LegendEntry/indicator handling | FR-018 controller tests | Compliant |
| FR-019 legend with bar swatch, only when profiled sessions run | `render.rs` render_help_overlay, legend_line | sidebar help overlay test | Compliant |
| FR-025 K8s rejects token profile before resources | `ws/k8s_deploy.go` checkK8sProfileSupported | `TestK8sDeployWorkspace_CreateRejectsOAuthTokenProfile`, quickstart | Compliant |
| FR-026 token profile maps to claude provider | `profile/claude.go` ProviderType, `ws/openshell.go` | `TestCreate_ProfileProviders_OAuthTokenProfile` | Compliant |
| SC-008, SC-010, SC-011 | as above | render state tests, hook unreadable-config test, `TestPickSequenceDistinct` | Met |
| SC-009 token profile on fresh remote, no login | SSH env delivery fix, file upload | unit tests; live host check pending | Met in code, live check pending |

## Deviations and Extra Behavior

1. **Fixed during implementation:** SSH attach did not deliver env-sourced profile credentials to `credentials.env` (affected `api_key` and `oauth_token` env profiles alike). `SSHWorkspace.Attach` now merges `profile.EnvCredentials` for harnesses present on the remote.
2. **Cleared-variable source rule** (data-model.md validation table): implemented in `profile.Resolve` rather than `config/validate.go`, because the auth variable tables live with the translators. Effect is the same: the profile is skipped by `sync` with the documented message.
3. **oauth_token harness/backend rule** lives in `Profile.Validate()` so that `add` reports the token problem instead of a misleading "requires auth.api_key" error; `validateProfiles` surfaces it with the same `profile "<name>":` prefix.
4. **Codex glyph mapping** `"codex" => "◆"` added to `agent_name_to_indicator` to match the Go adapter; without it every Codex profile would look like it declared an icon.
5. **Delete-confirm row** carries no edge bar. It is not one of the FR-016a states.

## Spec Evolution Candidates

- [ ] FR-016 still says the column is "reserved" only while a profiled session runs. Research R12 reuses the always-present margin column, so the layout is identical with and without profiles. Reword FR-016 to describe the margin column instead of a conditional reservation.
- [ ] Quickstart section 1 expects two written wrappers; the `home` login profile also gets `claude-home`.

## Conclusion

Revision is spec compliant. Remaining open items are live-environment checks (Zellij visual pass, real subscription token, SSH host, OpenShell gateway).
