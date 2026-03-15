# Tasks: Release Process

**Input**: Design documents from `/specs/021-release-process/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md, quickstart.md

**Tests**: Not explicitly requested. Verification via dry-run and manual validation.

**Organization**: Tasks grouped by user story for independent implementation.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: Setup

**Purpose**: Registry migration and version infrastructure

- [x] T001 Update `Makefile` default REGISTRY from `quay.io/rhuss` to `quay.io/cc-deck`
- [x] T002 [P] Update `cc-deck/internal/cmd/version.go` default ImageRegistry from `quay.io/rhuss` to `quay.io/cc-deck`
- [x] T003 [P] Update `demo-image/Containerfile` BASE_IMAGE arg from `quay.io/rhuss/cc-deck-base:latest` to `quay.io/cc-deck/cc-deck-base:latest`
- [x] T004 Update all `quay.io/rhuss` references in `README.md` to `quay.io/cc-deck`
- [x] T005 [P] Update all `quay.io/rhuss` references in `docs/` (one-liner.adoc, podman.adoc, kubernetes.adoc, credentials.adoc, first-session.adoc)
- [x] T006 [P] Update all `quay.io/rhuss` references in `demos/` (scripts, README, narration)
- [x] T007 [P] Update all `quay.io/rhuss` references in Go source and build files
- [x] T008 Verify: `rg 'quay.io/rhuss'` returns zero results across the entire codebase

**Checkpoint**: All registry references point to quay.io/cc-deck. No quay.io/rhuss remains.

---

## Phase 2: Foundational (GoReleaser Config)

**Purpose**: Core release automation configuration. BLOCKS all release user stories.

- [ ] T009 Create `.goreleaser.yaml` at project root with before hooks (WASM build + copy), builds section (dir: cc-deck, main: ./cmd/cc-deck, goos/goarch matrix, ldflags for Version/Commit/Date/ImageRegistry), archives section (tar.gz with README + LICENSE)
- [ ] T010 Add checksum configuration to `.goreleaser.yaml` (SHA-256 checksums file)
- [ ] T011 [P] Add nFPM configuration to `.goreleaser.yaml` for RPM and DEB packages (package name, description, license, homepage, maintainer, recommends: zellij)
- [ ] T012 Add changelog configuration to `.goreleaser.yaml` (from git commits, group by type)
- [ ] T013 Verify: `goreleaser release --snapshot --clean` produces archives for darwin/amd64, darwin/arm64, linux/amd64, linux/arm64 plus RPM and DEB packages in `dist/`

**Checkpoint**: GoReleaser produces all binary artifacts locally. Ready for CI integration.

---

## Phase 3: User Story 5 - Automated Release Pipeline (Priority: P1)

**Goal**: GitHub Actions release workflow triggered on version tags.

**Independent Test**: Push a test tag and verify the workflow runs and produces a GitHub Release.

### Implementation

- [ ] T014 [US5] Create `.github/workflows/release.yaml` with tag trigger (`v*`), Rust toolchain setup, GoReleaser action, and GitHub token permissions
- [ ] T015 [US5] Add `HOMEBREW_TAP_GITHUB_TOKEN` as a required secret in the release workflow (for Homebrew tap push)
- [ ] T016 [US5] Verify: push a test tag `v0.2.1-rc.1` and confirm the release workflow runs and creates a draft GitHub Release

**Checkpoint**: Tag push triggers automated release with all binary artifacts.

---

## Phase 4: User Story 1 - Homebrew Installation (Priority: P1)

**Goal**: macOS users install cc-deck via `brew install cc-deck/tap/cc-deck`.

**Independent Test**: Run `brew install cc-deck/tap/cc-deck` on macOS and verify `cc-deck --version` works.

### Implementation

- [ ] T017 [US1] Create `cc-deck/homebrew-tap` repository on GitHub with initial README
- [ ] T018 [US1] Add brews section to `.goreleaser.yaml` with tap repository, formula name, homepage, description, dependencies (zellij as recommended), and post-install caveats
- [ ] T019 [US1] Verify: after a release, the Homebrew tap repository contains a valid formula and `brew install cc-deck/tap/cc-deck` works

**Checkpoint**: Homebrew installation works end-to-end.

---

## Phase 5: User Story 2 - GitHub Release Downloads (Priority: P1)

**Goal**: Users download pre-built binaries from GitHub Releases.

**Independent Test**: Download archive for current platform, extract, verify binary works.

### Implementation

- [ ] T020 [US2] Update `README.md` with installation section covering Homebrew, binary download, RPM, DEB
- [ ] T021 [P] [US2] Update `docs/modules/ROOT/pages/install.adoc` with all installation methods
- [ ] T022 [US2] Verify: GitHub Release page has archives for all 4 platform/arch combinations plus checksums file

**Checkpoint**: Users can download and install from GitHub Releases.

---

## Phase 6: User Story 6 - Container Images on New Registry (Priority: P1)

**Goal**: Container images available at quay.io/cc-deck with version tags.

**Independent Test**: `podman pull quay.io/cc-deck/cc-deck-demo:latest` works for both arm64 and amd64.

### Implementation

- [ ] T023 [US6] Add container image build and push job to `.github/workflows/release.yaml`: login to quay.io, build base + demo images for arm64/amd64, push multi-arch manifests with version tag and latest
- [ ] T024 [P] [US6] Add `QUAY_USERNAME` and `QUAY_PASSWORD` as GitHub Actions secrets
- [ ] T025 [US6] Verify: after release, `podman pull quay.io/cc-deck/cc-deck-demo:0.3.0` and `podman pull quay.io/cc-deck/cc-deck-demo:latest` both work

**Checkpoint**: Container images available on new registry with version tags.

---

## Phase 7: User Story 3 - Linux Package Manager (Priority: P2)

**Goal**: RPM and DEB packages attached to GitHub Releases.

**Independent Test**: Install RPM on Fedora, DEB on Ubuntu, verify `cc-deck --version`.

### Implementation

- [ ] T026 [US3] Verify RPM package: download from GitHub Release, install on Fedora with `sudo dnf install ./cc-deck-*.rpm`, run `cc-deck --version`
- [ ] T027 [US3] Verify DEB package: download from GitHub Release, install on Ubuntu with `sudo apt install ./cc-deck_*.deb`, run `cc-deck --version`

**Checkpoint**: Linux packages install correctly via native package managers.

---

## Phase 8: User Story 4 - Flatpak (Priority: P2)

**Goal**: Flatpak manifest ready for Flathub submission.

**Independent Test**: Build Flatpak locally with `flatpak-builder`.

### Implementation

- [ ] T028 [P] [US4] Create `flatpak/io.github.cc_deck.cc_deck.yml` Flatpak manifest with binary source from GitHub Release, desktop file, and AppStream metadata
- [ ] T029 [P] [US4] Create `flatpak/cc-deck.desktop` desktop entry file
- [ ] T030 [P] [US4] Create `flatpak/cc-deck.metainfo.xml` AppStream metadata
- [ ] T031 [US4] Verify: `flatpak-builder` can build the Flatpak locally from the manifest

**Checkpoint**: Flatpak manifest ready for Flathub submission.

---

## Phase 9: Polish & Cross-Cutting Concerns

**Purpose**: Documentation, version sync, and final validation

- [ ] T032 Update `README.md` spec table with 021-release-process entry
- [ ] T033 [P] Update landing page at `cc-deck.github.io` with new installation methods (Homebrew, packages) in the steps section
- [ ] T034 [P] Update `docs/modules/ROOT/pages/one-liner.adoc` with `quay.io/cc-deck` image references
- [ ] T035 Run full release dry-run: `goreleaser release --snapshot --clean` and verify all artifacts
- [ ] T036 Document post-release version bump process in `specs/021-release-process/quickstart.md`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies, start immediately
- **Foundational (Phase 2)**: Can start in parallel with Phase 1 (different files)
- **US5 Release Pipeline (Phase 3)**: Depends on Phase 2 (GoReleaser config)
- **US1 Homebrew (Phase 4)**: Depends on Phase 3 (release workflow must exist)
- **US2 GitHub Releases (Phase 5)**: Depends on Phase 3 (release must produce artifacts)
- **US6 Container Images (Phase 6)**: Depends on Phase 1 (registry migration) + Phase 3 (release workflow)
- **US3 Linux Packages (Phase 7)**: Depends on Phase 3 (nFPM already in GoReleaser config)
- **US4 Flatpak (Phase 8)**: Independent of other phases (can start after Phase 1)
- **Polish (Phase 9)**: Depends on all other phases

### Parallel Opportunities

**Phase 1**: T002, T003, T005, T006, T007 can all run in parallel (different files)

**Phase 2**: T010, T011 can run in parallel with T009 completed first

**Phase 4 + Phase 8**: Homebrew and Flatpak can run in parallel (independent)

**Phase 5 + Phase 6**: GitHub Release docs and container image CI can run in parallel

---

## Implementation Strategy

### MVP First (Registry Migration + GoReleaser + Release Pipeline)

1. Complete Phase 1: Registry migration (T001-T008)
2. Complete Phase 2: GoReleaser config (T009-T013)
3. Complete Phase 3: Release workflow (T014-T016)
4. **STOP and VALIDATE**: Push a test tag, verify GitHub Release is created
5. This gives a working automated release with binaries and packages

### Incremental Delivery

1. Registry migration + GoReleaser + Release workflow -> Automated releases (MVP!)
2. Add US1 (Homebrew tap) -> macOS users can `brew install`
3. Add US2 (GitHub Release docs) -> Installation documentation complete
4. Add US6 (Container images) -> Images on new registry
5. Add US3 (Linux packages verification) -> RPM/DEB confirmed working
6. Add US4 (Flatpak manifest) -> Ready for Flathub
7. Polish -> Landing page, docs updates

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story
- Commit after each phase or logical group
- Test the release with a `-rc.1` tag before the actual release
- Container image build uses existing Makefile targets (Podman-based)
- Flatpak submission to Flathub is a manual follow-up after the manifest is ready
