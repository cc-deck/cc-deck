# Review Summary: cc-deck (Kubernetes CLI)

**Spec:** specs/002-cc-deck-k8s/spec.md | **Plan:** specs/002-cc-deck-k8s/plan.md
**Generated:** 2026-03-03

---

## Executive Summary

Developers using Claude Code often need isolated, secure environments on remote infrastructure rather than running sessions locally. cc-deck solves this by providing a single CLI command to deploy Claude Code + Zellij sessions on Kubernetes or OpenShift clusters, complete with persistent storage, network security controls, and credential management for multiple AI backends.

The tool creates a StatefulSet-based Pod for each session, ensuring stable naming and persistent storage. Developers can connect via terminal exec, a browser-based web client (new in Zellij 0.43), or port-forwarding. Credential profiles support both Anthropic's direct API and Google Vertex AI, stored in an XDG-conformant local config file. Network egress is locked down by default using Kubernetes NetworkPolicies, with OpenShift EgressFirewall for FQDN-based filtering. Bidirectional file sync lets developers push local repositories into the container and pull changes back.

The CLI is built in Go using Cobra (the same framework as kubectl and helm), with client-go for Kubernetes API access. It generates K8s resources programmatically using Server-Side Apply for idempotent operations, and auto-detects OpenShift clusters to create Routes and EgressFirewall resources automatically.

## PR Contents

| Artifact | Description |
|----------|-------------|
| `spec.md` | 15 functional requirements, 6 user stories, error handling, edge cases |
| `plan.md` | Go project structure with 5 internal packages, tech stack decisions |
| `research.md` | 7 research decisions covering K8s patterns, security, and tooling |
| `data-model.md` | Profile, Session, ConnectionInfo, Config entities with YAML schema |
| `contracts/cli-commands.md` | Full CLI command tree with flags and exit codes |
| `contracts/k8s-resources.md` | K8s resource templates and labeling conventions |
| `quickstart.md` | Development setup and first-run instructions |
| `tasks.md` | 48 tasks across 9 phases with dependency tracking |
| `review-summary.md` | This file |

## Technical Decisions

### Decision: client-go with Server-Side Apply
- **Chosen approach:** Generate K8s resources programmatically in Go, apply via SSA
- **Alternatives considered:**
  - Shell out to `kubectl apply`: Fragile, requires kubectl binary, harder error handling
  - controller-runtime: Over-engineered for a CLI (designed for operators)
- **Trade-off:** More Go code to write, but full control over resource generation and error handling

### Decision: Standard NetworkPolicy + OpenShift EgressFirewall
- **Chosen approach:** Dual-layer security. Standard NetworkPolicy (IP/CIDR) for all clusters, plus EgressFirewall (FQDN) on OpenShift
- **Alternatives considered:**
  - Standard NetworkPolicy only: Cannot filter by domain name, only IP ranges
  - Cilium/Calico-specific policies: Locks users into a specific CNI
- **Trade-off:** FQDN filtering only available on OpenShift. Vanilla K8s users get IP-based filtering with documentation on CNI-specific alternatives
- **Reviewer question:** Is IP-only filtering acceptable for vanilla K8s, or should we require a FQDN-capable CNI?

### Decision: Native Claude Code binary over npm
- **Chosen approach:** Install Claude Code native binary in the container image
- **Alternatives considered:**
  - npm install: Requires Node.js runtime, larger image, slower startup
- **Trade-off:** Native binary auto-updates and starts faster, but requires glibc (no Alpine)

## Critical References

| Reference | Why it needs attention |
|-----------|----------------------|
| `spec.md` FR-006: Vertex AI credentials | Multiple credential strategies (SA key vs Workload Identity). Must work on both GKE and OpenShift. |
| `research.md` Decision 5: Egress NetworkPolicy | Standard K8s NetworkPolicies cannot filter by FQDN. This is a fundamental limitation that affects the security promise. |
| `contracts/k8s-resources.md`: Credential mounting | Different mount strategies per backend. Getting this wrong exposes credentials. |
| `spec.md` FR-012: User overlays | Kustomize overlay support adds complexity. Needs clear documentation on what can be customized. |

## Reviewer Checklist

### Verify
- [ ] StatefulSet volumeClaimTemplate naming convention matches what delete workflow expects
- [ ] Credential mounting for Vertex AI works with both SA key file and Workload Identity
- [ ] EgressFirewall API (`k8s.ovn.org/v1`) is available on target OpenShift versions (4.12+)
- [ ] Zellij web server TLS requirement is handled (cert mounting or passthrough termination)

### Question
- [ ] Should `cc-deck deploy` create the K8s Secret for credentials if it doesn't exist, or always require pre-creation?
- [ ] Is the 10Gi default PVC size appropriate for typical Claude Code workloads?
- [ ] Should the base container image be part of this project or maintained separately?

### Watch out for
- [ ] Zellij web client requires WebSocket support in Ingress/Route. NGINX Ingress may need timeout annotations.
- [ ] EgressFirewall has a limit of 8,000 rules per namespace and DNS-based rules have race conditions with IP changes.
- [ ] `kubectl exec` TTY passthrough on Windows may behave differently than Linux/macOS.

## Scope Boundaries
- **In scope:** Deploy, connect, sync, profiles, egress security, session lifecycle on K8s/OpenShift
- **Out of scope:** Base container image build, multi-tenant sharing, GPU/model hosting, credential rotation, CI/CD, monitoring
- **Why these boundaries:** v1 focuses on the developer self-service workflow. The container image is a separate deliverable. Multi-tenant and enterprise features are future work.

## Naming & Schema Decisions

| Item | Name | Context |
|------|------|---------|
| CLI binary | cc-deck | Go binary in `cc-deck/` subdirectory |
| Config file | `~/.config/cc-deck/config.yaml` | XDG-conformant, YAML format |
| StatefulSet | `cc-deck-<name>` | Stable Pod: `cc-deck-<name>-0` |
| PVC | `data-cc-deck-<name>-0` | K8s volumeClaimTemplate convention |
| NetworkPolicy | `cc-deck-<name>-egress` | Per-session egress policy |
| Labels | `app.kubernetes.io/*` | Standard K8s label conventions |
| Credential profiles | Named in config.yaml | `anthropic-dev`, `vertex-prod`, etc. |

## Risk Areas

| Risk | Impact | Mitigation |
|------|--------|------------|
| Zellij web TLS requirement | High | Document cert mounting; provide self-signed cert generation helper |
| FQDN egress only on OpenShift | Medium | Document limitation; provide IP ranges for vanilla K8s |
| client-go version compatibility | Medium | Pin client-go to match target K8s version range |
| Container image maintenance | Medium | Separate image repo; pin versions in cc-deck config |
| Workload Identity on OpenShift | Low | Test with OpenShift Workload Identity Federation; document setup |

---
*Share this with reviewers. Full context in linked spec and plan.*
