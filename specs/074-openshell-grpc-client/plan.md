# Implementation Plan: OpenShell gRPC Client

**Branch**: `074-openshell-grpc-client` | **Date**: 2026-06-26 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `specs/074-openshell-grpc-client/spec.md`

## Summary

Replace the CLI-wrapping `cliClient` in `internal/openshell/client.go` with a `grpcClient` that talks directly to the OpenShell gateway via gRPC. The `Client` interface stays unchanged. Proto files are vendored from a specific OpenShell release tag and code-generated into Go types. SSH tunnel and file transfer are implemented in Go using `golang.org/x/crypto/ssh` with HTTP CONNECT through the gateway. The old `cliClient` is retained behind a `cli_legacy` build tag.

## Technical Context

**Language/Version**: Go 1.25 (from go.mod)
**Primary Dependencies**: google.golang.org/grpc (NEW), google.golang.org/protobuf (already indirect), golang.org/x/crypto/ssh (NEW), existing internal packages
**Storage**: N/A
**Testing**: `make test` (Go tests), `make lint` (Go linters)
**Target Platform**: macOS, Linux (CLI tool)
**Project Type**: CLI tool
**Constraints**: Client interface must stay unchanged; proto files from OpenShell v0.0.46 release

## Constitution Check

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Tests and docs | PASS | Unit tests for grpcClient. README update for changed dependency (CLI no longer required at runtime). |
| II. Interface contracts | PASS | Client interface unchanged. grpcClient satisfies the same contract as cliClient. |
| III. Build and tool rules | PASS | Use `make test`, `make lint`. Add `make proto` target for codegen. |
| IV. Plugin debug logging | N/A | No plugin changes. |

## Project Structure

### Source Code (files to create/modify)

```text
cc-deck/internal/openshell/
├── iface.go              # UNCHANGED - Client interface
├── client.go             # RENAMED to client_grpc.go - new gRPC implementation
├── client_legacy.go      # OLD cliClient behind //go:build cli_legacy tag
├── conn.go               # NEW - gRPC connection setup, mTLS cert discovery
├── tunnel.go             # NEW - SSH tunnel via HTTP CONNECT + golang.org/x/crypto/ssh
├── credentials.go        # UNCHANGED - credential resolution logic
├── credentials_test.go   # UNCHANGED
└── proto/                # NEW - vendored proto files + generated Go code
    ├── openshell.proto
    ├── datamodel.proto
    ├── sandbox.proto
    ├── openshell.pb.go       # generated
    ├── openshell_grpc.pb.go  # generated
    ├── datamodel.pb.go       # generated
    └── sandbox.pb.go         # generated

cc-deck/internal/ws/
└── openshell.go          # UNCHANGED - uses Client interface

Makefile                  # ADD proto target
```

## Research Findings

### RPC-to-Interface Method Mapping

| Client Method | gRPC RPC | Notes |
|---------------|----------|-------|
| `CreateSandbox(image, command, policy, providers)` | `CreateSandbox(CreateSandboxRequest)` | Build request with image, command, policy path, provider names |
| `GetSandbox(name)` | `GetSandbox(GetSandboxRequest{name})` | Map response sandbox state to `SandboxInfo` |
| `DeleteSandbox(name)` | `DeleteSandbox(DeleteSandboxRequest{name})` | Direct mapping |
| `ExecSandbox(name, cmd)` | `ExecSandbox(ExecSandboxRequest)` | Server-streaming, collect stdout/stderr/exit |
| `ExecSandboxStream(name, cmd)` | `ExecSandbox(ExecSandboxRequest)` | Same RPC, stream output to os.Stdout |
| `AttachExec(name, cmd)` | `ExecSandboxInteractive(bidi stream)` | Bidirectional streaming with PTY |
| `Upload(name, local, remote)` | `CreateSshSession` + SSH tar pipe | No upload RPC; use SSH tunnel |
| `Download(name, remote, local)` | `CreateSshSession` + SSH tar pipe | No download RPC; use SSH tunnel |
| `CreateProvider(name, type, fromExisting, creds)` | `CreateProvider(CreateProviderRequest{Provider})` | Map to Provider proto with credentials + config |
| `UpdateProvider(name, type, fromExisting, creds)` | `UpdateProvider(UpdateProviderRequest{Provider})` | Same Provider proto, no flag asymmetry |
| `DeleteProvider(name)` | `DeleteProvider(DeleteProviderRequest{name})` | Direct mapping |
| `EnsureProvider(...)` | Create, catch AlreadyExists, Update | Business logic stays in Go, not proto |

### mTLS Certificate Discovery

Resolution order (matching CLI behavior):
1. `$OPENSHELL_LOCAL_TLS_DIR` env var
2. `~/.local/state/openshell/homebrew/tls/` (brew install)
3. `$XDG_DATA_HOME/openshell/tls/` (manual install)
4. If localhost and no certs found: insecure connection (no TLS)

Files needed: `ca.crt`, `client/tls.crt`, `client/tls.key`

### SSH Tunnel Architecture

The CLI implements file transfer as: `CreateSshSession` gRPC call -> get token + gateway host:port -> build ProxyCommand -> SSH + tar pipe.

For Go-native implementation:
1. Call `CreateSshSession` RPC to get `token`, `gateway_host`, `gateway_port`
2. Open HTTP CONNECT to `gateway_host:gateway_port` with token in header
3. The CONNECT tunnel gives a raw TCP connection to the sandbox's SSH server
4. Use `golang.org/x/crypto/ssh` to establish SSH session over that connection
5. For upload: pipe tar archive over SSH stdin to `tar xf - -C <dest>`
6. For download: pipe `tar cf - <path>` over SSH stdout to local tar extraction

### Proto Codegen

Required proto files: `openshell.proto`, `datamodel.proto`, `sandbox.proto`
(Skip `inference.proto`, `compute_driver.proto`, `test.proto` as not needed)

Makefile target:
```
proto:
	protoc --go_out=. --go-grpc_out=. \
	  --go_opt=module=github.com/cc-deck/cc-deck \
	  --go-grpc_opt=module=github.com/cc-deck/cc-deck \
	  cc-deck/internal/openshell/proto/*.proto
```

## Complexity Tracking

No constitution violations. No complexity justifications needed.
