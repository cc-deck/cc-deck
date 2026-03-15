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

### Option A: Single continuous recording

```bash
make demo-setup
make demo-record DEMO=plugin
make demo-gif DEMO=plugin
make demo-clean
```

### Option B: Scene-by-scene recording (recommended)

Record each scene as a separate clip for precise audio alignment:

```bash
# 1. Set up demo projects
make demo-setup

# 2. Start Zellij and source the demo script in scene-by-scene mode
zellij --layout cc-deck
# Inside Zellij:
source demos/scripts/plugin-demo.sh --scene-by-scene
# Follow prompts: start/stop iShowU for each scene
# Save clips as: recordings/plugin-demo-scenes/scene-01.mov, scene-02.mov, ...

# 3. Generate per-scene voiceover audio
export OPENAI_API_KEY=sk-...
make demo-voiceover-scenes DEMO=plugin

# 4. Assemble final video (clips + audio, auto-timed)
make demo-assemble DEMO=plugin
# Output: recordings/plugin-demo-final.mp4

# 5. Clean up
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
| `.mp4` | `make demo-mp4` | Single-file video with voiceover |
| `-final.mp4` | `make demo-assemble` | Scene-assembled video with per-scene audio |

## Screenshots

Interactive screenshot capture for documentation images:

```bash
# Inside a running Zellij session with sessions set up:
source demos/scripts/screenshot-setup.sh
```

This walks through 4 sidebar states, pausing for manual capture at each:
1. **sidebar-overview.png** - Mixed session states (working, permission, done, paused)
2. **sidebar-navigation.png** - Navigation mode with cursor highlight
3. **sidebar-help.png** - Help overlay with keyboard shortcuts
4. **sidebar-search.png** - Search/filter mode

Screenshots are saved to `docs/modules/using/images/`.

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
zellij pipe --name "cc-deck:search" -- "text"  # Activate search with text
```

## Directory Structure

```
demos/
├── runner.sh               Framework (scene, pause, wait_for, cc_pipe)
├── voiceover.sh            TTS generation (OpenAI API, supports --per-scene)
├── assemble.sh             Combine scene clips + audio into final video
├── README.md               This file
├── scripts/
│   ├── plugin-demo.sh      Plugin features demo
│   ├── deploy-demo.sh      Container deployment demo
│   ├── image-demo.sh       Image builder demo
│   └── screenshot-setup.sh Interactive screenshot capture
├── projects/
│   ├── setup.sh            Create demo projects in /tmp
│   ├── cleanup.sh          Remove demo projects
│   ├── cc-deck-build.yaml  Pre-built manifest for image demo
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
