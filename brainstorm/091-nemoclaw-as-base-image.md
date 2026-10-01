# Brainstorm: NemoClaw as Base Image for cc-deck OpenShell Sandboxes

**Date:** 2026-08-12
**Status:** draft
**Related:** [01-cc-deck-openshell-backend](01-cc-deck-openshell-backend.md)

## Context

cc-deck currently builds its own sandbox images from a Fedora base (`cc-deck-base`) that includes developer tools, then layers Zellij, Claude Code, and cc-session on top (`cc-deck-demo`). These images are designed for the Podman/Docker compute driver and assume direct network access for inference.

NemoClaw is NVIDIA's pre-built agent sandbox image for OpenShell. It ships with OpenClaw (the agent gateway), a curated plugin system, built-in inference routing via `inference.local`, web search plugins, and diagnostics-otel for OTel trace export. NemoClaw images are built from the NVIDIA/NemoClaw Dockerfile, which has a sophisticated multi-stage build with SRI-verified plugin installation, security hardening (Landlock, config integrity, privilege separation), and a build-arg system that configures the image for different deployment targets.

The SAW (Secure Agent Workspace) validated pattern builds NemoClaw images on OpenShift via Helm-managed BuildConfigs, passing model, inference, and plugin configuration as build args.

## Question

Should cc-deck use NemoClaw as its base image when targeting OpenShell sandboxes, instead of building its own Fedora-based image?

## Current cc-deck Image Stack

```
Fedora 41 (registry.fedoraproject.org/fedora:41)
  └── cc-deck-base (dev tools, starship, git, ripgrep, etc.)
        └── cc-deck-demo (Zellij, Claude Code, cc-session, cc-setup)
```

Built via `podman build`, published to `quay.io/cc-deck/`.

## NemoClaw Image Stack

```
ghcr.io/nvidia/nemoclaw/sandbox-base:latest
  └── NemoClaw Dockerfile (multi-stage, ~1900 lines)
        - OpenClaw runtime + 70 bundled plugins
        - nemoclaw-start entrypoint (security hardening, config generation)
        - Inference routing via inference.local
        - Plugin system (diagnostics-otel, brave, etc.)
        - NEMOCLAW_* build args for deployment customization
```

Built via OpenShift BuildConfig or local Docker build.

## Two Directions

### A: NemoClaw as base, cc-deck layers on top

```
NemoClaw sandbox image (with OpenClaw, inference routing, plugins)
  └── cc-deck layer (Zellij, cc-session, workspace config)
```

Advantages:
- Agent functionality comes free (chat UI, plugin system, tool search, model routing)
- `diagnostics-otel` is already handled (build arg `NEMOCLAW_OPENCLAW_OTEL=1`)
- Inference routing works out of the box via `inference.local`
- Security hardening (config integrity, privilege separation) is baked in
- cc-deck just adds the terminal multiplexer and session management

Concerns:
- NemoClaw images are large (~2GB+ with OpenClaw runtime, node_modules, 70 plugins)
- cc-deck would depend on NemoClaw's release cadence and build system
- The entrypoint (`nemoclaw-start`) is complex and assumes it controls PID 1. Running Zellij as the agent process may conflict with NemoClaw's expectations
- cc-deck's developer tooling (ripgrep, fzf, starship) would need to be layered on top of a Debian-based NemoClaw image, not a Fedora one

### B: Keep separate images, share OpenShell infrastructure

```
cc-deck-base (Fedora, dev tools)
  └── cc-deck-demo (Zellij, Claude Code, cc-session)
        + openshell sandbox policy
        + openshell inference route
```

Advantages:
- cc-deck controls its own base, tooling, and image size
- Simpler, no dependency on NemoClaw release cycle
- Works with any OpenShell compute driver without NemoClaw-specific assumptions
- Can still use OpenShell's inference routing, policy, and OTEL independently

Concerns:
- No OpenClaw agent UI (but cc-deck has its own TUI)
- Need to build OTel trace export separately if wanted
- Missing NemoClaw's plugin ecosystem (web search, memory, etc.)

### C: NemoClaw for OpenShell, cc-deck-base for local

Use NemoClaw when deploying to OpenShell-managed sandboxes (K8s, VM driver), use cc-deck-base for local Podman/Docker development. cc-deck detects which backend is active and selects the appropriate image.

Advantages:
- Best of both worlds: NemoClaw's agent features when deployed to SAW/OpenShell, cc-deck's lean image for local dev
- Maps to the existing cc-deck workspace backend abstraction
- Doesn't force NemoClaw on users who just want a local sandboxed Claude session

Concerns:
- Two image maintenance tracks
- Behavioral differences between environments (plugins available in one, not the other)

## NemoClaw Build System

NemoClaw's build system is worth understanding regardless of which direction we choose:

- **Dockerfile**: ~1900-line multi-stage build. Stages: base prep, OpenClaw runtime install, config generation, plugin installation, security hardening
- **Build args**: `NEMOCLAW_MODEL`, `NEMOCLAW_INFERENCE_BASE_URL`, `NEMOCLAW_OPENCLAW_OTEL`, `NEMOCLAW_MANAGED_IMAGE_CAPABILITY_UNION`, etc. These configure the image for a specific deployment target
- **Plugin installation**: Uses pre-fetched, SRI-verified tarballs (`--network=none` build stage). No npm registry access at runtime. Plugins are installed via `openclaw plugins install "npm-pack:..."` which creates proper install records for the trust system
- **`NEMOCLAW_MANAGED_IMAGE_CAPABILITY_UNION=1`**: Builds a "neutral" image with all optional plugins pre-installed but none activated. Runtime config decides what turns on. This is how release images are published
- **SAW builds**: The SAW validated pattern wraps this in Helm-managed OpenShift BuildConfigs. Build args are passed through Helm values. The process is: `helm upgrade --install` (creates BuildConfig) then `oc start-build` (runs the Docker build on the cluster)

The key insight from our SAW work: NemoClaw's build system is designed for operators who publish pre-configured images. Individual users don't run the build. The published images at `quay.io/rh-ai-quickstart/` are ready to use.

## Open Questions

1. **Entrypoint conflict**: NemoClaw's `nemoclaw-start` is a 6000-line bash script that expects to control the container lifecycle. How does Zellij fit as the "agent command" when `nemoclaw-start` manages the OpenClaw gateway, token generation, config integrity, and process supervision?

2. **Image size vs. capability**: NemoClaw images include 70 OpenClaw plugins, most disabled. Is the image size acceptable for cc-deck's use case, or should we build a slimmer variant?

3. **Dev tools**: NemoClaw's base is Debian-derived. cc-deck's base is Fedora. The dev tool expectations (modern coreutils, zsh plugins, starship) are built for Fedora. How much friction would switching cause?

4. **Inference routing**: cc-deck currently expects the user to provide `ANTHROPIC_API_KEY` directly. NemoClaw routes through `inference.local` via the OpenShell gateway. These are fundamentally different models. Can cc-deck support both transparently?

5. **Published images**: Could cc-deck consume NemoClaw's published release images directly (e.g., `quay.io/rh-ai-quickstart/nemoclaw-sandbox:latest`) instead of building custom ones? What customization would be lost?
