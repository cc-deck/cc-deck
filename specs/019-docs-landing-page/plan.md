# Implementation Plan: cc-deck Documentation & Landing Page

**Branch**: `019-docs-landing-page` | **Date**: 2026-03-13 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/019-docs-landing-page/spec.md`

## Summary

Create the cc-deck project website with an Astro-based landing page and Antora-based
documentation site, both hosted at `cc-deck.github.io`. Documentation covers 8 modules
(overview, quickstarts, plugin, images, podman, kubernetes, reference, developer).
A pre-built demo image enables one-liner quickstarts.

## Technical Context

**Language/Version**: TypeScript (Astro 5.x), AsciiDoc (Antora 3.x), Containerfile (demo image)
**Primary Dependencies**: Astro, Tailwind CSS, Antora, AsciiDoc
**Storage**: N/A (static site)
**Testing**: Lighthouse (landing page), Antora build validation (docs), manual quickstart verification
**Target Platform**: Web (GitHub Pages), container registries (quay.io)
**Project Type**: Documentation site + landing page + demo container image
**Performance Goals**: Lighthouse 90+ for performance and accessibility
**Constraints**: Must work with GitHub Pages (static hosting only)
**Scale/Scope**: 8 doc modules, 1 landing page, 1 demo image

## Constitution Check

No constitution file exists. No gates to check.

## Project Structure

### Documentation (this feature)

```text
specs/019-docs-landing-page/
├── plan.md              # This file
├── spec.md              # Feature specification
├── research.md          # Technology decisions
├── data-model.md        # Content structure
├── quickstart.md        # Implementation walkthrough
└── checklists/
    └── requirements.md  # Quality checklist
```

### Source Code (two repositories)

```text
# Repository 1: cc-deck (existing, add docs/ directory)
cc-deck/
├── docs/                          # Antora documentation source
│   ├── antora.yml                 # Component descriptor
│   └── modules/
│       ├── ROOT/                  # Overview, what is cc-deck
│       │   ├── nav.adoc
│       │   └── pages/
│       │       └── index.adoc
│       ├── quickstarts/           # Getting started guides
│       │   ├── nav.adoc
│       │   └── pages/
│       ├── plugin/                # Zellij sidebar plugin
│       │   ├── nav.adoc
│       │   └── pages/
│       ├── images/                # Container image pipeline
│       │   ├── nav.adoc
│       │   └── pages/
│       ├── podman/                # Podman local deployment
│       │   ├── nav.adoc
│       │   └── pages/
│       ├── kubernetes/            # K8s/OpenShift deployment
│       │   ├── nav.adoc
│       │   └── pages/
│       ├── reference/             # CLI, manifest schema, config
│       │   ├── nav.adoc
│       │   └── pages/
│       └── developer/             # Architecture, contributing
│           ├── nav.adoc
│           └── pages/
├── assets/logo/                   # Logo assets (already committed)
└── demo-image/                    # Demo image build
    └── Containerfile

# Repository 2: cc-deck.github.io (new)
cc-deck.github.io/
├── antora-playbook.yml            # Pulls docs from cc-deck repo
├── astro.config.ts                # Astro site config
├── tailwind.config.js             # Tailwind with cc-deck colors
├── package.json
├── src/
│   ├── assets/
│   │   └── styles/
│   │       └── tailwind.css
│   ├── components/
│   │   ├── CustomStyles.astro     # cc-deck color variables
│   │   └── widgets/               # Hero, Features, Steps, CTA
│   ├── config.yaml                # Site metadata
│   ├── layouts/
│   ├── navigation.ts
│   └── pages/
│       └── index.astro            # Landing page
├── supplemental-ui/               # Antora UI customization
├── ui-bundle.zip                  # Custom Antora UI bundle
└── public/
    ├── favicon.ico
    └── logo.png
```

**Structure Decision**: Two-repo pattern matching antwort/antwort.github.io. Documentation
AsciiDoc sources live in the main repo for co-evolution with code. The landing page and
Antora playbook live in the `.github.io` repo for GitHub Pages deployment.

## Complexity Tracking

No constitution violations. No complexity justifications needed.
