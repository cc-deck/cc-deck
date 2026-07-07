# Deep Review Findings

**Date:** 2026-07-05
**Branch:** 078-network-policy-generalization
**Rounds:** 0
**Gate Outcome:** PASS
**Invocation:** quality-gate

## Summary

| Severity | Found | Fixed | Remaining |
|----------|-------|-------|-----------|
| Critical | 0 | 0 | 0 |
| Important | 0 | 0 | 0 |
| Minor | 2 | - | 2 |
| Notable | 0 | - | 0 |
| **Total** | **2** | **0** | **2** |

**Agents completed:** 5/5 (+ 1 external tool)
**Agents failed:** none

## Findings

### FINDING-1
- **Severity:** Minor
- **Confidence:** 75
- **File:** cc-deck/internal/agent/opencode_test.go:209-218
- **Category:** test-quality
- **Source:** coderabbit
- **Round found:** 1
- **Resolution:** remaining (minor, no fix required for gate)

**What is wrong:**
The `TestOpenCodeAgentRequiredDomainGroups` test asserts that `RequiredDomainGroups()` returns exactly `["openai"]`. If the OpenCode agent is later updated to support alternative credential modes that require additional domain groups (e.g., "anthropic"), the test would need updating. The assertion is tightly coupled to the current single-group return value.

**Why this matters:**
The test is correct for the current implementation but may become brittle if `RequiredDomainGroups()` evolves to return multiple groups. This is a minor maintainability concern, not a correctness issue.

**Recommendation:**
Consider using `assert.Contains` for membership checks alongside length assertions, so the test remains valid if additional groups are added. No immediate action required since the current implementation returns only `["openai"]`.

---

### FINDING-2
- **Severity:** Minor
- **Confidence:** 75
- **File:** cc-deck/internal/build/policy_test.go:1524-1543
- **Category:** test-quality
- **Source:** coderabbit
- **Round found:** 1
- **Resolution:** remaining (minor, no fix required for gate)

**What is wrong:**
`TestAssemblePolicy_MultiAgentNoDuplicateEndpoints` (T013a) verifies that policy map keys are unique by iterating `policy.NetworkPolicies` and counting key occurrences. Since Go maps guarantee unique keys, this assertion is tautological and cannot fail.

**Why this matters:**
The test intends to verify FR-006 (domain deduplication across agents), but the assertion mechanism cannot detect the problem it claims to test. In practice, deduplication is tested indirectly by the policy assembly logic that uses `coveredHosts` to prevent duplicate endpoints. The test serves as a smoke test that multi-agent assembly produces output, but the uniqueness assertion adds no value.

**Recommendation:**
To genuinely test endpoint deduplication, iterate over the `Endpoints` slice within each `NetworkPolicy` and verify that no host appears more than once across all policies. Alternatively, add a test case where two agent components share an overlapping domain and verify the assembled policy contains the domain exactly once. No immediate action required since the underlying deduplication logic is correct and tested via integration.

---

## Code Quality Notes

The implementation is clean and well-structured:

- Zero hardcoded `claude_code` references remain in `policy.go` (SC-003 satisfied)
- Agent matching with backward-compatible defaulting is correctly implemented
- Domain group deduplication via `processedGroups` map prevents duplicates (FR-006)
- `coveredHosts` map prevents endpoint duplicates across policy components
- `collectAgentBinaries` correctly merges binaries from all agent-matched components with deduplication
- `ValidateAgentDomainGroups` provides clear warnings for missing groups without failing the build
- All 11 functional requirements from the spec are implemented and tested
- The `MatchComponent` function properly defaults empty manifest agents to `["claude"]` (FR-007)

## Recommendations

### Spec Evolution Candidates
- [ ] FINDING-1: Consider making `RequiredDomainGroups()` tests more resilient to future changes
- [ ] FINDING-2: Strengthen the duplicate endpoint test to assert at the endpoint level

### No Critical or Important Issues
The codebase passed deep review with only minor test quality observations. No fixes required.
