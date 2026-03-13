# Research: cc-deck Documentation & Landing Page

**Date**: 2026-03-13
**Branch**: 019-docs-landing-page

## Astro Landing Page Pattern

### Decision: Clone antwort.github.io as template

**Rationale**: The antwort.github.io repo uses Astro 5 + Tailwind CSS with the exact
pattern needed (Hero, Features, Steps, CTA widgets, dark/light theme toggle, Antora
integration). Cloning and re-branding is faster than starting from scratch.

**Alternatives considered**:
- Docusaurus (rejected: Antora handles docs, Docusaurus would be redundant)
- Plain HTML (rejected: no dark/light theme support, no component reuse)
- Hugo (rejected: less flexible for custom landing pages)

## Antora Documentation Structure

### Decision: 8 modules matching the user stories

**Rationale**: Each module maps to a user story from the spec, making content planning
straightforward. Modules are independently navigable and can be developed in parallel.

**Module mapping**:
- ROOT: US1 (discovery, overview)
- quickstarts: US2 (one-liner demo)
- plugin: US3 (sidebar features)
- images: US4 (build pipeline)
- podman: US5 (local deployment)
- kubernetes: US6 (enterprise deployment)
- reference: cross-cutting (CLI, schema, config)
- developer: US7 (architecture, contributing)

## Color Scheme

### Decision: Deep blue primary with Claude orange accent

**Rationale**: Deep blue (#1e40af) is professional, accessible, and differentiates from
antwort's teal. The Claude orange asterisk (#E87B35) in the logo provides warm contrast.
Both work in dark and light themes.

**CSS variables** (matching antwort pattern):
```css
/* Light mode */
--aw-color-primary: rgb(30 64 175);     /* #1e40af */
--aw-color-secondary: rgb(37 99 235);   /* #2563eb */
--aw-color-accent: rgb(96 165 250);     /* #60a5fa */
--aw-color-bg-page: rgb(255 255 255);

/* Dark mode */
--aw-color-primary: rgb(96 165 250);    /* #60a5fa */
--aw-color-secondary: rgb(59 130 246);  /* #3b82f6 */
--aw-color-accent: rgb(96 165 250);     /* #60a5fa */
--aw-color-bg-page: rgb(3 6 32);        /* #030620 */
```

## Demo Image

### Decision: Minimal Containerfile on top of cc-deck-base

**Rationale**: The demo image adds only cc-deck + Zellij + Claude Code on top of the
base image. No project-specific tools. This keeps it small and generic for quickstarts.

**Build**: `make demo-image` target in the main Makefile. Uses the same cross-compile
and Node.js 20 pattern as user images.

**Authentication**: Supports both `ANTHROPIC_API_KEY` env var and Vertex AI
(`CLAUDE_CODE_USE_VERTEX` + gcloud mount).

## Antora UI Bundle

### Decision: Customize the default Antora UI with cc-deck branding

**Rationale**: The default Antora UI is functional. Customization via supplemental-ui
(logo, colors, favicon) is sufficient. A full custom UI bundle is overkill.

**Customizations**:
- Logo in header (cc-deck-wordmark.png)
- Favicon (cc-deck-icon.png)
- Color overrides via supplemental CSS
- Footer with GitHub link

## GitHub Pages Deployment

### Decision: GitHub Actions workflow for build and deploy

**Rationale**: Standard pattern for GitHub Pages. The workflow builds both Astro
(landing page) and Antora (docs), then deploys to Pages.

**Workflow triggers**:
- Push to `main` on cc-deck.github.io (landing page changes)
- Manual dispatch (to pull latest docs from cc-deck main)
- Webhook from cc-deck repo on docs changes (optional, can be manual initially)
