# Brainstorm: Tool PATH Restoration in Container Builds

**Date:** 2026-05-25
**Status:** active

## Problem Framing

Tools installed during container image builds (Go, Rust, Node.js) add their paths via `ENV PATH` in the Containerfile. When the sandbox starts a login shell, the shell initialization sequence can reset PATH (e.g., `/etc/environment` on Debian/Ubuntu sets a baseline PATH that overwrites the Docker ENV). The curated `.zshrc` then extends PATH with `$PATH`, but the tool install paths are already gone.

This causes `command not found` errors for tools like `go`, `cargo`, and `node` inside the sandbox, even though they are installed correctly.

The problem affects both OpenShell and container targets, since both use the same Containerfile generation pipeline.

## Approaches Considered

### A: Tool Path Registry in ContainerfileData (Chosen)

Add a `ToolPaths []string` field to `ContainerfileData`. The `ContainerDataForTarget()` function populates it by mapping manifest tools to their known install paths using a registry table (e.g., `go` -> `/usr/local/go/bin`, `cargo`/`rust` -> `$HOME/.cargo/bin`). The `05-shell-finalize.tmpl` template renders a `RUN` step that prepends these paths to `.zshrc` and `.bashrc`.

- Pros: Single source of truth in the build package. Template is clean. Adding a new tool means adding one line to the registry. Works for both OpenShell and container targets.
- Cons: Requires maintaining a tool-to-path mapping table. New tools not in the registry get no PATH entry (but fail visibly).

### B: Auto-collect from ENV PATH in rendered snippets

After rendering all Containerfile snippets, parse the output for `ENV PATH=` lines, extract the prepended directories, and pass them to the shell-finalize template.

- Pros: Fully automatic, no registry needed.
- Cons: Fragile text parsing. Couples finalize step to snippet rendering order. Harder to test.

### C: Capture-time injection into curated zshrc

During the capture command, when curating the `.zshrc`, cross-reference manifest tools and inject guarded PATH entries.

- Pros: No template changes needed.
- Cons: Mixes build concerns into user config. Duplicates PATH entries on re-capture. Curated zshrc becomes a mix of user preferences and build plumbing.

## Decision

Approach A: Tool Path Registry. It maintains clean separation between build knowledge (where tools are installed) and shell configuration (user preferences). The registry is easy to extend and test.

## Key Requirements

- A `ToolPaths` field on `ContainerfileData` holds the resolved list of paths
- `ContainerDataForTarget()` maps manifest tools to install paths via a registry table
- `05-shell-finalize.tmpl` renders a `RUN` step that prepends tool paths to both `.zshrc` and `.bashrc`
- The registry covers at minimum: Go (`/usr/local/go/bin`), Rust (`$HOME/.cargo/bin`), Node.js (already in standard PATH via distro package)
- Targets: OpenShell and container (not SSH)
- The curated `.zshrc` from capture should NOT include these tool paths (they are build plumbing, not user preferences)
- The `ENV PATH` lines in the Containerfile remain as defense-in-depth for non-interactive commands

## Open Questions

- Should the registry be a Go map constant or a configurable YAML file?
- How to handle tools with user-relative paths (e.g., `$HOME/.cargo/bin` vs `/sandbox/.cargo/bin`)? The template has access to `{{.HomeDir}}`.
- Should the shell-finalize step use `[ -d /path ] && ...` guards to avoid adding non-existent paths?
- How to handle the curated zshrc that already has tool paths (like the current `$GOPATH/bin` entry)? Should capture strip those, or let users manage them?
