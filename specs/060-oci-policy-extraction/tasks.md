# Tasks: OCI Policy Extraction

**Input**: Design documents from `specs/060-oci-policy-extraction/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: Setup

**Purpose**: Add go-containerregistry dependency and create the new package structure

- [x] T001 Add `github.com/google/go-containerregistry` dependency via `go get github.com/google/go-containerregistry` in cc-deck/
- [x] T002 Create package directory `cc-deck/internal/oci/` with doc.go containing package documentation

---

## Phase 2: Foundational (Core OCI Package)

**Purpose**: Implement the `internal/oci/` package that both user stories depend on

- [x] T003 [P] Implement `FindLayerContaining(img v1.Image, filePath string) (v1.Hash, error)` in `cc-deck/internal/oci/label.go` that walks image layers in reverse order, opens each as a tar archive, and returns the diff ID of the first layer containing the specified file path
- [x] T004 [P] Implement `AddLabel(imageRef, key, value string) error` in `cc-deck/internal/oci/label.go` that loads an image from the local podman daemon via `daemon.Image`, mutates the config to add the label using `mutate.Config`, and writes the image back via `daemon.Write`
- [x] T005 Implement `ExtractFileFromImage(imageRef, filePath string) ([]byte, error)` in `cc-deck/internal/oci/extract.go` that: (1) resolves the image from local daemon or remote registry, (2) checks for `dev.cc-deck.policy-layer` label, (3) if found, extracts file from that layer, (4) if not found or file missing, falls back to `FindLayerContaining` and extracts from the matched layer, (5) logs extraction source and outcome at INFO/DEBUG level
- [x] T006 [P] Write unit tests for `FindLayerContaining` and `AddLabel` in `cc-deck/internal/oci/label_test.go` using test images created via `mutate.AppendLayer` with synthetic tar layers containing test files
- [x] T007 [P] Write unit tests for `ExtractFileFromImage` in `cc-deck/internal/oci/extract_test.go` covering: labeled image (fast path), unlabeled image (fallback scan), missing file (error), and stale label (fallback)

**Checkpoint**: The `internal/oci/` package is complete and tested. Both user stories can now proceed.

---

## Phase 3: User Story 1 - Runtime Policy Extraction (Priority: P1)

**Goal**: `cc-deck ws new --type openshell` automatically extracts the policy file from the OCI image and passes it to sandbox creation.

**Independent Test**: Run `cc-deck ws new --type openshell` with a labeled or unlabeled openshell image and verify the sandbox starts with the correct policy.

### Implementation for User Story 1

- [x] T008 [US1] Modify `resolveSandboxConfig()` in `cc-deck/internal/ws/openshell.go` to call `oci.ExtractFileFromImage(def.SandboxImage, "/etc/openshell/policy.yaml")` when `def.Policy` is empty and `def.SandboxImage` is set, write the extracted bytes to a temp file via `os.CreateTemp`, and set `cfg.Policy` to the temp file path
- [x] T009 [US1] Add temp file cleanup in the `Create()` method of `cc-deck/internal/ws/openshell.go` using `defer os.Remove(tempPolicyPath)` after `resolveSandboxConfig` returns, ensuring cleanup on both success and failure
- [x] T010 [US1] Update error handling in `cc-deck/internal/ws/openshell.go` to produce a clear message when extraction fails, suggesting the `--policy` flag as a manual alternative (FR-010)
- [x] T011 [US1] Verify that `resolveSandboxConfig()` in `cc-deck/internal/ws/openshell.go` no longer relies on host-path resolution for policy files (FR-011). The current code sets `cfg.Policy = def.Policy` from the definition; confirm that no other code path resolves policy via filesystem lookup, and that the new OCI extraction path in T008 is the sole automatic resolution mechanism
- [x] T012 [US1] Write a unit test in `cc-deck/internal/ws/openshell_test.go` that verifies `resolveSandboxConfig` calls OCI extraction when no explicit policy is set, using a mock or test image

**Checkpoint**: User Story 1 is complete. `ws new --type openshell` can extract policies from images.

---

## Phase 4: User Story 2 - Build-Time Label Stamping (Priority: P2)

**Goal**: `cc-deck build run --target openshell` automatically stamps the built image with the `dev.cc-deck.policy-layer` label.

**Independent Test**: Build an openshell image and inspect labels to verify `dev.cc-deck.policy-layer` is present.

### Implementation for User Story 2

- [x] T013 [US2] Modify `runOpenShellBuild()` in `cc-deck/internal/cmd/build.go` to call `oci.FindLayerContaining` and `oci.AddLabel` after the `podman build` succeeds but before the optional push step, using the built image reference and the key `dev.cc-deck.policy-layer`
- [x] T014 [US2] Add error handling in `cc-deck/internal/cmd/build.go` so that if the policy file is not found in the built image, a warning is logged but the build does not fail
- [x] T015 [US2] Write a unit test in `cc-deck/internal/cmd/build_test.go` that verifies label stamping is called after a successful openshell build

**Checkpoint**: User Story 2 is complete. Built openshell images have the policy layer label.

---

## Phase 5: User Story 3 - Backward Compatibility (Priority: P3)

**Goal**: Images without the `dev.cc-deck.policy-layer` label still work for sandbox creation via fallback layer scan.

**Independent Test**: Use an unlabeled image with `ws new --type openshell` and verify the policy is extracted successfully.

### Implementation for User Story 3

- [x] T016 [US3] Verify the fallback path in `ExtractFileFromImage` handles unlabeled images correctly by adding a dedicated integration-style test in `cc-deck/internal/oci/extract_test.go` that creates a multi-layer test image without labels and confirms the correct file is extracted from the topmost layer

**Checkpoint**: All three user stories are complete and independently testable.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Documentation, cleanup, and cross-story validation

- [x] T017 [P] Update README.md with information about automatic policy extraction from OCI images during `ws new --type openshell`
- [x] T018 [P] Update CLI reference in `docs/modules/reference/pages/cli.adoc` to document the automatic policy extraction behavior of `ws new` and the label stamping behavior of `build run --target openshell`
- [x] T019 [P] Update configuration reference in `docs/modules/reference/pages/configuration.adoc` if any new configuration options or file locations are introduced
- [x] T020 [P] Create Antora guide page at `docs/modules/using/pages/oci-policy-extraction.adoc` explaining the two-phase approach (build-time labeling, runtime extraction) and the fallback behavior
- [x] T021 Run `make test` and `make lint` to verify all tests pass and code is clean

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies, can start immediately
- **Foundational (Phase 2)**: Depends on Phase 1 completion, BLOCKS all user stories
- **User Story 1 (Phase 3)**: Depends on Phase 2 completion
- **User Story 2 (Phase 4)**: Depends on Phase 2 completion, can run in parallel with US1
- **User Story 3 (Phase 5)**: Depends on Phase 2 completion, can run in parallel with US1/US2
- **Polish (Phase 6)**: Depends on all user stories being complete

### User Story Dependencies

- **User Story 1 (P1)**: Independent after Phase 2
- **User Story 2 (P2)**: Independent after Phase 2, can run in parallel with US1
- **User Story 3 (P3)**: Independent after Phase 2, primarily validates existing Phase 2 code

### Parallel Opportunities

- T003 and T004 can run in parallel (different functions in label.go, but same file so coordinate)
- T006 and T007 can run in parallel (different test files)
- US1, US2, and US3 implementation phases can run in parallel after Phase 2
- All Polish phase tasks marked [P] can run in parallel
- T017, T018, T019, T020 (docs) can all run in parallel

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (T001-T002)
2. Complete Phase 2: Foundational (T003-T007)
3. Complete Phase 3: User Story 1 (T008-T012)
4. **STOP and VALIDATE**: Test `ws new --type openshell` with a real openshell image
5. This delivers the core value: policy extraction works without host build directory

### Incremental Delivery

1. Setup + Foundational -> Core OCI package ready
2. Add User Story 1 -> Runtime extraction works (MVP)
3. Add User Story 2 -> Build-time optimization active
4. Add User Story 3 -> Backward compatibility validated
5. Polish -> Documentation complete

---

## Notes

- Total tasks: 21
- Tasks per user story: US1=5, US2=3, US3=1
- Parallel opportunities: T003/T004, T006/T007, US1/US2/US3 after Phase 2, all docs tasks
- The `internal/oci/` package has no dependencies on other internal packages, making it easy to test in isolation
- Constitution requires tests and documentation, covered by Phase 2 tests (T006-T007) and Phase 6 docs (T017-T020)
