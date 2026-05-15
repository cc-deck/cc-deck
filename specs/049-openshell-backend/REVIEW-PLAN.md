# Review Guide: OpenShell Backend for cc-deck

**Spec:** [spec.md](spec.md) | **Plan:** [plan.md](plan.md) | **Tasks:** [tasks.md](tasks.md)
**Generated:** 2026-05-04

---

## What This Spec Does

This feature adds an `openshell` workspace type to cc-deck, allowing developers to run Claude Code sessions inside OpenShell sandboxes with policy-enforced network and filesystem isolation. cc-deck communicates with the OpenShell gateway over gRPC to provision sandboxes, and uses SSH tunnels for session attach, file sync, and git harvest. The initial target is the Podman compute driver for local development.

**In scope:** gRPC client for the OpenShell gateway, sandbox provisioning, SSH tunnel attach, file sync (push/pull/harvest), exec, delete, state reconciliation, default network policy, reference Dockerfile, debug-level observability logging, credential delegation to gateway provider.

**Out of scope:** Kubernetes/OpenShift driver support (interface should not preclude it), Zellij sidebar plugin inside the sandbox (requires host-side pipes), multi-user concurrent attach (single-attach only), WebSocket tunneling, cc-deck-side credential/secret handling.

## Bigger Picture

cc-deck already supports local, container (Podman), compose, SSH, and Kubernetes workspace types. OpenShell adds a sixth type that uniquely provides policy-enforced sandboxing, where the sandbox infrastructure is managed by an external gateway rather than directly by cc-deck. This is the first backend where cc-deck delegates infrastructure management to a separate service.

The spec was originally written in a standalone openshell project and ported to cc-deck as feature 049. The OpenShell project itself is early-stage. The gRPC API surface is documented in a PRFAQ, not a versioned SDK. Proto files are pinned to a specific gateway release tag ([R8](research.md#r8-proto-versioning-strategy)) to manage this risk.

---

## Spec Review Guide (30 minutes)

> Focus your review on the parts that need human judgment most. Each section points to specific locations and frames the review as questions.

### Understanding the approach (8 min)

Read [User Story 1](spec.md#user-story-1---create-a-sandboxed-workspace-priority-p1) and [FR-002 through FR-004](spec.md#functional-requirements) for the core approach. As you read, consider:

- Does delegating all sandbox lifecycle to the OpenShell gateway (rather than calling Podman directly) add too much indirection for the local development use case?
- The spec pins proto files to a gateway release tag ([Assumptions](spec.md#assumptions), [R8](research.md#r8-proto-versioning-strategy)). Is tag-based pinning sufficient, or should the proto version be checked at connect time?
- The plan puts all workspace logic in two files (`openshell.go` and `channel_openshell.go`). The existing SSH backend uses a similar pattern (`ssh.go` at ~400 lines). Is that the right granularity, or should channel implementations be separate files?

### Key decisions that need your eyes (12 min)

**Single-attach semantics** ([FR-013](spec.md#functional-requirements), [Research R5](research.md#r5-concurrent-attach-handling))

The spec enforces one SSH tunnel per workspace, matching existing backends. The attach-detection mechanism uses PID liveness checks (`os.FindProcess` + signal 0). This works on Unix but is not atomic. If a tunnel process dies and a new unrelated process reuses the PID, the check gives a false positive.

- Is PID-based tunnel liveness detection reliable enough, or should the implementation also verify the SSH connection is functional (e.g., a keepalive probe)?

**TLS optional with warning** ([FR-010](spec.md#functional-requirements), [Research R6](research.md#r6-tls-configuration))

Plaintext gRPC is allowed for localhost, with a warning for non-localhost. This is pragmatic for local dev but sets a precedent.

- Are you comfortable with plaintext gRPC to any `localhost` address, including `127.0.0.1` and `::1`? Could a local process intercept this traffic?

**InfraState mapping** ([FR-009](spec.md#functional-requirements), [Research R4](research.md#r4-infrastate-mapping), [Data Model](data-model.md#infrastate-mapping))

OpenShell has five states; cc-deck has three. The "creating" state is treated as transient (Create blocks until running). The "deleted" state removes the workspace from the store entirely rather than mapping to an error.

- Is silently removing a workspace from the store when GetSandbox returns "deleted" the right behavior? Should the user be notified that external deletion occurred?

**Default network policy** ([FR-011](spec.md#functional-requirements), [Research R7](research.md#r7-network-policy-default))

The default policy allows nine domains (Anthropic, OpenAI, npm, PyPI, Go proxy, crates.io, GitHub, GitLab). Everything else is blocked.

- Is `api.openai.com` appropriate in the default? Some organizations may not want this allowed by default.
- Missing from the default: container registries (e.g., `ghcr.io`, `quay.io`), Homebrew (`brew.sh`), Ubuntu/Debian package mirrors. Are these needed for a typical coding session?

**Credential delegation** ([R10](research.md#r10-credential-handling), [Key Entities: Provider](spec.md#key-entities))

cc-deck passes only the provider name to the gateway. The gateway injects credentials. cc-deck never touches secrets.

- Is the provider name sufficient, or should cc-deck also pass provider-specific configuration (e.g., which API key to inject)?

### Areas where I'm less certain (5 min)

- [Phase 5, T018-T019](tasks.md#phase-5-user-story-3---sync-files-into-and-out-of-the-sandbox-priority-p2): These two tasks both write to the same file (`channel_openshell.go`). The plan notes this conflict but leaves resolution to the implementer. If parallel agents are used, this will cause merge conflicts.

- [Edge Cases](spec.md#edge-cases): Image pull progress and disk space monitoring now have user-facing error messages defined, but no dedicated tasks implement them. The error messages are expected to surface naturally through the gRPC error wrapping in T027. Is that sufficient, or should explicit handling tasks exist?

- [SC-001 through SC-003](spec.md#measurable-outcomes): Performance thresholds are defined (30s create, 5s attach, 30s sync) but no tasks create benchmarks or performance tests. These are manual verification criteria for now.

### Risks and open questions (5 min)

- [FR-007](spec.md#functional-requirements) specifies git harvest using `git ext::` protocol. This is an uncommon git transport. Does the existing cc-deck SSH git channel already use this pattern, or is this new ground?

- The [Assumptions](spec.md#assumptions) state that `CreateSshSession` "has known issues with K8s ingress." If a future spec adds K8s driver support, does this limitation affect the interface design chosen now?

- [T014](tasks.md#phase-3-user-story-1---create-a-sandboxed-workspace-priority-p1-mvp) creates a Dockerfile based on `debian:bookworm-slim`. OpenShell's supervisor injection assumes a specific entrypoint model. Is there a risk of incompatibility between the Dockerfile entrypoint and the supervisor injection mechanism?

---
*Full context in linked [spec](spec.md) and [plan](plan.md).*
