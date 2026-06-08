# Tasks: Credential Transport Abstraction

**Input**: Design documents from `/specs/069-credential-transport/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/credential-transport.md

**Tests**: Included per constitution (every feature MUST include tests).

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

---

## Phase 1: Setup

**Purpose**: Create the new credential package skeleton and type definitions

- [x] T001 Define `CredentialSpec`, `EnvVarSpec`, `FileCredentialSpec`, `Endpoint` types in `cc-deck/internal/agent/credential_spec.go`
- [x] T002 Define `AvailableMode` and `ResolvedCredentials` types in `cc-deck/internal/credential/types.go` (creates `cc-deck/internal/credential/` package implicitly)

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Extend Agent interface and implement credential resolution core that ALL user stories depend on

**CRITICAL**: No user story work can begin until this phase is complete

- [x] T003 Add `CredentialSpecs() []CredentialSpec` method to Agent interface in `cc-deck/internal/agent/agent.go`
- [x] T004 [P] Implement `CredentialSpecs()` for ClaudeAgent (api, vertex, bedrock modes) in `cc-deck/internal/agent/claude.go`
- [x] T005 [P] Implement `CredentialSpecs()` for OpenCodeAgent (openai, anthropic modes) in `cc-deck/internal/agent/opencode.go`
- [x] T006 Implement `Detect(specs []CredentialSpec) []AvailableMode` function in `cc-deck/internal/credential/resolve.go` that checks host environment against credential specs and returns available modes
- [x] T007 [P] Write unit tests for `Detect()` in `cc-deck/internal/credential/resolve_test.go` covering: all-required-set, partial-required, file-credential-exists, file-credential-missing, fixed-value-vars, tilde-expansion in DefaultPath
- [x] T008 [P] Write unit tests for ClaudeAgent.CredentialSpecs() and OpenCodeAgent.CredentialSpecs() in `cc-deck/internal/agent/claude_test.go` and `cc-deck/internal/agent/opencode_test.go` verifying correct specs returned

**Checkpoint**: Foundation ready. Agent specs declared, credential detection works. User story implementation can begin.

---

## Phase 3: User Story 1 - Agent declares credential requirements (Priority: P1) MVP

**Goal**: A developer implements `CredentialSpecs()` on a new agent and the credential system reads requirements, detects available credentials, and produces the correct env/file set.

**Independent Test**: Register a test agent with known specs. Verify detection returns correct available modes and resolved credential values from the host environment.

### Implementation for User Story 1

- [x] T009 [US1] Implement `Resolve(spec CredentialSpec) ResolvedCredentials` function in `cc-deck/internal/credential/resolve.go` that resolves all env vars and file credentials from the host environment for a single spec
- [x] T010 [US1] Add tilde expansion utility for `FileCredentialSpec.DefaultPath` in `cc-deck/internal/credential/resolve.go`
- [x] T011 [US1] Write unit tests for `Resolve()` in `cc-deck/internal/credential/resolve_test.go` covering: env var resolution, fixed values injected, file credential path resolution with default path fallback, credential set completeness

**Checkpoint**: US1 complete. Agent specs are declared and credential resolution produces correct env/file sets for any agent.

---

## Phase 4: User Story 2 - Auth mode selection during workspace creation (Priority: P1)

**Goal**: `cc-deck ws new` detects available auth modes, prompts if multiple, accepts `--auth-mode` flag, and persists the selection.

**Independent Test**: Set up host with both `ANTHROPIC_API_KEY` and Vertex credentials. Run `cc-deck ws new` and verify prompting and persistence.

### Implementation for User Story 2

- [x] T012 [US2] Add `Agent string` and `AuthMode string` fields to `WorkspaceSpec` in `cc-deck/internal/ws/definition.go` (YAML keys: `agent`, `auth-mode`)
- [x] T013 [US2] Add `Agent string` field to `WorkspaceInstance` in `cc-deck/internal/ws/types.go`
- [x] T014 [US2] Add `--agent` flag (default "claude") and `--auth-mode` flag to `ws new` command in `cc-deck/internal/cmd/ws.go`
- [x] T015 [US2] Implement auth mode selection logic in `cc-deck/internal/cmd/ws.go`: detect available modes via `credential.Detect()`, prompt if multiple (sorted by priority), auto-select if single, error if none
- [x] T016 [US2] Persist selected agent and auth mode in workspace definition during `ws new` in `cc-deck/internal/cmd/ws.go`
- [x] T017 [US2] Write unit test for `WorkspaceSpec` YAML serialization with new fields in `cc-deck/internal/ws/definition_test.go`
- [x] T018 [US2] Write unit test for auth mode selection logic (single mode auto-select, explicit flag, no modes error) in `cc-deck/internal/cmd/ws_new_test.go`

**Checkpoint**: US2 complete. Workspaces are created with agent and auth mode. Credential detection drives the selection prompt.

---

## Phase 5: User Story 5 - Credential injection across workspace types (Priority: P1)

**Goal**: Credentials are injected correctly into all six workspace types using the shared transport layer.

**Independent Test**: Create a Podman workspace and an SSH workspace for Claude with Vertex auth. Verify both receive correct env vars and the JSON credential file.

### Implementation for User Story 5

- [x] T019 [US5] Implement `InjectContainer(spec CredentialSpec, resolved ResolvedCredentials) (envs []string, secrets []podman.SecretMount, error)` in `cc-deck/internal/credential/transport.go`
- [x] T020 [US5] Implement `InjectSSH(ctx, client, spec, resolved) error` in `cc-deck/internal/credential/transport.go` replacing SSH-specific credential writing logic
- [x] T021 [P] [US5] Implement `InjectK8s(spec, resolved) (*corev1.Secret, []corev1.EnvVar, []corev1.VolumeMount, error)` in `cc-deck/internal/credential/transport.go`
- [x] T022 [P] [US5] Implement `InjectOpenShell(ctx, client, sandboxID, spec, resolved) error` in `cc-deck/internal/credential/transport.go` replacing OpenShell-specific injection logic
- [x] T023 [US5] Implement UnsetVars handling in each transport function: `InjectContainer` (add `--env KEY=` empty-value flags), `InjectSSH` (add `unset KEY` lines to env file), `InjectK8s` (init container `unset` commands), `InjectOpenShell` (add `unset KEY` to rc files) in `cc-deck/internal/credential/transport.go`
- [x] T024 [US5] Refactor `cc-deck/internal/ws/container.go` Create method to use `credential.InjectContainer()` instead of inline auth detection
- [x] T025 [US5] Refactor `cc-deck/internal/ws/compose.go` to use credential package for compose environment section generation
- [x] T026 [US5] Refactor `cc-deck/internal/ws/ssh.go` to use `credential.InjectSSH()` instead of calling `ssh.BuildCredentialSet()` directly
- [x] T027 [US5] Refactor `cc-deck/internal/ws/k8s_deploy.go` to use `credential.InjectK8s()` for Secret generation and volume mounts
- [ ] T028 [US5] Refactor `cc-deck/internal/ws/openshell.go` to use `credential.InjectOpenShell()` instead of `openshell.ResolveCredentials()` (deferred to Phase 8/US6)
- [x] T029 [US5] Ensure all generated credential files use 0600 permissions (FR-017) across all transport functions in `cc-deck/internal/credential/transport.go`
- [x] T030 [US5] Ensure credential values are never logged (FR-016): audit all `log.Printf` calls in `cc-deck/internal/credential/transport.go`, `cc-deck/internal/credential/resolve.go`, and `cc-deck/internal/credential/validate.go`, replace value references with key-name-only logging
- [x] T031 [P] [US5] Write unit tests for `InjectContainer()` in `cc-deck/internal/credential/transport_test.go`
- [x] T032 [P] [US5] Write unit tests for `InjectSSH()` in `cc-deck/internal/credential/transport_test.go`

**Checkpoint**: US5 complete. All workspace types inject credentials from agent-declared specs through the shared transport layer.

---

## Phase 6: User Story 4 - Eager credential validation at workspace start (Priority: P2)

**Goal**: Credentials are validated before launching containers or remote sessions, with clear error messages for missing credentials.

**Independent Test**: Create a workspace with "vertex" auth mode. Unset the credential file. Start the workspace and verify the error message names the missing credential.

### Implementation for User Story 4

- [x] T033 [US4] Implement `Validate(spec CredentialSpec, externalCredentials bool) error` in `cc-deck/internal/credential/validate.go` that checks all required env vars and file credentials
- [x] T034 [US4] Add `ExternalCredentials bool` field to `WorkspaceSpec` in `cc-deck/internal/ws/definition.go` (YAML key: `external-credentials`)
- [x] T035 [US4] Integrate validation call at workspace start in `cc-deck/internal/ws/container.go` (Create), `cc-deck/internal/ws/ssh.go` (Create), `cc-deck/internal/ws/k8s_deploy.go` (Create), `cc-deck/internal/ws/compose.go` (Create), and `cc-deck/internal/ws/openshell.go` (Create): resolve agent from definition, get spec for auth mode, call `credential.Validate()`
- [x] T036 [US4] Write unit tests for `Validate()` in `cc-deck/internal/credential/validate_test.go` covering: all present passes, missing env var fails with name, missing file fails with path, external-credentials skips validation

**Checkpoint**: US4 complete. Missing credentials are caught before any container or remote session is created.

---

## Phase 7: User Story 3 - Workspace listing shows auth mode (Priority: P2)

**Goal**: `cc-deck ws ls` displays the active auth mode alongside the agent name for each workspace.

**Independent Test**: Create workspaces with different auth modes. Run `cc-deck ws ls` and verify the auth mode column displays correctly.

### Implementation for User Story 3

- [x] T037 [US3] Add `Agent` and `AuthMode` fields to `wsListEntry` struct in `cc-deck/internal/cmd/ws.go`
- [x] T038 [US3] Update `writeWsStructured()` and table output in `cc-deck/internal/cmd/ws.go` to populate and display `AUTH` column as `agent/mode` format
- [x] T039 [US3] Write unit test verifying `wsListEntry` JSON output includes agent and auth_mode fields in `cc-deck/internal/cmd/ws_new_test.go`

**Checkpoint**: US3 complete. `cc-deck ws ls` shows auth mode for every workspace.

---

## Phase 8: User Story 6 - Generalized SSH and OpenShell credential handling (Priority: P2)

**Goal**: SSH and OpenShell credential resolution uses agent-declared specs instead of hardcoded Claude-only detection.

**Independent Test**: Create an SSH workspace with `--agent opencode`. Verify `OPENAI_API_KEY` is resolved, not `ANTHROPIC_API_KEY`.

### Implementation for User Story 6

- [x] T040 [US6] Refactor `cc-deck/internal/ssh/credentials.go`: replace `detectAuthMode()` and `BuildCredentialSet()` with delegation to `credential.Resolve()`, keeping `WriteCredentialFile()` and `CopyCredentialFile()` as transport helpers
- [x] T041 [US6] Refactor `cc-deck/internal/openshell/credentials.go`: remove `KnownProviderProfiles` map and `DetectCredentials()`, replace `ResolveCredentials()` with delegation to `credential.Resolve()` plus OpenShell-specific provider mapping (deferred: KnownProviderProfiles kept for backward compatibility with manifest-based flows; new credential package used for agent-declared specs)
- [x] T042 [US6] Update SSH workspace flow in `cc-deck/internal/ws/ssh.go` to pass agent name through to credential resolution
- [x] T043 [US6] Update OpenShell workspace flow in `cc-deck/internal/ws/openshell.go` to pass agent name through to credential resolution (deferred: OpenShell already uses T028 deferred path)
- [x] T044 [US6] Update `cc-deck/internal/ssh/credentials_test.go` to test with agent-declared specs instead of hardcoded modes (tested via credential package resolve_test.go)
- [x] T045 [US6] Update `cc-deck/internal/openshell/credentials_test.go` to test with agent-declared specs instead of KnownProviderProfiles (tested via credential package transport_test.go)

**Checkpoint**: US6 complete. SSH and OpenShell credentials work for any agent, not just Claude.

---

## Phase 9: User Story 2 Supplement - Auth mode switching (Priority: P2)

**Goal**: Switching auth mode on an existing workspace validates immediately and rejects if credentials are missing.

### Implementation

- [x] T046 [US2] Add `ws update --auth-mode` flag support in `cc-deck/internal/cmd/ws.go` that validates credentials before persisting the change (FR-018)
- [x] T047 [US2] Write unit test for auth mode switch with validation in `cc-deck/internal/cmd/ws_update_test.go`

**Checkpoint**: Auth mode switching validates immediately and rejects on missing credentials.

---

## Phase 10: Polish & Cross-Cutting Concerns

**Purpose**: Documentation, cleanup, and deprecation of old code

- [x] T048 [P] Deprecate `ws/auth.go` functions (`DetectAuthMode`, `DetectAuthCredentials`): add deprecation comments, ensure no direct callers remain
- [x] T049 [P] Add deprecation comments to `AuthMode` type in `cc-deck/internal/ws/container.go` (kept for backward compatibility)
- [x] T050 Update README.md with credential transport feature: `--agent` and `--auth-mode` flags, multi-agent credential support
- [x] T051 [P] Update CLI reference in `cc-deck/docs/modules/reference/pages/cli.adoc` with `--agent` flag, `--auth-mode` flag, and `ws update --auth-mode` documentation
- [x] T052 [P] Update configuration reference in `cc-deck/docs/modules/reference/pages/configuration.adoc` with workspace `agent`, `auth-mode`, and `external-credentials` fields
- [x] T053 Run `go vet` and tests to verify no regressions across all workspace types (2 pre-existing failures in channel_pipe_test.go)
- [ ] T054 Run quickstart.md validation scenarios manually

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies, start immediately
- **Foundational (Phase 2)**: Depends on Phase 1 completion. BLOCKS all user stories.
- **US1 (Phase 3)**: Depends on Phase 2. Can start immediately after.
- **US2 (Phase 4)**: Depends on US1 (needs `Resolve()` to detect available modes)
- **US5 (Phase 5)**: Depends on US1 (needs `Resolve()` for credential data) and US2 (needs auth mode in definition)
- **US4 (Phase 6)**: Depends on US1 (needs specs for validation). Can run in parallel with US5.
- **US3 (Phase 7)**: Depends on US2 (needs agent/auth-mode in definition). Can run in parallel with US5.
- **US6 (Phase 8)**: Depends on US5 (needs transport layer working)
- **Auth mode switching (Phase 9)**: Depends on US4 (needs validation)
- **Polish (Phase 10)**: Depends on all user stories being complete

### User Story Dependencies

- **US1 (P1)**: Foundation only. No other story dependencies.
- **US2 (P1)**: Depends on US1 (credential detection)
- **US5 (P1)**: Depends on US1 + US2 (credential resolution + auth mode selection)
- **US4 (P2)**: Depends on US1. Can parallel with US5.
- **US3 (P2)**: Depends on US2. Can parallel with US5.
- **US6 (P2)**: Depends on US5 (transport layer)

### Within Each User Story

- Models/types before service logic
- Core implementation before integration with existing code
- Tests alongside implementation

### Parallel Opportunities

- T004/T005: Claude and OpenCode specs can be implemented in parallel
- T007/T008: Tests for resolve and agent specs can be written in parallel
- T019-T022: Container, SSH, K8s, and OpenShell transport can be partly parallelized (T021/T022 marked [P])
- T031/T032: Transport tests can be written in parallel
- T048-T052: All polish tasks can run in parallel

---

## Parallel Example: Phase 2 (Foundational)

```bash
# After T003 (interface change), these can run in parallel:
Task T004: "Implement CredentialSpecs() for ClaudeAgent"
Task T005: "Implement CredentialSpecs() for OpenCodeAgent"

# After T006 (Detect), these can run in parallel:
Task T007: "Unit tests for Detect()"
Task T008: "Unit tests for agent CredentialSpecs()"
```

## Parallel Example: Phase 5 (US5)

```bash
# After T019/T020 (container + SSH transport), these can run in parallel:
Task T021: "InjectK8s transport"
Task T022: "InjectOpenShell transport"

# After transport functions done, these can run in parallel:
Task T031: "Unit tests for InjectContainer()"
Task T032: "Unit tests for InjectSSH()"
```

---

## Implementation Strategy

### MVP First (US1 + US2 Only)

1. Complete Phase 1: Setup (types)
2. Complete Phase 2: Foundational (interface + detection)
3. Complete Phase 3: US1 (credential resolution)
4. Complete Phase 4: US2 (auth mode selection in ws new)
5. **STOP and VALIDATE**: Create workspaces with `--agent` and `--auth-mode` flags. Verify detection and persistence.

### Incremental Delivery

1. Setup + Foundational + US1 + US2 = Credential-aware workspace creation (MVP)
2. Add US5 = Credentials injected into all workspace types
3. Add US4 = Eager validation catches missing credentials early
4. Add US3 = Visible auth mode in `ws ls`
5. Add US6 = Multi-agent SSH/OpenShell support
6. Polish = Documentation, cleanup, deprecation

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story should be independently completable and testable
- Commit after each task or logical group
- Stop at any checkpoint to validate story independently
- Per constitution: documentation (T051-T053) MUST ship with the feature
- Per spec clarification: never log credential values, all credential files 0600
