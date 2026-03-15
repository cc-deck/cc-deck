#!/usr/bin/env bash
# Screenshot Setup - Interactive Mode
#
# Sets up cc-deck sidebar in specific states for documentation screenshots.
# Run this INSIDE a Zellij session with cc-deck layout that already has
# 3-4 sessions running (use plugin-demo.sh first to set them up).
#
# For each screenshot, the script:
#   1. Arranges the sidebar into the target state via pipe commands
#   2. Pauses so you can capture a screenshot (Cmd+Shift+4 on macOS)
#   3. Resets state before the next screenshot
#
# Usage:
#   1. Start Zellij:  zellij --layout cc-deck
#   2. Create 3-4 sessions (tabs with Claude Code running)
#   3. Source this:    source demos/scripts/screenshot-setup.sh
#   4. Follow prompts to capture each screenshot
#
# Screenshots saved to: docs/modules/using/images/

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/../runner.sh"

PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
SCREENSHOT_DIR="${PROJECT_ROOT}/docs/modules/using/images"

mkdir -p "$SCREENSHOT_DIR"

# ---- Helpers ----------------------------------------------------------------

capture_prompt() {
    local name="$1"
    local description="$2"
    echo ""
    echo "============================================"
    echo "  Screenshot: ${name}"
    echo "  ${description}"
    echo "============================================"
    echo ""
    echo "Capture the sidebar now (Cmd+Shift+4 on macOS)."
    echo "Save as: ${SCREENSHOT_DIR}/${name}"
    echo ""
    echo ">>> Press Enter when done..."
    read -r
}

reset_sidebar() {
    # Exit navigation mode and help if active
    cc_pipe "help" 2>/dev/null || true
    pause 0.3
    cc_pipe "help" 2>/dev/null || true
    pause 0.3
    # Ensure nav mode is off (toggle twice to be safe)
    cc_pipe "nav-toggle" 2>/dev/null || true
    pause 0.3
    cc_pipe "nav-toggle" 2>/dev/null || true
    pause 0.3
}

# ---- Preflight --------------------------------------------------------------

echo ""
echo "=== cc-deck Sidebar Screenshots ==="
echo ""
echo "This script walks through 4 screenshot scenarios."
echo "Make sure you have 3-4 sessions running in different states:"
echo "  - At least one actively working"
echo "  - At least one waiting for permission (or done)"
echo "  - Ideally one paused"
echo ""
echo "If you do not have sessions set up yet, press Ctrl+C and"
echo "run the plugin demo first to create sessions."
echo ""
echo ">>> Press Enter to begin..."
read -r

# ---- Screenshot 1: Sidebar Overview -----------------------------------------
# Goal: Mixed session states (working, permission, idle/done, paused)
# The user should have already arranged sessions in various states.

echo ""
echo "--- Screenshot 1: sidebar-overview.png ---"
echo ""
echo "The sidebar should show sessions in mixed states:"
echo "  - One working (purple dot)"
echo "  - One waiting for permission (red warning)"
echo "  - One idle/done (dim circle or checkmark)"
echo "  - One paused (pause icon, dimmed)"
echo ""
echo "If you need to pause a session:"
echo "  1. Enter navigation mode (Alt+s)"
echo "  2. Move cursor to the session (j/k)"
echo "  3. Press 'p' to pause"
echo "  4. Press Esc to exit nav mode"
echo ""

# Make sure we're not in navigation mode for the overview
reset_sidebar
pause 0.5

capture_prompt "sidebar-overview.png" "Mixed session states (working, permission, done, paused)"

# ---- Screenshot 2: Navigation Mode ------------------------------------------
# Goal: Show cursor highlight and active session teal highlight

echo ""
echo "--- Screenshot 2: sidebar-navigation.png ---"
echo ""
echo "Entering navigation mode to show cursor..."

cc_pipe "nav-toggle"
pause 1

# Move cursor to second session for visual contrast
cc_pipe "nav-down"
pause 0.5

capture_prompt "sidebar-navigation.png" "Navigation mode with amber cursor and teal active highlight"

# Stay in navigation mode for the next screenshots

# ---- Screenshot 3: Help Overlay ----------------------------------------------
# Goal: Show keyboard shortcut overlay

echo ""
echo "--- Screenshot 3: sidebar-help.png ---"
echo ""
echo "Showing help overlay..."

# Exit nav mode first, then show help (help works outside nav mode too)
cc_pipe "nav-toggle"
pause 0.3
cc_pipe "help"
pause 1

capture_prompt "sidebar-help.png" "Help overlay with keyboard shortcuts"

# Close help
cc_pipe "help"
pause 0.5

# ---- Screenshot 4: Search/Filter --------------------------------------------
# Goal: Show filter input with typed text and filtered list

echo ""
echo "--- Screenshot 4: sidebar-search.png ---"
echo ""
echo "Activating search/filter mode..."

# Use the new search pipe to enter filter mode with sample text
cc_pipe "search" "todo"
pause 1

capture_prompt "sidebar-search.png" "Search/filter mode with input at bottom"

# ---- Cleanup -----------------------------------------------------------------

reset_sidebar

echo ""
echo "=== All screenshots captured ==="
echo ""
echo "Screenshots should be saved in:"
echo "  ${SCREENSHOT_DIR}/"
echo ""
echo "Minimum set for docs: sidebar-overview.png and sidebar-help.png"
echo ""
