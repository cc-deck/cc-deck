# Research: OpenShell Build Target

## R1: OpenShell Policy Schema (v1)

**Decision**: Use the existing `default-policy.yaml` at `cc-deck/internal/openshell/default-policy.yaml` as the reference for the policy schema. The policy file at `/etc/openshell/policy.yaml` uses YAML with four top-level sections: `filesystem_policy`, `landlock`, `process`, and `network_policies`.

**Rationale**: The existing default policy already captures the correct schema structure, including the `version: 1` field, `binaries` arrays with `path` fields, and `endpoints` with `host`/`port` pairs. The spec's FR-013 through FR-015 align exactly with this existing policy.

**Alternatives considered**:
- Inventing a new policy format: Rejected, the existing format is already established in the codebase and used by `openshell policy set`.
- Using JSON instead of YAML: Rejected, OpenShell expects YAML at `/etc/openshell/policy.yaml`.

## R2: Binary Path Resolution Strategy

**Decision**: Maintain a static lookup table mapping install methods to binary path patterns, with well-known overrides for common tools.

**Rationale**: Binary install paths are deterministic per install method:
- `install: package` (dnf) installs to `/usr/bin/<binary>`
- `install: github-release` with `install_path` uses that path directly
- npm global packages go to `/usr/local/bin/<name>`
- Well-known tools: Claude Code at `/usr/local/bin/claude`, node at `/usr/bin/node`

The AI-driven `/cc-deck.build` command already knows where tools install because it writes the Containerfile RUN instructions. During policy generation, the same knowledge applies.

**Alternatives considered**:
- Runtime discovery inside the built image: Rejected, adds complexity and requires running the image just to query paths.
- Manifest-level binary path declarations: Rejected, adds schema burden on users for information the build system already knows.

## R3: Policy Merge Semantics

**Decision**: Override by endpoint host. When `targets.openshell.policy.network_policies` defines an entry whose endpoint list includes a host that also appears in the auto-generated entries (from `network.allowed_domains`), the explicit entry replaces the auto-generated one entirely.

**Rationale**: The spec (FR-007) requires "explicit entries MUST override the auto-generated ones (not union)." Matching by endpoint host is the natural key because `allowed_domains` is a list of hosts. Per the edge case in the spec, explicit policy entries for domains NOT in `allowed_domains` are additive.

**Alternatives considered**:
- Deep merge (union of binaries): Rejected by spec. Overriding gives users full control.
- Match by policy name: Rejected, policy names are generated and may not match user-defined names.

## R4: Containerfile Generation Pattern

**Decision**: Follow the existing container target pattern (Section A in `cc-deck.build.md`). The OpenShell Containerfile differs in:
1. Base image: `ghcr.io/nvidia/openshell-community/sandboxes/base:latest` (Ubuntu-based, not Fedora)
2. User: `sandbox` (instead of `dev`)
3. Workdir: `/sandbox` (instead of `/home/dev`)
4. Skills directories: Create `/sandbox/.agents/skills/` and `/sandbox/.claude/skills/`
5. Policy embedding: COPY `openshell/policy.yaml` to `/etc/openshell/policy.yaml`
6. Build context at `openshell/context/` (instead of `container/context/`)
7. Entrypoint: `/bin/bash` (sandbox default)
8. Package manager detected via base image probing (apt-get for Ubuntu, not hardcoded dnf)

The mandatory cc-deck/Zellij stack IS included, adapted for `sandbox` user paths. This ensures the same TUI and tooling experience across all target types.

**Rationale**: Users expect the same experience regardless of target type. The OpenShell base image provides the sandbox runtime, supervisor, and policy enforcement. The Containerfile installs the full cc-deck stack (Zellij, cc-deck plugin, cc-session, cc-setup, Claude Code) on top, plus tools and the embedded policy.

**Alternatives considered**:
- Skipping the cc-deck/Zellij stack: Rejected after testing. Users need the same TUI and tools in OpenShell sandboxes as in container workspaces.
- Reusing the container target Containerfile directly: Rejected, the OpenShell base image has a different user model (`sandbox` user, `/sandbox` workdir) and is Ubuntu-based (not Fedora).

## R5: Build Init Scaffolding

**Decision**: Extend `InitSetupDir()` to recognize `--target openshell` and create `openshell/` directory. The `uncommentTargets()` function needs a new branch for openshell sections in the manifest template.

**Rationale**: Follows the existing pattern where `--target container` creates `container/context/` and `--target ssh` scaffolds Ansible roles. The openshell target just needs a directory for the generated Containerfile and policy.

**Alternatives considered**:
- Separate init command: Rejected, adding a target flag value is simpler and consistent.

## R6: detectRunTarget Extension

**Decision**: Add `openshell/Containerfile` detection to `detectRunTarget()`. When multiple targets have artifacts, the user must specify `--target`. Valid values become `container`, `ssh`, or `openshell`.

**Rationale**: The existing detection checks for `container/Containerfile` and `ssh/site.yml`. Adding `openshell/Containerfile` follows the same pattern. The `runContainerBuild()` function can be reused for openshell since both targets produce OCI images.

**Alternatives considered**:
- Separate `runOpenShellBuild()` function: Not needed, the image build is identical to container builds. Only the image ref resolution differs (reading from `targets.openshell` instead of `targets.container`).

## R7: Claude Code Installation in OpenShell Images

**Decision**: The OpenShell Containerfile MUST include Claude Code installation. Unlike the container target where `cc-deck config plugin install` sets up Zellij and cc-deck hooks, the OpenShell target only needs Claude Code itself (the native installer). Claude Code is the primary tool that runs inside OpenShell sandboxes.

**Rationale**: OpenShell sandboxes are designed to run AI coding agents. Claude Code is the core tool. The OpenShell base image does not include it.

**Alternatives considered**:
- Skip Claude Code, let the supervisor install it: Rejected, pre-installing Claude Code in the image reduces sandbox startup time.
