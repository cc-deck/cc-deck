#!/usr/bin/env bash
# Demo Runner Framework for cc-deck
# Provides helper functions for scripted terminal demos with
# scene management, checkpoint-based waits, and recording integration.

set -euo pipefail

# ─── Configuration ─────────────────────────────────────────────────────────────
DEMO_RUNNER_VERSION="0.1.0"
RECORDING_DIR="${RECORDING_DIR:-$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/recordings}"
RECORDING_PID=""
RECORDING_FILE=""
SCENE_COUNT=0
CURRENT_SCENE=""
VERBOSE="${VERBOSE:-false}"

# ─── Logging ───────────────────────────────────────────────────────────────────

_log() {
    local level="$1"; shift
    local ts
    ts="$(date '+%H:%M:%S')"
    echo "[$ts] [$level] $*" >&2
}

log_info()  { _log "INFO"  "$@"; }
log_warn()  { _log "WARN"  "$@"; }
log_error() { _log "ERROR" "$@"; }
log_debug() { [[ "$VERBOSE" == "true" ]] && _log "DEBUG" "$@" || true; }

# ─── Scene Management ─────────────────────────────────────────────────────────

# Mark the start of a named scene (chapter marker for voiceover alignment).
# Usage: scene "Installing the plugin"
scene() {
    local name="$1"
    SCENE_COUNT=$((SCENE_COUNT + 1))
    CURRENT_SCENE="$name"
    log_info "=== Scene $SCENE_COUNT: $name ==="

    # Write a marker that asciinema captures as a timestamp reference
    if [[ -n "$RECORDING_FILE" ]]; then
        echo "SCENE:${SCENE_COUNT}:${name}" > /dev/null
    fi
}

# ─── Timing and Waits ─────────────────────────────────────────────────────────

# Pause for a configurable number of seconds.
# Usage: pause 2
pause() {
    local seconds="${1:-1}"
    log_debug "Pausing for ${seconds}s"
    sleep "$seconds"
}

# Wait for a pattern to appear in the active terminal pane output.
# Uses checkpoint-based timing rather than fixed sleeps.
# Usage: wait_for "Task complete" 30
wait_for() {
    local pattern="$1"
    local timeout="${2:-60}"
    local interval="${3:-1}"
    local elapsed=0

    log_info "Waiting for pattern: '$pattern' (timeout: ${timeout}s)"

    while (( elapsed < timeout )); do
        # Capture the visible terminal content from the focused pane
        local content
        content="$(zellij action dump-screen /dev/stdout 2>/dev/null || true)"

        if echo "$content" | grep -qF "$pattern"; then
            log_info "Pattern found after ${elapsed}s"
            return 0
        fi

        sleep "$interval"
        elapsed=$((elapsed + interval))
    done

    log_warn "Timed out waiting for pattern: '$pattern' after ${timeout}s"
    return 1
}

# ─── Zellij Integration ───────────────────────────────────────────────────────

# Send a pipe message to the cc-deck plugin.
# Usage: cc_pipe "navigate:toggle"
# Usage: cc_pipe "search" "todo"    (with payload)
cc_pipe() {
    local action="$1"
    local payload="${2:-}"
    log_debug "Pipe: cc-deck:$action${payload:+ payload=$payload}"
    if [[ -n "$payload" ]]; then
        zellij pipe --name "cc-deck:${action}" -- "$payload"
    else
        zellij pipe --name "cc-deck:${action}"
    fi
}

# Type text into the focused pane as if entered from the keyboard.
# Usage: type_command "claude 'fix the tests'"
type_command() {
    local text="$1"
    log_debug "Typing: $text"
    zellij action write-chars "$text"
}

# Press Enter in the focused pane.
press_enter() {
    zellij action write 10
}

# Type a command and press Enter.
# Usage: run_command "ls -la"
run_command() {
    local text="$1"
    type_command "$text"
    pause 0.3
    press_enter
}

# Focus a specific pane by direction.
# Usage: focus_pane "right"
focus_pane() {
    local direction="$1"
    zellij action move-focus "$direction"
}

# Create a new tab with an optional name.
# Usage: new_tab "todo-api"
new_tab() {
    local name="${1:-}"
    if [[ -n "$name" ]]; then
        zellij action new-tab --name "$name"
    else
        zellij action new-tab
    fi
}

# ─── Recording Control ────────────────────────────────────────────────────────

# Start an asciinema recording.
# Usage: start_recording "plugin-demo"
start_recording() {
    local name="$1"
    local cols="${2:-200}"
    local rows="${3:-50}"

    mkdir -p "$RECORDING_DIR"
    RECORDING_FILE="${RECORDING_DIR}/${name}.cast"

    log_info "Starting recording: $RECORDING_FILE"

    asciinema rec \
        --cols "$cols" \
        --rows "$rows" \
        --idle-time-limit 3 \
        --command "sleep infinity" \
        "$RECORDING_FILE" &
    RECORDING_PID=$!

    # Give asciinema time to initialize
    pause 1
}

# Stop the current asciinema recording.
stop_recording() {
    if [[ -n "$RECORDING_PID" ]]; then
        log_info "Stopping recording (PID: $RECORDING_PID)"
        kill "$RECORDING_PID" 2>/dev/null || true
        wait "$RECORDING_PID" 2>/dev/null || true
        RECORDING_PID=""
        log_info "Recording saved: $RECORDING_FILE"
    else
        log_warn "No recording in progress"
    fi
}

# ─── Cleanup ──────────────────────────────────────────────────────────────────

_cleanup() {
    if [[ -n "$RECORDING_PID" ]]; then
        log_warn "Cleaning up: stopping recording"
        stop_recording
    fi
}

trap _cleanup EXIT

# ─── Usage ─────────────────────────────────────────────────────────────────────

# Source this file in your demo scripts:
#   source "$(dirname "$0")/../runner.sh"
#
# Then use the helper functions:
#   scene "Opening the sidebar"
#   cc_pipe "navigate:toggle"
#   pause 2
#   wait_for "todo-api" 10
#   type_command "claude 'add search endpoint'"
