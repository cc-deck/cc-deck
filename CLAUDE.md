# cc-mux Development Guidelines

## Content Creation (MANDATORY)

When creating or editing ANY documentation content (AsciiDoc, Markdown, landing page text):
- **ALWAYS use the prose plugin** with the `cc-deck` voice profile (`.style/voice.yaml`)
- **NEVER use em-dashes or en-dashes** (per global CLAUDE.md rules)
- **One sentence per line** in AsciiDoc files (semantic line breaks)
- Use the cc-deck voice: professional, thorough, no contractions, terminal-native analogies
- Run `/prose:check` before committing documentation changes

## Commands

```bash
make verify    # Run all tests + linting (Go and Rust)
make test      # Run all tests only
make lint      # Run all linters only
make install   # Build and install plugin into Zellij
```

<!-- SPECKIT START -->
<!-- SPECKIT END -->

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
5. **Configuration reference** (`docs/modules/reference/pages/configuration.adoc`) covers new config options or file locations
6. All documentation uses the **prose plugin** with the `cc-deck` voice profile

Documentation updates MUST happen as part of the same branch or commit that delivers the change, not as a follow-up task.
This applies to ALL user-visible changes, not just spec-driven features: new CLI subcommands, new flags, changed default behavior, new image layers, workarounds baked into generated artifacts, and bug fixes that alter observable behavior.
When a user-visible change is merged without documentation, treat it as a blocking issue before the next change begins.

### Interface implementations MUST satisfy behavioral contracts

When implementing a new backend for an existing interface (e.g., new Environment type):
1. Read the existing implementation(s) to understand full behavior
2. Cross-reference `specs/023-env-interface/contracts/environment-interface.md` for behavioral requirements
3. If the contract lacks requirements for a behavior you see in existing code, add them before implementing

### Build and tool rules

- **NEVER** run `go build` or `cargo build` directly. Use `make install`, `make test`, `make lint`
- XDG paths: Use `internal/xdg` package (NOT `adrg/xdg`). Paths are `~/.config/cc-deck/` and `~/.local/state/cc-deck/` on all platforms
- Container runtime: Use `podman` exclusively (never Docker)

### Plugin debug logging

- Enable: `touch ~/Library/Caches/org.Zellij-Contributors.Zellij/file:/Users/$USER/.config/zellij/plugins/cc_deck.wasm/plugin_cache/debug_enabled`
- Log: `~/Library/Caches/org.Zellij-Contributors.Zellij/file:/Users/$USER/.config/zellij/plugins/cc_deck.wasm/plugin_cache/debug.log`
- CAUTION: The path uses `file:/Users/$USER/...` (expanded), NOT `file:~/.config/...` (literal tilde). A literal tilde path is a different directory the plugin does not read.
- Truncate before reproducing: `: > <log path>`
- Flag checked once on plugin load; requires Zellij restart to take effect
- The debug logger silently drops re-entrant log calls to avoid panics in WASI's single-threaded mutex.

### Claude Code command files are executable code

Files under `internal/build/commands/*.md` are Claude Code skills executed during `cc-deck build run`. They contain live instructions that directly affect Containerfile generation and build behavior. Treat them with the same rigor as Go or Rust source code. Bot review comments on command files are as valid as comments on compiled code.

<!-- MANUAL ADDITIONS END -->
