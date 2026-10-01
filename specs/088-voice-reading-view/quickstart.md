# Quickstart: Validate the Voice Reading View

## Prerequisites

- `whisper-server` and `whisper-cli` on `PATH` (`brew install whisper-cpp`)
- A running workspace for `cc-deck ws voice <workspace>`
- Build and install via `make install` (never `go build`)
- Automated checks: `make test` and `make lint` pass

## Scenario 1: Reading view during a recording (US1)

1. `cc-deck ws voice <workspace>`; footer shows `g: turns` and no `v` hint.
2. Press `v`: nothing happens.
3. Press `r`, accept the filename. Footer now shows `v: read`.
4. Speak three sentences with pauses of about 4 seconds between them.
5. Press `v`: three blocks with `HH:MM:SS` lines and alternating `▌`/`┃` bars.
6. Press `k` a few times, speak again: position stays, footer shows `1 new ↓`.
7. Press `G`: view jumps to the end, footer shows `● following`.
8. Resize the pane: text re-wraps.
9. Press `esc`: normal view, recording continues. Press `v`, then `R`: view closes, recording stops.

Expected: no 200-entry truncation for long recordings; `+`/`-`/`d`/`m` do nothing inside the reading view.

## Scenario 2: Live latency (US2, SC-001)

1. Start a recording.
2. Say one sentence, then stay silent.
3. Expected: text appears within 5 seconds of the end of the sentence (passages end after 1 second of silence).
4. Stop the recording and dictate: dictation sensitivity and pause behavior are as before (2.5 second silence).

## Scenario 3: tdrz mode (US3)

1. With `whisper-cli` absent from `PATH` (temporarily rename it or use a shell with a reduced `PATH`): press `g`, expect an error naming `whisper-cli`, mode stays `basic`.
2. With the model missing: press `g`, expect the download prompt; press `n`, mode stays `basic`.
3. Press `g`, then `y`: progress appears in the footer; dictation keeps working; on completion the header shows `Turns: tdrz`.
4. Alternatively install up front: `cc-deck ws voice --setup --model small.en-tdrz`.
5. Start a recording and play a two-voice English exchange without pauses between speakers: each speaker change starts a new block (SC-003: at least 7 of 10).
6. Press `g` during the recording: mode does not change.
7. Stop the recording and dictate: delivery uses the configured model with no delay.
8. With `--verbose`, `~/.local/state/cc-deck/voice.log` shows the turn mode at recording start, each tdrz call with duration and turn count, and any fallback.

## Scenario 4: Transcript file (US4)

1. Record an exchange with at least two turns, stop.
2. `cat ~/.local/share/cc-deck/transcripts/<file>.txt`: turns separated by exactly one blank line; with timestamps on, each line keeps the `[HH:MM:SS] ` prefix.

## Scenario 5: Configuration

1. Add `defaults.voice.turn_mode: tdrz` and a `recording` block (see `contracts/config.md`).
2. Start the relay: header shows `Turns: tdrz`.
3. Set `recording.max_chunk: 99`: startup prints a config warning and the default 12 is used.
