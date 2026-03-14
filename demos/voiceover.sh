#!/usr/bin/env bash
# Voiceover Generation Script
# Converts narration scripts (text files with chapter markers) to audio
# using the OpenAI TTS API.
#
# Prerequisites:
#   - OPENAI_API_KEY environment variable set
#   - curl installed
#   - ffmpeg installed (for concatenation)
#
# Usage:
#   ./demos/voiceover.sh demos/narration/plugin-demo.txt
#   ./demos/voiceover.sh demos/narration/plugin-demo.txt --voice nova
#   ./demos/voiceover.sh demos/narration/plugin-demo.txt --model tts-1-hd

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RECORDING_DIR="${SCRIPT_DIR}/recordings"

VOICE="${VOICE:-nova}"
MODEL="${MODEL:-tts-1-hd}"
SPEED="${SPEED:-1.0}"

# ─── Argument Parsing ─────────────────────────────────────────────────────────

if [[ $# -lt 1 ]]; then
    echo "Usage: $0 <narration-file> [--voice <voice>] [--model <model>]"
    echo ""
    echo "Voices: alloy, echo, fable, onyx, nova, shimmer"
    echo "Models: tts-1, tts-1-hd"
    exit 1
fi

NARRATION_FILE="$1"
shift

while [[ $# -gt 0 ]]; do
    case "$1" in
        --voice) VOICE="$2"; shift 2 ;;
        --model) MODEL="$2"; shift 2 ;;
        --speed) SPEED="$2"; shift 2 ;;
        *) echo "Unknown option: $1"; exit 1 ;;
    esac
done

if [[ ! -f "$NARRATION_FILE" ]]; then
    echo "Error: narration file not found: $NARRATION_FILE"
    exit 1
fi

if [[ -z "${OPENAI_API_KEY:-}" ]]; then
    echo "Error: OPENAI_API_KEY not set"
    echo "Skipping voiceover generation (graceful degradation)"
    exit 0
fi

# ─── Extract Demo Name ────────────────────────────────────────────────────────

DEMO_NAME="$(basename "$NARRATION_FILE" .txt)"
OUTPUT_DIR="${RECORDING_DIR}/voiceover-parts"
OUTPUT_FILE="${RECORDING_DIR}/${DEMO_NAME}-voiceover.mp3"

mkdir -p "$OUTPUT_DIR"

# ─── Parse Scenes ─────────────────────────────────────────────────────────────

echo "Parsing narration: $NARRATION_FILE"
echo "Voice: $VOICE | Model: $MODEL | Speed: $SPEED"

SCENE_NUM=0
SCENE_TEXT=""
SCENE_NAME=""
CONCAT_LIST="${OUTPUT_DIR}/concat.txt"
> "$CONCAT_LIST"

process_scene() {
    local num="$1"
    local name="$2"
    local text="$3"

    if [[ -z "$text" ]]; then
        return
    fi

    local part_file="${OUTPUT_DIR}/${DEMO_NAME}-part-$(printf '%02d' "$num").mp3"
    echo "  Scene $num: $name"

    # Call OpenAI TTS API
    curl -s \
        -H "Authorization: Bearer ${OPENAI_API_KEY}" \
        -H "Content-Type: application/json" \
        -d "$(jq -n \
            --arg model "$MODEL" \
            --arg voice "$VOICE" \
            --arg input "$text" \
            --arg speed "$SPEED" \
            '{model: $model, voice: $voice, input: $input, speed: ($speed | tonumber)}')" \
        "https://api.openai.com/v1/audio/speech" \
        -o "$part_file"

    if [[ -f "$part_file" && -s "$part_file" ]]; then
        echo "file '${part_file}'" >> "$CONCAT_LIST"
    else
        echo "  Warning: failed to generate audio for scene $num"
    fi
}

while IFS= read -r line || [[ -n "$line" ]]; do
    if [[ "$line" =~ ^##\ scene: ]]; then
        # Process previous scene if any
        if [[ $SCENE_NUM -gt 0 ]]; then
            process_scene "$SCENE_NUM" "$SCENE_NAME" "$SCENE_TEXT"
        fi

        SCENE_NUM=$((SCENE_NUM + 1))
        SCENE_NAME="${line#\#\# scene:}"
        SCENE_TEXT=""
    elif [[ -n "$line" ]]; then
        if [[ -n "$SCENE_TEXT" ]]; then
            SCENE_TEXT="${SCENE_TEXT} ${line}"
        else
            SCENE_TEXT="$line"
        fi
    fi
done < "$NARRATION_FILE"

# Process last scene
if [[ $SCENE_NUM -gt 0 ]]; then
    process_scene "$SCENE_NUM" "$SCENE_NAME" "$SCENE_TEXT"
fi

# ─── Concatenate Parts ────────────────────────────────────────────────────────

if [[ -s "$CONCAT_LIST" ]]; then
    echo ""
    echo "Concatenating ${SCENE_NUM} parts into: $OUTPUT_FILE"
    ffmpeg -y -f concat -safe 0 -i "$CONCAT_LIST" -c copy "$OUTPUT_FILE" 2>/dev/null
    echo "Voiceover saved: $OUTPUT_FILE"

    # Clean up parts
    rm -rf "$OUTPUT_DIR"
else
    echo "Warning: no audio parts generated"
    exit 1
fi
