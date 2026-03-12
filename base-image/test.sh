#!/bin/bash
# Verify the cc-deck base container image.
# Usage: ./test.sh [image-name]
#   Default image: cc-deck-base:local

set -euo pipefail

# Use podman if available, fall back to docker
if command -v podman >/dev/null 2>&1; then
  RUNTIME=podman
elif command -v docker >/dev/null 2>&1; then
  RUNTIME=docker
else
  echo "Error: neither podman nor docker found"
  exit 1
fi

IMAGE="${1:-cc-deck-base:local}"
PASS=0
FAIL=0

echo "Testing image: $IMAGE (runtime: $RUNTIME)"
echo "========================================"

run_check() {
  local label="$1"
  shift
  if $RUNTIME run --rm "$IMAGE" bash -c "$*" >/dev/null 2>&1; then
    echo "  ✓ $label"
    PASS=$((PASS + 1))
  else
    echo "  ✗ $label"
    FAIL=$((FAIL + 1))
  fi
}

# --- Tools ---
echo ""
echo "Tools:"
TOOLS="git gh glab rg fd fzf jq yq bat lsd delta zoxide starship hx vim nano \
       curl wget htop nc dig ssh make node npm python3 uv zsh"
for cmd in $TOOLS; do
  run_check "$cmd" "command -v $cmd"
done

# --- User setup ---
echo ""
echo "User:"
run_check "user = coder"       '[ "$(whoami)" = "coder" ]'
run_check "uid = 1000"         '[ "$(id -u)" = "1000" ]'
run_check "shell = zsh"        '[ "$SHELL" = "/bin/zsh" ]'
run_check "home = /home/coder" '[ "$HOME" = "/home/coder" ]'
run_check "sudo (no password)" 'sudo -n true'

# --- XDG directories ---
echo ""
echo "XDG directories:"
run_check "~/.config exists"     '[ -d ~/.config ]'
run_check "~/.local exists"      '[ -d ~/.local ]'
run_check "~/.cache exists"      '[ -d ~/.cache ]'
run_check "~/.local/share exists" '[ -d ~/.local/share ]'

# --- npm prefix ---
echo ""
echo "npm configuration:"
run_check "npm prefix = ~/.local/lib/npm" \
  '[ "$(npm config get prefix)" = "/home/coder/.local/lib/npm" ]'
run_check "npm global install works" \
  'export PATH="$HOME/.local/lib/npm/bin:$PATH" && npm install -g cowsay >/dev/null 2>&1 && cowsay test >/dev/null 2>&1'

# --- Shell config ---
echo ""
echo "Shell configuration:"
run_check "starship.toml exists" '[ -f ~/.config/starship.toml ]'
run_check ".zshrc exists"        '[ -f ~/.zshrc ]'
run_check "git pager = delta"    '[ "$(git config --global core.pager)" = "delta" ]'

# --- Image metadata ---
echo ""
echo "Image info:"
SIZE=$($RUNTIME image inspect "$IMAGE" --format '{{.Size}}' 2>/dev/null)
SIZE_MB=$((SIZE / 1024 / 1024))
echo "  Image size: ${SIZE_MB} MB"
if [ "$SIZE_MB" -lt 1536 ]; then
  echo "  ✓ Under 1.5 GB limit"
  PASS=$((PASS + 1))
else
  echo "  ✗ Exceeds 1.5 GB limit"
  FAIL=$((FAIL + 1))
fi

# --- Summary ---
echo ""
echo "========================================"
echo "Results: $PASS passed, $FAIL failed"
echo "========================================"

[ "$FAIL" -eq 0 ] && exit 0 || exit 1
