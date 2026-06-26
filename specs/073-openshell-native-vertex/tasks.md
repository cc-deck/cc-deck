# Tasks: OpenShell Native Vertex Provider

**Input**: Design documents from `specs/073-openshell-native-vertex/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

---

## Phase 1: Setup

**Purpose**: No new project initialization needed. This is a modification of existing code.

*(No setup tasks - all changes are to existing files)*

---

## Phase 2: Foundational (Provider Config Enhancement)

**Purpose**: Add `Config` field to `ProviderConfig` and update `EnsureProvider` to support `--config` and `--from-gcloud-adc` flags. MUST complete before user story tasks.

- [ ] T001 Add `Config map[string]string` field to `ProviderConfig` struct in `cc-deck/internal/openshell/credentials.go`
- [ ] T002 Add `FromGcloudADC bool` field to `ProviderConfig` struct in `cc-deck/internal/openshell/credentials.go` to distinguish `--from-gcloud-adc` from `--from-existing`
- [ ] T003 Update `EnsureProvider` method in `cc-deck/internal/openshell/client.go` to pass `--config key=value` pairs and support `--from-gcloud-adc` flag when `FromGcloudADC` is true

**Checkpoint**: Foundation ready - `EnsureProvider` can create `google-cloud` providers with config options

---

## Phase 3: User Story 1 - OpenShell Vertex via Native Provider (Priority: P1) 🎯 MVP

**Goal**: OpenShell workspaces with Vertex AI use OpenShell's native `google-cloud` provider instead of cc-deck's custom file upload workaround.

**Independent Test**: Run `make test` and verify credential resolution produces a `google-cloud` provider config with correct config options instead of a `SkipProvider` config.

### Implementation for User Story 1

- [ ] T004 [US1] Update `claude-vertex` entry in `KnownProviderProfiles` in `cc-deck/internal/openshell/credentials.go`: change `Type` from `"claude"` to `"google-cloud"`, remove `FileVar` and `Endpoints` fields
- [ ] T005 [US1] Remove standalone `vertex` entry from `KnownProviderProfiles` map in `cc-deck/internal/openshell/credentials.go`
- [ ] T006 [US1] Remove `"vertex"` from detection order in `DetectCredentials` function in `cc-deck/internal/openshell/credentials.go` (line 256)
- [ ] T007 [US1] Rewrite the `if entry.Type == "claude-vertex"` special case in `ResolveCredentials` (lines 182-215) in `cc-deck/internal/openshell/credentials.go`: instead of `SkipProvider=true` with file credential resolution, produce a `ProviderConfig` with `Type: "google-cloud"`, `FromGcloudADC: true`, `Config: {"project_id": <value>, "region": <value>}`, and `EnvVarsToInject` for Claude Code env vars
- [ ] T008 [US1] Update post-start credential injection in `cc-deck/internal/ws/openshell.go` (lines 321-333): skip file upload for `google-cloud` provider type (no `UploadFileCredential` call), but still call `InjectEnvVars` for `CLAUDE_CODE_USE_VERTEX=1` and companion vars
- [ ] T009 [US1] Update `cc-deck/internal/openshell/credentials_test.go`: remove `"vertex"` from expected types in `KnownProviderProfiles` test, add test case for `claude-vertex` producing a `google-cloud` provider config with `FromGcloudADC=true`
- [ ] T010 [US1] Run `make test` to verify all existing tests pass with the credential path changes

**Checkpoint**: OpenShell workspaces with Vertex use the native `google-cloud` provider. Non-OpenShell paths unaffected.

---

## Phase 4: User Story 2 - Non-OpenShell Unchanged (Priority: P1)

**Goal**: Verify non-OpenShell workspace types continue working with existing Vertex credential handling.

**Independent Test**: Run `make test` and verify that agent-level credential specs, SSH credential handling, and container credential handling are unchanged.

### Implementation for User Story 2

- [ ] T011 [US2] Verify `cc-deck/internal/agent/claude.go` vertex credential spec (line 175) is unchanged - no modifications needed, just confirm the agent-level spec still declares `GOOGLE_APPLICATION_CREDENTIALS` for non-OpenShell paths
- [ ] T012 [US2] Verify `cc-deck/internal/network/builtin.go` `vertexai` domain group is preserved unchanged for non-OpenShell workspace types
- [ ] T013 [US2] Run `make test` to confirm no regressions in credential/validate_test.go and credential/resolve_test.go (which test agent-level vertex specs used by non-OpenShell paths)

**Checkpoint**: Non-OpenShell workspace types confirmed working identically.

---

## Phase 5: User Story 3 - Capture Detection (Priority: P2)

**Goal**: Credential detection during capture produces correct entries for OpenShell workspaces.

**Independent Test**: Verify `DetectCredentials` returns `claude-vertex` type (not `vertex`) when Vertex env vars are set.

### Implementation for User Story 3

- [ ] T014 [US3] Verify `DetectCredentials` in `cc-deck/internal/openshell/credentials.go` returns `claude-vertex` (not standalone `vertex`) when both `CLAUDE_CODE_USE_VERTEX` and `ANTHROPIC_VERTEX_PROJECT_ID` are set - the removal of the `vertex` entry in T005 handles this automatically
- [ ] T015 [US3] Add test case in `cc-deck/internal/openshell/credentials_test.go` verifying that detection with Vertex env vars returns `claude-vertex` type only (not `vertex`)

**Checkpoint**: Capture phase correctly detects Vertex credentials for OpenShell.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Documentation and cleanup

- [ ] T016 [P] Update README.md with changed Vertex AI authentication behavior for OpenShell workspaces
- [ ] T017 Run `make verify` (test + lint) to confirm everything passes

---

## Dependencies & Execution Order

### Phase Dependencies

- **Foundational (Phase 2)**: No dependencies - can start immediately
- **User Story 1 (Phase 3)**: Depends on Phase 2 (T001-T003) completion
- **User Story 2 (Phase 4)**: Can start after Phase 3 (verification tasks only)
- **User Story 3 (Phase 5)**: Can start after Phase 3 (T005 removal enables this)
- **Polish (Phase 6)**: Depends on all user stories being complete

### Within Each Phase

- T001-T003: T001 and T002 can run in parallel, T003 depends on both
- T004-T007: Sequential within `credentials.go` (same file)
- T008: Depends on T007 (needs the new ProviderConfig shape)
- T009-T010: After all code changes

### Parallel Opportunities

- T001 and T002 modify the same struct but different fields (parallel if editing tool supports it)
- T011 and T012 are verification-only tasks (can run in parallel)
- T016 and T017 can run in parallel

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 2: Add Config/FromGcloudADC to ProviderConfig
2. Complete Phase 3: Update claude-vertex profile and credential resolution
3. **STOP and VALIDATE**: `make test` passes, credential resolution produces google-cloud provider
4. Verify OpenShell workspace creation with Vertex AI works end-to-end

### Incremental Delivery

1. Foundation (T001-T003) → Provider infrastructure ready
2. User Story 1 (T004-T010) → Core functional change verified
3. User Story 2 (T011-T013) → Non-regression confirmed
4. User Story 3 (T014-T015) → Capture detection verified
5. Polish (T016-T017) → Documentation and final verification

---

## Notes

- Most changes are in `cc-deck/internal/openshell/credentials.go` (single file, sequential execution)
- The `UploadFileCredential` and `InjectEnvVars` functions in `credentials.go` are preserved (used by other credential types and for env var injection)
- The `vertexai` domain group in `network/builtin.go` is preserved (used by non-OpenShell workspace types)
- No new CLI commands or flags are added
