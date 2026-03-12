#!/bin/bash
set -euo pipefail

USERNAME="coder"
USER_UID=1000
USER_GID=1000

# Create user and group
groupadd -g "$USER_GID" "$USERNAME"
useradd -m -s /bin/zsh -u "$USER_UID" -g "$USER_GID" "$USERNAME"

# Passwordless sudo
echo "$USERNAME ALL=(ALL) NOPASSWD:ALL" > /etc/sudoers.d/$USERNAME
chmod 0440 /etc/sudoers.d/$USERNAME

# XDG directory structure
su - "$USERNAME" -c '
  mkdir -p ~/.config ~/.local/share ~/.local/bin ~/.local/state ~/.cache
'

# npm global prefix (avoids root for npm install -g)
runuser -l "$USERNAME" -c '
  mkdir -p ~/.local/lib/npm
  /usr/bin/npm config set prefix ~/.local/lib/npm
'

# Git config: delta as pager
runuser -l "$USERNAME" -c '
  git config --global core.pager delta
  git config --global delta.navigate true
  git config --global delta.side-by-side false
  git config --global merge.conflictstyle diff3
  git config --global diff.colorMoved default
'
