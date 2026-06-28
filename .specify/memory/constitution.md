# cc-deck Constitution

## Core Principles

### I. Every feature MUST include tests and documentation

A feature is NOT complete until:
1. Tests exist for the new code (unit tests at minimum, integration tests when touching external tools)
2. README.md is updated with user-facing changes
3. CLI reference (`docs/modules/reference/pages/cli.adoc`) covers new commands/flags
4. Antora docs have a guide page for substantial features
5. Configuration reference (`docs/modules/reference/pages/configuration.adoc`) covers new config options or file locations
6. All documentation uses the prose plugin with the `cc-deck` voice profile

Documentation updates MUST happen as part of the same branch or commit that delivers the feature, not as a follow-up task.
When a substantial feature is merged without documentation, treat it as a blocking issue before the next feature begins.

### II. Interface implementations MUST satisfy behavioral contracts

When implementing a new backend for an existing interface (e.g., new Environment type):
1. Read the existing implementation(s) to understand full behavior
2. Cross-reference `specs/023-env-interface/contracts/environment-interface.md` for behavioral requirements
3. If the contract lacks requirements for a behavior you see in existing code, add them before implementing

### III. Build and tool rules

- NEVER run `go build` or `cargo build` directly. Use `make install`, `make test`, `make lint`
- XDG paths: Use `internal/xdg` package (NOT `adrg/xdg`). Paths are `~/.config/cc-deck/` and `~/.local/state/cc-deck/` on all platforms
- Container runtime: Use `podman` exclusively (never Docker)

### IV. Plugin debug logging

The Zellij plugin uses opt-in debug logging via a WASI filesystem flag.

**Enabling debug**:
The `debug_enabled` marker file must exist in the plugin's WASI `/cache/` directory.
On macOS the host path is:
```
~/Library/Caches/org.Zellij-Contributors.Zellij/file:~/.config/zellij/plugins/cc_deck.wasm/plugin_cache/debug_enabled
```
To enable: `touch` the file at the host path above (it already exists if debug was enabled before).
To disable: remove the file. The flag is checked once on plugin load.

**Log location**:
Debug output is written to `/cache/debug.log` inside the WASI sandbox.
On macOS the host path is:
```
~/Library/Caches/org.Zellij-Contributors.Zellij/file:~/.config/zellij/plugins/cc_deck.wasm/plugin_cache/debug.log
```

**Usage notes**:
- Logging is buffered (flushed on timer tick or when buffer exceeds 50 lines)
- Multiple plugin instances (controller + sidebars) share the same log file
- Truncate with `: >` before reproducing an issue to keep output focused
- The log can grow large quickly; disable when not actively debugging

### V. Claude Code command files are executable code

Files under `internal/build/commands/*.md` are Claude Code skills executed
during `cc-deck build run`. They contain live instructions that directly
affect Containerfile generation and build behavior.

- NEVER dismiss review comments on `.md` command files as "just documentation"
- Treat command file instructions with the same rigor as Go or Rust source
- Verify claims against what Claude Code would actually do when executing
  those instructions
- Bot review comments on command files are as valid as comments on compiled code

## Governance

Constitution principles are enforced in CLAUDE.md and apply to ALL code changes, whether from a spec workflow or ad-hoc.
Amendments require updating both this file and the Constitution Principles section of CLAUDE.md.

**Version**: 1.3 | **Ratified**: 2026-03-30 | **Last Amended**: 2026-06-28
