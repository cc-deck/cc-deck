# <img src="assets/logo/cc-deck-icon.png" alt="cc-deck" width="60" valign="middle" /> &nbsp; cc-deck

[![CI](https://github.com/cc-deck/cc-deck/actions/workflows/ci.yaml/badge.svg)](https://github.com/cc-deck/cc-deck/actions/workflows/ci.yaml)
[![codecov](https://codecov.io/gh/cc-deck/cc-deck/graph/badge.svg)](https://codecov.io/gh/cc-deck/cc-deck)
[![Go](https://img.shields.io/badge/Go-1.25-00ADD8?logo=go)](https://go.dev)
[![Rust](https://img.shields.io/badge/Rust-stable-orange?logo=rust)](https://www.rust-lang.org)
[![Zellij](https://img.shields.io/badge/Zellij-0.43+-green)](https://zellij.dev)
[![License](https://img.shields.io/github/license/cc-deck/cc-deck)](LICENSE)

**The TweetDeck for Claude Code.** A [Zellij](https://zellij.dev) sidebar plugin that monitors, attends to, and orchestrates multiple Claude Code sessions from a single terminal view. Zellij is a modern terminal multiplexer (like tmux, but with a plugin system and built-in layout management).

**[Website](https://cc-deck.github.io)** · **[Documentation](https://cc-deck.github.io/docs/)** · **[Quickstart](#install)** · **[Contributing](CONTRIBUTING.md)**

---

## What is cc-deck?

Managing multiple Claude Code sessions from separate terminals quickly becomes unwieldy. You lose track of which session is waiting for input, which one just finished, and which one needs your attention next.

cc-deck solves this with a real-time sidebar that shows all your sessions and intelligently directs your attention where it matters most.

### Zellij Sidebar Plugin

The sidebar plugin tracks every Claude Code session across tabs. It shows activity status, handles permission requests, and provides keyboard-driven navigation. Smart attend automatically cycles through sessions that need your attention, prioritizing permission requests over completed tasks over idle sessions.

### Custom Container Images

An AI-driven build pipeline analyzes your local environment for tool dependencies, lets you configure shell, Zellij, and Claude Code settings, and generates optimized container images. Four Claude Code commands handle the workflow: extract, settings, build, push.

### Multi-Platform

Run cc-deck locally with Zellij, in Podman containers with mounted source code, or deploy as Deployments on Kubernetes and OpenShift. The sidebar experience is the same everywhere.

## Install

### Homebrew (macOS)

```bash
brew install cc-deck/tap/cc-deck
cc-deck plugin install
```

### Binary Download

Download the latest release from [GitHub Releases](https://github.com/cc-deck/cc-deck/releases):

```bash
# macOS (Apple Silicon)
curl -fsSL https://github.com/cc-deck/cc-deck/releases/latest/download/cc-deck_$(curl -s https://api.github.com/repos/cc-deck/cc-deck/releases/latest | jq -r .tag_name | sed 's/^v//')_darwin_arm64.tar.gz | tar -xz
sudo mv cc-deck /usr/local/bin/
cc-deck plugin install
```

### Linux Packages

```bash
# Fedora / RHEL
sudo dnf install ./cc-deck_*.rpm

# Debian / Ubuntu
sudo apt install ./cc-deck_*.deb
```

Download RPM and DEB packages from [GitHub Releases](https://github.com/cc-deck/cc-deck/releases). After installing, run `cc-deck plugin install` to set up the Zellij plugin and hooks.

### Demo Image (Try Without Installing)

```bash
podman run -it --rm \
  -e ANTHROPIC_API_KEY=sk-ant-... \
  quay.io/cc-deck/cc-deck-demo:latest
```

### Build from Source

```bash
git clone https://github.com/cc-deck/cc-deck.git
cd cc-deck
make install
```

Requires [Zellij](https://zellij.dev) 0.43+, [Go](https://go.dev) 1.22+, and [Rust](https://www.rust-lang.org) stable with `wasm32-wasip1` target.

## Usage

```bash
zellij --layout cc-deck
```

Or set as default in `~/.config/zellij/config.kdl`:

```kdl
default_layout "cc-deck"
```

## Layout Variants

Three layout styles are installed:

| Layout | Command | Description |
|--------|---------|-------------|
| `standard` | `zellij --layout cc-deck` | Sidebar + tab-bar + status-bar (default) |
| `minimal` | `zellij --layout cc-deck-minimal` | Sidebar + compact-bar |
| `clean` | `zellij --layout cc-deck-clean` | Sidebar only, no bars |

To change the default variant:

```bash
cc-deck plugin install --layout minimal --force
```

## Keyboard Shortcuts

### Global (from any tab)

| Key | Action |
|-----|--------|
| `Alt+s` | Open session list / cycle through sessions |
| `Alt+a` | Jump to next session needing attention |

### Session List (navigation mode)

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
| `/` | Search/filter by name |
| `?` | Show keyboard help |

### Mouse

| Action | Effect |
|--------|--------|
| Left-click session | Switch to that session |
| Right-click session | Rename session |
| Click [+] | New tab |

## Customizing Keybindings

### Plugin Shortcuts (Alt+s, Alt+a)

Edit the plugin config in the layout file (`~/.config/zellij/layouts/cc-deck.kdl`):

```kdl
plugin location="file:~/.config/zellij/plugins/cc_deck.wasm" {
    mode "sidebar"
    navigate_key "Super s"    // default: "Alt s"
    attend_key "Super n"      // default: "Alt a"
}
```

Key syntax follows [Zellij key format](https://zellij.dev/documentation/keybindings.html): `Alt`, `Ctrl`, `Super` (Cmd on macOS), `Shift` as modifiers, followed by the key character.

After editing, restart Zellij to apply.

> **Note**: `make install` overwrites the managed layout files (`cc-deck.kdl`, `cc-deck-standard.kdl`, `cc-deck-clean.kdl`). Use a **personal layout file** to preserve custom keybindings across reinstalls.

### Personal Layout (Recommended)

Create a personal layout that won't be overwritten by `make install`:

```bash
cp ~/.config/zellij/layouts/cc-deck.kdl ~/.config/zellij/layouts/cc-deck-personal.kdl
```

Edit `~/.config/zellij/layouts/cc-deck-personal.kdl` and add your custom keys to the plugin blocks:

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

Now `zellij` (without `--layout`) uses your personal keybindings automatically.

### Using Cmd Keys (macOS + Ghostty)

To use Cmd-based shortcuts, configure Ghostty to pass Cmd keys through to Zellij:

**Ghostty** (`~/.config/ghostty/config`):

```
keybind = cmd+s=unbind
keybind = cmd+n=unbind
```

## Session States

| Icon | State | Description |
|------|-------|-------------|
| ○ | Init | Session detected, Claude Code not yet producing output |
| ● | Working | Actively generating output or calling tools |
| ⚠ | Waiting (Permission) | Needs user permission to proceed (highest attend priority) |
| ⚠ | Waiting (Notification) | Paused with informational notification |
| ○ | Idle | Running but waiting for user input |
| ✓ | Done | Task completed |
| ✓ | Agent Done | Sub-agent completed |
| ⏸ | Paused | Excluded from attend cycling, name dimmed |

## Smart Attend (Alt+a)

Cycles through sessions in priority order:

1. **Permission requests** (oldest first, most urgent)
2. **Idle/done/init sessions** (newest first)
3. **Skips** working sessions and paused sessions

Subsequent presses cycle round-robin through the list.

## Build from Source

```bash
# Prerequisites
rustup target add wasm32-wasip1
# Go 1.22+ required

# Build and install
make install
```

## Uninstall

```bash
cc-deck plugin remove
```

## Project Structure

```
cc-zellij-plugin/   Zellij sidebar plugin (Rust, WASM)
cc-deck/            CLI tool (Go)
docs/               Antora documentation source
demos/              Demo recording system
demo-image/         Demo container image build
base-image/         Base container image build
specs/              Feature specifications (SDD)
```

## Contributing

Contributions are welcome. See [CONTRIBUTING.md](CONTRIBUTING.md) for the development process, including how we use Spec-Driven Development for larger changes.

## Feature Specifications

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
