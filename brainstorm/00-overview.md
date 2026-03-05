# Brainstorm Overview

Last updated: 2026-03-03

## Sessions

| # | Date | Topic | Status | Spec |
|---|------|-------|--------|------|
| 01 | 2026-03-02 | cc-deck-session-manager | spec-created | 001 |
| 02 | 2026-03-03 | cc-deck-k8s-cli | active | - |
| 03 | 2026-03-03 | repo-restructure | parked | - |
| 04 | 2026-03-03 | session-flavors | active | - |
| 05 | 2026-03-03 | clipboard-bridge | active | - |
| 06 | 2026-03-04 | plugin-lifecycle | complete | - |
| 07 | 2026-03-05 | plugin-bugfixes | complete | - |

## Open Threads

- Vertex AI Workload Identity vs service account key for GKE/OpenShift (from #02)
- Base container image definition (from #02)
- Move Rust code to cc-zellij-plugin/ subdirectory (from #03)
- Image building out of scope, future separate project (from #04)
- Composable features (devcontainer-style) deferred to future iteration (from #04)
- Zellij plugin clipboard integration is stretch goal (from #05)
- Build pipeline orchestration: Makefile/Taskfile location (from #06)
- Zellij version compatibility matrix for plugin SDK (from #06)
- Floating plugin pane API availability in zellij-tile 0.43 (from #07)
- Headless Zellij testing approach for CI (from #07)

## Parked Ideas

- Repo restructuring (#03): Move to monorepo structure after cc-deck spec is approved
  Reason: Spec creation is higher priority, restructuring is mechanical work
