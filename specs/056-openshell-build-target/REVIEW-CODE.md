# Code Review

## Code Review Guide (30 minutes)

> This section guides a code reviewer through the implementation changes,
> focusing on high-level questions that need human judgment.

**Changed files:** 6 source files (manifest.go, policy.go, init.go, build.go, build.yaml.tmpl, cc-deck.build.md), 4 test files, 2 documentation files (README.md, cli.adoc)

### Understanding the changes (8 min)

- Start with `cc-deck/internal/build/policy.go`: This is the only new source file. It defines all the OpenShell policy types (`OpenShellPolicy`, `FilesystemPolicy`, `NetworkPolicy`, etc.), default policy generation, policy merge logic, and the `WellKnownBinaries` reference table. Reading this file gives you the full mental model.
- Then `cc-deck/internal/build/manifest.go`: The `OpenShellTarget` struct and its methods (`OpenShellImageRef()`, `OpenShellBaseImage()`). These follow the exact same pattern as the existing `ContainerTarget`. Validation adds one check for `Name`.
- Question: Is splitting policy types into `policy.go` while keeping `OpenShellTarget` in `manifest.go` the right decomposition, or should all OpenShell types live together?

### Key decisions that need your eyes (12 min)

**Policy merge by endpoint host** (`policy.go:132-158`, relates to [policy-schema.md](contracts/policy-schema.md))

The merge iterates override entries, collects all endpoint hosts into a set, then deletes any auto-generated entries whose endpoints match an override host. This means a single override entry with `github.com` removes ALL auto-generated entries that have `github.com` as any endpoint, even if that auto-generated entry had multiple endpoints.
- Question: Is deleting the entire auto-generated entry correct when only one of its endpoints is overridden, or should it preserve non-matching endpoints?

**detectRunTarget triple-ambiguity** (`build.go:157-180`)

Changed from a boolean switch (`hasContainerfile && hasSSH`) to counting found targets. When `found > 1`, a generic "multiple target artifacts found" message is shown instead of listing which ones.
- Question: Would the user benefit from knowing *which* targets were detected (e.g., "container and openshell artifacts found")?

**validateRunFlags allows push for openshell** (`build.go:183-187`)

Push is now allowed for both container and openshell targets but not SSH. The error message was updated.
- Question: Is the push flow in `runOpenShellBuild` correct to tag and push with the registry prefix, matching the container pattern?

### Areas where I'm less certain (5 min)

- `policy.go:120`: **(Fixed in deep review)** The shallow copy issue was fixed by adding deep-copy of pointer fields and the map. The concern about shared references no longer applies.
- `build.go:250-278`: The `runOpenShellBuild` push flow mirrors `runContainerBuild` exactly but was not tested end-to-end (no integration test with a real registry). The unit tests only cover `detectRunTarget` and `validateRunFlags`.
- `init.go:160-185`: The openshell section detection in `uncommentTargets` depends on the template having `#   openshell:` with exactly 3 spaces of indentation after `#`. If the template formatting changes, uncommenting will silently fail.

### Deviations and risks (5 min)

- No deviations from [plan.md](plan.md) were identified. All file paths, struct names, method signatures, and behaviors match the plan.
- The `DefaultPolicy()` read_only paths include `/usr`, `/lib`, `/proc`, `/etc`, `/var/log` per the contracts, but the existing `default-policy.yaml` also includes `/app`. The implementation follows the contract (no `/app`), not the existing YAML. This is intentional per the spec but worth flagging.

---

## Deep Review Report

> Automated multi-perspective code review results. This section summarizes
> what was checked, what was found, and what remains for human review.

**Date:** 2026-05-15 | **Rounds:** 1/3 | **Gate:** PASS

### Review Agents

| Agent | Findings | Status |
|-------|----------|--------|
| Correctness | 4 | completed |
| Architecture & Idioms | 7 | completed |
| Security | 4 | completed |
| Production Readiness | 3 | completed |
| Test Quality | 8 | completed |
| CodeRabbit (external) | 9 | completed |
| Copilot (external) | 0 | skipped (not installed) |

### Findings Summary

| Severity | Found | Fixed | Remaining |
|----------|-------|-------|-----------|
| Critical | 0 | 0 | 0 |
| Important | 9 | 5 | 4 |
| Minor | 8 | - | 8 |

### What was fixed automatically

Fixed the `MergePolicy` shallow-copy bug by adding deep-copy of all pointer fields and the map (correctness agent). Added `openshell` support to `build verify` command with `runOpenShellVerify()` (architecture agent). Updated `--target` help text to include openshell (architecture agent). Renamed shadowed `os` variable to `ost` in `Validate()` (correctness + architecture agents). Strengthened three test assertions: `TestGeneratePolicy_EmptyDomains` now verifies the full default policy structure, `TestMarshalPolicy` now round-trips all field values, and added `TestDetectRunTarget_AllThreePresent_Error` for triple-ambiguity (test quality agent).

### What still needs human attention

All Critical and Important findings were resolved or are pre-existing patterns. 4 Important findings remain but follow existing codebase conventions:

- The `runOpenShellBuild`/`runContainerBuild` duplication is a candidate for future refactoring. Should they share a helper?
- Image name/tag/registry values are not validated for flag injection patterns. This applies equally to all target types. Is a separate security hardening pass warranted?
- `WellKnownBinaries` is exported but only consumed by the AI command spec at runtime. Is the current documentation sufficient to prevent a future developer from deleting it as "dead code"?

8 Minor findings remain (see [review-findings.md](review-findings.md) for details). No further review action needed for those.

### Recommendation

All auto-fixable findings addressed. 4 Important findings follow pre-existing patterns in the codebase and are not specific to this feature. Code is ready for human review with no blockers specific to the openshell build target implementation.
