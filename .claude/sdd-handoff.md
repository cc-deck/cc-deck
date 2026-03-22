# SDD Handoff: 025-compose-env

## Feature
Compose Environment with Multi-Container Orchestration

## Status
- [x] Brainstorm (`brainstorm/029-compose-environment.md`)
- [x] Specification (`specs/025-compose-env/spec.md`)
- [x] Spec Review (passed with revisions applied)
- [x] Clarify (no critical ambiguities found)
- [x] Plan (`specs/025-compose-env/plan.md`)
- [x] Tasks (`specs/025-compose-env/tasks.md`)
- [x] Plan Review (PASS with minor fixes applied, REVIEWERS.md generated)
- [x] Implementation (33/33 tasks complete, all tests passing)

## Key Context

**What**: A new `compose` environment type that uses `podman-compose` for multi-container orchestration, with optional network filtering via a tinyproxy proxy sidecar.

**Key Design Decisions** (from brainstorm):
- **Project-local**: Generated files in `.cc-deck/` (gitignored) within the project directory
- **Bind mount default**: Project directory mounted at `/workspace`, immediate bidirectional sync
- **Optional filtering**: `--allowed-domains` adds a proxy sidecar; without it, compose is still useful for future MCP sidecars
- **Shared helpers**: Extract auth detection and Zellij session check from `container.go` into shared helpers; compose uses `internal/podman` directly for secrets/volumes
- **Compose CLI lifecycle**: `podman-compose up/down/start/stop` for container management
- **Delete removes `.cc-deck/`**: Clean deletion of all generated artifacts
- **Gitignore**: Warn + `--gitignore` flag to auto-add `.cc-deck/` to `.gitignore`

**Existing Code to Reuse**:
- `internal/compose/generate.go`: Compose YAML generator (session + proxy services)
- `internal/compose/proxy.go`: Tinyproxy config generator
- `internal/network/domains.go`: Domain group resolver
- `internal/podman/*.go`: Secrets, volumes, exec, inspect
- `internal/env/container.go`: Auth detection, Zellij session check (extract as shared)

**Interface Contract**: Implements `Environment` from `specs/023-env-interface/contracts/environment-interface.md`. All behavioral requirements apply without deviation.

## Related Brainstorms
- `brainstorm/028-project-config.md`: Project-local `cc-deck.yaml` config (deferred, layers on top)
- `brainstorm/029-compose-environment.md`: Full design decisions document

## Key Files
- `cc-deck/internal/env/container.go` - Reference implementation to match behavior
- `cc-deck/internal/compose/generate.go` - Existing compose YAML generator
- `cc-deck/internal/compose/proxy.go` - Tinyproxy config generator
- `cc-deck/internal/network/domains.go` - Domain group resolver
- `cc-deck/internal/env/types.go` - Needs `EnvironmentTypeCompose` + `ComposeFields`
- `cc-deck/internal/env/factory.go` - Needs compose case
- `cc-deck/internal/env/definition.go` - Needs `AllowedDomains`, `ProjectDir` fields

## Next Step
Implementation complete. Run `/sdd:review-code` for spec compliance check, then commit and create PR.

## SDD State
sdd-initialized: true
