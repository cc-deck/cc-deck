# Code Review: Policy Binary Resolution

**Spec:** specs/061-policy-binary-resolution/spec.md
**Date:** 2026-05-24
**Reviewer:** Claude (speckit.spex-gates.review-code)

## Compliance Summary

**Overall Score: 100%**

- Functional Requirements: 9/9 (100%)
- Error Handling: N/A (no error cases in spec)
- Edge Cases: 4/4 (100%)
- Non-Functional: 5/5 (100%)

## Detailed Review

### Functional Requirements

#### FR-001: Resolve binary paths by looking up match.tools in manifest tools
**Implementation:** `cc-deck/internal/build/policy_binaries.go:64-118` (`resolveBinaries()`)
**Status:** Compliant
**Notes:** Iterates each component's `Match.Tools`, builds a `toolIndex` from manifest, and resolves paths per tool.

#### FR-002: Package tools default to /usr/bin/<tool-name>
**Implementation:** `cc-deck/internal/build/policy_binaries.go:94-96` (default case in switch)
**Status:** Compliant
**Notes:** `default` case handles both "package" and empty install type, adding `/usr/bin/<lower>`.

#### FR-003: github-release tools use install_path from manifest
**Implementation:** `cc-deck/internal/build/policy_binaries.go:90-93`
**Status:** Compliant
**Notes:** `case "github-release"` checks `entry.InstallPath` and adds it as the binary path.

#### FR-004: Well-known paths table maintained in codebase
**Implementation:** `cc-deck/internal/build/policy_binaries.go:10-57` (`wellKnownPaths` map)
**Status:** Compliant
**Notes:** Covers cargo, rustc, go, node, npm, npx, pip, pip3, uv, claude, git, gh with multiple alternative paths each.

#### FR-005: All well-known paths included in addition to manifest-derived path
**Implementation:** `cc-deck/internal/build/policy_binaries.go:100-105`
**Status:** Compliant
**Notes:** Well-known paths are added after the manifest-derived path, with deduplication via `seen` map.

#### FR-006: Explicit binaries preserved (no override)
**Implementation:** `cc-deck/internal/build/policy_binaries.go:75-77`
**Status:** Compliant
**Notes:** Components with `len(comp.Binaries) > 0` are skipped entirely. Tested in `TestResolveBinaries_ExplicitBinariesPreserved`.

#### FR-007: Tools not in manifest skipped silently
**Implementation:** `cc-deck/internal/build/policy_binaries.go:87-88`
**Status:** Compliant
**Notes:** If tool not found in `toolIndex`, no error is produced. Well-known paths are still added. Tested in `TestResolveBinaries_ToolNotInManifestSkipped`.

#### FR-008: Hardcoded binaries removed from embedded package registry components
**Implementation:** `cc-deck/internal/build/policies/{go,rust,node,python}.yaml`
**Status:** Compliant
**Notes:** Git diff confirms all four files had their `binaries:` sections removed entirely.

#### FR-009: Explicit binaries preserved on claude-code.yaml, vertex-ai.yaml, git-hosting.yaml
**Implementation:** `cc-deck/internal/build/policies/{claude-code,vertex-ai,git-hosting}.yaml`
**Status:** Compliant
**Notes:** All three files retain their explicit `binaries:` fields. These were not modified in this change.

### Edge Cases

#### Tool in match.tools not in manifest
**Status:** Compliant
**Notes:** `TestResolveBinaries_ToolNotInManifestSkipped` verifies this. Well-known paths still added but no `/usr/bin/` default.

#### Binary name differs from tool name
**Status:** Compliant
**Notes:** The `wellKnownPaths` table provides correct mappings (e.g., `cargo` maps to cargo-specific paths, not generic paths).

#### Multiple components reference the same tool
**Status:** Compliant
**Notes:** `TestResolveBinaries_MultipleComponents` verifies each component gets independent resolution.

#### Tool has both well-known paths and install_path
**Status:** Compliant
**Notes:** Both are included (deduplication via `seen` map). `addPath()` prevents duplicates.

### Success Criteria

#### SC-001: Package registry components work without hardcoded binaries
**Status:** Met
**Notes:** All embedded package components have binaries removed. `TestAssemblePolicy_ResolvesToolBinaries` and `TestAssemblePolicy_ComponentWithoutBinariesGetsResolved` verify resolution works.

#### SC-002: New catalog component with only endpoints/match.tools gets binary paths
**Status:** Met
**Notes:** The resolution function operates on matched components regardless of tier. Any component without explicit binaries gets resolution.

#### SC-003: Less than 10ms processing time
**Status:** Met (inferred)
**Notes:** The resolution is a simple map lookup + iteration, no I/O. 164 tests pass quickly.

#### SC-004: All existing tests continue to pass
**Status:** Met
**Notes:** 164 tests pass in the build package.

#### SC-005: Explicit binaries never modified by automatic resolution
**Status:** Met
**Notes:** `TestResolveBinaries_ExplicitBinariesPreserved` and `TestAssemblePolicy_ExplicitBinariesPreserved` both verify this.

### Extra Features (Not in Spec)

#### Case-insensitive tool lookup
**Location:** `cc-deck/internal/build/policy_binaries.go:85,68`
**Description:** Tool names are lowercased before lookup (`strings.ToLower`).
**Assessment:** Helpful addition. Prevents case mismatch issues.
**Recommendation:** Add to spec if desired, but harmless.

#### Well-known paths added even without manifest entry
**Location:** `cc-deck/internal/build/policy_binaries.go:100-105`
**Description:** If a tool is in match.tools but NOT in the manifest, well-known paths are still added (just no `/usr/bin/` default).
**Assessment:** Matches edge case spec: "Tools not listed in the manifest but pre-installed in the base image are not covered by automatic resolution (they rely on the well-known paths table)."
**Recommendation:** This is actually spec-compliant per the Assumptions section.

## Code Quality Notes

- Clean separation: `policy_binaries.go` is a focused file with a single responsibility
- Good deduplication via `seen` map in `addPath()`
- Comprehensive test coverage: 8 unit tests in `policy_binaries_test.go` + 3 integration tests in `policy_test.go`
- No unnecessary dependencies introduced
- README.md updated with feature description

## Gate Result

**PASS** - 100% spec compliance.

---

## Deep Review Report

**Date:** 2026-05-24
**Gate:** PASS (after fix round 1)
**Rounds:** 1
**Fix applied:** 1 Critical (nil guard on resolveBinaries)

### Review Agents

| Agent                   | Found | Fixed | Remaining | Status    |
|-------------------------|-------|-------|-----------|-----------|
| Correctness             |     0 |     0 |         0 | completed |
| Architecture & Idioms   |     1 |     0 |         1 | completed |
| Security                |     0 |     0 |         0 | completed |
| Production Readiness    |     0 |     0 |         0 | completed |
| Test Quality            |     0 |     0 |         0 | completed |
| CodeRabbit (external)   |     1 |     1 |         0 | completed |
| Copilot (external)      |     - |     - |         - | skipped (CLI not installed) |
| Test Suite (regression) |     0 |     0 |         0 | passed    |
|-------------------------|-------|-------|-----------|-----------|
| Total                   |     2 |     1 |         1 |           |

### Key fixes applied

1. Added nil guard to `resolveBinaries()` in `policy_binaries.go` to prevent panic when manifest is nil (coderabbit)

### Remaining findings (1 Minor)

- Duplicate test helper functions `binaryPaths()` and `policyBinaryPaths()` (architecture-agent, policy_binaries_test.go:236)

### Post-fix spec coverage

9/9 requirements verified. All covered.

### Test suite results

| Round | Test Command | Exit Code | Failures | Status |
|-------|-------------|-----------|----------|--------|
| 1     | go test ./internal/build/ | 0 | 0 | passed |

Details: specs/061-policy-binary-resolution/review-findings.md
