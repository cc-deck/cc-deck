# Research: Voice Reading View with Turn Detection

All decisions below were verified against the installed whisper.cpp 1.9.2 sources (`examples/server/server.cpp`, `examples/cli/cli.cpp`) and the current cc-deck code on 2026-10-01.

## R1: Source of the speaker-turn signal

- **Decision**: Run `whisper-cli -m <tdrz model> -f <wav> -tdrz -oj -of <tmp>/out -np` per recording passage and read `speaker_turn_next` from `<tmp>/out.json`.
- **Rationale**: `whisper-server` prints ` [SPEAKER_TURN]` only to its console (`server.cpp:441`, only with `--print-realtime`) and strips special tokens from `verbose_json`, so no HTTP response carries the turn signal. `whisper-cli` writes `"speaker_turn_next": true|false` per segment in its JSON output whenever `-tdrz` is set, in both `-oj` and `-ojf` (`cli.cpp:804`), together with `offsets.from`/`offsets.to` in milliseconds. JSON is a stable machine contract; the inline stdout marker is human output.
- **Alternatives considered**: Scraping `whisper-server` stdout with `--print-realtime` (fragile correlation between console lines and HTTP requests); patching `whisper-server` upstream (good follow-up, not a dependency); parsing the inline stdout marker (works, but console format is not a contract); a second `whisper-server` with the tdrz model (still no turn signal over HTTP).

## R2: Cost of a per-passage `whisper-cli` call

- **Decision**: Accept a process spawn per passage.
- **Rationale**: Measured `whisper-cli` with `ggml-small.en.bin` (same architecture as the tdrz model) at 0.46 to 0.47 seconds wall time per call for a 5 second utterance, warm, model load included. With 1 to 12 second passages this keeps SC-001 (5 seconds) comfortably.
- **Alternatives considered**: A persistent process (whisper-cli has no streaming stdin mode for files); in-process bindings (blocked by `CGO_ENABLED=0` release builds).

## R3: Model hosting and integrity

- **Decision**: Add `small.en-tdrz` to the model registry with URL `https://huggingface.co/akashmjn/tinydiarize-whisper.cpp/resolve/main/ggml-small.en-tdrz.bin` and a per-model tree API `https://huggingface.co/api/models/akashmjn/tinydiarize-whisper.cpp/tree/main`. Integrity uses the LFS SHA-256 from that tree API, exactly like the existing models.
- **Rationale**: The file is not in `ggerganov/whisper.cpp` (HEAD returns 404). The akashmjn repo lists `ggml-small.en-tdrz.bin` at 487,614,184 bytes. The existing `downloadModel` already writes to a temp file, verifies SHA-256, then renames, so a partial file can never pass as installed (FR-019).
- **Alternatives considered**: Hard-coding a SHA-256 in code (breaks silently if the repo re-uploads); skipping verification (violates FR-019).

## R4: VAD parameters during recording

- **Decision**: Replace the startup-only computation of `silenceSamples` and `maxSamples` with a per-frame snapshot of `VADConfig` obtained through a function the relay provides under its mutex (`NewVADFunc(func() VADConfig, sampleRate)`). Keep `NewVAD(*VADConfig, int)` as a thin wrapper for existing tests.
- **Rationale**: Today only `Threshold` is read live (through the pointer, unsynchronized), so changing silence or maximum duration at recording start would have no effect. The snapshot also removes the data race on `Threshold`.
- **Alternatives considered**: Restarting the VAD goroutine at recording start (drops in-flight audio and complicates shutdown).

## R5: Recording defaults

- **Decision**: Recording threshold 20% (RMS about 0.0035 on the 0.001 to 0.5 logarithmic scale), silence 1.0 second, maximum chunk 12 seconds, pause-break 3 seconds.
- **Rationale**: Today recording forces 0% (RMS 0.001), which sits below typical room noise, so silence never ends a chunk and every passage runs to the 30 second maximum. The dictation default (RMS 0.015) is about 44% and drops quiet remote voices. 20% sits between the two. The pause-break must exceed the silence duration so ordinary chunk boundaries do not start turns.
- **Alternatives considered**: Adaptive noise-floor tracking (more accurate, more complexity; revisit if tuning on real calls fails).

## R6: Passage timing

- **Decision**: The VAD counts samples since stream start and stamps each `Utterance` with `Start` (onset sample minus pre-roll) and `End` (start plus emitted length) as `time.Duration` offsets. The relay derives the pause gap as `u.Start - lastRecordingEnd`, and segment wall times as `recordingStartedAt + (offset - recordingStartOffset)`.
- **Rationale**: Sample counts are exact and independent of transcription latency; wall-clock arrival times would include variable transcription time.
- **Alternatives considered**: Wall-clock timestamps at event arrival (skewed by transcription latency and timeouts).

## R7: Dash-marker detection

- **Decision**: A marker is `-` (optionally preceded by whitespace) at the start of the raw text, or `-` preceded by `.`, `?`, or `!` plus optional whitespace and followed by whitespace. Regex for split points: `(^|[.?!])\s*-\s+`. Each split piece is sanitized with the existing helpers (`sanitizeTerminalText`, `stripBracketedAnnotations`, `stripLeadingDash`) after splitting.
- **Rationale**: Whisper emits speaker changes as `- Text. - Other text`. Requiring sentence punctuation before mid-text dashes avoids false turns for parenthetical dashes ("well - you know").
- **Alternatives considered**: Treating every ` - ` as a turn (too many false positives).

## R8: Text wrapping and gutter bars

- **Decision**: Wrap with `lipgloss.NewStyle().Width(w).Render(text)` (already used for error wrapping in `view.go`), then prefix every wrapped line with the block's gutter bar (`▌` in color 39, `┃` in color 105, alternating by block index).
- **Rationale**: No new dependency; lipgloss wraps on word boundaries.
- **Alternatives considered**: `muesli/reflow/wordwrap` directly (not a direct dependency today).

## R9: Background model download in bubbletea

- **Decision**: Start the download in a goroutine with a cancellable context owned by the model; progress and completion travel over a channel; a `tea.Cmd` waits for the next message (same pattern as `waitForEvent`). Quitting cancels the context; `downloadModel` already removes its temp file on error.
- **Rationale**: Keeps the TUI responsive (FR-019) and reuses the established event-loop pattern.
- **Alternatives considered**: Blocking download with a spinner (violates FR-019).

## R10: Timeout and fallback

- **Decision**: The relay wraps each `TurnTranscriber` call in `context.WithTimeout(ctx, 30*time.Second)`. On error or timeout it transcribes the same audio with the configured transcriber, splits dash markers, emits an `error` event with the reason, and logs the fallback when verbose.
- **Rationale**: FR-021 and the clarification; the recording never loses a passage.
- **Alternatives considered**: Dropping the passage (loses content).

## R11: Key choices

- **Decision**: `g` toggles turn mode only in the normal view while idle; the reading view binds `G` and `End` for jump-to-end and does not bind `g`. `v` and `g` are swallowed by the filename prompt as text input (existing prompt routing runs first).
- **Rationale**: Matches the spec; avoids pager `g` (top) confusion because the reading view never interprets `g`.
- **Alternatives considered**: `t` for turn mode (the user proposed `g`; kept).
