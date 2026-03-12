# Brainstorm Overview

Last updated: 2026-03-12

## Sessions

| # | Date | Topic | Status | Spec |
|---|------|-------|--------|------|
| 01 | 2026-03-02 | cc-deck-session-manager | superseded-by-08 | 001 |
| 02 | 2026-03-03 | cc-deck-k8s-cli | active | 002 |
| 03 | 2026-03-03 | repo-restructure | parked | - |
| 04 | 2026-03-03 | session-flavors | active | - |
| 05 | 2026-03-03 | clipboard-bridge | active | - |
| 06 | 2026-03-04 | plugin-lifecycle | superseded-by-08 | 009 |
| 07 | 2026-03-05 | plugin-bugfixes | superseded-by-08 | 010 |
| 08 | 2026-03-07 | cc-deck-v2-redesign | active | 012 |
| 09 | 2026-03-08 | cross-session-visibility | active | - |
| 10 | 2026-03-09 | keyboard-navigation | active | 013 |
| 11 | 2026-03-10 | pause-and-help | active | 014 |
| 15 | 2026-03-10 | session-save-restore | active | 015 |
| 16 | 2026-03-11 | k8s-integration-tests | active | - |
| 17 | 2026-03-12 | base-image | active | - |
| 18 | 2026-03-12 | build-manifest | active | - |
| 19 | 2026-03-12 | build-commands | active | - |
| 20 | 2026-03-12 | deploy-integration | active | - |

## Open Threads

- Vertex AI Workload Identity vs service account key for GKE/OpenShift (from #02)
- Base container image definition (from #02)
- Move Rust code to cc-zellij-plugin/ subdirectory (from #03)
- Image building: addressed in #17-#20 (base image, build manifest, build commands, deploy)
- Composable features (devcontainer-style) deferred to future iteration (from #04)
- Zellij plugin clipboard integration is stretch goal (from #05)
- Build pipeline orchestration: Makefile/Taskfile location (from #06)
- Zellij version compatibility matrix for plugin SDK (from #06)
- Floating plugin pane API availability in zellij-tile 0.43 (from #07)
- Headless Zellij testing approach for CI (from #07)

## Parked Ideas

- Repo restructuring (#03): Move to monorepo structure after cc-deck spec is approved
  Reason: Spec creation is higher priority, restructuring is mechanical work
