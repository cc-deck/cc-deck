# Feature Specification: cc-deck Documentation & Landing Page

**Feature Branch**: `019-docs-landing-page`
**Created**: 2026-03-13
**Status**: Evolved (2026-03-26)
**Input**: User description: "Documentation and landing page with Antora docs and Astro site"

> **Evolution Note (2026-03-26)**: Antora documentation modules implemented.
> Landing page is minimal ("Coming Soon" placeholder), not the full-featured
> page described in the spec. Astro config site URL needs correction
> (currently `antwort-dev.github.io`). Demo image registry updated from
> `quay.io/rhuss` to `quay.io/cc-deck`. AI command names updated:
> `/cc-deck.extract` is now `/cc-deck.capture`,
> `/cc-deck.settings` merged into `/cc-deck.capture`.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Discover cc-deck via Landing Page (Priority: P1)

A developer searching for Claude Code session management finds the cc-deck landing page.
The page clearly communicates what cc-deck does (Zellij sidebar plugin, custom container
images, multi-platform support), shows a quick demo, and provides a path to get started.
The developer can navigate to documentation, GitHub, or a one-liner quickstart.

**Why this priority**: Without a discoverable landing page, the project has no entry point
for new users. This is the top of the funnel.

**Independent Test**: Visit the landing page URL, verify it loads, shows the hero with
value proposition, features grid, getting started steps, and links to docs and GitHub.

**Acceptance Scenarios**:

1. **Given** a developer visits the landing page, **When** the page loads, **Then** they see
   the cc-deck logo, tagline ("Your Claude Code command center"), feature highlights, and
   call-to-action buttons for "Get Started" and "View on GitHub".
2. **Given** the landing page, **When** the user clicks "Get Started", **Then** they are
   taken to the quickstart documentation.
3. **Given** the landing page, **When** the user toggles dark/light mode, **Then** the page
   renders correctly in both themes with the deep blue color scheme.
4. **Given** a mobile device, **When** the user visits the landing page, **Then** the layout
   is responsive and all content is accessible.

---

### User Story 2 - Follow the Quickstart (Priority: P1)

A developer wants to try cc-deck immediately. The quickstart page provides a one-liner
command using a pre-built demo image that includes everything (Zellij, cc-deck plugin,
Claude Code). The developer runs the command, provides their API key or Vertex AI
credentials, and has a working cc-deck session in under 2 minutes.

**Why this priority**: A frictionless first experience is critical for adoption. If the
quickstart doesn't work in minutes, users leave.

**Independent Test**: Follow the quickstart instructions on a machine with podman installed,
verify a working Zellij session with cc-deck sidebar appears.

**Acceptance Scenarios**:

1. **Given** a machine with podman installed, **When** the user runs the one-liner quickstart
   with an API key, **Then** a container starts and they can connect with
   `podman exec -it cc-demo zellij --layout cc-deck`.
2. **Given** Vertex AI credentials, **When** the user follows the Vertex quickstart variant,
   **Then** Claude Code authenticates via the mounted gcloud credentials.
3. **Given** the quickstart page, **Then** it shows both API key and Vertex AI authentication
   options with complete commands.

---

### User Story 3 - Read Plugin Documentation (Priority: P1)

A user who has installed cc-deck wants to learn about the Zellij sidebar plugin features:
keyboard navigation, smart attend, pause, search, rename, session management. The plugin
documentation explains each feature with keybindings and usage examples.

**Why this priority**: The sidebar plugin is the core product. Users need to understand
its capabilities to use cc-deck effectively.

**Independent Test**: Navigate to the plugin documentation, verify all features are
documented with keybindings, screenshots, and usage scenarios.

**Acceptance Scenarios**:

1. **Given** the plugin documentation, **When** the user reads the navigation section,
   **Then** they find all keybindings (Alt+s, j/k, Enter, Esc, g/G, etc.) documented.
2. **Given** the plugin documentation, **When** the user reads the smart attend section,
   **Then** they understand the priority tiers and round-robin behavior.
3. **Given** the plugin documentation, **Then** it includes screenshots or diagrams showing
   the sidebar in action.

---

### User Story 4 - Build a Custom Container Image (Priority: P2)

A team lead wants to create a standardized Claude Code container image for their team
with specific tools, plugins, and MCP servers. The image documentation walks through
the full pipeline: init, extract, settings, build, push.

**Why this priority**: Custom images are a key differentiator but require the plugin
to be understood first.

**Independent Test**: Follow the image documentation to build a container image from
scratch using the cc-deck CLI and AI commands.

**Acceptance Scenarios**:

1. **Given** the images documentation, **When** the user follows the pipeline, **Then**
   they can build a working container image with their tools and settings.
2. **Given** the images documentation, **Then** it explains each AI command
   (/cc-deck.capture, /cc-deck.build, /cc-deck.push).
3. **Given** the images documentation, **Then** it includes the manifest schema reference
   with all sections and examples.

---

### User Story 5 - Deploy on Podman Locally (Priority: P2)

A developer wants to run cc-deck in a local container with access to their source code
and credentials. The Podman documentation provides a complete reference for volume mounts,
credential passthrough (API key and Vertex AI), persistent containers, GPU access, and
port forwarding for MCP servers.

**Why this priority**: Local Podman is the most common deployment target after native
installation.

**Independent Test**: Follow the Podman documentation to set up a persistent container
with mounted source code and working Claude Code authentication.

**Acceptance Scenarios**:

1. **Given** the Podman documentation, **When** the user follows the volume mount section,
   **Then** they can work on local source files from inside the container.
2. **Given** the Podman documentation, **Then** it covers API key and Vertex AI credential
   setup with complete commands.
3. **Given** the Podman documentation, **Then** it explains persistent containers
   (`sleep infinity`), reconnecting via `podman exec`, and Zellij session persistence.

---

### User Story 6 - Deploy on Kubernetes (Priority: P2)

A platform engineer wants to deploy cc-deck sessions on Kubernetes or OpenShift for
their team. The Kubernetes documentation covers StatefulSet deployment, PersistentVolume
setup for source code and state, credential injection via Secrets, RBAC, and scaling.

**Why this priority**: Kubernetes is the enterprise deployment target, important for
teams but requires more setup.

**Independent Test**: Follow the Kubernetes documentation to deploy a cc-deck StatefulSet
with persistent storage and working Claude Code authentication.

**Acceptance Scenarios**:

1. **Given** the Kubernetes documentation, **When** the user applies the example manifests,
   **Then** a cc-deck StatefulSet is created with PVC for persistent state.
2. **Given** the Kubernetes documentation, **Then** it covers credential injection via
   Secrets for both API key and Vertex AI service accounts.
3. **Given** the Kubernetes documentation, **Then** it includes connection instructions
   (port-forward, direct exec, or web terminal).

---

### User Story 7 - Explore Architecture and Contribute (Priority: P3)

A developer interested in contributing to cc-deck wants to understand the architecture
(Rust WASM plugin, Go CLI, two-component design), build from source, and submit changes.

**Why this priority**: Developer documentation is important for the open source community
but not for end users.

**Independent Test**: Follow the developer documentation to build cc-deck from source
and run the test suite.

**Acceptance Scenarios**:

1. **Given** the developer documentation, **When** the user follows the build instructions,
   **Then** they can build both the WASM plugin and Go CLI from source.
2. **Given** the architecture documentation, **Then** it explains the two-component design,
   sync protocol, and hook integration.

---

### Edge Cases

- What happens when the Antora docs reference a page that doesn't exist?
  Antora reports broken cross-references at build time. The build fails until fixed.
- What happens when the landing page is accessed without JavaScript?
  Core content (text, links) is still visible via server-rendered HTML.
- What happens when documentation links point to a branch that hasn't been merged?
  The Antora playbook pulls from `main` branch only, ensuring published docs are stable.

## Requirements *(mandatory)*

### Functional Requirements

#### Landing Page

- **FR-001**: The landing page MUST be hosted at `cc-deck.github.io` via GitHub Pages.
- **FR-002**: The landing page MUST include: hero section with logo and tagline, features
  grid highlighting USPs, getting started steps, and call-to-action links.
- **FR-003**: The landing page MUST support dark and light themes with a toggle control.
- **FR-004**: The landing page MUST use the deep blue (#1e40af) color scheme with the
  cc-deck logo (sidebar icon with orange asterisk).
- **FR-005**: The landing page MUST link to the documentation site and GitHub repository.
- **FR-006**: The landing page MUST be responsive for mobile, tablet, and desktop.

#### Documentation Site

- **FR-007**: Documentation MUST be authored in AsciiDoc using Antora for multi-module
  organization.
- **FR-008**: Documentation source MUST live in the main `cc-deck` repository under `docs/`.
- **FR-009**: The Antora playbook MUST reside in the `cc-deck.github.io` repository and
  pull content from the main repo's `main` branch.
- **FR-010**: Documentation MUST include these modules: ROOT (overview), quickstarts,
  plugin, images, podman, kubernetes, reference, developer.
- **FR-011**: The quickstart module MUST include a one-liner demo using a pre-built
  `cc-deck-demo` image with both API key and Vertex AI authentication options.
- **FR-012**: The plugin module MUST document all keyboard shortcuts, smart attend behavior,
  pause/resume, search, rename, and session management.
- **FR-013**: The images module MUST document the full build pipeline (init, capture,
  build, push) with manifest schema reference.
- **FR-014**: The podman module MUST be a complete reference covering volume mounts for
  local source directories, credential passthrough (API key and Vertex AI), persistent
  containers, GPU access, and port forwarding.
- **FR-015**: The kubernetes module MUST be a complete reference covering StatefulSet
  deployment, PersistentVolume setup, credential injection via Secrets, RBAC, and scaling.
- **FR-016**: The reference module MUST include CLI command reference, manifest schema,
  configuration options, and MCP label schema.

#### Demo Image

- **FR-017**: A pre-built `cc-deck-demo` image MUST be published to quay.io, containing
  the base image plus cc-deck, Zellij, and Claude Code pre-installed.
- **FR-018**: The demo image MUST work with a single environment variable for authentication
  (ANTHROPIC_API_KEY or Vertex AI variables).

### Key Entities

- **Landing Page**: Static site at `cc-deck.github.io` with hero, features, steps, and
  navigation to docs.
- **Documentation Site**: Antora-generated multi-module AsciiDoc site served under
  `cc-deck.github.io/docs/`.
- **Demo Image**: Pre-built container image at `quay.io/cc-deck/cc-deck-demo:latest` for
  quickstart workflows.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A new user can go from the landing page to a running cc-deck session in
  under 5 minutes using the quickstart one-liner.
- **SC-002**: All 8 documentation modules have at least one page with substantive content.
- **SC-003**: The landing page scores 90+ on Lighthouse for performance and accessibility.
- **SC-004**: Documentation covers 100% of CLI commands and manifest schema fields.
- **SC-005**: The Antora build completes without broken cross-references or warnings.
- **SC-006**: Both dark and light themes render correctly on the landing page across
  Chrome, Firefox, and Safari.
- **SC-007**: The demo image can be pulled and started with a working Zellij + cc-deck
  session using only environment variables for auth.

## Assumptions

- The `cc-deck.github.io` repository will be created under the `cc-deck` GitHub organization.
- GitHub Pages is used for hosting (free, automatic deployment).
- The landing page follows the same Astro + Tailwind structure as `antwort.github.io`.
- The Antora UI bundle is customized with cc-deck branding (logo, colors).
- The demo image is built from the same base image as user images (`cc-deck-base`).
- Logo assets are already available in `assets/logo/` (icon, wordmark, outline variants).
