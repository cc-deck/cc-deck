# Contract: Voice TUI Keys, Header, and Footer

## Normal view

| Key | Condition | Effect |
|-----|-----------|--------|
| `v` | `recState` is recording or paused | Open reading view |
| `v` | otherwise | No effect |
| `s` | `recState == recIdle` and no download running | Toggle `basic` / `tdrz` (with availability checks, see below) |
| `s` | recording, paused, or download running | No effect |
| `y` / `n` | download prompt open | Start download / dismiss prompt (mode stays `basic`) |
| `esc` | download prompt open | Same as `n` |
| existing keys | | Unchanged (`q`, `m`, `r`, `R`, `+`/`-`, `↑`/`↓`, `d`, `PgUp`/`PgDn`) |

`s` availability checks, in order:
1. `whisper-cli` not on `PATH`: error `separating speakers by voice needs whisper-cli (install whisper-cpp, e.g. brew install whisper-cpp)`; mode stays `basic`; no prompt.
2. Model `small.en-tdrz` missing: error plus prompt `telling speakers apart by voice needs an extra model (488 MB, English only). Download now? [y/n]`.
3. Both present: mode becomes `tdrz`.
Every successful switch (and a successful download) shows a notice in the status line for 5 seconds: `Next recording separates speakers by voice (English only)` or `Next recording separates speakers by pauses`.
Toggling from `tdrz` back to `basic` needs no checks.

While the filename prompt is open, all keys go to the prompt (existing behavior), so `v` and `s` are typed characters.

## Reading view

| Key | Effect |
|-----|--------|
| `↑` / `k` | Scroll up one line; stops following |
| `↓` / `j` | Scroll down one line; resumes following when the bottom is reached |
| `PgUp` / `PgDn` | Scroll by one page (same follow rule) |
| `G` / `End` | Jump to last line, resume following, reset the new-block counter |
| `esc` / `v` | Return to normal view (`v` toggles) |
| `r` | Pause or resume recording (as in normal view) |
| `R` | Stop recording; the view closes automatically |
| `q` / `ctrl+c` | Quit the relay (as in normal view) |
| `+` / `-` | Raise or lower the VAD threshold (as in normal view) |
| `d`, `m`, `s` | No effect |

## Header

Normal view, device line (no new header line, `headerLines` stays 6):

```text
Device:     (default)  Mode: VAD (auto)  Speakers: by pause  ● REC
```

Reading view, single line:

```text
 Reading  ● REC  ⣿⣿⣿⣶⠀⠀⠿⠀  T:20%  Speakers: by voice  2026-10-01-standup.txt
 Reading  ⏸ PAUSED  ⣤⠀⠀⠀⠀⠀⠿⠀  T:20%  Speakers: by pause  2026-10-01-standup.txt
(the file name is dropped first when the pane is too narrow for one line)
```

## Footer

Normal view hint line (appended to the existing hints):
- idle: `s: speaker split`
- recording or paused: `v: read`
- download running: `Downloading voice model: 37% (180/488 MB)` replaces the status line

Reading view hint line:

```text
 ↑↓/jk scroll  PgUp/PgDn page  G end  +/- threshold  v/esc back    ● following
 ↑↓/jk scroll  PgUp/PgDn page  G end  +/- threshold  v/esc back    3 new ↓
```

## Reading view body

```text
 14:03:12
 ▌ So the first thing I wanted to cover
 ▌ is the release timeline for next week.

 14:03:25
 ┃ Right, we still have the two blockers
 ┃ from the dependency update.
```

Empty recording placeholder: `Waiting for speech...` in the hint style.
