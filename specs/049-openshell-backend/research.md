# Research: OpenShell Backend for cc-deck

**Date**: 2026-04-30

## R1: OpenShell Gateway Communication

**Decision**: Wrap the `openshell` CLI binary for all gateway communication. The CLI provides correct syntax, handles mTLS transparently, and adapts to gateway version changes automatically.

**Rationale**: OpenShell is alpha software (v0.0.36, daily releases). The proto API changes frequently. The CLI team maintains backwards-compatibility for their UX, making it a more stable interface than raw proto definitions. The CLI also handles mTLS certificate management, gateway discovery, and credential injection transparently. Since cc-deck still needs the CLI for gateway management (`openshell gateway start/stop`), SSH proxy (ProxyCommand), and git ext:: transport, adding gRPC only for CRUD operations would mean maintaining two communication paths.

**Alternatives considered**:
- Direct gRPC via proto codegen: type-safe but adds protoc toolchain, mTLS cert handling, proto version pinning, and ~500 lines of new code for alpha software that can't fully eliminate the CLI dependency
- Hybrid (gRPC for lifecycle, CLI for transport): same complexity as pure gRPC with no additional benefit

**Source**: Brainstorm `brainstorm/049-openshell-grpc-vs-cli.md`

## R2: Interactive Attach via exec

**Decision**: Use `openshell sandbox exec --tty` for interactive attach instead of SSH tunnels. This simplifies the implementation by eliminating SSH tunnel management, ProxyCommand construction, and host key handling entirely.

**Rationale**: The openshell CLI's `exec --tty` flag provides proper TTY allocation for interactive sessions. Zellij runs independently inside the sandbox, so it survives exec disconnections. Reattaching is as simple as running exec again. For file transfer, `openshell sandbox upload/download` replaces the tar+base64+exec approach that was fragile for large files.

**Alternatives considered**:
- SSH tunnel via CreateSshSession RPC: requires HTTP CONNECT proxy setup, cert management, ProxyCommand construction. More complex for the same result.
- `openshell sandbox connect`: simple but doesn't allow customizing the command (need `zellij attach --create`)
- Direct SSH via ssh-config: requires parsing ssh-config output and managing ProxyCommand. The exec approach is simpler.

**Source**: Brainstorm `brainstorm/049-openshell-grpc-vs-cli.md`, OpenShell CLI docs

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

## R8: CLI Version Compatibility

**Decision**: Rely on the user-installed `openshell` CLI binary. The CLI version should match the running gateway. No proto files or version pinning is needed since the CLI handles API compatibility internally.

**Rationale**: With CLI wrapping, version compatibility is the CLI's responsibility. The CLI team maintains backwards-compatible output formats. If the CLI and gateway versions diverge, the CLI itself will surface errors. This is simpler than managing proto file versions.

**Source**: Decision to use CLI wrapping (brainstorm/049-openshell-grpc-vs-cli.md)

## R9: Observability Approach

**Decision**: Debug-level logging of all CLI invocation outcomes (create, attach, delete, exec, upload, download) using cc-deck's existing log output. No new metrics infrastructure for the MVP.

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
