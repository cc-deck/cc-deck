#!/usr/bin/env bash
# Plugin Demo - Manual Mode
#
# Run this INSIDE a Zellij session with cc-deck layout.
#
# Two recording modes:
#   Default:          Single continuous recording. Start your screen recorder
#                     before sourcing this script.
#   --scene-by-scene: Pause between scenes for start/stop of individual clips.
#                     Save each clip as scene-01.mov, scene-02.mov, etc. in
#                     demos/recordings/plugin-demo-scenes/
#
# Usage:
#   1. Start Zellij:  zellij --layout cc-deck
#   2. Source this:    source demos/scripts/plugin-demo.sh [--scene-by-scene]
#   3. Follow the prompts
#
# Each scene pauses and waits for you to press Enter before continuing.
# This gives you control over pacing.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/../runner.sh"

DEMO_DIR="/tmp/cc-deck-demo"
SCENE_BY_SCENE=false
SCENE_COUNTER=0
SCENES_DIR="${SCRIPT_DIR}/../recordings/plugin-demo-scenes"

# Parse arguments (when sourced, use $@ from the sourcing command)
for arg in "$@"; do
    case "$arg" in
        --scene-by-scene) SCENE_BY_SCENE=true ;;
    esac
done

if $SCENE_BY_SCENE; then
    mkdir -p "$SCENES_DIR"
fi

# ─── Helpers ──────────────────────────────────────────────────────────────────

# Wait for the user to press Enter before continuing
proceed() {
    if $SCENE_BY_SCENE; then
        echo ""
        echo ">>> STOP recording for scene $(printf '%02d' "$SCENE_COUNTER")."
        echo "    Save clip as: $(basename "$SCENES_DIR")/scene-$(printf '%02d' "$SCENE_COUNTER").mov"
        echo ""
        echo ">>> Press Enter when ready to START recording the next scene..."
        read -r
        SCENE_COUNTER=$((SCENE_COUNTER + 1))
        echo ">>> Recording scene $(printf '%02d' "$SCENE_COUNTER"). Go!"
        pause 1
    else
        echo ""
        echo ">>> Press Enter to continue to next scene..."
        read -r
    fi
}

# ─── Preflight ────────────────────────────────────────────────────────────────

if [[ ! -d "$DEMO_DIR/todo-api" ]]; then
    echo "Demo projects not found. Setting up..."
    "${SCRIPT_DIR}/../projects/setup.sh"
fi

echo ""
echo "=== cc-deck Plugin Demo ==="
if $SCENE_BY_SCENE; then
    echo "Mode: scene-by-scene recording"
    echo "Save clips to: $(basename "$SCENES_DIR")/"
    echo ""
    echo ">>> Start recording scene 01, then press Enter..."
    SCENE_COUNTER=1
    read -r
    echo ">>> Recording scene 01. Go!"
    pause 1
else
    echo "Make sure screen recording is running."
    echo ""
    proceed
fi

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
if $SCENE_BY_SCENE; then
    echo ">>> STOP recording for scene $(printf '%02d' "$SCENE_COUNTER")."
    echo ""
    echo "All clips should be in: $SCENES_DIR/"
    echo "Next steps:"
    echo "  1. Generate voiceover:  ./demos/voiceover.sh demos/narration/plugin-demo.txt --per-scene"
    echo "  2. Assemble:            ./demos/assemble.sh plugin-demo"
else
    echo "Stop your screen recording now."
fi
echo ""
