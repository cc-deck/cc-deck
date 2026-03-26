# Data Model: cc-deck Base Container Image

**Date**: 2026-03-12
**Branch**: 017-base-image

## Entities

### Base Image

The published multi-arch container image.

| Attribute | Description |
|-----------|-------------|
| Registry | `ghcr.io/rhuss/cc-deck-base` |
| Tags | `latest`, `vX.Y.Z`, `fedora-NN` |
| Architectures | `amd64`, `arm64` |
| Base OS | Fedora (version parameterized) |
| User | `coder` (UID 1000, GID 1000) |

### Coder User

The non-root user inside the container.

| Attribute | Value |
|-----------|-------|
| Username | `coder` |
| UID | 1000 |
| GID | 1000 |
| Home | `/home/coder` |
| Shell | `/bin/zsh` |
| Sudo | Passwordless |
| npm prefix | `~/.local/lib/npm` |

### XDG Directory Structure

| Directory | Path |
|-----------|------|
| XDG_CONFIG_HOME | `~/.config/` |
| XDG_DATA_HOME | `~/.local/share/` |
| XDG_CACHE_HOME | `~/.cache/` |
| XDG_STATE_HOME | `~/.local/state/` |
| npm global | `~/.local/lib/npm/` |
| npm bin (in PATH) | `~/.local/lib/npm/bin/` |

### Tool Categories

| Category | Tools |
|----------|-------|
| Version control | git, gh, glab |
| Search & files | ripgrep, fd-find, fzf, jq, yq, less, tree |
| Modern CLI | bat, lsd, delta, zoxide |
| Prompt | starship |
| Editors | helix, vim, nano |
| Network/system | curl, wget, htop, nmap-ncat, bind-utils, openssh-clients, make, sudo |
| Runtimes | nodejs (22.x), python3, uv |
| Shell | zsh |
| TLS | ca-certificates |

### Shell Configuration

| Component | Configuration |
|-----------|---------------|
| Prompt | starship (via `eval "$(starship init zsh)"`) |
| Directory jumping | zoxide (via `eval "$(zoxide init zsh)"`) |
| Fuzzy finder | fzf (via `source <(fzf --zsh)`) |
| Git pager | delta (via `git config --global core.pager delta`) |
| Aliases | `cat`→`bat`, `ls`→`lsd`, `ll`→`lsd -l`, `la`→`lsd -a` |

### Image Layer Structure

```
FROM fedora:{version}
  ├── Layer 1: System packages (dnf install, single layer)
  ├── Layer 2: Starship (GitHub release download)
  ├── Layer 3: User setup (coder user, XDG, zsh, npm prefix)
  └── Layer 4: Shell config (starship.toml, .zshrc, git config)
```

## Relationships

```
Base Image
  └── contains Coder User
        ├── has XDG Directories
        ├── has Shell Configuration
        └── has Tool Categories (all tools on PATH)
```

## No State Transitions

The base image is a static artifact. There are no state machines or lifecycle
transitions. The image is built, tagged, and pushed. Consumers pull it as-is.
