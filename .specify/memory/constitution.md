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

## Governance

Constitution principles are enforced in CLAUDE.md and apply to ALL code changes, whether from a spec workflow or ad-hoc.
Amendments require updating both this file and the Constitution Principles section of CLAUDE.md.

**Version**: 1.1 | **Ratified**: 2026-03-30 | **Last Amended**: 2026-04-28
