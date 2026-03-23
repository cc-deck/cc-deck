# Quickstart: Creating a Release

## Prerequisites

- GoReleaser installed (`brew install goreleaser`)
- Rust toolchain with `wasm32-wasip1` target
- GitHub token with `repo` and `packages` scope
- quay.io push credentials (for container images)

## Test a Release Locally (Dry Run)

```bash
goreleaser release --snapshot --clean
```

This builds all artifacts without publishing. Check `dist/` for the output.

## Create a Release

```bash
# 1. Ensure main branch is ready
git checkout main
git pull

# 2. Tag the release
git tag v0.3.0
git push origin v0.3.0

# 3. The GitHub Actions release workflow runs automatically
# It produces:
#   - GitHub Release with binaries, RPM, DEB, checksums
#   - Homebrew tap update
#   - Container images pushed to quay.io/cc-deck
```

## Post-Release

After the CI pipeline completes and the GitHub Release is published:

```bash
# 1. Build and push multi-arch container images (arm64 + amd64)
#    CI only builds amd64; multi-arch requires local push
make base-image-push
make demo-image-push

# 2. Verify Homebrew installation
brew update
brew install cc-deck/tap/cc-deck
cc-deck version

# 3. Bump version for next development cycle
#    Update Makefile VERSION (e.g., 0.6.0 -> 0.7.0)
#    Update cc-zellij-plugin/Cargo.toml version field
#    Commit: "Bump version to 0.7.0"
```

## Installation Methods (for users)

| Method | Command |
|--------|---------|
| Homebrew (macOS) | `brew install cc-deck/tap/cc-deck` |
| Binary (any) | Download from GitHub Releases |
| RPM (Fedora) | `sudo dnf install cc-deck-0.3.0.x86_64.rpm` |
| DEB (Ubuntu) | `sudo apt install ./cc-deck_0.3.0_amd64.deb` |
| Container | `podman run -it quay.io/cc-deck/cc-deck-demo:latest` |
