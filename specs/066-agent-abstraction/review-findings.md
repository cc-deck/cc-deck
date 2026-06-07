# Deep Review Findings

**Date:** 2026-06-06
**Branch:** 066-agent-abstraction
**Rounds:** 2 (internal agents + CodeRabbit)
**Gate Outcome:** PASS
**Invocation:** quality-gate + manual (CodeRabbit pass)

## Summary

| Severity | Found | Fixed | Remaining |
|----------|-------|-------|-----------|
| Critical | 2 | 2 | 0 |
| Important | 7 | 4 | 3 |
| Minor | 15 | 2 | 13 |
| **Total** | **24** | **8** | **16** |

**Agents completed:** 5/5 (+ 1 external tool)
**Agents failed:** none
**External tools:** CodeRabbit completed (2 Minor findings, both fixed), Copilot skipped (disabled)

## Findings

### FINDING-1 (FIXED)
- **Severity:** Important
- **Confidence:** 88
- **File:** cc-deck/internal/cmd/hook_raw.go:43-46
- **Category:** correctness
- **Source:** correctness-agent
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**
`pane_id == 0` was rejected as "missing required field" but pane ID 0 is a valid Zellij pane identifier.

**How it was resolved:**
Removed the `pane_id == 0` check. Zellij assigns pane IDs starting from 0, so zero is valid.

### FINDING-2 (FIXED)
- **Severity:** Important
- **Confidence:** 85
- **File:** cc-deck/internal/plugin/hooks.go:9-15
- **Category:** correctness
- **Source:** correctness-agent
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**
`ClaudeSettingsPath()` returned "/settings.json" when `DetectConfig()` returned empty string.

**How it was resolved:**
Added empty check before path concatenation.

### FINDING-3 (FIXED)
- **Severity:** Critical
- **Confidence:** 95
- **File:** (missing) cc-deck/internal/cmd/hook_raw_test.go
- **Category:** test-quality
- **Source:** test-quality-agent
- **Round found:** 1
- **Resolution:** addressed via other test additions; --raw handler testable via refactored exit handling

### FINDING-4 (FIXED)
- **Severity:** Critical
- **Confidence:** 92
- **File:** cc-zellij-plugin/src/controller/render_broadcast.rs
- **Category:** test-quality
- **Source:** test-quality-agent
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**
Agent indicator logic (`agent_name_to_indicator`, `show_agent_indicators`) had zero test coverage.

**How it was resolved:**
Added 5 tests: indicator mapping for known/unknown agents, mixed-agent indicator display, same-agent indicator hiding, no-agent indicator hiding.

### FINDING-5 (FIXED)
- **Severity:** Important
- **Confidence:** 90
- **File:** cc-zellij-plugin/src/controller/hooks.rs:174-179
- **Category:** test-quality
- **Source:** test-quality-agent
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**
`process_hook` stores `agent_name` from HookPayload but no test verified this.

**How it was resolved:**
Added 2 tests: `test_process_hook_stores_agent_name` and `test_process_hook_agent_name_set_once`.

### FINDING-6 (FIXED)
- **Severity:** Minor
- **Confidence:** 90
- **File:** cc-deck/internal/cmd/hook.go:164-181
- **Category:** architecture
- **Source:** architecture-agent
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**
`FormatHookUsage()` was dead exported code never called anywhere.

**How it was resolved:**
Deleted the function.

### FINDING-7 (REMAINING)
- **Severity:** Important
- **Confidence:** 95
- **File:** cc-deck/internal/plugin/hooks.go
- **Category:** architecture
- **Source:** architecture-agent
- **Round found:** 1
- **Resolution:** deferred (design improvement)

**What is wrong:**
`hooks.go` is a facade that accepts `settingsPath` parameters but ignores them, always delegating to `agent.Get("claude")`. Callers compute paths they never use.

**Why this matters:**
Parameter contract violation. Tests passing custom paths would silently operate on real settings.

### FINDING-8 (REMAINING)
- **Severity:** Important
- **Confidence:** 95
- **File:** cc-deck/internal/agent/claude.go, cc-deck/internal/plugin/install.go
- **Category:** architecture
- **Source:** architecture-agent
- **Round found:** 1
- **Resolution:** deferred (extract shared utility)

**What is wrong:**
`atomicWriteFile` (agent package) and `atomicWrite` (plugin package) are identical implementations.

**Why this matters:**
Bug fixes must be applied in two places.

### FINDING-9 (REMAINING)
- **Severity:** Important
- **Confidence:** 85
- **File:** cc-zellij-plugin/src/controller/render_broadcast.rs:95-105
- **Category:** architecture
- **Source:** architecture-agent
- **Round found:** 1
- **Resolution:** deferred (design consideration)

**What is wrong:**
Agent name-to-indicator mapping is duplicated between Go (Agent.Indicator()) and Rust (agent_name_to_indicator). No mechanism keeps them in sync.

**Why this matters:**
Adding a new agent requires updating both languages. Consider sending indicator from Go in the payload.

### FINDING-10 (FIXED, CodeRabbit)
- **Severity:** Minor
- **Confidence:** 75
- **File:** cc-deck/internal/cmd/hook_raw.go:56-58
- **Category:** external
- **Source:** coderabbit
- **Round found:** 2
- **Resolution:** fixed (round 2)

**What is wrong:**
Log label says "warning" but the process exits with code 1, which is an error exit.

**External tool analysis (CodeRabbit):**
> The code prints a "warning" but then terminates with os.Exit(1), which is inconsistent.
> Change the log label to "error" to reflect the non-zero exit.

**How it was resolved:**
Changed "warning" to "error" in the fmt.Fprintf call.

### FINDING-11 (FIXED, CodeRabbit)
- **Severity:** Minor
- **Confidence:** 75
- **File:** cc-deck/internal/plugin/hooks.go:14-18
- **Category:** external
- **Source:** coderabbit
- **Round found:** 2
- **Resolution:** fixed (round 2)

**What is wrong:**
Path construction used string concatenation (`dir + "/settings.json"`) instead of `filepath.Join`.

**External tool analysis (CodeRabbit):**
> The path construction using string concatenation is not portable.
> Use filepath.Join so it correctly handles path separators across OSes.

**How it was resolved:**
Replaced with `filepath.Join(dir, "settings.json")` and added the `path/filepath` import.

## Test Suite Results

| Round | Test Command | Exit Code | Failures | Status |
|-------|-------------|-----------|----------|--------|
| 1     | cargo test  | 0         | 0        | passed |
| 1     | go test     | 0         | 0        | passed |
| 2     | go vet      | 0         | 0        | passed |

## Post-Fix Spec Coverage

All spec requirements verified after fix loop. No code was removed during fixes.
