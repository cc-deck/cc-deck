# Deep Review Findings

**Date:** 2026-05-15
**Branch:** 056-openshell-build-target
**Rounds:** 1
**Gate Outcome:** PASS
**Invocation:** manual

## Summary

| Severity | Found | Fixed | Remaining |
|----------|-------|-------|-----------|
| Critical | 0 | 0 | 0 |
| Important | 9 | 5 | 4 |
| Minor | 8 | - | 8 |
| **Total** | **17** | **5** | **12** |

**Agents completed:** 5/5 (+ 1 external tool)
**Agents failed:** none

## Findings

### FINDING-1
- **Severity:** Important
- **Confidence:** 85
- **File:** cc-deck/internal/build/policy.go:120-121
- **Category:** correctness
- **Source:** correctness-agent
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**
`MergePolicy` performed a shallow copy of `*base`, sharing pointer fields (`FilesystemPolicy`, `Landlock`, `Process`) and the `NetworkPolicies` map between `base` and `result`. Mutations to the returned policy could corrupt the base.

**Why this matters:**
Latent mutation-safety bug. If a caller modified the returned policy's filesystem paths (e.g., appending to `ReadOnly`), it would also modify the base policy.

**How it was resolved:**
Added deep-copy of all pointer fields and the map unconditionally at the start of `MergePolicy`.

### FINDING-2
- **Severity:** Important
- **Confidence:** 90
- **File:** cc-deck/internal/cmd/build.go:253-307
- **Category:** architecture
- **Source:** architecture-agent (also reported by: production-readiness-agent, coderabbit)
- **Round found:** 1
- **Resolution:** remaining (pre-existing pattern)

**What is wrong:**
`runOpenShellBuild` is a near-duplicate of `runContainerBuild`. Both share the same build-tag-push flow, differing only in which ImageRef method and registry field are used.

**Why this matters:**
Future fixes or improvements must be applied to both copies.

**How it was resolved:**
Not fixed. This follows the existing pattern for `runContainerBuild`. Extracting a shared helper would require refactoring existing code beyond the scope of this feature.

### FINDING-3
- **Severity:** Important
- **Confidence:** 90
- **File:** cc-deck/internal/cmd/build.go:341-371
- **Category:** architecture
- **Source:** architecture-agent
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**
`newBuildVerifyCmd` did not accept `"openshell"` as a valid target. Users would get a confusing error when trying `cc-deck build verify --target openshell`.

**Why this matters:**
Incomplete integration of the new target type.

**How it was resolved:**
Added `"openshell"` case to verify's switch statement and implemented `runOpenShellVerify()` following the same pattern as `runContainerVerify()`.

### FINDING-4
- **Severity:** Important
- **Confidence:** 85
- **File:** cc-deck/internal/cmd/build.go:265-272
- **Category:** security
- **Source:** security-agent
- **Round found:** 1
- **Resolution:** remaining (pre-existing pattern)

**What is wrong:**
Image name/tag/registry values from `build.yaml` are passed to `exec.Command` arguments without format validation. Values starting with `-` could be interpreted as podman flags.

**Why this matters:**
Argument injection via flag-like image names/tags.

**How it was resolved:**
Not fixed. This is a pre-existing pattern in `runContainerBuild` that applies to all targets. Adding validation belongs in a separate security hardening task.

### FINDING-5
- **Severity:** Important
- **Confidence:** 95
- **File:** cc-deck/internal/build/policy.go:164-174
- **Category:** architecture
- **Source:** architecture-agent
- **Round found:** 1
- **Resolution:** remaining (by design)

**What is wrong:**
`WellKnownBinaries` is exported but not referenced by any Go code.

**Why this matters:**
Appears to be dead code.

**How it was resolved:**
Not removed. Task T017 explicitly requires this as a reference table for the AI command spec (`cc-deck.build.md` Section C2/C4). The AI command reads the spec at runtime and uses these paths during Containerfile generation.

### FINDING-6
- **Severity:** Important
- **Confidence:** 95
- **File:** cc-deck/internal/cmd/build_test.go
- **Category:** test-quality
- **Source:** test-quality-agent
- **Round found:** 1
- **Resolution:** remaining (pre-existing pattern)

**What is wrong:**
No test for `runOpenShellBuild()` error paths. The function has multiple conditional branches that are untested.

**Why this matters:**
Bugs in error paths would go undetected.

**How it was resolved:**
Not fixed. The function shells out to `podman` making unit testing impractical without mocking the exec layer. Same pattern as the existing untested `runContainerBuild`. A future refactoring to extract the build interface would enable testing both.

### FINDING-7
- **Severity:** Important
- **Confidence:** 90
- **File:** cc-deck/internal/build/policy_test.go:36-44
- **Category:** test-quality
- **Source:** test-quality-agent
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**
`TestGeneratePolicy_EmptyDomains` only checked `Version` and `NetworkPolicies`, not the full default policy structure.

**How it was resolved:**
Added assertions for `FilesystemPolicy`, `Landlock`, and `Process` fields.

### FINDING-8
- **Severity:** Important
- **Confidence:** 90
- **File:** cc-deck/internal/build/policy_test.go:71-82
- **Category:** test-quality
- **Source:** test-quality-agent
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**
`TestMarshalPolicy` only checked that top-level fields were non-nil, not that values survived the round-trip.

**How it was resolved:**
Added assertions for `IncludeWorkdir`, `ReadOnly`, `ReadWrite`, `Compatibility`, `RunAsUser`, and `RunAsGroup`.

### FINDING-9
- **Severity:** Important
- **Confidence:** 90
- **File:** cc-deck/internal/cmd/build_test.go
- **Category:** test-quality
- **Source:** test-quality-agent
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**
No test for the three-way ambiguity case where all three target types (container, ssh, openshell) have artifacts present.

**How it was resolved:**
Added `TestDetectRunTarget_AllThreePresent_Error` test case.

### FINDING-10
- **Severity:** Minor
- **Confidence:** 90
- **File:** cc-deck/internal/build/manifest.go:213
- **Category:** correctness
- **Source:** correctness-agent (also reported by: architecture-agent)
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**
Variable `os` in `Validate()` shadowed the imported `os` package.

**How it was resolved:**
Renamed to `ost`.

## Remaining Findings

Four Important findings remain, all following pre-existing patterns in the codebase or required by spec:
- F-2: Code duplication between `runOpenShellBuild`/`runContainerBuild` (refactoring beyond scope)
- F-4: Image name validation for flag injection (cross-cutting security hardening)
- F-5: `WellKnownBinaries` appears unused but is required by spec T017
- F-6: No unit test for `runOpenShellBuild` (shells out to podman, same as existing)

Eight Minor findings remain and are documented for reviewer awareness.
