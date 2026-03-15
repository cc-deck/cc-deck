#!/usr/bin/env bash
# Plugin Demo - Teleprompter + Session Setup
#
# Run from a SEPARATE terminal while recording the Zellij window.
# Automates session creation (scenes 1-3) and shows prompts for
# manual actions during the rest of the demo.
#
# Usage (from a terminal OUTSIDE Zellij):
#   1. Start Zellij:    zellij --layout cc-deck
#   2. Start screen recording on the Zellij window
#   3. From another terminal:
#      source demos/scripts/plugin-demo.sh
#   4. Follow prompts, press Enter to advance
#
# Designed to be sourced from both bash and zsh.

# Portable script directory detection (bash and zsh)
if [[ -n "${ZSH_VERSION:-}" ]]; then
    SCRIPT_DIR="${0:A:h}"
else
    SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
fi
source "${SCRIPT_DIR}/../runner.sh"

DEMO_DIR="/tmp/cc-deck-demo"

# ─── Helpers ──────────────────────────────────────────────────────────────────

# Quick-continue prompt (just press Enter)
next() {
    read -r
}

manual() {
    echo "    -> $1"
}

# ─── Preflight ────────────────────────────────────────────────────────────────

if [[ ! -d "$DEMO_DIR/todo-api" ]]; then
    echo "Demo projects not found. Setting up..."
    "${SCRIPT_DIR}/../projects/setup.sh"
fi

echo ""
echo "=== cc-deck Plugin Demo ==="
echo "Press Enter to advance. Actions happen in the Zellij window."
echo ""
next

# ═══════════════════════════════════════════════════════════════════════════════
#  PART 1: MOTIVATION (scenes 1-2)
# ═══════════════════════════════════════════════════════════════════════════════

echo "--- Scene 1: The problem with multiple sessions ---"
echo "Show tab chaos / empty Zellij sidebar"
next

echo "--- Scene 2: What cc-deck solves ---"
echo "Show the cc-deck sidebar, explain the concept"
next

# ═══════════════════════════════════════════════════════════════════════════════
#  PART 2: FEATURES (scenes 3-9)
# ═══════════════════════════════════════════════════════════════════════════════

echo "--- Scene 3: Launch and start sessions ---"
echo "Starting session 1 (todo-api)..."
run_command "cd ${DEMO_DIR}/todo-api && claude 'Look at the project and add a search endpoint to the TODO API'"
next

echo "Starting session 2 (weather-cli)..."
new_tab "weather-cli"
pause 1
focus_pane "right"
pause 0.5
run_command "cd ${DEMO_DIR}/weather-cli && claude 'Add a --format json flag to the weather CLI'"
next

echo "Starting session 3 (portfolio)..."
new_tab "portfolio"
pause 1
focus_pane "right"
pause 0.5
run_command "cd ${DEMO_DIR}/portfolio && claude 'Add a dark mode toggle to the portfolio page'"
echo "3 sessions running. Switch tabs to show teal highlight."
next

echo "--- Scene 4: Smart attend ---"
manual "Alt+a (attend) x3, show priority cycling"
next

echo "--- Scene 5: Navigation mode ---"
manual "Alt+s, j/k movement, g/G, Enter to select, Esc"
next

echo "--- Scene 6: Rename and pause ---"
manual "Alt+s, move to session, r to rename, type name, Enter"
manual "Move to another session, p to pause"
manual "Esc to exit"
next

echo "--- Scene 7: Help and new tabs ---"
manual "Alt+s, ? for help overlay, wait, ? to close"
manual "n for new tab, Esc"
next

echo "--- Scene 8: Session snapshots ---"
manual "cc-deck snapshot list"
manual "cc-deck snapshot save"
next

echo "--- Scene 9: Restore sessions ---"
manual "Exit Zellij, restart, cc-deck snapshot restore"
next

# ═══════════════════════════════════════════════════════════════════════════════
#  PART 3: OUTLOOK (scene 10)
# ═══════════════════════════════════════════════════════════════════════════════

echo "--- Scene 10: What comes next ---"
echo "Closing shot. Tease Podman/container demo."
next

echo ""
echo "=== Demo finished ==="
