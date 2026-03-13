# Data Model: cc-deck Documentation & Landing Page

**Date**: 2026-03-13
**Branch**: 019-docs-landing-page

## Entities

### Documentation Module

Each Antora module is a self-contained documentation section.

| Field | Type | Description |
|-------|------|-------------|
| name | string | Module directory name (e.g., `plugin`, `podman`) |
| title | string | Display title in navigation |
| nav | file | `nav.adoc` defining page hierarchy |
| pages | directory | AsciiDoc content pages |
| images | directory | Screenshots, diagrams (optional) |
| examples | directory | Code snippets, config examples (optional) |

### Landing Page Section

Content blocks on the landing page.

| Section | Purpose |
|---------|---------|
| Hero | Logo, tagline, subtitle, CTA buttons |
| Features | 4 USP cards with icons and descriptions |
| Steps | Getting started flow (numbered steps) |
| CallToAction | Final CTA with doc and GitHub links |

### Demo Image

| Field | Type | Description |
|-------|------|-------------|
| base | string | `quay.io/rhuss/cc-deck-base:latest` |
| tools | list | cc-deck, Zellij, Claude Code (private Node.js 20) |
| auth | env vars | `ANTHROPIC_API_KEY` or Vertex AI variables |
| cmd | string | `sleep infinity` |

## Content Structure

### Antora Component Descriptor (`docs/antora.yml`)

```yaml
name: cc-deck
title: cc-deck
version: '0.1'
start_page: ROOT:index.adoc
nav:
  - modules/ROOT/nav.adoc
  - modules/quickstarts/nav.adoc
  - modules/plugin/nav.adoc
  - modules/images/nav.adoc
  - modules/podman/nav.adoc
  - modules/kubernetes/nav.adoc
  - modules/reference/nav.adoc
  - modules/developer/nav.adoc
```

### Antora Playbook (`cc-deck.github.io/antora-playbook.yml`)

```yaml
site:
  title: cc-deck
  url: https://rhuss.github.io/cc-deck
  start_page: cc-deck::index.adoc

content:
  sources:
    - url: https://github.com/rhuss/cc-deck.git
      branches: main
      start_path: docs

ui:
  bundle:
    url: ./ui-bundle.zip
    snapshot: true
  supplemental_files: ./supplemental-ui

output:
  dir: ./dist/docs
```

## Module Content Outline

### ROOT
- `index.adoc`: What is cc-deck, feature overview, architecture diagram
- Links to all other modules

### quickstarts
- `one-liner.adoc`: Demo image quickstart (API key + Vertex AI)
- `install.adoc`: Native installation (make build, make install)
- `first-session.adoc`: First Zellij + Claude Code session

### plugin
- `overview.adoc`: Sidebar plugin concept
- `navigation.adoc`: Keyboard shortcuts, navigation mode
- `attend.adoc`: Smart attend algorithm, priority tiers
- `sessions.adoc`: Pause, rename, search, delete
- `configuration.adoc`: Layout variants, personal layout, key config

### images
- `overview.adoc`: Build pipeline concept
- `init.adoc`: cc-deck image init
- `extract.adoc`: /cc-deck.extract command
- `settings.adoc`: /cc-deck.settings command
- `build.adoc`: /cc-deck.build command
- `manifest.adoc`: cc-deck-build.yaml schema reference

### podman
- `quickstart.adoc`: Minimal local setup
- `volumes.adoc`: Source code mounts, persistent state
- `credentials.adoc`: API key and Vertex AI setup
- `advanced.adoc`: GPU passthrough, MCP port forwarding, networking

### kubernetes
- `quickstart.adoc`: Minimal K8s deployment
- `statefulset.adoc`: StatefulSet pattern with PVCs
- `credentials.adoc`: Secrets, service accounts, Vertex AI
- `rbac.adoc`: RBAC configuration
- `scaling.adoc`: Multi-session scaling, resource management

### reference
- `cli.adoc`: All CLI commands with flags and examples
- `manifest-schema.adoc`: Full cc-deck-build.yaml schema
- `configuration.adoc`: Config file, environment variables
- `mcp-labels.adoc`: MCP label schema for container images

### developer
- `architecture.adoc`: Two-component design, sync protocol
- `building.adoc`: Build from source (Rust + Go)
- `contributing.adoc`: PR guidelines, testing, code style
