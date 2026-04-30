# Feature Specification: Landing Page Revival

**Feature Branch**: `047-landing-page-revival`
**Created**: 2026-04-30
**Status**: Draft
**Input**: User description: "Replace the Coming Soon placeholder at cc-deck.github.io with a full-featured, developer-focused landing page showcasing cc-deck's complete feature set"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Discover cc-deck Value Proposition (Priority: P1)

A developer searching for Claude Code productivity tools lands on the cc-deck homepage. The page immediately communicates what cc-deck does: a sidebar plugin for managing multiple Claude Code sessions across any environment. The developer sees the core capabilities at a glance, understands the value, and finds a clear path to try it or learn more.

**Why this priority**: The landing page is the project's front door. Without a clear value proposition, potential users leave before exploring further. Every other user story depends on users arriving here first and understanding the product.

**Independent Test**: Visit the landing page URL, verify the hero section loads with title, tagline, description, and call-to-action buttons. Verify all six page sections render correctly.

**Acceptance Scenarios**:

1. **Given** a developer visits cc-deck.github.io, **When** the page loads, **Then** they see the cc-deck title, "Your Claude Code command center" subtitle, a one-sentence description, and two action buttons ("Get Started" and "GitHub").
2. **Given** the landing page, **When** the user scrolls past the hero, **Then** they see a section highlighting 6 sidebar plugin capabilities with icons and descriptions.
3. **Given** the landing page, **When** the user continues scrolling, **Then** they see a "Run Anywhere" section showing 6 workspace environment types.
4. **Given** a mobile device, **When** the user visits the landing page, **Then** the layout adapts to a single-column view with all content accessible.

---

### User Story 2 - Try cc-deck Quickly (Priority: P1)

A developer who understands the value proposition wants to try cc-deck immediately. The landing page provides two quickstart paths in a tabbed interface: a demo container one-liner for instant trial, and local installation instructions for permanent setup.

**Why this priority**: Equally critical as discovery. A clear value proposition without a frictionless path to try the product leads to bookmarks that are never revisited. The quickstart converts interest into action.

**Independent Test**: Navigate to the Get Started section, verify both tabs are functional, and confirm the code blocks display complete, copy-ready commands.

**Acceptance Scenarios**:

1. **Given** the landing page Get Started section, **When** the user clicks the "Try it now" tab, **Then** they see a container one-liner command that can be copied and run immediately.
2. **Given** the landing page Get Started section, **When** the user clicks the "Install locally" tab, **Then** they see installation and plugin setup commands.
3. **Given** the Get Started section with the default tab visible, **When** the user clicks the other tab, **Then** the displayed code block switches without a page reload.
4. **Given** a browser with JavaScript disabled, **When** the user views the Get Started section, **Then** both code blocks are visible in a stacked layout.

---

### User Story 3 - Explore Full Feature Set (Priority: P2)

A developer or team lead evaluating cc-deck for their workflow wants to understand the complete feature set beyond the sidebar plugin. The landing page surfaces secondary capabilities (custom image building, session snapshots, domain filtering, credential profiles) that differentiate cc-deck from simpler session management tools.

**Why this priority**: Secondary features are important for adoption decisions but not for initial discovery. Users who reach this section are already interested and evaluating fit.

**Independent Test**: Scroll to the "More Features" section and verify all four secondary feature cards render with icons and descriptions.

**Acceptance Scenarios**:

1. **Given** the landing page, **When** the user scrolls to the secondary features section, **Then** they see cards for Custom Image Builder, Session Snapshots, Domain Filtering, and Credential Profiles.
2. **Given** any feature card on the page, **When** the user reads the description, **Then** it accurately reflects the current cc-deck capabilities without referencing outdated or unimplemented features.

---

### User Story 4 - Navigate to Detailed Resources (Priority: P2)

A user who wants deeper information can navigate from the landing page to the documentation site or the GitHub repository. All navigation links are visible and functional.

**Why this priority**: Navigation is a supporting capability. It connects the landing page to deeper resources but is not a primary discovery or conversion mechanism.

**Independent Test**: Click each navigation link on the page and verify it reaches the correct destination.

**Acceptance Scenarios**:

1. **Given** the hero section, **When** the user clicks "Get Started", **Then** the page scrolls to the quickstart section.
2. **Given** the hero section, **When** the user clicks "GitHub", **Then** a new tab opens to the cc-deck GitHub repository.
3. **Given** the header navigation, **When** the user clicks "Docs", **Then** they navigate to the documentation site at /docs/.
4. **Given** any page section, **When** the user toggles the dark/light theme, **Then** the entire page renders correctly in the selected mode.

---

### Edge Cases

- What happens when no screenshot/GIF assets exist yet? Styled placeholder boxes appear with muted text indicating content is coming. No broken image tags are rendered.
- What happens when JavaScript is disabled? The tabbed quickstart section degrades to show both code blocks stacked vertically. All other content is server-rendered and unaffected.
- What happens when the page is viewed on an extremely narrow screen (< 320px)? Content remains readable with single-column layout, though some padding may be reduced.
- What happens when a user follows a quickstart command but cc-deck is not yet released? The command references publicly available container images and install methods. If unavailable, the user sees standard "not found" errors from the package manager or container registry.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The landing page MUST replace the current "Coming Soon" placeholder content entirely.
- **FR-002**: The hero section MUST display the project title ("cc-deck"), subtitle ("Your Claude Code command center"), a brief description, a visual placeholder slot for a terminal screenshot or recording, and two call-to-action buttons.
- **FR-003**: The "Get Started" button MUST scroll the page to the quickstart section. The "GitHub" button MUST open the repository in a new tab.
- **FR-004**: The sidebar plugin section MUST present 6 feature cards in a grid layout: Smart Attend, Keyboard Navigation, Session Status, Voice Control, Session Management, and Hook Integration. Each card MUST have an icon, title, and 1-2 sentence description.
- **FR-005**: The "Run Anywhere" section MUST present 6 workspace type cards in a grid layout: Local, Container, Compose, SSH, Kubernetes Deploy, and Kubernetes Sandbox. Each card MUST have an icon, name, and one-line description.
- **FR-006**: The quickstart section MUST offer two paths in a tabbed interface: a demo container one-liner ("Try it now") and local installation instructions ("Install locally"). The first tab MUST be selected by default.
- **FR-007**: The tab switcher MUST use only page-level scripting (no external dependencies). It MUST degrade gracefully when scripting is disabled by showing both code blocks stacked.
- **FR-008**: The secondary features section MUST present 4 feature cards: Custom Image Builder, Session Snapshots, Domain Filtering, and Credential Profiles. Each card MUST have an icon and description.
- **FR-009**: The page MUST support both dark and light display themes using the existing theme toggle, maintaining the deep blue (#1e40af) accent color.
- **FR-010**: The page layout MUST be responsive: single-column on mobile screens, multi-column grids on tablet and desktop screens.
- **FR-011**: Visual placeholder slots MUST render as styled terminal-like boxes with muted text (not broken image indicators) until real assets are available.
- **FR-012**: The page MUST reuse existing site widget components wherever possible. A new component is only permitted for the tabbed quickstart if no existing widget supports tabbed content.
- **FR-013**: The page header and footer navigation MUST remain unchanged (Docs link, GitHub link, license information).
- **FR-014**: No new third-party dependencies MUST be added to the site.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A first-time visitor can identify what cc-deck does (sidebar plugin for Claude Code session management) and how to try it within 30 seconds of landing on the page.
- **SC-002**: All 6 page sections (hero, sidebar features, environments, quickstart, secondary features, footer) render completely without errors in both dark and light modes.
- **SC-003**: The page achieves an accessibility score of 90 or higher when evaluated with standard accessibility auditing tools.
- **SC-004**: The tabbed quickstart interface works correctly: clicking a tab shows the corresponding content and hides the other.
- **SC-005**: All navigation links on the page reach their intended destinations (docs site, GitHub repository, quickstart anchor).
- **SC-006**: The page renders correctly across mobile (< 768px), tablet (768-1024px), and desktop (> 1024px) viewport sizes.
- **SC-007**: Feature descriptions on the page accurately reflect the current cc-deck capabilities with no references to outdated, renamed, or unimplemented features.
- **SC-008**: The page loads within 2 seconds on a standard broadband connection with no heavy media assets.

## Assumptions

- The cc-deck.github.io repository already has a working site build pipeline with dark/light theme support and component library.
- The existing widget components (Hero, Features, CallToAction, Footer) are stable and support the props needed for this page.
- Screenshot and recording assets are not yet available; placeholder slots will be used until assets are created separately.
- The demo container image and local install methods referenced in the quickstart are either already published or will be published before the landing page goes live.
- The Antora documentation site at /docs/ is functional and does not require changes as part of this feature.
- Logo assets are available in the existing site's public directory.
- The page will be deployed automatically via the existing GitHub Pages pipeline when changes are merged.
