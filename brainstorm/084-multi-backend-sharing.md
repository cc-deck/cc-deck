# Brainstorm: Multi-Backend Session Sharing

**Date:** 2026-07-22
**Status:** parked

## Problem Framing

CC Deck sessions don't only run locally. They can run via SSH connections, inside OpenShell sandboxes, or on Kubernetes-deployed environments. Each of these has a fundamentally different networking model:

- **Local (Zellij on laptop):** Zellij web server on localhost, needs outbound tunnel to reach collaborators
- **SSH backend:** Zellij runs on a remote host, web server binds there, may already be reachable or need its own tunnel
- **OpenShell backend:** Session runs in a gRPC-managed sandbox container, networking is controlled by the sandbox provider
- **Kubernetes backend:** Session runs in a pod, networking via K8s Services/Ingress, potentially already has public exposure through cluster ingress

The `cc-deck share` command needs to work across all these backends, but "start a tunnel from localhost" is only correct for the local case.

## Spike Findings (from 082)

### Validated: Local + Tunnel Pattern

The spike confirmed that the local pattern works:
1. Zellij web server on `127.0.0.1:8082`
2. Cloudflare Tunnel or Traefik proxy exposes it externally
3. Token auth works through the tunnel

### Validated: Traefik Direct Proxy

The home cluster can proxy directly to the laptop's Zellij server via headless Service + Endpoints. This pattern extends to K8s: if Zellij runs in a pod, a K8s Service + Ingress exposes it the same way.

### Key Constraint: Zellij Web Server Requires User Space

The Zellij web server is a user-space process, not a container sidecar. In K8s and OpenShell, the web server must run inside the same environment as the Zellij session. This is straightforward for K8s pods (just expose the port) but may be limited in OpenShell sandboxes (port exposure depends on the sandbox provider's networking policy).

## Approaches Considered

### A: Local-only sharing, defer remote backends

- Pros: Simplest, covers the primary use case
- Cons: Users on remote backends can't share, limiting the feature's reach

### B: Backend-aware sharing

- Pros: Each CC Deck backend knows how to expose its session appropriately
- Cons: Significant complexity, each backend needs its own sharing strategy

### C: Proxy-through-host model

- Pros: All sharing goes through the user's machine regardless of where the session runs (SSH port forward, K8s port-forward, then tunnel from localhost)
- Cons: Extra latency hop, but architecturally simple and uniform

## Decision

Parked: Start with local sharing (approach A). The proxy-through-host model (approach C) is the natural next step for SSH backends since `cc-deck` already manages SSH connections and could add a reverse port forward. K8s and OpenShell need more investigation.

**Preliminary direction:** Start with local + SSH (both can use localhost tunneling via port forwards). OpenShell and K8s backends can expose sessions through their native networking (K8s Ingress, OpenShell gateway) in later iterations.

## Key Requirements

- Each CC Deck backend (`local`, `ssh`, `openshell`, `kubernetes`) must implement a `ShareableSession` interface
- For SSH: automatically set up a reverse port forward from the remote Zellij web server to localhost, then use the standard tunnel
- For K8s: create a temporary Ingress resource pointing to the pod's Zellij web server port
- For OpenShell: explore whether the gRPC gateway can proxy WebSocket traffic

## Open Questions

- Does Zellij's web server work inside an OpenShell sandbox container? (WASI limitations?)
- For K8s pods, is a sidecar container for the tunnel more appropriate than a K8s Ingress?
- How does authentication work when the Zellij token database is inside a container that may be ephemeral?
- Should sharing be opt-in per backend (some environments may have security policies against it)?
- For SSH: does the existing SSH connection management in `cc-deck` support adding a reverse port forward mid-session?
