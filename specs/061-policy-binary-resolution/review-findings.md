# Deep Review Findings

**Date:** 2026-05-24
**Branch:** 061-policy-binary-resolution
**Rounds:** 1
**Gate Outcome:** PASS
**Invocation:** quality-gate

## Summary

| Severity | Found | Fixed | Remaining |
|----------|-------|-------|-----------|
| Critical | 1 | 1 | 0 |
| Important | 0 | 0 | 0 |
| Minor | 1 | - | 1 |
| **Total** | **2** | **1** | **1** |

**Agents completed:** 5/5 (+ 1 external tool)
**Agents failed:** none

## Findings

### FINDING-1
- **Severity:** Critical
- **Confidence:** 75
- **File:** cc-deck/internal/build/policy_binaries.go:64-69
- **Category:** correctness
- **Source:** coderabbit (external)
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**
`resolveBinaries` dereferences the `manifest` parameter without a nil guard. If `manifest` is nil, the function panics when accessing `manifest.Tools` on line 68.

**Why this matters:**
While all current callers pass non-nil manifests, this is a defensive programming gap. If a future caller passes nil (or if a test does), the function will panic instead of failing gracefully.

**How it was resolved:**
Added an early nil check at the top of `resolveBinaries`: if `manifest` is nil, the function returns the input components unchanged. This is safe because without a manifest there is no tool data to resolve.

**External tool analysis (CodeRabbit):**
> resolveBinaries currently dereferences manifest without a nil guard and will panic if manifest is nil; add an early nil check at the start of resolveBinaries (checking the manifest parameter) and return the input components unchanged (or otherwise handle/report the error) if manifest == nil, so subsequent access to manifest.Tools and the creation of toolIndex is safe.

### FINDING-2
- **Severity:** Minor
- **Confidence:** 70
- **File:** cc-deck/internal/build/policy_binaries_test.go:236-242
- **Category:** architecture
- **Source:** architecture-agent
- **Round found:** 1
- **Resolution:** remaining (Minor, not auto-fixed)

**What is wrong:**
Duplicate test helper functions: `binaryPaths()` in `policy_binaries_test.go` and `policyBinaryPaths()` in `policy_test.go` are identical functions that extract path strings from `[]PolicyBinary`.

**Why this matters:**
Minor maintenance burden. If the PolicyBinary type changes, both helpers need updating. However, test helper duplication is common in Go codebases and does not affect correctness.

## Post-Fix Spec Coverage

| Requirement | Implementation | Status |
|-------------|---------------|--------|
| FR-001: Resolve binary paths from match.tools | policy_binaries.go:resolveBinaries() | OK |
| FR-002: Package tools default to /usr/bin/ | policy_binaries.go:94-96 (default case) | OK |
| FR-003: github-release uses install_path | policy_binaries.go:90-93 | OK |
| FR-004: Well-known paths table | policy_binaries.go:10-57 (wellKnownPaths) | OK |
| FR-005: Include all well-known paths | policy_binaries.go:100-105 | OK |
| FR-006: Preserve explicit binaries | policy_binaries.go:75-77 | OK |
| FR-007: Skip missing tools silently | policy_binaries.go:87-88 | OK |
| FR-008: Remove hardcoded binaries from pkg components | policies/{go,rust,node,python}.yaml | OK |
| FR-009: Preserve explicit binaries on always-match components | policies/{claude-code,vertex-ai,git-hosting}.yaml | OK |

All spec requirements verified after fix loop.

## Test Suite Results

| Round | Test Command | Exit Code | Failures | Status |
|-------|-------------|-----------|----------|--------|
| 1     | go test ./internal/build/ | 0 | 0 | passed |

Test suite passed in all fix rounds.

## Remaining Findings

FINDING-2 (Minor): Duplicate test helpers. Not auto-fixed because Minor severity does not trigger the fix loop. Recommend consolidating in a future cleanup.
