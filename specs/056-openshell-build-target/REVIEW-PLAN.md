# Review Guide: OpenShell Build Target

**Spec:** [spec.md](spec.md) | **Plan:** [plan.md](plan.md) | **Tasks:** [tasks.md](tasks.md)
**Generated:** 2026-05-15

---

## What This Spec Does

cc-deck's build system currently generates container images and SSH provisioning playbooks from a single `build.yaml` manifest. This spec adds a third target, `openshell`, that produces OCI images compatible with OpenShell sandboxes. The key difference from the container target is that OpenShell images embed a `/etc/openshell/policy.yaml` restricting network access and filesystem permissions per the OpenShell sandbox model. The policy is auto-generated from the manifest's `network.allowed_domains` and can be overridden with per-binary scoping rules.

**In scope:** Manifest schema extension (`targets.openshell`), Containerfile generation for OpenShell base images, policy.yaml generation with merge semantics, CLI extensions (`--target openshell` for init/run/verify), and documentation.

**Out of scope:** Capture phase (auto-discovering tools from host), skills integration (populating the skills directories), runtime policy overrides, policy verification against a running sandbox, and pushing to registries (already works generically). The spec explicitly avoids the capture phase, which is reasonable given that manifests are assumed to already contain the needed tools and domains.

## Bigger Picture

This spec sits at the intersection of two cc-deck trajectories: the manifest-driven build system (introduced in feature 018, extended through 021-release-process) and the OpenShell workspace backend (feature 049). Currently, building an OpenShell sandbox image requires maintaining a separate static Containerfile. This feature closes that gap by making OpenShell images a first-class build target.

The policy generation aspect is interesting because it transforms a flat domain allow-list (`network.allowed_domains`) into a structured per-binary access control policy. This is a one-way bridge: the manifest schema stays simple while the generated artifact is more sophisticated. If OpenShell's policy schema evolves (v2), this generator will need updating, but the spec pins to v1 and treats the schema as stable.

The empty skills directories (`/sandbox/.agents/skills/`, `/sandbox/.claude/skills/`) created by FR-004 are a placeholder for a future feature. This is a clean boundary.

---

## Spec Review Guide (30 minutes)

> Focus your review on the policy generation design and the boundary between AI-driven and programmatic code.

### Understanding the approach (8 min)

Read [Functional Requirements](spec.md#functional-requirements) FR-001 through FR-006 for the core build flow, then [Section C description in plan.md](plan.md#source-code-repository-root) for the architecture.

- Does the split between Go code (policy structs, merge logic, CLI dispatch) and AI-driven code (Containerfile generation via `cc-deck.build.md` command spec) feel right? The AI generates the Containerfile and infers tool-to-domain associations, while Go handles policy schema serialization and merge. Is there risk of the AI-driven step producing inconsistent results?
- The spec states that tool-to-domain associations are "inferred by the AI-driven `/cc-deck.build` command during Containerfile generation." Is this inference reliable enough, or should the manifest include explicit tool-to-domain mappings?

### Key decisions that need your eyes (12 min)

**No mandatory cc-deck/Zellij layers** ([research.md R4](research.md#r4-containerfile-generation-pattern))

The container target has three mandatory layers (cc-session, cc-deck plugin install, Claude Code). The OpenShell target only installs Claude Code. The rationale is that OpenShell sandboxes run the supervisor, not Zellij.
- Is this correct? Could there be OpenShell use cases where Zellij/cc-deck inside the sandbox is desired?

**Policy merge: override by endpoint host** ([contracts/policy-schema.md, Merge section](contracts/policy-schema.md#merge-with-explicit-overrides))

When explicit policy entries overlap with auto-generated entries, matching is done by endpoint host. The explicit entry replaces the auto-generated one entirely (not a union).
- Is host-level matching the right granularity? If a user defines a policy for `github.com:443` and the auto-generated policy has `github.com:443` plus `github.com:22`, the override replaces both. Is that the intended behavior, or should matching be by host+port pair?

**Pessimistic default: no network if no domains** ([spec.md Edge Cases](spec.md#edge-cases))

When `allowed_domains` is empty and no explicit policies exist, the sandbox has zero outbound network access. This is secure-by-default but could surprise users who forget to add domains.
- Should the build command warn when generating a policy with empty `network_policies`?

**Binary path discovery is AI-driven, not programmatic** ([research.md R2](research.md#r2-binary-path-resolution-strategy))

The Go code maintains a `WellKnownBinaries` reference table, but actual binary-to-domain association happens during AI-driven Containerfile generation. This means the policy accuracy depends on the AI's inference quality.
- Is a well-known defaults table sufficient for reproducibility? Should there be a way for users to verify or override the inferred mappings?

### Areas where I'm less certain (5 min)

- [spec.md FR-006](spec.md#functional-requirements): The phrase "tool-to-domain association is inferred by the AI-driven `/cc-deck.build` command" pushes significant logic into an unverifiable AI step. The same manifest could produce different policies on different runs. Is this acceptable for a security-sensitive artifact like a sandbox policy?

- [plan.md Structure Decision](plan.md#source-code-repository-root): The plan creates `policy.go` for Go-side policy logic, but the heavy lifting (binary discovery, tool-domain inference) happens in the command spec. The boundary between what Go code does and what the AI command does could be clearer.

- [tasks.md T017](tasks.md): The `WellKnownBinaries` map is described as "not used programmatically by Go code" but serves as a reference. Is dead code the right approach, or should it be documentation only?

### Risks and open questions (5 min)

- If the OpenShell community base image changes its directory structure or user model (currently `sandbox` user, `/sandbox` workdir), the generated Containerfile will break. Is there a version pinning strategy beyond using `latest`? ([spec.md Assumptions](spec.md#assumptions))

- The `detectRunTarget()` function currently handles two-way ambiguity (container + SSH). With three targets, the error messaging needs to handle three-way ambiguity gracefully. Is "both container and SSH artifacts found; use --target to select one" still adequate when openshell is also present? ([spec.md Edge Cases](spec.md#edge-cases))

- The spec pins to OpenShell policy schema v1. What happens if a user's OpenShell gateway expects v2? Is there a version negotiation mechanism, or does the generated policy include `version: 1` unconditionally? ([contracts/policy-schema.md Invariants](contracts/policy-schema.md#invariants))

---
*Full context in linked [spec](spec.md) and [plan](plan.md).*
