# Tasks: Landing Page Revival

**Input**: Design documents from `specs/047-landing-page-revival/`
**Prerequisites**: plan.md (required), spec.md (required), research.md

**Tests**: No test tasks included (static site, verified by build + visual inspection).

**Organization**: Tasks grouped by user story for independent implementation and testing.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Path Conventions

All paths relative to `../cc-deck.github.io/` (the landing page repository).

---

## Phase 1: Setup

**Purpose**: Fix site configuration issues

- [ ] T001 Fix site URL in astro.config.ts from `antwort-dev.github.io` to `cc-deck.github.io` in ../cc-deck.github.io/astro.config.ts

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Create the tabbed code component needed by User Story 2

- [ ] T002 Create TabbedCode widget component in ../cc-deck.github.io/src/components/widgets/TabbedCode.astro with vanilla JS tab switcher, monospace code blocks, dark terminal styling, and graceful JS-disabled degradation (both blocks stacked)

**Checkpoint**: Foundation ready, all widget components available for page assembly.

---

## Phase 3: User Story 1 - Discover cc-deck Value Proposition (Priority: P1)

**Goal**: Replace the placeholder with a landing page that communicates what cc-deck does and showcases the sidebar plugin and workspace types.

**Independent Test**: Visit the page, verify hero section loads with title/subtitle/CTAs, sidebar plugin features grid (6 cards), and Run Anywhere grid (6 cards) all render correctly.

### Implementation for User Story 1

- [ ] T003 [US1] Replace placeholder content in ../cc-deck.github.io/src/pages/index.astro with Hero widget section: title "cc-deck", subtitle "Your Claude Code command center", description text, placeholder image div (styled terminal box), and two CTA buttons (Get Started anchor to #get-started, GitHub external link)
- [ ] T004 [US1] Add Sidebar Plugin Features section to ../cc-deck.github.io/src/pages/index.astro using Features widget with columns=3, tagline "Sidebar Plugin", title "Everything visible. One keystroke away.", and 6 items: Smart Attend (tabler:focus-2), Keyboard Navigation (tabler:keyboard), Session Status (tabler:activity), Voice Control (tabler:microphone), Session Management (tabler:layout-list), Hook Integration (tabler:webhook). Use descriptions from plan.md Feature Descriptions table.
- [ ] T005 [US1] Add Run Anywhere section to ../cc-deck.github.io/src/pages/index.astro using Features widget with columns=3, tagline "Workspaces", title "Run anywhere", and 6 items: Local (tabler:device-desktop), Container (tabler:box), Compose (tabler:stack-2), SSH (tabler:terminal), Kubernetes Deploy (tabler:cloud), Kubernetes Sandbox (tabler:flask). Use descriptions from plan.md Feature Descriptions table.

**Checkpoint**: Page shows hero, sidebar features, and workspace types. Core value proposition is visible.

---

## Phase 4: User Story 2 - Try cc-deck Quickly (Priority: P1)

**Goal**: Add tabbed quickstart section with demo container and local install paths.

**Independent Test**: Navigate to Get Started section, verify both tabs work (click toggles content), code blocks display complete commands.

### Implementation for User Story 2

- [ ] T006 [US2] Add Get Started section to ../cc-deck.github.io/src/pages/index.astro using TabbedCode widget with id="get-started", Tab 1 "Try it now" (podman run one-liner with demo image), Tab 2 "Install locally" (brew install cc-deck + cc-deck config plugin install)

**Checkpoint**: Visitors can find and copy quickstart commands.

---

## Phase 5: User Story 3 - Explore Full Feature Set (Priority: P2)

**Goal**: Add secondary features section showcasing image builder, snapshots, domain filtering, and credential profiles.

**Independent Test**: Scroll to More Features section, verify 4 feature cards render with icons and accurate descriptions.

### Implementation for User Story 3

- [ ] T007 [US3] Add More Features section to ../cc-deck.github.io/src/pages/index.astro using Features widget with columns=2, tagline "And more", title "Built for real workflows", and 4 items: Custom Image Builder (tabler:hammer), Session Snapshots (tabler:camera), Domain Filtering (tabler:filter), Credential Profiles (tabler:key). Use descriptions from plan.md Feature Descriptions table.

**Checkpoint**: Full feature set visible on the page.

---

## Phase 6: User Story 4 - Navigate to Detailed Resources (Priority: P2)

**Goal**: Verify all navigation links work correctly (docs, GitHub, anchor scroll, theme toggle).

**Independent Test**: Click each link, verify correct destination. Toggle dark/light mode, verify rendering.

### Implementation for User Story 4

- [ ] T008 [US4] Verify and fix navigation in ../cc-deck.github.io/src/navigation.ts if needed. Ensure header links (Docs, GitHub) and footer links are correct. Verify Get Started anchor (#get-started) scrolls correctly from hero CTA.

**Checkpoint**: All navigation paths functional.

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Verification and cleanup

- [ ] T009 Run `npm run build` in ../cc-deck.github.io/ to verify site builds without errors
- [ ] T010 Verify dark/light theme toggle renders all sections correctly in both modes
- [ ] T011 Verify responsive layout at mobile (<768px), tablet (768-1024px), and desktop (>1024px) viewports
- [ ] T012 Start dev server (`npm run dev`) and manually verify all 6 page sections render, tab switcher works, and all links navigate correctly

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies, start immediately
- **Foundational (Phase 2)**: No dependency on Phase 1 (different files), can run in parallel with T001
- **User Story 1 (Phase 3)**: Depends on Phase 1 (needs correct config)
- **User Story 2 (Phase 4)**: Depends on Phase 2 (needs TabbedCode component) and Phase 3 (index.astro structure)
- **User Story 3 (Phase 5)**: Depends on Phase 3 (index.astro structure)
- **User Story 4 (Phase 6)**: Depends on Phases 3-5 (all page sections present)
- **Polish (Phase 7)**: Depends on all user stories complete

### Within Each User Story

- Tasks within a story are sequential (all modify the same file: index.astro)
- US3 (Phase 5) can run in parallel with US2 (Phase 4) since they add independent sections

### Parallel Opportunities

```text
Parallel group 1: T001 (config fix) || T002 (TabbedCode component)
Sequential: T003 → T004 → T005 (all modify index.astro in sequence)
Sequential: T006 (adds to index.astro after T005)
Parallel after T005: T006 (US2) || T007 (US3) if editing different sections
Sequential: T008 → T009 → T010 → T011 → T012 (verification)
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Fix config (T001)
2. Complete Phase 3: Hero + Sidebar + Workspaces (T003-T005)
3. **STOP and VALIDATE**: Page shows value proposition
4. Already a massive improvement over "Coming Soon"

### Full Delivery

1. T001 + T002 (parallel): Config fix + TabbedCode component
2. T003 → T004 → T005: Build page sections (hero, sidebar, workspaces)
3. T006 + T007 (parallel if careful): Quickstart tabs + More Features
4. T008: Navigation verification
5. T009 → T012: Polish and verification

---

## Notes

- All implementation tasks modify files in `../cc-deck.github.io/` (sibling repository)
- Tasks T003-T007 all modify the same file (index.astro), so true parallelism is limited
- Feature descriptions are defined in plan.md Feature Descriptions tables
- Tabler icon names are defined in research.md Icon Selection tables
- Commit after each task or logical group
