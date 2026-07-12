# Deep Review Findings

**Date:** 2026-07-12
**Branch:** 081-codex-adapter
**Rounds:** 1
**Gate Outcome:** PASS
**Invocation:** superpowers

## Summary

| Severity | Found | Fixed | Remaining |
|----------|-------|-------|-----------|
| Critical | 0 | 0 | 0 |
| Important | 4 | 4 | 0 |
| Minor | 4 | 0 | 4 |
| Notable | 2 | 0 | 2 |
| **Total** | **10** | **4** | **6** |

**Agents completed:** 5/5 (+ 1 external tool)
**Agents failed:** none

## Findings

### FINDING-1
- **Severity:** Important
- **Confidence:** 95
- **File:** cc-deck/internal/agent/codex.go:216-220
- **Category:** correctness
- **Source:** correctness-agent (also reported by: coderabbit)
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**
`readCodexHooks()` could return a nil map when `hooks.json` contains the JSON literal `null`. `json.Unmarshal([]byte("null"), &mapVar)` succeeds and sets the map to nil. A subsequent `hooksFile["hooks"] = hooks` in `InstallHooks()` would panic on nil map assignment.

**Why this matters:**
A panic crashes the entire cc-deck process instead of returning a graceful error. While a file containing only `null` is unlikely, robustness against malformed input is a spec requirement (EH-001).

**How it was resolved:**
Added a nil guard after `json.Unmarshal` in `readCodexHooks`: if `hooksFile` is nil after successful unmarshal, initialize it to an empty map. This prevents the nil map panic and treats `null` as equivalent to `{}`.

### FINDING-2
- **Severity:** Important
- **Confidence:** 95
- **File:** cc-deck/internal/agent/codex_test.go:56-65
- **Category:** test-quality
- **Source:** test-quality-agent
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**
`TestCodexAgentInstallHooks` verified that each event had exactly 1 hook entry and the total event count was correct, but never checked the actual command string inside the hook entry. If `codexHookCommand()` returned a wrong string, this test would pass.

**Why this matters:**
The hook command is the mechanism that connects Codex to cc-deck. A wrong command means total integration failure, yet the primary install test would be green.

**How it was resolved:**
Added assertions that drill into each hook entry's action array and verify the command string matches `codexHookCommand()`.

### FINDING-3
- **Severity:** Important
- **Confidence:** 90
- **File:** cc-deck/internal/agent/codex_test.go:207-226
- **Category:** test-quality
- **Source:** test-quality-agent
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**
`TestCodexAgentUninstallHooks` only asserted `HooksInstalled() == false` after uninstall. It never read the file back to verify hooks were actually removed from the JSON. If `HooksInstalled()` had a bug (e.g., always returning false for empty hooks maps), the test would pass despite `UninstallHooks()` being a no-op.

**Why this matters:**
The test was circular: it verified UninstallHooks by calling HooksInstalled, which is the method that should also be independently tested.

**How it was resolved:**
Added file-level verification: after uninstall, the test reads and parses hooks.json and asserts that the `hooks` key is either absent or contains an empty map.

### FINDING-4
- **Severity:** Important
- **Confidence:** 85
- **File:** cc-deck/internal/agent/codex_test.go:390-392
- **Category:** test-quality
- **Source:** test-quality-agent
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**
`TestCodexAgentTranslateEvent` only checked `ToolName` when `wantTool != ""`. For the 7 out of 8 test cases where `wantTool` is empty, the test never asserted that `payload.ToolName` was actually empty.

**Why this matters:**
If `TranslateEvent` incorrectly populated `ToolName` for non-tool events (e.g., from a stale field or parsing bug), the test would not catch it.

**How it was resolved:**
Changed the conditional to always assert: `if payload.ToolName != tt.wantTool`. This verifies both presence and absence of ToolName.

### FINDING-5
- **Severity:** Minor
- **Confidence:** 85
- **File:** cc-deck/internal/agent/codex.go:200-203
- **Category:** production-readiness
- **Source:** correctness-agent (also reported by: security-agent, production-readiness-agent, coderabbit)
- **Round found:** 1
- **Resolution:** accepted (pre-existing pattern)

**What is wrong:**
`defaultCodexHooksPath()` silently discards the error from `os.UserHomeDir()`. If `$HOME` is unset, the result is a relative path `.codex/hooks.json`.

**Why this matters:**
In containerized environments, this could write hooks to an unexpected location. However, this is the identical pattern used in `defaultClaudeSettingsPath()` (claude.go:297-299) and `defaultOpenCodeConfigPath()` (opencode.go:233-234). Fixing this in isolation for Codex while leaving the others unchanged would be inconsistent.

### FINDING-6
- **Severity:** Minor
- **Confidence:** 75
- **File:** cc-deck/internal/agent/codex.go:48-60
- **Category:** correctness
- **Source:** coderabbit
- **Round found:** 1
- **Resolution:** accepted (defensive enhancement beyond spec)

**What is wrong:**
`InstallHooks()` uses Go type assertions to extract `hooks` and event arrays. If the JSON has valid structure but unexpected types (e.g., `"hooks": "string"`), the type assertion silently returns nil/false, treating it as empty.

**Why this matters:**
The current behavior is gracefully permissive: unexpected types are treated as absent and overwritten with correct structure. The Claude adapter uses the same pattern. Adding strict type validation would be defensive but goes beyond spec EH-001 (which covers malformed JSON, not schema validation).

### FINDING-7
- **Severity:** Minor
- **Confidence:** 80
- **File:** cc-deck/internal/agent/codex_test.go:416-427
- **Category:** test-quality
- **Source:** test-quality-agent
- **Round found:** 1
- **Resolution:** accepted (low risk)

**What is wrong:**
`TestCodexAgentTranslateEventIgnoresExtraFields` verifies no error is returned, but does not assert that extra fields are absent from the resulting NormalizedPayload.

**Why this matters:**
If a future change adds mapping for extra fields, this test would still pass. However, `NormalizedPayload` is a fixed struct without extensible fields, making this scenario unlikely.

### FINDING-8
- **Severity:** Minor
- **Confidence:** 75
- **File:** cc-deck/internal/agent/codex_test.go:429-449
- **Category:** test-quality
- **Source:** test-quality-agent
- **Round found:** 1
- **Resolution:** accepted (covered by existing "invalid JSON" case)

**What is wrong:**
Missing test cases for nil input and empty byte slice. The existing "invalid JSON" test case with `"not json"` covers the json.Unmarshal error path, and `{}` covers the empty-but-valid case with missing event name.

### FINDING-9
- **Severity:** Notable
- **Confidence:** 92
- **File:** cc-deck/internal/agent/codex.go:41-107
- **Category:** architecture
- **Source:** architecture-agent
- **Round found:** 1
- **Resolution:** noted (intentional per plan)

**What is wrong:**
`InstallHooks()`, `UninstallHooks()`, and `HooksInstalled()` are near-identical copies of the same methods in `claude.go`. This duplication could diverge over time.

**Why this matters:**
The plan explicitly calls for "following the claude.go pattern." Extracting shared infrastructure is a valid future improvement (tracked as a potential refactoring when a fourth JSON-hooks agent is added) but is out of scope for this feature.

### FINDING-10
- **Severity:** Notable
- **Confidence:** 88
- **File:** cc-deck/internal/agent/codex.go:184-186
- **Category:** architecture
- **Source:** architecture-agent
- **Round found:** 1
- **Resolution:** noted (matches existing pattern)

**What is wrong:**
`codexHookCommand()` hardcodes the agent name in the command string. Both `codexHookCommand()` and `ccDeckHookCommand()` return identical strings differing only in the agent name parameter.

**Why this matters:**
A shared `ccDeckHookCommand(agentName string)` would eliminate this duplication. However, the identical pattern exists in claude.go and is the established convention. Refactoring should happen across all adapters simultaneously, not piecemeal.
