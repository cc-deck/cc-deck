# Tasks: Policy Binary Resolution

**Input**: Design documents from `specs/061-policy-binary-resolution/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: Setup

**Purpose**: Remove hardcoded binaries from embedded components

- [ ] T001 [P] Remove the `binaries` section from `cc-deck/internal/build/policies/go.yaml` (keep key, name, match, endpoints)
- [ ] T002 [P] Remove the `binaries` section from `cc-deck/internal/build/policies/rust.yaml` (keep key, name, match, endpoints)
- [ ] T003 [P] Remove the `binaries` section from `cc-deck/internal/build/policies/node.yaml` (keep key, name, match, endpoints)
- [ ] T004 [P] Remove the `binaries` section from `cc-deck/internal/build/policies/python.yaml` (keep key, name, match, endpoints)

---

## Phase 2: Foundational (Resolution Logic)

**Purpose**: Implement the well-known paths table and binary resolution function

- [ ] T005 Create `cc-deck/internal/build/policy_binaries.go` with the well-known paths table (`var wellKnownPaths = map[string][]string{...}`) covering: cargo, rustc, go, claude, node, npm, npx, pip, pip3, uv, git, gh
- [ ] T006 Implement `resolveBinaries(components []PolicyComponent, manifest *Manifest) []PolicyComponent` in `cc-deck/internal/build/policy_binaries.go` that: (1) iterates matched components, (2) skips components with existing binaries, (3) for each tool in match.Tools looks up manifest.Tools to determine install path, (4) adds well-known paths, (5) deduplicates, (6) sets component.Binaries
- [ ] T007 [P] Write unit tests in `cc-deck/internal/build/policy_binaries_test.go` covering: package-installed tool resolves to /usr/bin/<name> plus well-known paths, github-release tool uses InstallPath, component with explicit binaries is preserved, tool not in manifest is skipped, deduplication works

**Checkpoint**: Resolution logic is complete and tested in isolation.

---

## Phase 3: User Story 1 - Automatic Binary Resolution (Priority: P1)

**Goal**: Policy assembly automatically populates binaries from manifest data.

**Independent Test**: Call AssemblePolicy with a manifest containing tools, verify output policy entries have resolved binaries.

### Implementation for User Story 1

- [ ] T008 [US1] Modify `AssemblePolicy()` in `cc-deck/internal/build/policy.go` to call `resolveBinaries(matched, manifest)` after component matching (after the sort, before building networkPolicies map), replacing `matched` with the resolved result
- [ ] T009 [US1] Write integration test `TestAssemblePolicy_ResolvesToolBinaries` in `cc-deck/internal/build/policy_test.go` that creates a manifest with tools (cargo as package, a custom tool as github-release), calls AssemblePolicy, and verifies the pkg_rust policy entry has cargo binary paths and the custom tool's policy entry has its install_path

**Checkpoint**: AssemblePolicy automatically resolves binaries. `make test` passes.

---

## Phase 4: User Story 2 - Catalog Components Without Binaries (Priority: P2)

**Goal**: Catalog components with no binaries get resolved paths from manifest.

**Independent Test**: Remove binaries from an embedded component, assemble with matching manifest, verify binaries are populated.

### Implementation for User Story 2

- [ ] T010 [US2] Write test `TestAssemblePolicy_ComponentWithoutBinariesGetsResolved` in `cc-deck/internal/build/policy_test.go` that verifies embedded go.yaml (now without binaries) gets `/usr/bin/go` and well-known paths when manifest contains a `go` tool entry

**Checkpoint**: Catalog components work without hardcoded binaries.

---

## Phase 5: User Story 3 - Explicit Override (Priority: P3)

**Goal**: Components with explicit binaries are not modified by resolution.

### Implementation for User Story 3

- [ ] T011 [US3] Write test `TestAssemblePolicy_ExplicitBinariesPreserved` in `cc-deck/internal/build/policy_test.go` that creates a component with explicit binaries, calls AssemblePolicy, and verifies the binaries are unchanged (no additional paths added)

**Checkpoint**: Explicit overrides work correctly.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Documentation and final validation

- [ ] T012 [P] Update README.md with a note about automatic binary resolution in policy assembly
- [ ] T013 Run `make test` and `make lint` to verify all tests pass and code is clean

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies, can start immediately
- **Foundational (Phase 2)**: Can run in parallel with Phase 1 (different files)
- **User Story 1 (Phase 3)**: Depends on Phase 2 (needs resolveBinaries function)
- **User Story 2 (Phase 4)**: Depends on Phase 1 (embedded components modified) and Phase 3
- **User Story 3 (Phase 5)**: Depends on Phase 3
- **Polish (Phase 6)**: Depends on all user stories

### Parallel Opportunities

- T001-T004 can all run in parallel (different YAML files)
- T005 and T007 can overlap (table definition vs tests)
- T010 and T011 are independent tests

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Remove hardcoded binaries (T001-T004)
2. Complete Phase 2: Resolution logic (T005-T007)
3. Complete Phase 3: Wire into AssemblePolicy (T008-T009)
4. **STOP and VALIDATE**: Run `make test`, verify pkg_rust gets binaries

### Incremental Delivery

1. Phase 1+2 -> Resolution infrastructure ready
2. Phase 3 -> AssemblePolicy uses resolution (MVP)
3. Phase 4 -> Confirm catalog independence
4. Phase 5 -> Confirm explicit override
5. Phase 6 -> Documentation

---

## Notes

- Total tasks: 13
- Tasks per user story: US1=2, US2=1, US3=1
- The well-known paths table is the most maintenance-prone part; new tools need entries added manually
- Existing tests that check for binaries in embedded components may need updating after Phase 1
