# Verification Report: Harness Profiles

**Date:** 2026-09-09
**Spec:** specs/087-harness-profiles/spec.md
**Branch:** 087-harness-profiles

## Closeout Gate

`spex-closeout-gate.sh specs/087-harness-profiles`: `CLOSEOUT_PASS` (no unresolved Critical or Important findings in REVIEW-CODE.md).

## Test Results

**Status:** PASS

`make verify` (fresh run after the last code change):

- Go: 23 packages `ok` (`go test ./...`), `go vet ./...` clean
- Rust: 436 unit and integration tests passed, 13 fuzz and property tests passed, 0 failed, `cargo clippy -- -D warnings` clean

## Code Hygiene

Reviewed against the mechanical checklist. One orphaned exported symbol (`Profile.WrapperName`, no caller outside tests) was turned into a package function and wired into both inline call sites (`profile/wrapper.go`, `session/restore.go`). No dead branches, unused parameters, shared-reference mutation, or orphaned references remain from the deep-review findings; dead helpers found by the review (`registerPluginInConfig`, `resolveRemote`, `defaultConfigSubdir`, unused `isolated` map and `checks` slice, duplicate `shellQuote`) were removed in earlier commits.

## Spec Compliance

**Status:** COMPLIANT

**Compliance Score:** 100%

FR-001 through FR-026 (27 requirements including FR-009a) are IMPLEMENTED with file-level evidence and a covering test each; the full matrix was produced against the current tree and is summarized here:

- Profile definition (FR-001 to FR-005): `internal/config/profile.go`, `internal/config/validate.go`; tests in `profile_test.go`, `validate_test.go`
- Wrapper generation (FR-006 to FR-013): `internal/profile/{translator,wrapper,configdir,sync,claude,codex,opencode}.go`, `internal/shellrc/block.go`, `build/templates/containerfile/05-shell-finalize.tmpl`; tests in `sync_test.go`, `contract_test.go`, per-translator golden tests, `review_fixes_test.go`, `shellrc/block_test.go`, `provision_test.go`
- Session identity and sidebar (FR-014 to FR-019): `internal/cmd/hook.go`, plugin `pipe_handler.rs`, `session.rs`, `controller/hooks.rs`, `controller/render_broadcast.rs`, `sidebar_plugin/render.rs`; tests in `hook_profile_test.go`, `controller/integration_tests.rs`, `sidebar_plugin/integration_tests.rs`
- Snapshot and restore (FR-020 to FR-023): `internal/session/{snapshot,save,restore}.go`; tests in `restore_test.go`
- CLI and compatibility (FR-024, FR-025): `internal/cmd/profile.go`; tests in `cmd/profile_test.go`, `ws/repos_test.go`
- OpenShell providers (FR-026): `internal/ws/openshell.go`, `internal/ws/profile_target.go`; tests in `openshell_test.go`

```
SPEC_COMPLIANCE_RESULT:
  total: 27
  implemented: 27
  missing: 0
  percentage: 100%
  gate: PASS
```

## Spec Drift Check

**Status:** NO DRIFT (after fix)

The compliance pass found that the sidebar applied the profile color to the activity indicator (status dot) instead of the harness glyph, contradicting FR-017 and the brainstorm decision. Fixed in this verification: the status dot keeps its status color and the glyph uses the profile color in both the plain and the highlighted row branch (`sidebar_plugin/render.rs`). No other divergence: wrapper naming, rc block mechanism, file locations, config-dir sharing, and the `cc-deck config profile` CLI path all match the spec.

## Success Criteria

**Status:** ALL MET (SC-003 verified by code and unit tests; the live SSH and OpenShell walk-through needs real infrastructure)

- [x] SC-001 profile add plus sync path complete and documented in the guide
- [x] SC-002 two profiles of one harness side by side: `test_profile_one_harness_two_profiles_shows_indicators`, per-profile env in wrappers
- [x] SC-003 same wrappers on local, SSH, OpenShell: `Provision` called from `ws/ssh.go` and `ws/openshell.go`, `provision_test.go`; the byte-identical wrapper is backend-agnostic. Live remote check pending (quickstart 5 and 6)
- [x] SC-004 snapshot restore under the right profile: `TestLaunchCommand_ClaudeWithProfile`, missing-profile fallback test
- [x] SC-005 pre-feature config and snapshots unchanged: `TestEffectiveAuth_Legacy`, `TestLoadActiveGitCredentials_LegacyProfile`, pre-feature snapshot tests
- [x] SC-006 no credential value in wrappers: `TestSync_WrapperContainsNoCredentialValue`, contract test 2
- [x] SC-007 new harness is one translator: `Translator` interface plus registry; sidebar, snapshot and transport are harness-agnostic

## Documentation

- `docs/modules/reference/pages/configuration.adoc`: profile schema, sources, defaults, file locations
- `docs/modules/reference/pages/cli.adoc`: `config profile sync`, `delete`, extended `add`, `list`, `show`
- `docs/modules/using/pages/harness-profiles.adoc` (registered in `nav.adoc`): two-account walk-through, sidebar legend, snapshots, login in remote workspaces, OpenShell re-create hint
- `README.md`: Harness profiles section

## Pending Manual Checks (not blocking, recorded in contracts/harness-translator.md)

1. macOS Keychain: whether Claude Code keys its subscription login by `CLAUDE_CONFIG_DIR`
2. Codex: `CODEX_HOME` honored for `config.toml` and `auth.json`
3. OpenCode: `OPENCODE_CONFIG` accepted as a file path and merged with the default config
4. Quickstart sections 5 and 6 on a real SSH host and OpenShell gateway

## Overall Status

**VERIFIED - Ready for completion**
