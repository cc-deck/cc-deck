# Feature Specification: OpenShell Build Target

**Feature Branch**: `056-openshell-build-target`
**Created**: 2026-05-15
**Status**: Draft
**Input**: Brainstorm session 053 (OpenShell image build integration)

## Purpose

Extend cc-deck's manifest-driven build system with an `openshell` target that generates OpenShell-compatible container images. A single build manifest produces images for multiple backends (container, ssh, openshell) from the same tool and configuration definitions. The OpenShell target generates a Containerfile using the OpenShell community base image and embeds a `/etc/openshell/policy.yaml` derived from the manifest's network configuration, enriched with per-binary scoping discovered during Containerfile generation.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Build OpenShell Image from Manifest (Priority: P1)

A developer who already uses `cc-deck build` for container images wants to also produce an OpenShell sandbox image from the same manifest. They add a `targets.openshell` section to their `build.yaml`, run `/cc-deck.build --target openshell`, and get a Containerfile that uses the OpenShell community base image with their tools installed and a policy file embedded.

**Why this priority**: This is the core feature. Without it, users must maintain a separate static Containerfile for OpenShell sandboxes, duplicating tool installation steps.

**Independent Test**: Add `targets.openshell` to an existing build.yaml with tools and allowed_domains defined. Run `/cc-deck.build --target openshell`. Verify the generated Containerfile uses the OpenShell base image, installs the declared tools, and COPYs a policy.yaml to `/etc/openshell/policy.yaml`. Build the image with `cc-deck image run --target openshell` and verify the policy file exists inside the image.

**Acceptance Scenarios**:

1. **Given** a build.yaml with tools and `targets.openshell` configured, **When** I run `/cc-deck.build --target openshell`, **Then** a Containerfile is generated in `openshell/Containerfile` using the OpenShell community base image
2. **Given** a generated OpenShell Containerfile, **When** I run `cc-deck image run --target openshell`, **Then** the image builds successfully and contains `/etc/openshell/policy.yaml`
3. **Given** `network.allowed_domains` contains `api.anthropic.com` and `github.com`, **When** the OpenShell Containerfile is generated, **Then** the embedded policy.yaml contains network_policies entries for both domains

---

### User Story 2 - Policy Merge with Explicit Overrides (Priority: P2)

A developer needs fine-grained control over the OpenShell sandbox policy. They define per-binary network rules under `targets.openshell.policy` that override the auto-generated rules from `network.allowed_domains`. For example, they restrict `github.com` access to only the `git` binary, while the auto-generated rule would allow all binaries.

**Why this priority**: The auto-generated policy from allowed_domains is a reasonable default, but production sandboxes often need tighter per-binary scoping. Without overrides, users must edit the generated policy file manually after each build.

**Independent Test**: Define `targets.openshell.policy.network_policies` with a rule for `github.com` scoped to `/usr/bin/git`. Also have `github.com` in `network.allowed_domains`. Build and verify the explicit per-binary rule replaces the auto-generated all-binaries rule.

**Acceptance Scenarios**:

1. **Given** `network.allowed_domains` includes `github.com` AND `targets.openshell.policy.network_policies` defines a `github.com` entry scoped to `/usr/bin/git`, **When** the policy is generated, **Then** the explicit entry overrides the auto-generated one (override, not union)
2. **Given** `targets.openshell.policy` defines `filesystem_policy` and `process` sections, **When** the policy is generated, **Then** these sections replace the defaults in the final policy file
3. **Given** only `network.allowed_domains` is defined (no explicit policy overrides), **When** the policy is generated, **Then** all domains are allowed for all discovered binaries (permissive default)

---

### User Story 3 - Binary Path Discovery During Build (Priority: P2)

When the build system generates install instructions for tools (e.g., `dnf install git`, `npm install -g claude`), it simultaneously resolves the binary paths where those tools will be installed. These paths are used in the OpenShell policy's `network_policies` to scope network access per binary.

**Why this priority**: Per-binary scoping is what makes OpenShell policies meaningful. Without binary path discovery, the policy would allow all binaries to access all endpoints, defeating the purpose of per-binary network rules.

**Independent Test**: Define tools including `git`, `node`, and `Claude Code` in the manifest. Generate the OpenShell Containerfile. Verify the policy contains network_policies entries with correct binary paths (`/usr/bin/git`, `/usr/bin/node`, `/usr/local/bin/claude`).

**Acceptance Scenarios**:

1. **Given** a manifest with `git` in tools and `github.com` in allowed_domains, **When** the Containerfile generator installs git via dnf, **Then** it records `/usr/bin/git` as the binary path and uses it in the policy's github endpoint
2. **Given** a manifest with `Claude Code` in tools, **When** the Containerfile generator installs Claude, **Then** it uses the well-known path `/usr/local/bin/claude` in the policy
3. **Given** a tool installed via a method with no known binary path convention, **When** the policy is generated, **Then** no binary restriction is applied for that tool's endpoints (all binaries allowed for those domains)

---

### Edge Cases

- What happens when `targets.openshell` is defined but `network.allowed_domains` is empty? The policy is generated with empty `network_policies` (no outbound network access). The user must add domains or explicit policy entries.
- What happens when both `targets.container` and `targets.openshell` are defined and the user runs `cc-deck image run` without `--target`? The CLI auto-detection finds both `container/Containerfile` and `openshell/Containerfile` and errors, asking the user to specify `--target`.
- What happens when the OpenShell base image is not accessible? The podman build fails with a standard image pull error. No special handling needed.
- What happens when `targets.openshell.policy` defines a network_policy for a domain NOT in `allowed_domains`? The explicit policy entry is included as-is. The `allowed_domains` list and the policy overrides are independent; overrides are additive.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The manifest schema MUST support a `targets.openshell` section with fields for `image` (name), `tag`, `base` (base image URL), `registry`, and an optional `policy` subsection
- **FR-002**: When `--target openshell` is specified, the build system MUST generate a Containerfile in `openshell/Containerfile` that uses the configured base image (defaulting to the OpenShell community base image)
- **FR-003**: The generated Containerfile MUST install all tools from the manifest `tools[]` section, following the same installation logic as the `container` target
- **FR-004**: The generated Containerfile MUST create the empty OpenShell skills directory structure at `/sandbox/.agents/skills/` and `/sandbox/.claude/skills/` (no content, no symlinks; actual skill population is a follow-up feature)
- **FR-005**: The build system MUST generate a `policy.yaml` file and COPY it to `/etc/openshell/policy.yaml` in the image
- **FR-006**: The default policy MUST be derived from `network.allowed_domains` by creating a `network_policies` entry for each domain with port 443 and the binary paths associated with tools that use those domains (tool-to-domain association is inferred by the AI-driven `/cc-deck.build` command during Containerfile generation)
- **FR-007**: When `targets.openshell.policy` defines entries that overlap with auto-generated entries (matched by endpoint host), the explicit entries MUST override the auto-generated ones (not union)
- **FR-008**: During Containerfile generation, the build system MUST resolve binary paths from install methods: system packages (dnf/apt) install to `/usr/bin/`, global npm packages to `/usr/local/bin/`, direct downloads to `/usr/local/bin/`
- **FR-009**: The build system MUST maintain well-known binary path defaults for common tools (Claude Code, git, node, python, go) as a fallback when install-method discovery is not available
- **FR-010**: `cc-deck image run --target openshell` MUST build the image using the generated Containerfile, same flow as `--target container`
- **FR-011**: `detectRunTarget()` MUST recognize `openshell/Containerfile` as a valid target alongside `container/Containerfile` and `ssh/site.yml`
- **FR-012**: The generated Containerfile MUST set `USER sandbox`, `WORKDIR /sandbox`, and `ENTRYPOINT ["/bin/bash"]` to match OpenShell sandbox conventions
- **FR-013**: The default policy MUST include standard filesystem_policy entries: read_only for `/usr`, `/lib`, `/proc`, `/etc`, `/var/log`; read_write for `/sandbox`, `/tmp`, `/dev/null`, `/dev/urandom`, `/dev/random`, `/dev/pts`
- **FR-014**: The default policy MUST set `process.run_as_user` and `process.run_as_group` to `sandbox`
- **FR-015**: The default policy MUST include `landlock.compatibility: best_effort` to enable Landlock LSM enforcement with graceful degradation on older kernels

### Key Entities

- **OpenShellTarget**: Extension of TargetsConfig with image, tag, base, registry, and policy fields. The policy field contains the full OpenShell policy schema (filesystem_policy, landlock, process, network_policies)
- **BinaryMapping**: Association between a tool name, its install method, and the resolved binary path. Built during Containerfile generation and consumed by policy generation
- **PolicyFile**: The generated `/etc/openshell/policy.yaml` containing merged default + override policies

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A user with an existing build.yaml can add `targets.openshell` and produce a buildable OpenShell image within 5 minutes
- **SC-002**: The generated policy.yaml is valid according to the OpenShell policy schema (version 1) and passes sandbox startup validation
- **SC-003**: Binary path discovery correctly resolves paths for all tools installed via system packages, npm, and direct downloads
- **SC-004**: Override semantics work correctly: explicit policy entries replace auto-generated entries for the same endpoint host, while entries for different hosts coexist

## Error Handling

- If `targets.openshell.base` is not specified, default to `ghcr.io/nvidia/openshell-community/sandboxes/base:latest`
- If a tool in `tools[]` cannot be mapped to a binary path (unknown install method, no well-known default), generate the network_policy entry without binary restrictions (all binaries allowed for that tool's domains)
- If `network.allowed_domains` is empty and no explicit `network_policies` overrides are provided, generate a policy with empty network_policies (sandbox has no outbound network access)

## Out of Scope

- Capture phase enhancements (auto-discovering tools and endpoints from the host environment). The manifest is assumed to already contain the needed tools and domains.
- OpenShell skills integration with cc-deck's plugin system. Skills directory is created but plugin-to-skill mapping is a separate feature.
- Runtime policy overrides via `OPENSHELL_SANDBOX_POLICY` env var. The build system only handles image-time policy embedding.
- Policy verification against a running sandbox. The build system generates and embeds the policy; runtime validation is the supervisor's responsibility.
- Push to registry (`cc-deck image push --target openshell`). This already works generically for any built image.

## Clarifications

### Session 2026-05-15

- Q: How does the build system know which tool uses which domain? -> A: The AI-driven `/cc-deck.build` command infers tool-to-domain associations during Containerfile generation. No explicit mapping in the manifest schema.
- Q: Should the default policy include a landlock section? -> A: Yes, include `landlock: { compatibility: best_effort }` for graceful degradation on older kernels.
- Q: What should populate the skills directory (FR-004)? -> A: Create empty directory structure only (`/sandbox/.agents/skills/`, `/sandbox/.claude/skills/`). Actual skill content is a follow-up feature.

## Assumptions

- The OpenShell community base image at `ghcr.io/nvidia/openshell-community/sandboxes/base:latest` contains the `supervisor` and `sandbox` users, the policy file convention at `/etc/openshell/policy.yaml`, and the standard directory structure
- The policy schema version 1 is stable and will not change incompatibly during this feature's lifecycle
- Tool installation in the Containerfile follows the same patterns as the existing `container` target (the `/cc-deck.build` command generates RUN instructions for each tool)
- Tool-to-domain associations (which tool needs which endpoint) are inferred by the AI-driven `/cc-deck.build` command, not declared in the manifest. This is consistent with how the command already infers install methods for tools.
- Binary paths are deterministic for a given install method (dnf always installs to `/usr/bin/`, npm global to `/usr/local/bin/`, etc.)
- The existing build manifest version (currently 3) can accommodate the new `targets.openshell` section without a version bump, as it is an additive, optional field
