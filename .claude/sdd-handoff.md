# Context Handoff: 024-container-env

## Feature
Container Environment - single `podman run` lifecycle with definition/state separation

## Branch
`024-container-env` (worktree at `../cc-deck-024-container-env`)

## Current State
- **Implementation complete** and manually tested
- All 18 FRs covered, 130 tests passing
- Manual walkthrough at `docs/walkthroughs/024-container-env.md`
- Constitution updated to v1.10.0 (Principles VII, XII, XIII added)

## What Was Built

### Core (from spec)
- `internal/podman/` package (shared podman CLI interaction layer, 6 files)
- `internal/env/container.go` (ContainerEnvironment implementing Environment interface)
- `internal/env/definition.go` (DefinitionStore for environments.yaml)
- State schema v2 (EnvironmentInstance, slim runtime records)
- Type renames: PodmanFields → ContainerFields, EnvironmentTypePodman → EnvironmentTypeContainer

### Beyond Spec (from manual testing and review)
- `internal/xdg/` package: Replaced `adrg/xdg` with internal implementation using `~/.config` and `~/.local` on all platforms (Constitution Principle XIII)
- `--auth` flag: Auto-detection of Claude Code auth mode (API, Vertex, Bedrock) with credential injection
- `--mount` flag: Arbitrary bind mounts for container environments
- File-based credential detection: GOOGLE_APPLICATION_CREDENTIALS auto-detected from default ADC path, mounted as podman secret file
- Proper Attach: Uses `zellij -n cc-deck` for layout creation, `zellij attach` for reconnection
- Orphaned resource cleanup: Create/delete handle stale containers and volumes gracefully
- VolumeCreate idempotency: Ignores "already exists" errors
- Interface behavioral contracts: Added to spec 023 environment interface

## Remaining Work
- [ ] Squash/clean up commits before merging to main (20+ commits from iterative development)
- [ ] Run full `make test` (requires WASM binary for embed.go)
- [ ] Verify walkthrough end-to-end on a clean environment
- [ ] Consider: sidebar refresh on reattach (brainstorm 027)
- [ ] Consider: Claude Code onboarding automation in container images

## Key Files
- `cc-deck/internal/podman/*.go` - Podman interaction layer
- `cc-deck/internal/env/container.go` - ContainerEnvironment (main implementation)
- `cc-deck/internal/env/definition.go` - DefinitionStore
- `cc-deck/internal/xdg/xdg.go` - XDG paths (Linux convention on all platforms)
- `cc-deck/internal/cmd/env.go` - CLI commands with new flags
- `cc-deck/internal/config/config.go` - ContainerDefaults
- `docs/walkthroughs/024-container-env.md` - Manual test walkthrough
- `specs/024-container-env/` - Spec, plan, tasks, contracts, REVIEWERS.md
- `.specify/memory/constitution.md` - v1.10.0 (3 new principles)
- `brainstorm/027-sidebar-state-refresh.md` - Sidebar refresh brainstorm

## Key Decisions Made During Implementation
- **XDG paths**: Use `~/.config` and `~/.local` on macOS (not `~/Library/Application Support/`). Replaced `adrg/xdg` with `internal/xdg` package.
- **Auth auto-detection**: `--auth auto` (default) detects Vertex > Bedrock > API from host env vars. Vertex also auto-detects ADC file from `~/.config/gcloud/application_default_credentials.json`.
- **No auto-mount**: Credential directories are NOT auto-mounted (hardcodes user path). File-based credentials use podman secret file mounts at `/run/secrets/` (user-independent).
- **Attach layout**: Uses `zellij -n cc-deck` (new-session-with-layout) for first attach. `zellij attach` for subsequent. `--layout` on `attach --create-background` does not work in any Zellij version.
- **Mounts are container-only**: The `mounts` field in environments.yaml is rejected by K8s types. K8s uses K8s Secrets for credentials.
- **Constitution v1.10.0**: Added Principle VII (Interface Behavioral Contracts), renumbered VIII-XIII, added XIII (XDG Paths).
