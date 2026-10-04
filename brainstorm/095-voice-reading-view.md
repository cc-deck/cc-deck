# Brainstorm: Voice Reading View

**Date:** 2026-10-01
**Status:** spec-created (specs/088-voice-reading-view, released in v0.17.0)
**Issue:** https://github.com/cc-deck/cc-deck/issues/43

## Problem Framing

The voice relay TUI doubles as a meeting and chat recorder (`r`), but its history pane is built for dictation debugging: one line per utterance with a status icon, timestamp, and latency.
When attention drifts during a call, there is no comfortable way to read back what was said in the last minutes.

Three gaps stand in the way:

- **Format.** Utterances are not merged into wrapped paragraphs, and metadata noise dominates the line.
- **No speaker turns.** Whisper returns plain text. Its turn hints (leading dash markers, bracketed annotations) are stripped in `relay.go` (`stripLeadingDash`, `stripBracketedAnnotations`) before the TUI sees the text.
- **Latency.** Recording mode forces the VAD threshold to 0 (`SetRecording` in `relay.go`), which disables silence detection. Audio reaches Whisper in back-to-back 30 second chunks (`MaxUtteranceDuration`), so text appears with 30+ seconds of delay. That is too slow for live catch-up.

Constraints from the session:

- During a chat, a single microphone picks up all participants (remote voices come through the laptop speakers). Any speaker separation must come from one mixed mono stream.
- Chats are mostly English, so English-only techniques are acceptable.

## Approaches Considered

### A: TUI-only pause heuristic
- Start a new block when the gap between two utterances exceeds a threshold. Relay unchanged.
- Pros: smallest change.
- Cons: pauses are not turns (fast exchanges merge, thinking pauses split). With the current recording mode there are no pauses at all (threshold 0). Needs rework once speaker labels arrive.

### B: Structured turn hints from the relay
- Transcription events carry segments (text, timing, turn start flag, empty speaker field). Turn hints come from Whisper dash markers plus pause gaps. Text delivered to agent panes stays sanitized as today.
- Pros: label-ready data model, rendering decoupled from detection.
- Cons: Whisper dash markers are sporadic and unreliable as the only signal.

### C: B plus tinydiarize (opt-in)
- tinydiarize is a fine-tuned `small.en` Whisper model that emits a speaker-turn token where the voice changes. The decoder uses both acoustic cues (voice change) and textual cues (question followed by answer). whisper.cpp supports it with `-tdrz`.
- Pros: detects turns without a pause and inside a chunk, does not split on thinking pauses, modest cost (about 0.5 seconds per call measured for `whisper-cli` with `small.en` on this machine, model load included).
- Cons: English only, `small.en` transcription quality (between `base.en` and `medium`), experimental (the author calls it a proof of concept; it tends to miss turns rather than invent them), extra 488 MB model, no speaker identity.

### D: B plus speaker embeddings
- Compute a voice embedding per utterance (CAM++, ERes2Net, or WeSpeaker via ONNX), cluster online into S1/S2/..., optionally enroll the user's voice for a "Me" label.
- Pros: real labels, speaker changes become turn breaks.
- Cons: an utterance with two speakers gets one label unless split via word timestamps, short utterances (under about 1.5 seconds) give unreliable embeddings, labels can drift, release builds use `CGO_ENABLED=0` so the sherpa-onnx Go bindings cannot run in-process (needs a sidecar), and a stored voiceprint is biometric data.

## Decision

Approach C, with tinydiarize as an opt-in turn mode selected before a recording starts.
Speaker labels (D) are deferred to a follow-up feature. The segment model carries an empty speaker field so labels can be added later without touching the rendering.

Findings that shaped the decision (whisper.cpp 1.9.2, the installed version):

- `whisper-server` does not expose the turn token in any HTTP response format. It prints ` [SPEAKER_TURN]` only to its own console, and only with `--print-realtime` (`examples/server/server.cpp:441`). `verbose_json` skips special tokens, which includes the turn token.
- `whisper-cli` exposes it in two ways: inline as ` [SPEAKER_TURN]` in stdout, and as `"speaker_turn_next": true` per segment in its JSON output (`examples/cli/cli.cpp:804`), together with segment timings.
- Therefore tdrz mode transcribes recordings through `whisper-cli -tdrz` with the tdrz model. The relay already has a CLI transcriber (`transcriber_cli.go`). `whisper-server` keeps the configured model, so "switch model at recording start, switch back at the end" needs no server restart and causes no gap.
- The tdrz model is not in the main `ggerganov/whisper.cpp` Hugging Face repo. It is `ggml-small.en-tdrz.bin` (488 MB) in `akashmjn/tinydiarize-whisper.cpp`.

## Key Requirements

### Reading view
- `v` opens the reading view. It is available only while a recording is active (recording or paused), and the `v` hint appears in the footer only then.
- `esc` returns to the normal view. When the recording stops (`R` or an error), the TUI returns to the normal view automatically.
- Content: the utterances of the current recording only, identical to what the transcript file receives. Paused stretches are excluded.
- Layout: chat blocks. Each turn block has a time line (`HH:MM:SS`) and a gutter bar whose style alternates between turns. Text within a turn is merged and soft-wrapped to the pane width. A later speaker label goes on the time line (`14:03:25  S2`).
- Header: a compact single line showing reading mode, recording state (REC or paused), transcript file name, and turn mode.
- Scrolling: `↑`/`↓` and `j`/`k` by line, `PgUp`/`PgDn` by page, `G`/`End` jumps to the last line.
- Live updates: the view follows new text while scrolled to the bottom. Scrolling up stops following and shows an "N new" indicator. `G` resumes following.
- Keys still active in the reading view: `r` (pause/resume), `R` (stop), `q` (quit). `+`/`-`, `d`, and `m` are inactive there, and `↑`/`↓` scroll instead of changing the threshold.

### Turn detection
- `g` in the normal view toggles the turn mode between `basic` and `tdrz`. The current mode is always visible in the header. The toggle is ignored while a recording runs, so the mode is chosen before pressing `r`.
- `basic` mode: a new block starts at Whisper dash markers (leading and mid-text `- `, preserved for the reading view instead of being discarded) and at pauses longer than a threshold.
- `tdrz` mode: the recording is transcribed by `whisper-cli -tdrz` with `ggml-small.en-tdrz.bin`, and a new block starts at each turn token. The configured model stays loaded in `whisper-server` and serves dictation again after the recording ends.
- Switching to `tdrz` without the model installed shows an error and offers a y/n download prompt. Nothing downloads automatically.
- Relay transcription events carry structured segments (text, timing, turn start flag, empty speaker field).

### Recording chunking
- Recording mode keeps silence detection active with its own settings instead of forcing the threshold to 0: a low threshold, roughly 1 second of silence, and a maximum chunk of roughly 10 to 15 seconds. Exact values need tuning.
- Goal: the reading view shows speech within a few seconds, so it works for live catch-up after a distraction.
- Restoring the dictation VAD settings when the recording stops works as today.

### Transcript file
- Turn breaks are written to the transcript file as a blank line between turns (with timestamps enabled, the timestamp marks the start of each turn). This also helps `/meeting-notes`.

### Constitution
- Tests for segment parsing, turn detection, block merging and wrapping, and follow logic.
- README, CLI reference (new keys), configuration reference (recording VAD settings, tdrz model location), and the Antora voice guide.

## Out of Scope
- Speaker identity and labels (approach D), to be brainstormed separately.
- Turn detection for non-English speech.
- Reading view outside recording mode, reopening older transcript files.
- Search, selection, or copy inside the reading view.
- An upstream whisper-server patch exposing `speaker_turn_next` in `verbose_json` (nice to have, not a dependency).

## Open Questions
- Recording VAD values: a threshold low enough for quiet remote voices from laptop speakers (threshold 0 was introduced in `f276b60` without a stated reason), silence duration, maximum chunk length. Measure on a real call.
- End-to-end verification of tdrz on a two-voice sample: JSON `speaker_turn_next` from `whisper-cli`, stdout markers, behavior at chunk boundaries (tdrz sees each chunk in isolation).
- Turn continuity across chunk boundaries in `tdrz` mode: does a new chunk continue the previous block unless the gap is long?
- Whether long pauses should also break blocks in `tdrz` mode.
- Key naming: `g` toggles the turn mode in the normal view, while pagers use `g`/`G` for top/end. The reading view only binds `G`, but confirm this does not confuse.
- Where the tdrz model is stored and how `ws voice setup` and the TUI prompt download it from the separate Hugging Face repo, including checksum handling.
- Whether the selected turn mode persists across relay restarts (config default) or is session-only.
- Fallback when `whisper-cli` fails in `tdrz` mode for a chunk (retry via `whisper-server` in `basic` mode?).
