# Tasks: Credential Transport Abstraction (Detect-All Revision)

**Input**: Design documents from `/specs/069-credential-transport/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/credential-transport.md

**Tests**: Included per constitution (every feature MUST include tests).

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

**Context**: The foundational credential package (`internal/credential`), agent interface changes, and transport functions already exist from the initial implementation. These tasks focus on the detect-all/opt-out model revision.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

---

## Phase 1: Setup

**Purpose**: Add new types and functions for the detect-all model

- [X] T001 Add `DetectedMode` type (AgentName, Spec, Resolved) to `cc-deck/internal/credential/types.go`
- [X] T002 Remove `Agent string` and `AuthMode string` fields from `WorkspaceSpec` in `cc-deck/internal/ws/definition.go`
- [X] T003 Remove `Agent string` field from `WorkspaceInstance` in `cc-deck/internal/ws/types.go`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Implement DetectAll, conflict detection, and credential merging that ALL user stories depend on

**CRITICAL**: No user story work can begin until this phase is complete

- [X] T004 Implement `DetectAll() []DetectedMode` in `cc-deck/internal/credential/resolve.go` that scans all registered agents via `agent.All()`, calls `Detect()` per agent, and returns all available modes with agent names
- [X] T006 Implement `MergeCredentials(modes []DetectedMode) (ResolvedCredentials, error)` in `cc-deck/internal/credential/resolve.go` that merges env vars (dedup same-name same-value, error on same-name different-value), collects file credentials, merges UnsetVars
- [X] T008 [P] Write unit tests for `DetectAll()` in `cc-deck/internal/credential/resolve_test.go` covering: multiple agents detected, single agent only, no credentials available
- [X] T010 [P] Write unit tests for `MergeCredentials()` in `cc-deck/internal/credential/resolve_test.go` covering: disjoint merge, same-key same-value dedup, file credential collection, UnsetVars merge

**Checkpoint**: Foundation ready. DetectAll and merging work. Workspace integration can begin.

---

## Phase 3: User Story 2 - Automatic multi-agent credential injection (Priority: P1) MVP

**Goal**: `cc-deck ws new` detects all available credentials from all agents, prompts on same-agent conflicts, and injects the merged set.

**Independent Test**: Set `ANTHROPIC_API_KEY` and `OPENAI_API_KEY`. Run `cc-deck ws new`. Verify both are injected without prompting.

### Implementation for User Story 2

- [X] T012 [US2] Replace `selectAuthMode()` in `cc-deck/internal/cmd/ws.go` with `resolveWorkspaceCredentials()` that calls `DetectAll()` and logs detected modes
- [X] T014 [US2] Remove `--agent` flag from `ws new` in `cc-deck/internal/cmd/ws.go` (keep `--auth` deprecated flag for legacy)
- [X] T016 [US2] Update `runWsNew()` to inject merged credentials via the appropriate transport function in `cc-deck/internal/cmd/ws.go`
- [X] T017 [US2] Write unit tests for `resolveWorkspaceCredentials()` in `cc-deck/internal/cmd/ws_new_test.go` covering: auto-injects all, no credentials warns

**Checkpoint**: US2 complete. `ws new` detects all credentials and injects them.

---

## Phase 4: User Story 5 - Credential injection across workspace types (Priority: P1)

**Goal**: All workspace types use the detect-all model instead of single-agent credential resolution.

**Independent Test**: Create a container workspace and an SSH workspace on a host with both Claude and OpenCode credentials. Verify both workspace types receive the full merged credential set.

### Implementation for User Story 5

- [X] T020 [US5] Refactor `cc-deck/internal/ws/container.go` Create method: add `DetectAll()` + per-mode `InjectContainer()` as primary path with legacy fallback
- [X] T021 [US5] Refactor `cc-deck/internal/ws/compose.go` Create method: add detect-all flow with legacy fallback
- [X] T022 [US5] Refactor `cc-deck/internal/ws/ssh.go` Attach method: add detect-all flow using `MergeCredentials()` + `InjectSSH()` with legacy fallback
- [X] T023 [US5] Refactor `cc-deck/internal/ws/k8s_deploy.go` Create method: add detect-all flow with legacy fallback
- [X] T025 [P] [US5] Update `cc-deck/internal/ws/definition_test.go`: update YAML round-trip tests for new field structure

**Checkpoint**: US5 complete. All workspace types use detect-all. Legacy dual-path code removed.

---

## Phase 6: User Story 4 - Eager credential validation at workspace start (Priority: P2)

**Goal**: Credentials are validated for all non-excluded modes before launching containers or remote sessions.

**Independent Test**: Create a workspace with vertex credentials available. Unset the credential file. Start the workspace and verify the error message names the missing credential.

### Implementation for User Story 4

- [X] T026 [US4] Update `Validate()` in `cc-deck/internal/credential/validate.go` to accept `[]DetectedMode` (validate all modes, not just one spec)
- [X] T027 [US4] Integrate multi-mode validation at workspace start in container.go: call `ValidateAll(detectedModes, def.ExternalCredentials)` before infrastructure provisioning
- [X] T028 [US4] Write unit tests for multi-mode `Validate()` in `cc-deck/internal/credential/validate_test.go`

**Checkpoint**: US4 complete. Missing credentials caught before any container or remote session is created.

---

## Phase 7: User Story 6 - Workspace listing shows credentials in verbose mode (Priority: P2)

**Goal**: `cc-deck ws ls -v` displays the injected auth modes, derived at display time from agent registry + host environment + exclusions.

### Implementation for User Story 6

- [X] T029 [US6] Update `buildAuthMap()` in `cc-deck/internal/cmd/ws.go` to derive AUTH from `DetectAll()` instead of stored agent/auth-mode fields
- [X] T030 [US6] Remove `Agent` and `AuthMode` fields from `wsListEntry` in `cc-deck/internal/cmd/ws.go`, keep only `Auth` (derived)
- [X] T031 [US6] Update `writeWsStructured()` JSON/YAML output to include `auth` field (derived list of `agent/mode` strings) in `cc-deck/internal/cmd/ws.go`
- [X] T032 [US6] Write unit test verifying `ws ls -v` shows AUTH column and default hides it in `cc-deck/internal/cmd/ws_new_test.go`

**Checkpoint**: US6 complete. `ws ls -v` shows derived auth info.

---

## Phase 8: User Story 8 - Generalized SSH credential handling (Priority: P2)

**Goal**: SSH uses detect-all model instead of hardcoded credential maps.

### Implementation for User Story 8

- [X] T036 [US8] Refactor `cc-deck/internal/ssh/credentials.go`: deprecate `BuildCredentialSet()` and `detectAuthMode()`, add Deprecated markers; SSH Attach method uses `credential.DetectAll()` + `credential.MergeCredentials()` as primary path with legacy fallback

**Checkpoint**: US8 complete. SSH credentials work for any agent via detect-all (with legacy fallback).

---

## Phase 10: Polish & Cross-Cutting Concerns

**Purpose**: Documentation, cleanup, and removal of deprecated code

- [X] T041 Update README.md with detect-all credential model
- [X] T042 [P] Update CLI reference in `cc-deck/docs/modules/reference/pages/cli.adoc` with credential transport documentation
- [X] T044 Run `make test` and `make lint` to verify no regressions
- [ ] T045 Run quickstart.md validation scenarios manually

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies, start immediately
- **Foundational (Phase 2)**: Depends on Phase 1 completion. BLOCKS all user stories.
- **US2 (Phase 3)**: Depends on Phase 2. MVP target.
- **US5 (Phase 4)**: Depends on US2 (needs detect-all in ws new working first)
- **US4 (Phase 5)**: Depends on US5 (needs workspace types using detect-all)
- **US6 (Phase 6)**: Depends on US2. Can parallel with US5.
- **US8 (Phase 8)**: Depends on US5 (needs workspace types migrated)
- **Polish (Phase 9)**: Depends on all user stories being complete

### Parallel Opportunities

- T008/T009/T010/T011: All foundational tests can be written in parallel
- T020-T024: Workspace type refactoring can be partially parallelized (different files)
- T039-T043: All polish tasks can run in parallel

---

## Implementation Strategy

### MVP First (US2 Only)

1. Complete Phase 1: Setup (types)
2. Complete Phase 2: Foundational (DetectAll, merge)
3. Complete Phase 3: US2 (detect-all in ws new)
4. **STOP and VALIDATE**: Create workspaces with multiple agent credentials. Verify auto-detection and injection.

### Incremental Delivery

1. Setup + Foundational + US2 = Detect-all workspace creation (MVP)
2. Add US5 = All workspace types use detect-all
3. Add US4 = Eager validation for all detected modes
4. Add US6 = Visible auth info in `ws ls -v`
5. Add US8 = SSH migration
6. Polish = Documentation, cleanup

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story should be independently completable and testable
- Commit after each task or logical group
- Stop at any checkpoint to validate story independently
- Per constitution: documentation (T041-T043) MUST ship with the feature
- Per spec FR-016: never log credential values
- Per spec FR-017: all credential files 0600
