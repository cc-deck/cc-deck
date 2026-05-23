# Deep Review Findings

**Date:** 2026-05-23
**Branch:** 060-oci-policy-extraction
**Rounds:** 1
**Gate Outcome:** PASS
**Invocation:** quality-gate

## Summary

| Severity | Found | Fixed | Remaining |
|----------|-------|-------|-----------|
| Critical | 1 | 1 | 0 |
| Important | 1 | 1 | 0 |
| Minor | 2 | - | 2 |
| **Total** | **4** | **2** | **2** |

**Agents completed:** 5/5 (+ 1 external tool)
**Agents failed:** none

## Findings

### FINDING-1
- **Severity:** Critical
- **Confidence:** 100
- **File:** cc-deck/go.mod:36-125, cc-deck/go.sum (12 locations)
- **Category:** correctness
- **Source:** test-suite
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**
Merge conflict markers (`<<<<<<<`, `=======`, `>>>>>>>`) from a prior `git stash pop` operation left go.mod and go.sum unparseable. The Go toolchain could not parse the module files, preventing all compilation and testing. The rtk wrapper masked this error, reporting "No tests found" instead of the actual parse failure.

**Why this matters:**
Complete build failure. No Go code could compile, no tests could run. The feature appeared untestable until the root cause was identified by running the go binary directly.

**How it was resolved:**
Resolved all three go.mod conflicts by keeping the "Updated upstream" side (which includes the go-containerregistry dependencies required for this feature: distribution/reference, docker/cli, docker-credential-helpers, etc.). Stripped conflict markers from go.sum and regenerated via `go mod tidy`. Also resolved conflicts in README.md (2 conflicts), cli.adoc (3 conflicts), and marked 5 unmerged env package files as resolved.

### FINDING-2
- **Severity:** Important
- **Confidence:** 95
- **File:** cc-deck/internal/oci/extract_test.go:181-185
- **Category:** test-quality
- **Source:** test-quality-agent
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**
In `TestExtractViaLayerScan_UnlabeledMultiLayerImage`, a closure containing `assert.Empty(t, cfg.Config.Labels)` was assigned to the blank identifier `_` and never called. The assertion was dead code.

```go
// Before (dead code):
_ = func() {
    cfg, err := img.ConfigFile()
    require.NoError(t, err)
    assert.Empty(t, cfg.Config.Labels)
}
```

**Why this matters:**
The test claimed to verify that unlabeled images have no labels, but this verification never executed. If the test image accidentally had labels (due to a bug in `buildTestImage` or `mutate`), the test would still pass, masking a test setup error. Dead assertions create false confidence in test coverage.

**How it was resolved:**
Replaced the unused closure with direct inline assertions:

```go
// After (executes):
cfg, err := img.ConfigFile()
require.NoError(t, err)
assert.Empty(t, cfg.Config.Labels)
```

### FINDING-3
- **Severity:** Minor
- **Confidence:** 60
- **File:** cc-deck/internal/oci/extract.go:163
- **Category:** correctness
- **Source:** correctness-agent
- **Round found:** 1
- **Resolution:** remaining (accepted risk)

**What is wrong:**
`io.Copy(&buf, tr)` reads the entire file contents into memory without imposing a size limit. A malicious or corrupted image could contain an arbitrarily large file at the `/etc/openshell/policy.yaml` path, potentially exhausting memory.

**Why this matters:**
Theoretical denial-of-service risk via memory exhaustion. In practice, the policy file path is a hardcoded constant (`/etc/openshell/policy.yaml`), policy files are YAML configs typically under 10 KB, and the images are built by the same tool (cc-deck build). The risk is negligible for the intended use case. A `io.LimitReader` guard could be added in a future hardening pass if untrusted images become a concern.

### FINDING-4
- **Severity:** Minor
- **Confidence:** 70
- **File:** cc-deck/internal/oci/extract.go:138-169 and cc-deck/internal/oci/label.go:59-84
- **Category:** architecture
- **Source:** architecture-agent
- **Round found:** 1
- **Resolution:** remaining (accepted)

**What is wrong:**
Two functions iterate tar entries with similar path normalization and header matching logic:
- `extractFileFromLayer` (extract.go:138-169): reads and returns file bytes
- `layerContainsFile` (label.go:59-84): checks existence without reading

Both normalize paths by stripping leading `./` and `/`, then compare against the normalized target path with `tar.TypeReg` type check.

**Why this matters:**
Minor code duplication. The functions serve different purposes: `extractFileFromLayer` allocates memory to return file contents, while `layerContainsFile` returns a boolean without allocation. Refactoring to share code would either force unnecessary memory allocation in the scan path or require a more complex abstraction (callback, option flags). The current duplication is intentional and results in clearer, more efficient code.

## Post-Fix Spec Coverage

All spec requirements verified after fix loop.

| Requirement | Implementation | Status |
|-------------|---------------|--------|
| FR-001: Extract file from OCI image | oci/extract.go:ExtractFileFromImage | verified |
| FR-002: Check policy-layer label first | oci/extract.go:extractViaLabel | verified |
| FR-003: Fallback to reverse layer scan | oci/extract.go:extractViaLayerScan | verified |
| FR-004: Resolve local and remote images | oci/extract.go:resolveImage | verified |
| FR-005: Standard credential chain | go-containerregistry defaults | verified |
| FR-006: Add label after openshell build | cmd/build.go + oci/label.go:StampPolicyLabel | verified |
| FR-007: No additional layers from label | oci/label.go:AddLabel (mutate.Config only) | verified |
| FR-008: Write to temp file, pass path | ws/openshell.go:resolveSandboxConfig | verified |
| FR-009: Clean up temp files | ws/openshell.go:Create (defer os.Remove) | verified |
| FR-010: Clear error with --policy suggestion | oci/extract.go + ws/openshell.go error msgs | verified |
| FR-011: Replace host-path resolution | ws/openshell.go (OCI extraction only) | verified |
| FR-012: INFO/DEBUG logging | oci/extract.go + oci/label.go log statements | verified |

## Test Suite Results

| Round | Test Command | Exit Code | Failures | Status |
|-------|-------------|-----------|----------|--------|
| 1     | go test ./internal/oci/... | 0 | 0 | passed |

Test suite passed in all fix rounds.

Note: 2 pre-existing test failures in `internal/ws/channel_pipe_test.go` (`TestExecPipeChannel_Send_Success` and `TestExecPipeChannel_SendReceive_Success`) are unrelated to this feature and exist on the main branch.

## Remaining Findings

Two Minor findings remain, both accepted as intentional design decisions:

1. **Unbounded file read** (FINDING-3): Accepted because policy files are small YAML configs from trusted images. Adding `io.LimitReader` is a future hardening option.

2. **Duplicated tar iteration** (FINDING-4): Accepted because the two functions serve different purposes (extract vs. check) and sharing code would add complexity without meaningful benefit.
