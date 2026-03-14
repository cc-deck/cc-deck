#!/usr/bin/env bash
# Record a cc-deck demo end-to-end.
#
# Flow:
#   1. Sets up demo projects
#   2. Generates a temporary KDL layout with a command pane running the demo script
#   3. Starts asciinema recording
#   4. Inside recording: launches Zellij with the demo layout
#   5. Zellij runs the demo script automatically in a stacked pane, then exits
#   6. Optionally converts to GIF and MP4
#
# Usage:
#   ./demos/record-demo.sh plugin          # Record plugin demo
#   ./demos/record-demo.sh plugin --gif    # Record + convert to GIF
#   ./demos/record-demo.sh plugin --all    # Record + GIF + voiceover + MP4

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
RECORDING_DIR="${SCRIPT_DIR}/recordings"

DEMO="${1:-plugin}"
shift || true

DO_GIF=false
DO_ALL=false
for arg in "$@"; do
    case "$arg" in
        --gif) DO_GIF=true ;;
        --all) DO_GIF=true; DO_ALL=true ;;
    esac
done

CAST_FILE="${RECORDING_DIR}/${DEMO}-demo.cast"
GIF_FILE="${RECORDING_DIR}/${DEMO}-demo.gif"
MP4_FILE="${RECORDING_DIR}/${DEMO}-demo.mp4"
DEMO_SCRIPT="${SCRIPT_DIR}/scripts/${DEMO}-demo.sh"

# ─── Preflight ────────────────────────────────────────────────────────────────

if [[ ! -f "$DEMO_SCRIPT" ]]; then
    echo "Error: demo script not found: $DEMO_SCRIPT"
    echo "Available demos: plugin, deploy, image"
    exit 1
fi

for cmd in asciinema zellij; do
    if ! command -v "$cmd" &>/dev/null; then
        echo "Error: $cmd not found"
        exit 1
    fi
done

mkdir -p "$RECORDING_DIR"

# ─── Setup Demo Projects ─────────────────────────────────────────────────────

echo "Setting up demo projects..."
"${SCRIPT_DIR}/projects/setup.sh"

# ─── Generate Demo Layout ────────────────────────────────────────────────────
# Create a temporary KDL layout that:
#   - Has the cc-deck sidebar plugin
#   - Has a main working pane (where the demo happens)
#   - Has a hidden "command" pane that runs the demo script
#
# The demo script uses `zellij action` and `zellij pipe` commands which
# operate on the CURRENT session automatically (no -s flag needed).

DEMO_LAYOUT=$(mktemp /tmp/cc-deck-demo-layout-XXXXXX)
mv "$DEMO_LAYOUT" "${DEMO_LAYOUT}.kdl"
DEMO_LAYOUT="${DEMO_LAYOUT}.kdl"

cat > "$DEMO_LAYOUT" << LAYOUT_EOF
layout {
    tab name="demo" focus=true {
        pane split_direction="vertical" {
            pane size=22 borderless=true {
                plugin location="file:$HOME/.config/zellij/plugins/cc_deck.wasm" {
                    mode "sidebar"
                }
            }
            pane focus=true
        }
    }
}
LAYOUT_EOF

# ─── Create Wrapper ──────────────────────────────────────────────────────────
# The wrapper is what asciinema runs as --command.
# It launches Zellij with the demo layout, then a background process
# waits for Zellij to be ready and injects the demo script.

WRAPPER=$(mktemp /tmp/cc-deck-demo-run-XXXXXX)
mv "$WRAPPER" "${WRAPPER}.sh"
WRAPPER="${WRAPPER}.sh"

cat > "$WRAPPER" << WRAPPER_EOF
#!/usr/bin/env bash
set -euo pipefail

# Background: wait for Zellij, then inject demo commands
(
    # Wait for Zellij session to be available
    for i in \$(seq 1 30); do
        if zellij action query-tab-names &>/dev/null 2>&1; then
            break
        fi
        /bin/sleep 1
    done
    /bin/sleep 2

    # Focus the right (main) pane and source the demo script
    zellij action write-chars "source ${DEMO_SCRIPT}"
    /bin/sleep 0.3
    zellij action write 10

    # Wait for demo to complete
    for i in \$(seq 1 300); do
        content=\$(zellij action dump-screen /dev/stdout 2>/dev/null || true)
        if echo "\$content" | grep -q "demo finished"; then
            /bin/sleep 2
            zellij action quit
            exit 0
        fi
        /bin/sleep 1
    done

    # Timeout fallback
    zellij action quit 2>/dev/null || true
) &

# Foreground: launch Zellij (blocks until quit)
exec zellij --layout "${DEMO_LAYOUT}"
WRAPPER_EOF

chmod +x "$WRAPPER"

# ─── Record ──────────────────────────────────────────────────────────────────

echo ""
echo "Recording: $CAST_FILE"
echo "Demo: $DEMO"
echo "Press Ctrl+C to abort."
echo ""

asciinema rec \
    --overwrite \
    --cols 200 \
    --rows 50 \
    --idle-time-limit 3 \
    --command "$WRAPPER" \
    "$CAST_FILE"

rm -f "$WRAPPER" "$DEMO_LAYOUT"

echo ""
echo "Recording saved: $CAST_FILE"

# ─── Convert to GIF ──────────────────────────────────────────────────────────

if $DO_GIF; then
    if ! command -v agg &>/dev/null; then
        echo "Warning: agg not found, skipping GIF conversion"
    else
        echo "Converting to GIF..."
        agg --cols 200 --rows 50 --idle-time-limit 3 --last-frame-duration 5 \
            "$CAST_FILE" "$GIF_FILE"
        echo "GIF saved: $GIF_FILE"
    fi
fi

# ─── Voiceover + MP4 ─────────────────────────────────────────────────────────

if $DO_ALL; then
    NARRATION="${SCRIPT_DIR}/narration/${DEMO}-demo.txt"
    VOICEOVER="${RECORDING_DIR}/${DEMO}-demo-voiceover.mp3"

    if [[ -f "$NARRATION" && -n "${OPENAI_API_KEY:-}" ]]; then
        echo "Generating voiceover..."
        "${SCRIPT_DIR}/voiceover.sh" "$NARRATION"
    fi

    if command -v ffmpeg &>/dev/null && [[ -f "$GIF_FILE" ]]; then
        echo "Converting to MP4..."
        if [[ -f "$VOICEOVER" ]]; then
            ffmpeg -y -i "$GIF_FILE" -i "$VOICEOVER" \
                -c:v libx264 -c:a aac -shortest "$MP4_FILE" 2>/dev/null
        else
            ffmpeg -y -i "$GIF_FILE" -c:v libx264 "$MP4_FILE" 2>/dev/null
        fi
        echo "MP4 saved: $MP4_FILE"
    fi
fi

echo ""
echo "Done. Output files in: $RECORDING_DIR/"
ls -la "$RECORDING_DIR"/${DEMO}-demo.* 2>/dev/null
