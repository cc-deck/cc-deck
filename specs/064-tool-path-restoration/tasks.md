# Tasks: Tool PATH Restoration in Container Builds

**Input**: Design documents from `/specs/064-tool-path-restoration/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, quickstart.md

**Organization**: Tasks are grouped by user story to enable independent implementation and testing.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: Foundational (Blocking Prerequisites)

**Purpose**: Add the tool path registry and resolution function

- [ ] T001 Add `toolPathRegistry` map constant in cc-deck/internal/build/containerfile.go: map `"go"` to `"/usr/local/go/bin"`, `"cargo"` to `"{home}/.cargo/bin"`, `"rust"` to `"{home}/.cargo/bin"`
- [ ] T002 Add `ResolveToolPaths(m *Manifest, homeDir string) []string` function in cc-deck/internal/build/containerfile.go: iterate manifest tools, match against registry keys using case-insensitive `strings.Contains`, replace `{home}` with homeDir, deduplicate results preserving order
- [ ] T003 Add `ToolPaths []string` field to `ContainerfileData` struct in cc-deck/internal/build/containerfile.go
- [ ] T004 Update `ContainerDataForTarget()` in cc-deck/internal/build/containerfile.go: call `ResolveToolPaths(m, homeDir)` and set the result on `ContainerfileData.ToolPaths` for both "container" and "openshell" targets
- [ ] T005 [P] Add unit tests for `ResolveToolPaths` in cc-deck/internal/build/containerfile_test.go: test Go tool match ("Go >= 1.25.0" matches "go" key), Rust/cargo match, no matches returns empty slice, deduplication (cargo+rust both map to same path), case-insensitive matching, home directory substitution for openshell (/sandbox) and container (/home/dev)
- [ ] T006 Run `make test` and `make lint` to verify foundational changes

**Checkpoint**: Registry and resolution function ready and tested.

---

## Phase 2: User Story 1+2 - Template Integration (Priority: P1+P2)

**Goal**: Prepend resolved tool paths to shell rc files in the Containerfile template

**Independent Test**: Build a container image with Go and Rust in the manifest, start a shell, verify both commands are on PATH

### Implementation

- [ ] T007 [US1] Add PATH prepend block at the top of cc-deck/internal/build/templates/containerfile/05-shell-finalize.tmpl: conditional on `.ToolPaths` being non-empty, generate a single `RUN` step that iterates `.bashrc` and `.zshrc` in `{{.HomeDir}}` and uses `sed -i '1i export PATH="<joined-paths>:$PATH"'` to prepend. No target gate (works for both openshell and container).
- [ ] T008 [P] [US1] Add test for template rendering with tool paths in cc-deck/internal/build/containerfile_test.go: render `05-shell-finalize` with ToolPaths populated, verify output contains the `sed` command with correct paths
- [ ] T009 [P] [US1] Add test for template rendering without tool paths in cc-deck/internal/build/containerfile_test.go: render `05-shell-finalize` with empty ToolPaths, verify no PATH prepend block is generated
- [ ] T010 [US1] Run `make test` and `make lint` to verify template changes

**Checkpoint**: Tool paths are prepended to shell rc files. US1 and US2 are independently testable.

---

## Phase 3: Polish & Cross-Cutting Concerns

**Purpose**: Documentation, cleanup, and final validation

- [ ] T011 [P] Update README.md with tool PATH restoration feature description
- [ ] T012 [P] Remove the manually added `/usr/local/go/bin` from the curated zshrc at /Users/rhuss/Development/ai/cc-deck/.cc-deck/setup/config/zshrc (line 19: replace `export PATH="/usr/local/go/bin:$GOPATH/bin:$PATH"` with `export PATH="$GOPATH/bin:$PATH"` since the build system now handles the Go install path)
- [ ] T013 Run full validation: `make test` and `make lint`, verify no regressions

---

## Dependencies & Execution Order

### Phase Dependencies

- **Foundational (Phase 1)**: No dependencies, can start immediately
- **US1+US2 (Phase 2)**: Depends on Phase 1 (needs ToolPaths field and ResolveToolPaths function)
- **Polish (Phase 3)**: Depends on Phase 2

### Parallel Opportunities

- T005 can run in parallel with T001-T004 (test file vs source file, but tests depend on the functions existing)
- T008, T009 can run in parallel (independent test functions)
- T011, T012 can run in parallel (different files)

---

## Implementation Strategy

### MVP First

1. Complete Phase 1: Registry + resolution (T001-T006)
2. Complete Phase 2: Template integration (T007-T010)
3. **STOP and VALIDATE**: Build image, verify `go` and `cargo` on PATH
4. Complete Phase 3: Cleanup (T011-T013)

---

## Notes

- T001-T004 all modify containerfile.go but add independent pieces (constant, function, field, caller update)
- The template change (T007) is a single block addition, not a modification of existing template logic
- T012 reverts the manual workaround applied earlier in this session
