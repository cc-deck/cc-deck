# cc-deck Base Container Image

Fedora Minimal developer toolbox for Claude Code development environments.

## What's Included

- **OS**: Fedora Minimal 41
- **Runtimes**: Node.js 22.x, Python 3.13 + uv
- **Image size**: ~1 GB
- **Shell**: zsh with starship prompt, zoxide, fzf, aliases
- **Version Control**: git, gh (GitHub CLI), glab (GitLab CLI)
- **Search**: ripgrep, fd-find, fzf, jq, yq
- **Modern CLI**: bat (cat), lsd (ls), delta (diff), zoxide (cd)
- **Editors**: helix (hx), vim, nano
- **Network**: curl, wget, htop, netcat, dig/nslookup, ssh/scp
- **Build**: make, sudo, ca-certificates

## What's NOT Included

Zellij, Claude Code, and cc-deck are deliberately excluded. They are added
during user image build (`cc-deck build`) to ensure version consistency.

## Usage

### Pull and run

```bash
podman pull quay.io/cc-deck/cc-deck-base:latest
podman run -it --rm quay.io/cc-deck/cc-deck-base:latest
```

### Build locally

```bash
podman build -t cc-deck-base:local .
```

### Multi-arch build

```bash
podman build --platform linux/amd64 -t cc-deck-base:amd64 .
podman build --platform linux/arm64 -t cc-deck-base:arm64 .
podman manifest create cc-deck-base:latest cc-deck-base:amd64 cc-deck-base:arm64
```

### Use as base for project images

```dockerfile
FROM quay.io/cc-deck/cc-deck-base:latest

# Add project tools
RUN dnf install -y golang && dnf clean all

# Install cc-deck (self-embeds from build context)
COPY cc-deck /usr/local/bin/cc-deck
RUN cc-deck plugin install --install-zellij --force

# Install Claude Code
RUN npm install -g @anthropic-ai/claude-code

USER dev
WORKDIR /home/dev
```

### Verify tools

```bash
podman run --rm cc-deck-base:local sh -c '
  git --version && gh --version && node --version && python3 --version &&
  rg --version && fd --version && bat --version && lsd --version &&
  delta --version && fzf --version && zoxide --version && starship --version &&
  hx --version && jq --version && yq --version && uv --version &&
  echo "All tools OK"
'
```

## User: dev

The image runs as a non-root user `dev` (UID 1000) with:
- Home: `/home/dev`
- Shell: zsh with starship prompt
- Passwordless sudo access
- npm global prefix: `~/.local/lib/npm` (no root needed for `npm install -g`)

## Testing

Run the verification suite (43 checks):

```bash
./test.sh                    # tests cc-deck-base:local
./test.sh <image-name>       # tests a specific image
```

## CI

GitHub Actions workflow (`.github/workflows/base-image.yaml`):
- **Push to main** with `base-image/` changes: rebuild + push `:latest`
- **Release published**: rebuild + push `:latest` and `:vX.Y.Z`
- **Manual dispatch**: rebuild + push `:latest`
- Multi-arch (amd64 + arm64), Trivy vulnerability scan (non-blocking)
