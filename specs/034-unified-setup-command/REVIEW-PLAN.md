# Plan Review: 034-unified-setup-command

**Reviewed**: 2026-04-08 (updated post-clarification) | **Reviewer**: spex:review-plan

## Coverage Matrix

| Requirement | Task(s) | Status |
|-------------|---------|--------|
| FR-001: CLI `cc-deck setup init` | 1.3, 2.1, 2.2, 2.3 | COVERED |
| FR-002: `--target` flag on init | 2.1, 2.3 | COVERED |
| FR-003: Install two Claude commands | 3.1, 3.2, 3.4 | COVERED |
| FR-004: `/cc-deck.capture` target-agnostic | 3.1 | COVERED |
| FR-005: `/cc-deck.build --target` dispatch | 3.2, 3.3 | COVERED |
| FR-006: Container build + push | 3.2 | COVERED |
| FR-007: SSH Ansible playbook generation | 3.3 | COVERED |
| FR-008: Idempotent standalone playbooks | 3.3 | COVERED |
| FR-009: Self-correction loop (3 retries) | 3.2, 3.3 | COVERED |
| FR-010: `cc-deck setup verify` | 5.1 | COVERED |
| FR-011: `cc-deck setup diff` | 5.2 | COVERED |
| FR-012: Dual target manifest | 1.2, 2.3 | COVERED |
| FR-013: SSH create_user with sudo + identity_file .pub key | 3.3 | COVERED (base role, clarified) |
| FR-014: Credential sourcing in shell | 3.3 | COVERED (shell_config role) |
| FR-015: Ansible availability check | 3.3 | COVERED |
| FR-016: `cc-deck plugin install` on remote | 3.3 | COVERED (cc_deck role) |
| FR-017: Rename image -> setup | 1.1, 1.3 | COVERED |
| FR-018: Lightweight probe | 4.1, 4.2 | COVERED |
| FR-019: Multiple envs same host | 4.2 | COVERED |

### User Story Coverage

| Story | Priority | Tasks | Status |
|-------|----------|-------|--------|
| US-1: Initialize setup profile | P1 | 1.3, 2.1, 2.2, 2.3, 3.1 | COVERED |
| US-2: Build container image | P1 | 1.2, 3.2, 3.4 | COVERED |
| US-3: Provision SSH via Ansible (6 AS) | P1 | 3.3, 4.1, 4.2 | COVERED |
| US-4: Single capture, dual target | P2 | 3.1 (target-agnostic capture) | COVERED |
| US-5: Detect manifest drift | P3 | 5.2 | COVERED |
| US-6: Verify provisioned target | P3 | 5.1 | COVERED |

### Success Criteria Coverage

| Criterion | Validation | Tasks |
|-----------|-----------|-------|
| SC-001: Bare VM to provisioned in <15min | Integration test | 7.3 |
| SC-002: Single capture drives both targets | Integration test | 7.3 |
| SC-003: Standalone playbook re-run | Integration test | 7.3 |
| SC-004: 80% self-correction success | Integration test | 7.3 |
| SC-005: F-001 through F-010 resolved | Integration test | 7.3 |
| SC-006: Verify detects missing tools | Unit + integration test | 5.1, 7.3 |

### Edge Case Coverage

| Edge Case | Handled By |
|-----------|-----------|
| Ansible not installed | Task 3.3 (FR-015 check) |
| SSH host unreachable | Task 3.3 (Ansible connection error, no retry) |
| Missing target section in manifest | Task 3.2, 3.3 (validation) |
| Roles directory not scaffolded | Task 3.3 (creates on first run) |
| 3 retries exhausted | Task 3.3 (stop and report) |
| Both targets, shared manifest change | Task 5.2 (diff reports both) |
| Ansible role files manually edited | Task 3.3 (diff-and-ask, per clarification) |

## Red Flag Scan

| # | Category | Finding | Severity | Recommendation |
|---|----------|---------|----------|----------------|
| 1 | Scope | Task 3.3 (SSH build command) is estimated L and covers 8 spec requirements (FR-005, FR-007-009, FR-013-016). Consider splitting. | MEDIUM | The task is a single Claude command file, so splitting it further would create artificial boundaries. The L estimate is appropriate. Accept as-is. |
| 2 | Risk | Task 7.3 (integration test) is high-risk and depends on external infrastructure (Hetzner VM). | LOW | This is inherent to the feature. The unit tests (7.1, 7.2) cover the code paths; the integration test validates end-to-end. No change needed. |
| 3 | Dependencies | Phase 4 (bootstrap simplification) is listed as parallelizable with Phase 3, but Task 4.2 deletes `bootstrap.go` which could conflict with Phase 3 work. | LOW | The code paths are independent (`internal/ssh/bootstrap.go` vs `internal/setup/commands/`). Parallel execution is safe. |
| 4 | Missing | No task for updating `.goreleaser.yaml` if the binary name or package structure changes. | INFO | The rename is `internal/build` -> `internal/setup`, which is internal. GoReleaser references the binary name (`cc-deck`) which is unchanged. No action needed. |

## Task Quality Assessment

| Task | Subject | Estimate | Has Files | Has Spec Ref | Has Acceptance | Grade |
|------|---------|----------|-----------|--------------|----------------|-------|
| 1.1 | Rename internal/build to internal/setup | S | Yes | FR-017 | Yes | A |
| 1.2 | Evolve manifest struct with Targets | M | Yes | FR-012 | Yes | A |
| 1.3 | Rename CLI command from image to setup | S | Yes | FR-017 | Yes | A |
| 2.1 | Add --target flag to init command | M | Yes | FR-001, FR-002 | Yes | A |
| 2.2 | Scaffold Ansible role skeletons | M | Yes | FR-001, FR-007 | Yes | A |
| 2.3 | Update manifest template for dual targets | S | Yes | FR-002, FR-012 | Yes | A |
| 3.1 | Update /cc-deck.capture command | M | Yes | FR-004 | Yes | A |
| 3.2 | Update /cc-deck.build for container target | M | Yes | FR-005, FR-006 | Yes | A |
| 3.3 | Add /cc-deck.build SSH target section | L | Yes | FR-005,007-009,013-016 | Yes | A |
| 3.4 | Remove /cc-deck.push command | S | Yes | FR-003 | Yes | A |
| 4.1 | Create lightweight probe | S | Yes | FR-018 | Yes | A |
| 4.2 | Simplify SSH environment Create() | M | Yes | FR-018, FR-019 | Yes | A |
| 5.1 | Add --target flag to verify command | M | Yes | FR-010 | Yes | A |
| 5.2 | Add --target flag to diff command | M | Yes | FR-011 | Yes | A |
| 6.1 | Update README.md | M | Yes | Constitution IX | Yes | A |
| 6.2 | Update CLI reference documentation | M | Yes | Constitution IX | Yes | A |
| 6.3 | Create Antora guide page | L | Yes | Constitution IX | Yes | A |
| 6.4 | Update landing page | S | Yes | Constitution IX | Yes | A |
| 7.1 | Unit tests for manifest evolution | M | Yes | - | Yes | A |
| 7.2 | Unit tests for probe | S | Yes | - | Yes | A |
| 7.3 | Integration test against SSH target | L | Yes | SC-001-006 | Yes | A |

## NFR Validation

| NFR | Status | Notes |
|-----|--------|-------|
| Documentation | PLANNED | Tasks 6.1-6.4 cover README, CLI reference, Antora guide, landing page |
| Testing | PLANNED | Tasks 7.1-7.3 cover unit and integration testing |
| Constitution compliance | PASS | All applicable principles addressed in plan |
| Prose plugin | REQUIRED | Tasks 6.1-6.4 explicitly note prose plugin usage |

## Verdict

**PASS** - The plan provides complete coverage of all 19 functional requirements, 6 user stories (including US-3 with 6 acceptance scenarios post-clarification), 6 success criteria, and 7 edge cases. All 21 tasks have clear subjects, file references, spec traceability, and acceptance criteria. No critical red flags identified.

### Post-Clarification Updates

Two clarifications from session 2026-04-08 were integrated:
1. **SSH role re-generation conflicts**: Task 3.3 updated to include AS-6 (diff-and-ask for role conflicts). Edge case coverage updated.
2. **SSH authorized key source**: FR-013 clarified to use `identity_file` `.pub` counterpart. Task 3.3 spec refs expanded to include FR-013.

### Recommendations

1. Task 3.3 is the largest single task (L, 8 FRs). During implementation, consider splitting the Claude command into sections if it exceeds the context window.
2. Integration testing (Task 7.3) should be the final validation step and may require multiple iterations.
3. Documentation tasks (Phase 6) should use the prose plugin per Constitution Principle XII.
