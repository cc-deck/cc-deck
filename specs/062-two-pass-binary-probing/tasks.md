# Tasks: Two-Pass Binary Probing

**Input**: Design documents from `/specs/062-two-pass-binary-probing/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md, contracts/

**Tests**: Tests are included as they are required by the constitution (Principle I).

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: Setup

**Purpose**: No new project setup needed. Existing Go project with established structure.

(No tasks in this phase; project already initialized.)

---

## Phase 2: Foundational (Component Schema Extension)

**Purpose**: Extend the PolicyComponent struct and validation to support the new `probe_binaries` and `runtime_globs` fields. MUST complete before any user story work begins.

- [X] T001 Add `ProbeBinaries []string` and `RuntimeGlobs []string` fields to `PolicyComponent` struct in `cc-deck/internal/build/component.go`. Add yaml tags `probe_binaries,omitempty` and `runtime_globs,omitempty`. See data-model.md for exact struct definition.
- [X] T002 Extend `ValidateComponent()` in `cc-deck/internal/build/component.go` to validate new fields: `probe_binaries` entries must not contain `/` (binary names only); `runtime_globs` entries must start with `/` (absolute paths). Both fields are optional. See contracts/component-schema.md for validation rules.
- [X] T003 Add tests for new field parsing and validation in `cc-deck/internal/build/component_test.go`. Test cases: (a) component with probe_binaries parses correctly, (b) component with runtime_globs parses correctly, (c) probe_binaries entry containing `/` fails validation, (d) runtime_globs entry not starting with `/` fails validation, (e) both fields omitted passes validation.
- [X] T004 Run `make test` and `make lint` to verify foundational changes pass.

**Checkpoint**: PolicyComponent struct supports new fields with validation. All existing tests still pass.

---

## Phase 3: User Story 1 - Automatic Binary Path Discovery via Image Probing (Priority: P1)

**Goal**: Build an OpenShell image using a two-pass process: first pass builds without binary restrictions, probe step discovers actual binary paths, second pass rebuilds with correct policy.

**Independent Test**: Build an OpenShell image with a manifest that includes cargo and python3. Inspect the generated policy.yaml and verify that binary paths match actual locations in the image. Compare against running `which cargo` inside the image.

### Implementation for User Story 1

- [X] T005 [P] [US1] Create `ProbeResult` and `ProbeReport` types in new file `cc-deck/internal/build/probe.go`. ProbeResult has fields: Binary (string, json:"binary"), Path (string, json:"path"), Method (string, json:"method"). ProbeReport has fields: Results (map[string][]ProbeResult keyed by component key), Warnings ([]string), Duration (time.Duration). See data-model.md for details.
- [X] T006 [P] [US1] Implement `collectProbeBinaries(comp PolicyComponent) []string` in `cc-deck/internal/build/probe.go`. Returns `comp.ProbeBinaries` if non-empty, otherwise returns `comp.Match.Tools`. This provides the fallback behavior per FR-003.
- [X] T007 [US1] Implement `generateProbeScript(components []PolicyComponent) string` in `cc-deck/internal/build/probe.go`. For each component, collect probe binaries via `collectProbeBinaries()`. Generate a shell script where each binary gets: `timeout 30 which <binary>` with JSON output on success, fallback to `timeout 30 find / -name <binary> -type f -executable -print -quit` with JSON output, or not-found JSON on both failures. Output format is one JSON line per binary per research.md Decision 2. Include component key as context in the JSON so results can be mapped back.
- [X] T008 [US1] Implement `ProbeBinaries(ctx context.Context, runtime string, imageRef string, components []PolicyComponent) (*ProbeReport, error)` in `cc-deck/internal/build/probe.go`. Filter components to only those needing probing (no explicit binaries, has match.tools or probe_binaries). Generate probe script via `generateProbeScript()`. Execute `podman run --rm <imageRef> /bin/sh -c <script>` with 5-minute context timeout. Parse JSON lines from stdout into ProbeReport. Collect warnings for not-found binaries. Record total duration. See research.md Decisions 1, 2, 8.
- [X] T009 [P] [US1] Add unit tests for probe logic in new file `cc-deck/internal/build/probe_test.go`. Test cases: (a) `collectProbeBinaries` returns probe_binaries when set, (b) `collectProbeBinaries` falls back to match.tools when probe_binaries absent, (c) `generateProbeScript` generates correct shell script with timeout wrappers and JSON output, (d) `parseProbeOutput` (or inline parsing in ProbeBinaries) correctly parses JSON lines into ProbeReport, (e) not-found binaries produce warnings, (f) components with explicit binaries are excluded from probing.
- [X] T010 [US1] Implement `applyProbeResults(components []PolicyComponent, report *ProbeReport) []PolicyComponent` in `cc-deck/internal/build/policy.go`. For each component: if it has explicit Binaries (len > 0), preserve unchanged; otherwise, populate Binaries from probe results (found paths as PolicyBinary entries) combined with component's RuntimeGlobs (also as PolicyBinary entries). Deduplicate paths. This replaces the `resolveBinaries()` call at line 127 of policy.go.
- [X] T011 [US1] Implement `stripBinaries(components []PolicyComponent) []PolicyComponent` in `cc-deck/internal/build/policy.go`. Returns a copy of components where all entries without explicit binaries have their Binaries field set to nil. Components with explicit binaries (from YAML, e.g., claude-code.yaml) keep their binaries. Used for first-pass policy generation per research.md Decision 3.
- [X] T012 [US1] Refactor `AssemblePolicy()` in `cc-deck/internal/build/policy.go` to accept an `AssemblyOptions` struct with a `StripBinaries bool` field. When `StripBinaries` is true, call `stripBinaries()` instead of `resolveBinaries()` and return both the PolicyFile and the matched components list (needed for probing). Add a new `AssemblyResult` struct containing `Policy *PolicyFile` and `MatchedComponents []PolicyComponent`.
- [X] T013 [US1] Modify `runOpenShellBuild()` in `cc-deck/internal/cmd/build.go` to implement the two-pass flow. Current single-pass logic (lines 347-408) becomes: (1) call `refreshOpenShellPolicy()` in first-pass mode (stripped binaries), (2) build with `podman build -t <imageRef>:probe-build`, (3) call `build.ProbeBinaries()` with probe-build image and matched components, (4) call `refreshOpenShellPolicy()` in second-pass mode (applying probe results), (5) build final image with `podman build -t <imageRef>`, (6) call `oci.StampPolicyLabel()` on final image, (7) cleanup: `podman rmi <imageRef>:probe-build`. Skip two-pass entirely when no components need probing (no match.tools on any matched component). See research.md Decision 7 for tagging.
- [X] T014 [US1] Implement failure handling in `runOpenShellBuild()` in `cc-deck/internal/cmd/build.go`. On probe error or second-pass build failure: retag first-pass image as `<imageRef>:probe-debug` (via `podman tag`), keep it for debugging, return error with message suggesting inspection. Per FR-013 and research.md Decision 7.
- [X] T015 [US1] Update `refreshOpenShellPolicy()` in `cc-deck/internal/cmd/build.go` (lines 887-910) to accept a `*build.ProbeReport` parameter (nil for first pass, populated for second pass). When nil, call `AssemblePolicy()` with `AssemblyOptions{StripBinaries: true}`. When non-nil, call `AssemblePolicy()` with default options then `applyProbeResults()` with the probe report. Write the resulting policy to `openshell/policy.yaml`.
- [X] T016 [US1] Add tests for `applyProbeResults()` and `stripBinaries()` in `cc-deck/internal/build/policy_test.go`. Test cases: (a) explicit binaries preserved by both functions, (b) stripBinaries clears non-explicit binaries, (c) applyProbeResults populates from probe results, (d) applyProbeResults merges runtime_globs with probe results, (e) applyProbeResults deduplicates paths, (f) not-found binaries get only runtime_globs (no probed path).
- [X] T017 [US1] Run `make test` and `make lint` to verify US1 implementation.

**Checkpoint**: Two-pass build works end-to-end. Probed binary paths appear in the final policy. First-pass image cleaned up on success, retained on failure.

---

## Phase 4: User Story 2 - Runtime-Created Binaries Covered by Glob Patterns (Priority: P1)

**Goal**: Policy includes glob patterns for tools that create binaries at runtime (Python venvs, Rust toolchains, npx). These patterns allow the OpenShell supervisor to authorize binaries that did not exist at build time.

**Independent Test**: Build an image with Python tools. Inspect policy.yaml and verify it contains glob patterns like `/sandbox/**/bin/pip` alongside the probed path `/usr/bin/pip`.

### Implementation for User Story 2

- [X] T018 [P] [US2] Update `cc-deck/internal/build/policies/python.yaml` to add `probe_binaries: [pip, pip3, uv, python3]` and `runtime_globs: [/sandbox/**/bin/pip, /sandbox/**/bin/pip3, /sandbox/**/bin/uv, /sandbox/**/bin/python, /sandbox/**/bin/python3]`. Keep existing key, name, match, and endpoints unchanged.
- [X] T019 [P] [US2] Update `cc-deck/internal/build/policies/rust.yaml` to add `probe_binaries: [cargo, rustc]` and `runtime_globs: [/sandbox/.rustup/toolchains/*/bin/cargo, /sandbox/.rustup/toolchains/*/bin/rustc]`. Keep existing fields unchanged.
- [X] T020 [P] [US2] Update `cc-deck/internal/build/policies/node.yaml` to add `probe_binaries: [node, npm, npx]` and `runtime_globs: [/sandbox/**/node_modules/.bin/*]`. Keep existing fields unchanged.
- [X] T021 [P] [US2] Update `cc-deck/internal/build/policies/go.yaml` to add `probe_binaries: [go]` and `runtime_globs: [/sandbox/go/bin/*]`. Keep existing fields unchanged.
- [X] T022 [US2] Add test in `cc-deck/internal/build/component_test.go` that loads each updated embedded YAML component and verifies `ProbeBinaries` and `RuntimeGlobs` fields are populated correctly. Verify claude-code.yaml, git-hosting.yaml, and vertex-ai.yaml have empty ProbeBinaries and RuntimeGlobs.
- [X] T023 [US2] Run `make test` and `make lint` to verify US2 implementation.

**Checkpoint**: All 4 tool-matched component YAMLs have probe_binaries and runtime_globs. Policy assembly includes glob patterns in the binaries field alongside probed paths.

---

## Phase 5: User Story 3 - Well-Known Paths Table Eliminated (Priority: P2)

**Goal**: Remove the `wellKnownPaths` table and `resolveBinaries()` function from `policy_binaries.go`. New tools only need a component YAML change, not Go code changes.

**Independent Test**: Add a hypothetical new policy component (e.g., for mix/Hex) with only match.tools and probe_binaries. Build an image. Verify the policy contains the correct probed path without any changes to Go code.

### Implementation for User Story 3

- [X] T024 [US3] Verify that all entries in `wellKnownPaths` table (in `cc-deck/internal/build/policy_binaries.go` lines 10-57) are covered by the new `probe_binaries` and `runtime_globs` fields in the updated component YAML files. Cross-reference each tool (cargo, rustc, go, node, npm, npx, pip, pip3, uv, claude, git, gh) against the YAML files. For tools in always-match components (claude, git, gh), verify they have explicit binaries and are unaffected. Document any gaps.
- [X] T025 [US3] Remove `cc-deck/internal/build/policy_binaries.go` entirely (wellKnownPaths table, resolveBinaries(), addPath()).
- [X] T026 [US3] Remove `cc-deck/internal/build/policy_binaries_test.go` entirely.
- [X] T027 [US3] Remove the `resolveBinaries()` call at line 127 of `cc-deck/internal/build/policy.go`. In the refactored `AssemblePolicy()` (from T012), the `StripBinaries` path calls `stripBinaries()` and the default path calls `applyProbeResults()`. For non-OpenShell targets that never call the two-pass flow, components with explicit binaries already work; components without explicit binaries get no binary restrictions (empty binaries field), which is correct since non-OpenShell targets do not use the OpenShell supervisor.
- [X] T028 [US3] Run `make test` and `make lint` to verify clean removal. All existing tests must pass.

**Checkpoint**: `policy_binaries.go` no longer exists. No hardcoded binary path table. Adding a new tool requires only a component YAML change.

---

## Phase 6: User Story 4 - Layer Caching Minimizes Rebuild Cost (Priority: P3)

**Goal**: The second-pass rebuild reuses all cached layers from the first pass. Only the policy COPY layer and later layers rebuild.

**Independent Test**: Build an image. Time the build. Change only the manifest endpoints. Rebuild. Verify the second pass completes quickly (under 10 seconds with warm cache).

### Implementation for User Story 4

- [X] T029 [US4] Verify that the Containerfile template ordering in `cc-deck/internal/build/templates/containerfile/` places all tool installation layers (03-mandatory-stack, generated package/language/github-release sections) before the policy COPY layer (04-openshell-extras). No code changes expected; this is a verification task. Document the layer ordering.
- [X] T030 [US4] Add build output messages in `runOpenShellBuild()` in `cc-deck/internal/cmd/build.go` that report timing for each phase: first-pass build time, probe time, second-pass build time. Use `time.Since()` to measure. Print as `fmt.Printf("  First pass: %s\n  Probe: %s\n  Second pass: %s\n", ...)`. This helps developers verify caching behavior.
- [X] T031 [US4] Run `make test` and `make lint` to verify US4 implementation.

**Checkpoint**: Build timing output confirms second-pass rebuild is fast on warm cache.

---

## Phase 7: Polish and Cross-Cutting Concerns

**Purpose**: Documentation, cleanup, and final verification across all user stories.

- [X] T032 [P] Update `README.md` with user-facing changes: explain that `cc-deck build run` for OpenShell targets now uses a two-pass probe-based build. Mention the `<name>:probe-debug` image retained on failure. Reference the new component YAML fields for component authors.
- [X] T033 [P] Update CLI reference documentation (`docs/modules/reference/pages/cli.adoc`) if build command output format changes (timing output from T030, new probe status messages).
- [X] T034 [P] Update configuration reference documentation (`docs/modules/reference/pages/configuration.adoc`) to document the new `probe_binaries` and `runtime_globs` fields in policy component YAML files. Include the schema from contracts/component-schema.md.
- [X] T035 Run full `make test` and `make lint` to verify all changes pass across the entire project.
- [ ] T036 Run `make install` and perform a manual smoke test: build an OpenShell image with a manifest including Python and Rust tools. Verify the policy.yaml contains probed paths and runtime globs. Verify the first-pass image is cleaned up.

---

## Dependencies and Execution Order

### Phase Dependencies

- **Phase 1 (Setup)**: Empty, no work needed
- **Phase 2 (Foundational)**: No dependencies, start immediately. BLOCKS all user stories.
- **Phase 3 (US1)**: Depends on Phase 2 completion
- **Phase 4 (US2)**: Depends on Phase 2 completion. Can run in parallel with Phase 3 (YAML changes are independent files)
- **Phase 5 (US3)**: Depends on Phase 3 AND Phase 4 completion (must remove old code only after new code works)
- **Phase 6 (US4)**: Depends on Phase 3 completion (needs two-pass flow to measure)
- **Phase 7 (Polish)**: Depends on all user stories being complete

### User Story Dependencies

- **US1 (P1)**: Can start after Foundational (Phase 2). Core two-pass mechanism.
- **US2 (P1)**: Can start after Foundational (Phase 2). YAML changes are independent of US1 code, but final integration requires US1's `applyProbeResults()`.
- **US3 (P2)**: Depends on US1 + US2. Cannot remove old code until new code works.
- **US4 (P3)**: Depends on US1. Verification and timing instrumentation.

### Within Each User Story

- Types and helpers before main functions
- Policy assembly functions before build command integration
- Tests after implementation
- Lint/test checkpoint at end of each story

### Parallel Opportunities

- T005 and T006 (types/helpers in probe.go) can run in parallel with T009 (tests in probe_test.go structure)
- T018, T019, T020, T021 (YAML updates) can all run in parallel
- T032, T033, T034 (docs) can all run in parallel
- US1 code tasks and US2 YAML tasks can run in parallel (different files)

---

## Parallel Example: User Story 2

```bash
# Launch all YAML updates in parallel (different files, no dependencies):
Task: "Update python.yaml with probe_binaries and runtime_globs"
Task: "Update rust.yaml with probe_binaries and runtime_globs"
Task: "Update node.yaml with probe_binaries and runtime_globs"
Task: "Update go.yaml with probe_binaries and runtime_globs"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 2: Foundational (component schema extension)
2. Complete Phase 3: User Story 1 (two-pass probe mechanism)
3. **STOP and VALIDATE**: Test with a real build, verify probed paths appear in policy
4. The well-known paths table is still present but bypassed by the two-pass flow

### Incremental Delivery

1. Foundational → schema ready
2. US1 → two-pass probe works end-to-end (MVP)
3. US2 → YAML components have runtime globs
4. US3 → old code removed, fully self-contained components
5. US4 → timing instrumentation confirms cache performance
6. Polish → documentation updated, smoke test passes

### Single Developer Strategy

Execute phases sequentially in order: 2 → 3 → 4 → 5 → 6 → 7. Within phases, parallelize tasks marked [P] where possible.

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story should be independently completable and testable
- Commit after each task or logical group
- Stop at any checkpoint to validate story independently
- Use `make test` and `make lint` (never `go build` directly)
- Use `podman` exclusively (never Docker)
