# Implementation Plan: Landing Page Revival

**Branch**: `047-landing-page-revival` | **Date**: 2026-04-30 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `specs/047-landing-page-revival/spec.md`

## Summary

Replace the "Coming Soon" placeholder at cc-deck.github.io with a full-featured, developer-focused landing page. The page uses the existing Astro 5.x + Tailwind CSS framework and its widget component library (Hero, Features, Footer) to present cc-deck's complete feature set across six sections: hero, sidebar plugin features, workspace types, tabbed quickstart, secondary features, and footer.

## Technical Context

**Language/Version**: TypeScript (Astro 5.12.9)
**Primary Dependencies**: Astro 5.12.9, Tailwind CSS 3.4.17, astro-icon 1.1.5, @iconify-json/tabler 1.2.20
**Storage**: N/A (static site)
**Testing**: Visual inspection, `astro check`, `astro build` (no unit test framework)
**Target Platform**: Static site hosted on GitHub Pages
**Project Type**: Static website (Astro)
**Performance Goals**: Page load under 2 seconds, Lighthouse accessibility 90+
**Constraints**: No new npm dependencies, reuse existing components, dark/light theme support
**Scale/Scope**: Single page (index.astro) in existing site repository

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Applies? | Status |
|-----------|----------|--------|
| I. Tests and documentation | Partially | This IS the documentation/landing page. No new CLI commands or config options. README.md update may be needed. |
| II. Interface contracts | No | No new interface implementations. |
| III. Build and tool rules | Partially | Container runtime references in quickstart must use "podman" (not Docker). No `go build` or `cargo build` involved. |

**Gate result**: PASS. No violations.

## Project Structure

### Documentation (this feature)

```text
specs/047-landing-page-revival/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── spec.md              # Feature specification
├── REVIEW-SPEC.md       # Spec review
├── checklists/
│   └── requirements.md  # Quality checklist
└── tasks.md             # Phase 2 output (created by /speckit-tasks)
```

### Source Code (cc-deck.github.io repository)

```text
cc-deck.github.io/
├── src/
│   ├── pages/
│   │   └── index.astro          # MODIFY: Replace placeholder with full page
│   ├── components/
│   │   └── widgets/
│   │       └── TabbedCode.astro # NEW: Tabbed code block component
│   └── navigation.ts           # REVIEW: May need updates if nav changes
├── astro.config.ts              # FIX: Site URL (antwort-dev -> cc-deck)
└── public/                      # Existing assets (logo.png, cc-deck-icon.svg)
```

**Structure Decision**: Minimal changes. One modified file (index.astro), one new component (TabbedCode.astro), one config fix (astro.config.ts). All work in the existing cc-deck.github.io repository at `../cc-deck.github.io/`.

## Implementation Approach

### Phase 1: Fix Site Configuration

Fix the astro.config.ts site URL from `antwort-dev.github.io` to `cc-deck.github.io`.

### Phase 2: Create TabbedCode Component

Create `src/components/widgets/TabbedCode.astro` for the tabbed quickstart section:
- Two tabs with vanilla JS switcher
- Monospace code blocks with dark terminal styling
- Graceful degradation: both blocks visible when JS disabled (use `<noscript>` class override or CSS-only approach)
- Follows existing widget patterns (WidgetWrapper, Headline)

### Phase 3: Build Landing Page

Replace `src/pages/index.astro` with the full 6-section page:

1. **Hero** (existing Hero widget):
   - title: "cc-deck"
   - subtitle: "Your Claude Code command center"
   - content: "Manage multiple Claude Code sessions from a single sidebar. Run anywhere: local, container, SSH, Kubernetes."
   - actions: [Get Started (anchor #get-started), GitHub (external link)]
   - image: Styled placeholder div (terminal-like box)

2. **Sidebar Plugin** (existing Features widget, columns=3):
   - tagline: "Sidebar Plugin"
   - title: "Everything visible. One keystroke away."
   - 6 items with Tabler icons: Smart Attend (tabler:focus-2), Keyboard Navigation (tabler:keyboard), Session Status (tabler:activity), Voice Control (tabler:microphone), Session Management (tabler:layout-list), Hook Integration (tabler:webhook)

3. **Run Anywhere** (existing Features widget, columns=3):
   - tagline: "Workspaces"
   - title: "Run anywhere"
   - 6 items: Local (tabler:device-desktop), Container (tabler:box), Compose (tabler:stack-2), SSH (tabler:terminal), Kubernetes Deploy (tabler:cloud), Kubernetes Sandbox (tabler:flask)

4. **Get Started** (new TabbedCode widget):
   - id: "get-started" (anchor target)
   - Tab 1 "Try it now": `podman run` one-liner with demo image
   - Tab 2 "Install locally": brew install + cc-deck config plugin install

5. **More Features** (existing Features widget, columns=2):
   - tagline: "And more"
   - title: "Built for real workflows"
   - 4 items: Custom Image Builder (tabler:hammer), Session Snapshots (tabler:camera), Domain Filtering (tabler:filter), Credential Profiles (tabler:key)

6. **Footer** (existing, via PageLayout, no changes)

### Phase 4: Verify

- Run `astro check` and `astro build` to verify no errors
- Test dark/light mode toggle
- Test responsive layout at mobile/tablet/desktop breakpoints
- Verify all links work (docs, GitHub, anchor scroll)
- Test tab switcher functionality

## Feature Descriptions

Accurate descriptions for each feature card, reflecting current capabilities:

### Sidebar Plugin Features

| Feature | Description |
|---------|-------------|
| Smart Attend | Automatically cycles through sessions that need attention, prioritized by urgency: permission requests first, then notifications, then completed work. |
| Keyboard Navigation | Full cursor-based control with familiar keybindings. Navigate sessions, switch focus, search, rename, and delete without leaving the keyboard. |
| Session Status | Real-time activity indicators show which sessions are working, waiting for input, idle, or finished. See the state of every session at a glance. |
| Voice Control | Dictate into your active session hands-free. Built-in speech recognition with configurable trigger words for submitting prompts and cycling sessions. |
| Session Management | Rename, delete, pause, and filter sessions from the sidebar. Inline editing and confirmation dialogs keep you in control. |
| Hook Integration | Automatic Claude Code event tracking. The sidebar updates session states in real time as Claude Code works, waits, or finishes. |

### Workspace Types

| Type | Description |
|------|-------------|
| Local | Native Zellij sessions on your host machine. Zero overhead, direct file access. |
| Container | Isolated environment in a single Podman container with custom images and volume mounts. |
| Compose | Multi-container stacks via podman-compose for complex development setups. |
| SSH | Remote workspaces on any machine reachable via SSH, with jump host support. |
| Kubernetes Deploy | Persistent workspaces as StatefulSets with PVC storage for team environments. |
| Kubernetes Sandbox | Ephemeral pods for temporary, disposable coding sessions. |

### Secondary Features

| Feature | Description |
|---------|-------------|
| Custom Image Builder | Build container images from a declarative manifest. Define your tools, plugins, and MCP servers, then build and push with a single command. |
| Session Snapshots | Save and restore your entire workspace state. Pick up exactly where you left off, even across machine reboots. |
| Domain Filtering | Control which domains your coding sessions can reach. Group domains into allowlists for security-conscious environments. |
| Credential Profiles | Manage API keys and Vertex AI credentials as named profiles. Switch between authentication methods without editing config files. |

## Complexity Tracking

No constitution violations. No complexity justifications needed.
