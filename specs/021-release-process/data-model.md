# Data Model: 021-release-process

## Entities

### Release Artifact

A versioned output produced by the release pipeline.

| Field | Description |
|-------|-------------|
| name | Archive or package filename (e.g., `cc-deck_0.3.0_linux_amd64.tar.gz`) |
| platform | Target OS: `darwin`, `linux` |
| architecture | Target arch: `amd64`, `arm64` |
| format | Archive or package type: `tar.gz`, `rpm`, `deb` |
| version | Semantic version from git tag (e.g., `0.3.0`) |
| checksum | SHA-256 hash |

### Homebrew Formula

Auto-generated Ruby file in the tap repository.

| Field | Description |
|-------|-------------|
| name | Formula name: `cc-deck` |
| version | Release version |
| url | Download URL for the archive (per-platform) |
| sha256 | Archive checksum |
| dependencies | `zellij` (recommended, not required) |

### Container Image

OCI multi-arch image on quay.io.

| Field | Description |
|-------|-------------|
| name | Image name: `cc-deck-base` or `cc-deck-demo` |
| registry | `quay.io/cc-deck` |
| tag | Version tag (e.g., `0.3.0`) plus `latest` |
| platforms | `linux/arm64`, `linux/amd64` |

### Version Info (build-time injection)

| Field | Go Variable | Source |
|-------|-------------|--------|
| Version | `internal/cmd.Version` | Git tag (stripped `v` prefix) |
| Commit | `internal/cmd.Commit` | Git commit SHA |
| Date | `internal/cmd.Date` | Build timestamp |
| ImageRegistry | `internal/cmd.ImageRegistry` | `quay.io/cc-deck` |

## State Transitions

### Release Pipeline

```
Tag pushed → WASM build → Go cross-compile → Archive → Package (RPM/DEB)
                                                    → Homebrew tap update
                                                    → GitHub Release publish
                                                    → Container image build → Push to quay.io
```

### Version Lifecycle

```
Development (0.2.0-dev in Makefile/Cargo.toml)
  → Tag v0.3.0 (GoReleaser reads version from tag)
  → Release published
  → Post-release commit: bump Makefile/Cargo.toml to 0.3.1-dev
```
