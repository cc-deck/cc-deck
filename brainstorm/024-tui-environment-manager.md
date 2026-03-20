# Brainstorm: TUI Environment Manager

**Date:** 2026-03-19
**Status:** Brainstorm (deferred from 023-execution-environments)
**Depends on:** 023-execution-environments (interface + state.yaml)

## Context

The execution environments brainstorm (023) originally included a TUI for managing all environments. This was split out as a separate future feature to keep 023 focused on the interface, CLI, and environment implementations.

## Concept

A terminal-based UI for managing cc-deck environments, built with [bubbletea](https://github.com/charmbracelet/bubbletea) (Go, fits the existing CLI stack). Launched via `cc-deck tui` (subcommand, not a separate binary).

## Core Features

### Environment List View

The main screen shows all tracked environments with live status:

```
╔═══════════════════════════════════════════════════════════════════╗
║  cc-deck environments                              ↑↓ navigate  ║
╠═══════════════════════════════════════════════════════════════════╣
║                                                                   ║
║  > my-project      podman    ● running   volume    2h ago        ║
║    backend-work    k8s       ● running   pvc/10Gi  30m ago       ║
║    eval-run-42     sandbox   ● running   emptyDir  never         ║
║    old-project     podman    ○ stopped   volume    3d ago        ║
║                                                                   ║
╠═══════════════════════════════════════════════════════════════════╣
║  Enter: attach  s: status  d: delete  n: new  q: quit           ║
╚═══════════════════════════════════════════════════════════════════╝
```

### Environment Detail View

Pressing `s` on a selected environment shows detailed status (via exec):

```
╔═══════════════════════════════════════════════════════════════════╗
║  backend-work (k8s)                                  Esc: back  ║
╠═══════════════════════════════════════════════════════════════════╣
║                                                                   ║
║  Status:    Running (5d 3h)                                      ║
║  Storage:   PVC (10Gi, gp3)                                      ║
║  Sync:      git-harvest (push: 2h ago, harvest: 30m ago)         ║
║  Namespace: cc-deck                                              ║
║  Profile:   anthropic-prod                                       ║
║                                                                   ║
║  Agent Sessions:                                                  ║
║    api-refactor      ⚠ Permission  feat/api-v2     2m ago        ║
║    docs-update       ● Working     docs/quickstart 1m ago        ║
║    bugfix-123        ✓ Done        fix/null-ptr    15m ago       ║
║                                                                   ║
╠═══════════════════════════════════════════════════════════════════╣
║  Enter: attach  h: harvest  p: push  r: refresh                 ║
╚═══════════════════════════════════════════════════════════════════╝
```

### Key Interactions

| Key | List View | Detail View |
|---|---|---|
| Enter | Attach to environment | Attach to environment |
| s | Show detail view | - |
| Esc | Quit | Back to list |
| n | Create new environment | - |
| d | Delete environment (with confirm) | - |
| h | - | Harvest (git) |
| p | - | Push files |
| r | Refresh status | Refresh status |
| q | Quit | Quit |

## Data Model

The TUI reads from the same `state.yaml` that the CLI uses. It polls actual status from runtimes (podman/kubectl) on a configurable interval (default: 30 seconds). The detail view triggers exec calls to read session state from inside the environment.

## Implementation Considerations

- **bubbletea** for the TUI framework (Go, well-maintained, active ecosystem)
- **lipgloss** for styling (same ecosystem)
- Reuse all `Environment` interface methods from 023
- `cc-deck tui` subcommand (not a separate binary)
- Consider read-only mode vs interactive mode (create/delete from TUI could be risky)
- Auto-refresh with configurable interval
- Respect terminal size, handle resize events

## Relationship to CLI

The TUI is a convenience layer over the same operations available via `cc-deck env` commands. It does not introduce new functionality, only presents existing information in an interactive format. Users who prefer the CLI can continue using `cc-deck env list` and `cc-deck env status`.

## Open Questions

1. Should the TUI support creating new environments (interactive wizard), or only list/attach/manage existing ones?
2. Should the detail view auto-refresh agent session states, or only on manual refresh (to avoid exec latency)?
3. Should the TUI show a notification when an agent session changes state (e.g., transitions from Working to Permission)?
4. Is bubbletea the right choice, or should we consider a simpler approach (e.g., `watch`-based refresh of `cc-deck env list`)?
