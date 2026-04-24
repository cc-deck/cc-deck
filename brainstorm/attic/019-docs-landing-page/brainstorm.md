# Brainstorm: cc-deck Documentation & Landing Page

**Date**: 2026-03-13
**Feature**: 019-docs-landing-page

## Decision: Two-Repo Architecture

Same pattern as antwort/antwort.github.io:

| Repo | Purpose | Tech |
|------|---------|------|
| `rhuss/cc-deck` | Source code + Antora docs in `docs/` | Go, Rust, Antora AsciiDoc |
| `rhuss/cc-deck.github.io` | Landing page + Antora playbook | Astro + Tailwind + Antora |

Antora playbook in `.github.io` pulls docs from main repo's `docs/` directory.

## Documentation Modules (Antora)

```
docs/modules/
  ROOT/          # What is cc-deck, overview, feature highlights
  quickstarts/   # 5-min install, first session, one-liner demo
  plugin/        # Zellij sidebar: navigation, attend, pause, shortcuts
  images/        # Container image pipeline: extract, settings, build, push
  podman/        # Full reference: volumes, credentials, GPU, persistent containers
  kubernetes/    # Full reference: StatefulSets, PVCs, RBAC, scaling, credentials
  reference/     # CLI reference, manifest schema, config, MCP labels
  developer/     # Architecture, contributing, building from source
```

## Landing Page Structure

Following antwort pattern: Hero, Features, Steps, CallToAction.

### Hero
```
cc-deck
Your Claude Code command center.

Session management for Claude Code on any platform.
Zellij sidebar plugin. Custom container images. Local or Kubernetes.

[Get Started]  [View on GitHub]
```

### Features (USPs)
1. **Zellij sidebar plugin**: Session tracking, smart attend, keyboard navigation, pause/resume
2. **Custom container images**: AI-driven build pipeline (extract tools, configure settings, build)
3. **Multi-platform**: Runs locally, in Podman containers, on Kubernetes/OpenShift
4. **Session management**: Snapshots, multi-session orchestration, persistent state

### Steps (Getting Started)
Install, launch, build image, deploy

### CallToAction
GitHub link, quickstart docs

## Target Audience

Claude Code users who want flexible session monitoring across platforms (local, remote, containers, K8s). The primary experience is the Zellij sidebar plugin. Container images and K8s deployment are the scaling story.

## Color Scheme

Deep blue primary, dark/light theme support (same approach as antwort):

| Role | Light mode | Dark mode |
|------|-----------|-----------|
| Primary | `#1e40af` (deep blue) | `#60a5fa` (bright blue) |
| Secondary | `#2563eb` | `#3b82f6` |
| Accent | `#60a5fa` | `#60a5fa` |
| Background | `#ffffff` (white) | `#030620` (dark, matching antwort) |
| Claude orange | `#E87B35` | `#E87B35` (logo accent only) |

## Logo

Direction: Stylized terminal window with sidebar + Claude asterisk in orange.

### Image Generation Prompts

**Prompt 1 (Primary logo, clean/minimal):**

> A minimalist tech logo for "cc-deck", a developer tool for managing AI coding sessions. The design shows a stylized terminal window with a narrow sidebar on the left side. The sidebar contains 3-4 small horizontal lines representing session entries. The main area of the terminal is empty, representing the workspace. In the top-right corner of the terminal window, a small schematic six-pointed asterisk symbol (resembling the Claude AI logo) glows in warm orange (#E87B35). The terminal frame uses deep blue (#1e40af) lines on a transparent background. The overall style is geometric, flat, and modern, suitable for use as a favicon, GitHub avatar, and documentation header. No text in the logo. Clean vector style, no gradients, no shadows, no 3D effects.

**Prompt 2 (Icon variant with more personality):**

> A square app icon for "cc-deck", a developer command center tool. The design shows a dark blue (#1e40af) rounded rectangle resembling a terminal window. Inside, a thin vertical line divides it into a narrow left sidebar and wide right pane. The sidebar has 3 small dots or short lines stacked vertically (representing tracked sessions, one highlighted in teal #0d9488). In the upper right of the main pane, a small six-pointed asterisk (the Claude AI star symbol) in warm orange (#E87B35) with slightly rounded points. The background of the terminal interior is dark slate (#0f172a). Flat design, no text, suitable for 64x64px and 512x512px. Modern developer tooling aesthetic.

**Prompt 3 (Wordmark variant for docs header):**

> A horizontal wordmark logo reading "cc-deck" in a modern monospace font (like JetBrains Mono or Fira Code). The "cc" is in deep blue (#1e40af). The hyphen is replaced by a small schematic six-pointed asterisk in warm orange (#E87B35), resembling the Claude AI logo. The "deck" is in a slightly lighter blue (#3b82f6). Below the wordmark in smaller text: "Claude Code Command Center" in medium gray. Clean flat design on transparent background, suitable for website header and documentation.

**Prompt 4 (Favicon/compact):**

> A compact square icon: a dark blue (#1e40af) rounded square with a white vertical line creating a sidebar split. A small warm orange (#E87B35) six-pointed asterisk sits in the upper portion of the main area. Minimal, recognizable at 16x16px. Flat vector, no gradients.

## Podman Section (Full Reference)

Comprehensive guide covering:
- Volume mounts for local source directories (`-v ~/projects:/home/coder/projects`)
- Credential passthrough (API key env var, Vertex AI gcloud mount)
- GPU passthrough for local models
- Port forwarding for MCP servers
- Persistent containers (`sleep infinity` pattern)
- Networking and DNS
- Working directly on local files (bidirectional mount)

## Kubernetes Section (Full Reference)

Comprehensive guide covering:
- StatefulSet deployment patterns
- PersistentVolume/PVC for source code and state
- Credential injection via Secrets (API key, Vertex AI service accounts)
- RBAC configuration
- Scaling multiple sessions
- Service for MCP server access
- Port-forward for development

## Quickstart: One-Liner Demo

Pre-built `cc-deck-demo` image (on top of `cc-deck-base`) with everything pre-installed:

```bash
# API key
podman run -d --name cc-demo -e ANTHROPIC_API_KEY=sk-ant-... quay.io/rhuss/cc-deck-demo:latest
podman exec -it cc-demo zellij --layout cc-deck

# Vertex AI
podman run -d --name cc-demo \
  -e CLAUDE_CODE_USE_VERTEX=1 -e CLOUD_ML_REGION=us-east5 \
  -e ANTHROPIC_VERTEX_PROJECT_ID=my-project \
  -v ~/.config/gcloud:/home/coder/.config/gcloud:ro \
  quay.io/rhuss/cc-deck-demo:latest
podman exec -it cc-demo zellij --layout cc-deck
```

New deliverable: `make demo-image` Makefile target to build `cc-deck-demo`.

## Open Items

- Logo images (user is generating, will upload)
- Demo image Containerfile (derived from base image + cc-deck + Zellij + Claude Code)
- Antora UI bundle customization (colors, logo placement)
