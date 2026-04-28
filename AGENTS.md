# Agent Instructions

This project uses **bd** (beads) for issue tracking. Run `bd onboard` to get started.

## Quick Reference

```bash
bd ready              # Find available work
bd show <id>          # View issue details
bd update <id> --status in_progress  # Claim work
bd close <id>         # Complete work
bd sync               # Sync with git
```

## Landing the Plane (Session Completion)

**When ending a work session**, you MUST complete ALL steps below. Work is NOT complete until `git push` succeeds.

**MANDATORY WORKFLOW:**

1. **File issues for remaining work** - Create issues for anything that needs follow-up
2. **Run quality gates** (if code changed) - Tests, linters, builds
3. **Update issue status** - Close finished work, update in-progress items
4. **PUSH TO REMOTE** - This is MANDATORY:
   ```bash
   git pull --rebase
   bd sync
   git push
   git status  # MUST show "up to date with origin"
   ```
5. **Clean up** - Clear stashes, prune remote branches
6. **Verify** - All changes committed AND pushed
7. **Hand off** - Provide context for next session

**CRITICAL RULES:**
- Work is NOT complete until `git push` succeeds
- NEVER stop before pushing - that leaves work stranded locally
- NEVER say "ready to push when you are" - YOU must push
- If push fails, resolve and retry until it succeeds


## Active Technologies
- Go 1.25 (from go.mod) + cobra v1.10.2 (CLI), adrg/xdg v0.5.3 (XDG paths), gopkg.in/yaml.v3 (YAML), client-go v0.35.2 (K8s API) (041-workspace-channels)
- N/A (channels are stateless transport abstractions) (041-workspace-channels)
- Go 1.25 (from go.mod) + cobra v1.10.2 (CLI), gopkg.in/yaml.v3 (YAML), adrg/xdg replacement via internal/xdg (XDG paths) (043-workspace-lifecycle)
- YAML state file at `~/.local/state/cc-deck/state.yaml` (043-workspace-lifecycle)
- Go 1.25 (CLI), Rust stable edition 2021 wasm32-wasip1 (plugin) + cobra (CLI), charmbracelet/bubbletea + lipgloss + bubbles (TUI), gen2brain/malgo (audio, CGo), zellij-tile 0.43.1 (plugin SDK), serde/serde_json (plugin serialization) (042-voice-relay)
- `~/.cache/cc-deck/models/` (whisper models, XDG cache), WASI `/cache/` (plugin state) (042-voice-relay)

## Recent Changes
- 041-workspace-channels: Added Go 1.25 (from go.mod) + cobra v1.10.2 (CLI), adrg/xdg v0.5.3 (XDG paths), gopkg.in/yaml.v3 (YAML), client-go v0.35.2 (K8s API)
