# Tasks: Deterministic Policy Generation

**Input**: Design documents from `/specs/059-deterministic-policy-generation/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/component-file-format.md

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3, US4)
- User stories map to spec.md stories:
  - US1: Policy generated deterministically from manifest (P1)
  - US2: Component files define policy fragments (P1)
  - US3: Remote catalog updates without binary release (P2)
  - US4: User-local policy overrides (P3)

---

## Phase 1: Setup

**Purpose**: Define component types and create embedded component YAML files

- [X] T001 Define PolicyComponent and MatchCondition structs in cc-deck/internal/build/component.go
- [X] T002 [P] Create claude-code.yaml component from DefaultPolicy() claude_code section in cc-deck/internal/build/policies/claude-code.yaml
- [X] T003 [P] Create git-hosting.yaml component from DefaultPolicy() github section in cc-deck/internal/build/policies/git-hosting.yaml
- [X] T004 [P] Create rust.yaml component from toolEndpoints["rust"]/["cargo"] in cc-deck/internal/build/policies/rust.yaml
- [X] T005 [P] Create go.yaml component from toolEndpoints["go"] in cc-deck/internal/build/policies/go.yaml
- [X] T006 [P] Create node.yaml component from toolEndpoints["node"]/["npm"] in cc-deck/internal/build/policies/node.yaml
- [X] T007 [P] Create python.yaml component from toolEndpoints["python"]/["pip"]/["uv"] in cc-deck/internal/build/policies/python.yaml
- [X] T008 [P] Create vertex-ai.yaml component from vertexEndpoints() in cc-deck/internal/build/policies/vertex-ai.yaml
- [X] T009 Add go:embed directive for policies/*.yaml in cc-deck/internal/build/embed.go

---

## Phase 2: Foundational (Component Loading and Matching)

**Purpose**: Core component loading, validation, matching, and precedence resolution. MUST complete before any user story work.

**CRITICAL**: No user story work can begin until this phase is complete

- [X] T010 Implement LoadComponentsFromFS() to parse YAML component files from an fs.FS in cc-deck/internal/build/component.go
- [X] T011 Implement ValidateComponent() with required field checks per contracts/component-file-format.md in cc-deck/internal/build/component.go
- [X] T012 Implement MatchComponent() to evaluate match conditions (always, tools, credentials, features) against a Manifest in cc-deck/internal/build/component.go
- [X] T013 Implement ResolveComponents() for filename-stem precedence resolution across multiple tiers in cc-deck/internal/build/component.go
- [X] T014 [P] Unit tests for LoadComponentsFromFS with valid and invalid YAML in cc-deck/internal/build/component_test.go
- [X] T015 [P] Unit tests for ValidateComponent covering all required field checks in cc-deck/internal/build/component_test.go
- [X] T016 [P] Unit tests for MatchComponent covering always, tools, credentials, features, and OR semantics in cc-deck/internal/build/component_test.go
- [X] T017 [P] Unit tests for ResolveComponents verifying filename-stem precedence across tiers in cc-deck/internal/build/component_test.go

**Checkpoint**: Component loading pipeline ready. User story implementation can now begin.

---

## Phase 3: User Stories 1+2 - Deterministic Assembly + Component Files (Priority: P1) MVP

**Goal**: Replace hardcoded policy generation with component-based deterministic assembly. Running `build refresh` produces a byte-identical `openshell/policy.yaml` from component files every time.

**Independent Test**: Run `cc-deck build refresh` twice with the same manifest. Verify both runs produce byte-identical `openshell/policy.yaml`. Verify the policy includes correct endpoints for each tool and credential in the manifest.

### Implementation

- [X] T018 [US1] Implement AssemblePolicy() that loads embedded components, evaluates matches, sorts by key, and produces a PolicyFile in cc-deck/internal/build/policy.go
- [X] T019 [US2] Implement LoadEmbeddedComponents() using the go:embed policies FS in cc-deck/internal/build/component.go
- [X] T020 [US1] Integrate AssemblePolicy into refreshOpenShellTarget() to write openshell/policy.yaml in cc-deck/internal/cmd/build.go
- [X] T021 [US1] Apply MergePolicy() for targets.openshell.policy overrides after component assembly in cc-deck/internal/cmd/build.go
- [X] T022 [US1] Remove hardcoded toolEndpoints map, addToolEndpoints(), vertexEndpoints(), claudeCodeBinaries() from cc-deck/internal/build/policy.go
- [X] T023 [US1] Remove GeneratePolicy() function, replace callers with AssemblePolicy() in cc-deck/internal/build/policy.go
- [X] T024 [P] [US1] Test determinism: same manifest produces byte-identical output across two AssemblePolicy calls in cc-deck/internal/build/policy_test.go
- [X] T025 [P] [US1] Test component matching: cargo manifest matches rust.yaml endpoints in cc-deck/internal/build/policy_test.go
- [X] T026 [P] [US1] Test component matching: claude-vertex credential matches vertex-ai.yaml in cc-deck/internal/build/policy_test.go
- [X] T027 [P] [US1] Test always-true components (claude-code, git-hosting) appear with empty manifest in cc-deck/internal/build/policy_test.go
- [X] T028 [US1] Update existing TestDefaultPolicy, TestGeneratePolicy, TestMergePolicy tests to use component-based approach in cc-deck/internal/build/policy_test.go

**Checkpoint**: US1+US2 complete. `build refresh` deterministically generates policy from embedded components. MVP functional.

---

## Phase 4: User Story 3 - Remote Catalog (Priority: P2)

**Goal**: Users can update policy components without upgrading the cc-deck binary by running `capture` to fetch from the catalog repo.

**Independent Test**: Modify a component in the catalog repo. Run `cc-deck capture`. Verify the updated component is cached locally. Run `build refresh`. Verify the new endpoint appears in the policy.

### Implementation

- [X] T029 [US3] Implement CatalogIndex struct and FetchCatalogIndex() to download and parse catalog.yaml in cc-deck/internal/build/catalog.go
- [X] T030 [US3] Implement DownloadCatalogComponents() to fetch all component files and cache in .cc-deck/setup/openshell/components/ in cc-deck/internal/build/catalog.go
- [X] T031 [US3] Implement offline fallback: warn on network failure, continue without updating cache in cc-deck/internal/build/catalog.go
- [X] T032 [US3] Add catalog fetch step to capture command after workspace scan in cc-deck/internal/build/commands/cc-deck.capture.md
- [X] T033 [US3] Update AssemblePolicy() to load cached catalog components as middle precedence tier in cc-deck/internal/build/policy.go
- [X] T034 [P] [US3] Unit test for CatalogIndex parsing in cc-deck/internal/build/catalog_test.go
- [X] T035 [P] [US3] Unit test for offline fallback (network error produces warning, no crash) in cc-deck/internal/build/catalog_test.go
- [X] T036 [US3] Test catalog precedence: cached component overrides embedded component with same filename stem in cc-deck/internal/build/policy_test.go

**Checkpoint**: US3 complete. Catalog updates propagate to policy via capture + refresh.

---

## Phase 5: User Story 4 - User-Local Overrides (Priority: P3)

**Goal**: Users can add custom component files in `.cc-deck/setup/openshell/policies/` to include project-specific endpoints in the generated policy.

**Independent Test**: Create a custom component file in `.cc-deck/setup/openshell/policies/`. Run `build refresh`. Verify the custom endpoints appear in the generated policy alongside standard components.

### Implementation

- [X] T037 [US4] Add user-local component directory loading from .cc-deck/setup/openshell/policies/ to AssemblePolicy() as highest precedence tier in cc-deck/internal/build/policy.go
- [X] T038 [P] [US4] Test user-local component inclusion (always-true custom component appears in output) in cc-deck/internal/build/policy_test.go
- [X] T039 [P] [US4] Test user-local precedence: user-local overrides catalog component with same filename stem in cc-deck/internal/build/policy_test.go

**Checkpoint**: All user stories complete. Full three-tier component resolution operational.

---

## Phase 6: Polish and Cross-Cutting Concerns

**Purpose**: Documentation, cleanup, and validation across all stories

- [X] T040 [P] Update CLI reference for build refresh policy generation in docs/modules/reference/pages/cli.adoc
- [X] T041 [P] Update configuration reference for component file locations (.cc-deck/setup/openshell/policies/, .cc-deck/setup/openshell/components/) in docs/modules/reference/pages/configuration.adoc
- [X] T042 [P] Create Antora guide page for policy component system in docs/modules/using/pages/policy-components.adoc
- [X] T043 Update README.md with deterministic policy generation overview
- [X] T044 Remove deprecated WellKnownBinaries map and slugify() if no longer referenced in cc-deck/internal/build/policy.go
- [X] T045 Remove embedded default-policy.yaml from cc-deck/internal/openshell/ and update client.go CreateSandbox() to require an explicit policy file path (remove the fallback to embedded default; callers must provide the path to the generated openshell/policy.yaml)
- [X] T046 Run quickstart.md validation end-to-end

---

## Dependencies and Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies, can start immediately
- **Foundational (Phase 2)**: Depends on T001 (types) and T009 (embed). BLOCKS all user stories.
- **US1+US2 (Phase 3)**: Depends on Phase 2 completion. MVP delivery point.
- **US3 (Phase 4)**: Depends on Phase 3 (needs AssemblePolicy). Independent of US4.
- **US4 (Phase 5)**: Depends on Phase 3 (needs AssemblePolicy). Independent of US3.
- **Polish (Phase 6)**: Depends on all desired user stories being complete.

### User Story Dependencies

- **US1+US2 (P1)**: Can start after Foundational (Phase 2). No dependencies on other stories.
- **US3 (P2)**: Can start after US1+US2 (needs AssemblePolicy to extend). Independent of US4.
- **US4 (P3)**: Can start after US1+US2 (needs AssemblePolicy to extend). Independent of US3.

### Within Each Phase

- Tasks marked [P] can run in parallel (different files)
- Component YAML files (T002-T008) are fully parallel
- Test tasks (T014-T017, T024-T027, T034-T035, T038-T039) within a phase are parallel
- Implementation tasks without [P] must run sequentially

### Parallel Opportunities

- **Phase 1**: T002-T008 all create independent YAML files (7 parallel tasks)
- **Phase 2**: T014-T017 test different functions (4 parallel tasks)
- **Phase 3**: T024-T027 test different scenarios (4 parallel tasks)
- **Phase 4+5**: US3 and US4 are independent and can run in parallel after Phase 3

---

## Parallel Example: Phase 1 Setup

```bash
# All component YAML files can be created in parallel:
Task: "Create claude-code.yaml in cc-deck/internal/build/policies/"
Task: "Create git-hosting.yaml in cc-deck/internal/build/policies/"
Task: "Create rust.yaml in cc-deck/internal/build/policies/"
Task: "Create go.yaml in cc-deck/internal/build/policies/"
Task: "Create node.yaml in cc-deck/internal/build/policies/"
Task: "Create python.yaml in cc-deck/internal/build/policies/"
Task: "Create vertex-ai.yaml in cc-deck/internal/build/policies/"
```

## Parallel Example: Phase 3 Tests

```bash
# All determinism and matching tests in parallel:
Task: "Test determinism: byte-identical output in policy_test.go"
Task: "Test component matching: cargo → rust.yaml in policy_test.go"
Task: "Test component matching: claude-vertex → vertex-ai.yaml in policy_test.go"
Task: "Test always-true components with empty manifest in policy_test.go"
```

---

## Implementation Strategy

### MVP First (US1+US2 Only)

1. Complete Phase 1: Setup (types + YAML files + embed)
2. Complete Phase 2: Foundational (loading + matching + precedence)
3. Complete Phase 3: US1+US2 (assembly + build refresh integration)
4. **STOP and VALIDATE**: Test determinism, component matching, OpenShell 0.0.46 compliance
5. This is a shippable increment: deterministic policy from embedded components

### Incremental Delivery

1. Setup + Foundational + US1+US2 → Deterministic policy from embedded components (MVP)
2. Add US3 → Catalog updates without binary release
3. Add US4 → User-local custom components
4. Polish → Documentation, cleanup, validation
5. Each phase adds value without breaking previous functionality

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- US1 and US2 are combined in Phase 3 because US2 (component files) is a prerequisite for US1 (deterministic assembly) and both are P1
- US3 and US4 extend AssemblePolicy() with additional component tiers but are independently testable
- Constitution requires: tests, CLI reference, config reference, Antora guide, README update
- All documentation must use the prose plugin with cc-deck voice profile
- Use `make test` and `make lint`, never `go build` directly
