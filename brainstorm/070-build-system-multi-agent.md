# Brainstorm: Build System Multi-Agent Support

**Date:** 2026-06-06
**Status:** active
**Depends on:** 066-agent-abstraction (core Agent interface), 068-network-policy-generalization, 069-credential-transport-abstraction
**Extracted from:** 022-multi-agent-support

## Problem Framing

The build system (Containerfile generation, manifest processing, image builds) currently only supports Claude Code. Agent binary installation is hardcoded to `curl -fsSL https://claude.ai/install.sh | sh`. Probe checks reference `claude --version`. The base image assumes a single agent. When users want to build workspace images that include multiple agents (e.g., Claude Code + Codex + OpenCode), the build system must support declaring which agents to include, generating multi-agent install stages, and configuring hooks for all included agents.

## Prior Analysis (from brainstorm 022)

### Manifest Extension

The `build.yaml` manifest needs an `agents` section:

```yaml
# build.yaml
version: 3
agents:
  - claude
  - codex
targets:
  container:
    name: my-workspace
    base: quay.io/cc-deck/cc-deck-base:latest
```

### Build Stages per Agent

Each agent contributes to the Containerfile through its Agent interface:

1. **Binary installation** (`InstallScript()`): Each agent knows how to install itself
2. **Hook registration**: `cc-deck plugin install` runs inside the image and auto-detects + hooks all agents
3. **Config scaffolding** (`ConfigPaths()`): Agent config directories that need to exist or be pre-populated
4. **Trust suppression** (`SandboxConfigScript()`): Pre-configure agents for non-interactive use

### Multi-Agent Containerfile Pattern

```dockerfile
# Stage: Install agents (generated from manifest agents list)
RUN curl -fsSL https://claude.ai/install.sh | sh           # claude
RUN npm install -g @openai/codex                            # codex

# Stage: Install cc-deck + hooks for all agents
COPY cc-deck /usr/local/bin/cc-deck
RUN cc-deck plugin install --install-zellij
# Auto-detects claude + codex, installs hooks for both
```

### Installation Safety (from paude analysis)

paude wraps agent install commands in `SHELL ["/bin/bash", "-o", "pipefail", "-c"]` and verifies the binary exists after installation. This prevents silent `curl | sh` failures. The `InstallScript()` method should include similar verification.

### Per-Agent Domain Aliases in Build

Each agent declares `extra_domain_aliases` that get merged into the network policy during build. This means the policy composition depends on which agents are in the manifest.

## Approaches Considered

### A: Agent Interface Drives Build (Recommended)

The Agent interface provides `InstallScript()`, `ConfigPaths()`, `SandboxConfigScript()`, and `ProbeCommands()` methods. The build system iterates the manifest's agents list, looks up each agent in the registry, and calls these methods to generate Containerfile stages.

- Pros: Single source of truth (Go code). Adding a new agent automatically works in builds. Type-safe.
- Cons: Changing an install script requires a cc-deck rebuild.

### B: External Install Scripts

Ship per-agent install scripts as files (`build/agents/claude/install.sh`, `build/agents/codex/install.sh`). The build system copies and runs them.

- Pros: Easy to modify without recompiling. Users can customize.
- Cons: Scripts are a second source of truth alongside the Agent interface. Must keep in sync.

### C: Hybrid

Agent interface provides defaults. Users can override with custom scripts in the manifest.

- Pros: Best of both. Defaults work out of the box, customization possible.
- Cons: Override resolution adds complexity.

## Decision

Approach A (Agent Interface Drives Build) is recommended, consistent with the pure Go interface decision from brainstorm 022 revisit. To be confirmed during specification.

## Key Requirements

- Manifest `agents` field: list of agent names to include in the image
- Default agents list when `agents` is omitted: `["claude"]` for backward compatibility
- Each agent's `InstallScript()` must include pipefail and binary verification
- `cc-deck plugin install` must work inside a container build context (no Zellij session needed for hook installation)
- Probe checks must be generated per agent (not hardcoded to `claude --version`)
- Base image must not assume any specific agent
- Multi-agent images must include the `cc-deck-agent-wrapper` script
- Build time for multi-agent images should be reasonable (parallel agent installs where possible)

## Open Questions

- Should the base image include any agents by default, or should all agents be opt-in via the manifest?
- How do we handle agent version pinning? (e.g., "install Claude Code v2.1.x, not latest")
- Should there be a `cc-deck build agents list` command that shows available agents and their install requirements?
- How do we handle agents that require different base images or OS-level dependencies? (e.g., npm for Codex, Python for some agents)
- Should the build system validate that all agents in the manifest are actually installable before starting the build?
- Exposed ports for web-based agents (from paude analysis): OpenClaw has a web UI. Should the manifest support per-agent port exposure?
