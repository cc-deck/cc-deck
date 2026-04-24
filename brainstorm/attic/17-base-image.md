# 17: cc-deck Base Container Image

## Problem

cc-deck needs a well-equipped base container image as the foundation for
user-specific Claude Code images. The base image is a pure developer toolbox
containing OS packages, runtimes, and CLI tools. It does NOT include Zellij,
Claude Code, or cc-deck, all of which are added during user image build
(brainstorm 18/19) to ensure version consistency.

## Decisions

| Question | Decision | Rationale |
|----------|----------|-----------|
| Base OS | Fedora (latest) | Fresh packages, RHEL alignment, good container support |
| Node.js | `dnf install nodejs` | Fedora 41 ships 22.x, sufficient for CC |
| Python | `python3` + `uv` | Matches project conventions, fast venv management |
| Zellij | NOT in base image | Installed during user build via `cc-deck plugin install --install-zellij` for version compatibility |
| Claude Code | NOT in base image | Installed during user build via `npm install -g` |
| cc-deck | NOT in base image | Self-embeds during user build via `cc-deck plugin install` |
| Multi-arch | amd64 + arm64 | Apple Silicon local dev + mixed-arch k8s clusters |
| Registry | `ghcr.io/rhuss/cc-deck-base` | Personal account for now |
| Location | `base-image/` top-level dir | Independent of Go CLI and Rust plugin |

## Base Image Contents

### Core Runtime

- **Fedora** (latest stable): Base OS
- **Node.js** (LTS via dnf): Required for Claude Code at user image build time
- **Python 3** + **uv**: Required for MCP servers and tooling

### Developer Tools

| Tool | Purpose | Install Method |
|------|---------|----------------|
| git | Version control | dnf |
| gh | GitHub CLI | dnf or GitHub release |
| glab | GitLab CLI | dnf or GitHub release |
| curl, wget | HTTP clients | dnf |
| jq, yq | JSON/YAML processing | dnf |
| ripgrep (rg) | Fast search | dnf |
| fd-find (fd) | Fast file finder | dnf |
| bat | Syntax-highlighted cat | dnf |
| lsd | Modern ls with colors/icons | dnf or GitHub release |
| delta | Git diff pager with syntax highlighting | dnf or GitHub release |
| fzf | Fuzzy finder | dnf or GitHub release |
| zoxide | Smart directory jumping (z) | dnf or GitHub release |
| starship | Cross-shell prompt (git/k8s/python context) | GitHub release |
| helix (hx) | Modern text editor | dnf or GitHub release |
| vim, nano | Fallback text editors | dnf |
| htop | Process monitor | dnf |
| nc (netcat) | Network debugging | dnf |
| dig, nslookup | DNS debugging | dnf (bind-utils) |
| less, tree | File browsing | dnf |
| make | Build automation | dnf |
| ssh, scp | Remote access | dnf (openssh-clients) |
| ca-certificates | TLS trust store | dnf |
| sudo | Privilege escalation | dnf |

### Non-root User

The image creates a non-root user `coder` with:
- Home directory at `/home/coder`
- Proper XDG directory structure (`~/.config/`, `~/.local/`, `~/.cache/`)
- Passwordless sudo access (for installing additional tools at runtime)
- Default shell: **zsh** with starship prompt and zoxide integration
- npm global directory set to `~/.local/lib/npm` (avoids root for npm -g)

## Image Layering Strategy

```
Layer 1: Fedora + system packages (changes rarely)
Layer 2: Node.js via dnf (changes with Fedora version)
Layer 3: Python 3 + uv (changes rarely)
Layer 4: Modern CLI tools (ripgrep, fd, bat, lsd, delta, fzf, zoxide, helix, jq, yq, gh, glab)
Layer 5: Starship prompt + default starship.toml config
Layer 6: Non-root user setup + zsh + XDG dirs + npm config
```

The base image is deliberately minimal. All cc-deck-specific components
(Zellij, Claude Code, cc-deck CLI, WASM plugin) are added during user
image build. This means:
- Base image rebuilds are rare (OS updates, tool additions)
- No weekly rebuild needed for Claude Code updates
- cc-deck version is always consistent with the user's local install
- Zellij version always matches the WASM plugin's target SDK

## What Gets Added During User Image Build

The user image (built via `cc-deck build`, see brainstorm 18/19) layers on:

```
Layer 6:  Zellij (via cc-deck plugin install --install-zellij, version-matched)
Layer 7:  cc-deck CLI + WASM plugin + layouts (self-embedded from local binary)
Layer 8:  Claude Code (npm install -g @anthropic-ai/claude-code)
Layer 9:  cc-setup, cc-session, other GitHub tools
Layer 10: Project-specific tools (from cc-deck.extract analysis)
Layer 11: Zellij config (from local machine or vanilla)
Layer 12: Project config (CLAUDE.md, hooks, settings)
```

### cc-deck Self-Embedding

`cc-deck build` copies its own binary and embeds itself into the user image:
1. Finds its own executable via `os.Executable()`
2. Copies it to the build context
3. The generated Containerfile runs `cc-deck plugin install --install-zellij`
4. This installs the WASM plugin, layouts, AND the matching Zellij binary

The cc-deck binary embeds the compatible Zellij version at compile time:
```go
const ZellijCompatVersion = "0.43.1" // from zellij-tile Cargo.toml
```

### Companion Tools

Always included in user images (downloaded from GitHub releases):
- **cc-setup** (`github.com/rhuss/cc-setup`): Claude Code environment setup
- **cc-session** (`github.com/rhuss/cc-session`): Claude Code session management

Additional tools can be specified in the manifest:
```yaml
# cc-deck-build.yaml
github_tools:
  - repo: rhuss/cc-setup
    binary: cc-setup
  - repo: rhuss/cc-session
    binary: cc-session
  - repo: someorg/other-tool
    binary: other-tool
```

The `cc-deck.containerfile` command generates multi-arch download instructions
using `${TARGETARCH}` for each tool.

### Zellij Config from Local Machine

Users can include their local Zellij configuration in the image:

```yaml
# cc-deck-build.yaml
settings:
  zellij_config: current    # Copy from ~/.config/zellij/
  # zellij_config: vanilla  # Only cc-deck defaults (no local config)
  # zellij_config: /path/to # Custom directory
```

When `current` is selected:
1. Copy everything from `~/.config/zellij/` into the build context
2. Include in the image at `/home/coder/.config/zellij/`
3. Run `cc-deck plugin install --force` which overwrites cc-deck-managed files
   (layouts, WASM plugin) but preserves user's config.kdl, themes, and
   non-cc-deck plugins

## Directory Structure

```
base-image/
  Containerfile
  scripts/
    install-tools.sh        # CLI tools installation
    setup-user.sh           # Non-root user + zsh + XDG setup
  config/
    starship.toml           # Default starship prompt config
    zshrc                   # Base .zshrc (starship init, zoxide init, aliases)
  README.md                 # Usage instructions
```

## Registry & Versioning

- Registry: `ghcr.io/rhuss/cc-deck-base`
- Tags: `latest`, `vX.Y.Z` (matching cc-deck release), `fedora-NN`
- Multi-arch: `amd64` + `arm64` via `podman manifest`
- Rebuild triggers: Fedora version bump, tool updates, manual

## CI Pipeline

```
1. Build for amd64 + arm64 (podman build --platform)
2. Create manifest list (podman manifest create)
3. Push to ghcr.io/rhuss/cc-deck-base
4. Tag: latest + version + date
```

Trigger: manual or on cc-deck release (no weekly cron needed since CC/Zellij
are not in the base image).

## Shell Configuration

The base image includes a default `.zshrc` with:
- Starship prompt initialization (`eval "$(starship init zsh)"`)
- Zoxide initialization (`eval "$(zoxide init zsh)"`)
- Useful aliases: `cat→bat`, `ls→lsd`, `ll→lsd -l`, `la→lsd -a`
- Delta configured as git diff pager (`git config --global core.pager delta`)
- fzf integration (`source <(fzf --zsh)`)

The starship config (`starship.toml`) includes git branch, directory,
python venv, and kubernetes context modules by default.

## Open Questions

- Should `dust` and `hyperfine` be included? They're useful but less
  commonly needed in containers. Leave for user image tools section.
- Should the base `.zshrc` include zsh-autosuggestions and
  zsh-syntax-highlighting plugins? They improve the interactive experience
  but add dependencies.
