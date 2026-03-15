#!/usr/bin/env bash
# Plugin Demo - Hybrid Recording Script
#
# Drives the Zellij session from a SEPARATE terminal while iShowU (or similar)
# records only the Zellij window. Automated parts inject commands via
# `zellij action` and `zellij pipe`. Interactive parts prompt you to perform
# manual keypresses in the Zellij window.
#
# Scene-by-scene mode: pause between scenes so you can start/stop
# your screen recorder for each clip individually.
#
# Usage (from a terminal OUTSIDE Zellij):
#   1. Start Zellij:    zellij --layout cc-deck
#   2. Start iShowU recording the Zellij window
#   3. From another terminal:
#      source demos/scripts/plugin-demo.sh [--scene-by-scene]
#   4. Follow prompts in this terminal; actions happen in Zellij
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
SCENE_BY_SCENE=false
SCENE_COUNTER=0
SCENES_DIR="${SCRIPT_DIR}/../recordings/plugin-demo-scenes"

# Parse arguments
for arg in "$@"; do
    case "$arg" in
        --scene-by-scene) SCENE_BY_SCENE=true ;;
    esac
done

if $SCENE_BY_SCENE; then
    mkdir -p "$SCENES_DIR"
fi

# ─── Helpers ──────────────────────────────────────────────────────────────────

proceed() {
    if $SCENE_BY_SCENE; then
        echo ""
        echo ">>> STOP recording for scene $(printf '%02d' "$SCENE_COUNTER")."
        echo "    Save clip as: plugin-demo-scenes/scene-$(printf '%02d' "$SCENE_COUNTER").mov"
        echo ""
        echo ">>> Press Enter when ready to START recording the next scene..."
        read -r
        SCENE_COUNTER=$((SCENE_COUNTER + 1))
        echo ">>> Recording scene $(printf '%02d' "$SCENE_COUNTER"). Go!"
        pause 1
    else
        echo ""
        echo ">>> Press Enter to continue..."
        read -r
    fi
}

# Prompt for a manual action in the Zellij window
manual() {
    echo ""
    echo "    ACTION: $1"
}

# ─── Preflight ────────────────────────────────────────────────────────────────

if [[ ! -d "$DEMO_DIR/todo-api" ]]; then
    echo "Demo projects not found. Setting up..."
    "${SCRIPT_DIR}/../projects/setup.sh"
fi

echo ""
echo "=== cc-deck Plugin Demo (Hybrid Mode) ==="
echo ""
echo "This terminal drives the demo. Actions happen in your Zellij window."
echo "Keep this terminal visible to you but NOT in the recording area."
echo ""
if $SCENE_BY_SCENE; then
    echo "Mode: scene-by-scene"
    echo "Clips go to: plugin-demo-scenes/"
    echo ""
    echo ">>> Start recording scene 01 in iShowU, then press Enter..."
    SCENE_COUNTER=1
    read -r
    echo ">>> Scene 01 recording. Go!"
    pause 1
else
    echo "Start your screen recorder on the Zellij window, then press Enter."
    read -r
fi

# ═══════════════════════════════════════════════════════════════════════════════
# Scene 1: Launch Zellij (intro shot)
# ═══════════════════════════════════════════════════════════════════════════════

scene "Launch Zellij with cc-deck"
echo "Show the empty Zellij window with the cc-deck sidebar."
echo "Let it sit for a few seconds so viewers see the starting state."
pause 4

proceed

# ═══════════════════════════════════════════════════════════════════════════════
# Scene 2: Start first session
# Automated: types cd + claude command into the focused pane
# ═══════════════════════════════════════════════════════════════════════════════

scene "Start first session (todo-api)"
echo "Injecting command into Zellij pane..."

run_command "cd ${DEMO_DIR}/todo-api && claude 'Look at the project and add a search endpoint to the TODO API'"

echo "Wait until the sidebar shows the session, then proceed."

proceed

# ═══════════════════════════════════════════════════════════════════════════════
# Scene 3: Open second session
# Automated: creates new tab, types cd + claude
# ═══════════════════════════════════════════════════════════════════════════════

scene "Open second session (weather-cli)"
echo "Creating new tab and starting second session..."

new_tab "weather-cli"
pause 1
focus_pane "right"
pause 0.5

run_command "cd ${DEMO_DIR}/weather-cli && claude 'Add a --format json flag to the weather CLI'"

echo "Wait until the sidebar shows two sessions, then proceed."

proceed

# ═══════════════════════════════════════════════════════════════════════════════
# Scene 4: Open third session
# Automated: creates new tab, types cd + claude
# ═══════════════════════════════════════════════════════════════════════════════

scene "Open third session (portfolio)"
echo "Creating new tab and starting third session..."

new_tab "portfolio"
pause 1
focus_pane "right"
pause 0.5

run_command "cd ${DEMO_DIR}/portfolio && claude 'Add a dark mode toggle to the portfolio page'"

echo "Wait until the sidebar shows three sessions, then proceed."

proceed

# ═══════════════════════════════════════════════════════════════════════════════
# Scene 5: Navigate between sessions (MANUAL)
# You perform these actions in the Zellij window
# ═══════════════════════════════════════════════════════════════════════════════

scene "Navigate between sessions"
echo ""
echo "Perform these actions in the Zellij window:"
manual "Press Alt+s to enter navigation mode (amber cursor appears)"
manual "Press k or Up to move cursor up through the list"
manual "Press k again to reach the first session"
manual "Press Enter to jump to that session"
manual "Wait a beat, then press Esc to exit navigation mode"
echo ""
echo ">>> Press Enter here when done..."
read -r

proceed

# ═══════════════════════════════════════════════════════════════════════════════
# Scene 6: Smart attend (MANUAL)
# ═══════════════════════════════════════════════════════════════════════════════

scene "Smart attend in action"
echo ""
echo "Perform these actions in the Zellij window:"
manual "Press Alt+a to smart-attend (jumps to neediest session)"
manual "Wait 2-3 seconds"
manual "Press Alt+a again (cycles to next session)"
manual "Wait 2-3 seconds"
manual "Press Alt+a one more time"
echo ""
echo ">>> Press Enter here when done..."
read -r

proceed

# ═══════════════════════════════════════════════════════════════════════════════
# Scene 7: Session management (MANUAL)
# Pause, rename, help overlay
# ═══════════════════════════════════════════════════════════════════════════════

scene "Session management"
echo ""
echo "Perform these actions in the Zellij window:"
manual "Press Alt+s to enter navigation mode"
manual "Move cursor to a session with j/k"
manual "Press p to pause that session (pause icon appears, text dims)"
manual "Press ? to show the help overlay"
manual "Wait 3-4 seconds for viewers to read the shortcuts"
manual "Press ? again to close help"
manual "Press Esc to exit navigation mode"
echo ""
echo ">>> Press Enter here when done..."
read -r

proceed

# ═══════════════════════════════════════════════════════════════════════════════
# Scene 8: Demo complete (outro shot)
# ═══════════════════════════════════════════════════════════════════════════════

scene "Demo complete"
echo "Let the final state sit for a few seconds as an outro."
pause 4

# ─── Done ─────────────────────────────────────────────────────────────────────

echo ""
echo "=== Plugin demo finished ==="
if $SCENE_BY_SCENE; then
    echo ">>> STOP recording for scene $(printf '%02d' "$SCENE_COUNTER")."
    echo ""
    echo "All clips should be in: $SCENES_DIR/"
    echo ""
    echo "Next steps:"
    echo "  1. Generate voiceover:  ./demos/voiceover.sh demos/narration/plugin-demo.txt --per-scene"
    echo "  2. Assemble:            ./demos/assemble.sh plugin-demo"
else
    echo "Stop your screen recorder now."
fi
echo ""
