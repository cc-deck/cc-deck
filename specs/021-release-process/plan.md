# Implementation Plan: Release Process

**Branch**: `021-release-process` | **Date**: 2026-03-15 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/021-release-process/spec.md`

## Summary

Set up an automated release pipeline using GoReleaser for cross-platform binary distribution (macOS + Linux, arm64 + amd64), Homebrew tap, RPM/DEB packages, and a Flatpak manifest. Migrate container images from `quay.io/rhuss` to `quay.io/cc-deck`. GitHub Actions release workflow triggered on version tags.

## Technical Context

**Language/Version**: Go 1.25 (CLI), Rust stable wasm32-wasip1 (WASM plugin), YAML (GoReleaser config), Bash (CI scripts)
**Primary Dependencies**: GoReleaser (release automation), nFPM (RPM/DEB packaging, built into GoReleaser), Podman (container images)
**Storage**: N/A (release artifacts stored on GitHub Releases and quay.io)
**Testing**: `goreleaser release --snapshot --clean` for local dry run, CI verification
**Target Platform**: macOS (darwin/amd64, darwin/arm64), Linux (linux/amd64, linux/arm64)
**Project Type**: Release engineering / CI/CD pipeline
**Performance Goals**: Full release pipeline under 15 minutes
**Constraints**: WASM must be built before Go cross-compile. GoReleaser runs inside `cc-deck/` subdirectory. Container builds use Podman (not Docker).

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Gate | Status | Notes |
|------|--------|-------|
| I. Two-Component Architecture | PASS | WASM built first as pre-hook, Go CLI cross-compiled by GoReleaser |
| II. Plugin Installation | N/A | Release pipeline does not install plugins |
| III. WASM Filename Convention | PASS | `cc_deck.wasm` (underscore) used in embed path |
| IV. WASM Host Function Gating | N/A | No plugin code changes |
| V. Zellij API Research | N/A | No Zellij API usage |
| VI. Build via Makefile Only | PASS | GoReleaser replaces `make cross-cli` for release builds. Local dev still uses Makefile. |
| VII. Simplicity | PASS | GoReleaser config is declarative YAML, no custom scripting |
| VIII. Documentation Freshness | PASS | README, docs, landing page updated with new install methods and registry |
| IX. Spec Tracking | PASS | Spec 021 added to README table |

## Project Structure

### Documentation (this feature)

```text
specs/021-release-process/
├── spec.md
├── plan.md              # This file
├── research.md
├── data-model.md
├── quickstart.md
└── checklists/
    └── requirements.md
```

### Source Code (repository root)

```text
.goreleaser.yaml                    # NEW: GoReleaser configuration
.github/workflows/release.yaml     # NEW: Release workflow (tag-triggered)

Makefile                            # MODIFIED: REGISTRY default → quay.io/cc-deck
cc-deck/internal/cmd/version.go    # MODIFIED: ImageRegistry default → quay.io/cc-deck

demo-image/Containerfile            # MODIFIED: BASE_IMAGE → quay.io/cc-deck/cc-deck-base
docs/modules/ROOT/pages/one-liner.adoc  # MODIFIED: image references
docs/modules/running/pages/podman.adoc  # MODIFIED: image references
docs/modules/running/pages/kubernetes.adoc  # MODIFIED: image references
README.md                          # MODIFIED: install methods, registry, spec table

flatpak/                            # NEW: Flatpak manifest directory
├── io.github.cc_deck.cc_deck.yml
├── cc-deck.desktop
└── cc-deck.metainfo.xml
```

**Structure Decision**: `.goreleaser.yaml` at project root. GoReleaser `builds.dir` set to `cc-deck/` for the Go build. Container image builds remain in the existing Makefile targets, triggered as a separate CI job after GoReleaser.

## Implementation Strategy

### Phase Order

1. **Registry migration** (FR-010): Update all `quay.io/rhuss` references first. This unblocks everything else.
2. **GoReleaser config** (FR-001 to FR-006, FR-009, FR-011, FR-013): Core release automation.
3. **GitHub Actions workflow** (FR-008): Release pipeline triggered on tags.
4. **Homebrew tap** (FR-004): Create repository, configure GoReleaser brews section.
5. **Container image CI** (FR-007): Add image build/push job to release workflow.
6. **Flatpak manifest** (FR-012): Create manifest files for Flathub submission.
7. **Documentation** (FR-010 continued): Update README with install methods, update docs.

### GoReleaser Configuration Details

```yaml
# Key sections in .goreleaser.yaml
before:
  hooks:
    - cargo build --target wasm32-wasip1 --release
    - mkdir -p cc-deck/internal/plugin
    - cp cc-zellij-plugin/target/wasm32-wasip1/release/cc_deck.wasm cc-deck/internal/plugin/

builds:
  - dir: cc-deck
    main: ./cmd/cc-deck
    binary: cc-deck
    ldflags:
      - -X github.com/rhuss/cc-mux/cc-deck/internal/cmd.Version={{.Version}}
      - -X github.com/rhuss/cc-mux/cc-deck/internal/cmd.Commit={{.Commit}}
      - -X github.com/rhuss/cc-mux/cc-deck/internal/cmd.Date={{.Date}}
      - -X github.com/rhuss/cc-mux/cc-deck/internal/cmd.ImageRegistry=quay.io/cc-deck
    goos: [linux, darwin]
    goarch: [amd64, arm64]

archives:
  - format: tar.gz
    files: [README.md, LICENSE]

nfpms:
  - formats: [rpm, deb]
    recommends: [zellij]

brews:
  - repository:
      owner: cc-deck
      name: homebrew-tap
    homepage: https://cc-deck.github.io
```

### Container Image Release Job

Separate GitHub Actions job after GoReleaser:
1. Log in to quay.io with `QUAY_USERNAME` / `QUAY_PASSWORD` secrets
2. Build base image for arm64 + amd64
3. Build demo image for arm64 + amd64 (uses GoReleaser-produced linux binaries from `dist/`)
4. Push multi-arch manifests with version tag and `latest`
