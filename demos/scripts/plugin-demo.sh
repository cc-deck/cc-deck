#!/usr/bin/env bash
# Plugin Demo - Manual Mode
#
# Run this INSIDE a Zellij session with cc-deck layout.
# Start your screen recording (e.g., macOS screen capture) BEFORE sourcing this.
#
# Usage:
#   1. Start Zellij:  zellij --layout cc-deck
#   2. Start screen recording
#   3. Source this:    source demos/scripts/plugin-demo.sh
#   4. Stop screen recording when done
#
# Each scene pauses and waits for you to press Enter before continuing.
# This gives you control over pacing.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/../runner.sh"

DEMO_DIR="/tmp/cc-deck-demo"

# ─── Helpers ──────────────────────────────────────────────────────────────────

# Wait for the user to press Enter before continuing
proceed() {
    echo ""
    echo ">>> Press Enter to continue to next scene..."
    read -r
}

# ─── Preflight ────────────────────────────────────────────────────────────────

if [[ ! -d "$DEMO_DIR/todo-api" ]]; then
    echo "Demo projects not found. Setting up..."
    "${SCRIPT_DIR}/../projects/setup.sh"
fi

echo ""
echo "=== cc-deck Plugin Demo ==="
echo "Make sure screen recording is running."
echo ""

proceed

# ─── Scene 1: Start First Session ─────────────────────────────────────────────

scene "Start first session (todo-api)"
echo "Opening todo-api project and starting Claude Code..."

run_command "cd ${DEMO_DIR}/todo-api && claude 'Look at the project and add a search endpoint to the TODO API'"

echo "Watch the sidebar pick up the session."
proceed

# ─── Scene 2: Open Second Session ─────────────────────────────────────────────

scene "Open second session (weather-cli)"

new_tab "weather-cli"
pause 1
focus_pane "right"
pause 0.5

run_command "cd ${DEMO_DIR}/weather-cli && claude 'Add a --format json flag to the weather CLI'"

echo "Sidebar now shows two sessions."
proceed

# ─── Scene 3: Open Third Session ──────────────────────────────────────────────

scene "Open third session (portfolio)"

new_tab "portfolio"
pause 1
focus_pane "right"
pause 0.5

run_command "cd ${DEMO_DIR}/portfolio && claude 'Add a dark mode toggle to the portfolio page'"

echo "Three sessions visible in the sidebar."
proceed

# ─── Scene 4: Navigate the Sidebar ────────────────────────────────────────────

scene "Navigate between sessions"
echo "Toggling navigation mode and moving through sessions..."

cc_pipe "nav-toggle"
pause 1.5

cc_pipe "nav-up"
pause 1
cc_pipe "nav-up"
pause 1

echo "Selecting first session..."
cc_pipe "nav-select"

proceed

# ─── Scene 5: Smart Attend ────────────────────────────────────────────────────

scene "Smart attend"
echo "Cycling through sessions that need attention..."

cc_pipe "attend"
pause 2

cc_pipe "attend"
pause 2

cc_pipe "attend"
pause 2

proceed

# ─── Scene 6: Session Management ──────────────────────────────────────────────

scene "Session management"

echo "Toggle navigation, pause a session, show help..."

cc_pipe "nav-toggle"
pause 1

cc_pipe "nav-down"
pause 0.5

echo "Pausing selected session..."
cc_pipe "pause"
pause 2

echo "Showing help overlay..."
cc_pipe "help"
pause 3

echo "Closing help..."
cc_pipe "help"
pause 1

cc_pipe "nav-toggle"
pause 1

proceed

# ─── Done ─────────────────────────────────────────────────────────────────────

echo ""
echo "=== Plugin demo finished ==="
echo "Stop your screen recording now."
echo ""
