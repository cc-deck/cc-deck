# Brainstorm: OpenShell Image Build Integration

**Date:** 2026-05-15
**Status:** active

## Problem Framing

cc-deck's build system (`cc-deck build`) generates container images from a declarative manifest (`build.yaml`). Today it supports two targets: `container` (Podman/K8s) and `ssh` (Ansible provisioning). OpenShell sandboxes have specific image requirements that differ from standard container images: a mandatory `sandbox` user, a policy file at `/etc/openshell/policy.yaml`, supervisor compatibility, and a skills directory convention.

The challenge is integrating OpenShell image builds into the existing manifest-driven system without breaking the other targets, while ensuring that the OpenShell sandbox policy (which controls network access, filesystem restrictions, and process identity) is generated correctly from the manifest and baked into the image.

## Key Findings

### OpenShell Sandbox Image Requirements

From the official OpenShell community base image (`ghcr.io/nvidia/openshell-community/sandboxes/base`):

| Requirement | Detail |
|-------------|--------|
| Users | `supervisor` (nologin, system) + `sandbox` (bash, home=/sandbox) |
| Final USER | `sandbox` |
| WORKDIR | `/sandbox` |
| Policy file | `/etc/openshell/policy.yaml` (read by supervisor at startup) |
| Skills | `/sandbox/.agents/skills/` with symlinks into `/sandbox/.claude/skills/` |
| Python | Managed by uv, venv at `/sandbox/.venv` |
| Network tools | iproute2, nftables for namespace and bypass detection |
| Entrypoint | `/bin/bash` (supervisor wraps this) |

### Policy File Convention

The supervisor inside the sandbox reads `/etc/openshell/policy.yaml` at boot. This file is the source of truth for sandbox security. It cannot be embedded as image metadata or labels. It must be a file inside the image at that exact path.

The policy schema (version 1) has four sections:

| Section | Type | Purpose |
|---------|------|---------|
| `filesystem_policy` | Static (locked at creation) | read_only/read_write path lists |
| `landlock` | Static | LSM enforcement mode |
| `process` | Static | run_as_user/run_as_group |
| `network_policies` | Dynamic (hot-reloadable) | Per-binary endpoint rules with L7 inspection |

### Current Build Manifest Structure

The existing `build.yaml` manifest has:
- `tools[]`: Free-form tool descriptions
- `sources[]`: Repository provenance
- `plugins[]`: Claude Code plugins
- `mcp[]`: MCP server sidecars
- `settings`: Shell, Zellij, Claude configuration
- `network.allowed_domains[]`: Simplified domain allowlist
- `targets.container`: Image name, tag, base image, registry
- `targets.ssh`: Host, port, identity file

### Three Container-Based Workspace Types

| Type | Image base | User model | Policy mechanism |
|------|-----------|------------|-----------------|
| `container` (Podman) | `cc-deck-base` | Root or configurable | `allowed_domains` (iptables) |
| `k8s-deploy` | `cc-deck-base` | Configurable | K8s NetworkPolicy |
| `openshell` | OpenShell community base | Must be `sandbox:sandbox` | `/etc/openshell/policy.yaml` |

The tooling installed (Zellij, Claude Code, language runtimes) is largely the same across all three. The image structure and security model differ.

## Approaches Considered

### A: One Manifest, Multiple Build Targets (Recommended)

Extend the existing `targets` section with an `openshell` target alongside `container` and `ssh`. During the capture/extraction phase, tools and configuration go into shared sections. During build generation, the Containerfile template differs per target.

```yaml
version: 3
tools:
  - Go 1.25
  - Python 3.13
  - Zellij

network:
  allowed_domains:
    - api.anthropic.com
    - github.com
    - registry.npmjs.org

targets:
  container:
    image: quay.io/cc-deck/my-workspace
    base: quay.io/cc-deck/cc-deck-base:latest
  openshell:
    image: quay.io/cc-deck/my-workspace-openshell
    base: ghcr.io/nvidia/openshell-community/sandboxes/base:latest
    policy:
      filesystem_policy:
        include_workdir: true
        read_only: [/usr, /lib, /proc, /etc, /var/log]
        read_write: [/sandbox, /tmp, /dev/null, /dev/urandom, /dev/random, /dev/pts]
      landlock:
        compatibility: best_effort
      process:
        run_as_user: sandbox
        run_as_group: sandbox
      network_policies:
        vertex_ai:
          name: vertex-ai
          endpoints:
            - host: aiplatform.googleapis.com
              port: 443
            - host: oauth2.googleapis.com
              port: 443
          binaries:
            - path: /usr/local/bin/claude
            - path: /usr/bin/node
        github:
          name: github
          endpoints:
            - host: github.com
              port: 443
          binaries:
            - path: /usr/bin/git
```

- Pros: Single source of truth. Capture phase fills both `allowed_domains` (simplified, cross-target) and `openshell.policy` (full, target-specific). One project can build for multiple backends. Follows existing target pattern.
- Cons: Manifest grows larger. Policy section is OpenShell-specific knowledge in a general tool.

### B: Separate OpenShell Manifest

Create a parallel `openshell-build.yaml` with OpenShell-specific schema. The standard `build.yaml` stays unchanged.

- Pros: Clean separation. No pollution of the general manifest.
- Cons: Duplication of tool/plugin/settings sections. Two manifests to keep in sync. Capture phase needs to write to two files.

### C: Policy-Only Extension File

Keep the manifest as-is. Add a separate `openshell-policy.yaml` in the build directory that the OpenShell Containerfile template COPYs into the image. The manifest's `allowed_domains` is not connected to it.

- Pros: Simplest implementation. No manifest schema changes.
- Cons: Policy and manifest are disconnected. Capture phase can't auto-generate the policy from discovered tools. Users must manually maintain both.

## Decision

**Approach A: One manifest, multiple build targets.** The `targets.openshell` section contains the full OpenShell policy schema nested under `policy`. The build system:

1. During **capture**: Discovers tools, endpoints, and binaries. Populates `network.allowed_domains` (simplified) and `targets.openshell.policy.network_policies` (full, with per-binary rules and L7 scoping).
2. During **build**: Generates a target-specific Containerfile. For `openshell`, uses the OpenShell community base image, creates the `sandbox` user convention, and writes the policy YAML to `/etc/openshell/policy.yaml`.
3. During **verify**: For `openshell`, checks that the policy file exists at the expected path, the `sandbox` user exists, and core tools are available.
4. During **diff**: Compares manifest policy against the image's embedded policy for drift detection.

### Policy-to-AllowedDomains Derivation

The `network.allowed_domains` list can be derived from `targets.openshell.policy.network_policies` by extracting all unique endpoint hosts. This means capture only needs to populate the full policy, and the simplified list is a projection. Alternatively, capture populates `allowed_domains` first (simpler discovery), and the full policy is enriched from it during build generation.

### Containerfile Template Differences

| Aspect | container target | openshell target |
|--------|-----------------|-----------------|
| Base image | `cc-deck-base` | OpenShell community base |
| User | Configurable | Always `sandbox:sandbox` |
| Policy file | Not generated | `/etc/openshell/policy.yaml` from manifest |
| Skills dir | Not present | `/sandbox/.agents/skills/` convention |
| Network filtering | iptables rules from `allowed_domains` | Policy-driven (proxy-based, per-binary) |
| Entrypoint | `sleep infinity` or configurable | `/bin/bash` (supervisor wraps) |

## Open Threads

- How should the capture phase discover per-binary network rules? Today it discovers tools and domains, but OpenShell policies bind endpoints to specific binary paths. Should capture infer binary paths from tool names (e.g., "Claude Code" maps to `/usr/local/bin/claude`)?
- Should `cc-deck build run --target openshell` also push to a registry, or only build locally? OpenShell's `--from <dir>` can build and import directly.
- How does the skills directory convention interact with cc-deck's plugin system? Should plugins be installed as OpenShell skills?
- Should `cc-deck build verify --target openshell` actually create a sandbox and test it, or just inspect the image?
- The `/dev/urandom` and `/dev/pts` entries in the filesystem policy are Zellij-specific. Should the build system add these automatically when Zellij is in the tools list?
- How to handle the `OPENSHELL_SANDBOX_POLICY` env var override at runtime vs the image-embedded policy? Document the precedence: `--policy` flag > env var > image-embedded.
