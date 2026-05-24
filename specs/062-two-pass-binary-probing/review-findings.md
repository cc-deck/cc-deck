# Deep Review Findings

**Date:** 2026-05-24
**Branch:** 062-two-pass-binary-probing
**Rounds:** 1
**Gate Outcome:** PASS
**Invocation:** manual

## Summary

| Severity | Found | Fixed | Remaining |
|----------|-------|-------|-----------|
| Critical | 1 | 1 | 0 |
| Important | 5 | 5 | 0 |
| Minor | 11 | - | 11 |
| **Total** | **17** | **6** | **11** |

**Agents completed:** 5/5 (+ 1 external tool)
**Agents failed:** none

## Findings

### FINDING-1
- **Severity:** Critical
- **Confidence:** 90
- **File:** cc-deck/internal/cmd/build.go:453-454
- **Category:** correctness
- **Source:** correctness-agent (also reported by: coderabbit)
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**
`probeTag` and `debugTag` were constructed by appending `:probe-build` and `:probe-debug` to `imageRef`, which already contains a colon-separated tag (e.g., `myapp:latest`). This produced invalid OCI image references like `myapp:latest:probe-build` with two colons.

**Why this matters:**
Every podman command using these references (build, run, tag, rmi) would operate on a malformed reference, potentially causing build failures, incorrect tagging, or runtime errors.

**How it was resolved:**
Split `imageRef` at the last colon (after the last `/` to handle registry paths) to extract the image name, then construct intermediate tags using only the name portion: `imageName + ":probe-build"`.

### FINDING-2
- **Severity:** Important
- **Confidence:** 85
- **File:** cc-deck/internal/build/probe.go:40-47, cc-deck/internal/build/component.go:89-92
- **Category:** security
- **Source:** security-agent (also reported by: production-readiness-agent, coderabbit)
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**
`generateProbeScript` interpolated binary names directly into a shell script via `fmt.Sprintf` without sanitizing for shell metacharacters. The `ValidateComponent` function only rejected binary names containing `/`, allowing names like `pip;rm -rf /` or `$(malicious)` to pass validation.

Additionally, when `ProbeBinaries` was empty, the fallback to `Match.Tools` entries had no validation at all.

**Why this matters:**
A crafted catalog or user-local component YAML could inject shell commands into the probe script running inside the container. While the blast radius is limited to a disposable container, arbitrary code execution is still a meaningful concern.

**How it was resolved:**
1. Added `isValidBinaryName()` using regex `^[a-zA-Z0-9._-]+$` to enforce a strict allowlist.
2. Applied this validation in `ValidateComponent` for `ProbeBinaries` entries.
3. Applied the same check in `generateProbeScript` to skip unsafe names from `Match.Tools` fallback.
4. Also added backslash rejection to the path separator check.

### FINDING-3
- **Severity:** Important
- **Confidence:** 75
- **File:** cc-deck/internal/build/probe.go:41-46
- **Category:** correctness
- **Source:** correctness-agent
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**
Per-binary timeout of 30 seconds (FR-004) was applied to `which` and `find` independently, not combined. Each binary could consume up to 60 seconds (30s for `which` + 30s for `find`) rather than the 30 seconds specified in the spec.

**Why this matters:**
FR-004 explicitly requires "Each individual binary probe (`which` + optional `find` fallback) MUST time out after 30 seconds." The double timeout could also interact with the 5-minute total timeout in unexpected ways.

**How it was resolved:**
Wrapped the entire per-binary command chain (`which` + `find` fallback) in a single `timeout 30 sh -c '...'` so both lookups share the 30-second budget.

### FINDING-4
- **Severity:** Important
- **Confidence:** 95
- **File:** cc-deck/internal/cmd/build.go:1011-1069
- **Category:** architecture
- **Source:** architecture-agent
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**
`writeOpenShellPolicy` and `refreshOpenShellPolicy` were near-duplicate functions doing the same thing: resolve catalog/user-local dirs, assemble policy, merge overrides, marshal YAML, write to `openshell/policy.yaml`. The only difference was that `refreshOpenShellPolicy` accepted a `*ProbeReport` parameter.

**Why this matters:**
The duplication would diverge when someone changes one but not the other, leading to inconsistent policy assembly behavior.

**How it was resolved:**
Replaced all `writeOpenShellPolicy` calls with `refreshOpenShellPolicy(dir, m, nil)` and removed the `writeOpenShellPolicy` function entirely.

### FINDING-5
- **Severity:** Important
- **Confidence:** 90
- **File:** cc-deck/internal/cmd/build.go:489, 504, 517
- **Category:** production-readiness
- **Source:** production-readiness-agent
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**
On probe or second-pass build failure, `probeTag` was retagged to `debugTag` but `probeTag` was never removed. Both image tags lingered in the local container store, causing image accumulation across retries.

**Why this matters:**
Repeated build failures would accumulate probe-build images, wasting disk space. The debug tag was created correctly but the intermediate tag was orphaned.

**How it was resolved:**
Extracted a `retainDebugImage` helper that retags to `debugTag` then removes `probeTag`, ensuring only the debug image remains on failure paths.

### FINDING-6
- **Severity:** Important
- **Confidence:** 90
- **File:** cc-deck/internal/build/policy.go:385-394
- **Category:** architecture
- **Source:** correctness-agent (also reported by: architecture-agent)
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**
`stripBinaries` had a misleading doc comment stating "entries without explicit binaries have their Binaries field cleared." The function body sets `Binaries = nil` only when `len(comp.Binaries) == 0`, which is a no-op since the field is already nil/empty.

**Why this matters:**
The misleading documentation could cause confusion about the function's purpose. The function is actually a defensive normalization pass that creates a shallow copy and preserves explicit binaries per FR-011.

**How it was resolved:**
Updated the doc comment to accurately describe the function's behavior as a normalization pass that preserves explicit binaries per FR-011.

### FINDING-7 (Minor)
- **Severity:** Minor
- **Confidence:** 85
- **File:** cc-deck/internal/build/probe_test.go:104-133
- **Category:** test-quality
- **Source:** test-quality-agent
- **Round found:** 1
- **Resolution:** remaining

`TestProbeBinaries_ExcludesExplicitBinaries` re-implements the filtering logic from `ProbeBinaries` in the test body rather than testing the actual function.

### FINDING-8 (Minor)
- **Severity:** Minor
- **Confidence:** 90
- **File:** cc-deck/internal/cmd/build.go:409-424
- **Category:** test-quality
- **Source:** test-quality-agent
- **Round found:** 1
- **Resolution:** remaining

No unit test coverage for `componentsNeedProbing` orchestration logic.

### FINDING-9 (Minor)
- **Severity:** Minor
- **Confidence:** 80
- **File:** cc-deck/internal/build/probe.go
- **Category:** architecture
- **Source:** architecture-agent
- **Round found:** 1
- **Resolution:** remaining

Magic string constants for `ProbeResult.Method` ("which", "find", "not-found") used across multiple files without constants.

### FINDING-10 (Minor)
- **Severity:** Minor
- **Confidence:** 85
- **File:** cc-deck/internal/cmd/build.go
- **Category:** architecture
- **Source:** architecture-agent
- **Round found:** 1
- **Resolution:** remaining

Duplicated push logic in `runContainerBuild` and `runOpenShellBuild`. Pre-dates this change but extended by it.

### FINDING-11 (Minor)
- **Severity:** Minor
- **Confidence:** 80
- **File:** cc-deck/internal/cmd/build.go:409-424, 476-483
- **Category:** architecture
- **Source:** architecture-agent
- **Round found:** 1
- **Resolution:** remaining

`componentsNeedProbing` re-assembles policy redundantly (same assembly call repeated in `runTwoPassBuild`).

### FINDING-12 (Minor)
- **Severity:** Minor
- **Confidence:** 80
- **File:** cc-deck/internal/build/policy_test.go
- **Category:** test-quality
- **Source:** test-quality-agent
- **Round found:** 1
- **Resolution:** remaining

Missing test for `applyProbeResults` when `ProbeReport` is nil.

### FINDING-13 (Minor)
- **Severity:** Minor
- **Confidence:** 78
- **File:** cc-deck/internal/build/probe_test.go
- **Category:** test-quality
- **Source:** test-quality-agent
- **Round found:** 1
- **Resolution:** remaining

Spec acceptance scenario 5 (new tool discovered purely via `match.tools` fallback through `applyProbeResults`) has no dedicated end-to-end unit test.

### FINDING-14 (Minor)
- **Severity:** Minor
- **Confidence:** 75
- **File:** cc-deck/internal/build/probe_test.go
- **Category:** test-quality
- **Source:** test-quality-agent
- **Round found:** 1
- **Resolution:** remaining

Missing edge case tests: empty ProbeBinaries + empty Match.Tools, malformed but structurally valid JSON in parseProbeOutput.

### FINDING-15 (Minor)
- **Severity:** Minor
- **Confidence:** 72
- **File:** cc-deck/internal/build/probe.go:42-45
- **Category:** security
- **Source:** security-agent
- **Round found:** 1
- **Resolution:** remaining

`comp.Key` interpolated into shell script without character validation. Lower risk since keys come from YAML files, not user input, and the strict binary name validation now covers the primary injection vector.

### FINDING-16 (Minor)
- **Severity:** Minor
- **Confidence:** 75
- **File:** cc-deck/internal/build/component_test.go:617-652
- **Category:** test-quality
- **Source:** coderabbit
- **Round found:** 1
- **Resolution:** remaining

Test `TestEmbeddedComponents_ProbeBinariesAndRuntimeGlobs` should verify component presence with `require.Contains` before asserting fields.

### FINDING-17 (Minor)
- **Severity:** Minor
- **Confidence:** 70
- **File:** cc-deck/internal/build/component.go:89-92
- **Category:** correctness
- **Source:** coderabbit
- **Round found:** 1
- **Resolution:** fixed (round 1, as part of FINDING-2)

ProbeBinaries validation should also reject backslash (`\`) path separator.

## Test Suite Results

| Round | Test Command | Exit Code | Failures | Status |
|-------|-------------|-----------|----------|--------|
| 1     | go test ./internal/build/... | 0 | 0 | passed |
