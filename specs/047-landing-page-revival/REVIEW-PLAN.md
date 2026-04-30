# Review Guide: Landing Page Revival

**Spec:** [spec.md](spec.md) | **Plan:** [plan.md](plan.md) | **Tasks:** [tasks.md](tasks.md)
**Generated:** 2026-04-30

---

## What This Spec Does

The cc-deck project website currently shows a "Coming Soon" placeholder. This spec replaces it with a full landing page that showcases the sidebar plugin, six workspace types, a tabbed quickstart, and secondary features. The target audience is both individual developers and platform engineers evaluating Claude Code session management tools.

**In scope:** Single-page replacement of `index.astro` in the `cc-deck.github.io` repository, one new Astro component (TabbedCode), and a config URL fix.

**Out of scope:** Documentation site changes (Antora), blog, analytics, actual screenshot/GIF assets, CI/CD pipeline changes. The page uses placeholder image slots that will be filled separately.

## Bigger Picture

This landing page has been "Coming Soon" since spec 019 (March 2026). The project has since added voice relay, session snapshots, domain filtering, Kubernetes workspaces, SSH environments, and compose support. None of this is visible to potential users arriving at the homepage.

The landing page is a prerequisite for the project's discoverability. Without it, the documentation site at `/docs/` is the only entry point, and it assumes the visitor already knows what cc-deck is. This spec fills the discovery gap.

The existing Astro + Tailwind site was scaffolded from the `antwort.github.io` template. The Antora docs integration is already working. After this landing page ships, the natural next steps would be demo recordings (spec 020, partially done) and a quickstart guide refresh.

---

## Spec Review Guide (30 minutes)

> Focus your review on whether the page structure and feature descriptions accurately represent cc-deck today.

### Understanding the approach (8 min)

Read [User Story 1](spec.md#user-story-1---discover-cc-deck-value-proposition-priority-p1) and the [page structure in the plan](plan.md#phase-3-build-landing-page). As you read, consider:

- Does leading with the sidebar plugin (before workspace types) match how you think about the product's identity? Or should "Run Anywhere" come first?
- Is "Your Claude Code command center" still the right subtitle, or has the project evolved past that framing?
- The page has no screenshots or recordings. Is a text-only landing page credible enough for a first release, or should we block on at least one visual asset?

### Key decisions that need your eyes (12 min)

**Feature card descriptions** ([plan.md Feature Descriptions](plan.md#feature-descriptions))

All 16 feature cards (6 sidebar + 6 workspace + 4 secondary) have written descriptions. These are the most visible text on the page and need to be accurate.
- Question for reviewer: Do the descriptions match current behavior? In particular, is "Kubernetes Sandbox" described as "Ephemeral pods for temporary, disposable coding sessions" accurate given the feature is marked as "planned" in the brainstorm inventory?

**Icon selection** ([research.md Icon Selection](research.md))

Tabler icons were chosen for each card based on semantic fit. Some choices are debatable (e.g., `tabler:flask` for Kubernetes Sandbox, `tabler:focus-2` for Smart Attend).
- Question for reviewer: Do any icons feel misleading or confusing for the concept they represent?

**TabbedCode component** ([plan.md Phase 2](plan.md#phase-2-create-tabbedcode-component))

A new Astro component is created specifically for the tabbed quickstart. The spec permits this (FR-012) since no existing widget supports tabs.
- Question for reviewer: Should the tab switcher use CSS-only approach (`:target` or hidden radio buttons) instead of vanilla JS for zero-JS operation? The spec requires JS-disabled degradation but the plan uses a JS toggle with stacked fallback.

**Quickstart commands** ([tasks.md T006](tasks.md))

The exact `podman run` and `brew install` commands are not yet specified in the plan. They reference "demo image" and "brew install cc-deck" generically.
- Question for reviewer: Are these commands published and working? If not, should the quickstart use placeholder text until the release process (spec 021) delivers them?

### Areas where I am less certain (5 min)

- [spec.md Assumptions](spec.md#assumptions): The spec assumes logo assets exist in `public/`. The research found `logo.png` and `cc-deck-icon.svg`, but the spec originally referenced "logo, wordmark, outline variants" from spec 019. It is unclear which logo variant the hero should use.
- [plan.md astro.config.ts fix](plan.md#phase-1-fix-site-configuration): The site URL is currently `antwort-dev.github.io`. Changing it to `cc-deck.github.io` may affect Antora integration or existing deployed links. I did not verify whether any Antora config references this URL.
- [tasks.md T008](tasks.md): Navigation verification is listed as a user story task, but it is really a verification step. If navigation is already correct (no changes to `navigation.ts`), this task does nothing. This is fine, but the task framing is slightly awkward.

### Risks and open questions (5 min)

- The landing page lives in a sibling repository (`../cc-deck.github.io/`), not the main cc-deck repo. Implementation will modify files outside the current working directory. Does the CI/CD for that repository deploy automatically on push to main?
- If the `podman run` demo image is not yet published to `quay.io/cc-deck/`, the quickstart section will show commands that fail. Should the quickstart be behind a feature flag or condition until the image is available?
- The "Kubernetes Sandbox" feature appears in the workspace types grid, but the brainstorm inventory marks it as "planned." Including an unimplemented feature on the landing page could mislead users.

---
*Full context in linked [spec](spec.md) and [plan](plan.md).*
