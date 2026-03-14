#!/usr/bin/env bash
# Image Builder Demo: creating a bespoke cc-deck image
# Records a scripted demo showing the AI-driven image build pipeline:
# initializing a build directory, reviewing the manifest, and building.
#
# Prerequisites:
#   - cc-deck CLI installed (make install)
#   - podman installed
#   - Demo projects set up (demos/projects/setup.sh)
#   - asciinema installed (for recording)
#
# Usage:
#   ./demos/scripts/image-demo.sh              # Run demo (no recording)
#   RECORD=1 ./demos/scripts/image-demo.sh     # Record to .cast file

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/../runner.sh"

BUILD_DIR="/tmp/cc-deck-image-demo"

# ─── Preflight ────────────────────────────────────────────────────────────────

if ! command -v cc-deck &>/dev/null; then
    log_error "cc-deck CLI not found. Run: make install"
    exit 1
fi

# ─── Recording Setup ─────────────────────────────────────────────────────────

if [[ "${RECORD:-}" == "1" ]]; then
    start_recording "image-demo"
fi

# ─── Scene 1: Initialize Build Directory ─────────────────────────────────────

scene "Initialize image build"

# Clean up any previous run
rm -rf "$BUILD_DIR"

run_command "cc-deck image init ${BUILD_DIR}"
pause 2

run_command "cd ${BUILD_DIR}"
pause 1

# ─── Scene 2: Review the Manifest ────────────────────────────────────────────

scene "Review the manifest"

run_command "cat cc-deck-build.yaml"
pause 4

# ─── Scene 3: Copy Pre-Built Manifest ────────────────────────────────────────

scene "Use demo manifest with tools"

# Copy the pre-built manifest that includes tools for all three demo projects
if [[ -f "${SCRIPT_DIR}/../projects/cc-deck-build.yaml" ]]; then
    cp "${SCRIPT_DIR}/../projects/cc-deck-build.yaml" "${BUILD_DIR}/cc-deck-build.yaml"
fi

run_command "cat cc-deck-build.yaml"
pause 4

# ─── Scene 4: Show Build Commands ────────────────────────────────────────────

scene "AI-driven build commands"

# Show the available slash commands
run_command "ls .claude/commands/"
pause 3

# Show what the extract command does
log_info "In a real session, you would run:"
log_info "  /cc-deck.extract   - analyze repos for dependencies"
log_info "  /cc-deck.settings  - select local config to include"
log_info "  /cc-deck.build     - generate Containerfile and build"
pause 4

# ─── Cleanup ──────────────────────────────────────────────────────────────────

scene "Demo complete"
pause 2

rm -rf "$BUILD_DIR"

# ─── Stop Recording ──────────────────────────────────────────────────────────

if [[ "${RECORD:-}" == "1" ]]; then
    stop_recording
    log_info "Recording saved to: ${RECORDING_DIR}/image-demo.cast"
fi

log_info "Image builder demo finished. ${SCENE_COUNT} scenes recorded."
