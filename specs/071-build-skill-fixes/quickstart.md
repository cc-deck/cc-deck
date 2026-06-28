# Quickstart: Build Skill Iteration Reduction

## What

13 targeted edits to 4 files that eliminate build iteration loops in OpenShell builds.

## Files to Edit

| File | Changes | Lines affected |
|------|---------|---------------|
| `cc-deck/internal/build/commands/cc-deck.build.md` | Changes 1-11 (A2, C2, Key Rules) | ~150 lines added/modified |
| `cc-deck/internal/build/commands/cc-deck.capture.md` | Changes 7, 12-16 (Steps 5, 11) | ~80 lines added/modified |
| `cc-deck/internal/build/templates/containerfile/05-shell-finalize.tmpl` | Change 11 (TERM guard) | 1 line modified |
| `cc-deck/internal/build/templates/containerfile/03-mandatory-stack.tmpl` | Changes 5, 6, 19 (cache, marketplace) | 3 lines modified |

## Change Map

| # | Change | File | Section |
|---|--------|------|---------|
| 1 | USER root after header | build.md | C2 assembly order |
| 2 | Asset verification | build.md | A2 + C2 tool resolution |
| 3 | Snippet escape hatch | build.md | C2 snippet note |
| 4 | Claude Code npm fallback | build.md | C2 mandatory stack docs |
| 5 | Cache dir ownership | build.md + tmpl | C2 + 03-mandatory-stack.tmpl |
| 6 | Marketplace setup | build.md + tmpl | A2 + C2 plugin handling + 03-mandatory-stack.tmpl |
| 7 | Shell config scanning | build.md + capture.md | C2 base image probing + Step 5c |
| 8 | Base image documentation | build.md | Key Rules |
| 9 | jq merge command | build.md | A2 + C2 settings handling |
| 10 | post_install protocol | build.md | A2 + C2 GitHub release tools |
| 11 | Starship TERM guard | build.md + tmpl | C2 + 05-shell-finalize.tmpl |
| 12 | fzf from GitHub | capture.md | Step 5c + Step 11 |
| 13 | compinit preamble | capture.md | Step 5c |
| 14 | Capture asset verification | capture.md | Step 11 |
| 15 | post_install dry-run | capture.md | Step 11 |
| 16 | build refresh verify | build.md | (new subsection or note) |

## Validation

```bash
# Full validation cycle
/cc-deck.capture --all
/cc-deck.build --target openshell
# Expected: first-try success, 0 self-correction iterations
```
