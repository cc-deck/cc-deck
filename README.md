# <img src="assets/logo/cc-deck-icon.png" alt="cc-deck" width="60" valign="middle" /> &nbsp; cc-deck

[![CI](https://github.com/cc-deck/cc-deck/actions/workflows/ci.yaml/badge.svg)](https://github.com/cc-deck/cc-deck/actions/workflows/ci.yaml)
[![codecov](https://codecov.io/gh/cc-deck/cc-deck/graph/badge.svg)](https://codecov.io/gh/cc-deck/cc-deck)
[![Go](https://img.shields.io/badge/Go-1.25-00ADD8?logo=go)](https://go.dev)
[![Rust](https://img.shields.io/badge/Rust-stable-orange?logo=rust)](https://www.rust-lang.org)
[![Zellij](https://img.shields.io/badge/Zellij-0.44+-green)](https://zellij.dev)
[![License](https://img.shields.io/github/license/cc-deck/cc-deck)](LICENSE)
[![Beta](https://img.shields.io/badge/status-beta-orange)](https://github.com/cc-deck/cc-deck)

**The TweetDeck for AI coding agents.** A [Zellij](https://zellij.dev) sidebar plugin that monitors, attends to, and orchestrates multiple AI agent sessions from a single terminal view. Supports Claude Code, OpenCode, and other agents through a pluggable Agent interface. Zellij is a modern terminal multiplexer (like tmux, but with a plugin system and built-in layout management).

> [!WARNING]
> **Beta software.** APIs, configuration formats, and behavior may change between releases. The author uses it daily for real work and it generally does what it promises. Bug reports and feedback are welcome.

**[Website](https://cc-deck.github.io)** · **[Documentation](https://cc-deck.github.io/docs/)** · **[Quickstart](#quickstart)** · **[Contributing](CONTRIBUTING.md)**

---

## Quickstart

### Homebrew (macOS)

```bash
brew install cc-deck/tap/cc-deck
cc-deck config plugin install
zellij --layout cc-deck
```

### Binary download

Download the latest release from [GitHub Releases](https://github.com/cc-deck/cc-deck/releases):

```bash
# macOS (Apple Silicon)
curl -fsSL https://github.com/cc-deck/cc-deck/releases/latest/download/cc-deck_$(curl -s https://api.github.com/repos/cc-deck/cc-deck/releases/latest | jq -r .tag_name | sed 's/^v//')_darwin_arm64.tar.gz | tar -xz
sudo mv cc-deck /usr/local/bin/
cc-deck config plugin install
zellij --layout cc-deck
```

### Linux packages

```bash
# Fedora / RHEL
sudo dnf install ./cc-deck_*.rpm

# Debian / Ubuntu
sudo apt install ./cc-deck_*.deb
```

Download RPM and DEB packages from [GitHub Releases](https://github.com/cc-deck/cc-deck/releases). After installing, run `cc-deck config plugin install` to set up the Zellij plugin and hooks.

### Container workspace

```bash
cc-deck ws new demo --type container --image quay.io/cc-deck/cc-deck-demo
cc-deck ws attach demo
```

### Build from source

```bash
git clone https://github.com/cc-deck/cc-deck.git
cd cc-deck
make install
```

Requires [Zellij](https://zellij.dev) 0.44+, [Go](https://go.dev) 1.22+, and [Rust](https://www.rust-lang.org) stable with `wasm32-wasip1` target.

---

## What does cc-deck do?

Running multiple Claude Code sessions in separate terminals gets messy fast. You lose track of which session needs your input, which one just finished, and which one is stuck waiting for permission.

cc-deck puts a real-time sidebar next to your sessions that shows what each one is doing and directs your attention where it matters.

### Sidebar plugin

The sidebar tracks every Claude Code session across Zellij tabs. It shows activity status, handles permission requests, and provides keyboard-driven navigation. `Alt+a` (smart attend) cycles through sessions that need you, prioritizing permission requests over finished tasks over idle sessions. `Alt+w` jumps between working sessions. Click, type, or use vim-style `j`/`k` navigation.

Session indicators fade over time: a green checkmark dims to grey after five minutes, an idle circle darkens over an hour. You can tell at a glance how fresh each session is.

### Workspace management

The `cc-deck ws` command manages Claude Code sessions across local, containerized, remote, and sandboxed backends. See the [workspace management](#workspace-management) section for the full subcommand reference, project-local configuration, and workspace type details.

```bash
cc-deck ws new my-project --type container --image quay.io/cc-deck/cc-deck-demo
cc-deck ws attach my-project
```

Git repos are cloned into the workspace automatically when you pass `--repo` flags or run `ws new` from inside a git repository. Up to four repos clone in parallel.

### Build command

`cc-deck build` replicates your local developer environment to container images, SSH machines, or OpenShell sandboxes from a single manifest. Two Claude Code slash commands handle the workflow:

- `/cc-deck.capture` discovers your local tools, shell config, plugins, MCP servers, and credentials
- `/cc-deck.build --target container|ssh|openshell` generates and builds the target

Capture once, build for any target. The manifest (`build.yaml`) stores tool requirements, settings paths, network domain groups, and credential declarations. After generating artifacts, `cc-deck build run` executes them without Claude Code. Use `/cc-deck.capture --all` to auto-accept all proposals without prompting.

Before generating a Containerfile, the build probes the base image to discover its OS, package manager, and pre-installed tools. This means switching your manifest's base image (from Fedora to UBI to Debian) produces a working build on the first attempt. Tools already present at a compatible version are skipped. Incompatible versions are shadowed via `/usr/local/bin`. Probe results are cached by image digest so repeat builds add sub-second overhead. Run `cc-deck build probe <image>` standalone to inspect any base image, or add `probe_tools:` to the manifest for custom tool checks.

Fixed Containerfile layers (the cc-deck/Zellij/Claude Code stack, shell finalization, footer) are rendered from Go templates during `build init`. Claude Code copies these snippets verbatim and generates only the variable parts between them, keeping structural layers deterministic across builds. After upgrading cc-deck, run `cc-deck build refresh` to re-extract updated snippets without overwriting your manifest.

Tools with non-standard install paths (Go at `/usr/local/go/bin`, Rust/Cargo at `$HOME/.cargo/bin`) are detected from the manifest and their paths are prepended to `.bashrc` and `.zshrc` during image generation. This ensures tools remain accessible in interactive shell sessions even when login initialization resets the Docker `ENV PATH`. The build system maintains an internal registry that maps tool names to their install paths, so adding support for a new tool requires a single registry entry with no template changes.

For OpenShell targets, a `getifaddrs` shim is compiled into the image automatically. The shim works around the OpenShell supervisor's seccomp filter that blocks `AF_NETLINK` sockets, which would otherwise cause Claude Code (Node.js) to crash with `getifaddrs returned an error` on API calls.

For OpenShell targets, the build generates a `policy.yaml` with network restrictions. Package registry endpoints (crates.io, proxy.golang.org, npmjs.org, pypi.org) are added automatically when the corresponding tools appear in the manifest. MCP server endpoints are also included when the manifest contains MCP entries with an `endpoint` field (in `host:port` format). The capture command extracts these endpoints from HTTP/SSE server URLs and `mcp-remote` arguments automatically. After a successful build, cc-deck stamps the image with a `dev.cc-deck.policy-layer` label that records which layer contains the policy file. At `ws new` time, the policy is extracted directly from the OCI image (using the label for fast single-layer fetch, or falling back to a full layer scan for unlabeled images). This means you no longer need the original build directory on the host to create sandboxes. If extraction fails, pass `--policy` to provide the policy file manually. Credentials for OpenShell are declared in the manifest without storing secrets (`credentials: [{type: claude-vertex}, {type: github}]`). At `ws new` time, cc-deck resolves values from your host environment and creates OpenShell providers on the gateway. Supported types: `claude`, `claude-vertex`, `github`, `gitlab`, `openai`, `nvidia`, `generic`. For Vertex AI (`claude-vertex`), cc-deck creates an OpenShell `google-cloud` provider via `--from-gcloud-adc`, which runs a GCE metadata emulator inside the sandbox. GCP credentials never enter the sandbox process. Claude Code env vars (`CLAUDE_CODE_USE_VERTEX`, `ANTHROPIC_VERTEX_PROJECT_ID`, `CLOUD_ML_REGION`) are still injected as non-secret configuration.

### Network filtering

Containerized sessions can restrict outbound network access to specific domains, preventing code or secret exfiltration. Domain groups (`python`, `rust`, `github`, etc.) describe allowed domains by ecosystem. See the [network filtering](#network-filtering-1) section for setup, domain groups, and customization.

### Voice relay

Voice relay lets you dictate into any workspace session using local speech-to-text via whisper.cpp. Audio stays on your machine. A note indicator in the sidebar shows connection status. Toggle mute from the sidebar (`Alt+v`) or the voice TUI (`m`). Say "send" to submit a prompt.

```bash
brew install whisper-cpp
cc-deck ws voice --setup
cc-deck ws voice my-project
```

### Multi-agent support

cc-deck supports multiple AI coding agents through a pluggable Agent interface. Each agent gets automatic detection, hook installation, and event translation.

| Agent | Indicator | Integration |
|-------|-----------|-------------|
| Claude Code | `[CC]` | Hook events via `settings.json` |
| OpenCode | `[OC]` | TypeScript plugin via `~/.config/opencode/plugins/` |

When sessions from different agents are active, the sidebar shows agent indicators (`[CC]`, `[OC]`) before each session name. With a single agent type, indicators are hidden.

Use `cc-deck hook --raw` to send pre-normalized JSON payloads from custom integrations.

### Credential transport

Each agent declares its supported auth modes via `CredentialSpecs()`.
When creating a workspace, cc-deck detects which modes have credentials available on the host and lets you choose:

```bash
# Auto-detect auth mode (prompts if multiple available)
cc-deck ws new my-project --type container --agent claude

# Explicit auth mode
cc-deck ws new my-project --type container --agent claude --auth-mode vertex

# OpenCode with OpenAI credentials
cc-deck ws new my-project --type container --agent opencode --auth-mode openai
```

The selected auth mode is persisted in the workspace definition and shown in `cc-deck ws ls`.
Credentials are validated at workspace start before any container or remote session is created.
To switch auth mode on an existing workspace:

```bash
cc-deck ws update my-project --auth-mode bedrock
```

For K8s workspaces where credentials come from Secrets or external providers, mark the workspace as externally provided to skip host-side validation.

### Multi-platform

Run cc-deck locally with Zellij, in Podman containers, or on Kubernetes clusters with persistent StatefulSet-backed workspaces. OpenShift is detected automatically. The sidebar works the same everywhere.

---

## Usage

```bash
zellij --layout cc-deck
```

Or set as default in `~/.config/zellij/config.kdl`:

```kdl
default_layout "cc-deck"
```

## Layout variants

Three layout styles are installed:

| Layout | Command | Description |
|--------|---------|-------------|
| `standard` | `zellij --layout cc-deck` | Sidebar + tab-bar + status-bar (default) |
| `minimal` | `zellij --layout cc-deck-minimal` | Sidebar + compact-bar |
| `clean` | `zellij --layout cc-deck-clean` | Sidebar only, no bars |

To change the default variant:

```bash
cc-deck config plugin install --layout minimal --force
```

## Keyboard shortcuts

### Global (from any tab)

| Key | Action |
|-----|--------|
| `Alt+s` | Open session list / cycle through sessions |
| `Alt+a` | Jump to next session needing attention |
| `Alt+w` | Jump to next working session |

### Session list (navigation mode)

| Key | Action |
|-----|--------|
| `j` / `↓` | Move cursor down |
| `k` / `↑` | Move cursor up |
| `Enter` | Switch to selected session |
| `Esc` | Cancel (return to original session) |
| `r` | Rename session |
| `d` | Delete session (with confirmation) |
| `p` | Pause/unpause session |
| `n` | New tab |
| `S` | Sort sessions by activity |
| `J` / `K` | Move session down/up in sort order |
| `/` | Search/filter by name |
| `?` | Show keyboard help |

### Mouse

| Action | Effect |
|--------|--------|
| Left-click session | Switch to that session |
| Right-click session | Rename session |
| Click [+] | New tab |

## Customizing keybindings

### Plugin shortcuts (Alt+s, Alt+a)

Edit the plugin config in the layout file (`~/.config/zellij/layouts/cc-deck.kdl`):

```kdl
plugin location="file:~/.config/zellij/plugins/cc_deck.wasm" {
    mode "sidebar"
    navigate_key "Super s"    // default: "Alt s"
    attend_key "Super n"      // default: "Alt a"
    working_key "Alt w"       // default: "Alt w"
    done_timeout "300"        // seconds before Working to Done and Done to Idle (default: 300)
    idle_fade_secs "3600"     // idle indicator fade duration in seconds (default: 3600)
    auto_pause_secs "3600"    // auto-pause after idle for this many seconds (default: 3600, 0 to disable)
    attend_cycle_ms "2000"    // rapid-cycle window for attend/working in ms (default: 2000, 0 to disable)
}
```

Key syntax follows [Zellij key format](https://zellij.dev/documentation/keybindings.html): `Alt`, `Ctrl`, `Super` (Cmd on macOS), `Shift` as modifiers, followed by the key character.

After editing, restart Zellij to apply.

> **Note**: `make install` overwrites the managed layout files (`cc-deck.kdl`, `cc-deck-standard.kdl`, `cc-deck-clean.kdl`). Use a **personal layout file** to preserve custom keybindings across reinstalls.

### Personal layout (recommended)

Create a personal layout that is not overwritten by `make install`:

```bash
cp ~/.config/zellij/layouts/cc-deck.kdl ~/.config/zellij/layouts/cc-deck-personal.kdl
```

Edit `~/.config/zellij/layouts/cc-deck-personal.kdl` with your custom keys:

```kdl
plugin location="file:~/.config/zellij/plugins/cc_deck.wasm" {
    mode "sidebar"
    navigate_key "Super s"
    attend_key "Super n"
}
```

Then set it as default in `~/.config/zellij/config.kdl`:

```kdl
default_layout "cc-deck-personal"
```

### Using Cmd keys (macOS + Ghostty)

To use Cmd-based shortcuts, configure Ghostty to pass Cmd keys through to Zellij:

**Ghostty** (`~/.config/ghostty/config`):

```
keybind = cmd+s=unbind
keybind = cmd+n=unbind
```

## Session states

| Icon | State | Description |
|------|-------|-------------|
| ○ | Init | Session detected, not yet producing output |
| ● | Working | Actively generating output or calling tools |
| ⚠ | Waiting (Permission) | Needs user permission to proceed (highest attend priority) |
| ⚠ | Waiting (Notification) | Paused with informational notification |
| ✓ | Done | Task completed (green, fades to grey over 5 minutes) |
| ✓ | Agent Done | Sub-agent completed |
| ○ | Idle | Waiting for user input (fades to dark grey over 1 hour) |
| ⏸︎ | Paused | Excluded from attend cycling, name dimmed |

### Session lifecycle

Sessions progress through states based on activity timeouts:

```
Working ──[5m idle]──> Done ──[5m]──> Idle ──[1h]──> Paused
```

- **Working to Done**: When no hook events arrive for 5 minutes, the session is considered complete. This is a fallback since Claude Code does not always fire the `Stop` hook on natural response completion.
- **Done to Idle**: After 5 more minutes, the green checkmark fades to a grey circle.
- **Idle to Paused**: After 1 hour of inactivity, the session auto-pauses. Paused sessions are excluded from attend cycling and hook processing.
- **Auto-unpause**: Switching to a paused session (via click, navigate, or attend) unpauses it automatically.

### Fading indicators

Session indicators use time-aware color fading to show freshness:

- **Done** (✓): Fades from bright green to light grey over 5 minutes
- **Idle** (○): Fades from light grey to dark grey over 1 hour

The fade follows a square-root curve: changes are most visible in the first few minutes and taper off.

## Smart attend (Alt+a)

Uses exclusive tiers. Only the highest non-empty tier is cycled:

1. **⚠ Waiting** (permission first, then notification, oldest first). When waiting sessions exist, Alt+a cycles only among those.
2. **✓ Done** (most recently finished first). Only used when no waiting sessions exist.
3. **○ Idle/Init** (tab order). Only used when nothing else needs attention.
4. **Skips**: Working and Paused sessions are never attended.

Subsequent presses round-robin within the selected tier. If the current session is already the attend target, it skips to the next candidate.

## Working jump (Alt+w)

Cycles through sessions that are actively running, ordered by most recently active first. Only Working sessions (purple ●) are included. Waiting sessions are excluded because they need attention, which is Alt+a's job.

Rapid presses within 2 seconds cycle through all working sessions without revisiting. After a 2-second pause, the next press resets to the most recent working session.

## Network filtering

Containerized sessions can restrict outbound network access to specific domains, preventing code or secret exfiltration from YOLO-mode agents.

### Quick setup

Add a `network` section to your `build.yaml`:

```yaml
network:
  allowed_domains:
    - github
    - python
    - golang
```

Then create a compose workspace with network filtering:

```bash
cc-deck ws new my-session --type compose --allowed-domains python,github
```

The session container runs on an internal network with all traffic routed through a tinyproxy sidecar that only allows the specified domains.

### Domain groups

Built-in groups cover common ecosystems. Run `cc-deck config domains list` to see all available groups:

| Group | Covers |
|-------|--------|
| `python` | pypi.org, files.pythonhosted.org |
| `nodejs` | registry.npmjs.org, yarnpkg.com |
| `rust` | crates.io, static.crates.io |
| `golang` | proxy.golang.org, sum.golang.org |
| `github` | github.com, ghcr.io, githubusercontent.com |
| `gitlab` | gitlab.com, registry.gitlab.com |
| `docker` | registry-1.docker.io, auth.docker.io |
| `quay` | quay.io, cdn.quay.io |

Backend domains (Anthropic or Vertex AI) are included automatically.

### Customizing domain groups

Create `~/.config/cc-deck/domains.yaml` to extend or override built-in groups:

```bash
cc-deck config domains init    # Seed config with commented built-in definitions
```

```yaml
# Extend built-in python group with internal registry
python:
  extends: builtin
  domains:
    - pypi.internal.corp

# Create a custom group
company:
  domains:
    - artifacts.internal.corp
    - git.internal.corp
```

### Create-time domain overrides

```bash
# Specify domain groups when creating a compose workspace
cc-deck ws new my-session --type compose --allowed-domains rust,github

# Add or remove domains at runtime on a running session
cc-deck config domains add my-session rust
cc-deck config domains remove my-session docker
```

### Debugging blocked domains

```bash
cc-deck config domains blocked my-session        # Show denied requests
cc-deck config domains add my-session pypi.org   # Add domain at runtime
cc-deck config domains show python               # Inspect a group's domains
```

### OpenShell policy components

For OpenShell targets, `build refresh` assembles `openshell/policy.yaml` from declarative YAML component files. Components are loaded from three tiers in precedence order:

1. **Embedded** (built into the binary): Claude Code, GitHub, and package registry endpoints
2. **Cached catalog** (`.cc-deck/setup/openshell/components/`): fetched by `capture`
3. **User-local** (`.cc-deck/setup/openshell/policies/`): project-specific custom endpoints

The assembly is deterministic: the same manifest with the same components always produces identical output. Components declare match conditions (`always`, `tools`, `credentials`) and are included only when their conditions match the manifest.

Binary paths for network policy entries are discovered automatically using a two-pass build process. The first pass builds the image without binary restrictions. A probe step then runs `which` and `find` inside the built image to discover actual binary locations. The second pass rebuilds with the corrected policy containing probed paths and runtime glob patterns. This approach works regardless of install method, base image, or tool version. Components with explicit `binaries` fields are preserved as-is, providing an override mechanism for custom installations.

Each component YAML can optionally declare `probe_binaries` (binary names to search for) and `runtime_globs` (glob patterns for binaries created at runtime, such as Python venvs or Rust toolchains). If a probe or second-pass build fails, the first-pass image is retained with a `:probe-debug` tag for inspection. For the full workflow and troubleshooting, see the [Build Command](docs/modules/using/pages/build.adoc) and [Policy Components](docs/modules/using/pages/policy-components.adoc) guides.

```bash
cc-deck build refresh    # Assemble policy from components + manifest
```

### OpenShell SDK

OpenShell workspace operations (create, attach, exec, push, pull, delete) use the OpenShell Go SDK (`github.com/rhuss/openshell-sdk-go`) instead of shelling out to the `openshell` CLI binary. The SDK communicates with the gateway via gRPC directly, providing typed errors and eliminating the CLI binary as a runtime dependency.

The git channel (`ext::openshell` transport) still uses the CLI binary for `git push`/`fetch` operations, since git's ext protocol requires a command-line tool for stdin/stdout piping.

### Egress recording

If you are unsure which domains your tools contact at runtime, `build record` launches an interactive session with a DNS logger sidecar that captures all outbound queries. On exit, cc-deck deduplicates the domains, filters out infrastructure noise (Podman internals, mDNS, reverse DNS), matches them against the catalog, and appends any new domains to `build.yaml` `network.allowed_domains`. Run `build refresh` afterward to regenerate the policy.

```bash
cc-deck build record     # Start a recording session
cc-deck build refresh    # Regenerate policy with recorded domains
```

See the [egress recording guide](docs/modules/using/pages/egress-recording.adoc) for a full walkthrough.

### Custom endpoints

To add custom endpoints, create a component file in `.cc-deck/setup/openshell/policies/`:

```yaml
key: internal_api
name: Internal API
match:
  always: true
endpoints:
  - host: api.internal.corp
    port: 8443
```

## Workspace management

The `cc-deck ws` command group manages Claude Code sessions across all supported backends.

| Type | What it does |
|------|-------------|
| `local` | Zellij session on the host machine (default) |
| `container` | Single container managed by Podman |
| `compose` | Multi-container setup via podman-compose |
| `ssh` | Remote machine over SSH |
| `k8s-deploy` | Persistent Kubernetes workspace with StatefulSet |
| `openshell` | OpenShell sandbox with security policies |

| Subcommand | Description |
|------------|-------------|
| `cc-deck ws new` | Create a new workspace |
| `cc-deck ws attach` | Attach to a workspace (auto-starts infrastructure if needed) |
| `cc-deck ws kill-session` | Kill the Zellij session without affecting infrastructure |
| `cc-deck ws start` | Start infrastructure for container/compose/k8s workspaces |
| `cc-deck ws stop` | Stop infrastructure (kills session first, then stops container/pod) |
| `cc-deck ws delete` | Delete a workspace and its resources |
| `cc-deck ws list` | List all workspaces with type-appropriate state display |
| `cc-deck ws status` | Show detailed status of a workspace |
| `cc-deck ws prune` | Remove stale project registry entries |

### Project-local configuration

Workspace definitions can live inside the project repository in a `.cc-deck/` directory at the git root. Team members can clone and create matching workspaces without manual flag passing.

```bash
# Set up a new project
cc-deck ws new --type compose --image quay.io/cc-deck/cc-deck-demo:latest
git add .cc-deck/ && git commit -m "Add cc-deck workspace config"

# Team member clones and gets the same environment
git clone git@github.com:org/my-api.git && cd my-api
cc-deck ws new     # reads .cc-deck/environment.yaml
cc-deck ws attach  # no name needed inside the project
```

The `.cc-deck/` directory separates committed artifacts from runtime state:

```
.cc-deck/
  environment.yaml    # Committed: declarative definition
  .gitignore          # Committed: ignores status.yaml and run/
  image/              # Committed: build manifest, Containerfile
  status.yaml         # Gitignored: runtime state
  run/                # Gitignored: generated compose files
```

When no workspace name is provided, `cc-deck` looks for `.cc-deck/environment.yaml` at the git root, then walks up the directory tree. All lifecycle commands (attach, status, start, stop, kill) support this implicit resolution.

### Compose workspaces

Compose workspaces use `podman-compose` for multi-container orchestration. They generate runtime artifacts in `.cc-deck/run/` within the project directory.

```bash
cc-deck ws new --type compose
cc-deck ws new --type compose --allowed-domains anthropic,github
cc-deck ws attach
```

The project directory is bind-mounted at `/workspace` by default for bidirectional file sync.

### SSH workspaces

SSH workspaces run Zellij sessions on persistent remote machines. You connect over SSH, work inside the remote Zellij session, and detach when finished. The session continues running on the remote host.

```bash
cc-deck ws new remote-dev --type ssh --host user@dev.example.com
cc-deck ws attach remote-dev
cc-deck ws refresh-creds remote-dev
cc-deck ws push remote-dev ./src
```

Pre-flight checks during creation verify SSH connectivity and offer to install missing tools on the remote.

### Variants

When the same project needs multiple isolated container instances (for example, per-worktree containers), use the `--variant` flag:

```bash
cc-deck ws new --variant auth    # container: cc-deck-my-api-auth
cc-deck ws new --variant bugfix  # container: cc-deck-my-api-bugfix
```

## Test coverage

Coverage measurement for the Rust plugin uses [cargo-llvm-cov](https://github.com/taiki-e/cargo-llvm-cov). Install prerequisites first:

```bash
cargo install cargo-llvm-cov
rustup component add llvm-tools-preview
```

Then use the Makefile targets:

| Target | Description |
|--------|-------------|
| `make coverage` | Generate an HTML report and open it in the browser |
| `make coverage-summary` | Print a per-module coverage table to the terminal |
| `make coverage-json` | Write machine-readable JSON to `cc-zellij-plugin/target/llvm-cov/coverage.json` |

Coverage runs tests on the native target, not wasm32. Code behind `#[cfg(target_family = "wasm")]` guards is unreachable during measurement.

### Integration tests

The plugin includes integration tests that exercise `SidebarRendererPlugin` and `ControllerPlugin` through their `ZellijPlugin` trait methods (`load`, `update`, `pipe`) with synthetic events. These tests verify the full event dispatch chain without a running Zellij instance.

Integration tests cover render payload pipeline, hook event processing, discovery protocol handshake, action dispatch, permission deferral, mode transitions, error handling, and protocol roundtrips.

```bash
make test                # all tests (unit + Rust)
make test-e2e            # CLI end-to-end tests
make test-images         # build and probe all base images
make test-images-quick   # probe default base images only
make test-images-session # session smoke test (requires API key)
cargo test --lib integration_tests   # plugin integration tests only
```

The image probe suite builds container images from each entry in `base-images.yaml` and validates that key binaries, user setup, and permissions are correct.
Filter to a single base with `make test-images BASE=nvidia-upstream`.

## Uninstall

```bash
cc-deck config plugin remove
```

## Project structure

```
cc-zellij-plugin/   Zellij sidebar plugin (Rust, WASM)
cc-deck/            CLI tool (Go)
docs/               Antora documentation source
demos/              Demo recording system
demo-image/         Demo container image build
base-image/         Base container image build
base-images.yaml    Base image registry (tested targets)
specs/              Feature specifications (SDD)
```

## Known issues

### Duplicate controller instances (Zellij bug)

Zellij 0.43 and 0.44 occasionally create two WASM instances of a background plugin when `load_plugins` races with the `AddClient` event. This causes duplicate keybinding registrations and render broadcasts.

cc-deck mitigates this with a leader election protocol. On startup, each controller broadcasts a probe with its plugin ID. The instance with the lowest ID activates within two seconds; the other stays dormant. The leader sends a periodic heartbeat (every 30 seconds) so the dormant instance can detect failure and re-activate.

The only visible effect is a two-second delay before keybindings become active on a fresh Zellij start, which overlaps with Zellij's own initialization time.

## Contributing

Contributions are welcome. See [CONTRIBUTING.md](CONTRIBUTING.md) for the development process, including how Spec-Driven Development is used for larger changes.

## Feature specifications

cc-deck follows [Spec-Driven Development](CONTRIBUTING.md#spec-driven-development). Each feature starts with a specification before implementation. Current specs:

| ID | Feature | Status |
|----|---------|--------|
| [002](specs/002-cc-deck-k8s/) | Kubernetes CLI | Planned |
| [012](specs/012-sidebar-plugin/) | Sidebar Plugin | Implemented |
| [013](specs/013-keyboard-navigation/) | Keyboard Navigation & Global Shortcuts | Implemented |
| [014](specs/014-pause-and-help/) | Session Pause Mode & Keyboard Help | Implemented |
| [015](specs/015-session-save-restore/) | Session Save and Restore | Planned |
| [016](specs/016-k8s-integration-tests/) | K8s Integration Tests | Planned |
| [017](specs/017-base-image/) | Base Container Image | Implemented |
| [018](specs/018-build-manifest/) | Build Pipeline | In Progress |
| [019](specs/019-docs-landing-page/) | Documentation & Landing Page | In Progress |
| [020](specs/020-demo-recordings/) | Demo Recording System | In Progress |
| [021](specs/021-release-process/) | Release Process | Implemented |
| [022](specs/022-network-filtering/) | Network Security & Domain Filtering | In Progress |
| [023](specs/023-env-interface/) | Environment Interface and CLI | Planned |
| [024](specs/024-container-env/) | Container Environment | Implemented |
| [025a](specs/025-sidebar-state-refresh/) | Sidebar State Refresh on Reattach | In Progress |
| [025b](specs/025-compose-env/) | Compose Environment | In Progress |
| [026](specs/026-project-local-config/) | Project-Local Config | Implemented |
| [027](specs/027-cli-restructuring/) | CLI Command Restructuring | In Progress |
| [028](specs/028-k8s-deploy/) | K8s Deploy Environment | Implemented |
| [030](specs/030-single-instance-arch/) | Single Instance Architecture | Implemented |
| [031](specs/031-single-binary-merge/) | Single Binary Merge | Implemented |
| [033](specs/033-ssh-environment/) | SSH Remote Execution | In Progress |
| [034](specs/034-unified-setup-command/) | Build Command | Planned |
| [036](specs/036-setup-run-command/) | Setup Run Command | Implemented |
| [037](specs/037-env-lifecycle-fixes/) | Environment Lifecycle Fixes | In Progress |
| [038](specs/038-workspace-repos/) | Workspace Repos | In Progress |
| [039](specs/039-cli-rename-ws-build/) | CLI Rename: Workspace & Build | In Progress |
| [041](specs/041-workspace-channels/) | Workspace Channels | In Progress |
| [042](specs/042-voice-relay/) | Voice Relay | Implemented |
| [045](specs/045-voice-sidebar-integration/) | Voice Sidebar Integration | In Progress |
| [056](specs/056-openshell-build-target/) | OpenShell Build Target | In Progress |
| [058](specs/058-openshell-credential-injection/) | OpenShell Credential Injection | In Progress |
| [075](specs/075-openshell-sdk-migration/) | OpenShell SDK Migration | In Progress |
