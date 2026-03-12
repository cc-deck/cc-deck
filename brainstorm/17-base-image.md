# 17: cc-deck Base Container Image

## Problem

cc-deck needs a well-equipped base container image for running Claude Code sessions.
The image must include Zellij, Claude Code, and a solid set of developer tools.
Currently, no base image exists. The Containerfile for the base image lives in its own directory
within the cc-deck repo and is published to a registry.

## Base Image Requirements

### Core Runtime
- **Zellij** (latest stable): Terminal multiplexer, required for cc-deck plugin
- **Claude Code** (latest): Installed via npm at build time (changes frequently)
- **cc-deck CLI + WASM plugin**: Embedded from the cc-deck build artifacts
- **Node.js** (LTS): Required for Claude Code runtime

### Essential Developer Tools

| Tool | Purpose |
|------|---------|
| git | Version control |
| gh | GitHub CLI |
| glab | GitLab CLI |
| curl, wget | HTTP clients |
| jq, yq | JSON/YAML processing |
| ripgrep (rg) | Fast search |
| fd-find (fd) | Fast file finder |
| bat | Syntax-highlighted cat |
| eza | Modern ls |
| vim, nano | Text editors |
| htop | Process monitor |
| nc (netcat) | Network debugging |
| dig, nslookup | DNS debugging |
| less, tree | File browsing |
| make | Build automation |
| ssh, scp | Remote access |
| ca-certificates | TLS trust store |

### Base OS Choice

Candidates:
- **Fedora** (latest): Fresh packages, dnf, good container support, familiar to RHEL users
- **Ubuntu LTS** (24.04): Widest ecosystem, apt, most tools available
- **Alpine**: Smallest footprint but musl libc can cause compatibility issues

Recommendation: **Fedora** for freshest packages and RHEL alignment. Falls back to Ubuntu if
any tool has packaging issues on Fedora.

### Non-root User

The image should create a non-root user (`coder` or `claude`) with:
- Home directory at `/home/coder`
- Proper XDG directory structure
- sudo access (for installing additional tools at runtime)
- Default shell: bash

### Image Layering Strategy

```
Layer 1: Base OS + system packages (changes rarely)
Layer 2: Node.js + Claude Code (changes with CC releases)
Layer 3: Zellij + cc-deck artifacts (changes with cc-deck releases)
Layer 4: Developer tools (git, gh, ripgrep, etc.)
Layer 5: Configuration (default layouts, settings)
```

Layer ordering optimizes for rebuild speed: CC updates (most frequent) don't invalidate tool layers.

## Directory Structure

```
cc-deck/
  base-image/
    Containerfile
    scripts/
      install-tools.sh
      install-claude.sh
      setup-user.sh
    config/
      default-layout.kdl
      default-config.kdl
```

## Registry & Versioning

- Published to: `ghcr.io/cc-deck/base` (or similar)
- Tags: `latest`, `vX.Y.Z` (matching cc-deck release), date-based for CI
- Rebuild triggers: cc-deck release, weekly schedule (for Claude Code updates)

## Open Questions

- Should the base image include Python (needed by many MCP servers)?
  Probably yes, as a thin install (python3 + pip/uv) without heavy ML libs.
- Should Zellij be compiled from source or installed from package manager?
  Package manager if available, otherwise GitHub release binary.
- CI pipeline: GitHub Actions? Build on tag + weekly cron?
