# Review Summary: Plugin Lifecycle Management

**Feature**: 009-plugin-lifecycle
**Date**: 2026-03-04
**Reviewer**: Automated (sdd:review-plan)

## Overall Assessment: PASS (Score: 9/10)

The plan and tasks are well-structured, appropriately scoped, and ready for implementation.

---

## Coverage Matrix

| FR | Description | Task(s) | Covered |
|----|-------------|---------|---------|
| FR-001 | Embed WASM binary at build time | T003, T004 | Yes |
| FR-002 | Write binary to Zellij plugins dir | T009 | Yes |
| FR-003 | Create parent directories | T009 | Yes |
| FR-004 | Install layout file | T010 | Yes |
| FR-005 | Two layout templates (minimal/full) | T006, T010 | Yes |
| FR-006 | Default to minimal layout | T010, T012 | Yes |
| FR-007 | Prompt before overwrite | T009 | Yes |
| FR-008 | Inject into default layout | T014 | Yes |
| FR-009 | Detect/skip duplicate injection | T014 | Yes |
| FR-010 | Display install summary | T011 | Yes |
| FR-011 | Report installation state | T017 | Yes |
| FR-012 | Zellij version compatibility check | T008, T017 | Yes |
| FR-013 | List layout files referencing plugin | T017, T018 | Yes |
| FR-014 | Warn if Zellij not found | T025 | Yes |
| FR-015 | Remove binary + layout + injection | T021 | Yes |
| FR-016 | Preserve non-cc-deck layout content | T006 (RemoveInjection), T021 | Yes |
| FR-017 | Report removal summary | T021 | Yes |
| FR-018 | Handle not-installed on remove | T022 | Yes |
| FR-019 | Respect ZELLIJ_CONFIG_DIR | T005 | Yes |
| FR-020 | Local scope only | Architectural (no remote code) | Yes |
| FR-021 | Actionable permission error messages | T024 | Yes |
| FR-022 | Skip unparseable layout injection | T015 | Yes |
| FR-023 | Warn about restart on remove | T022 | Yes |
| FR-024 | Atomic file writes | T009 | Yes |

**Coverage**: 24/24 FRs covered (100%)

---

## Task Quality Assessment

### Format Compliance

- All 27 tasks use correct checklist format: `- [ ] TXXX [labels] description with file path`
- All tasks have beads IDs assigned
- Story labels ([US1]-[US5]) correctly applied to user story phases
- [P] markers applied to parallelizable tasks

### Task Granularity

- **Good**: Tasks are right-sized (each produces a concrete artifact or testable behavior)
- **Good**: No tasks span multiple files without [P] marking
- **Good**: Each phase has a checkpoint for validation

### Red Flag Scan

| Check | Result |
|-------|--------|
| Tasks with no file path | None found |
| Vague tasks ("improve", "refactor") | None found |
| Missing story labels in story phases | None found |
| Cross-file tasks without [P] marker | T024, T025 touch two files each but are polish/cross-cutting, acceptable |
| Circular dependencies | None found |
| Orphaned tasks (no phase) | None found |

### Dependency Correctness

- Phase 1 -> Phase 2 -> Phase 3 (MVP path): Correct, linear
- Phase 4 depends on Phase 3: Correct (extends install command)
- Phases 5 and 6 depend only on Phase 2: Correct (independent)
- Phase 7 depends on Phases 3-6: Correct (polish after features)
- US4 and US5 can parallel with US1+2: Correctly identified

---

## NFR Validation

| NFR/Constraint | How Addressed |
|----------------|---------------|
| Offline-capable | T004 embeds binary via go:embed, no network calls |
| Single binary | T003 + T004 produce embedded artifact |
| Atomic writes | T009 specifies temp file + rename |
| ZELLIJ_CONFIG_DIR | T005 reads env var with fallback |

---

## Recommendations

### Minor (non-blocking)

1. **T007 (state.go) may be premature**: The `DetectInstallState()` function is only used by `Status()` (T017). Consider merging T007 into T017 to reduce foundational phase scope and get to MVP faster. The InstallState struct could live in status.go.

2. **T003 is a manual build step**: Consider adding a note that this task is a prerequisite for `go build` and should be automated by T026 (Makefile). The ordering is correct but worth calling out.

### Observations (no action needed)

- Smart decision to combine US1 and US2 into one phase since layout creation is integral to install.
- The 27-task count is appropriate for the feature scope (~600-800 lines of new Go code).
- No test tasks generated (correct, not requested in spec).

---

## Reviewer Guidance

When reviewing this plan/PR, focus on:

1. **research.md**: Verify the string-level injection approach (no KDL parser) is acceptable given the sentinel marker strategy
2. **data-model.md**: Check that InstallState captures enough information for the status command's rich output
3. **contracts/cli-commands.md**: Validate the output format matches the existing cc-deck CLI style (tabwriter alignment, output flag support)
4. **tasks.md**: Confirm phase dependencies are correct and MVP scope (Phase 1-3, 12 tasks) is achievable

---

**Verdict**: Plan is solid, tasks are complete, coverage is 100%. Ready for implementation or PR.
