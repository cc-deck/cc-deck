# Quickstart: Recording a Demo

## Prerequisites

- Zellij installed with cc-deck plugin (`make install`)
- asciinema, agg, ffmpeg installed (`brew install asciinema agg ffmpeg`)
- Claude Code installed and API key configured

## Record the Plugin Demo

```bash
# 1. Set up demo projects
./demos/projects/setup.sh

# 2. Record the demo
make demo-record DEMO=plugin

# 3. Convert to GIF (landing page)
make demo-gif DEMO=plugin

# 4. Convert to MP4 with voiceover (optional)
export OPENAI_API_KEY=sk-...
make demo-mp4 DEMO=plugin

# 5. Clean up demo projects
./demos/projects/cleanup.sh
```

## Available Demos

| Demo | Script | Duration | Description |
|------|--------|----------|-------------|
| plugin | `plugin-demo.sh` | ~60s (short) / ~5min (full) | Plugin features and navigation |
| deploy | `deploy-demo.sh` | ~3min | Image deployment options |
| image | `image-demo.sh` | ~3min | Custom image creation |

## Pipe Commands for Scripting

Control the plugin from demo scripts without key simulation:

```bash
# Navigate the sidebar
zellij pipe --name "cc-deck:nav-toggle"
zellij pipe --name "cc-deck:nav-down"
zellij pipe --name "cc-deck:nav-select"

# Smart attend
zellij pipe --name "cc-deck:attend"
```

## Output Formats

| Format | Tool | Use Case |
|--------|------|----------|
| `.cast` | asciinema | Raw recording (source of truth) |
| `.gif` | agg | Landing page hero clip |
| `.mp4` | ffmpeg + voiceover | Team sharing, presentations |
| embed | asciinema-player | Documentation pages |
