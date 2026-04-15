# Plan Review: Environment Lifecycle Fixes

**Branch**: `037-env-lifecycle-fixes` | **Reviewed**: 2026-04-14

## Coverage Matrix

| Requirement | Plan Phase | Task | Test Coverage | Status |
|-------------|-----------|------|---------------|--------|
| FR-001 (Ignore project-local when name differs) | Phase 2 | 2.1 | env_create_test.go | Covered |
| FR-002 (Global definition lookup) | Phase 2 | 2.1 | env_create_test.go | Covered |
| FR-002a (No scaffolding for global defs) | Phase 2 | 2.1 | env_create_test.go | Covered |
| FR-003 (Fallback to local type) | Phase 2 | 2.1 | env_create_test.go | Covered |
| FR-004 (Preserve project-local match) | Phase 2 | 2.1 | env_create_test.go (regression) | Covered |
| FR-005 (SSH delete removes definition) | Phase 1 | 1.1 | ssh_test.go | Covered |
| FR-006 (Best-effort delete) | Phase 1 | 1.1 | ssh_test.go | Covered |
| FR-007 (SOURCE column in ls) | Phase 4 | 4.1 | env list tests | Covered |
| FR-008 (No PATH column) | Phase 4 | 4.1 | env list tests | Covered |
| FR-009 (Project path in status) | Phase 5 | 5.1 | env status tests | Covered |
| FR-010 (SOURCE in structured output) | Phase 4 | 4.1 | JSON/YAML tests | Covered |
| FR-011 (--type precedence preserved) | Phase 2 | 2.1 | env_create_test.go | Covered |
| FR-012 (--global/--local flags) | Phase 3 | 3.1 | env_create_test.go | Covered |
| FR-013 (--global error on missing) | Phase 3 | 3.1 | env_create_test.go | Covered |
| FR-014 (--local error on missing) | Phase 3 | 3.1 | env_create_test.go | Covered |
| FR-015 (--global/--local + --type) | Phase 3 | 3.1 | env_create_test.go | Covered |

**Coverage**: 16/16 requirements covered (100%)

## Success Criteria Mapping

| Criterion | Validated By |
|-----------|-------------|
| SC-001 (Global name creates correct type) | Task 2.1 tests |
| SC-002 (No ghost entries after delete) | Task 1.1 tests |
| SC-003 (Correct SOURCE values) | Task 4.1 tests |
| SC-004 (No regressions) | Task 6.1: `make test` |

## Red Flag Scan

| Check | Result | Notes |
|-------|--------|-------|
| Missing test coverage | PASS | All FRs have associated tests |
| Constitution violations | PASS | All principles checked, none violated |
| Unresolved clarifications | PASS | All clarifications recorded in spec |
| Scope creep | PASS | `--global`/`--local` is justified by edge case resolution |
| Missing error handling | PASS | FR-006 best-effort, FR-013/014 error returns specified |
| Breaking changes | WARN | PATH column removal (FR-008) is a breaking change for scripts parsing table output |
| Missing documentation tasks | PASS | Phase 6 covers README, CLI reference |

### Breaking Change Note

FR-008 (PATH column removal) changes the table output format. Users or scripts that parse `cc-deck ls` table output for the PATH column will break. However:
- The PATH column was conditional (only shown when project environments existed)
- Structured output (`--output json/yaml`) never had a `path` field, so JSON/YAML consumers are unaffected
- The SOURCE column replaces PATH, providing more useful information

This is acceptable for a pre-1.0 CLI tool.

## Task Quality Assessment

| Task | Self-contained | Testable | Clear acceptance | File targets | Verdict |
|------|---------------|----------|-----------------|-------------|---------|
| 1.1 SSH delete cleanup | Yes | Yes | Yes | ssh.go, ssh_test.go | PASS |
| 2.1 Type resolution | Yes | Yes | Yes | env.go, env_create_test.go | PASS |
| 3.1 --global/--local flags | Depends on 2.1 | Yes | Yes | env.go, env_create_test.go | PASS |
| 4.1 List SOURCE/PATH | Yes | Yes | Yes | env.go | PASS |
| 5.1 Status project path | Yes | Yes | Yes | env.go | PASS |
| 6.1 Documentation | Yes | Yes (make test/lint) | Yes | README.md, cli.adoc | PASS |

## Phase Ordering Assessment

The phase ordering is correct:
1. Phase 1 (SSH delete) is independent and delivers immediate value
2. Phase 2 (type resolution) is the core fix, no dependencies
3. Phase 3 (flags) correctly depends on Phase 2's resolution refactoring
4. Phase 4 (list) and Phase 5 (status) are independent of each other and of Phases 1-3
5. Phase 6 (docs) correctly comes last

Tasks 1.1, 4.1, and 5.1 could be parallelized since they are independent.

## Recommendation

**Proceed to implementation.** The plan has full FR coverage, well-scoped tasks, and a logical phase ordering. The only advisory note is the breaking change to table output (PATH removal), which is acceptable for a pre-1.0 tool.
