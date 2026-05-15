# Brainstorm: OpenShell Gateway Communication - gRPC vs CLI Wrapping

**Date:** 2026-05-06
**Status:** active
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
- **Doesn't eliminate CLI dependency**: Still needs the CLI for gateway management, SSH proxy, and git ext:: transport. So the CLI must be installed regardless.
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
- **CLI handles complexity**: mTLS, gateway discovery, cert management, credential injection, SSH proxy setup. All handled transparently by the CLI. cc-deck doesn't need to know about any of it.
- **Version alignment**: If the user has `openshell` v0.0.36 installed, the CLI syntax matches that version. No proto version pinning needed.
- **Proven pattern**: cc-deck already wraps other CLIs successfully (ssh, git, podman, zellij). The SSH workspace backend works this way. It's a known, debuggable pattern.
- **Smaller change surface**: Fewer new files, no new dependencies, no build toolchain changes. Lower risk of introducing bugs.
- **CLI output is a contract**: The CLI's `--output json` flag produces structured JSON that's more stable than raw proto wire format (OpenShell team actively maintains CLI UX backwards compatibility).

### Weaknesses

- **Output parsing fragility**: Even with JSON output, parsing CLI output is less type-safe than proto-generated types. If the JSON schema changes, parsing breaks silently.
- **Process overhead**: Each operation spawns a subprocess. For polling (GetSandbox every 2s during creation), that's many process spawns vs. a single persistent gRPC connection.
- **Streaming limitations**: `ExecSandbox` through the CLI captures all stdout as a flat byte buffer. No separation of stdout vs stderr events. Exit codes come from the process exit, which works but loses gRPC-level error context.
- **Error handling**: CLI errors are stderr strings. "sandbox not found" vs. "gateway unreachable" vs. "permission denied" all need string matching rather than structured error codes.
- **Version coupling**: The CLI binary must be compatible with the gateway version. If the user upgrades the gateway but not the CLI (or vice versa), things break silently.
- **Harder to test**: Mocking subprocess execution is less clean than mocking a Go interface. Current tests don't test through the client at all, partly because of this.

## Approach C: Hybrid (gRPC for Lifecycle, CLI for Transport)

Use gRPC for sandbox CRUD (create, get, delete, status) where type safety matters most. Keep CLI wrapping for SSH proxy and git ext:: transport where the CLI provides genuine value.

This is effectively Approach A but acknowledging that the CLI dependency remains for SSH and git. The question becomes: is the gRPC migration for CRUD operations worth the complexity, given that you can't fully eliminate the CLI?

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

5. **What's the real-world failure mode?** CLI wrapping fails with "openshell not found" (clear) or output format changes (debuggable). gRPC fails with TLS errors, proto version mismatches, or connection management bugs (harder to debug for users).
