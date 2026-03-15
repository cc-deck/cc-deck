#!/usr/bin/env bash
# Voiceover Generation Script
# Converts narration scripts (text files with chapter markers) to audio
# using the OpenAI TTS API.
#
# Modes:
#   Default:      Generate a single concatenated voiceover file
#   --per-scene:  Keep individual scene audio files + timing manifest
#
# Prerequisites:
#   - OPENAI_API_KEY environment variable set
#   - curl installed
#   - ffmpeg installed (for concatenation and duration probing)
#
# Usage:
#   ./demos/voiceover.sh demos/narration/plugin-demo.txt
#   ./demos/voiceover.sh demos/narration/plugin-demo.txt --per-scene
#   ./demos/voiceover.sh demos/narration/plugin-demo.txt --voice nova
#   ./demos/voiceover.sh demos/narration/plugin-demo.txt --model tts-1-hd

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RECORDING_DIR="${SCRIPT_DIR}/recordings"

VOICE="${VOICE:-echo}"
MODEL="${MODEL:-tts-1-hd}"
SPEED="${SPEED:-1.0}"
PER_SCENE=false

# ─── Argument Parsing ─────────────────────────────────────────────────────────

if [[ $# -lt 1 ]]; then
    echo "Usage: $0 <narration-file> [--per-scene] [--voice <voice>] [--model <model>]"
    echo ""
    echo "Options:"
    echo "  --per-scene  Keep individual scene audio files + generate timing manifest"
    echo "  --voice      TTS voice (alloy, echo, fable, onyx, nova, shimmer)"
    echo "  --model      TTS model (tts-1, tts-1-hd)"
    echo "  --speed      Playback speed (0.25-4.0, default 1.0)"
    exit 1
fi

NARRATION_FILE="$1"
shift

while [[ $# -gt 0 ]]; do
    case "$1" in
        --per-scene) PER_SCENE=true; shift ;;
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
PARTS_DIR="${RECORDING_DIR}/${DEMO_NAME}-scenes"
OUTPUT_FILE="${RECORDING_DIR}/${DEMO_NAME}-voiceover.mp3"
MANIFEST_FILE="${RECORDING_DIR}/${DEMO_NAME}-manifest.txt"

mkdir -p "$PARTS_DIR"

# ─── Parse Scenes ─────────────────────────────────────────────────────────────

echo "Parsing narration: $NARRATION_FILE"
echo "Voice: $VOICE | Model: $MODEL | Speed: $SPEED"
if $PER_SCENE; then
    echo "Mode: per-scene (keeping individual files)"
fi

SCENE_NUM=0
SCENE_TEXT=""
SCENE_NAME=""
CONCAT_LIST="${PARTS_DIR}/concat.txt"
> "$CONCAT_LIST"

# Collect scene names for manifest
declare -a SCENE_NAMES=()
declare -a SCENE_FILES=()

process_scene() {
    local num="$1"
    local name="$2"
    local text="$3"

    if [[ -z "$text" ]]; then
        return
    fi

    local part_file="${PARTS_DIR}/scene-$(printf '%02d' "$num").mp3"
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
        SCENE_NAMES+=("$name")
        SCENE_FILES+=("$part_file")
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

# ─── Generate Timing Manifest ───────────────────────────────────────────────

if [[ ${#SCENE_FILES[@]} -gt 0 ]]; then
    echo ""
    echo "Generating timing manifest: $MANIFEST_FILE"
    {
        echo "# Scene timing manifest for ${DEMO_NAME}"
        echo "# Format: scene_num | duration_secs | audio_file | scene_name"
        echo "#"
        for i in "${!SCENE_FILES[@]}"; do
            local_file="${SCENE_FILES[$i]}"
            local_name="${SCENE_NAMES[$i]}"
            local_num=$((i + 1))
            # Probe duration with ffprobe
            duration=$(ffprobe -v quiet -show_entries format=duration \
                -of default=noprint_wrappers=1:nokey=1 "$local_file" 2>/dev/null || echo "0.0")
            printf '%02d | %6.2f | %s | %s\n' "$local_num" "$duration" \
                "$(basename "$local_file")" "$local_name"
        done
    } > "$MANIFEST_FILE"
    cat "$MANIFEST_FILE"
fi

# ─── Concatenate or Keep Parts ───────────────────────────────────────────────

if [[ -s "$CONCAT_LIST" ]]; then
    echo ""
    echo "Concatenating ${SCENE_NUM} parts into: $OUTPUT_FILE"
    ffmpeg -y -f concat -safe 0 -i "$CONCAT_LIST" -c copy "$OUTPUT_FILE" 2>/dev/null
    echo "Voiceover saved: $OUTPUT_FILE"

    if $PER_SCENE; then
        # Keep parts directory, remove only the concat list
        rm -f "$CONCAT_LIST"
        echo ""
        echo "Per-scene audio files kept in: $PARTS_DIR/"
        ls -la "$PARTS_DIR"/scene-*.mp3
    else
        # Clean up parts
        rm -rf "$PARTS_DIR"
    fi
else
    echo "Warning: no audio parts generated"
    exit 1
fi
