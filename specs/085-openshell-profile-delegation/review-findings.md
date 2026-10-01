# Deep Review Findings

**Date:** 2026-07-31
**Branch:** 085-openshell-profile-delegation
**Rounds:** 1
**Gate Outcome:** PASS
**Invocation:** quality-gate

## Summary

| Severity | Found | Fixed | Remaining |
|----------|-------|-------|-----------|
| Critical | 0 | 0 | 0 |
| Important | 10 | 10 | 0 |
| Minor | 14 | 0 | 14 |
| **Total** | **24** | **10** | **14** |

**Agents completed:** 5/5 (+ 1 external tool)
**Agents failed:** 0

## Findings

### FINDING-1
- **Severity:** Important
- **Confidence:** 92
- **File:** internal/build/profiles.go:70-77
- **Category:** correctness
- **Source:** correctness-agent (also reported by: test-quality-agent)
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**
`BuildProfileManifest` iterated all entries in `AllowedDomainsPerAgent` without filtering by active agents. A manifest with `allowed_domains_per_agent: {opencode: ["internal.corp.com"]}` but no `opencode` agent would still include `internal.corp.com` in the profile manifest, granting unintended network access.

**Why this matters:**
Violates spec User Story 4 acceptance scenario 3, which requires domains for inactive agents to be excluded. The existing policy code in `policy.go:237-253` correctly filters by active agents.

**How it was resolved:**
Added `activeAgents` map built from `manifest.EffectiveAgents()` and skip domains for agents not in the map.

---

### FINDING-2
- **Severity:** Important
- **Confidence:** 92
- **File:** internal/openshell/profiles.go:119-130
- **Category:** correctness
- **Source:** production-readiness-agent (also reported by: correctness-agent)
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**
`VerifyProfiles` treated every error from `client.Providers().Profiles().Get()` as "profile not found" and silently skipped the profile. Transient errors (network timeouts, authentication failures) were indistinguishable from a genuinely missing profile.

**Why this matters:**
If the gateway has a transient error, valid profiles would be classified as "missing" and skipped, resulting in a sandbox with reduced network access. The rest of the codebase consistently uses `v1.IsNotFound(err)` to distinguish "not found" from other errors.

**How it was resolved:**
Changed `VerifyProfiles` to return a third `error` value. Only `v1.IsNotFound(err)` profiles are classified as missing; other errors propagate. Updated caller in `createProfileProviders`. Added `TestVerifyProfiles_TransientError` test with a new `errorProfileClient` mock.

---

### FINDING-3
- **Severity:** Important
- **Confidence:** 95
- **File:** internal/openshell/profiles.go:13-19
- **Category:** architecture (dead code)
- **Source:** architecture-agent
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**
`ProfileMapping` struct was defined but never used anywhere in the codebase.

**How it was resolved:**
Removed the unused struct and its comment.

---

### FINDING-4
- **Severity:** Important
- **Confidence:** 95
- **File:** internal/ws/openshell.go:356-360, 478-483
- **Category:** architecture (misleading naming)
- **Source:** architecture-agent (also reported by: coderabbit)
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**
`resolveAgentNames` accepted a `*WorkspaceDefinition` parameter it ignored (named `_`), always returning `[]string{"claude"}`. The doc comment was misleading. The caller in `Create()` had a redundant fallback that could never trigger.

**How it was resolved:**
Removed the unused parameter. Updated the comment to say it's a stub. Simplified the caller to a single line: `agentNames := resolveAgentNames()`.

---

### FINDING-5
- **Severity:** Important
- **Confidence:** 85
- **File:** internal/ws/openshell.go:229-247
- **Category:** security
- **Source:** security-agent (also reported by: architecture-agent)
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**
`parseHostPort` used `strings.LastIndex(endpoint, ":")` which breaks for IPv6 addresses (e.g., `[::1]:8080`). It also did not validate hostnames for special characters. The codebase already uses `net.SplitHostPort` elsewhere (`internal/openshell/client.go:57`).

**How it was resolved:**
Replaced custom implementation with `net.SplitHostPort` from the standard library, keeping the port range validation.

---

### FINDING-6
- **Severity:** Important
- **Confidence:** 85
- **File:** internal/ws/openshell_test.go:559-570
- **Category:** test-quality
- **Source:** test-quality-agent
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**
`TestCreate_ProfileBased_NoPolicy` claimed to verify no SandboxPolicy is generated (FR-001) but only checked sandbox phase, not the absence of a Policy field.

**How it was resolved:**
Added `assert.Nil(t, sb.Spec.Policy, "SandboxSpec.Policy must be nil for profile-based path (FR-001)")`.

---

### FINDING-7
- **Severity:** Important
- **Confidence:** 85
- **File:** internal/ws/openshell_test.go:620-631
- **Category:** test-quality
- **Source:** test-quality-agent
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**
`TestCreateProfileProviders_WithProfiles` verified return values but never confirmed that `Providers().Ensure()` actually created providers in the fake. The test would pass even if `Ensure()` calls were removed.

**How it was resolved:**
Added verification loop that calls `client.Providers().Get()` for each returned provider name and asserts the provider exists with a non-empty Type.

---

### FINDING-8
- **Severity:** Important
- **Confidence:** 85
- **File:** internal/ws/openshell_test.go:758-769
- **Category:** test-quality
- **Source:** test-quality-agent
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**
`TestImportMCPProfile_SingleEndpoint` only checked the returned provider name but did not verify profile import or provider creation side effects.

**How it was resolved:**
Added assertions to verify: (1) profile exists in the fake profile client with correct endpoints, (2) provider exists via `Providers().Get()` with correct Type.

---

### FINDING-9
- **Severity:** Important
- **Confidence:** 80
- **File:** internal/openshell/profiles_test.go:96-112
- **Category:** test-quality
- **Source:** test-quality-agent
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**
`TestResolveProfiles_FullCoverage` checked subset inclusion (all expected profiles present) but not exactness. Would not catch spurious profile additions.

**How it was resolved:**
Added `assert.Equal(t, len(expected), len(profiles), ...)` to verify no extra profiles are present.

---

### FINDING-10
- **Severity:** Minor
- **Confidence:** 75
- **File:** internal/cmd/build.go:1019-1065
- **Category:** external (CodeRabbit)
- **Source:** coderabbit
- **Round found:** 1
- **Resolution:** deferred

**What is wrong:**
`refreshOpenShellPolicy` still assembles `policy.yaml` via `AssemblePolicyWithOptions` before generating `profiles.yaml`. If assembly fails, profile manifest generation is blocked.

**Why this matters:**
Latent dependency between the old policy path and the new profile path. Currently the component files still exist so assembly succeeds, but this couples the two systems.

**External tool analysis (CodeRabbit):**
> Restrict AssemblePolicyWithOptions and its policy.yaml write to the Compose path, or separate the OpenShell and Compose refresh paths.

**Why deferred:**
The component files are still present and assembly succeeds. This is an architectural cleanup for a future PR, not a correctness issue.

---

### FINDING-M1
- **Severity:** Minor
- **Confidence:** 88
- **File:** internal/build/profiles.go:28-81
- **Category:** correctness
- **Source:** correctness-agent
- **Resolution:** remaining

`BuildProfileManifest` never populates `AgentBinaries`. The field is defined and serialized, but the build function doesn't extract binary paths from matched components. At runtime, `importMCPProfile` receives empty binaries.

---

### FINDING-M2
- **Severity:** Minor
- **Confidence:** 85
- **File:** internal/cmd/build.go:608-662
- **Category:** architecture
- **Source:** architecture-agent
- **Resolution:** remaining

`runOpenShellVerify` and `runContainerVerify` are near-identical functions that could be extracted into a shared helper.

---

### FINDING-M3
- **Severity:** Minor
- **Confidence:** 80
- **File:** internal/cmd/build.go:324-346, 385-407
- **Category:** architecture
- **Source:** architecture-agent
- **Resolution:** remaining

Push logic is copy-pasted between `runContainerBuild` and `runOpenShellBuild`.

---

### FINDING-M4
- **Severity:** Minor
- **Confidence:** 80
- **File:** internal/ws/openshell.go:128-227
- **Category:** architecture
- **Source:** architecture-agent
- **Resolution:** remaining

`importMCPProfile` and `importCustomDomainsProfile` share ~40 lines of identical import-and-ensure structure that could be extracted.

---

### FINDING-M5
- **Severity:** Minor
- **Confidence:** 85
- **File:** internal/ws/openshell.go:164-169, 209-215
- **Category:** production-readiness
- **Source:** production-readiness-agent
- **Resolution:** remaining (spec-compliant)

`importMCPProfile` and `importCustomDomainsProfile` swallow `Import()` failures, returning `("", nil)`. This is spec-compliant behavior per FR-008 ("Import() failure: warn and continue").

---

### FINDING-M6
- **Severity:** Minor
- **Confidence:** 78
- **File:** internal/ws/openshell.go:441-454
- **Category:** production-readiness
- **Source:** production-readiness-agent
- **Resolution:** remaining

`createProfileProviders` does not clean up already-created providers when a mid-sequence `Ensure()` call fails.

---

### FINDING-M7
- **Severity:** Minor
- **Confidence:** 75
- **File:** internal/ws/openshell.go:111-121
- **Category:** production-readiness
- **Source:** production-readiness-agent
- **Resolution:** remaining

`extractProfileManifest` conflates "file not in image" with OCI infrastructure errors.

---

### FINDING-M8
- **Severity:** Minor
- **Confidence:** 70
- **File:** internal/ws/openshell_test.go
- **Category:** test-quality
- **Source:** test-quality-agent
- **Resolution:** remaining

Missing test for `parseHostPort(":8080")` empty-host edge case.

---

### FINDING-M9
- **Severity:** Minor
- **Confidence:** 70
- **File:** internal/openshell/profiles_test.go:63
- **Category:** test-quality
- **Source:** test-quality-agent
- **Resolution:** remaining

Hardcoded profile count `2` in `TestResolveProfiles_AlwaysIncluded` will break if always-included list changes.

---

### FINDING-M10
- **Severity:** Minor
- **Confidence:** 75
- **File:** internal/ws/openshell.go
- **Category:** security
- **Source:** security-agent
- **Resolution:** remaining

Credential transport over non-TLS gateway connections. Default config uses plaintext to localhost, acceptable for local dev but not for remote gateways.

---

### FINDING-M11
- **Severity:** Minor
- **Confidence:** 72
- **File:** internal/build/profiles.go:89-94
- **Category:** security
- **Source:** security-agent
- **Resolution:** remaining

No size limit on YAML from OCI image. A malicious image could embed a large `profiles.yaml`.

---

### FINDING-M12
- **Severity:** Minor
- **Confidence:** 80
- **File:** internal/credential/transport.go:270-278
- **Category:** security
- **Source:** security-agent
- **Resolution:** remaining (pre-existing code)

`injectOpenShellEnvVar` does not validate env var keys against shell metacharacters. Defense-in-depth concern in pre-existing code.

---

### FINDING-M13
- **Severity:** Minor
- **Confidence:** 70
- **File:** docs/modules/reference/pages/configuration.adoc:92-94
- **Category:** external (CodeRabbit)
- **Source:** coderabbit
- **Resolution:** remaining

Profiles definition in docs doesn't mention auto-detected components as a source.

---

### FINDING-M14
- **Severity:** Minor
- **Confidence:** 70
- **File:** README.md
- **Category:** external (CodeRabbit)
- **Source:** coderabbit
- **Resolution:** remaining

Tool examples in README use bare strings instead of object format with `name` field matching actual manifest schema.

## Post-Fix Verification

**Spec compliance (post-fix):** 14/14 FRs verified. No dropped requirements.

**Test results:**
- openshell: 52 tests passing (51 + 1 new TransientError test)
- build: 280 tests passing
- ws: 405 tests passing
- Lint: clean (`go vet` + `cargo clippy`)
- Pre-existing failures: compose smoke tests (podman-compose Python compatibility, unrelated)
