# Deep Review Findings

**Date:** 2026-10-12
**Branch:** 087-harness-profiles
**Scope:** revision 2026-10-12, `79f1ccd..HEAD` (39 non-spec files); the earlier feature code was deep-reviewed on 2026-09-09
**Rounds:** 1
**Gate Outcome:** PASS
**Invocation:** quality-gate (after_implement via review-code)

## Summary

| Severity | Found | Fixed | Remaining |
|----------|-------|-------|-----------|
| Critical | 0 | 0 | 0 |
| Important | 6 | 6 | 0 |
| Minor | 21 | 15 | 6 |
| Notable | 4 | - | 4 |
| **Total** | **31** | **21** | **10** |

**Agents completed:** 5/5 (+ CodeRabbit; re-review: 1 combined agent + CodeRabbit)
**Agents failed:** Codex (usage limit until 2026-10-17); Copilot not installed

## Findings

### FINDING-1 (Important, architecture, fixed round 1)
- **File:** cc-zellij-plugin/src/sidebar_plugin/render.rs `agent_indicator_color`
- **What is wrong:** The glyph-to-brand-color table had no entry for the Codex glyph `◆`, so Codex glyphs rendered in the grey fallback.
- **Why this matters:** FR-017a requires the harness brand color; Claude next to Codex is the main case where glyphs show.
- **How it was resolved:** Added `◆ => (16, 163, 127)` and a test that every harness glyph maps to a non-fallback color.

### FINDING-2 (Important, architecture, fixed round 1)
- **File:** cc-zellij-plugin/src/controller/render_broadcast.rs legend; cc-deck/internal/cmd/hook.go
- **What is wrong:** The legend inferred a declared icon by comparing `agent_indicator` against a Rust copy of the Go glyph table.
- **Why this matters:** Any mismatch across the language boundary showed a harness glyph as a fake icon, and an icon equal to its harness glyph vanished from the legend.
- **How it was resolved:** The hook sends an explicit `profile_icon`; `HookPayload`, `Session` and `LegendEntry.icon` (renamed from `indicator`) carry it; the comparison is gone. Contracts and data model updated.

### FINDING-3 (Important, correctness + test-quality, fixed round 1)
- **File:** cc-deck/internal/cmd/hook.go; hook_profile_test.go
- **What is wrong:** SC-010 said "zero reads of config.yaml", but badge evaluation still loads config on every event with a cwd, and the unreadable-directory test could not prove the absence of a read.
- **Why this matters:** The criterion was unprovable as written and the test passed for the wrong reason.
- **How it was resolved:** SC-010 narrowed to profile data, matching the plan's performance goal (badge evaluation predates the feature). `enrichProfile` loads through a `loadProfileConfig` seam; tests assert 0 loads on the env path and 1 load on the fallback.

### FINDING-4 (Important, test-quality, fixed round 1)
- **File:** cc-deck/internal/cmd/hook.go fallback
- **What is wrong:** An invalid hand-edited `color:` was copied verbatim into the payload; the plugin dropped it and the entry lost its edge bar.
- **Why this matters:** FR-014a and the spec edge case require the derived color for a profiled session without a valid color.
- **How it was resolved:** New `profile.EffectiveColor` (valid declared color, else derived) used by resolve, the collision check, the picker, the hook, restore-meta and `profile show`; test with `color: "red"`.

### FINDING-5 (Important, test-quality, fixed round 1)
- **File:** cc-deck/internal/profile (new wrapper_exec_test.go)
- **What is wrong:** No test executed a generated wrapper; acceptance 1a, FR-007a and FR-013 rested on golden text.
- **Why this matters:** A wrapper with the right text but wrong behavior would pass.
- **How it was resolved:** Tests run rendered wrappers with a stub `claude` that prints its environment: inherited API key, auth token, Vertex switch and icon are cleared; token env and file sources reach `CLAUDE_CODE_OAUTH_TOKEN` (no trailing newline); missing env and file tokens exit 1 naming the profile without starting the harness; login profiles clear all auth variables.

### FINDING-6 (Important, production-readiness + test-quality, fixed round 1)
- **File:** cc-deck/internal/cmd/ws.go `runWsRefreshCreds`; cc-deck/internal/ws/ssh.go
- **What is wrong:** `ws refresh-creds` rewrote `credentials.env` from detected agent credentials only, dropping the profile env credentials that attach wrote, and refused to run for token-only setups. The attach merge had no test.
- **Why this matters:** FR-002a for the rotation path; a refresh broke `claude-private` on the remote.
- **How it was resolved:** Pure `sshCredentialSet` plus `ws.SSHCredentialSet`, used by attach and refresh-creds; a merge conflict skips injection instead of writing a partial set; five unit tests; CLI reference updated.

### Minor findings fixed
- Env icon accepted without validation (terminal escape injection); new `config.ValidIcon` rejects control and format runes, invalid UTF-8 and multi-glyph values, used by the hook and by config validation (security).
- Wrapper without icon inherited `CC_DECK_PROFILE_ICON` from an enclosing profiled session; wrappers now always export it (empty when undeclared) (correctness).
- `Provision` warned about color collisions for profiles not provisioned on the target; scoped to provisioned profiles (production-readiness).
- Effective color logic duplicated in four places, two more found in re-review; unified in `EffectiveColor` (architecture).
- Docs claimed the SSH token appears in no generated file; corrected to describe `credentials.env` (security).
- AsciiDoc sentences sharing a line in new bullets and table cells (CodeRabbit).
- `SSHCredentialSet` swallowed a config load error silently; now logged (re-review).
- Missing tests: derived-color collision, invalid profile excluded from collisions, Provision warnings, add next to a colorless profile, unsafe config icon, `EffectiveColor` table, add refusing on an invalid existing color (test-quality).

### Minor findings remaining
- `credentials.env` quotes values with Go `%q`, which is not shell-safe for `$`, backticks or non-ASCII (pre-existing transport code; see inbox `per-profile-remote-secret-scope`).
- Remote credential files are written via argv and chmod'ed after creation (pre-existing; same inbox entry).
- oauth_token support is decided both in `config.Profile.Validate` and in `Translator.SupportsOAuthToken` (architecture).
- `HueDistance` is exported but used only by tests (architecture; kept because T067 specifies it).
- `session_entry_lines` computes `row_highlight` twice and has an unreachable bold branch (architecture).
- The SC-010 counting seam can be bypassed by a future direct `config.Load` in `enrichProfile`; Attach and `SSHCredentialSet` still each build their own target (re-review).

## Notable Observations

Captured in `brainstorm/idea-inbox.md`: `per-profile-remote-secret-scope`, `env-override-warning-in-validate`, `derived-color-palette-shift`, `anthropic-auth-token-precedence`.

## Post-Fix Spec Coverage

The fix round removed code (old color helper, legend glyph comparison, inline icon checks). FR-005 (icon rule, `ValidIcon`), FR-014a (`enrichProfile`), FR-017 (picker via `EffectiveColor`), FR-018 and FR-019 (legend `profile_icon`) were rechecked: implementation present and covered by passing tests. All spec requirements verified after the fix loop.

## Test Suite Results

| Round | Test Command | Exit Code | Failures | Status |
|-------|-------------|-----------|----------|--------|
| 1 | make test | 0 | 0 | passed |
| final | make verify | 0 | 0 | passed (Go 23 packages ok, Rust 452 + 13 passed, vet and clippy clean) |
