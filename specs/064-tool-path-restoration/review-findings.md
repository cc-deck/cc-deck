# Deep Review Findings

**Date:** 2026-05-25
**Branch:** main (uncommitted changes)
**Rounds:** 1
**Gate Outcome:** PASS
**Invocation:** quality-gate

## Summary

| Severity | Found | Fixed | Remaining |
|----------|-------|-------|-----------|
| Critical | 0 | 0 | 0 |
| Important | 0 | 0 | 0 |
| Minor | 2 | 1 | 1 |
| **Total** | **2** | **1** | **1** |

**Agents completed:** 5/5 (+ 0 external tools)
**Agents failed:** none

## Findings

### FINDING-1
- **Severity:** Minor
- **Confidence:** 85
- **File:** cc-deck/internal/build/containerfile.go:13-16
- **Category:** architecture
- **Source:** architecture-agent
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**
The godoc comment on `toolPathRegistry` described the matching strategy as "case-insensitive substring" matching, but the implementation uses word-boundary matching via `containsWord`. The comment was misleading and did not match the actual behavior.

**Why this matters:**
Misleading comments erode trust in documentation and can cause future developers to make incorrect assumptions about the matching behavior. A developer reading the comment might expect "cargo" to match "go" (substring), when it actually does not (word boundary).

**How it was resolved:**
Updated the comment to accurately say "word-boundary matching" instead of "substrings".

### FINDING-2
- **Severity:** Minor
- **Confidence:** 75
- **File:** cc-deck/internal/build/containerfile_test.go
- **Category:** test-quality
- **Source:** test-quality-agent
- **Round found:** 1
- **Resolution:** remaining (Minor, no auto-fix)

**What is wrong:**
No test explicitly verifies that tool names containing "go" as a substring (but not as a standalone word) are correctly rejected. For example, "Django" or "Mongoose" contain "go" but should not match the "go" registry key.

**Why this matters:**
The word-boundary matching is a deliberate improvement over the spec's original "substring matching" clarification. A regression test that validates this distinction would protect the behavior if someone refactors `containsWord` in the future.

**How it could be resolved:**
Add a test like `TestResolveToolPaths_SubstringNotWord` that verifies `ResolveToolPaths` returns an empty slice for a manifest with a tool named "Django" or "Mongoose".

## Post-Fix Spec Coverage

All spec requirements verified after fix loop.

| Requirement | Implementation | Status |
|-------------|---------------|--------|
| FR-001: Tool path registry | containerfile.go:17-21 `toolPathRegistry` | OK |
| FR-002: Resolve registry against manifest | containerfile.go:30-54 `ResolveToolPaths()` | OK |
| FR-003: Prepend to .zshrc and .bashrc | 05-shell-finalize.tmpl:1-9 | OK |
| FR-004: After tool installs, before user config | Template ordering (05-shell-finalize after 03-mandatory-stack) | OK |
| FR-005: Deduplicate paths | containerfile.go:43-46 `seen` map | OK |
| FR-006: Resolve home directory placeholder | containerfile.go:42 `strings.ReplaceAll` | OK |
| FR-007: No entries for standard PATH tools | Registry only has non-standard paths | OK |
| FR-008: ENV PATH unchanged | No diff in 03-mandatory-stack.tmpl | OK |
| FR-009: Works for openshell and container | Both branches in `ContainerDataForTarget` | OK |
| FR-010: Does not affect SSH | SSH not handled in containerfile.go | OK |

## Test Suite Results

| Round | Test Command | Exit Code | Failures | Status |
|-------|-------------|-----------|----------|--------|
| 1     | go test ./internal/build/ | 0 | 0 | passed |

Test suite passed in all fix rounds.

## Remaining Findings

FINDING-2 (Minor): Missing word-boundary regression test. Not blocking since it is Minor severity and all current acceptance scenarios are covered by existing tests.
