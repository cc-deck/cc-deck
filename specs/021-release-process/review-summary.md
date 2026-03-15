# Review Summary: 021-release-process

**Reviewed**: 2026-03-15
**Artifacts**: spec.md, plan.md, tasks.md, research.md, data-model.md, quickstart.md

## Coverage Matrix

| Spec Requirement | Plan Section | Task(s) | Status |
|-----------------|-------------|---------|--------|
| FR-001: Cross-platform build (4 combos) | GoReleaser builds section | T009, T013 | Covered |
| FR-002: tar.gz archives with README/LICENSE | GoReleaser archives section | T009 | Covered |
| FR-003: SHA-256 checksums | GoReleaser checksum config | T010 | Covered |
| FR-004: Homebrew formula in tap repo | GoReleaser brews section | T017, T018, T019 | Covered |
| FR-005: RPM packages (amd64, arm64) | GoReleaser nFPM section | T011, T026 | Covered |
| FR-006: DEB packages (amd64, arm64) | GoReleaser nFPM section | T011, T027 | Covered |
| FR-007: Multi-arch container images to quay.io/cc-deck | Container image release job | T023, T024, T025 | Covered |
| FR-008: Tag-triggered release | GitHub Actions workflow | T014, T016 | Covered |
| FR-009: WASM build before Go cross-compile | GoReleaser before hooks | T009 | Covered |
| FR-010: Registry migration quay.io/rhuss → quay.io/cc-deck | Setup phase | T001-T008 | Covered |
| FR-011: Changelog from commits | GoReleaser changelog config | T012 | Covered |
| FR-012: Flatpak manifest | Flatpak phase | T028-T031 | Covered |
| FR-013: Version from git tag | GoReleaser ldflags | T009 | Covered |
| US1: Homebrew install | Phase 4 | T017-T019 | Covered |
| US2: GitHub Release downloads | Phase 5 | T020-T022 | Covered |
| US3: RPM/DEB installation | Phase 7 | T026-T027 | Covered |
| US4: Flatpak | Phase 8 | T028-T031 | Covered |
| US5: Automated release pipeline | Phase 3 | T014-T016 | Covered |
| US6: Container images on new registry | Phase 6 | T023-T025 | Covered |
| SC-001: Single tag produces full release | Pipeline + verification | T016, T035 | Covered |
| SC-002: Homebrew install within 5 min | Homebrew tap auto-update | T019 | Covered |
| SC-003: Artifacts available within 10 min | Pipeline performance goal | T016 | Covered |
| SC-004: Multi-arch container images | Container image job | T025 | Covered |
| SC-005: Zero quay.io/rhuss references | Registry migration verification | T008 | Covered |
| SC-006: Local dry run | GoReleaser snapshot | T013, T035 | Covered |
| Edge: WASM build failure | GoReleaser before hooks fail-fast | T009 (implicit) | Covered |
| Edge: Homebrew tap update failure | Separate job, non-blocking | T014 (workflow design) | Covered |
| Edge: quay.io unreachable | Separate job from GoReleaser | T023 (separate job) | Covered |
| Edge: Old registry images | Documentation migration | T004-T007, T034 | Covered |

**Coverage**: 29/29 items fully covered.

## Red Flags

None. The plan is well-structured with clear phase dependencies.

## Task Quality

| Criterion | Assessment |
|-----------|-----------|
| Exact file paths | All tasks specify target files |
| Testable completion | Checkpoints at each phase boundary, verification tasks (T008, T013, T016, T019, T022, T025) |
| Dependency ordering | Correct: Phase 2 blocks Phase 3, Phase 3 blocks Phases 4-7 |
| Parallel markers | Present and accurate ([P] on independent tasks) |
| Size consistency | Tasks are well-scoped (one logical unit each) |
| Story traceability | All user story tasks tagged with [US*] |

## Observations

1. **Clean separation**: Registry migration (Phase 1) is independent from GoReleaser setup (Phase 2), allowing parallel execution.

2. **Good MVP scope**: Phases 1-3 produce a working automated release before tackling Homebrew, docs, or Flatpak.

3. **Verification tasks**: Each phase has a verification task (T008, T013, T016, T019, T022, T025, T026-T027, T031, T035), which is excellent for incremental validation.

4. **Container image strategy**: Correctly uses a separate CI job with Podman rather than trying to force GoReleaser's Docker support, aligning with the project's container runtime preference.

5. **T034 overlap with T005**: T034 updates `one-liner.adoc` with quay.io/cc-deck references, but T005 already covers docs/ references in Phase 1. T034 may be redundant. Consider removing T034 or clarifying it handles content beyond registry references (e.g., adding new install methods).

## Recommendation

**APPROVE**. All spec requirements are covered. The plan and tasks are consistent, well-ordered, and ready for implementation. The single minor overlap (T034/T005) does not block execution.

## Reviewer Guidance

When reviewing this spec PR, focus on:

1. **GoReleaser config** (plan.md, GoReleaser Configuration Details): Verify the before hooks correctly build WASM, the ldflags match the Go source, and the builds.dir is set to `cc-deck/`.
2. **Registry migration scope** (tasks T001-T008): Confirm all files with `quay.io/rhuss` references are listed. Run `rg quay.io/rhuss` to validate completeness.
3. **Container image CI** (tasks T023-T025): Verify the release workflow uses Podman (not Docker) and the multi-arch manifest push pattern matches the existing Makefile targets.
4. **Flatpak feasibility** (tasks T028-T031): Flatpak for CLI tools is unusual. Consider whether this adds sufficient value for a terminal-only application. The manifest may need special permissions for host filesystem access.
