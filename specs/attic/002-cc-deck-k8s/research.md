# Research: cc-deck (Kubernetes CLI)

**Date:** 2026-03-03
**Feature:** specs/002-cc-deck-k8s/spec.md

## Decision 1: K8s Client Library

**Decision:** Use `client-go` with typed clients for built-in resources and dynamic client for OpenShift Routes. Use Server-Side Apply for idempotent resource creation.

**Rationale:** Industry standard. Typed clients give compile-time safety for StatefulSets, Services, PVCs, NetworkPolicies. Dynamic client avoids importing OpenShift libraries for Route creation. SSA is the recommended approach for tools managing K8s objects.

**Alternatives considered:**
- Raw `kubectl apply` via exec: Fragile, requires kubectl binary, harder to handle errors programmatically
- controller-runtime: Over-engineered for a CLI tool (designed for operators/controllers)

## Decision 2: OpenShift Detection

**Decision:** Use `client-go/discovery` to check for `route.openshift.io/v1` API group at runtime. Create Routes on OpenShift, Ingress on vanilla K8s.

**Rationale:** Red Hat explicitly recommends checking for API resources rather than vendor detection. Portable and reliable.

## Decision 3: Zellij Web Client

**Decision:** Zellij 0.43+ has a built-in web server on port 8082. Configure with `web_server true` and `web_server_ip "0.0.0.0"` in the container. TLS is required for non-localhost. Mount a TLS Secret into the Pod.

**Rationale:** Native Zellij feature, WebSocket-based, no sidecar needed. On OpenShift, use Route with `passthrough` TLS termination.

## Decision 4: Container Image

**Decision:** Base on `debian:bookworm-slim`. Install Claude Code native binary (not npm), Zellij musl static binary, and git/ripgrep. Expected size: 400-600 MB.

**Rationale:** Claude Code native binary requires glibc (not Alpine-compatible). Debian slim is the smallest glibc base. Native Claude binary starts faster and auto-updates.

## Decision 5: Egress NetworkPolicy

**Decision:** Generate standard Kubernetes NetworkPolicy for default-deny egress with IP/CIDR allowlisting. On OpenShift, additionally generate an EgressFirewall (`k8s.ovn.org/v1`) for FQDN-based allowlisting.

**Rationale:** Standard NetworkPolicies do NOT support FQDN/DNS filtering (only IP/CIDR). OpenShift's EgressFirewall fills this gap. On vanilla K8s, FQDN filtering requires CNI-specific policies (Cilium, Calico) which we document but don't generate.

**Key constraint:** DNS egress must be scoped to kube-dns pods specifically to prevent DNS exfiltration.

## Decision 6: XDG Config

**Decision:** Use `github.com/adrg/xdg` for XDG Base Directory support. Config at `$XDG_CONFIG_HOME/cc-deck/config.yaml`.

**Rationale:** Full XDG spec coverage, cross-platform, 700+ stars, actively maintained. Go stdlib has no XDG support.

## Decision 7: CLI Framework

**Decision:** Use Cobra (`github.com/spf13/cobra`) with Viper for configuration.

**Rationale:** Used by kubectl, helm, oc, gh. K8s users expect the same flag patterns. Viper integration unifies flags, env vars, and config files. Auto-generated shell completions.
