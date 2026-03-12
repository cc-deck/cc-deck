#!/bin/bash
set -euo pipefail

# Install all developer tools via dnf in a single layer
dnf install -y --setopt=install_weak_deps=False \
  git \
  gh \
  glab \
  ripgrep \
  fd-find \
  fzf \
  jq \
  yq \
  bat \
  lsd \
  git-delta \
  zoxide \
  helix \
  vim-enhanced \
  nano \
  curl \
  wget \
  htop \
  nmap-ncat \
  bind-utils \
  openssh-clients \
  make \
  sudo \
  tree \
  less \
  ca-certificates \
  nodejs \
  npm \
  python3 \
  python3-pip \
  uv \
  zsh \
  && dnf clean all \
  && rm -rf /var/cache/dnf

# Install starship from GitHub releases (not available in Fedora repos)
STARSHIP_VERSION="${STARSHIP_VERSION:-latest}"
ARCH=$(uname -m)
case "$ARCH" in
  x86_64)  STARSHIP_ARCH="x86_64-unknown-linux-musl" ;;
  aarch64) STARSHIP_ARCH="aarch64-unknown-linux-musl" ;;
  *)       echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

if [ "$STARSHIP_VERSION" = "latest" ]; then
  STARSHIP_URL="https://github.com/starship/starship/releases/latest/download/starship-${STARSHIP_ARCH}.tar.gz"
else
  STARSHIP_URL="https://github.com/starship/starship/releases/download/v${STARSHIP_VERSION}/starship-${STARSHIP_ARCH}.tar.gz"
fi

echo "Installing starship from ${STARSHIP_URL}"
curl -fsSL "$STARSHIP_URL" | tar xz -C /usr/local/bin
chmod +x /usr/local/bin/starship
starship --version
