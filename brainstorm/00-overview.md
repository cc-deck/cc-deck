# Brainstorm Overview

Last updated: 2026-04-30

## Active Brainstorms

| # | Date | Topic | Status | Spec |
|---|------|-------|--------|------|
| 022 | 2026-03-19 | Multi-agent support | brainstorm | - |
| 023 | 2026-03-12 | Git workflow | brainstorm | - |
| 025 | 2026-03-12 | Security model | brainstorm | - |
| 040 | 2026-04-21 | Workspace channels | specified | 041 |
| 042 | 2026-04-23 | Voice relay | brainstorm | - |
| 043 | 2026-04-23 | Clipboard bridge | brainstorm | - |
| 044 | 2026-04-24 | Sidebar session isolation | brainstorm | - |
| 045 | 2026-04-29 | Voice sidebar integration | active | - |
| 046 | 2026-04-30 | Voice attend stop word | active | - |

## Open Threads

- Multi-agent support: Agent adapter protocol, unified hook interface for Claude/Codex/Gemini, agent wrapper for hookless agents, credential transport design (from #022)
- Voice relay: speech-to-text relay via PipeChannel, plugin-side handler, local capture strategy (from #042, depends on spec 041)
- Voice sidebar integration: ♫ indicator, mute toggle, [[command]] protocol, PTT removal, bidirectional state sync (from #045)
- Voice attend stop word: whether additional voice actions beyond "submit" and "attend" will be needed (from #046)
- Clipboard bridge: image paste in remote workspaces via DataChannel + xclip shim (from #043, depends on spec 041)
- Git workflow: git push/harvest patterns for remote workspaces (from #023)
- Security model: credential proxy, git-push restriction, sandbox hardening (from #025)

## Attic

Completed brainstorms that have corresponding specs (active or attic) are in `brainstorm/attic/`. See `ls brainstorm/attic/` for the full list.
