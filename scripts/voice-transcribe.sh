#!/usr/bin/env bash
#
# voice-transcribe.sh - Local microphone-to-text transcription
#
# Captures audio from the default microphone using ffmpeg (avfoundation),
# detects speech segments via energy-based VAD, and transcribes each
# utterance with whisper-cli. Transcripts are printed to stdout and
# optionally appended to a file.
#
# Dependencies: ffmpeg, whisper-cli (brew install ffmpeg whisper-cpp)
#
# Usage:
#   ./voice-transcribe.sh                    # print to stdout
#   ./voice-transcribe.sh -o transcript.txt  # also append to file
#   ./voice-transcribe.sh -m base.en         # use base.en model (faster)
#   ./voice-transcribe.sh -m small.en        # use small.en model (better)
#   ./voice-transcribe.sh -t 0.02            # raise VAD threshold (noisy room)
#
set -euo pipefail

# --- Configuration ---
MODEL="${WHISPER_MODEL:-base.en}"
MODEL_DIR="${XDG_CACHE_HOME:-$HOME/.cache}/cc-deck/models"
SAMPLE_RATE=16000
VAD_THRESHOLD=0.015       # RMS energy threshold for speech detection
SILENCE_DURATION=2.0      # seconds of silence to end an utterance
MAX_UTTERANCE=30          # maximum utterance length in seconds
OUTPUT_FILE=""

# --- Argument parsing ---
while [[ $# -gt 0 ]]; do
    case "$1" in
        -m|--model) MODEL="$2"; shift 2 ;;
        -o|--output) OUTPUT_FILE="$2"; shift 2 ;;
        -t|--threshold) VAD_THRESHOLD="$2"; shift 2 ;;
        -s|--silence) SILENCE_DURATION="$2"; shift 2 ;;
        -h|--help)
            echo "Usage: $(basename "$0") [options]"
            echo "Options:"
            echo "  -m, --model MODEL    Whisper model (default: base.en)"
            echo "                       Available: tiny.en, base.en, small.en, medium"
            echo "  -o, --output FILE    Append transcripts to file"
            echo "  -t, --threshold N    VAD threshold 0.0-1.0 (default: 0.015)"
            echo "  -s, --silence SECS   Silence to end utterance (default: 2.0)"
            echo "  -h, --help           Show this help"
            exit 0
            ;;
        *) echo "Unknown option: $1" >&2; exit 1 ;;
    esac
done

# --- Model filename mapping ---
declare -A MODEL_FILES=(
    ["tiny.en"]="ggml-tiny.en.bin"
    ["base.en"]="ggml-base.en.bin"
    ["small.en"]="ggml-small.en.bin"
    ["medium"]="ggml-medium.bin"
)

declare -A MODEL_URLS=(
    ["tiny.en"]="https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-tiny.en.bin"
    ["base.en"]="https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-base.en.bin"
    ["small.en"]="https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-small.en.bin"
    ["medium"]="https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-medium.bin"
)

MODEL_FILE="${MODEL_FILES[$MODEL]:-}"
if [[ -z "$MODEL_FILE" ]]; then
    echo "Unknown model: $MODEL" >&2
    echo "Available: tiny.en, base.en, small.en, medium" >&2
    exit 1
fi

MODEL_PATH="$MODEL_DIR/$MODEL_FILE"

# --- Dependency checks ---
for cmd in ffmpeg whisper-cli; do
    if ! command -v "$cmd" >/dev/null 2>&1; then
        echo "Missing dependency: $cmd" >&2
        echo "Install with: brew install ${cmd/whisper-cli/whisper-cpp}" >&2
        exit 1
    fi
done

# --- Download model if needed ---
if [[ ! -f "$MODEL_PATH" ]]; then
    echo "Downloading model $MODEL..." >&2
    mkdir -p "$MODEL_DIR"
    curl -L --progress-bar "${MODEL_URLS[$MODEL]}" -o "$MODEL_PATH.tmp"
    mv "$MODEL_PATH.tmp" "$MODEL_PATH"
    echo "Model saved to $MODEL_PATH" >&2
fi

# --- Cleanup ---
TMPDIR_WORK=$(mktemp -d)
FIFO="$TMPDIR_WORK/audio.fifo"
mkfifo "$FIFO"

cleanup() {
    kill "$FFMPEG_PID" 2>/dev/null || true
    rm -rf "$TMPDIR_WORK"
}
trap cleanup EXIT INT TERM

# --- Start mic capture ---
echo "Listening... (Ctrl+C to stop)" >&2
echo "Model: $MODEL | Threshold: $VAD_THRESHOLD | Silence: ${SILENCE_DURATION}s" >&2
echo "---" >&2

ffmpeg -f avfoundation -i ":0" \
    -f s16le -ac 1 -ar "$SAMPLE_RATE" \
    -loglevel error \
    "$FIFO" &
FFMPEG_PID=$!

# --- VAD + Transcription loop ---
#
# Read raw PCM from the FIFO in chunks, compute RMS energy, detect
# speech onset/offset, write each utterance to a temp WAV file,
# and transcribe it.

FRAME_SAMPLES=$((SAMPLE_RATE / 50))  # 20ms frames
FRAME_BYTES=$((FRAME_SAMPLES * 2))   # 16-bit = 2 bytes per sample
SILENCE_FRAMES=$(awk "BEGIN {printf \"%d\", $SILENCE_DURATION * 50}")
MAX_FRAMES=$(awk "BEGIN {printf \"%d\", $MAX_UTTERANCE * 50}")

speaking=false
silence_count=0
frame_count=0
utterance_file=""

# Write a WAV header for 16-bit mono PCM
write_wav_header() {
    local file="$1" data_size="$2"
    local file_size=$((data_size + 36))
    # Use printf for binary header (little-endian)
    printf 'RIFF' > "$file"
    printf "$(printf '\\x%02x\\x%02x\\x%02x\\x%02x' \
        $((file_size & 0xFF)) $(((file_size >> 8) & 0xFF)) \
        $(((file_size >> 16) & 0xFF)) $(((file_size >> 24) & 0xFF)))" >> "$file"
    printf 'WAVEfmt ' >> "$file"
    printf '\x10\x00\x00\x00' >> "$file"  # chunk size 16
    printf '\x01\x00' >> "$file"          # PCM format
    printf '\x01\x00' >> "$file"          # 1 channel
    local sr=$SAMPLE_RATE
    printf "$(printf '\\x%02x\\x%02x\\x%02x\\x%02x' \
        $((sr & 0xFF)) $(((sr >> 8) & 0xFF)) \
        $(((sr >> 16) & 0xFF)) $(((sr >> 24) & 0xFF)))" >> "$file"
    local byte_rate=$((sr * 2))
    printf "$(printf '\\x%02x\\x%02x\\x%02x\\x%02x' \
        $((byte_rate & 0xFF)) $(((byte_rate >> 8) & 0xFF)) \
        $(((byte_rate >> 16) & 0xFF)) $(((byte_rate >> 24) & 0xFF)))" >> "$file"
    printf '\x02\x00' >> "$file"          # block align
    printf '\x10\x00' >> "$file"          # 16 bits per sample
    printf 'data' >> "$file"
    printf "$(printf '\\x%02x\\x%02x\\x%02x\\x%02x' \
        $((data_size & 0xFF)) $(((data_size >> 8) & 0xFF)) \
        $(((data_size >> 16) & 0xFF)) $(((data_size >> 24) & 0xFF)))" >> "$file"
}

transcribe_utterance() {
    local raw_file="$1"
    local data_size
    data_size=$(stat -f%z "$raw_file" 2>/dev/null || stat -c%s "$raw_file" 2>/dev/null)
    if [[ "$data_size" -lt $((SAMPLE_RATE / 2)) ]]; then
        return  # skip very short utterances (<0.25s)
    fi

    local wav_file="$TMPDIR_WORK/utterance.wav"
    write_wav_header "$wav_file" "$data_size"
    cat "$raw_file" >> "$wav_file"

    local text
    text=$(whisper-cli -m "$MODEL_PATH" -f "$wav_file" --no-timestamps 2>/dev/null | sed '/^$/d' | sed 's/^ *//')

    if [[ -n "$text" && "$text" != "[BLANK_AUDIO]" ]]; then
        local ts
        ts=$(date '+%H:%M:%S')
        echo "[$ts] $text"
        if [[ -n "$OUTPUT_FILE" ]]; then
            echo "[$ts] $text" >> "$OUTPUT_FILE"
        fi
    fi

    rm -f "$wav_file" "$raw_file"
}

start_utterance() {
    utterance_file="$TMPDIR_WORK/utt_$$.raw"
    : > "$utterance_file"
    speaking=true
    silence_count=0
    frame_count=0
}

end_utterance() {
    speaking=false
    silence_count=0
    frame_count=0
    if [[ -n "$utterance_file" && -f "$utterance_file" ]]; then
        transcribe_utterance "$utterance_file" &
    fi
    utterance_file=""
}

# Main loop: read PCM frames, compute RMS, segment by VAD
while dd bs=$FRAME_BYTES count=1 2>/dev/null < "$FIFO" > "$TMPDIR_WORK/frame.raw"; do
    frame_size=$(stat -f%z "$TMPDIR_WORK/frame.raw" 2>/dev/null || stat -c%s "$TMPDIR_WORK/frame.raw" 2>/dev/null)
    [[ "$frame_size" -lt "$FRAME_BYTES" ]] && break

    # Compute RMS energy using od + awk (pure shell, no compiled helper)
    rms=$(od -An -td2 -v "$TMPDIR_WORK/frame.raw" | tr -s ' ' '\n' | awk -v n="$FRAME_SAMPLES" '
        BEGIN { sum = 0; count = 0 }
        NF > 0 && $1 != "" { v = $1 / 32768.0; sum += v * v; count++ }
        END { if (count > 0) printf "%.6f", sqrt(sum / count); else print "0" }
    ')

    is_loud=$(awk "BEGIN { print ($rms >= $VAD_THRESHOLD) ? 1 : 0 }")

    if [[ "$speaking" == false ]]; then
        if [[ "$is_loud" -eq 1 ]]; then
            start_utterance
            cat "$TMPDIR_WORK/frame.raw" >> "$utterance_file"
        fi
    else
        cat "$TMPDIR_WORK/frame.raw" >> "$utterance_file"
        frame_count=$((frame_count + 1))

        if [[ "$is_loud" -eq 0 ]]; then
            silence_count=$((silence_count + 1))
        else
            silence_count=0
        fi

        if [[ "$silence_count" -ge "$SILENCE_FRAMES" || "$frame_count" -ge "$MAX_FRAMES" ]]; then
            end_utterance
        fi
    fi
done

# Flush last utterance if still speaking
if [[ "$speaking" == true && -n "$utterance_file" ]]; then
    end_utterance
fi

wait
