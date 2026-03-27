# Implementation Plan: Kubernetes Deploy Environment

**Branch**: `028-k8s-deploy` | **Date**: 2026-03-27 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/028-k8s-deploy/spec.md`

## Summary

Implement `K8sDeployEnvironment`, a new backend for the existing `Environment` interface that provisions persistent developer workspaces on Kubernetes clusters. Each environment is backed by a StatefulSet with a PVC for persistent storage, accessed via `kubectl exec`. The implementation includes credential management (inline, existing Secrets, External Secrets Operator), MCP sidecar generation from build manifests, NetworkPolicy-based egress filtering, OpenShift detection for Routes and EgressFirewalls, tar-over-exec file synchronization, git harvesting, and integration tests using kind.

## Technical Context

**Language/Version**: Go 1.25 (from go.mod)
**Primary Dependencies**: cobra v1.10.2 (CLI), gopkg.in/yaml.v3 (YAML parsing), testify v1.11.1 (testing); NEW: k8s.io/client-go (K8s API), k8s.io/apimachinery (K8s types)
**Storage**: K8s PVCs (via StatefulSet volumeClaimTemplates) for workspace persistence; XDG state file (`~/.local/state/cc-deck/state.yaml`) for local tracking
**Testing**: Go testing + testify (unit); kind cluster + Go integration tests (integration); GitHub Actions CI
**Target Platform**: Linux/macOS CLI (kubectl must be available)
**Project Type**: CLI tool (adding new environment backend to existing codebase)
**Performance Goals**: Pod readiness within 5 minutes (configurable timeout); integration test suite under 5 minutes
**Constraints**: Must satisfy Environment interface behavioral contract; must work on vanilla K8s and OpenShift; must use internal/xdg (not adrg/xdg); credentials must be volume-mounted (never env vars)
**Scale/Scope**: Single-user environments; one Pod per environment

### Resolved Decisions (from research.md)

1. **Resource naming**: All resources use `cc-deck-<name>` prefix with standard `app.kubernetes.io/` labels (name, instance, managed-by, component)
2. **Kubeconfig context**: Support both `--kubeconfig` and `--context` flags; default to current kubeconfig/context
3. **ESO API version**: Use `external-secrets.io/v1` (v1beta1 removed in ESO v0.17.0+)
4. **NetworkPolicy IPs**: Point-in-time resolution at create time; no automatic refresh (matches compose proxy behavior)
5. **Git harvest**: Use git's `ext::kubectl exec -i <pod> -- %S /workspace` remote helper protocol
6. **Image pull auth**: Cluster-level concern; not cc-deck's responsibility
7. **OpenShift Route**: Generated only when OpenShift detected AND web port specified; targets headless Service
8. **Stub image**: Alpine + sleep infinity + fake zellij (matching existing Containerfile.stub)
9. **K8s API**: client-go for CRUD operations; kubectl exec (via os/exec) for interactive attach and file sync
10. **Manifest file name**: `cc-deck-image.yaml` (not `cc-deck-build.yaml`); loaded via `build.LoadManifest()`
11. **MCP sidecars**: Add optional `Image` field to `MCPEntry`; only entries with `Image` become sidecars
12. **OpenShift detection**: Discovery client checks for `route.openshift.io/v1` and `k8s.ovn.org/v1`

## Constitution Check

*GATE: Pre-Phase 0 check PASSED. Post-Phase 1 re-check below.*

| Principle | Gate | Pre-Phase 0 | Post-Phase 1 | Notes |
|-----------|------|-------------|-------------|-------|
| I. Two-Component Architecture | INFO | PASS | PASS | CLI-only feature (Go); no WASM plugin changes |
| II. Plugin Installation | N/A | PASS | PASS | No plugin changes |
| VI. Build via Makefile Only | GATE | PASS | PASS | All builds via `make install`, `make test`, `make lint` |
| VII. Interface Behavioral Contracts | GATE | PASS | PASS | Contracts defined in `contracts/`; behavioral requirements from 023-env-interface satisfied |
| VIII. Simplicity | GATE | PASS | PASS | 8 source files (k8s_*.go), no new packages, no abstractions beyond existing patterns |
| IX. Documentation Freshness | GATE | PENDING | PASS | Tasks must include: README update, CLI reference, user guide, landing page card |
| XII. Prose Plugin | GATE | PENDING | PASS | Tasks must require prose plugin for all doc content |
| XIII. XDG Paths | GATE | PASS | PASS | Uses internal/xdg for state file paths |
| XIV. No Dotfile Nesting | N/A | PASS | PASS | No new dot directories introduced |

**Gate Evaluation**: All gates PASS. No violations requiring justification.

## Project Structure

### Documentation (this feature)

```text
specs/028-k8s-deploy/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output
└── tasks.md             # Phase 2 output (/speckit.tasks command)
```

### Source Code (repository root)

```text
cc-deck/internal/
├── env/
│   ├── k8s_deploy.go          # K8sDeployEnvironment implementation
│   ├── k8s_deploy_test.go     # Unit tests
│   ├── k8s_resources.go       # K8s resource generation (StatefulSet, Service, etc.)
│   ├── k8s_resources_test.go  # Resource generation tests
│   ├── k8s_credentials.go     # Credential management (inline, existing, ESO)
│   ├── k8s_credentials_test.go
│   ├── k8s_sync.go            # Push/Pull/Harvest via kubectl exec
│   ├── k8s_sync_test.go
│   ├── k8s_openshift.go       # OpenShift detection and resource generation
│   ├── k8s_openshift_test.go
│   ├── factory.go             # Updated with k8s-deploy registration
│   └── types.go               # K8sFields already defined
├── cmd/
│   └── env.go                 # Updated with k8s-deploy CLI flags
├── network/
│   └── (existing)             # Reused for domain resolution
└── integration/
    └── k8s_deploy_test.go     # Integration tests (kind cluster)

.github/workflows/
└── integration-k8s.yaml       # CI workflow for kind-based tests
```

**Structure Decision**: Single project, extending the existing `cc-deck/internal/env/` package with new files prefixed `k8s_`. No new packages needed. Integration tests go in the existing `internal/integration/` directory pattern.

## Complexity Tracking

No violations requiring justification. The feature is a new implementation of an existing interface, following established patterns.
