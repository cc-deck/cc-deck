#!/usr/bin/env bash
# Setup demo projects for cc-deck recordings.
# Copies project templates to /tmp/cc-deck-demo/ and initializes
# git repositories with realistic commit history.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEMO_DIR="/tmp/cc-deck-demo"

echo "Setting up demo projects in $DEMO_DIR ..."

# Clean any previous setup
if [[ -d "$DEMO_DIR" ]]; then
    echo "Removing existing demo directory ..."
    rm -rf "$DEMO_DIR"
fi

mkdir -p "$DEMO_DIR"

# ─── Helper ────────────────────────────────────────────────────────────────────

# Initialize a demo project with 2 commits to create realistic git history.
# Commit 1: initial project scaffold (README + main files)
# Commit 2: add CLAUDE.md with the pre-staged task
init_project() {
    local name="$1"
    local src="$SCRIPT_DIR/$name"
    local dest="$DEMO_DIR/$name"

    echo "  Setting up $name ..."
    cp -r "$src" "$dest"
    cd "$dest"

    git init -q
    git checkout -q -b main

    # Commit 1: initial project files (everything except CLAUDE.md)
    git add -A
    git reset -- CLAUDE.md >/dev/null 2>&1 || true
    git commit -q -m "Initial project setup

Add project scaffold with core files and README."

    # Commit 2: add the task instructions
    git add CLAUDE.md
    git commit -q -m "Add development task instructions

Define the next feature to implement."

    echo "    Created $name with $(git rev-list --count HEAD) commits"
}

# ─── Projects ─────────────────────────────────────────────────────────────────

init_project "todo-api"
init_project "weather-cli"
init_project "portfolio"

echo ""
echo "Demo projects ready in $DEMO_DIR"
echo "  todo-api/     - Python FastAPI TODO app"
echo "  weather-cli/  - Go weather CLI tool"
echo "  portfolio/    - HTML/CSS portfolio page"
