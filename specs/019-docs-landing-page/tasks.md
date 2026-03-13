# Tasks: cc-deck Documentation & Landing Page

**Input**: Design documents from `/specs/019-docs-landing-page/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md, quickstart.md

**Tests**: No automated tests requested. Verification via Lighthouse, Antora build, and manual checks.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: Setup

**Purpose**: Create both repositories and shared infrastructure.

- [x] T001 Create `cc-deck.github.io` repository on GitHub with GitHub Pages enabled
- [x] T002 Clone antwort.github.io as template for `cc-deck.github.io/`, strip antwort-specific content
- [x] T003 [P] Create Antora docs skeleton in `docs/antora.yml` and `docs/modules/` with all 8 module directories (ROOT, quickstarts, plugin, images, podman, kubernetes, reference, developer)
- [ ] T004 [P] Create `demo-image/Containerfile` for the pre-built demo image

**Checkpoint**: Both repos exist. Antora skeleton has all 8 module directories.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Landing page infrastructure and Antora build pipeline.

**Note**: The landing page and Antora pipeline must work before any content can be published.

- [x] T005 Configure `cc-deck.github.io/astro.config.ts` with site URL, output dir, and Antora docs integration
- [x] T006 Create `cc-deck.github.io/src/components/CustomStyles.astro` with deep blue color scheme (primary #1e40af, dark/light mode variables)
- [x] T007 [P] Create `cc-deck.github.io/antora-playbook.yml` pulling from `rhuss/cc-deck.git` main branch, `docs/` start path
- [x] T008 [P] Copy logo assets to `cc-deck.github.io/public/` (favicon from `assets/logo/cc-deck-icon.png`, wordmark for header)
- [ ] T009 [P] Create `cc-deck.github.io/supplemental-ui/` with logo header partial and CSS color overrides for Antora
- [ ] T010 Create GitHub Actions workflow in `cc-deck.github.io/.github/workflows/deploy.yml` to build Astro + Antora and deploy to Pages
- [ ] T011 Verify: landing page builds locally (`npm run dev`), Antora builds (`npm run build`), both deploy to GitHub Pages

**Checkpoint**: Landing page renders at cc-deck.github.io. Antora docs build at /docs/.

---

## Phase 3: User Story 1 - Landing Page (Priority: P1)

**Goal**: A landing page that communicates cc-deck's value proposition with hero, features, steps, and CTAs.

**Independent Test**: Visit cc-deck.github.io, verify hero, features grid, steps, CTA, dark/light toggle, responsive layout.

### Implementation for User Story 1

- [x] T012 [US1] Create `cc-deck.github.io/src/pages/index.astro` with Hero section (logo, tagline "Your Claude Code command center", subtitle, Get Started + GitHub buttons)
- [x] T013 [US1] Add Features widget to `index.astro` with 4 USP cards: Zellij sidebar plugin, custom container images, multi-platform support, session management
- [x] T014 [US1] Add Steps widget to `index.astro` showing getting started flow (Install, Launch, Build Image, Deploy)
- [x] T015 [US1] Add CallToAction widget at bottom of `index.astro` with links to quickstart docs and GitHub
- [x] T016 [US1] Implement dark/light theme toggle in `cc-deck.github.io/src/components/` (matching antwort pattern)
- [ ] T017 [US1] Verify: Lighthouse score 90+ for performance and accessibility, responsive on mobile/tablet/desktop

**Checkpoint**: Landing page complete with all sections, themes, and responsive design.

---

## Phase 4: User Story 2 - Quickstart (Priority: P1)

**Goal**: One-liner quickstart using the demo image, with both API key and Vertex AI auth options.

**Independent Test**: Follow the quickstart on a machine with podman, get a working Zellij + cc-deck session.

### Implementation for User Story 2

- [ ] T018 [US2] Build `demo-image/Containerfile` using cc-deck-base + cc-deck + Zellij + Claude Code (private Node.js 20 pattern)
- [ ] T019 [US2] Add `make demo-image` target in `Makefile` (depends on cross-cli, single platform for testing)
- [ ] T020 [US2] Write `docs/modules/quickstarts/pages/one-liner.adoc` with API key and Vertex AI quickstart commands
- [ ] T021 [US2] Write `docs/modules/quickstarts/pages/install.adoc` covering native installation (make build, make install)
- [ ] T022 [P] [US2] Write `docs/modules/quickstarts/pages/first-session.adoc` covering first Zellij + Claude Code session
- [ ] T023 [US2] Create `docs/modules/quickstarts/nav.adoc` with page ordering
- [ ] T024 [US2] Verify: run the one-liner quickstart, confirm working Zellij session with cc-deck sidebar

**Checkpoint**: Demo image builds. Quickstart docs guide users to a working session in under 5 minutes.

---

## Phase 5: User Story 3 - Plugin Documentation (Priority: P1)

**Goal**: Complete documentation of the Zellij sidebar plugin features.

**Independent Test**: Navigate plugin docs, verify all keybindings and features are documented.

### Implementation for User Story 3

- [x] T025 [P] [US3] Write `docs/modules/plugin/pages/overview.adoc` introducing the sidebar plugin concept
- [x] T026 [P] [US3] Write `docs/modules/plugin/pages/navigation.adoc` with all keybindings (Alt+s, j/k, Enter, Esc, g/G, r, d, p, n, /, ?)
- [x] T027 [P] [US3] Write `docs/modules/plugin/pages/attend.adoc` documenting smart attend algorithm and priority tiers
- [x] T028 [P] [US3] Write `docs/modules/plugin/pages/sessions.adoc` covering pause, rename, search, delete, new tab
- [x] T029 [P] [US3] Write `docs/modules/plugin/pages/configuration.adoc` covering layout variants, personal layout, key config, Ghostty integration
- [x] T030 [US3] Create `docs/modules/plugin/nav.adoc` with page ordering
- [ ] T031 [US3] Add screenshots or diagrams of the sidebar in action to `docs/modules/plugin/images/`

**Checkpoint**: Plugin documentation covers all features with keybindings and visuals.

---

## Phase 6: User Story 4 - Image Pipeline Documentation (Priority: P2)

**Goal**: Document the full container image build pipeline.

**Independent Test**: Follow the docs to build a custom container image from scratch.

### Implementation for User Story 4

- [x] T032 [P] [US4] Write `docs/modules/images/pages/overview.adoc` explaining the CLI-AI-CLI build pipeline concept
- [x] T033 [P] [US4] Write `docs/modules/images/pages/init.adoc` documenting `cc-deck image init`
- [x] T034 [P] [US4] Write `docs/modules/images/pages/extract.adoc` documenting `/cc-deck.extract` command
- [x] T035 [P] [US4] Write `docs/modules/images/pages/settings.adoc` documenting `/cc-deck.settings` command (all 5 sections)
- [x] T036 [P] [US4] Write `docs/modules/images/pages/build.adoc` documenting `/cc-deck.build` command (self-correction, multi-arch, Node.js 20)
- [x] T037 [P] [US4] Write `docs/modules/images/pages/manifest.adoc` with full `cc-deck-build.yaml` schema reference
- [x] T038 [US4] Create `docs/modules/images/nav.adoc` with page ordering

**Checkpoint**: Image pipeline fully documented with manifest schema reference.

---

## Phase 7: User Story 5 - Podman Reference (Priority: P2)

**Goal**: Complete Podman local deployment reference.

**Independent Test**: Follow docs to set up a persistent container with mounted source code.

### Implementation for User Story 5

- [x] T039 [P] [US5] Write `docs/modules/podman/pages/quickstart.adoc` with minimal local setup
- [x] T040 [P] [US5] Write `docs/modules/podman/pages/volumes.adoc` covering source code mounts, persistent state, bidirectional editing
- [x] T041 [P] [US5] Write `docs/modules/podman/pages/credentials.adoc` covering API key and Vertex AI (gcloud mount) setup
- [x] T042 [P] [US5] Write `docs/modules/podman/pages/advanced.adoc` covering GPU passthrough, MCP port forwarding, networking, resource limits
- [x] T043 [US5] Create `docs/modules/podman/nav.adoc` with page ordering

**Checkpoint**: Podman reference covers all deployment scenarios.

---

## Phase 8: User Story 6 - Kubernetes Reference (Priority: P2)

**Goal**: Complete Kubernetes deployment reference.

**Independent Test**: Follow docs to deploy a cc-deck StatefulSet with PVC and credentials.

### Implementation for User Story 6

- [x] T044 [P] [US6] Write `docs/modules/kubernetes/pages/quickstart.adoc` with minimal K8s deployment
- [x] T045 [P] [US6] Write `docs/modules/kubernetes/pages/statefulset.adoc` covering StatefulSet pattern with PVCs
- [x] T046 [P] [US6] Write `docs/modules/kubernetes/pages/credentials.adoc` covering Secrets, service accounts, Vertex AI
- [x] T047 [P] [US6] Write `docs/modules/kubernetes/pages/rbac.adoc` covering RBAC configuration
- [x] T048 [P] [US6] Write `docs/modules/kubernetes/pages/scaling.adoc` covering multi-session scaling, resource management
- [x] T049 [US6] Create `docs/modules/kubernetes/nav.adoc` with page ordering

**Checkpoint**: Kubernetes reference covers deployment, persistence, credentials, RBAC, scaling.

---

## Phase 9: User Story 7 - Developer Documentation (Priority: P3)

**Goal**: Architecture overview and contributor guide.

**Independent Test**: Follow build instructions to compile cc-deck from source.

### Implementation for User Story 7

- [x] T050 [P] [US7] Write `docs/modules/developer/pages/architecture.adoc` covering two-component design (Rust WASM + Go CLI), sync protocol, hook integration
- [x] T051 [P] [US7] Write `docs/modules/developer/pages/building.adoc` covering build from source (make build, prerequisites, cross-compile)
- [x] T052 [P] [US7] Write `docs/modules/developer/pages/contributing.adoc` covering PR guidelines, testing, code style
- [x] T053 [US7] Create `docs/modules/developer/nav.adoc` with page ordering

**Checkpoint**: Developer documentation enables contributors to build and submit changes.

---

## Phase 10: Polish & Cross-Cutting Concerns

**Purpose**: ROOT module, reference docs, and final validation.

- [x] T054 [P] Write `docs/modules/ROOT/pages/index.adoc` with project overview, feature highlights, and links to all modules
- [ ] T055 [P] Write `docs/modules/reference/pages/cli.adoc` with all CLI commands, flags, and examples
- [ ] T056 [P] Write `docs/modules/reference/pages/manifest-schema.adoc` with full cc-deck-build.yaml schema
- [ ] T057 [P] Write `docs/modules/reference/pages/configuration.adoc` covering config file, env vars, XDG paths
- [ ] T058 [P] Write `docs/modules/reference/pages/mcp-labels.adoc` covering MCP label schema for container images
- [ ] T059 Create `docs/modules/ROOT/nav.adoc` and `docs/modules/reference/nav.adoc`
- [ ] T060 Verify: Antora build completes without warnings or broken cross-references
- [ ] T061 Verify: all 8 modules have substantive content, navigation works end-to-end
- [ ] T062 Push demo image to `quay.io/rhuss/cc-deck-demo:latest`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies, start immediately
- **Foundational (Phase 2)**: Depends on Phase 1 (needs repos and skeleton)
- **US1 Landing Page (Phase 3)**: Depends on Phase 2 (needs site infrastructure)
- **US2 Quickstart (Phase 4)**: Depends on Phase 2 (needs Antora) + demo image build
- **US3 Plugin Docs (Phase 5)**: Depends on Phase 2 only (Antora skeleton)
- **US4 Image Docs (Phase 6)**: Depends on Phase 2 only
- **US5 Podman Docs (Phase 7)**: Depends on Phase 2 only
- **US6 K8s Docs (Phase 8)**: Depends on Phase 2 only
- **US7 Developer Docs (Phase 9)**: Depends on Phase 2 only
- **Polish (Phase 10)**: Depends on all content phases

### User Story Dependencies

- **US1 (P1)**: Blocked by Foundational only
- **US2 (P1)**: Blocked by Foundational + demo image (T018-T019)
- **US3 (P1)**: Blocked by Foundational only (independent of US1, US2)
- **US4 (P2)**: Blocked by Foundational only
- **US5 (P2)**: Blocked by Foundational only
- **US6 (P2)**: Blocked by Foundational only
- **US7 (P3)**: Blocked by Foundational only

### Parallel Opportunities

- US3, US4, US5, US6, US7 can ALL run in parallel after Phase 2
- Within each doc module, all page writes are parallel ([P])
- T003 and T004 can run in parallel (different repos/directories)
- T007, T008, T009 can run in parallel (different files)

---

## Implementation Strategy

### MVP First (User Stories 1-2)

1. Complete Phases 1-2: Setup + Foundational
2. Complete Phase 3: US1 (Landing Page)
3. Complete Phase 4: US2 (Quickstart + Demo Image)
4. **STOP and VALIDATE**: Landing page live, quickstart works end-to-end
5. This gives a working public-facing site with a try-it-now experience

### Incremental Delivery

1. Setup + Foundational + US1 + US2 -> Live site with quickstart (MVP)
2. Add US3 -> Plugin documentation
3. Add US4 -> Image pipeline documentation
4. Add US5 + US6 -> Podman and K8s references (can be parallel)
5. Add US7 -> Developer documentation
6. Polish -> ROOT overview, reference docs, final validation

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- All doc pages are AsciiDoc (.adoc) following Antora conventions
- One sentence per line in AsciiDoc files (per CLAUDE.md style rules)
- Commit after each module or logical group
- Screenshots for the plugin module should be captured from a real cc-deck session


<!-- SDD-TRAIT:beads -->
## Beads Task Management

This project uses beads (`bd`) for persistent task tracking across sessions:
- Run `/sdd:beads-task-sync` to create bd issues from this file
- `bd ready --json` returns unblocked tasks (dependencies resolved)
- `bd close <id>` marks a task complete (use `-r "reason"` for close reason, NOT `--comment`)
- `bd comments add <id> "text"` adds a detailed comment to an issue
- `bd sync` persists state to git
- `bd create "DISCOVERED: [short title]" --labels discovered` tracks new work
  - Keep titles crisp (under 80 chars); add details via `bd comments add <id> "details"`
- Run `/sdd:beads-task-sync --reverse` to update checkboxes from bd state
- **Always use `jq` to parse bd JSON output, NEVER inline Python one-liners**
