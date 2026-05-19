# Tasks: OpenShell Credential Injection

**Input**: Design documents from `specs/058-openshell-credential-injection/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

---

## Phase 1: Setup

**Purpose**: Verify baseline and project structure

- [ ] T001 Verify all existing tests pass with `make test`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Manifest schema extension and OpenShell client provider methods used by all user stories

**CRITICAL**: No user story work can begin until this phase is complete

- [ ] T002 [P] Add `CredentialEntry` struct and `Credentials` field to `Manifest` in `cc-deck/internal/build/manifest.go`. Fields: `Type` (string, yaml:"type"), `EnvVars` ([]string, yaml:"env_vars,omitempty"), `File` (string, yaml:"file,omitempty"), `Endpoints` ([]PolicyEndpoint, yaml:"endpoints,omitempty"). Add `Credentials []CredentialEntry` to the `Manifest` struct.
- [ ] T003 [P] Add `KnownProviderProfiles` map in new file `cc-deck/internal/openshell/credentials.go`. Define profiles for claude, anthropic, github, gitlab, openai, nvidia, vertex, generic. Each profile has Type, DetectVars, RequiredVars, FileVar, and Endpoints fields per data-model.md. Include `ResolveDefaultEnvVars(credType string) []string` function that returns default env vars for known types.
- [ ] T004 [P] Add provider management methods to `openshell.Client` interface in `cc-deck/internal/openshell/iface.go`: `CreateProvider`, `UpdateProvider`, `DeleteProvider`, `EnsureProvider` per contracts/provider-client.md.
- [ ] T005 Implement provider management methods on `cliClient` in `cc-deck/internal/openshell/client.go`. Wrap `openshell provider create/update/delete` CLI commands using existing `execCLI` pattern. `EnsureProvider` tries create, falls back to update on "already exists" error.
- [ ] T006 [P] Add unit tests for `CredentialEntry` YAML parsing in `cc-deck/internal/build/manifest_test.go`. Test: marshal/unmarshal round-trip, default env_vars resolution, generic type with endpoints.
- [ ] T007 [P] Add unit tests for `KnownProviderProfiles` and `ResolveDefaultEnvVars` in `cc-deck/internal/openshell/credentials_test.go`.

**Checkpoint**: Foundation ready. Manifest parses credentials, client can manage providers, profiles are defined.

---

## Phase 3: User Story 1 - Create OpenShell Workspace with API Key Credentials (Priority: P1)

**Goal**: `cc-deck ws new --type openshell` reads credentials from manifest, creates OpenShell providers, and attaches them to the sandbox.

**Independent Test**: Create workspace with `build.yaml` declaring `claude` credential. Verify provider is created on the gateway and Claude Code can authenticate inside the sandbox.

### Implementation for User Story 1

- [ ] T008 [US1] Add `resolveCredentials` function in `cc-deck/internal/openshell/credentials.go`. Reads `[]CredentialEntry` from manifest, resolves env vars from host environment, returns list of provider configs to create. Skips entries with missing required env vars (emits warning via log). Uses `ResolveDefaultEnvVars` for entries without explicit `env_vars`.
- [ ] T009 [US1] Add `loadManifestCredentials` function in `cc-deck/internal/ws/openshell.go`. Locates `build.yaml` from workspace definition's `ProjectDir` (walk up to find `.cc-deck/setup/build.yaml`), loads manifest, returns `Credentials` slice. Returns nil if no manifest or no credentials section.
- [ ] T010 [US1] Modify `OpenShellWorkspace.Create` in `cc-deck/internal/ws/openshell.go`. After `pollUntilRunning`, before repo cloning: call `loadManifestCredentials`, then `resolveCredentials`, then `EnsureProvider` for each resolved provider. Collect provider names and pass as comma-separated `--provider` list to `CreateSandbox`. Update `CreateSandbox` signature to accept multiple providers.
- [ ] T011 [US1] Update `CreateSandbox` in `cc-deck/internal/openshell/client.go` to accept a `[]string` of provider names instead of a single string. Build `--provider <name>` flags for each. Update `iface.go` signature accordingly.
- [ ] T012 [US1] Update `SandboxConfig.Provider` field in `cc-deck/internal/ws/openshell.go` from `string` to `[]string`. Update `resolveSandboxConfig` to handle the list.
- [ ] T013 [P] [US1] Add unit tests for `resolveCredentials` in `cc-deck/internal/openshell/credentials_test.go`. Test: API key present, API key missing (warning), multiple credential types, unknown type falls back to generic.
- [ ] T014 [US1] Run `make test` to verify all tests pass.

**Checkpoint**: Workspace creation with API key credentials works end-to-end.

---

## Phase 4: User Story 2 - Capture Credential Requirements (Priority: P2)

**Goal**: `/cc-deck.capture` detects credentials from host env and writes them to `build.yaml`.

**Independent Test**: Run capture with `ANTHROPIC_API_KEY` and `GITHUB_TOKEN` set. Verify `credentials` section appears in `build.yaml`.

### Implementation for User Story 2

- [ ] T015 [US2] Add `DetectCredentials` function in `cc-deck/internal/openshell/credentials.go`. Scans host environment for all `KnownProviderProfiles` detection vars. Returns list of detected `CredentialEntry` values with type and env_vars populated.
- [ ] T016 [US2] Add credential detection step to capture command in `cc-deck/internal/build/commands/cc-deck.capture.md`. Insert as new Step 10 (renumber Target Configuration to Step 11). Step presents detected credentials using the standard AskUserQuestion accept/exclude pattern. Writes confirmed entries to `credentials` section of manifest.
- [ ] T017 [US2] Update `build.yaml.tmpl` in `cc-deck/internal/build/templates/build.yaml.tmpl` to include a commented `credentials` section with examples for claude, github, and vertex types.
- [ ] T018 [P] [US2] Add unit tests for `DetectCredentials` in `cc-deck/internal/openshell/credentials_test.go`. Test: detects ANTHROPIC_API_KEY as claude, detects GITHUB_TOKEN as github, returns empty for no matches, deduplicates when both GH_TOKEN and GITHUB_TOKEN are set.

**Checkpoint**: Capture wizard discovers and records credential requirements.

---

## Phase 5: User Story 3 - File-Based Credential Upload for Vertex (Priority: P3)

**Goal**: Workspace creation uploads Vertex service account JSON and adds GCP endpoints to network policy.

**Independent Test**: Set `GOOGLE_APPLICATION_CREDENTIALS` to a JSON file. Create workspace with vertex credential in manifest. Verify file is uploaded and env var set inside sandbox.

### Implementation for User Story 3

- [ ] T019 [US3] Add `uploadFileCredential` function in `cc-deck/internal/openshell/credentials.go`. Takes client, sandbox ID, local file path, and remote destination path. Calls `client.Upload()` then `client.ExecSandbox()` to append `export` line to `.bashrc` and `.zshrc`.
- [ ] T020 [US3] Modify `OpenShellWorkspace.Create` in `cc-deck/internal/ws/openshell.go`. After provider creation loop, check for file-based credentials. For each, validate the local file exists, then call `uploadFileCredential` with remote path `/sandbox/.config/gcloud/credentials.json`.
- [ ] T021 [US3] Add Vertex GCP endpoints to `GeneratePolicy` in `cc-deck/internal/build/policy.go`. When manifest `Credentials` contains a `vertex` entry, add network policy entries for `oauth2.googleapis.com:443` and `{region}-aiplatform.googleapis.com:443`. Region from `CLOUD_ML_REGION` env or default `us-east1`.
- [ ] T022 [US3] Add `generic` type endpoint injection to `GeneratePolicy` in `cc-deck/internal/build/policy.go`. When a `generic` credential entry has `Endpoints`, add them to the policy's `NetworkPolicies`.
- [ ] T023 [P] [US3] Add unit tests for Vertex policy generation in `cc-deck/internal/build/policy_test.go`. Test: vertex credential adds GCP endpoints, generic credential adds custom endpoints, no credentials produces default policy only.
- [ ] T024 [US3] Run `make test` to verify all tests pass.

**Checkpoint**: Vertex file upload and policy generation work.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Documentation and cleanup

- [ ] T025 [P] Update CLI reference in `docs/modules/reference/pages/configuration.adoc` to document the `credentials` section of `build.yaml` with field descriptions and examples.
- [ ] T026 [P] Update `README.md` with a section on credential management for OpenShell workspaces.
- [ ] T027 Run `make test` and `make lint` for final validation.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies
- **Foundational (Phase 2)**: Depends on Setup. BLOCKS all user stories.
- **User Story 1 (Phase 3)**: Depends on Foundational (T002-T007)
- **User Story 2 (Phase 4)**: Depends on Foundational (T003 for KnownProviderProfiles)
- **User Story 3 (Phase 5)**: Depends on US1 (T010 for Create flow changes)
- **Polish (Phase 6)**: Depends on all user stories complete

### User Story Dependencies

- **US1 (P1)**: Can start after Foundational. No dependencies on other stories.
- **US2 (P2)**: Can start after Foundational (only needs T003). Independent of US1.
- **US3 (P3)**: Depends on US1 (T010 modifies Create flow that T020 extends). Depends on Foundational for policy types.

### Within Each User Story

- Data types before logic (manifest types before credential resolution)
- Client methods before workspace integration
- Core implementation before tests

### Parallel Opportunities

- T002, T003, T004 can all run in parallel (different files)
- T006, T007 can run in parallel (different test files)
- US1 and US2 can run in parallel after Foundational (US2 only needs T003)
- T013, T018, T023 (tests) can run in parallel with non-test tasks in the same phase

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational
3. Complete Phase 3: User Story 1 (API key credentials)
4. **STOP and VALIDATE**: Create an OpenShell workspace with claude credential, verify provider is created
5. Deploy/demo if ready

### Incremental Delivery

1. Setup + Foundational -> Foundation ready
2. Add US1 (API key providers) -> Test independently -> MVP!
3. Add US2 (capture detection) -> Test independently -> Better DX
4. Add US3 (Vertex file upload + policy) -> Test independently -> Full feature
5. Polish -> Documentation complete

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- US1 and US2 can be implemented in parallel since US2 only depends on T003 (KnownProviderProfiles)
- US3 depends on US1's Create flow modifications
- Commit after each task or logical group
