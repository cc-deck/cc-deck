# Tasks: cc-deck Base Container Image

**Input**: Design documents from `/specs/017-base-image/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md, quickstart.md

**Tests**: Verification script included (shell-based tool checks, not unit tests).

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: Setup

**Purpose**: Create the base-image directory structure and placeholder files.

- [ ] T001 (cc-mux-x7o.1) Create directory structure: `base-image/`, `base-image/scripts/`, `base-image/config/`
- [ ] T002 (cc-mux-x7o.2) [P] Create `base-image/README.md` with build and usage instructions from quickstart.md

**Checkpoint**: Directory structure ready for implementation.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Shell configuration files that all subsequent tasks depend on.

**⚠️ CRITICAL**: These config files are COPY'd into the image by the Containerfile.

- [ ] T003 (cc-mux-tnx.1) [P] Create `base-image/config/starship.toml` with default prompt config (git branch, directory, python venv, kubernetes context)
- [ ] T004 (cc-mux-tnx.2) [P] Create `base-image/config/zshrc` with starship init, zoxide init, fzf integration, aliases (cat→bat, ls→lsd, ll→lsd -l, la→lsd -a), and PATH for npm global bin

**Checkpoint**: Config files ready to be embedded in the image.

---

## Phase 3: User Story 1 - Build a Project-Specific Claude Code Image (Priority: P1) 🎯 MVP

**Goal**: Create a fully functional Containerfile that produces a Fedora-based developer toolbox image with all required tools, runtimes, non-root user, and shell configuration.

**Independent Test**: Build the image and verify all tools are present by running the verification commands from quickstart.md.

### Implementation for User Story 1

- [ ] T005 (cc-mux-kef.1) [US1] Create `base-image/scripts/install-tools.sh` with dnf install for all packages: git, gh, glab, ripgrep, fd-find, fzf, jq, yq, bat, lsd, git-delta, zoxide, helix, vim-enhanced, nano, curl, wget, htop, nmap-ncat, bind-utils, openssh-clients, make, sudo, tree, less, ca-certificates, nodejs, python3, python3-pip, zsh, uv
- [ ] T006 (cc-mux-kef.2) [US1] Add starship download from GitHub releases to `base-image/scripts/install-tools.sh` with architecture detection (x86_64 vs aarch64, using TARGETARCH build arg)
- [ ] T007 (cc-mux-kef.3) [US1] Create `base-image/scripts/setup-user.sh` to create coder user (UID 1000), set zsh as default shell, configure passwordless sudo, create XDG directories, set npm global prefix to ~/.local/lib/npm, configure git to use delta as pager
- [ ] T008 (cc-mux-kef.4) [US1] Create `base-image/Containerfile` with parameterized Fedora version ARG, COPY scripts, RUN install-tools.sh, RUN setup-user.sh, COPY config files, set USER coder and WORKDIR /home/coder
- [ ] T009 (cc-mux-kef.5) [US1] Build and verify image locally with `podman build -t cc-deck-base:local base-image/` and run verification commands from quickstart.md

**Checkpoint**: Base image builds and all tools are present. User Story 1 is complete.

---

## Phase 4: User Story 2 - Multi-Architecture Support (Priority: P1)

**Goal**: Build and verify the image for both amd64 and arm64 architectures.

**Independent Test**: Build for both architectures and verify tool availability on each.

### Implementation for User Story 2

- [ ] T010 (cc-mux-b6n.1) [US2] Update `base-image/scripts/install-tools.sh` to use TARGETARCH for starship binary download (map amd64→x86_64, arm64→aarch64)
- [ ] T011 (cc-mux-b6n.2) [US2] Build image for arm64 with `podman build --platform linux/arm64 -t cc-deck-base:arm64 base-image/`
- [ ] T012 (cc-mux-b6n.3) [US2] Build image for amd64 with `podman build --platform linux/amd64 -t cc-deck-base:amd64 base-image/`
- [ ] T013 (cc-mux-b6n.4) [US2] Create multi-arch manifest: `podman manifest create cc-deck-base:latest cc-deck-base:amd64 cc-deck-base:arm64`

**Checkpoint**: Multi-arch manifest created, both architectures verified.

---

## Phase 5: User Story 3 - Reproducible Shell Environment (Priority: P2)

**Goal**: Verify the shell configuration works correctly (starship prompt, aliases, delta, fzf, zoxide).

**Independent Test**: Start a container and verify the shell experience matches expectations.

### Implementation for User Story 3

- [ ] T014 (cc-mux-uwk.1) [US3] Verify starship prompt renders correctly in a running container (shows directory and git context)
- [ ] T015 (cc-mux-uwk.2) [US3] Verify aliases work: `ls` shows lsd output, `cat` shows bat output, `git diff` uses delta
- [ ] T016 (cc-mux-uwk.3) [US3] Verify fzf and zoxide are initialized and functional in the zsh shell
- [ ] T017 (cc-mux-uwk.4) [US3] Verify npm global install works without root: `npm install -g cowsay && cowsay hello`

**Checkpoint**: Shell environment is fully configured and verified.

---

## Phase 6: User Story 4 - Base Image Maintenance (Priority: P3)

**Goal**: Set up CI pipeline for automated multi-arch builds and registry publishing.

**Independent Test**: Trigger the CI pipeline and verify images are pushed to ghcr.io.

### Implementation for User Story 4

- [ ] T018 (cc-mux-jgv.1) [US4] Create `.github/workflows/base-image.yml` with manual trigger, multi-arch build (amd64 + arm64), ghcr.io push, and tagging (latest, vX.Y.Z, fedora-NN)
- [ ] T019 (cc-mux-jgv.2) [US4] Add vulnerability scan step to CI workflow (informational, non-blocking) using `podman scan` or `trivy`
- [ ] T020 (cc-mux-jgv.3) [US4] Push initial image to `ghcr.io/rhuss/cc-deck-base:latest` via CI or manual push

**Checkpoint**: CI pipeline builds, scans, and pushes multi-arch images to ghcr.io.

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Documentation and image size optimization.

- [ ] T021 (cc-mux-65c.1) [P] Verify image size is under 1.5 GB compressed (`podman image inspect --format '{{.Size}}'`)
- [ ] T022 (cc-mux-65c.2) [P] Optimize Containerfile layer ordering for cache efficiency (system packages first, config last)
- [ ] T023 (cc-mux-65c.3) Update `base-image/README.md` with final build instructions, image size, and tool inventory

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies, start immediately
- **Foundational (Phase 2)**: No dependencies, can run in parallel with Phase 1
- **User Story 1 (Phase 3)**: Depends on Phase 1 + Phase 2 completion
- **User Story 2 (Phase 4)**: Depends on User Story 1 (needs working Containerfile)
- **User Story 3 (Phase 5)**: Depends on User Story 1 (needs working image)
- **User Story 4 (Phase 6)**: Depends on User Story 2 (needs multi-arch build working)
- **Polish (Phase 7)**: Depends on all user stories

### User Story Dependencies

- **User Story 1 (P1)**: Blocked by Setup + Foundational only
- **User Story 2 (P1)**: Depends on US1 (Containerfile must exist)
- **User Story 3 (P2)**: Depends on US1 (image must build)
- **User Story 4 (P3)**: Depends on US2 (multi-arch build must work)

### Parallel Opportunities

- T001 and T002 can run in parallel (different files)
- T003 and T004 can run in parallel (different config files)
- T014, T015, T016, T017 can run in parallel (independent verifications)
- T021, T022 can run in parallel (independent checks)

---

## Parallel Example: User Story 1

```bash
# Phase 2 config files can be created in parallel:
Task: "Create base-image/config/starship.toml"
Task: "Create base-image/config/zshrc"

# Phase 3 scripts can be created in parallel:
Task: "Create base-image/scripts/install-tools.sh"
Task: "Create base-image/scripts/setup-user.sh"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (directory structure)
2. Complete Phase 2: Foundational (config files)
3. Complete Phase 3: User Story 1 (Containerfile + scripts)
4. **STOP and VALIDATE**: Build image, run verification
5. Working base image available for local use

### Incremental Delivery

1. Setup + Foundational + US1 → Working local image (MVP!)
2. Add US2 → Multi-arch support
3. Add US3 → Shell environment verification
4. Add US4 → CI pipeline + registry publishing
5. Polish → Size optimization + docs

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Containerfile uses `ARG FEDORA_VERSION=41` for easy version bumps
- starship is the only tool not available via dnf (downloaded from GitHub releases)
- All other tools installed via single `dnf install` command for minimal layers
- Commit after each task or logical group


