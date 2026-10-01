# Contract: Configuration Keys

New keys under `defaults.voice` in `~/.config/cc-deck/config.yaml`:

```yaml
defaults:
  voice:
    turn_mode: basic          # basic | tdrz; initial turn mode of each relay session
    recording:
      threshold: 20           # 0-100, logarithmic like defaults.voice.threshold
      silence: 1.0            # seconds of silence that end a recording passage
      max_chunk: 12           # maximum seconds per recording passage
      pause_break: 3.0        # seconds of silence that start a new turn
```

All keys are optional. Missing keys use the defaults shown.

Validation (`config.validateVoice`, reported by `ValidateAndWarn`):

| Key | Rule | Finding on violation |
|-----|------|----------------------|
| `turn_mode` | `basic` or `tdrz` (case-insensitive) | Warning; `basic` used |
| `recording.threshold` | 0 to 100 | Warning; default used |
| `recording.silence` | greater than 0, at most 10 | Warning; default used |
| `recording.max_chunk` | 2 to 30 | Warning; default used |
| `recording.pause_break` | greater than the effective `recording.silence` | Warning; default used |

CLI: no new flags. `cc-deck ws voice --setup --model small.en-tdrz` installs the turn-aware model through the existing setup flow (FR-020).
