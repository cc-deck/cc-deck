# Deep Review Findings

**Date:** 2026-05-18
**Branch:** 056-sidebar-badges
**Rounds:** 0
**Gate Outcome:** PASS
**Invocation:** quality-gate

## Summary

| Severity | Found | Fixed | Remaining |
|----------|-------|-------|-----------|
| Critical | 0 | 0 | 0 |
| Important | 0 | 0 | 0 |
| Minor | 2 | 0 | 2 |
| **Total** | **2** | **0** | **2** |

**Agents completed:** 5/5 (+ 1 external tool)
**Agents failed:** none

## Deep Review Report

### Spec Compliance

**Overall Score: 100% (10/10)**

| Requirement | Implementation | Status |
|-------------|---------------|--------|
| FR-001: Badge rules in config.yaml | config.go:24 Badges field | Compliant |
| FR-002: Rule fields (name, file, format, extract, values, default) | config.go:28-35 BadgeRule struct | Compliant |
| FR-003: CLI evaluates on each hook event | hook.go:144-151 badge.Evaluate call | Compliant |
| FR-004: Resolved badges in hook payload | hook.go:39 Badges field, hook.go:157 | Compliant |
| FR-005: Plugin renders on line 2 before branch | render.rs:396-407 badge_prefix | Compliant |
| FR-006: Multiple badges in config order | badge.go:21-30 sequential iteration | Compliant |
| FR-007: Silent failure | badge.go:36-43 return "" on errors | Compliant |
| FR-008: Nested dot-paths | badge.go:62-80 segment traversal | Compliant |
| FR-009: Default emoji fallback | badge.go:47-49, 55-57 | Compliant |
| FR-010: Skip when no config/working_dir | badge.go:17-18 early return | Compliant |

### Review Agents Summary

| Agent | Found | Fixed | Remaining | Status |
|-------|-------|-------|-----------|--------|
| Correctness | 0 | 0 | 0 | completed |
| Architecture & Idioms | 0 | 0 | 0 | completed |
| Security | 1 | 0 | 1 | completed |
| Production Readiness | 1 | 0 | 1 | completed |
| Test Quality | 0 | 0 | 0 | completed |
| CodeRabbit (external) | 0 | 0 | 0 | completed |
| Copilot (external) | 0 | 0 | 0 | skipped (not installed) |
| **Total** | **2** | **0** | **2** | |

## Findings

### FINDING-1
- **Severity:** Minor
- **Confidence:** 70
- **File:** cc-deck/internal/badge/badge.go:34
- **Lines:** 34-35
- **Category:** security
- **Source:** security-agent
- **Round found:** 1
- **Resolution:** accepted (by design)

**What is wrong:**
`filepath.Join(workingDir, rule.File)` does not validate that the resulting path stays within `workingDir`. A badge rule with `file: "../../etc/passwd"` could read files outside the working directory.

**Why this matters:**
Theoretically allows reading arbitrary files on the filesystem. However, badge rules come exclusively from the user's own `~/.config/cc-deck/config.yaml`, which is not externally controlled. The user already has filesystem access, so this is not an escalation.

**How it was resolved:**
Accepted as by-design. The config file is user-controlled, and restricting paths would add complexity without security benefit. Documented as a known design choice.

### FINDING-2
- **Severity:** Minor
- **Confidence:** 72
- **File:** cc-deck/internal/cmd/hook.go:145
- **Lines:** 145-149
- **Category:** production-readiness
- **Source:** production-agent
- **Round found:** 1
- **Resolution:** accepted (within spec target)

**What is wrong:**
`config.Load("")` is called on every hook event, reading and parsing the YAML config file from disk each time. With frequent hook events during active coding, this means repeated file I/O.

**Why this matters:**
Could add latency to hook processing. However, the config file is typically small (<1KB), and YAML parsing takes well under 1ms. The spec target of <50ms per evaluation is easily met. Caching by mtime could be added in a future iteration if profiling shows this is a bottleneck.

**How it was resolved:**
Accepted. Performance is within spec target. Caching is documented as a future optimization opportunity in the brainstorm's Open Questions section.

## Post-Fix Spec Coverage

No code was removed during review. All 10 functional requirements verified after implementation.

| Requirement | Implementation | Status |
|-------------|---------------|--------|
| FR-001 | config.go:24 | ✓ |
| FR-002 | config.go:28-35 | ✓ |
| FR-003 | hook.go:144-151 | ✓ |
| FR-004 | hook.go:39,157 | ✓ |
| FR-005 | render.rs:396-407 | ✓ |
| FR-006 | badge.go:21-30 | ✓ |
| FR-007 | badge.go:36-43 | ✓ |
| FR-008 | badge.go:62-80 | ✓ |
| FR-009 | badge.go:47-57 | ✓ |
| FR-010 | badge.go:17-18 | ✓ |

All spec requirements verified. No requirements dropped.
