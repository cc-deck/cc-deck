# Research: Build Skill Iteration Reduction

## Current State Analysis

### Build Skill (`cc-deck.build.md`, 834 lines)

**Section A2 (Container Build)** (lines 68-183):
- Snippet handling: "Copy content EXACTLY as-is" (line 72)
- Tool resolution: `install: github-release` uses `asset_pattern` from manifest (line 87)
- Plugin handling: `source: marketplace -> claude plugins install <name>` (line 103)
- Settings merge: "merge user preferences... Never overwrite cc-deck hooks" (line 182) but no jq command provided
- No post_install protocol documented

**Section C2 (OpenShell Build)** (lines 613-671):
- Same snippet rule: "Copy content EXACTLY as-is" (line 617)
- Base image probing present but no shell config scanning (lines 619-629)
- Assembly order: 11 steps, no USER root insertion mentioned (lines 642-654)
- Settings handling: "Same rules as Section A" (line 658)

**Key Rules** (lines 823-834):
- OpenShell note mentions `sandbox` user and `/sandbox` workdir but not Ubuntu/Fedora distinction
- No mention of tool availability differences between base images

### Capture Skill (`cc-deck.capture.md`, 1032 lines)

**Step 5c (Shell Config Curation)** (around line 100+):
- Extracts aliases, functions, env vars, keybindings
- "Strip out" rules remove macOS-specific items and plugin managers
- "Guard unresolved commands" wraps with availability checks
- No compinit preamble rule when stripping plugin managers
- No shell config dependency scanning for implicit tool requirements

**Step 11 (Target Configuration)**:
- No asset pattern verification against GitHub API
- No post_install dry-run validation

### Templates

**`03-mandatory-stack.tmpl`** (30 lines):
- `chown -R {{.User}}:{{.User}} {{.HomeDir}}/.config/zellij {{.HomeDir}}/.cache/zellij {{.HomeDir}}/.claude` (line 21)
- Cache parent directory NOT included in chown
- No marketplace setup before plugin installs
- Claude Code via native installer only (line 27)

**`05-shell-finalize.tmpl`** (32 lines):
- Starship init unconditional: `echo 'eval "$(starship init '"$SHELL_NAME"')"' >> "$RC"` (line 23)
- No TERM=dumb guard

## Decisions

### D1: Where to insert USER root in C2

**Decision**: Between step 1 (01-header.txt) and step 3 (system packages layer) in the C2 assembly order.
**Rationale**: The header snippet sets FROM and ARG. All subsequent RUN layers need root. The USER root line is generated (not in a snippet) so it stays visible.

### D2: Asset verification protocol

**Decision**: Two-phase verification with GitHub API query + tarball structure probe.
**Rationale**: The API query catches naming mismatches (gnu vs musl, .tar.gz vs .tar.xz). The tarball probe catches nested directory structures. Both run before any Containerfile line is written.
**Implementation**: Add a new subsection under "Tool resolution" in both A2 and C2 with the exact verification steps.

### D3: Snippet modification scope

**Decision**: Allow modification only for download/extraction commands, with a comment documenting the change.
**Rationale**: The "copy verbatim" rule exists for good reason (snippets are pre-rendered). The escape hatch is narrow: only download URLs and tar extraction commands can be fixed.

### D4: Template changes vs skill instruction changes

**Decision**: Changes to `03-mandatory-stack.tmpl` and `05-shell-finalize.tmpl` are direct template edits. All other changes are skill markdown instruction edits.
**Rationale**: Templates are Go-processed at `build refresh` time and produce snippet files. Editing templates ensures all future snippet regenerations include the fixes. Skill instructions guide the LLM during Containerfile generation.

### D5: Shell config dependency scanning location

**Decision**: Split between capture (Step 5c) and build (C2).
- Capture: scan shell config for tool references, flag them in manifest
- Build: cross-reference manifest flags with base image probe, install missing tools
**Rationale**: Capture detects what the shell config needs. Build resolves what the base image lacks. This mirrors the dual-phase asset verification pattern.

### D6: fzf installation strategy

**Decision**: Install from GitHub releases when `source <(fzf --zsh)` is detected.
**Rationale**: Ubuntu 24.04 ships fzf 0.44, which lacks the `--zsh` flag (added in 0.48). GitHub releases provide a single static binary. This matches the existing pattern for starship, lsd, and other tools.

## Alternatives Considered

### Alt: Go code for asset verification
Rejected in brainstorm. Skill instructions are sufficient because the LLM already has access to `curl` and `tar` during Containerfile generation. Adding Go code would create a parallel verification path that needs to stay in sync with the skill.

### Alt: Remove fzf --zsh from curated config
Rejected. The `--zsh` flag provides better integration (key bindings + completion) than the older file-based init. Installing a recent fzf is the correct fix.

### Alt: Always use npm for Claude Code
Rejected in brainstorm. Native installer is primary, npm is fallback on OOM (exit 137). The native installer bundles its own Node.js, which is more reliable when the base image's Node.js version is unknown.
