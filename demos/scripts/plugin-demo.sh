#!/usr/bin/env bash
# Plugin Demo - Hybrid Recording Script
#
# Three-part demo:
#   Part 1 (scenes 1-2): Motivation and problem statement
#   Part 2 (scenes 3-8): Live feature walkthrough in Zellij
#   Part 3 (scene 9):    Outlook and teaser for container demo
#
# Drives the Zellij session from a SEPARATE terminal while iShowU (or similar)
# records only the Zellij window. Automated parts inject commands via
# `zellij action` and `zellij pipe`. Interactive parts prompt you to perform
# manual keypresses in the Zellij window.
#
# Usage (from a terminal OUTSIDE Zellij):
#   1. Start Zellij:    zellij --layout cc-deck
#   2. From another terminal:
#      source demos/scripts/plugin-demo.sh [--scene-by-scene]
#   3. Follow prompts in this terminal; actions happen in Zellij
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

manual() {
    echo "    -> $1"
}

# ─── Preflight ────────────────────────────────────────────────────────────────

if [[ ! -d "$DEMO_DIR/todo-api" ]]; then
    echo "Demo projects not found. Setting up..."
    "${SCRIPT_DIR}/../projects/setup.sh"
fi

echo ""
echo "=== cc-deck Plugin Demo (Hybrid Mode) ==="
echo ""
echo "Three parts: (1) Motivation, (2) Features, (3) Outlook"
echo "This terminal drives the demo. Actions happen in your Zellij window."
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
#  PART 1: MOTIVATION
# ═══════════════════════════════════════════════════════════════════════════════

# ─── Scene 1: The problem ────────────────────────────────────────────────────
# Show: Could be a title card, or the Zellij window with multiple messy
# terminals to illustrate tab chaos. Keep it simple.

scene "The problem with multiple sessions"
echo "PART 1: Motivation"
echo ""
echo "Show the problem: multiple terminals, tab confusion."
echo "This scene sets up context for the voiceover."
echo "You can show a messy multi-tab terminal, or just the empty Zellij window."
pause 5

proceed

# ─── Scene 2: What cc-deck solves ────────────────────────────────────────────
# Show: The empty cc-deck sidebar, clean and ready.

scene "What cc-deck solves"
echo "Show the cc-deck sidebar (empty). The voiceover explains the solution."
echo "Let it sit for a few seconds."
pause 5

proceed

# ═══════════════════════════════════════════════════════════════════════════════
#  PART 2: FEATURE WALKTHROUGH
# ═══════════════════════════════════════════════════════════════════════════════

# ─── Scene 3: Launch and start sessions ──────────────────────────────────────
# Automated: create tabs and start Claude Code in each

scene "Launch and start sessions"
echo "PART 2: Feature walkthrough"
echo ""
echo "Creating three sessions automatically..."

# First session in the existing tab
run_command "cd ${DEMO_DIR}/todo-api && claude 'Look at the project and add a search endpoint to the TODO API'"
echo "  Session 1 (todo-api) started. Wait for sidebar to show it."
echo ""
echo ">>> Press Enter when sidebar shows the first session..."
read -r

# Second session
new_tab "weather-cli"
pause 1
focus_pane "right"
pause 0.5
run_command "cd ${DEMO_DIR}/weather-cli && claude 'Add a --format json flag to the weather CLI'"
echo "  Session 2 (weather-cli) started."
echo ""
echo ">>> Press Enter when sidebar shows two sessions..."
read -r

# Third session
new_tab "portfolio"
pause 1
focus_pane "right"
pause 0.5
run_command "cd ${DEMO_DIR}/portfolio && claude 'Add a dark mode toggle to the portfolio page'"
echo "  Session 3 (portfolio) started."
echo ""
echo ">>> Press Enter when sidebar shows three sessions..."
read -r

echo "All three sessions running. Switch between tabs manually to show"
echo "the teal highlight moving in the sidebar."
manual "Click or switch to different tabs a few times"
manual "Let the viewer see the sidebar highlight follow you"
echo ""
echo ">>> Press Enter when done showing tab switching..."
read -r

proceed

# ─── Scene 4: Smart attend ──────────────────────────────────────────────────

scene "Switching tabs and smart attend"
echo "Perform these actions in the Zellij window:"
echo ""
manual "Press Alt+a to smart-attend (jumps to neediest session)"
manual "Wait 2-3 seconds"
manual "Press Alt+a again (cycles to next)"
manual "Wait 2-3 seconds"
manual "Press Alt+a one more time"
manual "Let the viewer see how it prioritizes sessions"
echo ""
echo ">>> Press Enter when done..."
read -r

proceed

# ─── Scene 5: Navigation mode ───────────────────────────────────────────────

scene "Navigation mode"
echo "Perform these actions in the Zellij window:"
echo ""
manual "Press Alt+s to enter navigation mode (amber cursor appears)"
manual "Press j/k or arrow keys to move through the list"
manual "Press g to jump to first, G to jump to last"
manual "Press Enter to switch to the highlighted session"
manual "Press Esc to exit navigation mode"
echo ""
echo ">>> Press Enter when done..."
read -r

proceed

# ─── Scene 6: Rename and pause ──────────────────────────────────────────────

scene "Rename and pause"
echo "Perform these actions in the Zellij window:"
echo ""
echo "  Rename:"
manual "Press Alt+s to enter navigation mode"
manual "Move cursor to a session"
manual "Press r to start renaming"
manual "Type a descriptive name (e.g., 'API search feature')"
manual "Press Enter to confirm"
echo ""
echo "  Pause:"
manual "Move cursor to another session"
manual "Press p to pause it (pause icon appears, text dims)"
manual "Press Esc to exit navigation mode"
echo ""
echo "Note for voiceover: mention that renamed sessions persist across"
echo "Zellij restarts, and paused sessions are skipped by smart attend."
echo ""
echo ">>> Press Enter when done..."
read -r

proceed

# ─── Scene 7: Help and new tabs ─────────────────────────────────────────────

scene "Help and new tabs"
echo "Perform these actions in the Zellij window:"
echo ""
echo "  Help overlay:"
manual "Press Alt+s to enter navigation mode"
manual "Press ? to show help overlay"
manual "Wait 3-4 seconds for viewers to read"
manual "Press ? again to close help"
echo ""
echo "  New tab:"
manual "Press n to create a new tab"
manual "Show that the sidebar picks it up"
manual "Press Esc to exit navigation mode"
echo ""
echo ">>> Press Enter when done..."
read -r

proceed

# ─── Scene 8: Session snapshots ─────────────────────────────────────────────

scene "Session snapshots"
echo "Perform these actions in a Zellij pane (shown in recording):"
echo ""
manual "Type: cc-deck snapshot save"
manual "Show the output confirming sessions were saved"
manual "Optionally show: cc-deck snapshot list"
echo ""
echo "The voiceover explains save/restore workflow."
echo ""
echo ">>> Press Enter when done..."
read -r

proceed

# ═══════════════════════════════════════════════════════════════════════════════
#  PART 3: OUTLOOK
# ═══════════════════════════════════════════════════════════════════════════════

# ─── Scene 9: What comes next ───────────────────────────────────────────────

scene "What comes next"
echo "PART 3: Outlook"
echo ""
echo "This is a closing scene. You can show:"
manual "The final sidebar state with all sessions"
manual "Or a quick flash of a container terminal (optional)"
echo ""
echo "The voiceover teases the container/Podman demo."
echo "Let it sit for a few seconds as an outro."
pause 5

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
