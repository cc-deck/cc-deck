# Deep Review Findings

**Date:** 2026-07-08
**Branch:** 079-credential-transport
**Rounds:** 0
**Gate Outcome:** PASS
**Invocation:** quality-gate

## Summary

| Severity | Found | Fixed | Remaining |
|----------|-------|-------|-----------|
| Critical | 0 | 0 | 0 |
| Important | 0 | 0 | 0 |
| Minor | 6 | - | 6 |
| Notable | 3 | - | 3 |
| **Total** | **9** | **0** | **9** |

**Agents completed:** 5/5 (+ 1 external tool)
**Agents failed:** none

## Stage 1: Spec Compliance

**Score: 100%** - All functional requirements (FR-001 through FR-008), success criteria (SC-001 through SC-004), and non-functional requirements verified.

## Findings

### FINDING-1
- **Severity:** Minor
- **Confidence:** 90
- **File:** cc-deck/internal/ws/openshell.go:244
- **Category:** architecture
- **Source:** architecture-agent
- **Round found:** 1
- **Resolution:** remaining (Minor, not in fix scope)

**What is wrong:**
`resolveAgentName` takes a `*WorkspaceDefinition` parameter it ignores, always returning `"claude"`. The caller block (lines 339-344) performs a definition lookup solely to pass to this function.

**Why this matters:**
Dead parameter creates misleading API surface. The function could be replaced with `agentName := "claude"`.

### FINDING-2
- **Severity:** Minor
- **Confidence:** 85
- **File:** cc-deck/internal/ssh/credentials.go:1
- **Category:** architecture
- **Source:** architecture-agent
- **Round found:** 1
- **Resolution:** remaining (Minor, not in fix scope)

**What is wrong:**
`ssh/credentials.go` and `ssh/credentials_test.go` are empty stub files (package declaration only, 12 bytes each). All credential logic migrated to `internal/credential`.

**Why this matters:**
Empty files add noise. The `ssh` package has other active files (`client.go`, `probe.go`) so the package survives deletion.

### FINDING-3
- **Severity:** Minor
- **Confidence:** 80
- **File:** cc-deck/internal/ws/openshell.go:285
- **Category:** architecture
- **Source:** architecture-agent
- **Round found:** 1
- **Resolution:** remaining (Minor, not in fix scope)

**What is wrong:**
`mapToOpenShellProvider` switch handles "api" and "vertex" but has no default case. Unrecognized spec names (e.g., future modes) silently return empty strings with no log output. Similarly, `selectCredentialMode` returns false without logging when explicit auth doesn't match available modes.

**Why this matters:**
Silent failures make debugging harder when new credential modes are added.

### FINDING-4
- **Severity:** Minor
- **Confidence:** 75
- **File:** cc-deck/internal/cmd/ws.go:25
- **Category:** architecture
- **Source:** architecture-agent
- **Round found:** 1
- **Resolution:** remaining (Minor, not in fix scope)

**What is wrong:**
The `sshPkg` import alias was needed when `ssh.BuildCredentialSet` coexisted with other ssh references. Now the only usage is `sshPkg.NewClient(...)`, so the alias can be dropped.

**Why this matters:**
Cosmetic cleanup. Non-standard alias adds minor cognitive overhead.

### FINDING-5
- **Severity:** Minor
- **Confidence:** 80
- **File:** cc-deck/internal/openshell/credentials_test.go:73
- **Category:** test-quality
- **Source:** test-quality-agent
- **Round found:** 1
- **Resolution:** remaining (Minor, not in fix scope)

**What is wrong:**
`TestOpenShellClientAdapter_ExecRun` has a success test but no error-propagation test. `FileUpload` has both. If ExecRun were refactored to swallow errors, no test would catch it.

**Why this matters:**
Asymmetric test coverage for symmetric adapter methods.

### FINDING-6
- **Severity:** Minor
- **Confidence:** 78
- **File:** cc-deck/internal/ws/openshell_test.go:430-467
- **Category:** test-quality
- **Source:** test-quality-agent
- **Round found:** 1
- **Resolution:** remaining (Minor, not in fix scope)

**What is wrong:**
Several `mapToOpenShellProvider` tests check individual map keys without asserting map size, allowing phantom keys to go undetected. The Bedrock test only checks `providerType` is empty without verifying other return values.

**Why this matters:**
Under-assertions could mask future regressions.

## Notable Observations

### NOTABLE-1
- **File:** cc-deck/internal/ws/ssh.go:194
- **Category:** production-readiness
- **Source:** production-agent
- **Description:** The removal of the `else` fallback branch means `ssh.BuildCredentialSet()` is no longer called as a backup when `credential.DetectAll()` finds nothing.
- **Rationale:** Intentional per spec. `DetectAll()` is a superset of `BuildCredentialSet()`. The fallback existed only for migration transition; the spec mandates its removal.

### NOTABLE-2
- **File:** cc-deck/internal/cmd/ws.go:1818
- **Category:** production-readiness
- **Source:** production-agent
- **Description:** The credential-refresh command no longer uses the workspace's `auth` field to filter credential detection.
- **Rationale:** Intentional per spec. `DetectAll()` is more comprehensive than the old `BuildCredentialSet(def.Auth)`. The auth field filtering is handled at the OpenShell level via `selectCredentialMode`, not at the SSH level.

### NOTABLE-3
- **File:** cc-deck/internal/credential/transport.go:87
- **Category:** production-readiness
- **Source:** production-agent
- **Description:** `MergeCredentials` collects multiple file credentials into `FileCredentials` (plural) but `InjectSSH` processes `FileCredential` (singular, set to `files[0]`).
- **Rationale:** Pre-existing design in the credential package, not introduced by this migration. Multi-file-credential scenarios are not affected because current agent definitions use at most one file credential.

## Test Suite Results

No test command detected; post-fix test step was skipped (no fix loop was needed).
