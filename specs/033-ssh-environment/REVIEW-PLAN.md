# Plan Review: SSH Remote Execution Environment

**Branch**: `033-ssh-environment` | **Reviewed**: 2026-04-07

## Coverage Matrix

Maps every functional requirement (FR) and success criterion (SC) to the plan phase that implements it.

| Requirement | Description | Plan Phase | Coverage |
|-------------|-------------|------------|----------|
| FR-001 | SSH environment type | Phase 1 | FULL |
| FR-002 | Definition with host + type:ssh | Phase 1 (task 2) | FULL |
| FR-003 | SSH config overrides + workspace default | Phase 1 (task 2) | FULL |
| FR-004 | Respect ~/.ssh/config | Phase 1 (task 4, client) | FULL |
| FR-005 | Pre-flight checks during create | Phase 3 | FULL |
| FR-006 | Automated tool installation | Phase 3 (task 1-2) | FULL |
| FR-007 | Nested Zellij detection | Phase 2 (task 1) | FULL |
| FR-008 | Create remote Zellij session | Phase 2 (task 1) | FULL |
| FR-009 | Replace local process (syscall.Exec) | Phase 2 (task 1) | FULL |
| FR-010 | Remote session persists after detach | Phase 2 (inherent) | FULL |
| FR-011 | Auth modes (auto/api/vertex/bedrock/none) | Phase 4 (task 1) | FULL |
| FR-012 | Persist credentials to file | Phase 4 (task 1) | FULL |
| FR-013 | Refresh credentials on attach | Phase 4 (task 2) | FULL |
| FR-014 | File-based credentials (GCP JSON) | Phase 4 (task 1) | FULL |
| FR-015 | Explicit credentials list | Phase 4 (task 1) | FULL |
| FR-016 | Arbitrary env vars via env field | Phase 4 (task 1) | FULL |
| FR-017 | Credential refresh without attach | Phase 4 (task 3) | FULL |
| FR-018 | Status via SSH query | Phase 2 (task 2) | FULL |
| FR-019 | Push/Pull via rsync | Phase 5 (task 1-2) | FULL |
| FR-020 | Exec in workspace directory | Phase 5 (task 3) | FULL |
| FR-021 | Harvest via temp git remote | Phase 5 (task 4) | FULL |
| FR-022 | Start/Stop as ErrNotSupported | Phase 1 (task 5) | FULL |
| FR-023 | Delete with optional force | Phase 1 (task 5) | FULL |
| FR-024 | Update LastAttached | Phase 2 (task 1) | FULL |
| FR-025 | Interface contract compliance | Phase 1-6 (all) | FULL |
| FR-026 | Parallel status reconciliation | Phase 6 (task 1-2) | FULL |
| FR-027 | System ssh binary | Phase 1 (task 4) | FULL |
| SC-001 | Define/create/attach < 5 min | Phase 1-2 | FULL |
| SC-002 | Session persists after detach | Phase 2 | FULL |
| SC-003 | Pre-flight < 30s | Phase 3 | FULL |
| SC-004 | Status < 10s | Phase 2 + Phase 6 | FULL |
| SC-005 | Incremental file sync | Phase 5 | FULL |
| SC-006 | Harvest + PR in one op | Phase 5 (task 4) | FULL |
| SC-007 | Credentials persist across cycles | Phase 4 | FULL |
| SC-008 | Credential refresh < 5s | Phase 4 (task 3) | FULL |
| SC-009 | Backward compatibility | Phase 6 (task 3) | FULL |

**Coverage**: 27/27 FRs covered, 9/9 SCs covered. **100% coverage.**

## User Story Traceability

| User Story | Priority | Plan Phases |
|------------|----------|-------------|
| US-1: Connect to remote machine | P1 | Phase 1, 2 |
| US-2: Pre-flight bootstrap | P1 | Phase 3 |
| US-3: Credential forwarding | P2 | Phase 4 |
| US-4: Remote status/monitoring | P2 | Phase 2, 6 |
| US-5: Refresh credentials | P2 | Phase 4 |
| US-6: File synchronization | P3 | Phase 5 |
| US-7: Remote command execution | P3 | Phase 5 |
| US-8: Harvest git commits | P3 | Phase 5 |

All 8 user stories are covered. Priority ordering (P1 first) is respected in phase sequencing.

## Red Flags

### RF-1: Definition Field YAML Tag Convention (LOW)

The data model shows definition fields using kebab-case YAML tags (`identity-file`, `jump-host`) matching existing definition patterns, while SSHFields in state uses snake_case (`identity_file`). This is consistent with the existing codebase (definitions use kebab-case, state uses snake_case), but should be verified during implementation.

**Impact**: Low. Follows existing convention.

### RF-2: Credential File Sourcing Mechanism (LOW)

The plan references Zellij layout ENV directive for credential sourcing, but does not detail how the cc-deck layout on the remote will be configured to source `~/.config/cc-deck/credentials.env`. The layout is deployed during pre-flight bootstrap (Phase 3), and must include the sourcing mechanism.

**Impact**: Low. The cc-deck layout already supports customization via the plugin install process (Phase 3 installs the cc-deck plugin which includes the layout).

### RF-3: No Explicit Timeout Configuration (INFORMATIONAL)

SC-004 requires status checks within 10 seconds, but the plan does not specify a configurable SSH connection timeout. The implementation should use a hardcoded default (e.g., 5 seconds for status, 10 seconds for pre-flight) that can be made configurable later if needed.

**Impact**: Informational. Implementation detail, not a plan gap.

## Task Quality Assessment

| Phase | Task Count | Clarity | Testability | Dependencies |
|-------|-----------|---------|-------------|--------------|
| Phase 1 | 8 | HIGH | HIGH | None |
| Phase 2 | 4 | HIGH | HIGH | Phase 1 |
| Phase 3 | 4 | HIGH | MEDIUM | Phase 1 |
| Phase 4 | 4 | HIGH | HIGH | Phase 2 |
| Phase 5 | 5 | HIGH | HIGH | Phase 1, 2 |
| Phase 6 | 5 | HIGH | MEDIUM | All prior |
| Phase 7 | 5 | HIGH | HIGH | All prior |

**Task clarity**: All tasks describe what to implement and where. File paths and method signatures are specified.
**Testability**: Most phases have explicit "unit tests" tasks. Phase 3 and 6 have lower testability scores due to interactive prompts and integration test requirements.
**Dependencies**: Clean dependency chain. No circular dependencies. P1 stories (Phase 1-2) are implemented first.

## Constitution Compliance

All 14 constitution principles checked in the plan. 10 PASS, 3 PENDING (documentation, to be completed in Phase 7), 1 N/A.

PENDING items are all in Phase 7 (documentation) which is the final phase. This is the correct sequencing.

## Research Quality

10 decisions documented in research.md. Each includes:
- Decision statement
- Rationale
- Alternatives considered

All 4 spec clarifications (credential sourcing, workspace default, harvest mechanism, auto auth order) are reflected in research decisions.

## Verdict

**APPROVED.** The plan has 100% requirement coverage, clean phase sequencing aligned with priority ordering, and no blocking red flags. All informational items are implementation details that do not affect the plan structure.

### Recommended Review Focus

For reviewers of the spec PR:
1. **Phase 1 task 4** (SSH client): Verify the proposed API surface covers all SSH operations needed
2. **Phase 3** (pre-flight): Confirm the interactive UX for tool installation is appropriate for the target users
3. **Phase 4** (credentials): Review the credential file format and sourcing mechanism for security implications
4. **Phase 5 task 4** (harvest): Verify the temporary git remote approach works with SSH transport URLs
