# Deep Review Findings

**Date:** 2026-05-24
**Branch:** 063-mcp-endpoint-policy
**Rounds:** 0
**Gate Outcome:** PASS
**Invocation:** quality-gate

## Summary

| Severity | Found | Fixed | Remaining |
|----------|-------|-------|-----------|
| Critical | 0 | 0 | 0 |
| Important | 0 | 0 | 0 |
| Minor | 1 | - | 1 |
| **Total** | **1** | **0** | **1** |

**Agents completed:** 5/5 (+ 0 external tools)
**Agents failed:** none

## Findings

### FINDING-1
- **Severity:** Minor
- **Confidence:** 72
- **File:** cc-deck/internal/build/policy_test.go
- **Lines:** N/A (missing test)
- **Category:** test-quality
- **Source:** test-quality-agent
- **Round found:** 1
- **Resolution:** remaining (Minor, does not block gate)

**What is wrong:**
The spec's edge case section states: "What happens when two MCP servers share the same endpoint host but different ports? Each should produce its own separate policy entry with a unique key." There is no dedicated test verifying this behavior.

**Why this matters:**
While the code naturally handles this case because policy keys are derived from the MCP server name (not the host), the spec explicitly calls it out as an edge case. A regression test would document the expected behavior and catch future changes that might break this invariant.

**How it was resolved:**
Not fixed. This is a Minor finding that does not block the gate. The behavior is implicitly covered by the key-derivation logic (keys use server names, not hosts), so the risk of regression is low.

## Post-Fix Spec Coverage

No fix loop was executed. All 15 functional requirements verified during Stage 1 spec compliance check:

| Requirement | Implementation | Status |
|-------------|---------------|--------|
| FR-001: Optional endpoint field | manifest.go:107 | PASS |
| FR-002: Policy entry for MCP with endpoint | policy.go:257-279 | PASS |
| FR-003: Key as mcp_slugified_name | policy.go:272 | PASS |
| FR-004: Endpoint host and port | policy.go:273-277 | PASS |
| FR-005: Claude Code binary paths | policy.go:278 | PASS |
| FR-006: No entry without endpoint | policy.go:259-261 | PASS |
| FR-007: Capture extracts HTTP/SSE endpoints | cc-deck.capture.md:689 | PASS |
| FR-008: Capture extracts mcp-remote endpoints | cc-deck.capture.md:690 | PASS |
| FR-009: Capture presents for confirmation | cc-deck.capture.md:696-731 | PASS |
| FR-010: pkg_node binary augmentation | policy.go:294-308 | PASS |
| FR-011: Backward compatibility | manifest.go:107 (omitempty) | PASS |
| FR-012: OpenShell-only | No MCP refs in containerfile/SSH | PASS |
| FR-013: Slugification rules | policy.go:495-501 | PASS |
| FR-014: Missing claude_code graceful skip | policy.go:281-289 | PASS |
| FR-015: Malformed endpoint skip with warning | policy.go:263-266 | PASS |

## Test Suite Results

Test suite (`make test`) was run prior to review. The `internal/build` package (containing all MCP endpoint policy code) passed cleanly. Pre-existing failures in `internal/cmd` (compose smoke tests requiring podman) and `internal/ws` are unrelated to this feature.

| Package | Status | Notes |
|---------|--------|-------|
| internal/build | PASS (0.611s) | All MCP policy tests pass |
| internal/cmd | FAIL (pre-existing) | Compose smoke tests need podman |
| internal/ws | FAIL (pre-existing) | Unrelated to this feature |

## Remaining Findings

One Minor finding remains (FINDING-1: missing edge case test). This does not block the gate.
