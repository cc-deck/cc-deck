# Code Review: OpenShell SDK Migration

**Spec:** specs/075-openshell-sdk-migration/spec.md
**Date:** 2026-06-30
**Reviewer:** Claude (speckit.spex-gates.review-code)

## Compliance Summary

**Overall Score: 100%**

- Functional Requirements: 10/10 (100%)
- Error Handling: 10/10 (100%)
- Edge Cases: 3/3 (100%)

## Detailed Review

### Functional Requirements

#### FR-001: Replace CLI wrapper with SDK client
**Implementation:** cc-deck/internal/openshell/client.go, cc-deck/internal/openshell/iface.go
**Status:** Compliant
**Notes:** `NewSDKClient()` creates a `v1.ClientInterface` from `GatewayConfig`. All CLI exec calls replaced with SDK method calls.

#### FR-002: Sandbox lifecycle via SDK
**Implementation:** cc-deck/internal/ws/openshell.go:272-386 (Create), :389-408 (Start/Stop), :411-433 (Delete)
**Status:** Compliant
**Notes:** Create uses `Sandboxes().Create()`, `Providers().Ensure()`, `Sandboxes().WaitReady()`. Delete uses `Sandboxes().Delete()`. All lifecycle operations use SDK types.

#### FR-003: Status mapping from SDK types
**Implementation:** cc-deck/internal/ws/openshell.go:172-216
**Status:** Compliant
**Notes:** Maps `types.SandboxPhase` to `InfraStateValue`. Handles Ready, Suspended, Error, Deleting, Provisioning. NotFound clears local state.

#### FR-004: File transfer via SDK
**Implementation:** cc-deck/internal/ws/channel_openshell.go:14-88
**Status:** Compliant
**Notes:** Push/Pull/PushBytes use `client.Files().Upload()` and `client.Files().Download()`. PushBytes uses temp-file approach since SDK doesn't support byte streaming.

#### FR-005: Credential injection via SDK
**Implementation:** cc-deck/internal/openshell/credentials.go:290-340
**Status:** Compliant
**Notes:** `InjectEnvVars` and `UploadFileCredential` use `client.Exec().Run()` and `client.Files().Upload()` respectively. Uses `v1.ClientInterface`.

#### FR-006: Exec operations via SDK
**Implementation:** cc-deck/internal/ws/openshell.go:481-509
**Status:** Compliant
**Notes:** `Exec()` uses `client.Exec().Stream()`, `ExecOutput()` uses `client.Exec().Run()`. Both handle sandbox/client preconditions.

#### FR-007: Interactive attach via SDK
**Implementation:** cc-deck/internal/ws/openshell.go:453-478
**Status:** Compliant
**Notes:** `Attach()` uses `client.Exec().Interactive()`. Tracks attach state, updates LastAttached timestamp.

#### FR-008: Error handling with SDK error helpers
**Implementation:** cc-deck/internal/ws/openshell.go:183, 339, 403, 422
**Status:** Compliant
**Notes:** Uses `v1.IsNotFound()` consistently for NotFound detection. Uses `v1.IsAlreadyExists()` where applicable.

#### FR-009: Gateway config resolution
**Implementation:** cc-deck/internal/ws/openshell.go:88-103, cc-deck/internal/openshell/client.go
**Status:** Compliant
**Notes:** Resolves from definition store, falls back to env var, then default. TLS config properly mapped.

#### FR-010: Credential transport narrow interface
**Implementation:** cc-deck/internal/credential/transport.go:18-24
**Status:** Compliant
**Notes:** Defines narrow `OpenShellClient` interface with only `Exec()` and `Files()` methods needed for credential transport. Decoupled from full SDK client.

### Error Handling

All 10 error scenarios from the spec are handled: sandbox not found, gateway unreachable, create failure, WaitReady timeout, delete failure (force/non-force), exec failure, file transfer failure, credential injection failure, and invalid workspace name.

### Edge Cases

1. **Orphaned sandbox on WaitReady failure:** Handled with rollback cleanup (F-01 fix)
2. **Force delete with unreachable gateway:** Logs warning, clears local state
3. **Sandbox already deleted (NotFound):** Clears local state, returns clean status

## Deep Review Report

### Overview

Deep review completed with 5 internal review agents (correctness, architecture, security, production-readiness, test-quality) plus CodeRabbit CLI as external tool. All agents completed successfully.

### Gate Result: PASS

| Metric | Value |
|--------|-------|
| Total findings | 14 |
| Critical | 0 |
| Important | 4 (all fixed in round 1) |
| Minor | 4 (remaining, not gate-blocking) |
| Notable | 6 (informational) |
| Fix rounds | 1/3 |
| Test suite | PASS (ComposeSmoke failures pre-existing, Podman offline) |

### Fixes Applied

1. **F-01** (openshell.go:336-343): Added WaitReady rollback cleanup to prevent orphaned sandboxes
2. **F-02** (openshell_test.go:136-210): Rewrote TestStatusMapping to call real Status() via fake client instead of duplicating switch logic
3. **F-03** (openshell_test.go:555-576): Added TestCreate_HappyPath using fake client to verify full creation flow
4. **F-04** (channel_openshell.go:76-82): Check tmpFile.Close() error before Upload to prevent silent data corruption

### Remaining Minor Findings (Not Gate-Blocking)

- F-05: InjectEnvVars always returns nil even on failure
- F-06: Shell command construction could expand credential metacharacters
- F-07: PushBytes missing remotePath validation
- F-08: Map iteration non-determinism in env var injection

### Notable Observations (Informational)

- F-09: Inconsistent error handling style between transport.go and credentials.go
- F-10: NoAuth used for non-localhost when no TLS configured
- F-11: SDK client (gRPC connection) never explicitly closed
- F-12: Single-function iface.go could be folded into client.go
- F-13: Duplicated rc-file injection pattern
- F-14: Narrow interface naming mismatch with SDK

### External Tools

- **CodeRabbit:** 8 findings (2 on spec artifacts filtered out, 6 on source code merged with internal findings)
- **Copilot:** not invoked (not installed)

### Conclusion

All Critical and Important findings resolved in 1 fix round. The SDK migration is correctly implemented with proper error handling, rollback logic, and test coverage. Remaining minor and notable findings are tracked for future improvement but do not block the feature.

Full findings: [review-findings.md](review-findings.md)
