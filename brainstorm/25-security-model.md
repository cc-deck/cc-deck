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

## Updated Learnings from paude (May 2026)

Source: `bbrowning/paude` v0.15.0, specifically `src/paude/agents/base.py`.

### Explicit Secret vs Passthrough Env Var Separation

paude makes a clear distinction between two categories of environment variables in its `AgentConfig`:

- **`passthrough_env_vars`**: Regular env vars forwarded from the host to the container. These appear in the container spec and are visible in `podman inspect` or K8s Pod manifests. Examples: `GOOGLE_CLOUD_LOCATION`, `CLOUD_ML_REGION`.

- **`secret_env_vars`**: Sensitive credentials that must be delivered securely. These are explicitly excluded from `build_environment_from_config()` and handled separately by `build_secret_environment_from_config()`. The separation ensures secrets are never accidentally included in container specs, image layers, or debug output.

The `build_environment_from_config()` function filters secrets by maintaining a `secret_set` and skipping any var in that set during passthrough iteration:

```python
secret_set = set(config.secret_env_vars)
for var in config.passthrough_env_vars:
    if var in secret_set:
        continue
    # ... add to env
```

cc-deck should adopt this pattern. Currently, all credential env vars are treated uniformly. A formal distinction would prevent accidental exposure in compose files, `podman inspect` output, or K8s Pod specs. The workspace definition could gain explicit fields:

```yaml
credentials:
  passthrough:    # visible in container spec, OK for non-sensitive config
    - GOOGLE_CLOUD_LOCATION
    - CLOUD_ML_REGION
  secrets:        # delivered via podman secret / K8s Secret, never in spec
    - ANTHROPIC_API_KEY
    - OPENAI_API_KEY
```

### Equivalent Variable Syncing

paude maintains a list of equivalent env var pairs (e.g., `GOOGLE_CLOUD_LOCATION` and `CLOUD_ML_REGION`) and ensures both are set when either is present. This prevents subtle configuration failures where a tool expects one name but the user set the other.

cc-deck could implement this for known equivalences in the agent adapter layer, or document it as a convention.

### Trust/Onboarding Suppression via Config Injection

paude's `apply_sandbox_config()` generates shell scripts that pre-configure agents to skip interactive prompts inside containers:

- **Claude Code**: Sets `hasCompletedOnboarding: true` and `hasTrustDialogAccepted: true` for the workspace path in `~/.claude.json`. Uses `jq` for safe JSON manipulation.
- **Gemini CLI**: Writes the workspace path to `~/.gemini/trustedFolders.json` with `"TRUST_FOLDER"` status.

This is a security-relevant pattern: it ensures agents start in a known state without human interaction, which is essential for YOLO mode and fire-and-forget workflows. cc-deck currently handles Claude Code trust via hook configuration, but extending this to multi-agent scenarios would need the same approach.

### DNS Leak Prevention via Proxy dnsmasq

paude's proxy sidecar runs `dnsmasq` bound to `127.0.0.1:53` before starting the actual proxy binary. This exists because some tools (notably Rust's `reqwest` HTTP client) bypass the system DNS resolver and query DNS directly. Without a local DNS server, these tools could fail or leak DNS queries outside the filtered network.

The dnsmasq config:
1. Parses upstream servers from `/etc/resolv.conf`
2. Accepts an optional `PROXY_DNS` env var for custom upstream
3. Falls back to public DNS (8.8.8.8, 1.1.1.1)

cc-deck's proxy sidecar design (brainstorm 22) should include dnsmasq or equivalent DNS forwarding to prevent DNS-level leaks that bypass the HTTP proxy.

## Open Questions

- Should the credential watchdog be on by default or opt-in?
- How to handle credential re-injection after watchdog removal (user must re-authenticate, or cached securely)?
- Should cc-deck enforce network filtering when YOLO mode is detected, or just warn?
- How to audit agent actions within the container (shell history, file modifications)?
- Integration with external secret management (HashiCorp Vault, AWS Secrets Manager, OpenShift Vault integration)?
- Should cc-deck provide a "security score" for a deployment configuration?
- Should the secret vs passthrough distinction be enforced at the Go type level (separate fields in the workspace struct) or at the config level (separate YAML keys)?
- Should equivalent env var syncing be hardcoded per agent or configurable in the agent adapter config?
