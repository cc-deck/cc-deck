# Deep Review Findings

**Date:** 2026-05-24
**Branch:** 062-egress-recording-mode
**Rounds:** 1
**Gate Outcome:** PASS
**Invocation:** manual

## Summary

| Severity | Found | Fixed | Remaining |
|----------|-------|-------|-----------|
| Critical | 2 | 2 | 0 |
| Important | 12 | 8 | 4 |
| Minor | 14 | 0 | 14 |
| **Total** | **28** | **10** | **18** |

**Agents completed:** 5/5 (+ 1 external tool)
**Agents failed:** none

## Critical Findings (Fixed)

### FINDING-1: Unsynchronized cleanup() double invocation
- **File:** cc-deck/internal/record/record.go:71-85
- **Source:** production-readiness (also: correctness)
- **Resolution:** Fixed (round 1) - wrapped cleanup with `sync.Once`

### FINDING-2: CoreDNS log writes to stdout, not file
- **File:** cc-deck/internal/record/record.go:33-41, 239-255
- **Source:** coderabbit (external)
- **Resolution:** Fixed (round 1) - changed sidecar command to redirect stdout to log file via `sh -c`

## Important Findings (Fixed)

### FINDING-3: Signal handler goroutine leaks after function returns
- **File:** cc-deck/internal/record/record.go:78-85
- **Source:** production-readiness (also: correctness)
- **Resolution:** Fixed - added done channel and signal.Stop to cleanly terminate goroutine

### FINDING-4: os.Exit(1) bypasses defer os.RemoveAll(tmpDir)
- **File:** cc-deck/internal/record/record.go:60, 84
- **Source:** production-readiness (also: correctness)
- **Resolution:** Fixed - moved tmpDir cleanup into sync.Once cleanup function

### FINDING-5: Workspace container can tamper with DNS log (writable volume)
- **File:** cc-deck/internal/record/record.go:98-101
- **Source:** security
- **Resolution:** Fixed - changed volume mount to `:ro` in workspace container

### FINDING-6: extractDNSLog "No such file" detection broken with cmd.Output()
- **File:** cc-deck/internal/record/record.go:231-244
- **Source:** correctness
- **Resolution:** Fixed - switched to CombinedOutput() for stderr capture

### FINDING-7: Unused Timestamp field in DNSLogEntry
- **File:** cc-deck/internal/record/dns.go:15-16
- **Source:** architecture (also: coderabbit)
- **Resolution:** Fixed - removed field and time import

### FINDING-8: Unused SessionConfig fields (SetupDir, ManifestPath)
- **File:** cc-deck/internal/record/record.go:25-29
- **Source:** architecture
- **Resolution:** Fixed - removed fields, updated caller

### FINDING-9: FilteredCount conflates dedup and noise (misleading metric)
- **File:** cc-deck/internal/record/record.go:119-127
- **Source:** architecture
- **Resolution:** Fixed - introduced FilterResult struct with separate NoiseCount

### FINDING-10: Unused tmpDir parameter in extractDNSLog
- **File:** cc-deck/internal/record/record.go:228
- **Source:** architecture
- **Resolution:** Fixed - removed parameter

## Important Findings (Remaining - Minor Impact)

### FINDING-11: busybox extraction container escapes pod cleanup on SIGINT
- **File:** cc-deck/internal/record/record.go:239-255
- **Source:** production-readiness
- Narrow race window during extraction. busybox uses --rm so self-cleans on exit.

### FINDING-12: Direct exec.Command calls bypass internal/podman package
- **File:** cc-deck/internal/record/record.go (validateImage, runInPod, extractDNSLog)
- **Source:** architecture
- Convention improvement for future work. Functions are internal to this package.

### FINDING-13: Redundant deduplication in FilterNoise and DeduplicateDomains
- **File:** cc-deck/internal/record/dns.go, record.go
- **Source:** architecture
- Both functions deduplicate by design; FilterNoise for filtering, DeduplicateDomains adds sorting.

### FINDING-14: Swallowed errors in BuildDomainIndex
- **File:** cc-deck/internal/record/catalog.go:16,24,32
- **Source:** correctness (also: architecture)
- Follows existing codebase pattern; catalog loading is best-effort.

## Test Quality Improvements (Applied)

- TestParseDNSLog: exact count assertion (12 entries) instead of > 0
- TestFilterNoise: added assert.Len for result count
- TestPrintSummary_NewDomains: added covered domain assertions (FR-013)

## Test Quality Findings (Remaining - Minor)

- pod_test.go: tests only check function existence (no behavioral assertions)
- catalog_test.go: no tests with non-nil catalogFS/userLocalFS
- record_test.go: PrintSummary tests use os.Stdout swap (fragile)
- record_test.go: TestRecordingResult_Fields tests struct assignment only
