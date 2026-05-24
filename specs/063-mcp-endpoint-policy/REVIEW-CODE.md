# Code Review: MCP Endpoint Policy Integration

**Spec:** specs/063-mcp-endpoint-policy/spec.md
**Date:** 2026-05-24
**Reviewer:** Claude (speckit.spex-gates.review-code)

## Compliance Summary

**Overall Score: 100%**

- Functional Requirements: 15/15 (100%)
- Error Handling: 2/2 (100%)
- Edge Cases: 4/4 (100%)
- Non-Functional: 2/2 (100%)

## Detailed Review

### Functional Requirements

#### FR-001: Optional endpoint field on MCP entries
**Implementation:** cc-deck/internal/build/manifest.go:107
**Status:** Compliant
**Notes:** `Endpoint string yaml:"endpoint,omitempty"` added to MCPEntry struct

#### FR-002: Policy entry for each MCP with non-empty endpoint
**Implementation:** cc-deck/internal/build/policy.go:257-279
**Status:** Compliant
**Notes:** Loop iterates manifest.MCP, skips empty endpoints, generates NetworkPolicy entries

#### FR-003: Key as mcp_slugified_name
**Implementation:** cc-deck/internal/build/policy.go:272
**Status:** Compliant
**Notes:** `key := "mcp_" + slugifyMCPName(mcp.Name)`

#### FR-004: Endpoint host and port as single endpoint entry
**Implementation:** cc-deck/internal/build/policy.go:273-277
**Status:** Compliant
**Notes:** `Endpoints: []PolicyEndpoint{{Host: host, Port: port}}`

#### FR-005: Claude Code binary paths in each MCP entry
**Implementation:** cc-deck/internal/build/policy.go:278
**Status:** Compliant
**Notes:** `Binaries: claudeCodeBinaries` where claudeCodeBinaries is obtained from the claude_code component

#### FR-006: No policy entry for MCP without endpoint
**Implementation:** cc-deck/internal/build/policy.go:259-261
**Status:** Compliant
**Notes:** `if mcp.Endpoint == "" { continue }`

#### FR-007: Capture extracts endpoints from HTTP/SSE servers
**Implementation:** cc-deck/internal/build/commands/cc-deck.capture.md:689
**Status:** Compliant
**Notes:** Parses `url` field, extracts host:port. Default ports (443 for HTTPS, 80 for HTTP) when no explicit port.

#### FR-008: Capture extracts endpoints from stdio servers with mcp-remote
**Implementation:** cc-deck/internal/build/commands/cc-deck.capture.md:690
**Status:** Compliant
**Notes:** Scans `args` array for HTTPS URLs, parses first match to extract host:port

#### FR-009: Capture presents endpoints for user confirmation
**Implementation:** cc-deck/internal/build/commands/cc-deck.capture.md:696-731
**Status:** Compliant
**Notes:** Shows endpoints in categorized list with AskUserQuestion for confirmation

#### FR-010: pkg_node binary augmentation
**Implementation:** cc-deck/internal/build/policy.go:294-308
**Status:** Compliant
**Notes:** When hasMCPEndpoints is true and pkg_node exists, appends claude_code binaries with deduplication

#### FR-011: Backward compatibility for existing manifests
**Implementation:** cc-deck/internal/build/manifest.go:107, policy.go:257
**Status:** Compliant
**Notes:** `omitempty` tag ensures no YAML output for empty endpoint. Nil check on claudeCodeBinaries prevents nil dereference.

#### FR-012: OpenShell-only, container/SSH unaffected
**Implementation:** MCP endpoint code is exclusively in policy.go
**Status:** Compliant
**Notes:** No MCP endpoint references in containerfile.go or SSH code. Verified via codebase search.

#### FR-013: Slugification rules
**Implementation:** cc-deck/internal/build/policy.go:495-501
**Status:** Compliant
**Notes:** `nonAlphanumRe` regex replaces all non-alphanumeric chars with underscores, trims leading/trailing underscores, lowercases result

#### FR-014: Missing claude_code component graceful skip
**Implementation:** cc-deck/internal/build/policy.go:281-289
**Status:** Compliant
**Notes:** Checks if any MCP entries have endpoints before issuing warning. No error returned.

#### FR-015: Malformed endpoint skip with warning
**Implementation:** cc-deck/internal/build/policy.go:263-266
**Status:** Compliant
**Notes:** parseMCPEndpoint returns error, warning printed, `continue` skips the entry

### Error Handling

#### Missing port in endpoint
**Implemented:** Yes
**Location:** cc-deck/internal/build/policy.go:507-509
**Response:** Returns error "missing port in endpoint"
**Status:** Compliant

#### Non-numeric or out-of-range port
**Implemented:** Yes
**Location:** cc-deck/internal/build/policy.go:514-521
**Response:** Returns descriptive error, entry skipped with warning
**Status:** Compliant

### Edge Cases

#### No explicit port
**Handling:** parseMCPEndpoint returns error, entry skipped with warning
**Status:** Compliant (per spec: "validation error and the entry should be skipped with a warning")

#### Malformed endpoint string
**Handling:** parseMCPEndpoint validates format, skips with warning on any error
**Status:** Compliant

#### Two MCP servers with same host, different ports
**Handling:** Each gets unique key based on server name, not host. Separate policy entries generated.
**Status:** Compliant (implicit via key derivation, no dedicated test)

#### No MCP entries in manifest
**Handling:** Policy assembly proceeds normally, no MCP-related entries generated
**Status:** Compliant (tested by TestAssemblePolicy_MCPNoEntriesBackwardCompatible)

### Extra Features (Not in Spec)

No extra features found. All code maps to specific spec requirements.

## Code Quality Notes

- Code follows existing patterns in the file (credential processing, domain group processing)
- Warning output uses `fmt.Printf`/`fmt.Println` consistent with rest of file
- Regex compiled at package level (correct Go practice)
- Helper functions are well-scoped and tested independently
- 14 new test functions provide thorough coverage of all paths

## Deep Review Report

### Gate Outcome: PASS

Deep review completed with 5 internal review agents. No external tools were invoked (CodeRabbit and Copilot disabled by caller).

### Review Agents

| Agent                   | Found | Fixed | Remaining | Status    |
|-------------------------|-------|-------|-----------|-----------|
| Correctness             |     0 |     0 |         0 | completed |
| Architecture & Idioms   |     0 |     0 |         0 | completed |
| Security                |     0 |     0 |         0 | completed |
| Production Readiness    |     0 |     0 |         0 | completed |
| Test Quality            |     1 |     0 |         1 | completed |
| CodeRabbit (external)   |     - |     - |         - | skipped (disabled) |
| Copilot (external)      |     - |     - |         - | skipped (disabled) |
| Test Suite (regression) |     0 |     0 |         0 | passed (build pkg) |
|-------------------------|-------|-------|-----------|-----------|
| Total                   |     1 |     0 |         1 |           |

### Findings Detail

**FINDING-1 (Minor, test-quality, confidence 72):**
Missing dedicated test for spec edge case "two MCP servers sharing same endpoint host but different ports." The behavior is implicitly correct because policy keys are derived from server names, not hosts. Risk of regression is low.

### Post-Fix Spec Coverage

No fix loop was executed. All 15 functional requirements verified:

| Requirement | Implementation | Status |
|-------------|---------------|--------|
| FR-001 | manifest.go:107 | PASS |
| FR-002 | policy.go:257-279 | PASS |
| FR-003 | policy.go:272 | PASS |
| FR-004 | policy.go:273-277 | PASS |
| FR-005 | policy.go:278 | PASS |
| FR-006 | policy.go:259-261 | PASS |
| FR-007 | cc-deck.capture.md:689 | PASS |
| FR-008 | cc-deck.capture.md:690 | PASS |
| FR-009 | cc-deck.capture.md:696-731 | PASS |
| FR-010 | policy.go:294-308 | PASS |
| FR-011 | manifest.go:107 | PASS |
| FR-012 | No refs in container/SSH | PASS |
| FR-013 | policy.go:495-501 | PASS |
| FR-014 | policy.go:281-289 | PASS |
| FR-015 | policy.go:263-266 | PASS |

### Test Suite Results

| Package | Status | Notes |
|---------|--------|-------|
| internal/build | PASS (0.611s) | All 14 new MCP tests pass |
| internal/cmd | FAIL (pre-existing) | Compose smoke tests need podman |
| internal/ws | FAIL (pre-existing) | Unrelated to this feature |

## Recommendations

### Optional Improvements
- [ ] Add a test for the edge case where two MCP servers share the same host but have different ports (FINDING-1)

## Conclusion

The implementation achieves 100% spec compliance across all 15 functional requirements, all error handling cases, and all edge cases. Code quality is high, following existing patterns. The single Minor finding (missing edge case test) does not affect correctness or block the gate. The feature is ready for verification.
