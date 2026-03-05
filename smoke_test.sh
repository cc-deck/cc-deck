#!/bin/bash
# cc-deck plugin smoke test
#
# Run this INSIDE a Zellij session that has the cc-deck plugin loaded:
#   zellij --layout cc-deck
#   ./smoke_test.sh
#
# Each test sends a pipe message to the plugin and pauses for inspection.

set -e

PLUGIN="file:$HOME/.config/zellij/plugins/cc_deck.wasm"

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
RED='\033[0;31m'
NC='\033[0m'

pass=0
fail=0
skip=0

pause() {
    echo ""
    echo -e "${YELLOW}  >>> Press any key to continue (q to quit)${NC}"
    read -rsn1 key
    if [[ "$key" == "q" ]]; then
        echo ""
        echo -e "${CYAN}Aborted. Passed: $pass, Failed: $fail, Skipped: $skip${NC}"
        exit 0
    fi
    echo ""
}

header() {
    echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${CYAN}  TEST $1: $2${NC}"
    echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
}

# ── Pre-flight ──────────────────────────────────────────

echo -e "${CYAN}cc-deck Plugin Smoke Test${NC}"
echo ""

if ! zellij action write-chars "" 2>/dev/null; then
    echo -e "${RED}ERROR: Not running inside a Zellij session!${NC}"
    echo "Start Zellij first: zellij --layout cc-deck"
    exit 1
fi

if [[ ! -f "$HOME/.config/zellij/plugins/cc_deck.wasm" ]]; then
    echo -e "${RED}ERROR: Plugin not installed!${NC}"
    echo "Run: cc-deck plugin install --force --layout full"
    exit 1
fi

echo -e "${GREEN}Pre-flight OK${NC}: Running inside Zellij, plugin installed."
echo ""
echo "Look at the cc-deck status bar at the bottom of your screen."
echo "It should currently show: cc-deck: no sessions"
pause

# ── Test 1: Plugin status via CLI ───────────────────────

header 1 "CLI plugin status"
echo "Running: cc-deck plugin status"
echo ""
if command -v cc-deck &>/dev/null; then
    cc-deck plugin status
    ((pass++))
elif [[ -f ./cc-deck/cc-deck ]]; then
    ./cc-deck/cc-deck plugin status
    ((pass++))
else
    echo -e "${RED}cc-deck binary not found. Run 'make build' first.${NC}"
    ((fail++))
fi
pause

# ── Test 2: Create a new session ────────────────────────

header 2 "new_session (create a Claude Code tab)"
echo "Sending: zellij pipe --name new_session"
echo ""
echo -e "${YELLOW}EXPECT:${NC} A new tab opens running 'claude'."
echo "        The status bar should show one session."
echo "        (If 'claude' is not on PATH, the tab will open and close.)"
echo ""
zellij pipe --name new_session
((pass++))
pause

# ── Test 3: Create a second session with custom cwd ─────

header 3 "new_session with custom directory"
echo "Sending: zellij pipe --name new_session -- /tmp"
echo ""
echo -e "${YELLOW}EXPECT:${NC} A second tab opens with cwd set to /tmp."
echo "        The status bar should now show two sessions."
echo ""
zellij pipe --name new_session -- /tmp
((pass++))
pause

# ── Test 4: Switch sessions (by index) ──────────────────

header 4 "switch_session_1 (switch to first session)"
echo "Sending: zellij pipe --name switch_session_1"
echo ""
echo -e "${YELLOW}EXPECT:${NC} Focus moves to the first session's tab."
echo "        The status bar highlights session 1."
echo ""
zellij pipe --name switch_session_1
((pass++))
pause

# ── Test 5: Open picker ────────────────────────────────

header 5 "open_picker"
echo "Sending: zellij pipe --name open_picker"
echo ""
echo -e "${YELLOW}EXPECT:${NC} The plugin pane gets focus."
echo "        NOTE: The picker overlay needs rows > 1 to render."
echo "        With size=1, you may only see focus shift."
echo ""
zellij pipe --name open_picker
((pass++))
pause

# Close the picker by sending it again (toggle)
echo "Closing picker (toggle)..."
sleep 1
zellij pipe --name open_picker 2>/dev/null || true

# ── Test 6: Rename session ──────────────────────────────

header 6 "rename_session"
echo "Sending: zellij pipe --name rename_session"
echo ""
echo -e "${YELLOW}EXPECT:${NC} Focus moves to the plugin pane for rename input."
echo "        NOTE: Like the picker, needs rows > 1 to show the prompt."
echo "        Press Escape in the plugin pane to cancel."
echo ""
zellij pipe --name rename_session
((pass++))
pause

# ── Test 7: Close session (with confirm) ────────────────

header 7 "close_session"
echo "Sending: zellij pipe --name close_session"
echo ""
echo -e "${YELLOW}EXPECT:${NC} Focus moves to the plugin pane for close confirmation."
echo "        NOTE: Like the picker, needs rows > 1 to show the prompt."
echo "        Press Escape in the plugin pane to cancel, or 'y' to confirm."
echo ""
zellij pipe --name close_session
((pass++))
pause

# ── Test 8: Claude hook simulation ──────────────────────

header 8 "Claude Code hook messages"
echo "Simulating hook: cc-deck::working::999"
echo ""
echo -e "${YELLOW}EXPECT:${NC} No crash. The message targets pane 999 which"
echo "        doesn't exist, so it should be silently ignored."
echo ""
zellij pipe --name "cc-deck::working::999" 2>/dev/null || true
echo -e "${GREEN}No crash - good.${NC}"
((pass++))
pause

# ── Test 9: Keybinding test ─────────────────────────────

header 9 "Keybinding delivery (manual)"
echo "This test is manual. Try pressing these keybindings:"
echo ""
echo "  Option+Shift+N  - Should create a new session"
echo "  Option+Shift+T  - Should open the picker"
echo "  Option+Shift+R  - Should start rename"
echo "  Option+Shift+X  - Should start close confirmation"
echo ""
echo -e "${YELLOW}If none of these work, keybindings are not reaching Zellij.${NC}"
echo "Use 'zellij pipe --name ...' as an alternative."
echo ""
echo -e "Mark this as ${GREEN}pass${NC} or ${RED}fail${NC}? (p/f/s=skip)"
read -rsn1 key
case "$key" in
    p) ((pass++)); echo -e "${GREEN}Marked as PASS${NC}" ;;
    f) ((fail++)); echo -e "${RED}Marked as FAIL${NC}" ;;
    *) ((skip++)); echo -e "${YELLOW}Skipped${NC}" ;;
esac
pause

# ── Test 10: JSON status output ─────────────────────────

header 10 "CLI plugin status (JSON)"
echo "Running: cc-deck plugin status -o json"
echo ""
if command -v cc-deck &>/dev/null; then
    cc-deck plugin status -o json
    ((pass++))
elif [[ -f ./cc-deck/cc-deck ]]; then
    ./cc-deck/cc-deck plugin status -o json
    ((pass++))
else
    echo -e "${RED}cc-deck binary not found.${NC}"
    ((fail++))
fi
pause

# ── Summary ─────────────────────────────────────────────

echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${CYAN}  SMOKE TEST COMPLETE${NC}"
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
echo -e "  ${GREEN}Passed:  $pass${NC}"
echo -e "  ${RED}Failed:  $fail${NC}"
echo -e "  ${YELLOW}Skipped: $skip${NC}"
echo ""

if [[ $fail -eq 0 ]]; then
    echo -e "${GREEN}All tests passed!${NC}"
else
    echo -e "${RED}Some tests failed. Check the output above for details.${NC}"
fi
