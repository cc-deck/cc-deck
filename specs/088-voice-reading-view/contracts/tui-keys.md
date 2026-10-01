# Contract: Voice TUI Keys, Header, and Footer

## Normal view

| Key | Condition | Effect |
|-----|-----------|--------|
| `v` | `recState` is recording or paused | Open reading view |
| `v` | otherwise | No effect |
| `g` | `recState == recIdle` and no download running | Toggle `basic` / `tdrz` (with availability checks, see below) |
| `g` | recording, paused, or download running | No effect |
| `y` / `n` | download prompt open | Start download / dismiss prompt (mode stays `basic`) |
| `esc` | download prompt open | Same as `n` |
| existing keys | | Unchanged (`q`, `m`, `r`, `R`, `+`/`-`, `↑`/`↓`, `d`, `PgUp`/`PgDn`) |

`g` availability checks, in order:
1. `whisper-cli` not on `PATH`: error `turn mode tdrz needs whisper-cli (install whisper-cpp, e.g. brew install whisper-cpp)`; mode stays `basic`; no prompt.
2. Model `small.en-tdrz` missing: error plus prompt `tdrz model not installed (488 MB). Download now? [y/n]`.
3. Both present: mode becomes `tdrz`.
Toggling from `tdrz` back to `basic` needs no checks.

While the filename prompt is open, all keys go to the prompt (existing behavior), so `v` and `g` are typed characters.

## Reading view

| Key | Effect |
|-----|--------|
| `↑` / `k` | Scroll up one line; stops following |
| `↓` / `j` | Scroll down one line; resumes following when the bottom is reached |
| `PgUp` / `PgDn` | Scroll by one page (same follow rule) |
| `G` / `End` | Jump to last line, resume following, reset the new-block counter |
| `esc` | Return to normal view |
| `r` | Pause or resume recording (as in normal view) |
| `R` | Stop recording; the view closes automatically |
| `q` / `ctrl+c` | Quit the relay (as in normal view) |
| `+`, `-`, `d`, `m`, `g`, `v` | No effect |

## Header

Normal view, device line (no new header line, `headerLines` stays 6):

```text
Device:     (default)  Mode: VAD (auto)  Turns: basic  ● REC
```

Reading view, single line:

```text
 Reading  ● REC  2026-10-01-standup.txt  Turns: tdrz
 Reading  ⏸ PAUSED  2026-10-01-standup.txt  Turns: basic
```

## Footer

Normal view hint line (appended to the existing hints):
- idle: `g: turns`
- recording or paused: `v: read`
- download running: `Downloading tdrz model: 37% (180/465 MB)` replaces the status line

Reading view hint line:

```text
 ↑↓/jk scroll  PgUp/PgDn page  G end  esc back    ● following
 ↑↓/jk scroll  PgUp/PgDn page  G end  esc back    3 new ↓
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
