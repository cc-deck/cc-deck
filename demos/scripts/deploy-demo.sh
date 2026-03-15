#!/usr/bin/env bash
# Deploy Demo: cc-deck container deployment
# Records a scripted demo showing how to launch cc-deck in a container,
# start Claude Code sessions, and reconnect to a persistent container.
#
# Prerequisites:
#   - podman installed
#   - cc-deck demo image built (make demo-image)
#   - asciinema installed (for recording)
#
# Usage:
#   ./demos/scripts/deploy-demo.sh              # Run demo (no recording)
#   RECORD=1 ./demos/scripts/deploy-demo.sh     # Record to .cast file

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/../runner.sh"

CONTAINER_NAME="cc-deck-demo-$$"
DEMO_IMAGE="${DEMO_IMAGE:-quay.io/cc-deck/cc-deck-demo:latest}"

# ─── Preflight ────────────────────────────────────────────────────────────────

if ! command -v podman &>/dev/null; then
    log_error "podman not found"
    exit 1
fi

# ─── Recording Setup ─────────────────────────────────────────────────────────

if [[ "${RECORD:-}" == "1" ]]; then
    start_recording "deploy-demo"
fi

# ─── Scene 1: Interactive Launch ──────────────────────────────────────────────

scene "One-liner interactive launch"

# Show the one-liner command
type_command "podman run -it --rm -e ANTHROPIC_API_KEY=\$ANTHROPIC_API_KEY ${DEMO_IMAGE}"
pause 3

# Don't actually run it (would take over the terminal). Show the command only.
# Clear the line
type_command ""
press_enter
pause 1

# ─── Scene 2: Persistent Container ───────────────────────────────────────────

scene "Launch persistent container"

run_command "podman run -d --name ${CONTAINER_NAME} -e ANTHROPIC_API_KEY=\$ANTHROPIC_API_KEY ${DEMO_IMAGE} sleep infinity"
pause 2

# ─── Scene 3: Connect and Start Working ──────────────────────────────────────

scene "Connect to container"

run_command "podman exec -it ${CONTAINER_NAME} zellij --layout cc-deck"
pause 3

# ─── Scene 4: Start Claude Code in Container ─────────────────────────────────

scene "Start Claude Code sessions"

run_command "claude 'Hello, show me what tools are available'"
pause 5

wait_for "●" 20 || log_warn "Claude Code may not have started"
pause 3

# ─── Scene 5: Disconnect and Reconnect ────────────────────────────────────────

scene "Disconnect and reconnect"

# Exit Zellij (detach)
pause 2

# Show reconnect command
log_info "Demonstrating reconnect flow"
pause 2

# ─── Cleanup ──────────────────────────────────────────────────────────────────

scene "Demo complete"
pause 2

# Stop and remove the container
podman rm -f "${CONTAINER_NAME}" &>/dev/null || true

# ─── Stop Recording ──────────────────────────────────────────────────────────

if [[ "${RECORD:-}" == "1" ]]; then
    stop_recording
    log_info "Recording saved to: ${RECORDING_DIR}/deploy-demo.cast"
fi

log_info "Deploy demo finished. ${SCENE_COUNT} scenes recorded."
