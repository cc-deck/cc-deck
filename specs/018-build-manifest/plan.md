# Implementation Plan: cc-deck Build Pipeline

**Branch**: `018-build-manifest` | **Date**: 2026-03-12 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/018-build-manifest/spec.md`

## Summary

Implement the cc-deck container image build pipeline: manifest schema (`cc-deck-build.yaml`),
CLI commands (`build init`, `build`, `push`, `build verify`, `build diff`), and AI-driven
Claude Code commands (`cc-deck.extract`, `cc-deck.plugin`, `cc-deck.mcp`,
`cc-deck.containerfile`, `cc-deck.publish`). The pipeline follows a CLI-AI-CLI flow:
scaffold with CLI, populate with AI, build/push with CLI.

## Technical Context

**Language/Version**: Go 1.22+ (existing cc-deck CLI), Markdown (Claude Code commands)
**Primary Dependencies**: cobra (CLI), gopkg.in/yaml.v3 (manifest parsing), go:embed (asset embedding)
**Storage**: Filesystem (build directory, manifest YAML)
**Testing**: Go tests for CLI commands, manual testing for AI commands
**Target Platform**: macOS + Linux (CLI), any platform (container images)
**Project Type**: CLI tool extension + Claude Code commands
**Performance Goals**: `build init` < 5s, `build` depends on container runtime
**Constraints**: Must work with both podman and docker
**Scale/Scope**: 5 CLI commands + 5 Claude Code commands + manifest schema

## Constitution Check

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Two-Component Architecture | N/A | This feature is Go CLI only (no Rust/WASM) |
| II. Plugin Installation | PASS | `cc-deck build` uses `cc-deck plugin install` inside the image |
| III. WASM Filename Convention | N/A | No WASM in this feature |
| IV. WASM Host Function Gating | N/A | No WASM in this feature |
| V. Zellij API Research Order | N/A | No Zellij API usage |
| VI. Simplicity | PASS | Flat manifest schema, minimal abstractions |

All gates pass.

## Project Structure

### Documentation (this feature)

```text
specs/018-build-manifest/
├── plan.md              # This file
├── spec.md              # Feature specification
├── research.md          # Technical decisions
├── data-model.md        # Manifest schema, entity definitions
├── quickstart.md        # End-to-end workflow example
├── contracts/
│   └── cli-commands.md  # CLI command contracts
├── checklists/
│   └── requirements.md  # Quality checklist
└── tasks.md             # Task breakdown
```

### Source Code (repository root)

```text
cc-deck/
├── internal/
│   ├── cmd/
│   │   └── build.go           # build init, build, push, verify, diff commands
│   ├── build/
│   │   ├── manifest.go        # Manifest struct + YAML parsing + validation
│   │   ├── runtime.go         # Container runtime detection (podman/docker)
│   │   ├── embed.go           # go:embed for commands + scripts
│   │   └── init.go            # Build directory scaffolding
│   └── plugin/
│       └── install.go         # Existing (extended with --install-zellij)
├── internal/build/
│   └── commands/              # Embedded Claude Code commands
│       ├── cc-deck.extract.md
│       ├── cc-deck.plugin.md
│       ├── cc-deck.mcp.md
│       ├── cc-deck.containerfile.md
│       └── cc-deck.publish.md
└── internal/build/
    └── scripts/               # Embedded helper scripts
        ├── validate-manifest.sh
        └── update-manifest.sh
```

**Structure Decision**: Extend the existing `cc-deck/internal/` structure with a new `build/`
package. Commands and scripts embedded via `go:embed` in `embed.go`.

## Complexity Tracking

No constitution violations. No complexity justifications needed.
