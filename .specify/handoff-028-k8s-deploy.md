# Handoff: 028-k8s-deploy

**Branch**: `028-k8s-deploy`
**Worktree**: `/Users/rhuss/Development/ai/cc-deck/wt/cc-deck-k8s-deploy`
**Created**: 2026-03-27
**Phase**: Spec complete, ready for planning

## Summary

K8sDeployEnvironment: a new `Environment` interface implementation for persistent Kubernetes workloads using StatefulSets. This is the first of two K8s specs (029-k8s-sandbox will follow).

## Key Decisions from Brainstorm

- **Clean implementation** (not refactoring existing code)
- **StatefulSet** (replicas=1) for predictable Pod naming (`cc-deck-<name>-0`)
- **Credentials**: K8s Secrets volume-mounted at `/run/secrets/cc-deck/` (never env vars). Three modes: inline (`--credential`), existing (`--existing-secret`), ESO (`--secret-store`)
- **External Secrets Operator**: Generate `ExternalSecret` CRs for vault integration. Vanilla K8s first, OpenShift opt-in. Not coupled to any specific backend.
- **Network filtering**: Reuse `internal/network/` domain resolution. Same UX as Podman/compose (`--allow-domain`, `--allow-group`, `--no-network-policy`)
- **MCP sidecars**: Generated from build manifest as Pod sidecar containers sharing localhost
- **OpenShift**: Auto-detect via API discovery (Route, EgressFirewall)
- **Testing**: kind for local/CI, manual verification on full OpenShift
- **Dapr**: Evaluated and skipped (wrong abstraction layer, excessive overhead)

## Spec Structure

- 8 user stories (2x P1, 5x P2, 1x P3)
- 20 functional requirements
- 8 success criteria
- Explicit contract reference to `specs/023-env-interface/contracts/environment-interface.md`

## Dependencies

- 023-env-interface (completed): Environment interface, state store, factory
- 022-network-filtering (97%): Domain resolution in `internal/network/`
- 018-build-manifest (85%): Build manifest format for MCP entries
- 017-base-image (completed): Container image

## Existing Code to Build On

- `cc-deck/internal/env/types.go`: K8sFields, SandboxFields already defined
- `cc-deck/internal/env/factory.go`: Needs K8sDeploy case added
- `cc-deck/internal/env/interface.go`: Environment interface contract
- `cc-deck/internal/env/container.go`: Reference implementation (~570 lines)
- `cc-deck/internal/env/compose.go`: Reference implementation (~705 lines)
- `cc-deck/internal/env/auth.go`: Credential modes (reuse patterns)
- `cc-deck/go.mod`: client-go v0.35.2 already present

## Next Step

Run `/speckit.plan` or `/sdd:brainstorm` for planning in this worktree.
