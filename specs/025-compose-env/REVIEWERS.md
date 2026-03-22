# Review Guide: 025-compose-env (Compose Environment)

**Feature**: Compose Environment with Multi-Container Orchestration
**Branch**: `025-compose-env`
**Date**: 2026-03-21

## Summary

This spec adds a `compose` environment type that uses `podman-compose` for multi-container orchestration with optional network filtering via a tinyproxy proxy sidecar. It implements the existing `Environment` interface, reusing compose YAML generation, domain resolution, and auth detection from previous features.

## Review Checklist

### Spec Review (`spec.md`)

- [ ] All 5 user stories have clear acceptance scenarios with Given/When/Then format
- [ ] 23 functional requirements cover the full feature surface
- [ ] Edge cases are enumerated (7 cases)
- [ ] Success criteria are measurable (6 criteria)
- [ ] Interface contract reference is explicit (023-env-interface behavioral requirements)
- [ ] Priorities are consistent: P1 (core create/attach), P2 (filtering, lifecycle, credentials), P3 (gitignore)

### Plan Review (`plan.md`)

- [ ] Technical context matches project reality (Go 1.25, cobra, internal packages)
- [ ] Constitution check covers all 13 principles with correct status
- [ ] Project structure shows all new/modified files with clear rationale
- [ ] No new external dependencies introduced
- [ ] Reuses existing packages (internal/compose, internal/podman, internal/network)

### Tasks Review (`tasks.md`)

- [ ] All 23 FRs are mapped to at least one task
- [ ] Task format follows `- [ ] [ID] [P?] [Story] Description with file path`
- [ ] Dependencies are correct (Phase 2 blocks all stories, US1 blocks US2-5)
- [ ] Parallel opportunities are correctly identified
- [ ] MVP scope is clear (Phase 1-3, 14 tasks)
- [ ] Each user story phase has independent test criteria

### Data Model Review (`data-model.md`)

- [ ] `ComposeFields` struct matches brainstorm D9
- [ ] `EnvironmentInstance.Type` field is backward compatible
- [ ] `EnvironmentDefinition` additions are `omitempty` for backward compatibility
- [ ] Credential flow diagram matches the .env file approach (not podman secrets)
- [ ] Naming conventions are consistent with container type

### Research Review (`research.md`)

- [ ] All 8 decisions have rationale and alternatives considered
- [ ] R-02 (credential injection via .env) is justified against podman secrets
- [ ] R-06 (Type field on EnvironmentInstance) is backward compatible
- [ ] R-07 (push/pull via exec+tar or podman cp) is consistent with codebase

## Key Areas to Focus On

### 1. Interface Contract Compliance (HIGH PRIORITY)

The compose environment MUST satisfy all behavioral requirements from `specs/023-env-interface/contracts/environment-interface.md`:
- Nested Zellij detection on Attach
- Session creation with `--layout cc-deck`
- Auto-start on Attach
- LastAttached timestamp update
- Name validation on Create
- Cleanup on failure (FR-020)
- Running check on Delete

**Verify**: T011 (Create), T012 (Attach), T016 (Delete) descriptions match these requirements.

### 2. Auth Helper Extraction (MEDIUM PRIORITY)

T002 extracts `detectAuthMode()`, `detectAuthCredentials()`, and `containerHasZellijSession()` from `container.go`. This is a refactoring task that must not break the existing container type.

**Verify**: After T002, `make test` must pass with no regressions. The exported function signatures must be backward compatible.

### 3. Storage Default Override (MEDIUM PRIORITY)

Container type defaults to `named-volume`. Compose type defaults to `host-path` (FR-003). The CLI must switch defaults based on `--type`.

**Verify**: T008 and T011 handle this correctly.

### 4. Credential Injection Mechanism (MEDIUM PRIORITY)

Compose uses `.env` file (not podman secrets) for environment variable credentials. File-based credentials are copied to `.cc-deck/secrets/` and mounted via compose volumes. The detection logic is shared with container type.

**Verify**: T020 implements compose-native injection correctly. T021 tests parity with container type.

## Artifacts

| File | Purpose |
|------|---------|
| `spec.md` | Feature specification with user stories and requirements |
| `plan.md` | Implementation plan with technical context and constitution check |
| `tasks.md` | 33 tasks organized by user story with dependencies |
| `research.md` | 8 resolved design decisions with rationale |
| `data-model.md` | Entity model, state transitions, credential flow |
| `quickstart.md` | Usage guide for compose environments |
| `REVIEWERS.md` | This file |

## Review Verdict Template

```
APPROVED / APPROVED WITH COMMENTS / CHANGES REQUESTED

Findings:
- ...
```
