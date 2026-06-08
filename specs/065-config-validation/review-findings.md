# Deep Review Findings

**Date:** 2026-06-02
**Branch:** 065-config-validation
**Rounds:** 0 (no fix loop needed)
**Gate Outcome:** PASS
**Invocation:** quality-gate

## Summary

| Severity | Found | Fixed | Remaining |
|----------|-------|-------|-----------|
| Critical | 0 | 0 | 0 |
| Important | 0 | 0 | 0 |
| Minor | 6 | 0 | 6 |
| **Total** | **6** | **0** | **6** |

**Agents completed:** 5/5 (+ 0 external tools)
**Agents failed:** none

## Findings

### FINDING-1
- **Severity:** Minor
- **Confidence:** 85
- **File:** cc-deck/internal/config/validate.go:122-129
- **Category:** correctness
- **Source:** correctness-agent (also reported by: architecture-agent)
- **Round found:** 1
- **Resolution:** remaining (minor, cosmetic)

**What is wrong:**
The `isEmoji` function has a nested `switch` inside an `if` block that checks `r >= 0x2600 && r <= 0x27BF`. The switch cases cover 0x2600-0x26FF and 0x2700-0x27BF, which together span the entire outer `if` range. The inner switch adds no filtering value.

**Why this matters:**
Redundant branching logic can confuse future maintainers into thinking the switch provides additional filtering when it does not.

**Suggested fix:**
Remove the inner switch and return true directly from the outer if block.

### FINDING-2
- **Severity:** Minor
- **Confidence:** 75
- **File:** cc-deck/internal/config/validate.go:257
- **Category:** architecture
- **Source:** architecture-agent
- **Round found:** 1
- **Resolution:** remaining (minor)

**What is wrong:**
The `suggestedReplacement` function checks `unicode.Is(unicode.So, r)` first in the OR chain. Since this function is only called from `checkIconWidth` after `isEmoji` or `isEastAsianWide` already matched, the `unicode.So` check is effectively dead code in the first branch.

**Why this matters:**
Dead conditions make the function harder to reason about and may mislead maintainers about the function's intended scope.

### FINDING-3
- **Severity:** Minor
- **Confidence:** 80
- **File:** cc-deck/internal/config/validate.go:318
- **Category:** production-readiness
- **Source:** production-readiness-agent
- **Round found:** 1
- **Resolution:** remaining (minor)

**What is wrong:**
Map iteration over `badge.Values` is non-deterministic in Go. Multiple runs of `cc-deck config check` on the same config may produce findings for the same badge in different orders.

**Why this matters:**
Non-deterministic output is harder to diff or compare between runs. Not a correctness issue but reduces output quality for scripted workflows.

### FINDING-4
- **Severity:** Minor
- **Confidence:** 70
- **File:** cc-deck/internal/config/validate_test.go
- **Category:** test-quality
- **Source:** test-quality-agent
- **Round found:** 1
- **Resolution:** remaining (minor)

**What is wrong:**
No test for `checkIconWidth` with a multi-rune icon string (e.g., where the first rune is Narrow but a later rune is Wide). The function only checks the first rune, which is by design, but a test documenting this would prevent future regressions.

### FINDING-5
- **Severity:** Minor
- **Confidence:** 72
- **File:** cc-deck/internal/config/validate_test.go
- **Category:** test-quality
- **Source:** test-quality-agent
- **Round found:** 1
- **Resolution:** remaining (minor)

**What is wrong:**
No test for `validateProfiles` when `defaultProfile` is non-empty but `profiles` is nil/empty. The `len(profiles) > 0` guard silently accepts this case without producing a finding.

### FINDING-6
- **Severity:** Minor
- **Confidence:** 70
- **File:** cc-deck/internal/config/validate_test.go:519-586
- **Category:** test-quality
- **Source:** test-quality-agent
- **Round found:** 1
- **Resolution:** remaining (minor)

**What is wrong:**
`ValidateAndWarn` tests redirect `os.Stderr` via `os.Pipe()`. This is a global mutation that could race with parallel test execution. In this package, tests run sequentially so the risk is contained.

## Test Suite Results

| Round | Test Command | Exit Code | Failures | Status |
|-------|-------------|-----------|----------|--------|
| pre-review | make test | 2 | 2 | pre-existing failures |

Pre-existing test failures in `TestComposeSmokeFullLifecycle` and `TestComposeSmokeNetworkFiltering` (compose smoke tests requiring podman infrastructure). Not related to config validation changes.

Validation-specific tests: 79/79 passed (`go test ./internal/config/... ./internal/cmd/... -run "TestCheck|TestValidate|TestParseBadge|TestIsEmoji|TestIsEast|TestConfig"`).
