# Research: 021-release-process

**Date**: 2026-03-15
**Status**: Complete

## R1: GoReleaser Configuration for Rust+Go Hybrid

**Decision**: Use GoReleaser with `before` hooks to build the WASM plugin, then let GoReleaser handle Go cross-compilation.

**Rationale**: GoReleaser cannot cross-compile Rust, but the WASM target (`wasm32-wasip1`) is platform-independent. Build WASM once, then GoReleaser cross-compiles the Go CLI (which embeds the WASM via `go:embed`) for all target platforms.

**Key findings**:
- WASM binary is embedded at `cc-deck/internal/plugin/cc_deck.wasm` via `//go:embed cc_deck.wasm`
- GoReleaser `before.hooks` can run `cargo build --target wasm32-wasip1 --release` and copy the result
- Go ldflags inject: `Version`, `Commit`, `Date`, `ImageRegistry` (currently only Version and ImageRegistry are set)
- GoReleaser auto-sets `Version` from git tag, `Commit` and `Date` from git metadata
- Module path: `github.com/rhuss/cc-mux/cc-deck`, binary at `./cmd/cc-deck`
- GoReleaser `dir` field can set the build working directory to `cc-deck/`

**Alternatives considered**:
- Build WASM per-platform: Rejected, WASM is platform-independent
- Use Makefile from GoReleaser: Rejected, GoReleaser handles cross-compile better

## R2: nFPM Configuration for RPM/DEB

**Decision**: Use GoReleaser's built-in nFPM integration for RPM and DEB packages.

**Rationale**: nFPM is bundled with GoReleaser. No separate tool installation needed. Produces standard RPM and DEB packages with proper metadata.

**Key findings**:
- nFPM config is inline in `.goreleaser.yaml` under `nfpms:`
- Supports RPM and DEB simultaneously from the same config
- Can declare Zellij as a `recommends` (optional dependency)
- Package description, license, homepage, maintainer set in nFPM section
- Binary installed to `/usr/local/bin/cc-deck` by default

## R3: Homebrew Tap Setup

**Decision**: Create `cc-deck/homebrew-tap` repository with auto-generated formula.

**Rationale**: GoReleaser's `brews:` section auto-generates and pushes the formula. Requires a separate repository and a GitHub token with write access.

**Key findings**:
- Repository: `cc-deck/homebrew-tap`
- Formula name: `cc-deck`
- Install command: `brew install cc-deck/tap/cc-deck`
- GoReleaser pushes formula via `HOMEBREW_TAP_GITHUB_TOKEN` secret
- Formula declares `depends_on "zellij" => :recommended` (optional)
- Post-install message instructs user to run `cc-deck plugin install`

## R4: Container Image Build in Release Pipeline

**Decision**: Separate CI step after GoReleaser, not using GoReleaser's Docker support.

**Rationale**: cc-deck uses Podman for multi-arch manifest builds. GoReleaser's Docker support uses `docker buildx` which does not align with the existing Podman workflow. Keep image builds as a separate job using the existing Makefile targets.

**Key findings**:
- Current images: `quay.io/rhuss/cc-deck-base`, `quay.io/rhuss/cc-deck-demo`
- New registry: `quay.io/cc-deck/cc-deck-base`, `quay.io/cc-deck/cc-deck-demo`
- Multi-arch via `podman manifest` (arm64 + amd64)
- Makefile targets: `base-image-push`, `demo-image-push` with `podman manifest push --all`
- Need `QUAY_USERNAME` and `QUAY_PASSWORD` GitHub secrets for push
- Images tagged with version (`0.3.0`) and `latest`

## R5: Registry Migration Scope

**Decision**: Update all `quay.io/rhuss` references to `quay.io/cc-deck` across the codebase.

**Key findings**:
- Makefile `REGISTRY` default: `quay.io/rhuss` (line 14)
- Go ldflags `ImageRegistry`: `quay.io/rhuss` (Makefile line 24)
- Go version.go default: `quay.io/rhuss` (line 19)
- Demo image Containerfile `ARG BASE_IMAGE`: `quay.io/rhuss/cc-deck-base:latest`
- Documentation: README.md, docs/ quickstart pages, demos/
- Demo scripts reference `quay.io/rhuss/cc-deck-demo:latest`

## R6: Flatpak Manifest

**Decision**: Create a Flatpak manifest for future Flathub submission. Not automated in the release pipeline initially.

**Rationale**: Flathub submission is a manual review process. The manifest is created as part of the release spec but submitted separately. GoReleaser does not support Flatpak.

**Key findings**:
- App ID: `io.github.cc_deck.cc_deck`
- Runtime: `org.freedesktop.Platform` (Zellij is a terminal app)
- Build system: simple (copy pre-built binary)
- Manifest file: `flatpak/io.github.cc_deck.cc_deck.yml`
- Desktop file and AppStream metadata needed for Flathub
