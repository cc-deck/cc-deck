<!--
Sync Impact Report
- Version change: 1.9.0 → 1.10.0
- Modified principles: Renumbered VIII-XII → VIII-XIII (inserted VII)
- Added sections: Principle VII (Interface Behavioral Contracts), Principle XIII (XDG Paths)
- Removed sections: none
- Templates requiring updates:
  - .specify/templates/spec-template.md ✅ no update needed
  - .specify/templates/plan-template.md ✅ no update needed
  - .specify/templates/tasks-template.md ✅ no update needed
  - specs/023-env-interface/contracts/environment-interface.md ✅ updated with behavioral requirements
- Follow-up TODOs:
  - Update references to old principle numbers in CLAUDE.md if any
  - The spec review checklist should include "cross-reference existing implementations"
-->

# cc-deck Constitution

## Core Principles

### I. Two-Component Architecture

cc-deck consists of a Rust WASM plugin (`cc-zellij-plugin/`) and a Go CLI (`cc-deck/`). Features that touch both components must be coordinated. The WASM plugin runs inside Zellij, the CLI runs on the host.

### II. Plugin Installation (NON-NEGOTIABLE)

ALWAYS run `make install` from the **project root directory** (where the Makefile lives). NEVER run it from a subdirectory like `cc-zellij-plugin/` or `cc-deck/`. NEVER use `make dev-install`, ad-hoc `cp` commands, or manual file copies. The plugin has been silently not picked up multiple times with shortcuts. `make install` does a release build and runs `cc-deck plugin install --force` which handles WASM binary, layout, hooks, and settings.json correctly.

After installation, ALWAYS kill all Zellij sessions before testing:
```bash
make install
zellij kill-all-sessions -y 2>/dev/null; zellij --layout cc-deck
```

Running Zellij sessions keep compiled plugins in memory. Clearing disk cache does NOT affect running sessions.

### III. WASM Filename Convention

The WASM binary is ALWAYS `cc_deck.wasm` (underscore), matching Cargo's output. The layout files reference this name. Never use `cc-deck.wasm` (hyphen).

### IV. WASM Host Function Gating

All Zellij host functions (`run_command`, `pipe_message_to_plugin`, `set_selectable`, `focus_plugin_pane`, `reconfigure`, etc.) MUST be `#[cfg(target_family = "wasm")]` gated with no-op stubs for native builds. They link-fail on native targets. Tests run on native.

### V. Zellij API Research Order (NON-NEGOTIABLE)

When a Zellij plugin API feature doesn't work as expected, research in this order:

1. **Official documentation first**: Check `zellij.dev` (plugin API commands, keybindings, possible actions, pipe documentation)
2. **Zellij source code second**: Check the zellij-tile SDK source and zellij-utils KDL parser for exact syntax and available options
3. **Reference plugins third**: Check existing plugins for working patterns

Do NOT guess at API syntax or invent approaches without verifying against the documentation and source. Many hours have been wasted on incorrect API usage (wrong `MessagePlugin` URL, `Run` creating visible panes, `KeybindPipe` without targets).

### VI. Build via Makefile Only (NON-NEGOTIABLE)

NEVER run `go build` or `cargo build` directly. ALWAYS use the Makefile targets from the **project root**. The Go project directory is named `cc-deck/`, which collides with the binary output name. Running `go build -o cc-deck` inside `cc-deck/` overwrites the directory with a binary, destroying all source files.

Safe commands:
```bash
make install    # Full build (Rust + Go) + install plugin
make test       # Run all tests
make lint       # Run linters
```

If you must build the Go binary in isolation (e.g., for testing a new command), use an explicit output path that does NOT match any directory name:
```bash
cd cc-deck && go build -o /tmp/cc-deck-test ./cmd/cc-deck
```

### VII. Interface Behavioral Contracts (NON-NEGOTIABLE)

When implementing a new backend for an existing interface (e.g., a new `Environment` type), the implementation MUST satisfy the behavioral requirements documented in the interface contract, not just the method signatures. Before writing a new implementation:

1. Read the existing implementation(s) to understand the full behavior (not just the interface signature).
2. Cross-reference the interface contract for documented behavioral requirements.
3. If the contract lacks behavioral requirements, add them before implementing.

Behavioral requirements include error handling patterns, state management (timestamps, status updates), user experience checks (nested session detection), and resource lifecycle (cleanup on failure, layout initialization).

A spec for a new implementation MUST explicitly reference the interface behavioral contract and note any deviations.

### VIII. Simplicity

Follow YAGNI. Don't add features, abstractions, or error handling beyond what's needed. Three similar lines of code is better than a premature abstraction.

### IX. Documentation Freshness (NON-NEGOTIABLE)

A feature is NOT complete until its documentation is updated. ALWAYS update these as part of every feature implementation:

1. **README.md**: Update with user-facing feature descriptions, usage examples, and CLI reference changes. This is mandatory for every feature, no exceptions.
2. **Feature specs table**: Add or update the feature entry in the README's "Feature Specifications" table (see Principle X).
3. **Landing page**: For substantial features (new CLI commands, new deployment modes, new user-visible capabilities), update the landing page. The landing page repo is **`cc-deck/cc-deck.github.io`** (Astro site at https://cc-deck.github.io). If the repo location is unclear or the worktree is not available, ask the user before proceeding.
4. **Antora docs**: If the `docs/` directory exists in the working tree, update relevant Antora modules (quickstarts, reference, etc.).

For larger features (new CLI command groups, new deployment modes, new configuration systems), the documentation MUST include all of the following:

- **User guide page**: A dedicated Antora page in the appropriate module (`running/`, `using/`, `images/`) covering overview, quick start, how it works, and usage examples. Add the page to the module's `nav.adoc`.
- **CLI reference**: Add all new commands, subcommands, and flags to `docs/modules/reference/pages/cli.adoc` with usage examples and flag tables.
- **Configuration reference**: Document new config files, environment variables, or schema fields in the appropriate reference pages (`configuration.adoc`, `manifest-schema.adoc`).
- **Landing page feature card**: Add a feature card to the features section of the Astro landing page at `cc-deck.github.io`.

Use parallel agents to create documentation concurrently when updating multiple files.

Do NOT mark a feature as complete or propose a commit without verifying documentation is current.

### X. Spec Tracking in README

When a new feature specification is created and merged, add it to the "Feature Specifications" table in `README.md` with its ID, title, and status. Update the status column when implementation progresses or completes. The README spec table is the public-facing summary of all design work.

### XI. Release Process

Releases are triggered by pushing a version tag (`v*`). The CI pipeline handles binaries, packages, and Homebrew automatically. However, the following manual steps are required for each release:

1. **Multi-arch container images**: CI builds amd64 only. Run locally from project root:
   ```bash
   make base-image-push
   make demo-image-push
   ```
   This builds and pushes arm64 + amd64 manifests to quay.io/cc-deck.

2. **Post-release version bump**: After the tag is pushed, update version for next development cycle:
   ```bash
   # Update Makefile VERSION and cc-zellij-plugin/Cargo.toml version
   # Commit: "Bump version to X.Y.Z-dev"
   ```

3. **Verify Homebrew formula**: After the release workflow completes, verify:
   ```bash
   brew update
   brew install cc-deck/tap/cc-deck
   cc-deck --version
   ```

When Claude Code triggers a release, execute these steps automatically after confirming the tag push succeeded.

### XII. Prose Plugin for Documentation (NON-NEGOTIABLE)

All documentation text content creation and editing MUST use the **prose plugin** (`/prose:write` for new content, `/prose:rewrite` for editing existing content, `/prose:check` before committing). The voice profile to use is **`cc-deck`** (defined in `.style/voice.yaml`).

This applies to:
- Antora documentation pages (AsciiDoc)
- README sections and feature descriptions
- Landing page copy
- Brainstorming documents and feature specs (prose sections, not code/tables)
- Commit message descriptions and PR bodies for documentation changes

The prose plugin enforces the cc-deck voice: professional, thorough, no contractions, terminal-native analogies, context before commands, and reasoning-first explanations. It also catches AI writing patterns before they reach the repository.

Do NOT write documentation text directly without running it through the prose plugin. The voice profile ensures consistency across all cc-deck documentation.

### XIII. XDG Paths on All Platforms (NON-NEGOTIABLE)

cc-deck MUST use standard XDG Base Directory paths on **all platforms**, including macOS:

- Config: `~/.config/cc-deck/` (`$XDG_CONFIG_HOME/cc-deck/`)
- State: `~/.local/state/cc-deck/` (`$XDG_STATE_HOME/cc-deck/`)
- Data: `~/.local/share/cc-deck/` (`$XDG_DATA_HOME/cc-deck/`)
- Cache: `~/.cache/cc-deck/` (`$XDG_CACHE_HOME/cc-deck/`)

Do NOT use `adrg/xdg`, which maps to platform-native directories on macOS (`~/Library/Application Support/`). This causes user confusion and breaks documentation that references `~/.config` paths.

Instead, use a minimal internal `xdg` helper or compute paths directly:
```go
func configHome() string {
    if v := os.Getenv("XDG_CONFIG_HOME"); v != "" {
        return v
    }
    home, _ := os.UserHomeDir()
    return filepath.Join(home, ".config")
}
```

Environment variable overrides (`$XDG_CONFIG_HOME`, `$XDG_STATE_HOME`) MUST be respected when set. The `~/.config` and `~/.local` defaults apply only when the environment variables are not set.

Files affected by this policy: `config.go`, `state.go`, `definition.go`, `snapshot.go`, `network/config.go`, `main.go`.

### XIV. No Dotfile Nesting

Files inside a dot directory (e.g., `.cc-deck/`) MUST NOT use a dot prefix themselves. The parent directory already signals "hidden/generated." Nested dotfiles are redundant and harder to discover when inspecting directory contents.

- GOOD: `.cc-deck/env`, `.cc-deck/compose.yaml`, `.cc-deck/secrets/`
- BAD: `.cc-deck/.env`, `.cc-deck/.gitkeep`

## Development Workflow

- `make install` for building and installing (NON-NEGOTIABLE, see Principle VI)
- `make test` for running all tests (Go + Rust)
- `make lint` for linting (Go vet + Rust clippy)
- NEVER run `go build` or `cargo build` directly (see Principle VI)
- Commit after each logical task or phase

## Testing

- `cargo test` runs on native target with WASM host function stubs
- Live testing requires Zellij with the cc-deck layout
- Debug logging via `/cache/debug.log` in the WASI filesystem (check `~/Library/Caches/org.Zellij-Contributors.Zellij/`)

## Governance

This constitution supersedes ad-hoc practices. Amendments require updating this file and the project memory.

**Version**: 1.11.0 | **Ratified**: 2026-03-09 | **Last Amended**: 2026-03-22
