# Reviewers Guide: 028-k8s-deploy

## Feature Summary

Implements `K8sDeployEnvironment`, a new Environment interface backend that provisions persistent developer workspaces on Kubernetes clusters using StatefulSets with PVCs. Includes credential management (inline, existing Secrets, ESO), MCP sidecar generation, NetworkPolicy egress filtering, OpenShift compatibility, file sync, git harvesting, and kind-based integration tests.

## Review Focus Areas

### 1. Environment Interface Contract Compliance
**Priority**: Critical
**Files**: `specs/028-k8s-deploy/contracts/`, `specs/028-k8s-deploy/data-model.md`
**Check**: Verify all behavioral requirements from `specs/attic/023-env-interface/contracts/environment-interface.md` are addressed:
- Create: name validation, tool check, conflict detection, state recording, cleanup on failure
- Attach: nested Zellij detection, auto-start, timestamp update, session with layout
- Delete: running check, best-effort cleanup, state removal
- Status: reconciliation against K8s API

### 2. Credential Security Model
**Priority**: Critical
**Files**: `specs/028-k8s-deploy/contracts/credential-management.md`
**Check**: Credentials are NEVER exposed as environment variables (FR-005). Volume mount at `/run/secrets/cc-deck/` only. Existing Secrets are preserved on delete. ESO uses `external-secrets.io/v1`.

### 3. Resource Generation Correctness
**Priority**: High
**Files**: `specs/028-k8s-deploy/contracts/k8s-resource-generation.md`, `specs/028-k8s-deploy/data-model.md`
**Check**: StatefulSet uses `volumeClaimTemplates` (not pre-created PVC). Headless Service has `clusterIP: None`. Standard labels applied. Pod naming follows `cc-deck-<name>-0` convention.

### 4. Network Filtering UX Consistency
**Priority**: Medium
**Files**: `specs/028-k8s-deploy/research.md` (section 4)
**Check**: `--allow-domain`, `--allow-group`, `--no-network-policy` flags produce identical UX across Podman, compose, and k8s-deploy environments. Domain resolution uses the same `network.Resolver` as compose.

### 5. OpenShift Detection
**Priority**: Medium
**Files**: `specs/028-k8s-deploy/research.md` (section 12)
**Check**: Uses API discovery (not user flags). Route API: `route.openshift.io/v1`. EgressFirewall API: `k8s.ovn.org/v1`. Resources generated only when APIs are available.

### 6. Task Dependency Graph
**Priority**: Low
**Files**: `specs/028-k8s-deploy/tasks.md` (Dependencies section)
**Check**: US1 is the MVP foundation. US2-US8 branch from US1 with correct dependency ordering. No circular dependencies.

## Artifacts Checklist

| Artifact | Path | Status |
|----------|------|--------|
| Spec | `specs/028-k8s-deploy/spec.md` | Complete |
| Plan | `specs/028-k8s-deploy/plan.md` | Complete |
| Research | `specs/028-k8s-deploy/research.md` | Complete (13 decisions) |
| Data Model | `specs/028-k8s-deploy/data-model.md` | Complete |
| Contracts | `specs/028-k8s-deploy/contracts/` | 2 contracts |
| Quickstart | `specs/028-k8s-deploy/quickstart.md` | Complete |
| Tasks | `specs/028-k8s-deploy/tasks.md` | 57 tasks, 11 phases |
| REVIEWERS | `specs/028-k8s-deploy/REVIEWERS.md` | This file |

## Key Decisions to Validate

1. **client-go for API, kubectl exec for interactive** (research.md #9): Is this the right split?
2. **Point-in-time IP resolution for NetworkPolicies** (research.md #4): Acceptable limitation?
3. **MCP Image field addition** (research.md #11): Backward-compatible schema change?
4. **ESO v1 only** (research.md #3): Any users still on v1beta1?
5. **ext::kubectl exec for git harvest** (research.md #5): Any security concerns with this transport?
