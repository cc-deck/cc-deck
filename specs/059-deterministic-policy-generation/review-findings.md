# Deep Review Findings

**Date:** 2026-05-22
**Branch:** 059-deterministic-policy-generation
**Rounds:** 1
**Gate Outcome:** PASS
**Invocation:** quality-gate

## Summary

| Severity | Found | Fixed | Remaining |
|----------|-------|-------|-----------|
| Critical | 0 | 0 | 0 |
| Important | 6 | 6 | 0 |
| Minor | 11 | 0 | 11 |
| **Total** | **17** | **6** | **11** |

**Agents completed:** 5/5 (+ 1 external tool)
**Agents failed:** none

## Findings

### FINDING-1
- **Severity:** Important
- **Confidence:** 92
- **File:** cc-deck/internal/build/catalog.go:62-84
- **Category:** security
- **Source:** security-agent (also reported by: production-agent)
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**
Filenames from the remote catalog index were used directly in `filepath.Join(cacheDir, filename)` without sanitization. A malicious catalog could supply filenames like `../../.ssh/authorized_keys` to write files outside the cache directory.

**Why this matters:**
Classic path traversal vulnerability. The catalog is fetched over HTTP from a remote server, so the filenames are untrusted input.

**How it was resolved:**
Added filename validation that rejects any filename containing `..`, `/`, or `\`. Only bare filenames (no directory components) are accepted.

### FINDING-2
- **Severity:** Important
- **Confidence:** 88
- **File:** cc-deck/internal/build/catalog.go:55-57
- **Category:** security
- **Source:** security-agent
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**
The `CatalogIndex.BaseURL` field from the remote YAML response could override the application-provided `baseURL`, enabling SSRF by redirecting component downloads to attacker-controlled URLs.

**Why this matters:**
In cloud environments, this could be used to access instance metadata services. Even locally, it could probe internal network services.

**How it was resolved:**
Removed the `base_url` override. The caller-provided `baseURL` is always used; the index's `base_url` field is ignored.

### FINDING-3
- **Severity:** Important
- **Confidence:** 90
- **File:** cc-deck/internal/build/catalog.go:34,70
- **Category:** security
- **Source:** security-agent (also reported by: production-agent, coderabbit)
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**
Both `io.ReadAll` calls read HTTP response bodies with no size limit. A malicious server could send a multi-gigabyte response to exhaust memory.

**Why this matters:**
Standard DoS vector against HTTP clients. The 30-second timeout provides some implicit bound but is insufficient on fast connections.

**How it was resolved:**
Added `io.LimitReader` with 64 KB for the catalog index and 512 KB for component files. Also fixed the status code check order (CodeRabbit finding): check HTTP status before reading the body.

### FINDING-4
- **Severity:** Important
- **Confidence:** 95
- **File:** cc-deck/internal/build/component.go:168-206
- **Category:** architecture
- **Source:** architecture-agent (also reported by: correctness-agent, production-agent)
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**
`LoadComponentTier` read the filesystem twice: first via `LoadComponentsFromFS` (whose results were discarded), then again with its own read loop. This doubled I/O and parsing work, and created a TOCTOU risk if the filesystem changed between reads.

**Why this matters:**
Duplication that will diverge as validation logic evolves. The two code paths had different error handling (one collected warnings, the other silently continued).

**How it was resolved:**
Refactored `LoadComponentTier` to a single-pass implementation that reads the directory once, processes each file once, and collects warnings consistently.

### FINDING-5
- **Severity:** Important
- **Confidence:** 75
- **File:** cc-deck/internal/build/catalog.go:70-76
- **Category:** correctness
- **Source:** coderabbit
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**
The code called `io.ReadAll(resp.Body)` before checking `resp.StatusCode`, causing unnecessary reads for non-200 responses.

**Why this matters:**
Wastes bandwidth and memory reading error responses that will be discarded.

**How it was resolved:**
Moved the status code check before the body read. Non-200 responses are now closed immediately without reading.

### FINDING-6
- **Severity:** Important
- **Confidence:** 90
- **File:** cc-deck/internal/build/policy.go:276-281
- **Category:** correctness
- **Source:** correctness-agent
- **Round found:** 1
- **Resolution:** fixed (round 1)

**What is wrong:**
The `slugify` function replaced both `.` and `-` with `_`, creating collisions between domains that differ only in dots, hyphens, or underscores (e.g., `api.my-service.com` and `api.my_service.com` both became `api_my_service_com`).

**Why this matters:**
Valid user-specified domains could be silently dropped when two domains produce the same slug, violating FR-001 (deterministic, complete policy generation).

**How it was resolved:**
Changed `slugify` to only replace `.` with `_`, preserving hyphens. YAML map keys support hyphens. Added a test verifying no collision between hyphenated and underscored domains.

## Remaining Findings (Minor)

- **architecture:** `fmt.Printf` used for warnings in library code instead of `log.Printf` (convention mismatch)
- **architecture:** `runContainerBuild`/`runOpenShellBuild` duplication in cmd/build.go (pre-existing)
- **architecture:** `features` match condition accepted by validation but never evaluated
- **correctness:** `MergePolicy` returns base pointer directly when overrides is nil (latent mutation risk)
- **security:** Credential values passed as CLI arguments visible in process table (pre-existing)
- **security:** Port range not validated (1-65535) in `ValidateComponent`
- **test-quality:** No direct tests for `LoadComponentTier` or `LoadEmbeddedComponents`
- **test-quality:** `features` match condition has zero test coverage
- **test-quality:** Determinism test uses only 2 iterations (weak statistical confidence)
- **test-quality:** No test that `MergePolicy` does not mutate base
- **test-quality:** `base_url` override path in catalog tests never exercised
