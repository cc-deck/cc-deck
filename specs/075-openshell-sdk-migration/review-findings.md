# Deep Review Findings

**Date:** 2026-06-30
**Branch:** 075-openshell-sdk-migration
**Rounds:** 1
**Gate Outcome:** PASS
**Invocation:** quality-gate

## Summary

| Severity | Found | Fixed | Remaining |
|----------|-------|-------|-----------|
| Critical | 0 | 0 | 0 |
| Important | 4 | 4 | 0 |
| Minor | 4 | - | 4 |
| Notable | 6 | - | 6 |
| **Total** | **14** | **4** | **10** |

**Agents completed:** 5/5 (+ 1 external tool: CodeRabbit)
**Agents failed:** none

## Findings

### F-01
- **Severity:** Important
- **Confidence:** 90
- **File:** cc-deck/internal/ws/openshell.go:330-343
- **Category:** correctness
- **Source:** correctness-agent (also reported by: coderabbit)
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**
When `WaitReady` fails after `Create` succeeds, the sandbox is left orphaned on the gateway. The original code returned the error without cleaning up the created sandbox, and `sandboxID` remained set despite the sandbox being in an unusable state.

**Why this matters:**
Orphaned sandboxes consume gateway resources and cannot be reclaimed by the user since `sandboxID` is set but the workspace is in a broken state. Repeated failures accumulate orphans.

**How it was resolved:**
Added rollback logic: on `WaitReady` failure, a cleanup `Delete` call is issued with a 30s timeout context. The `sandboxID` is reset to `""` regardless, so the workspace returns to a clean pre-create state.

### F-02
- **Severity:** Important
- **Confidence:** 85
- **File:** cc-deck/internal/ws/openshell_test.go:136-210
- **Category:** test-quality
- **Source:** tests-agent
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**
`TestStatusMapping` duplicated the production switch logic inline instead of calling the real `Status()` method. The test verified its own duplicated logic, not the production code path.

**Why this matters:**
If the production switch statement changes (e.g., adding a new phase), the test would still pass with its stale copy, giving false confidence. Tests that don't exercise production code are worse than no tests.

**How it was resolved:**
Rewrote the test to use `fake.NewClient()` with a `phaseOverrideClient` wrapper that overrides the phase returned by `Sandboxes().Get()`. Each test case seeds a sandbox, calls the real `w.Status(ctx)`, and asserts the returned `InfraState` and `Message`. Added `Deleting` and `NotFound` edge case tests. A `newOpenShellWS` helper pre-triggers `sync.Once` so `ensureClient()` doesn't overwrite the injected fake.

### F-03
- **Severity:** Important
- **Confidence:** 80
- **File:** cc-deck/internal/ws/openshell_test.go:555-576
- **Category:** test-quality
- **Source:** tests-agent
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**
No happy-path test for `Create()`. All existing Create-related tests only covered error paths (no sandbox, no gateway). The main creation flow with sandbox provisioning, `WaitReady`, and state persistence was untested.

**Why this matters:**
The Create path is the most complex method in the workspace layer (credential resolution, provider creation, sandbox creation, readiness wait, state persistence). Without a happy-path test, regressions in this flow would only be caught in integration environments.

**How it was resolved:**
Added `TestCreate_HappyPath` using `fake.NewClient()`. Verifies that after `Create()`: the sandbox exists in the fake store with `SandboxReady` phase, the workspace instance is persisted to the state store with correct type, infra state, sandbox ID, and gateway address.

### F-04
- **Severity:** Important
- **Confidence:** 90
- **File:** cc-deck/internal/ws/channel_openshell.go:76-82
- **Category:** correctness
- **Source:** correctness-agent (also reported by: coderabbit)
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**
`PushBytes` ignored the error from `tmpFile.Close()`. If the close failed (e.g., filesystem full, flush error), incomplete data would be uploaded silently.

**Why this matters:**
`Close()` on a writable file flushes buffered data to disk. Ignoring its error means partial writes could be uploaded to the sandbox, causing silent data corruption in transferred files.

**How it was resolved:**
Split the write-close sequence: on write error, close is called (error ignored since the write error takes precedence) and a channel error is returned. On successful write, close error is now checked and returned before proceeding to upload.

### F-05
- **Severity:** Minor
- **File:** cc-deck/internal/openshell/credentials.go:305-316
- **Category:** correctness
- **Source:** correctness-agent
- **Round found:** 1
- **Resolution:** remaining (minor, not gate-blocking)

**Description:** `InjectEnvVars` always returns nil even when both rc-file write attempts fail. The function logs warnings but never surfaces errors to the caller.

### F-06
- **Severity:** Minor
- **File:** cc-deck/internal/openshell/credentials.go:307-310
- **Category:** security
- **Source:** security-agent (also reported by: coderabbit)
- **Round found:** 1
- **Resolution:** remaining (minor, not gate-blocking)

**Description:** Shell command construction via `fmt.Sprintf("echo %q ...")` inside `Exec().Run()` could expand credential text unexpectedly if values contain shell metacharacters.

### F-07
- **Severity:** Minor
- **File:** cc-deck/internal/ws/channel_openshell.go:82
- **Category:** correctness
- **Source:** coderabbit
- **Round found:** 1
- **Resolution:** remaining (minor, not gate-blocking)

**Description:** `PushBytes` does not validate `remotePath` before passing it to `Upload`. An empty remote path could produce undefined behavior on the gateway.

### F-08
- **Severity:** Minor
- **File:** cc-deck/internal/openshell/credentials.go:308
- **Category:** production-readiness
- **Source:** production-agent
- **Round found:** 1
- **Resolution:** remaining (minor, not gate-blocking)

**Description:** Map iteration order in `InjectEnvVars` is non-deterministic. While functionally correct, it makes debugging harder since rc-file entries appear in random order across runs.

### F-09
- **Severity:** Notable
- **File:** cc-deck/internal/credential/transport.go, cc-deck/internal/openshell/credentials.go
- **Category:** architecture
- **Source:** architecture-agent
- **Round found:** 1
- **Resolution:** remaining (notable, informational)

**Description:** Error handling style differs between `transport.go` (structured error types) and `credentials.go` (raw error wrapping). Not a bug, but a consistency gap.

### F-10
- **Severity:** Notable
- **File:** cc-deck/internal/openshell/client.go:45-48
- **Category:** security
- **Source:** security-agent
- **Round found:** 1
- **Resolution:** remaining (notable, informational)

**Description:** `NoAuth()` is used for all non-localhost connections when no TLS cert paths are configured. This is acceptable for development but should be revisited for production deployments.

### F-11
- **Severity:** Notable
- **File:** cc-deck/internal/ws/openshell.go:54
- **Category:** production-readiness
- **Source:** production-agent
- **Round found:** 1
- **Resolution:** remaining (notable, informational)

**Description:** The SDK client (gRPC connection) is never explicitly closed. Relies on process exit for cleanup.

### F-12
- **Severity:** Notable
- **File:** cc-deck/internal/openshell/iface.go
- **Category:** architecture
- **Source:** architecture-agent
- **Round found:** 1
- **Resolution:** remaining (notable, informational)

**Description:** Single-function file containing only the `NewSDKClient` factory. Could be folded into `client.go`.

### F-13
- **Severity:** Notable
- **File:** cc-deck/internal/openshell/credentials.go, cc-deck/internal/credential/transport.go
- **Category:** architecture
- **Source:** architecture-agent
- **Round found:** 1
- **Resolution:** remaining (notable, informational)

**Description:** Duplicated rc-file injection pattern between credentials.go (InjectEnvVars) and transport.go (credential transport). Both write to `.bashrc`/`.zshrc` via exec.

### F-14
- **Severity:** Notable
- **File:** cc-deck/internal/credential/transport.go:222-225
- **Category:** architecture
- **Source:** architecture-agent
- **Round found:** 1
- **Resolution:** remaining (notable, informational)

**Description:** The narrow `OpenShellClient` interface in transport.go uses different naming than the SDK's `ClientInterface`. Functional but could be confusing.
