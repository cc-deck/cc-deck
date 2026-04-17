# Review Plan: Remote Workspace Repository Provisioning

**Branch**: `038-workspace-repos` | **Date**: 2026-04-17

## Quality Checks Completed

| Check | Status | Score |
|-------|--------|-------|
| Spec Review (`/spex:review-spec`) | PASS | 8.7/10 |
| Clarification (`/speckit-clarify`) | PASS | 5/5 questions resolved |
| Plan Review (`/spex:review-plan`) | PASS | See below |

## Coverage Matrix

Maps spec requirements (FR-001 to FR-026) to implementation tasks.

| Requirement | Description | Task(s) | Status |
|-------------|-------------|---------|--------|
| FR-001 | Repos field on definition | 1.3 | Covered |
| FR-002 | RepoEntry fields (url, branch, target) | 1.1 | Covered |
| FR-003 | Clone during env create | 1.2 | Covered |
| FR-004 | Default target from URL | 1.1 | Covered |
| FR-005 | Custom target directory | 1.1 | Covered |
| FR-006 | Skip existing directories | 1.2 | Covered |
| FR-007 | Clone failures as warnings | 1.2 | Covered |
| FR-008 | Idempotent re-run | 1.2, 6.1 | Covered |
| FR-009 | Parallel cloning (max 4) | 1.2 | Covered |
| FR-010 | Auto-detect origin remote | 4.2 | Covered |
| FR-011 | Add extra remotes after clone | 4.2 | Covered |
| FR-012 | Auto-detected repo as working dir | 4.2 | Covered |
| FR-013 | Deduplicate auto-detected URL | 1.1, 4.2 | Covered |
| FR-014 | Env type support + local warning | 3.1-3.4, 5.1 | Covered |
| FR-015 | SSH: clone via SSH exec | 3.1 | Covered |
| FR-016 | Container: clone via podman exec | 3.2 | Covered |
| FR-017 | Compose: clone in primary container | 3.3 | Covered |
| FR-018 | K8s: clone via kubectl exec | 3.4 | Covered |
| FR-019 | Workspace resolution from definition | 3.1-3.4 | Covered |
| FR-020 | Use Profile credential model | 3.1 | Covered |
| FR-021 | SSH agent forwarding (SSH only) | 2.1, 3.1 | Covered |
| FR-022 | Token-based auth for HTTPS | 1.2 | Covered |
| FR-023 | Token injection + post-clone cleanup | 1.2 | Covered |
| FR-024 | --repo CLI flag (repeatable) | 4.1 | Covered |
| FR-025 | --branch flag with validation | 4.1 | Covered |
| FR-026 | CLI repos not persisted | 4.1 | Covered |

**Coverage**: 26/26 requirements mapped to tasks (100%)

## Red Flag Scan

| Flag | Severity | Assessment |
|------|----------|------------|
| FR-020 credential resolution scope | Medium | **RESOLVED**: Added `resolveGitCredentials()` to Task 1.2. Tasks 3.1-3.4 now all call it with the active Profile. FR-020 mapped to all env type tasks. |
| Extra remotes data flow (Task 4.2) | Medium | **RESOLVED**: `cloneRepos()` signature updated with `extraRemotes map[string]string` parameter. Task 4.2 collects remotes as map, passes separately. RepoEntry stays simple. |
| No clone timeout specified | Low | Go context propagation handles this; each environment's Create() has a context. Individual clone timeout can be added during implementation if needed. |
| --branch pairing semantics | Low | Positional association is clear from the task description. Implementation will parse `--repo A --branch X --repo B` as: A gets branch X, B gets default. |
| Extra remote name conflicts | Low | FR-011 adds remotes that may already exist. Treat as warning per FR-007's non-fatal approach. |
| No rollback on partial clone failure | Low | By design (FR-007). Successfully cloned repos persist even if others fail. This is the correct behavior for provisioning. |

**No critical or high-severity red flags. Two medium findings identified and resolved in plan/tasks update.**

## Task Quality Assessment

| Criterion | Status | Notes |
|-----------|--------|-------|
| Each task has clear acceptance criteria | PASS | All 17 tasks have explicit acceptance conditions |
| Dependencies are explicit | PASS | Depends-on fields specified where applicable |
| Tasks are independently testable | PASS | Each task can be verified via `make test` |
| Effort estimates provided | PASS | Small/Medium/Large per task |
| Priority assigned | PASS | P1 (core), P2 (warning + docs), P3 (guide page) |
| Constitution compliance | PASS | All 14 principles checked in plan |
| Contract defined | PASS | `contracts/repo-cloning.md` with 10 behavioral requirements |

## NFR Validation

| NFR | Addressed | How |
|-----|-----------|-----|
| Performance | Yes | Max 4 concurrent clones (FR-009), overhead target in plan |
| Security | Yes | Token cleanup after clone (FR-023), SSH agent forwarding scope-limited (FR-021) |
| Idempotency | Yes | Directory existence check (FR-006, FR-008) |
| Error resilience | Yes | Non-fatal failures (FR-007) |
| Testing | Yes | Unit tests (Task 1.4), integration tests (Task 6.1), CLI tests (Task 6.2) |
| Documentation | Yes | README (Task 7.1), CLI ref (Task 7.2), guide (Task 7.3) |

## Reviewer Guidance

When reviewing this feature, focus on:

1. **Security**: Verify token injection in FR-023 implementation. Ensure token never persists in `.git/config` after clone. Check that `git remote set-url` runs even if post-clone steps (branch checkout, remote add) fail.

2. **Concurrency**: Verify the goroutine pool respects the max-4 limit. Check for races in result collection (use mutex or channel-based approach).

3. **URL normalization**: Edge cases in SSH URL parsing (`git@host:path` vs `ssh://git@host/path`). Test with GitHub, GitLab, and Bitbucket URL formats.

4. **Auto-detection merging**: Verify deduplication handles the case where auto-detected repo URL matches an explicit entry with different branch/target. The explicit entry should take precedence.

5. **Workspace resolution**: Confirm that the `Workspace` field from `EnvironmentDefinition` is used as the base path when set, and type defaults apply when empty. Test with SSH (resolves `~` on remote) and container types.

## Recommendation

The spec is sound (8.7/10), all clarifications are resolved, and the plan provides full coverage of all 26 requirements across 17 well-defined tasks. No blocking issues identified.

**Verdict**: Ready for implementation. Proceed with `/speckit.implement` or commit spec artifacts and create a spec PR.
