# 25: Security Model and Credential Management

**Date**: 2026-03-16
**Status**: brainstorm
**Feature**: Defense-in-depth security for containerized AI agent sessions
**Inspired by**: paude project (Ben Browning)

## Problem

Running AI coding agents with elevated permissions (such as `--dangerously-skip-permissions` or equivalent YOLO modes) creates significant security risks. The agent has full shell access within its container and could potentially exfiltrate code, secrets, or credentials. cc-deck currently relies on container isolation and Podman secrets for credential management but lacks a formal security model, credential lifecycle management, and active monitoring. A comprehensive security model must address network exfiltration, credential exposure, workspace integrity, and auditability.

## paude's Approach

Ben Browning's paude project demonstrates a defense-in-depth security model for containerized AI agents:

**Defense-in-Depth Layers**:
- Network filtering via Squid proxy (allowlist for approved domains)
- Read-only mounts for credentials (prevents write-back or modification)
- No SSH or git credentials in container (blocks SSH-based exfiltration)
- Credential watchdog process (lifecycle management)

**Credential Watchdog**:
Monitors tmux clients, agent CPU usage, and file activity. Removes credentials after a period of inactivity (agent idle or disconnected). This prevents credentials from sitting exposed in an unattended container.

**Verified Attack Vectors** (tested and mitigated):
- HTTP exfiltration: blocked by proxy (only allowed domains pass)
- SSH git push: blocked (no SSH credentials in container)
- HTTPS git push: blocked by proxy (github.com allowed for read, push requires auth which is not present)
- GitHub CLI writes: blocked (no GH_TOKEN with write scopes)
- Container escape: mitigated by rootless Podman or Pod security policies

**Accepted Risks** (documented and deliberate):
- Workspace destruction: acceptable because git is the backup (changes can be re-cloned)
- Secrets readable in container memory: acceptable because network filtering prevents exfiltration

**Security Enforcement**:
- `--yolo` mode is only safe when network filtering is active (enforced by paude)
- Read-only gcloud mount: credentials mounted read-only, config files copied (not mounted) to prevent write-back
- OpenShift: tmpfs credential sync via `oc cp` with `.ready` marker file pattern

## Decisions

| Question | Decision | Rationale |
|----------|----------|-----------|
| Security model | Defense-in-depth (multiple independent layers) | No single layer is sufficient; compromise of one layer should not compromise everything |
| Credential storage (Podman) | Podman secrets (current approach, retain) | Already implemented, secure, follows best practices |
| Credential storage (K8s) | K8s Secrets with RBAC restrictions | Native K8s mechanism, can restrict access per namespace |
| Credential lifecycle | Optional watchdog for credential removal after inactivity | Reduces exposure window for unattended sessions |
| Network filtering dependency | YOLO mode requires network filtering | Without filtering, YOLO mode is too risky for production use |
| Audit logging | Log agent actions and blocked network attempts | Essential for forensics and compliance |
| Threat model documentation | Create formal threat model as part of cc-deck docs | Transparency about what is and is not protected |

## Threat Model

### Attack Vectors and Mitigations

| Vector | Mitigation | Layer |
|--------|-----------|-------|
| HTTP/HTTPS data exfiltration | Domain-filtered proxy (Podman) or NetworkPolicy (K8s) | Network |
| SSH-based git push | No SSH keys in container | Credential |
| GitHub CLI write operations | Token scoped to read-only, or no token | Credential |
| Container escape | Rootless Podman or K8s Pod Security Standards | Runtime |
| Credential theft from memory | Network filtering prevents exfiltration even if stolen | Network |
| Unattended credential exposure | Credential watchdog removes creds after inactivity | Lifecycle |

### Accepted Risks

- Workspace file destruction (git provides recovery)
- Secrets readable in container process memory (network layer blocks exfiltration)
- Agent could consume excessive compute resources (out of scope, handled by resource limits)

## Adaptation: Podman

- Podman secrets for credential injection (current approach, retain and document)
- Add optional credential watchdog as a sidecar or background process
- Network filtering via Squid proxy sidecar (see brainstorm 22)
- Read-only mounts for credential files where possible
- Rootless Podman provides container escape mitigation by default
- `cc-deck deploy --compose` generates security-hardened compose.yaml:
  - No privileged containers
  - Read-only root filesystem where feasible
  - Dropped capabilities
  - Network isolation via internal network

## Adaptation: Kubernetes

- K8s Secrets for credential storage (current approach)
- RBAC policies: service account per session with minimal permissions
- NetworkPolicy for egress control (see brainstorm 22)
- Pod Security Standards (restricted profile) for runtime hardening
- Optional credential rotation via CronJob or sidecar container
- Resource limits (CPU, memory) enforced via LimitRange or resource quotas
- Audit logging via K8s audit policy for session namespace events
- `cc-deck deploy --k8s` generates:
  - ServiceAccount with minimal RBAC
  - NetworkPolicy with egress allowlist
  - Pod Security context (non-root, read-only rootfs, dropped caps)

## Adaptation: OpenShift

- tmpfs credential sync pattern (from paude): `oc cp` credentials into tmpfs mount, `.ready` marker file signals availability
- Security Context Constraints (SCCs) for pod security (more granular than K8s PSS)
- EgressFirewall CRD for network control (see brainstorm 22)
- Credential watchdog as a sidecar container that monitors agent activity
- OpenShift's built-in audit logging for namespace events
- Integration with OpenShift's OAuth proxy for web-based access
- `cc-deck deploy --openshift` generates SCC-aware manifests

## Credential Watchdog Design

```
Watchdog Process (sidecar or background)
  |
  +-- Monitor: agent process CPU usage
  +-- Monitor: file system activity in workspace
  +-- Monitor: terminal client connections
  |
  +-- On inactivity timeout (configurable, default 30min):
  |     1. Remove credential files from tmpfs
  |     2. Unset credential env vars (signal agent process)
  |     3. Log credential removal event
  |
  +-- On reconnection:
        1. Re-inject credentials (requires user authentication)
        2. Log credential restoration event
```

## Security Documentation

- Formal threat model document in cc-deck docs (Antora module: reference)
- Per-deployment-target security checklist
- Hardening guide for production deployments
- Incident response playbook for detected exfiltration attempts

## Open Questions

- Should the credential watchdog be on by default or opt-in?
- How to handle credential re-injection after watchdog removal (user must re-authenticate, or cached securely)?
- Should cc-deck enforce network filtering when YOLO mode is detected, or just warn?
- How to audit agent actions within the container (shell history, file modifications)?
- Integration with external secret management (HashiCorp Vault, AWS Secrets Manager, OpenShift Vault integration)?
- Should cc-deck provide a "security score" for a deployment configuration?
