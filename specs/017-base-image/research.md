# Research: cc-deck Base Container Image

**Date**: 2026-03-12
**Branch**: 017-base-image

## Tool Availability in Fedora 41 (x86_64 + aarch64)

### All Available via `dnf install`

| Tool | Fedora Package | Notes |
|------|----------------|-------|
| git | git | |
| gh | gh | GitHub CLI |
| glab | glab | GitLab CLI |
| ripgrep | ripgrep | Provides `rg` |
| fd-find | fd-find | Provides `fd` |
| fzf | fzf | |
| jq | jq | |
| yq | yq | v4.47.1 |
| bat | bat | |
| lsd | lsd | |
| delta | git-delta | Package name is `git-delta` |
| zoxide | zoxide | |
| helix | helix | Provides `hx` |
| curl | curl | |
| wget | wget | |
| htop | htop | |
| netcat | nmap-ncat | Package name is `nmap-ncat` |
| dig/nslookup | bind-utils | |
| ssh/scp | openssh-clients | |
| make | make | |
| sudo | sudo | |
| tree | tree | |
| less | less | |
| vim | vim-enhanced | Full vim, not vim-minimal |
| nano | nano | |
| ca-certificates | ca-certificates | |
| nodejs | nodejs | v22.x LTS in Fedora 41 |
| python3 | python3 | |
| uv | uv | v0.9.7 in Fedora 41 repos |
| zsh | zsh | |

### Requires GitHub Release or COPR

| Tool | Install Method | Rationale |
|------|---------------|-----------|
| starship | GitHub release binary | Not in official Fedora repos. COPR (atim/starship) available but adds external repo dependency. GitHub release is simpler for a container build. |

**Decision**: Install starship from GitHub releases. Binary pattern:
- amd64: `starship-x86_64-unknown-linux-musl.tar.gz`
- arm64: `starship-aarch64-unknown-linux-musl.tar.gz`

## Key Technical Decisions

### Decision 1: Single `dnf install` for most tools

**Decision**: Use a single `dnf install` layer for all dnf-available tools.
**Rationale**: Minimizes image layers and total image size. Package cache is cleaned in the same layer.
**Alternatives**: Separate layers per tool category (rejected: increases image size with no benefit).

### Decision 2: Starship via GitHub release

**Decision**: Download starship binary from GitHub releases during build.
**Rationale**: No COPR dependency, simpler and more predictable. Binary is statically linked (musl).
**Alternatives**: COPR repo (rejected: adds external repo trust, may lag behind).

### Decision 3: npm prefix via `.npmrc`

**Decision**: Set npm prefix to `~/.local/lib/npm` via `.npmrc` in the coder user's home.
Add `~/.local/lib/npm/bin` to PATH.
**Rationale**: Standard npm configuration, survives across shell sessions.
**Alternatives**: Environment variable only (rejected: less persistent).

### Decision 4: Fedora version as build ARG

**Decision**: Use `ARG FEDORA_VERSION=41` at the top of the Containerfile.
**Rationale**: Easy to bump for new Fedora releases, visible in build logs.
**Alternatives**: Hardcoded (rejected: harder to maintain).

### Decision 5: Multi-arch via separate builds + manifest

**Decision**: Build separate images per arch, combine with `podman manifest`.
**Rationale**: Standard approach for multi-arch container images. Works with GitHub Actions and local builds.
**Alternatives**: QEMU cross-compilation (rejected: slower, less reliable for native packages).
