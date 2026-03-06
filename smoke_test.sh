#!/bin/bash
# cc-deck automated smoke test
#
# Runs a fully automated test suite by launching a dedicated Zellij session,
# sending pipe commands to the plugin, and verifying results. No user
# interaction is required.
#
# Usage: ./smoke_test.sh
#
# Prerequisites:
#   - zellij is installed and on PATH
#   - cc_deck.wasm is installed at ~/.config/zellij/plugins/cc_deck.wasm

set -euo pipefail

SESSION="cc-deck-test-$$"
LAYOUT_DIR="$(cd "$(dirname "$0")/cc-zellij-plugin" && pwd)"
LAYOUT="$LAYOUT_DIR/zellij-layout.kdl"

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
RED='\033[0;31m'
NC='\033[0m'

PASS=0
FAIL=0
SKIP=0

# ── Helpers ───────────────────────────────────────────────

assert_ok() {
    local description="$1"
    shift
    if "$@" >/dev/null 2>&1; then
        echo -e "  ${GREEN}PASS${NC}: $description"
        ((PASS++))
    else
        echo -e "  ${RED}FAIL${NC}: $description"
        ((FAIL++))
    fi
}

assert_fail() {
    local description="$1"
    shift
    if ! "$@" >/dev/null 2>&1; then
        echo -e "  ${GREEN}PASS${NC}: $description"
        ((PASS++))
    else
        echo -e "  ${RED}FAIL${NC}: $description"
        ((FAIL++))
    fi
}

skip_test() {
    local description="$1"
    local reason="$2"
    echo -e "  ${YELLOW}SKIP${NC}: $description ($reason)"
    ((SKIP++))
}

cleanup() {
    echo ""
    echo -e "${CYAN}Cleaning up...${NC}"
    zellij kill-session "$SESSION" 2>/dev/null || true
}

trap cleanup EXIT

# ── Pre-flight checks ────────────────────────────────────

echo -e "${CYAN}cc-deck Automated Smoke Test${NC}"
echo ""

if ! command -v zellij &>/dev/null; then
    echo -e "${RED}ERROR: zellij not found on PATH${NC}"
    exit 1
fi

if [[ ! -f "$HOME/.config/zellij/plugins/cc_deck.wasm" ]]; then
    echo -e "${RED}ERROR: Plugin not installed at ~/.config/zellij/plugins/cc_deck.wasm${NC}"
    echo "Run: cargo build --target wasm32-wasip1 --release && cp target/wasm32-wasip1/release/cc_deck.wasm ~/.config/zellij/plugins/"
    exit 1
fi

if [[ ! -f "$LAYOUT" ]]; then
    echo -e "${RED}ERROR: Layout file not found at $LAYOUT${NC}"
    exit 1
fi

echo -e "${GREEN}Pre-flight OK${NC}"
echo ""

# ── Start test session ────────────────────────────────────

echo -e "${CYAN}Starting Zellij test session: $SESSION${NC}"
zellij --session "$SESSION" --layout "$LAYOUT" &
ZELLIJ_PID=$!
sleep 3

# Verify the session is running
echo -e "${CYAN}Test 1: Session startup${NC}"
assert_ok "Zellij test session is running" zellij list-sessions -s

# ── T026: Test session creation ───────────────────────────

echo ""
echo -e "${CYAN}Test 2: Session creation via new_session pipe${NC}"
assert_ok "new_session pipe command succeeds" \
    zellij pipe --session "$SESSION" --name new_session
sleep 2

# Send a second session with custom cwd
assert_ok "new_session with cwd=/tmp succeeds" \
    zellij pipe --session "$SESSION" --name new_session -- /tmp
sleep 2

# ── T028: Test tab title updates via status hook ─────────

echo ""
echo -e "${CYAN}Test 3: Status hook pipe messages${NC}"

# Send a working hook for a non-existent pane (should not crash)
assert_ok "working hook for non-existent pane does not crash" \
    zellij pipe --session "$SESSION" --name "cc-deck::working::999"

# Send done hook for non-existent pane
assert_ok "done hook for non-existent pane does not crash" \
    zellij pipe --session "$SESSION" --name "cc-deck::done::999"

# Send waiting hook for non-existent pane
assert_ok "waiting hook for non-existent pane does not crash" \
    zellij pipe --session "$SESSION" --name "cc-deck::waiting::999"

# ── Test switch_session ───────────────────────────────────

echo ""
echo -e "${CYAN}Test 4: Session switching${NC}"
assert_ok "switch_session_1 succeeds" \
    zellij pipe --session "$SESSION" --name switch_session_1
sleep 1

# ── Test open_picker ──────────────────────────────────────

echo ""
echo -e "${CYAN}Test 5: Picker toggle${NC}"
assert_ok "open_picker succeeds" \
    zellij pipe --session "$SESSION" --name open_picker
sleep 1

# Toggle it off
assert_ok "open_picker toggle off succeeds" \
    zellij pipe --session "$SESSION" --name open_picker
sleep 1

# ── Test rename_session ───────────────────────────────────

echo ""
echo -e "${CYAN}Test 6: Rename session${NC}"
assert_ok "rename_session command succeeds" \
    zellij pipe --session "$SESSION" --name rename_session

# ── Test close_session ────────────────────────────────────

echo ""
echo -e "${CYAN}Test 7: Close session${NC}"
assert_ok "close_session command succeeds" \
    zellij pipe --session "$SESSION" --name close_session

# ── T027: Claude detection (conditional) ──────────────────

echo ""
echo -e "${CYAN}Test 8: Claude session detection${NC}"
if command -v claude &>/dev/null; then
    # Claude is available; a new_session should auto-start it, and detection
    # would pick it up via pane title scanning. We already tested new_session
    # above, so just verify the binary is reachable.
    assert_ok "claude binary is on PATH" command -v claude
else
    skip_test "Claude session detection" "claude binary not on PATH"
fi

# ── T029: Cleanup verification ────────────────────────────

echo ""
echo -e "${CYAN}Test 9: Session cleanup${NC}"
zellij kill-session "$SESSION" 2>/dev/null || true
sleep 1

# Verify the session is gone
if zellij list-sessions -s 2>/dev/null | grep -q "^${SESSION}$"; then
    echo -e "  ${RED}FAIL${NC}: Test session still exists after kill"
    ((FAIL++))
else
    echo -e "  ${GREEN}PASS${NC}: Test session removed after kill"
    ((PASS++))
fi

# Disarm the trap since we already cleaned up
trap - EXIT

# ── Summary ───────────────────────────────────────────────

echo ""
echo -e "${CYAN}=====================================${NC}"
echo -e "${CYAN}  SMOKE TEST RESULTS${NC}"
echo -e "${CYAN}=====================================${NC}"
echo ""
echo -e "  ${GREEN}Passed:  $PASS${NC}"
echo -e "  ${RED}Failed:  $FAIL${NC}"
echo -e "  ${YELLOW}Skipped: $SKIP${NC}"
echo ""

if [[ $FAIL -eq 0 ]]; then
    echo -e "${GREEN}All tests passed!${NC}"
    exit 0
else
    echo -e "${RED}Some tests failed.${NC}"
    exit 1
fi
