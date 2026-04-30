# Research: Landing Page Revival

**Date**: 2026-04-30
**Feature**: specs/047-landing-page-revival/spec.md

## Site Framework

- **Decision**: Use existing Astro 5.12.9 + Tailwind CSS 3.4.17 setup
- **Rationale**: Site already configured with dark/light theme, icon library, and component library
- **Alternatives considered**: None (spec requires reusing existing framework)

## Icon Library

- **Decision**: Use Tabler icons via `astro-icon` with `tabler:icon-name` format
- **Rationale**: All Tabler icons are available (configured with `['*']` wildcard in astro.config.ts). The `@iconify-json/tabler` package is already installed.
- **Alternatives considered**: flat-color-icons (limited set, not suitable for feature cards)

## Component Selection

### Hero Section
- **Decision**: Use existing `Hero` widget component
- **Rationale**: Supports title, subtitle, content slot, actions array (CallToAction buttons), and image slot. Matches all FR-002 requirements.
- **Props needed**: title, subtitle, actions (2 CTAs), image (placeholder HTML)

### Feature Grid Sections (Sidebar Plugin, Run Anywhere, More Features)
- **Decision**: Use existing `Features` widget component
- **Rationale**: Supports items array (Item with icon, title, description), configurable columns (2 or 3), and defaultIcon. Wraps `ItemGrid` which renders icons via `astro-icon`.
- **Props needed**: title, tagline, items (array of Item), columns (2 or 3)

### Quickstart Section (Tabbed)
- **Decision**: Create a new `TabbedCode` component (inline in index.astro or as separate widget)
- **Rationale**: No existing widget supports tabbed content. The tab switcher is simple vanilla JS (toggle display of two `<div>` elements). FR-012 permits a new component specifically for this case.
- **Alternatives considered**: Using Steps widget (rejected: steps are sequential, not tabbed), using Content widget (rejected: no tab switching capability)

### Footer
- **Decision**: Use existing Footer via PageLayout (already rendered automatically)
- **Rationale**: FR-013 requires footer to remain unchanged

## Page Layout

- **Decision**: Use `PageLayout` (not `LandingLayout`)
- **Rationale**: PageLayout provides the standard header with Docs/GitHub links and footer. LandingLayout adds a Download button and Announcement bar which are not needed.

## Placeholder Image Strategy

- **Decision**: Render a styled `<div>` with terminal appearance (dark background, rounded corners, muted text) instead of an `<img>` tag
- **Rationale**: FR-011 requires no broken image indicators. A styled div ensures clean rendering regardless of asset availability.

## Site URL Configuration

- **Decision**: Fix astro.config.ts site URL from `antwort-dev.github.io` to `cc-deck.github.io`
- **Rationale**: Config currently references wrong domain (noted in spec 019 evolution note). Should be corrected as part of this feature.

## Tabler Icon Selection

Based on feature semantics and available Tabler icons:

### Sidebar Plugin Features
| Feature | Icon | Rationale |
|---------|------|-----------|
| Smart Attend | `tabler:focus-2` | Represents focused attention/targeting |
| Keyboard Navigation | `tabler:keyboard` | Direct representation |
| Session Status | `tabler:activity` | Activity/status monitoring |
| Voice Control | `tabler:microphone` | Voice input |
| Session Management | `tabler:layout-list` | List/session management |
| Hook Integration | `tabler:webhook` | Hook/integration concept |

### Workspace Types
| Type | Icon | Rationale |
|------|------|-----------|
| Local | `tabler:device-desktop` | Local machine |
| Container | `tabler:box` | Container concept |
| Compose | `tabler:stack-2` | Stacked containers |
| SSH | `tabler:terminal` | Remote terminal |
| Kubernetes Deploy | `tabler:cloud` | Cloud deployment |
| Kubernetes Sandbox | `tabler:flask` | Ephemeral/experimental |

### Secondary Features
| Feature | Icon | Rationale |
|---------|------|-----------|
| Custom Image Builder | `tabler:hammer` | Build tool |
| Session Snapshots | `tabler:camera` | Snapshot capture |
| Domain Filtering | `tabler:filter` | Filtering concept |
| Credential Profiles | `tabler:key` | Authentication/credentials |
