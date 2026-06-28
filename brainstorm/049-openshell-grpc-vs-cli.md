# Brainstorm: OpenShell Gateway Communication - gRPC vs CLI Wrapping

**Date:** 2026-05-06
**Status:** active
**Revisited:** 2026-06-26
**Context:** The OpenShell backend (049) needs to communicate with the OpenShell gateway. Two approaches are viable: direct gRPC client or wrapping the `openshell` CLI binary. The current implementation shells out to the CLI but uses incorrect syntax. Either way, the code needs rework.

## Problem Framing

cc-deck's OpenShell backend needs to:
1. Manage sandbox lifecycle (create, get, delete)
2. Execute commands inside sandboxes (streaming output)
3. Establish SSH sessions for interactive attach
4. Transfer files via data/git channels

The OpenShell gateway exposes a gRPC API (proto files at `github.com/NVIDIA/OpenShell/proto/`). The `openshell` CLI wraps this same gRPC API and adds convenience features (gateway bootstrap, credential auto-discovery, SSH config generation).

## Approach A: Direct gRPC Client

Generate Go client code from OpenShell proto files. cc-deck talks to the gateway directly over gRPC.

**What this involves:**
- Download and vendor 3 proto files (openshell.proto, sandbox.proto, datamodel.proto)
- Add protoc toolchain to build system (Makefile target)
- Add google.golang.org/grpc as a direct dependency
- Rewrite client.go with typed gRPC calls
- Handle TLS/mTLS configuration (the gateway uses mTLS by default)
- Handle streaming for ExecSandbox (server-streaming RPC)
- Extract a Client interface for testability

**Still needs the CLI for:**
- Gateway management (`openshell gateway start/stop`)
- SSH ProxyCommand (the SSH tunnel goes through the gateway via HTTP CONNECT)
- Git ext:: transport (git needs a command-line tool for stdin/stdout piping)

### Strengths

- **Type safety**: Proto-generated types eliminate string parsing entirely. No more `parseSandboxState()` guessing from CLI output.
- **Streaming support**: `ExecSandbox` returns a proper server stream. stdout/stderr chunks arrive typed and separated, with a clean exit code event. The CLI approach captures flat stdout and loses stderr/exit code separation.
- **No CLI version coupling**: The proto files are pinned to a known version. The CLI binary version could drift independently of what cc-deck expects.
- **Testability**: gRPC services have established mocking patterns. The generated client interface can be replaced with a mock server in tests.
- **Error semantics**: gRPC status codes (NotFound, Unavailable, DeadlineExceeded) are structured. CLI errors are just stderr text that needs parsing.

### Weaknesses

- **Build complexity**: Adds protoc toolchain, proto file vendoring, and code generation to the project. Currently the project has zero proto infrastructure.
- **mTLS configuration**: The gateway uses mTLS by default. The CLI handles this transparently (it knows where the certs are). A direct gRPC client needs explicit cert path configuration, which varies by platform and OpenShell version.
- **Proto API stability**: OpenShell is alpha (v0.0.36). The proto API could change between releases. Pinning helps but creates a manual update burden.
- **Does not eliminate CLI dependency**: Still needs the CLI for gateway management, SSH proxy, and git ext:: transport. So the CLI must be installed regardless.
- **Larger change surface**: Complete client rewrite plus new dependencies plus build infrastructure. More code to maintain, more things to break.
- **gRPC connection management**: Need to handle connection lifecycle (dial, reconnect, close) that the CLI handles implicitly per-invocation.

## Approach B: CLI Wrapping (Fixed)

Fix the current CLI syntax to match the real `openshell` CLI. Keep the `os/exec` transport but make it correct.

**What this involves:**
- Fix CLI command syntax to match real openshell CLI:
  - `sandbox create --from <image> -- <command>` (not `--image`/`--command`)
  - `sandbox exec -n <name> -- <cmd>` (not positional sandbox ID)
  - `sandbox get <name>` (not `--output json`)
  - `sandbox delete <name>`
  - `sandbox ssh-config <name>` or `sandbox connect <name>`
- Fix output parsing to match real CLI output formats (JSON where available)
- Remove the `--gateway` per-command flag (gateway is configured globally)
- Configure the gateway via `openshell gateway add` or env var before use

**Does NOT need:**
- Proto files, protoc, code generation
- google.golang.org/grpc dependency
- TLS certificate handling (CLI does it)
- Streaming protocol implementation

### Strengths

- **Simpler implementation**: Fix existing code rather than rewrite. Change ~50 lines of CLI arguments vs. ~500 lines of gRPC client + interface + codegen infrastructure.
- **CLI handles complexity**: mTLS, gateway discovery, cert management, credential injection, SSH proxy setup. All handled transparently by the CLI. cc-deck does not need to know about any of it.
- **Version alignment**: If the user has `openshell` v0.0.36 installed, the CLI syntax matches that version. No proto version pinning needed.
- **Proven pattern**: cc-deck already wraps other CLIs successfully (ssh, git, podman, zellij). The SSH workspace backend works this way. It is a known, debuggable pattern.
- **Smaller change surface**: Fewer new files, no new dependencies, no build toolchain changes. Lower risk of introducing bugs.
- **CLI output is a contract**: The CLI's `--output json` flag produces structured JSON that is more stable than raw proto wire format (OpenShell team actively maintains CLI UX backwards compatibility).

### Weaknesses

- **Output parsing fragility**: Even with JSON output, parsing CLI output is less type-safe than proto-generated types. If the JSON schema changes, parsing breaks silently.
- **Process overhead**: Each operation spawns a subprocess. For polling (GetSandbox every 2s during creation), that is many process spawns vs. a single persistent gRPC connection.
- **Streaming limitations**: `ExecSandbox` through the CLI captures all stdout as a flat byte buffer. No separation of stdout vs stderr events. Exit codes come from the process exit, which works but loses gRPC-level error context.
- **Error handling**: CLI errors are stderr strings. "sandbox not found" vs. "gateway unreachable" vs. "permission denied" all need string matching rather than structured error codes.
- **Version coupling**: The CLI binary must be compatible with the gateway version. If the user upgrades the gateway but not the CLI (or vice versa), things break silently.
- **Harder to test**: Mocking subprocess execution is less clean than mocking a Go interface. Current tests do not test through the client at all, partly because of this.

## Approach C: Hybrid (gRPC for Lifecycle, CLI for Transport)

Use gRPC for sandbox CRUD (create, get, delete, status) where type safety matters most. Keep CLI wrapping for SSH proxy and git ext:: transport where the CLI provides genuine value.

This is effectively Approach A but acknowledging that the CLI dependency remains for SSH and git. The question becomes: is the gRPC migration for CRUD operations worth the complexity, given that you cannot fully eliminate the CLI?

## Comparison Matrix

| Dimension | gRPC (A) | CLI Fixed (B) |
|-----------|----------|---------------|
| Type safety | Strong (proto) | Weak (JSON parsing) |
| Build complexity | High (protoc + deps) | None |
| New dependencies | grpc, protobuf | None |
| CLI still needed? | Yes (gateway, SSH, git) | Yes (everything) |
| Streaming exec | Native server stream | Subprocess stdout capture |
| Error handling | gRPC status codes | String matching |
| mTLS handling | Manual cert config | CLI handles it |
| Test mocking | Clean interface mock | Subprocess mock |
| Lines of change | ~500+ new, ~200 modified | ~50 modified |
| Maintenance burden | Proto updates + codegen | CLI syntax changes |
| Time to working local run | Days | Hours |

## Open Questions

1. **How stable is the OpenShell CLI output format?** If `openshell sandbox get --output json` produces stable JSON, the parsing fragility argument weakens significantly.

2. **Does the process-per-operation overhead matter?** For cc-deck's usage pattern (create once, poll a few times, attach, done), the overhead of subprocess spawning is negligible.

3. **How often does the proto API change?** At v0.0.36 with daily releases, the proto could change frequently. But the CLI adapts automatically.

4. **Is the gRPC investment worthwhile for alpha software?** If OpenShell reaches 1.0 with a stable proto API, gRPC becomes clearly better. At v0.0.x, the investment may be premature.

5. **What is the real-world failure mode?** CLI wrapping fails with "openshell not found" (clear) or output format changes (debuggable). gRPC fails with TLS errors, proto version mismatches, or connection management bugs (harder to debug for users).

---

## Revisit: 2026-06-26

### Updated Problem Framing

Six weeks of production CLI wrapping (Approach B) have exposed concrete failure modes that the original analysis predicted but underestimated:

**Vertex provider migration (brainstorm #075)**: Switching from cc-deck's homegrown Vertex credential handling to OpenShell's native `google-cloud` provider required three consecutive bug fixes:
1. `CreateProvider` needed `--from-gcloud-adc` instead of `--from-existing` for `google-cloud` type, plus `--config` instead of `--credential`
2. `UpdateProvider` had the same flag incompatibility (the update command does not support `--from-gcloud-adc` at all)
3. `EnsureProvider` failed when deleting a provider attached to stale sandboxes

Each of these was a runtime failure discovered only during manual testing. With gRPC, the `Provider` proto message has separate `credentials` and `config` map fields, so there is no flag confusion. The `CreateProvider` and `UpdateProvider` RPCs accept the same `Provider` struct. These bugs would not have existed.

**Supervisor/gateway version mismatch**: Building the gateway from source (`main` branch) pulled `supervisor:latest` from ghcr.io, which was from a different release and required `OPENSHELL_SANDBOX_TOKEN` that the gateway wasn't injecting. This was a runtime crash with no compile-time signal.

**CLI flag changes across versions**: The CLI changed from positional `image` argument to `--from <image>`. The `provider create` command added `--from-gcloud-adc` and `--config` flags. The `provider update` command does NOT support `--from-gcloud-adc` even though `create` does. None of these asymmetries are discoverable until runtime.

### Key Finding: CLI is NOT Needed for SSH or File Transfer

The original analysis assumed "CLI still needed for SSH and git." Research into the actual OpenShell CLI source reveals this is wrong:

**SSH**: The CLI itself calls `CreateSshSession` via gRPC (`ssh.rs:98`) to get a token, gateway host, and port. It then builds a `ProxyCommand` that points to the `openshell` binary as a tunnel proxy (HTTP CONNECT through the gateway). In Go, this tunnel can be implemented natively: call `CreateSshSession` via gRPC, then use `golang.org/x/crypto/ssh` with a custom `net.Conn` that tunnels through the gateway's HTTP CONNECT endpoint.

**File upload**: The CLI implements upload as SSH + tar pipe (`ssh.rs:751`). It opens an SSH session, then streams a tar archive over stdin to `tar xf -` on the remote side. No special gRPC RPC exists. Go can do the same via its SSH library once the tunnel is established.

**Interactive exec**: The `ExecSandboxInteractive` RPC is a bidirectional gRPC stream. Full PTY support without the CLI.

This means **the CLI can be fully eliminated as a runtime dependency**. It is only needed for one-time gateway setup (`openshell gateway start`), which is a setup concern, not a runtime concern.

### Updated Comparison Matrix

| Dimension | gRPC (A, updated) | CLI Fixed (B, current) |
|-----------|-------------------|----------------------|
| Type safety | Strong (proto, compile-time) | Weak (string flags, runtime failures) |
| Build complexity | Medium (protoc + deps, one-time setup) | None |
| New dependencies | grpc, protobuf, x/crypto/ssh | None |
| CLI runtime dependency | None (setup only) | Required for everything |
| Provider creation | `CreateProviderRequest{Provider{config: {...}}}` | `--from-gcloud-adc --config key=value` (version-dependent) |
| Provider update | Same proto as create | Different flags than create (no `--from-gcloud-adc`) |
| Streaming exec | Native server/bidi stream | Subprocess stdout capture |
| File upload | Go SSH + tar pipe (same as CLI internally) | CLI subprocess + tar pipe |
| Error handling | gRPC status codes | String matching |
| mTLS handling | Load certs from known XDG/brew paths | CLI handles it |
| Test mocking | Clean interface mock (existing `Client` interface) | Subprocess mock |
| Lines of change | ~800 new (grpc client + SSH tunnel + proto) | ~50 per CLI flag fix |
| Failure discovery | Compile time (proto mismatch) | Runtime (wrong flags, output changes) |
| K8s operator readiness | gRPC client reusable for operator controller | CLI wrapping will not work in-cluster |

### New Approaches Considered

The three original approaches (A: gRPC, B: CLI, C: Hybrid) still apply. The revisit adds evidence and corrects assumptions.

**Original Approach A is now stronger** because:
- CLI is NOT needed for SSH or file transfer (corrects the original assumption)
- Real-world CLI flag breakage (Vertex provider) validates the compile-time safety argument
- K8s operator work (brainstorm #1719) needs gRPC anyway (CLI wrapping does not work in-cluster)

**Original Approach B is now weaker** because:
- Three runtime bugs in a single feature migration (Vertex provider)
- Provider create/update API asymmetry only discoverable at runtime
- No path to K8s operator integration

**Original Approach C (Hybrid) is eliminated** because the SSH/upload CLI dependency assumption was wrong. There is no reason to keep a partial CLI dependency.

### Updated Decision

**Chosen: Approach A (Full gRPC replacement)**

Implement a `grpcClient` that implements the existing `Client` interface. The `Client` interface stays unchanged, so `ws/openshell.go` and all callers do not need modifications.

**Implementation plan:**
1. Add proto codegen infrastructure (Makefile target, vendored proto files pinned to a release tag)
2. Implement `grpcClient` with mTLS connection setup (cert paths from XDG/brew locations)
3. Migrate sandbox CRUD: `CreateSandbox`, `GetSandbox`, `DeleteSandbox`
4. Migrate provider management: `CreateProvider`, `UpdateProvider`, `DeleteProvider`, `EnsureProvider`
5. Migrate exec: `ExecSandbox` (server-streaming), `ExecSandboxInteractive` (bidi-streaming)
6. Implement Go-native SSH tunnel: `CreateSshSession` gRPC call + HTTP CONNECT proxy + `golang.org/x/crypto/ssh`
7. Migrate file transfer: `Upload` and `Download` via SSH + tar pipe over Go SSH
8. Remove `cliClient` (keep as `cliClient_legacy.go` behind build tag for one release)

**Proto pinning strategy:** Vendor proto files from a specific OpenShell release tag (not `main`). When updating, regenerate and fix compile errors. This makes API changes visible and intentional.

**mTLS cert resolution:** Check paths in order: `$OPENSHELL_LOCAL_TLS_DIR` env var, `~/.local/state/openshell/homebrew/tls/`, XDG data dir. Fall back to insecure (no TLS) for localhost connections (matching CLI behavior).

### Open Threads
- Proto file vendoring: copy from OpenShell release tarball or use `buf` for dependency management?
- SSH tunnel implementation: use `golang.org/x/crypto/ssh` directly or `net/http` HTTP CONNECT to gateway tunnel endpoint?
- The edge tunnel (WebSocket-based, for gateways behind Cloudflare) is complex. Defer until needed (local dev and K8s do not use edge auth).
- Should the legacy `cliClient` be kept behind a build tag for one release, or removed immediately?
