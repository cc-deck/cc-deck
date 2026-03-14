# Implementation Plan: Demo Recording System

**Branch**: `020-demo-recordings` | **Date**: 2026-03-14 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/020-demo-recordings/spec.md`

## Summary

Build an automated demo recording system for cc-deck. Add pipe message handlers to the Zellij plugin for programmatic control, create three demo projects as recording subjects, build a shell-based demo runner framework, and produce terminal recordings in multiple output formats (GIF for landing page, MP4 with voiceover for team sharing, embeddable for docs).

## Technical Context

**Language/Version**: Rust stable (wasm32-wasip1) for plugin pipe handlers, Bash for demo scripts, Python/Go/HTML for demo projects
**Primary Dependencies**: zellij-tile 0.43.1 (plugin SDK), asciinema 3.2.0 (recording), agg 1.7.0 (GIF), ffmpeg 8.0.1 (video/audio)
**Storage**: Filesystem (demos/ directory for scripts, projects, recordings)
**Testing**: Manual verification of recordings + pipe handler unit tests via `cargo test`
**Target Platform**: macOS (recording environment), Linux (demo container playback)
**Project Type**: Plugin extension + tooling scripts
**Performance Goals**: Demo setup < 30s, full pipeline < 15 min
**Constraints**: No OS-level key simulation, pipe-only plugin control

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Gate | Status | Notes |
|------|--------|-------|
| I. Two-Component Architecture | PASS | Plugin gets pipe handlers (Rust), demo scripts are standalone (Bash) |
| II. Plugin Installation | PASS | Demo scripts use `make install` before recording |
| III. WASM Filename Convention | PASS | No change to WASM naming |
| IV. WASM Host Function Gating | PASS | New pipe handler methods use existing gated host functions |
| V. Zellij API Research | PASS | Pipe API researched via zellij-tile source and docs |
| VI. Build via Makefile Only | PASS | New `demo-*` Makefile targets added, no direct build commands |
| VII. Simplicity | PASS | Minimal additions: enum variants + match arms for pipes, shell scripts for demos |
| VIII. Documentation Freshness | PASS | README updated with demo instructions after completion |
| IX. Spec Tracking | PASS | Spec added to README table |

## Project Structure

### Documentation (this feature)

```text
specs/020-demo-recordings/
├── spec.md
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output
│   └── pipe-commands.md
└── checklists/
    └── requirements.md
```

### Source Code (repository root)

```text
cc-zellij-plugin/src/
├── pipe_handler.rs      # MODIFIED: new PipeAction variants + parsing
└── main.rs              # MODIFIED: new match arms in pipe() handler

demos/                   # NEW: demo recording system
├── runner.sh            # Demo runner framework (helper functions)
├── scripts/
│   ├── plugin-demo.sh   # Plugin features demo script
│   ├── deploy-demo.sh   # Image deployment demo script
│   └── image-demo.sh    # Custom image creation demo script
├── projects/
│   ├── setup.sh         # Create all demo projects
│   ├── cleanup.sh       # Remove demo projects
│   ├── todo-api/        # Python FastAPI project template
│   ├── weather-cli/     # Go CLI project template
│   └── portfolio/       # HTML/CSS/JS project template
├── narration/
│   ├── plugin-demo.txt  # Voiceover script with chapter markers
│   ├── deploy-demo.txt
│   └── image-demo.txt
├── voiceover.sh         # TTS generation script (OpenAI API)
└── recordings/          # Generated output (gitignored)
    └── .gitkeep

Makefile                 # MODIFIED: new Demo section with targets
```

**Structure Decision**: Flat `demos/` directory at project root. Keeps demo assets separate from source code. Demo projects are templates copied to `/tmp/cc-deck-demo/` during recording. Recordings directory is gitignored (generated output).
