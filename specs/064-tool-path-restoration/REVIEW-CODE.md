# Code Review: Tool PATH Restoration in Container Builds

**Spec:** specs/064-tool-path-restoration/spec.md
**Date:** 2026-05-25
**Reviewer:** Claude (speckit.spex-gates.review-code)

## Compliance Summary

**Overall Score: 100%**

- Functional Requirements: 10/10 (100%)
- Error Handling: N/A (build-time feature, no runtime errors)
- Edge Cases: 4/4 (100%)
- Success Criteria: 4/4 (100%)

## Detailed Review

### Functional Requirements

#### FR-001: Tool path registry
**Implementation:** cc-deck/internal/build/containerfile.go:17-21
**Status:** Compliant
**Notes:** Registry maps "go", "cargo", "rust" to their install paths with `{home}` placeholder.

#### FR-002: Resolve registry against manifest tool list
**Implementation:** cc-deck/internal/build/containerfile.go:30-54 `ResolveToolPaths()`
**Status:** Compliant
**Notes:** Iterates manifest tools, matches against registry using case-insensitive word-boundary matching, replaces placeholder, deduplicates.

#### FR-003: Prepend resolved paths to .zshrc and .bashrc
**Implementation:** cc-deck/internal/build/templates/containerfile/05-shell-finalize.tmpl:1-9
**Status:** Compliant
**Notes:** Uses `sed -i '1i ...'` to prepend export PATH line to both rc files.

#### FR-004: PATH prepend after tool installations, before user config
**Implementation:** Template ordering (05-shell-finalize runs after 03-mandatory-stack)
**Status:** Compliant
**Notes:** The `sed -i '1i'` inserts at line 1 of rc files, ensuring paths appear before any user config in those files.

#### FR-005: Deduplicate paths
**Implementation:** cc-deck/internal/build/containerfile.go:43-46
**Status:** Compliant
**Notes:** Uses `seen` map to skip duplicate resolved paths. Tested with cargo+rust both mapping to same path.

#### FR-006: Home directory placeholder resolution
**Implementation:** cc-deck/internal/build/containerfile.go:42
**Status:** Compliant
**Notes:** `strings.ReplaceAll(pathTmpl, "{home}", homeDir)` resolves to `/sandbox` for openshell, `/home/dev` for container.

#### FR-007: No entries for standard PATH tools
**Implementation:** Registry only contains `/usr/local/go/bin` and `{home}/.cargo/bin`
**Status:** Compliant
**Notes:** Standard paths like `/usr/local/bin` are not in the registry. Node.js test confirms no match.

#### FR-008: ENV PATH lines unchanged
**Implementation:** No changes to 03-mandatory-stack.tmpl
**Status:** Compliant
**Notes:** Verified via `git diff main` showing no changes to ENV PATH in any template.

#### FR-009: Works for both openshell and container
**Implementation:** cc-deck/internal/build/containerfile.go:121,133
**Status:** Compliant
**Notes:** `ContainerDataForTarget` calls `ResolveToolPaths` for both targets. Template has no target gate.

#### FR-010: Does not affect SSH targets
**Implementation:** SSH is not handled in containerfile.go
**Status:** Compliant
**Notes:** SSH targets do not go through Containerfile generation.

### Edge Cases

#### No matching registry entries
**Status:** Compliant
**Evidence:** `TestResolveToolPaths_NoMatches` (Node.js returns empty), `TestRenderSnippets_WithoutToolPaths` (no PATH block generated)

#### Multiple tools map to same path (deduplication)
**Status:** Compliant
**Evidence:** `TestResolveToolPaths_Deduplication` (cargo+rust -> single path)

#### Home directory varies between targets
**Status:** Compliant
**Evidence:** `TestResolveToolPaths_HomeDirSubstitution_Container` (/home/dev) and `_OpenShell` (/sandbox)

#### RC file doesn't exist yet
**Status:** Compliant
**Evidence:** Template uses `if [ -f "$RC" ]` guard before `sed`

### Success Criteria

#### SC-001: Tools accessible in interactive shell
**Status:** Compliant
**Evidence:** PATH prepended to line 1 of rc files via `sed -i '1i ...'`

#### SC-002: Adding new tool = one registry entry
**Status:** Compliant
**Evidence:** Registry is a simple Go map. No template changes needed for new entries.

#### SC-003: Zero regression for empty registries
**Status:** Compliant
**Evidence:** Template conditional on `.ToolPaths` non-empty. Tests confirm no PATH block for nil/empty.

#### SC-004: Single additional Containerfile layer
**Status:** Compliant
**Evidence:** Single `RUN` step in template with `for` loop over rc files.

### Extra Features (Not in Spec)

#### Word-boundary matching (instead of substring matching)
**Location:** cc-deck/internal/build/containerfile.go:58-76
**Description:** The spec clarification says "case-insensitive substring matching" but the implementation uses word-boundary matching via `containsWord()`. This prevents "cargo" from matching the "go" registry key.
**Assessment:** Beneficial improvement. Prevents false positives.
**Recommendation:** Update spec clarification to reflect word-boundary matching (spec evolution candidate).

#### Sorted output
**Location:** cc-deck/internal/build/containerfile.go:53
**Description:** Results are sorted alphabetically for deterministic output.
**Assessment:** Helpful addition for reproducible builds.
**Recommendation:** No change needed.

## Deep Review Report

### Review Agents Summary

| Agent                   | Found | Fixed | Remaining | Status    |
|-------------------------|-------|-------|-----------|-----------|
| Correctness             |     0 |     0 |         0 | completed |
| Architecture & Idioms   |     1 |     1 |         0 | completed |
| Security                |     0 |     0 |         0 | completed |
| Production Readiness    |     0 |     0 |         0 | completed |
| Test Quality            |     1 |     0 |         1 | completed |
| CodeRabbit (external)   |     - |     - |         - | skipped (disabled) |
| Copilot (external)      |     - |     - |         - | skipped (disabled) |
| Test Suite (regression) |     0 |     0 |         0 | passed    |
|-------------------------|-------|-------|-----------|-----------|
| Total                   |     2 |     1 |         1 |           |

### Gate Outcome: PASS

No Critical or Important findings. 1 Minor finding remaining (missing word-boundary regression test).

### Key Fixes Applied

1. Fixed misleading godoc comment on `toolPathRegistry`: changed "substrings" to "word-boundary matching" to accurately describe the `containsWord` behavior (architecture-agent)

### Remaining Findings (0 Critical/Important)

1 Minor finding: missing test for substring-but-not-word-boundary rejection (e.g., "Django" should not match "go"). Not blocking.

### Post-Fix Spec Coverage

All 10 functional requirements verified. No requirements dropped during fix loop.

### Test Suite Results

| Round | Test Command | Exit Code | Failures | Status |
|-------|-------------|-----------|----------|--------|
| 1     | go test ./internal/build/ | 0 | 0 | passed |

## Code Quality Notes

- Clean, focused implementation with minimal changes to existing code
- Good use of existing patterns (extending `ContainerfileData` struct, adding to `ContainerDataForTarget`)
- Comprehensive test coverage (12 unit tests for `ResolveToolPaths`, 4 template rendering tests)
- Template uses defensive `if [ -f "$RC" ]` guard for robustness
- Deterministic output via sort ensures reproducible builds

## Conclusion

Implementation is fully compliant with the specification (100% compliance score). The code is clean, well-tested, and follows existing codebase patterns. The single deviation from the spec (word-boundary matching instead of substring matching) is a beneficial improvement that should be reflected in the spec via evolution.
