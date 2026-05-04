# Research: OpenShell Backend for cc-deck

**Date**: 2026-04-30

## R1: OpenShell gRPC Proto Availability

**Decision**: Use OpenShell's published proto files from the upstream repository for Go client code generation via `protoc-gen-go` and `protoc-gen-go-grpc`.

**Rationale**: The PRFAQ documents a stable gRPC API surface with specific RPC names (CreateSandbox, DeleteSandbox, GetSandbox, ExecSandbox, CreateSshSession, PushSandboxLogs, SetPolicy, GetPolicy). The gateway already serves these on a multiplexed port. Proto files are the source of truth for client generation.

**Alternatives considered**:
- Hand-written gRPC client without proto codegen: fragile, version-coupling risk
- REST wrapper: OpenShell doesn't expose REST, only gRPC
- CLI wrapping (`openshell sandbox create`): brittle output parsing, version coupling

**Source**: PRFAQ Appendix "OpenShell gRPC API surface"

## R2: SSH Tunnel via CreateSshSession

**Decision**: Use OpenShell's `CreateSshSession` RPC to establish SSH tunnels into sandboxes. Reuse cc-deck's existing SSH backend patterns for Zellij session management, data channel (rsync/tar), and git channel (ext:: protocol).

**Rationale**: `CreateSshSession` provides an authenticated, tunnel into the sandbox through the gateway. The gateway routes the HTTP CONNECT request to the correct supervisor. This works reliably for the Podman driver (local gateway). For K8s, HTTP CONNECT has known issues with port-forward and Routes, but that's out of scope.

**Alternatives considered**:
- Direct Podman exec: bypasses OpenShell security model, no policy enforcement on tunnel
- WebSocket tunneling: not implemented in OpenShell yet
- gRPC streaming for terminal: would require new RPCs, not in current API

**Source**: PRFAQ "How the sandbox actually works", OpenShell on OpenShift appendix

## R3: Sandbox Image Strategy

**Decision**: cc-deck provides a reference Dockerfile (`build/Dockerfile.openshell`) that builds an image with Zellij, Claude Code, git, and common development tools. Users can customize or bring their own image via workspace definition YAML.

**Rationale**: OpenShell's `CreateSandbox` accepts a container image reference. The image must be compatible with the supervisor injection (OpenShell injects the supervisor binary via volume mount for Podman, init container for K8s). A reference Dockerfile gives users a working starting point.

**Alternatives considered**:
- Platform-provided base images (Model B from PRFAQ): not built yet, proposed for Red Hat to publish
- No default image, user always provides: poor developer experience for getting started
- Bake supervisor into image: conflicts with OpenShell's injection model

**Image contents**:
- Zellij (latest stable)
- Claude Code (via npm or standalone binary)
- Git, standard Unix tools
- Node.js runtime (for Claude Code)
- Working directory at `/sandbox`

## R4: InfraState Mapping

**Decision**: Map OpenShell sandbox states to cc-deck's existing InfraStateValue enum without adding new values.

**Rationale**: cc-deck has three InfraState values: `running`, `stopped`, `error`. OpenShell has richer states (creating, running, suspended, error, deleted). The mapping:

| OpenShell State | cc-deck InfraState | Notes |
|---|---|---|
| creating | (not stored, transient) | Create blocks until running or error |
| running | `running` | Normal operating state |
| suspended | `stopped` | Sandbox paused, can resume |
| error | `error` | Sandbox failed |
| deleted | (remove from state store) | Sandbox gone, clean up local state |

**Alternatives considered**:
- Add new InfraState values (starting, suspended): changes core cc-deck model for one backend
- Store OpenShell native state alongside: adds complexity, dual-state confusion

**Source**: Clarification session 2026-04-30 Q1

## R5: Concurrent Attach Handling

**Decision**: Enforce single-attach semantics. Track SSH tunnel ownership in workspace state. Second attach attempt fails with "workspace already attached" error.

**Rationale**: Matches existing cc-deck SSH and container backend behavior. Avoids complexity of shared Zellij sessions across multiple terminals.

**Implementation**: Store `attached_pid` (or tunnel handle) in workspace instance state. On attach, check if an active tunnel exists. On detach or tunnel drop, clear the marker. Stale markers (process died) detected via PID liveness check.

**Source**: Clarification session 2026-04-30 Q2

## R6: TLS Configuration

**Decision**: TLS optional. Plaintext allowed for localhost (`127.0.0.1`, `::1`, `localhost`). Warning emitted for non-localhost connections without TLS.

**Rationale**: Local Podman development doesn't need certs. Remote gateways (future K8s integration) should use TLS but that's not the initial target.

**Implementation**: Gateway connection config in workspace definition YAML supports `tls: true/false` and `tls_cert_path`. Default behavior: auto-detect based on host.

**Source**: Clarification session 2026-04-30 Q3

## R7: Network Policy Default

**Decision**: cc-deck ships a default OPA/Rego-compatible YAML policy for OpenShell sandboxes. Users can override via workspace definition.

**Default allowed domains**:
- `api.anthropic.com` (Claude API)
- `api.openai.com` (OpenAI API)
- `registry.npmjs.org` (npm)
- `pypi.org`, `files.pythonhosted.org` (Python)
- `proxy.golang.org` (Go modules)
- `crates.io` (Rust)
- `github.com`, `gitlab.com` (Git hosting)

**Everything else blocked by default.**

**Rationale**: These are the minimum domains needed for a developer coding session. The list is conservative. Users add project-specific domains (internal APIs, artifact registries) via workspace definition.

**Source**: Brainstorm document "Network Policy Defaults" section

## R8: Proto Versioning Strategy

**Decision**: Pin proto files to a specific gateway release tag. Vendor the generated Go code into `cc-deck/internal/openshell/proto/`. Document the minimum compatible gateway version alongside the proto files.

**Rationale**: The OpenShell gRPC API is early-stage (documented in PRFAQ, not a versioned SDK). Pinning to a release tag provides reproducibility. Explicit version tracking allows controlled upgrades when the gateway API evolves.

**Alternatives considered**:
- No version tracking (vendor proto files without tag reference): risk of silent API drift
- gRPC reflection at runtime: adds complexity, loses type safety, not appropriate for a CLI tool

**Source**: Clarification session 2026-05-04

## R9: Observability Approach

**Decision**: Debug-level logging of all gRPC call outcomes (connect, create, attach, delete, exec) using cc-deck's existing log output. No new metrics infrastructure for the MVP.

**Rationale**: Sufficient for troubleshooting sandbox lifecycle issues during development. The OpenShell gateway provides its own OCSF logging for security and audit events. cc-deck's debug logs complement the gateway's logs without duplicating them.

**Alternatives considered**:
- No cc-deck logging (rely entirely on gateway OCSF): insufficient for debugging cc-deck-side issues
- Structured JSON log file: overengineering for an MVP

**Source**: Clarification session 2026-05-04

## R10: Credential Handling

**Decision**: cc-deck passes the `provider` name from the workspace definition to CreateSandbox. The OpenShell gateway's provider mechanism handles all credential injection (API keys, tokens). cc-deck never handles secrets directly.

**Rationale**: Keeps the security boundary clean. The gateway already has a credential provider abstraction. cc-deck acting as a credential intermediary would add attack surface with no benefit.

**Alternatives considered**:
- cc-deck injects `ANTHROPIC_API_KEY` from host env: mixes security boundaries, cc-deck becomes a secret handler
- Defer credential support entirely: blocks real usage scenarios where the agent needs API access

**Source**: Clarification session 2026-05-04
