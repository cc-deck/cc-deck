# Demo Recording System

Automated demo recordings for cc-deck. Each demo is fully scripted and can be re-run to produce updated recordings.

## Prerequisites

- cc-deck installed (`make install`)
- [asciinema](https://asciinema.org/) 3.2+ (`brew install asciinema`)
- [agg](https://github.com/asciinema/agg) 1.7+ (`brew install agg`)
- [ffmpeg](https://ffmpeg.org/) 8+ (`brew install ffmpeg`)
- Claude Code installed with API key configured

For voiceover generation:
- `OPENAI_API_KEY` environment variable set

## Quick Start

```bash
# Set up demo projects
make demo-setup

# Record the plugin demo
make demo-record DEMO=plugin

# Convert to GIF (for landing page)
make demo-gif DEMO=plugin

# Convert to MP4 (optional, with voiceover)
export OPENAI_API_KEY=sk-...
make demo-voiceover DEMO=plugin
make demo-mp4 DEMO=plugin

# Clean up
make demo-clean
```

## Available Demos

| Demo | Script | Description |
|------|--------|-------------|
| `plugin` | `scripts/plugin-demo.sh` | Sidebar features, navigation, smart attend |
| `deploy` | `scripts/deploy-demo.sh` | Container deployment and reconnection |
| `image` | `scripts/image-demo.sh` | Bespoke image creation pipeline |

## Output Formats

| Format | Command | Use Case |
|--------|---------|----------|
| `.cast` | `make demo-record` | Raw asciinema recording |
| `.gif` | `make demo-gif` | Landing page hero clip |
| `.mp4` | `make demo-mp4` | Video with optional voiceover |

## Pipe Commands

Demo scripts control the plugin via pipe messages instead of key simulation:

```bash
zellij pipe --name "cc-deck:nav-toggle"    # Toggle navigation mode
zellij pipe --name "cc-deck:nav-up"        # Move cursor up
zellij pipe --name "cc-deck:nav-down"      # Move cursor down
zellij pipe --name "cc-deck:nav-select"    # Select session
zellij pipe --name "cc-deck:attend"        # Smart attend
zellij pipe --name "cc-deck:pause"         # Toggle pause
zellij pipe --name "cc-deck:help"          # Toggle help
```

## Directory Structure

```
demos/
├── runner.sh               Framework (scene, pause, wait_for, cc_pipe)
├── voiceover.sh            TTS generation (OpenAI API)
├── README.md               This file
├── scripts/
│   ├── plugin-demo.sh      Plugin features demo
│   ├── deploy-demo.sh      Container deployment demo
│   └── image-demo.sh       Image builder demo
├── projects/
│   ├── setup.sh            Create demo projects in /tmp
│   ├── cleanup.sh          Remove demo projects
│   ├── cc-deck-image.yaml  Pre-built manifest for image demo
│   ├── todo-api/           Python FastAPI template
│   ├── weather-cli/        Go CLI template
│   └── portfolio/          HTML/CSS template
├── narration/
│   ├── plugin-demo.txt     Voiceover script with chapter markers
│   ├── deploy-demo.txt
│   └── image-demo.txt
└── recordings/             Generated output (gitignored)
    └── .gitkeep
```

## Troubleshooting

**Claude Code does not start**: Verify `ANTHROPIC_API_KEY` is set and Claude Code is installed.

**Pipe commands have no effect**: Ensure you are running inside a Zellij session with the cc-deck plugin loaded (`zellij --layout cc-deck`).

**Recording is empty**: The demo script must run inside an active Zellij session. Start Zellij first, then run the script from a pane within the session.

**Voiceover fails**: Check that `OPENAI_API_KEY` is set. The script degrades gracefully if the API is unavailable.
