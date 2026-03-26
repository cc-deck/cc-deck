# Review Guide: 024 Container Environment

**Feature**: Container Environment (single `podman run` lifecycle)
**Branch**: `024-container-env`
**Date**: 2026-03-20

## Review Summary

| Aspect | Score | Notes |
|--------|-------|-------|
| Spec Completeness | 9/10 | Strong. Minor ambiguities around credential schema and `--all-ports` semantics |
| Plan Quality | 9/10 | Clean architecture. Shared podman package justified. |
| Task Coverage | 9/10 | All 18 FRs covered. One medium issue with factory signature change. |
| Task Quality | 8/10 | Good format compliance. One broad task (T004). |
| Overall | 9/10 | Ready for implementation |

## Coverage Matrix: Functional Requirements

| FR | Description | Tasks | Status |
|----|-------------|-------|--------|
| FR-001 | Container type via `podman run` | T020-T027, T008-T013 | COVERED |
| FR-002 | Definition/state separation | T014-T015, T017-T018 | COVERED |
| FR-003 | Shared podman interaction layer | T008-T013 | COVERED |
| FR-004 | Named volume + bind mount storage | T023, T028 | COVERED |
| FR-005 | `sleep infinity` entrypoint | T023 | COVERED |
| FR-006 | Attach via `podman exec -it` | T024 | COVERED |
| FR-007 | Credentials via podman secrets | T044-T046 | COVERED |
| FR-008 | Auto-detect host env vars | T044 | COVERED |
| FR-009 | File transfer via `podman cp` | T041-T043 | COVERED |
| FR-010 | Reconciliation via `podman inspect` | T036-T039 | COVERED |
| FR-011 | Rootless podman auto-detection | T008 | COVERED |
| FR-012 | Explicit port exposure only | T028, T020 | COVERED |
| FR-013 | Delete volumes by default, `--keep-volumes` | T025, T030 | COVERED |
| FR-014 | Full resource cleanup on delete | T025 | COVERED |
| FR-015 | Demo image fallback with warning | T032 | COVERED |
| FR-016 | Reject unsupported operations | T026 | COVERED |
| FR-017 | Config-file defaults | T031 | COVERED |
| FR-018 | Auto-start on attach | T024 | COVERED |

## Coverage Matrix: Success Criteria

| SC | Description | Tasks | Status |
|----|-------------|-------|--------|
| SC-001 | Create + attach < 30s | T023, T024, T057 | COVERED |
| SC-002 | Stop/restart preserves data | T033-T034, T057 | COVERED |
| SC-003 | List reconciliation < 2s | T037-T038, T057 | COVERED |
| SC-004 | Credentials not in inspect | T044-T046 | COVERED |
| SC-005 | File transfer up to 100MB | T041-T043, T057 | COVERED |
| SC-006 | Future compose needs only new file | T008-T013 (shared), T014-T015 (shared schema) | COVERED |
| SC-007 | Hand-edit definitions respected | T050-T051 | COVERED |

## Coverage Matrix: User Stories

| Story | Priority | Tasks | Count | Independent Test |
|-------|----------|-------|-------|-----------------|
| US1: Create/Attach/Delete | P1 | T020-T032 | 13 | Yes |
| US2: Stop/Start | P1 | T033-T035 | 3 | Yes |
| US3: List/Inspect | P1 | T036-T040 | 5 | Yes |
| US4: File Transfer | P2 | T041-T043 | 3 | Yes |
| US5: Credentials | P2 | T044-T047 | 4 | Yes |
| US6: Exec | P3 | T048-T049 | 2 | Yes |
| US7: Hand-Edit | P3 | T050-T051 | 2 | Yes |

## Red Flags

### Medium: Factory Signature Change (T027)

**Issue**: T027 adds `DefinitionStore` to the factory, but the current `NewEnvironment()` signature only takes `FileStateStore`. Changing it requires updating `LocalEnvironment` construction too (even though it does not use the definition store). This is not explicitly covered by any task.

**Recommendation**: Add a note to T027 that the factory signature must be updated, and LocalEnvironment's construction must accept (and ignore) the definition store parameter. Alternatively, create the `ContainerEnvironment` directly in the factory without changing the function signature by passing the definition store through a package-level variable or constructor option.

### Low: T004 Breadth

**Issue**: T004 says "Update all references to old type/field names across `cc-deck/internal/env/*.go` and `cc-deck/internal/cmd/env.go`". This is broad. The implementor should use `rg` to find all `EnvironmentTypePodman`, `PodmanFields`, and `.Podman` references.

**Recommendation**: Acceptable as-is since a simple search-and-replace covers it. The follow-up T007 (`make test` passes) validates completeness.

### Low: US5 Modifies US1's Create Flow

**Issue**: US5 (credentials) adds logic to the `Create()` method implemented in US1. If done sequentially (as intended), this is fine. But if parallelized, merge conflicts are likely.

**Recommendation**: The dependency graph correctly shows US5 depends on US1. No action needed as long as US5 is implemented after US1.

### Info: Edge Case - `--storage host-path` Without `--path`

**Issue**: The spec says to use current working directory when `--path` is omitted with `--storage host-path`. This is implicitly handled in T028/T029 flag handling but not called out explicitly.

**Recommendation**: The implementor should add this default in T029 when processing the `--path` flag.

## Review Checklist for Implementation

When reviewing implementation PRs, verify:

- [ ] `EnvironmentTypePodman` is fully removed (no remnants)
- [ ] `PodmanFields` is fully renamed to `ContainerFields`
- [ ] `internal/podman/` package functions are idempotent for remove operations
- [ ] `podman inspect` nil return (container not found) is handled gracefully
- [ ] Secret values are never logged or printed
- [ ] Partial cleanup failures produce warnings, not errors (FR-014)
- [ ] Demo image fallback shows a user-visible warning (FR-015)
- [ ] `Harvest()` returns clear error suggesting push/pull (FR-016)
- [ ] Auto-start on attach works for stopped containers (FR-018)
- [ ] Definition store uses atomic writes (tmp + rename)
- [ ] State store version is 2
- [ ] `CC_DECK_DEFINITIONS_FILE` env override works in tests
- [ ] `make test` and `make lint` pass
- [ ] README spec table updated
- [ ] Documentation uses prose plugin with cc-deck voice

## Artifacts to Review

| File | Purpose |
|------|---------|
| `specs/024-container-env/spec.md` | Feature specification |
| `specs/024-container-env/plan.md` | Implementation plan |
| `specs/024-container-env/research.md` | Research decisions |
| `specs/024-container-env/data-model.md` | Entity definitions and schemas |
| `specs/024-container-env/quickstart.md` | Usage examples |
| `specs/024-container-env/contracts/podman-package.md` | Podman package API |
| `specs/024-container-env/contracts/container-environment.md` | ContainerEnvironment contract |
| `specs/024-container-env/contracts/definition-store.md` | DefinitionStore contract |
| `specs/024-container-env/contracts/cli-extensions.md` | CLI flag extensions |
| `specs/024-container-env/tasks.md` | Task breakdown (58 tasks) |
