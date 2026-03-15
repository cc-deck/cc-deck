#!/usr/bin/env bash
# Quick Demo Recording - ~20 second sidebar demo for README/landing page
#
# Creates a scripted asciinema recording showing cc-deck sidebar features:
# sessions appearing, status changes, smart attend, navigation, pause, help.
#
# Usage (two terminals):
#   Terminal 1:
#     asciinema rec --cols 120 --rows 30 \
#       --command "zellij --session ccdemo --layout cc-deck" \
#       demos/recordings/quick-demo.cast
#
#   Terminal 2 (after Zellij is visible):
#     bash demos/scripts/quick-demo.sh
#
# After recording:
#   agg --theme monokai demos/recordings/quick-demo.cast quick-demo.gif
#
# The script has two phases:
#   1. Setup: creates tabs, discovers pane IDs, clears screens
#   2. Demo: timed sequence of hook events and sidebar commands
#
# Trim the setup from the .cast file by noting the timestamp when
# ">>> Starting demo" appears, then: agg --start <seconds> ...

set -euo pipefail

SESSION="${CCDEMO_SESSION:-ccdemo}"
TMPDIR_DEMO="/tmp/ccdemo-setup"

# ─── Helpers ──────────────────────────────────────────────────────────────────

# Zellij action shorthand
za() { zellij -s "$SESSION" action "$@"; }

# Pipe command to cc-deck plugin
zp() {
    local name="$1"; shift
    if [[ $# -gt 0 ]]; then
        zellij -s "$SESSION" pipe --name "$name" -- "$@"
    else
        zellij -s "$SESSION" pipe --name "$name"
    fi
}

# Inject a hook event for a specific pane
hook() {
    local pane_id="$1"
    local event="$2"
    local cwd="${3:-/tmp}"
    zp "cc-deck:hook" "{\"pane_id\":${pane_id},\"hook_event_name\":\"${event}\",\"cwd\":\"${cwd}\"}"
}

# Type a command in the focused pane and press Enter
type_and_run() {
    za write-chars "$1"
    sleep 0.1
    za write 10
}

# Discover pane ID: type command to write $ZELLIJ_PANE_ID to a temp file
discover_pane() {
    local n="$1"
    type_and_run "echo \$ZELLIJ_PANE_ID > ${TMPDIR_DEMO}/pane-${n}"
    sleep 0.3
}

# Read a discovered pane ID
read_pane() {
    local n="$1"
    cat "${TMPDIR_DEMO}/pane-${n}" 2>/dev/null | tr -d '[:space:]'
}

# ─── Wait for Zellij ──────────────────────────────────────────────────────────

echo ""
echo "=== cc-deck Quick Demo ==="
echo "Waiting for Zellij session '${SESSION}'..."

while ! zellij list-sessions 2>/dev/null | grep -q "$SESSION"; do
    sleep 0.5
done
echo "Session found. Waiting for layout to render..."
sleep 2

# ─── Setup Phase ──────────────────────────────────────────────────────────────

echo "Setting up tabs and discovering pane IDs..."
rm -rf "$TMPDIR_DEMO"
mkdir -p "$TMPDIR_DEMO"

# Tab 1 (already exists from layout, named "main")
# Focus the terminal pane (right of sidebar)
za move-focus right
sleep 0.3
discover_pane 1

# Tab 2: weather-cli
za new-tab --name "weather-cli"
sleep 0.5
# In new tab from template, sidebar is on left, terminal on right
za move-focus right 2>/dev/null || true
sleep 0.2
discover_pane 2

# Tab 3: portfolio
za new-tab --name "portfolio"
sleep 0.5
za move-focus right 2>/dev/null || true
sleep 0.2
discover_pane 3

# Read pane IDs
PANE1=$(read_pane 1)
PANE2=$(read_pane 2)
PANE3=$(read_pane 3)

echo "Pane IDs: tab1=${PANE1}, tab2=${PANE2}, tab3=${PANE3}"

if [[ -z "$PANE1" || -z "$PANE2" || -z "$PANE3" ]]; then
    echo "ERROR: Could not discover all pane IDs."
    echo "ZELLIJ_PANE_ID might not be set. Try Zellij 0.41+."
    echo ""
    echo "Fallback: set pane IDs manually:"
    echo "  PANE1=<id> PANE2=<id> PANE3=<id> bash $0"
    exit 1
fi

# Rename tab 1 to "todo-api"
za go-to-tab 1
sleep 0.3
za rename-tab "todo-api"
sleep 0.3

# Clear all terminal screens (hide setup commands)
for tab in 3 2 1; do
    za go-to-tab "$tab"
    sleep 0.2
    za move-focus right 2>/dev/null || true
    sleep 0.1
    type_and_run "clear"
    sleep 0.2
done

# Back to tab 1
za go-to-tab 1
sleep 0.3

echo ""
echo "=== Setup complete ==="
echo ""
echo "The Zellij window should show 3 clean tabs with an empty sidebar."
echo "Note the timestamp in Terminal 1's asciinema output."
echo ""
echo ">>> Press Enter to start the demo sequence..."
read -r

# ─── Demo Phase (timed sequence) ──────────────────────────────────────────────

echo ">>> Starting demo..."

# ── Sessions appear one by one (Working) ──
# Session 1: todo-api starts working
hook "$PANE1" "SessionStart" "/projects/todo-api"
sleep 0.3
hook "$PANE1" "PreToolUse" "/projects/todo-api"
sleep 1.2

# Session 2: weather-cli starts working
hook "$PANE2" "SessionStart" "/projects/weather-cli"
sleep 0.3
hook "$PANE2" "PreToolUse" "/projects/weather-cli"
sleep 1.2

# Session 3: portfolio starts working
hook "$PANE3" "SessionStart" "/projects/portfolio"
sleep 0.3
hook "$PANE3" "PreToolUse" "/projects/portfolio"
sleep 1.5

# ── Status changes ──
# portfolio finishes (Done ✓)
hook "$PANE3" "Stop" "/projects/portfolio"
sleep 1.2

# todo-api needs permission (red ⚠)
hook "$PANE1" "PermissionRequest" "/projects/todo-api"
sleep 1.5

# ── Smart attend: jumps to permission session ──
zp "cc-deck:attend"
sleep 2

# ── Navigation mode ──
zp "cc-deck:nav-toggle"
sleep 1

# Move cursor down through sessions
zp "cc-deck:nav-down"
sleep 0.7
zp "cc-deck:nav-down"
sleep 1

# ── Pause the session at cursor ──
zp "cc-deck:pause"
sleep 1.5

# ── Help overlay ──
zp "cc-deck:help"
sleep 3

# ── Close help and exit nav mode ──
zp "cc-deck:help"
sleep 0.5
zp "cc-deck:nav-toggle"
sleep 1.5

echo ""
echo "=== Demo complete (about 20 seconds) ==="
echo ""
echo "Stop the recording in Terminal 1 (Ctrl+C or exit Zellij)."
echo ""
echo "Post-processing:"
echo "  # Find the setup/demo boundary timestamp in the .cast file"
echo "  # Convert to GIF (trim first N seconds to skip setup):"
echo "  agg --theme monokai demos/recordings/quick-demo.cast quick-demo.gif"
echo ""
echo "Cleanup:"
echo "  zellij kill-session $SESSION"
echo "  rm -rf $TMPDIR_DEMO"
