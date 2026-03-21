# cc-mux Development Guidelines

Auto-generated from all feature plans. Last updated: 2026-03-21

## Content Creation (MANDATORY)

When creating or editing ANY documentation content (AsciiDoc, Markdown, landing page text):
- **ALWAYS use the prose plugin** with the `cc-deck` voice profile (`.style/voice.yaml`)
- **NEVER use em-dashes or en-dashes** (per global CLAUDE.md rules)
- **One sentence per line** in AsciiDoc files (semantic line breaks)
- Use the cc-deck voice: professional, thorough, no contractions, terminal-native analogies
- Run `/prose:check` before committing documentation changes

## Active Technologies
- Go 1.22+ + cobra (CLI), viper (config), client-go (K8s API), adrg/xdg (XDG paths), serde/yaml (config parsing) (002-cc-deck-k8s)
- XDG config file (`~/.config/cc-deck/config.yaml`) for local state; K8s PVCs for remote persistent storage (002-cc-deck-k8s)
- Go 1.22+ (existing project uses Go 1.25 in go.mod) + cobra (CLI), go:embed (binary embedding), os/exec (Zellij detection) (009-plugin-lifecycle)
- Filesystem only (WASM binary, KDL layout files) (009-plugin-lifecycle)
- Rust (stable, wasm32-wasip1 target) with zellij-tile 0.43.1 + zellij-tile 0.43 (plugin SDK), serde/serde_json (serialization) (010-plugin-bugfixes)
- WASI `/cache/` directory for persistent state (recent.json) (010-plugin-bugfixes)
- Rust stable (edition 2021, wasm32-wasip1 target) for plugin; Go 1.22+ for CLI + zellij-tile 0.43.1 (plugin SDK), serde/serde_json 1.x; cobra (CLI), encoding/json (Go stdlib) (012-sidebar-plugin)
- WASI `/cache/` directory for plugin state; filesystem for installation artifacts (012-sidebar-plugin)
- WASI `/cache/` for plugin state (013-keyboard-navigation)
- Rust stable (edition 2021, wasm32-wasip1 target) + zellij-tile 0.43.1, serde/serde_json 1.x (014-pause-and-help)
- Go 1.22+ (CLI), Rust stable wasm32-wasip1 (plugin) + cobra (CLI), adrg/xdg (XDG paths), serde/serde_json (plugin serialization), zellij-tile 0.43.1 (plugin SDK) (015-session-save-restore)
- JSON files in `$XDG_CONFIG_HOME/cc-deck/sessions/` (015-session-save-restore)
- Go 1.22+ (go.mod specifies 1.25) + k8s.io/client-go v0.35.2, github.com/stretchr/testify (new), cobra (existing) (016-k8s-integration-tests)
- N/A (tests create/delete K8s resources) (016-k8s-integration-tests)
- Containerfile (OCI image build), shell scripts (bash) + Fedora 41 base image, dnf packages, starship (GitHub release) (017-base-image)
- N/A (stateless image artifact) (017-base-image)
- Go 1.22+ (existing cc-deck CLI), Markdown (Claude Code commands) + cobra (CLI), gopkg.in/yaml.v3 (manifest parsing), go:embed (asset embedding) (018-build-manifest)
- Filesystem (build directory, manifest YAML) (018-build-manifest)
- TypeScript (Astro 5.x), AsciiDoc (Antora 3.x), Containerfile (demo image) + Astro, Tailwind CSS, Antora, AsciiDoc (019-docs-landing-page)
- N/A (static site) (019-docs-landing-page)
- Rust stable (wasm32-wasip1) for plugin pipe handlers, Bash for demo scripts, Python/Go/HTML for demo projects + zellij-tile 0.43.1 (plugin SDK), asciinema 3.2.0 (recording), agg 1.7.0 (GIF), ffmpeg 8.0.1 (video/audio) (020-demo-recordings)
- Filesystem (demos/ directory for scripts, projects, recordings) (020-demo-recordings)
- Go 1.25 (CLI), Rust stable wasm32-wasip1 (WASM plugin), YAML (GoReleaser config), Bash (CI scripts) + GoReleaser (release automation), nFPM (RPM/DEB packaging, built into GoReleaser), Podman (container images) (021-release-process)
- N/A (release artifacts stored on GitHub Releases and quay.io) (021-release-process)
- Go 1.25 (existing project) + cobra (CLI), adrg/xdg (config paths), gopkg.in/yaml.v3 (YAML), client-go (K8s API) (022-network-filtering)
- `~/.config/cc-deck/domains.yaml` (user domain groups), `cc-deck-build.yaml` (manifest) (022-network-filtering)
- Go 1.25 (from go.mod) + cobra v1.10.2 (CLI), adrg/xdg v0.5.3 (XDG paths), gopkg.in/yaml.v3 (YAML), client-go v0.35.2 (K8s, existing) (023-env-interface)
- YAML file at `$XDG_STATE_HOME/cc-deck/state.yaml` (new), `$XDG_CONFIG_HOME/cc-deck/config.yaml` (existing, migration source) (023-env-interface)
- Go 1.25 (from go.mod) + cobra v1.10.2 (CLI), adrg/xdg v0.5.3 (XDG paths), gopkg.in/yaml.v3 (YAML parsing), client-go v0.35.2 (K8s, existing) (024-container-env)
- YAML files: `$XDG_CONFIG_HOME/cc-deck/environments.yaml` (definitions), `$XDG_STATE_HOME/cc-deck/state.yaml` (runtime state) (024-container-env)
- Go 1.25 (from go.mod) + cobra v1.10.2 (CLI), gopkg.in/yaml.v3 (YAML parsing), internal/xdg (XDG paths), internal/podman (container interaction), internal/compose (YAML generation), internal/network (domain resolution) (025-compose-env)
- YAML files at `$XDG_CONFIG_HOME/cc-deck/environments.yaml` (definitions) and `$XDG_STATE_HOME/cc-deck/state.yaml` (runtime state). Project-local `.cc-deck/` directory for generated compose artifacts. (025-compose-env)

- Rust (stable, latest edition 2021+) + `zellij-tile` (plugin SDK), `serde`/`serde_json` (serialization) (001-cc-deck)

## Project Structure

```text
cc-zellij-plugin/   # Zellij plugin (Rust)
cc-deck/            # CLI tool (Go)
specs/              # Feature specifications
brainstorm/         # Design notes
```

## Commands

cargo test [ONLY COMMANDS FOR ACTIVE TECHNOLOGIES][ONLY COMMANDS FOR ACTIVE TECHNOLOGIES] cargo clippy

## Code Style

Rust (stable, latest edition 2021+): Follow standard conventions

## Recent Changes
- 025-compose-env: Added Go 1.25 (from go.mod) + cobra v1.10.2 (CLI), gopkg.in/yaml.v3 (YAML parsing), internal/xdg (XDG paths), internal/podman (container interaction), internal/compose (YAML generation), internal/network (domain resolution)
- 024-container-env: Added Go 1.25 (from go.mod) + cobra v1.10.2 (CLI), adrg/xdg v0.5.3 (XDG paths), gopkg.in/yaml.v3 (YAML parsing), client-go v0.35.2 (K8s, existing)
- 023-env-interface: Added Go 1.25 (from go.mod) + cobra v1.10.2 (CLI), adrg/xdg v0.5.3 (XDG paths), gopkg.in/yaml.v3 (YAML), client-go v0.35.2 (K8s, existing)


<!-- MANUAL ADDITIONS START -->

## Constitution Principles (ALWAYS ENFORCED)

These rules apply to ALL code changes, whether from a spec workflow or ad-hoc.
Full constitution at `.specify/memory/constitution.md`.

### Every feature MUST include tests and documentation

A feature is NOT complete until:
1. **Tests** exist for the new code (unit tests at minimum, integration tests when touching external tools)
2. **README.md** is updated with user-facing changes
3. **CLI reference** (`docs/modules/reference/pages/cli.adoc`) covers new commands/flags
4. **Antora docs** have a guide page for substantial features
5. All documentation uses the **prose plugin** with the `cc-deck` voice profile

### Interface implementations MUST satisfy behavioral contracts

When implementing a new backend for an existing interface (e.g., new Environment type):
1. Read the existing implementation(s) to understand full behavior
2. Cross-reference `specs/023-env-interface/contracts/environment-interface.md` for behavioral requirements
3. If the contract lacks requirements for a behavior you see in existing code, add them before implementing

### Build and tool rules

- **NEVER** run `go build` or `cargo build` directly. Use `make install`, `make test`, `make lint`
- XDG paths: Use `internal/xdg` package (NOT `adrg/xdg`). Paths are `~/.config/cc-deck/` and `~/.local/state/cc-deck/` on all platforms
- Container runtime: Use `podman` exclusively (never Docker)

<!-- MANUAL ADDITIONS END -->
