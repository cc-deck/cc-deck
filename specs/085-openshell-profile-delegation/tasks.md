# Tasks: OpenShell Profile Delegation

**Input**: Design documents from `specs/085-openshell-profile-delegation/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: Setup

**Purpose**: Profile mapping infrastructure that all user stories depend on

- [ ] T001 Create `ProfileMapping` type and static mapping table in `cc-deck/internal/openshell/profiles.go` with `ResolveProfiles(agents, tools, credentials []string) []string` function returning deduplicated, sorted profile IDs
- [ ] T002 [P] Add unit tests for `ResolveProfiles()` covering: single agent, multiple agents, tool mapping, deduplication, always-included profiles (github, gitlab), empty inputs defaulting to claude in `cc-deck/internal/openshell/profiles_test.go`
- [ ] T003 Create `ProfileManifest` struct and `BuildProfileManifest(manifest *Manifest) (*ProfileManifest, error)` in `cc-deck/internal/build/profiles.go` with YAML serialization to `profiles.yaml` format
- [ ] T004 [P] Add unit tests for `BuildProfileManifest()` covering: manifest with agents+tools, empty manifest defaulting to claude, deterministic sorting in `cc-deck/internal/build/profiles_test.go`
- [ ] T005 Add `SanitizeWorkspaceName(name string) string` helper to `cc-deck/internal/openshell/profiles.go` for deterministic ephemeral profile naming (lowercase alphanumeric + hyphens, truncated to 50 chars)

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Fix hardcoded agent name and add profile verification capability

**Warning**: No user story work can begin until this phase is complete

- [ ] T006 Replace hardcoded `resolveAgentName()` in `cc-deck/internal/ws/openshell.go` (line 244) to read agents from the profile manifest or manifest config instead of always returning `"claude"`
- [ ] T007 Update credential resolution in `cc-deck/internal/ws/openshell.go` (line 342) to iterate over all agents from the profile manifest instead of hardcoding `agentName := "claude"`
- [ ] T008 Add `VerifyProfiles(ctx context.Context, client v1.ClientInterface, profileIDs []string) (verified []string, missing []string, err error)` to `cc-deck/internal/openshell/profiles.go` that calls `ProfileInterface.Get()` for each profile ID
- [ ] T009 [P] Add unit tests for `VerifyProfiles()` using `fake.NewClient()` in `cc-deck/internal/openshell/profiles_test.go`
  - **Interfaces**: Depends on T008. Test with mock profiles: all found, some missing, all missing. Verify warnings emitted for missing profiles.
- [ ] T010 Update `mapToOpenShellProvider()` in `cc-deck/internal/ws/openshell.go` (line 288) to set `Provider.Type` to the profile ID from the mapping table, not just hardcoded `"claude"` and `"google-cloud"`

**Checkpoint**: Multi-agent support and profile verification ready

---

## Phase 3: User Story 1 - Profile-Based Sandbox Creation (Priority: P1)

**Goal**: Workspace creation uses profile references instead of policy generation

**Independent Test**: Create a workspace with `agents: [claude]` and `tools: [python]`. Verify no `SandboxPolicy` is constructed and providers reference the correct profiles.

- [ ] T011 [US1] Add `extractProfileManifest(image string) (*ProfileManifest, error)` to `cc-deck/internal/ws/openshell.go` that extracts `/etc/openshell/profiles.yaml` from OCI image (reuse `oci.ExtractFileFromImage` pattern)
- [ ] T012 [US1] Add `createProfileProviders(ctx context.Context, client v1.ClientInterface, profileIDs []string, credentials *credential.ResolvedCredentials) ([]string, error)` to `cc-deck/internal/ws/openshell.go` that creates providers referencing each profile ID via `Providers().Ensure()`, merging credentials for credential-carrying profiles
- [ ] T013 [US1] Modify `Create()` in `cc-deck/internal/ws/openshell.go` to use the profile-based path: extract profile manifest, verify profiles, create providers, set `SandboxSpec.Policy` to nil
  - **Interfaces**: Consumes T011 (`extractProfileManifest`), T008 (`VerifyProfiles`), T012 (`createProfileProviders`). The `SandboxSpec.Providers` list is populated from created provider names. `SandboxSpec.Policy` is set to nil.
- [ ] T014 [P] [US1] Add unit tests for `extractProfileManifest()` in `cc-deck/internal/ws/openshell_test.go` verifying YAML parsing and error handling
- [ ] T015 [P] [US1] Add unit tests for `createProfileProviders()` in `cc-deck/internal/ws/openshell_test.go` using `fake.NewClient()`, verifying provider creation with correct `Type` and credential merging
- [ ] T016 [US1] Add integration test for profile-based `Create()` in `cc-deck/internal/ws/openshell_test.go` verifying: no `SandboxPolicy` passed, correct provider names in `SandboxSpec.Providers`, backward compatibility with empty agents field

**Checkpoint**: Profile-based sandbox creation works for standard profiles

---

## Phase 4: User Story 2 - Auto-Detection Maps to Profiles (Priority: P1)

**Goal**: Build phase generates profile manifest from detected tools

**Independent Test**: Build an image with Python + Node.js without listing them in manifest. Verify the profile manifest contains "python" and "nodejs".

- [ ] T017 [US2] Add OpenShell profile manifest generation to the build pipeline in `cc-deck/internal/build/policy.go`: when `manifest.Targets.OpenShell` is configured, call `BuildProfileManifest()` instead of `AssemblePolicy()` and write `/etc/openshell/profiles.yaml` to the build output
  - **Interfaces**: Consumes `BuildProfileManifest()` from T003. Reads `manifest.EffectiveAgents()`, matched tool components, and the profile mapping table. Writes YAML to the build context.
- [ ] T018 [P] [US2] Add unit tests verifying profile manifest generation includes auto-detected tools (matched via component `Match.Tools`) in `cc-deck/internal/build/policy_test.go`
- [ ] T019 [US2] Add unit test verifying union merge: manifest `tools: [python]` + detected Node.js results in both profiles present, with no duplicates in `cc-deck/internal/build/policy_test.go`
- [ ] T020 [US2] Add unit test verifying backward compatibility: non-OpenShell targets still produce full policy via `AssemblePolicy()` in `cc-deck/internal/build/policy_test.go`

**Checkpoint**: Build phase produces profile manifests for OpenShell targets

---

## Phase 5: User Story 3 - MCP Endpoint Profiles (Priority: P2)

**Goal**: Custom MCP endpoints handled via ephemeral profiles

**Independent Test**: Configure an MCP endpoint in manifest. Verify an ephemeral profile is imported and a provider references it.

- [ ] T021 [US3] Add `importMCPProfile(ctx context.Context, client v1.ClientInterface, workspaceName string, mcpEntries []MCPEntry, agentBinaries []string) (string, error)` to `cc-deck/internal/ws/openshell.go` that imports a single profile via `ProfileInterface.Import()` with all MCP endpoints and agent binaries, using name `cc-deck-<sanitized-name>-mcp`
- [ ] T022 [US3] Integrate `importMCPProfile()` into `Create()` in `cc-deck/internal/ws/openshell.go`: when manifest has MCP entries, import the profile and add its provider to `SandboxSpec.Providers`
- [ ] T023 [P] [US3] Add unit tests for `importMCPProfile()` in `cc-deck/internal/ws/openshell_test.go` using `fake.NewClient()`: single endpoint, multiple endpoints, import failure (warn and continue)

**Checkpoint**: MCP endpoints work through profile delegation

---

## Phase 6: User Story 4 - User-Defined Domain Overrides as Profiles (Priority: P2)

**Goal**: Custom domain overrides handled via ephemeral profiles

**Independent Test**: Add `allowed_domains: ["custom.example.com"]` to manifest. Verify an ephemeral profile is imported with that domain.

- [ ] T024 [US4] Add `importCustomDomainsProfile(ctx context.Context, client v1.ClientInterface, workspaceName string, domains []string) (string, error)` to `cc-deck/internal/ws/openshell.go` that imports a profile with user-defined domain endpoints, using name `cc-deck-<sanitized-name>-custom`
- [ ] T025 [US4] Integrate `importCustomDomainsProfile()` into `Create()` in `cc-deck/internal/ws/openshell.go`: resolve `AllowedDomains` + filtered `AllowedDomainsPerAgent` into domain list, import profile, add provider
- [ ] T026 [P] [US4] Add unit tests for `importCustomDomainsProfile()` in `cc-deck/internal/ws/openshell_test.go`: basic domains, per-agent filtering (included vs excluded agent), empty domains (skip import)

**Checkpoint**: User-defined domain overrides work through profile delegation

---

## Phase 7: User Story 5 - Policy Code Removal (Priority: P3)

**Goal**: Remove dead policy code from the OpenShell workspace path

**Independent Test**: Search the OpenShell workspace creation code for any remaining `SandboxPolicy` construction. None should exist.

- [ ] T027 [US5] Remove `loadSDKPolicy()` and related policy-loading code from `cc-deck/internal/ws/openshell.go` (the function that reads policy YAML and converts to SDK types)
- [ ] T028 [US5] Remove `SandboxConfig.Policy` field and policy extraction from `resolveSandboxConfig()` in `cc-deck/internal/ws/openshell.go`
- [ ] T029 [US5] Update or remove tests in `cc-deck/internal/ws/openshell_test.go` that assert on `SandboxPolicy` construction for OpenShell targets
- [ ] T030 [US5] Verify that compose environment tests in `cc-deck/internal/build/policy_test.go` still pass (non-OpenShell targets unaffected)

**Checkpoint**: Zero `SandboxPolicy` construction in OpenShell code path

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: Documentation, validation, and cleanup

- [ ] T031 [P] Update `./README.md` with profile delegation documentation: manifest `agents` and `tools` fields, profile manifest concept, ephemeral profiles for MCP/custom domains
- [ ] T032 [P] Update `docs/modules/reference/pages/configuration.adoc` with profile-related manifest fields and `profiles.yaml` format
- [ ] T033 Run `make verify` to confirm all tests pass and linting is clean
- [ ] T034 Verify backward compatibility: build for both OpenShell and compose targets, confirm compose output is unchanged

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies, can start immediately
- **Foundational (Phase 2)**: Depends on T001 (profile mapping table)
- **User Story 1 (Phase 3)**: Depends on Phase 2 completion
- **User Story 2 (Phase 4)**: Depends on T003 (BuildProfileManifest)
- **User Story 3 (Phase 5)**: Depends on Phase 3 (needs profile-based Create)
- **User Story 4 (Phase 6)**: Depends on Phase 3 (needs profile-based Create)
- **User Story 5 (Phase 7)**: Depends on Phases 3-6 (remove code only after new path is complete)
- **Polish (Phase 8)**: Depends on all user stories being complete

### User Story Dependencies

- **US1 (P1)**: Depends on Foundational, no dependencies on other stories
- **US2 (P1)**: Depends on Setup (T003), can run parallel with US1 after Phase 2
- **US3 (P2)**: Depends on US1 (needs profile-based Create flow)
- **US4 (P2)**: Depends on US1 (needs profile-based Create flow)
- **US5 (P3)**: Depends on US1, US3, US4 (remove code only after replacements work)

### Parallel Opportunities

- T002 and T004 can run in parallel (different test files)
- T009 can run in parallel with other Phase 2 tasks (different test file)
- T014 and T015 can run in parallel (different test aspects)
- T018 can run in parallel with T019 (different test scenarios)
- T023 and T026 can run in parallel (different test files)
- T031 and T032 can run in parallel (different doc files)
- US1 and US2 can run in parallel after Phase 2

---

## Implementation Strategy

### MVP First (User Stories 1 + 2)

1. Complete Phase 1: Setup (profile mapping + manifest types)
2. Complete Phase 2: Foundational (fix hardcoded agent, add profile verification)
3. Complete Phase 3: US1 - Profile-Based Sandbox Creation
4. Complete Phase 4: US2 - Auto-Detection Maps to Profiles
5. **STOP and VALIDATE**: Test with Claude-only and Claude+OpenCode manifests
6. Verify backward compatibility for compose targets

### Incremental Delivery

1. Setup + Foundational -> Core infrastructure ready
2. Add US1 -> Profile-based sandbox creation works (MVP)
3. Add US2 -> Build phase generates profile manifests
4. Add US3 -> MCP endpoints via ephemeral profiles
5. Add US4 -> User domain overrides via ephemeral profiles
6. Add US5 -> Dead policy code removed
7. Polish -> Documentation and final validation

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- The OpenShell gateway MUST have profiles for all 13 categories before testing US1
- Compose/container environments are NOT affected by any of these changes
- The `fake.NewClient()` from the SDK is used for all unit tests (no running gateway needed)
