# Brainstorm: Landing Page Revival

**Date:** 2026-04-30
**Status:** active
**Spec:** (pending, will be created by ship pipeline)

## Problem Framing

The cc-deck landing page at cc-deck.github.io is a minimal "Coming Soon" placeholder with just a title, tagline, and two buttons.
The project has grown significantly since the original spec 019 was written, with 6 workspace types, a full-featured Zellij sidebar plugin, voice relay, build system, session snapshots, domain filtering, and credential management.
The current placeholder does not communicate any of this.
The page needs to be replaced with a feature-complete, developer-focused landing page that showcases the full cc-deck feature set.

### Current State

The existing page (`cc-deck.github.io/src/pages/index.astro`) is a centered "Coming Soon" badge with:
- Title: "cc-deck"
- Subtitle: "Your Claude Code command center."
- Description: "The TweetDeck for Claude Code. Manage in one place, run anywhere."
- Two buttons: "Read the Docs" and "GitHub"

The site uses Astro 5.x + Tailwind CSS with a dark/light theme toggle and deep blue (#1e40af) accent.
Existing widget components are available: Hero, Features, Features2, Features3, Steps, Steps2, CallToAction, Content, FAQs, Stats, Testimonials.
Antora documentation is integrated and served under /docs/.

### Feature Inventory (as of 2026-04-30)

**CLI Commands (cc-deck):**
- Workspace lifecycle: new, attach, start, stop, delete, kill-session, update, refresh-creds
- Workspace data: exec, push, pull, harvest, voice, pipe
- Workspace info: list, status, logs, prune
- Build system: init, run, verify, diff
- Config: plugin install/status/remove, profile add/list/use/show, domains init/list/show/blocked/add/remove
- Session snapshots: save, restore, list, remove

**Zellij Sidebar Plugin:**
- Smart Attend with 4-tier priority (permission-waiting, notification-waiting, done/agent-done, idle)
- Keyboard navigation (cursor-based, j/k/Enter/Esc/g/G, slash search, inline rename, delete with confirmation)
- Session status indicators (init, working, waiting, idle, done, agent-done)
- Voice integration (connection indicator, mute toggle via Alt+m)
- Hook integration (automatic Claude Code event tracking)
- Tab-aware session filtering (per-Zellij-session isolation)
- Help overlay, overflow indicators, git branch display

**Workspace Types:**
- Local (native Zellij sessions)
- Container (single Podman container)
- Compose (multi-container podman-compose stacks)
- SSH (remote machines with jump host support)
- Kubernetes Deploy (persistent StatefulSets with PVC)
- Kubernetes Sandbox (ephemeral pods, planned)

**Voice Relay:**
- Local microphone capture with Whisper transcription
- Voice Activity Detection with mute/unmute
- Stop word detection ("submit", "enter") for prompt submission
- Attend trigger word ("next") for session cycling
- Filler word stripping
- Bidirectional mute state sync with sidebar

**Supporting Features:**
- Custom image builder (manifest-driven container builds)
- Session snapshots (save/restore workspace state)
- Domain filtering (network access control via domain groups)
- Credential profiles (API key and Vertex AI management)

## Approaches Considered

### A: Feature-First Layout (Chosen)

A clean, developer-focused page that leads with the sidebar plugin, then cascades through the full feature set:

1. Hero (title, subtitle, placeholder GIF slot, two CTAs)
2. Sidebar Plugin features (3-column grid: Smart Attend, Keyboard Nav, Session Status, Voice, Session Mgmt, Hooks)
3. Run Anywhere (3-column grid: 6 workspace types)
4. Get Started (tabbed: demo container one-liner / local install)
5. More Features (2-column grid: image builder, snapshots, domain filtering, credential profiles)
6. Footer (docs, GitHub, license)

- Pros: Mirrors natural discovery flow. Sidebar is the hook, environments are the scale story. Technical and focused.
- Cons: Longer page, more content to maintain.

### B: Minimal Technical

Three sections: Hero, single feature grid, quickstart block. Everything else in docs.

- Pros: Low maintenance, fast to build.
- Cons: Undersells the project's breadth.

### C: Use-Case Driven

Organize by user journeys: "Solo developer", "Team lead", "Platform engineer".

- Pros: Speaks to each audience directly.
- Cons: Features duplicated across sections, harder to maintain, feels like marketing.

## Decision

Chosen Approach A (Feature-First Layout) because:
- Both individual developers and teams/platform engineers are the target audience
- Developer tool positioning (technical, focused, no marketing fluff) was selected
- The sidebar plugin should be highlighted first as the core differentiator
- Plan for visual placeholders now (styled terminal boxes), add screenshots/GIFs later
- Quickstart section should have both demo container one-liner and local install in tabs
- Keep the existing Astro + Tailwind framework

### Key Design Decisions

1. **Audience**: Both individual developers and teams/platform engineers equally
2. **Tone**: Developer tool, not product page. "Here's what it does and how to get started."
3. **Lead feature**: Sidebar plugin first (core differentiator), then environments (scale story)
4. **Visuals**: Plan for image/GIF slots with styled placeholders until assets are ready
5. **Quickstart**: Two paths in tabs (demo container + local install)
6. **Framework**: Keep Astro + Tailwind (already set up with theme, components, Antora integration)

### Requirements Summary

- Replace "Coming Soon" stub with full 6-section page
- Reuse existing Astro widgets (Hero, Features, CallToAction, Footer)
- New component only for tabbed quickstart if needed (vanilla JS tab switcher)
- Dark/light theme compatibility with deep blue (#1e40af) accent
- Responsive (mobile single-column, tablet/desktop multi-column)
- Server-rendered HTML, only client-side JS is tab switcher
- No new npm dependencies

## Open Threads

- Specific Tabler icons for each feature card (decide during implementation)
- Exact demo container one-liner wording (depends on current image tag and registry)
- Local install path: brew vs curl one-liner (depends on release process status)
- When to create actual screenshots/GIFs to replace placeholders
