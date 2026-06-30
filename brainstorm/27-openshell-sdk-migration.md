# Brainstorm: OpenShell SDK Migration

**Date:** 2026-06-30
**Status:** active

## Problem Framing

cc-deck's OpenShell integration (`internal/openshell/`) currently wraps the `openshell` CLI binary via `os/exec.Command`. Every gateway operation (CreateSandbox, DeleteSandbox, GetSandbox, ExecSandbox, Upload, Download, provider CRUD) spawns a child process, captures stdout, and parses the text output.

This approach has several problems:

1. **Fragile output parsing.** The CLI uses ANSI color codes, progress spinners, and non-deterministic output formatting. The `execCLICaptureName` function reads partial output then kills the process because the CLI blocks with a spinner in non-TTY mode. Error detection relies on string matching ("not found", "already exists").

2. **CLI binary dependency.** The `openshell` binary must be installed on the host. This adds an external dependency that's hard to version-pin and impossible to vendor.

3. **No structured errors.** CLI exit codes and stderr messages lose the gRPC status codes that the gateway returns. Error handling degrades to substring matching.

4. **No testability.** The CLI wrapper can't be unit tested without a running gateway and the CLI binary installed. There's no fake or mock implementation.

5. **Missing capabilities.** The CLI doesn't expose Watch (event streaming), WaitReady (blocking until sandbox is ready), or InteractiveSession (bidirectional PTY). These are available only via the gRPC API.

The OpenShell SDK (`github.com/rhuss/openshell-sdk-go`) is a Go client library that talks gRPC directly to the gateway. It provides typed sub-clients for sandboxes, providers, exec, files, SSH, health, and policy. It includes a `fake` package (in-memory implementation following the client-go/kubernetes/fake pattern) for consumer test suites.

## Approaches Considered

### A: Big Bang Replacement (Chosen)

Replace `internal/openshell/client.go` entirely. Drop the custom `Client` interface and `cliClient` struct. Import the SDK's `ClientInterface` and sub-clients directly. Consumers call `sdk.Sandboxes().Create(...)` instead of `client.CreateSandbox(...)`. Remove all CLI output parsing code (ANSI stripping, sandbox name parsing, process kill hack). Add the SDK via `go.mod` with a `replace` directive pointing to the local checkout during development.

- Pros: Cleanest result. No dead code or adapter layers. Full access to SDK features. One migration pass. No CLI binary required. Structured gRPC errors. Fake client for tests.
- Cons: Touches ~22 consumer files in one pass. Larger PR. Needs `replace` directive until the SDK is published. All-or-nothing for the gRPC dependency.

### B: Adapter Behind Current Interface

Keep the `Client` interface. Create an `sdkClient` struct implementing it by delegating to the SDK. Replace `NewClient` to return `sdkClient`. Zero consumer changes.

- Pros: Smallest PR. Zero consumer changes. Easy rollback.
- Cons: Loses the SDK's richer type system (SandboxSpec, Watch, InteractiveSession) behind the flat interface. Adding new capabilities later requires interface expansion anyway. Creates an unnecessary adapter layer that will be thrown away.

### C: Phased Migration

Phase 1: Add SDK dependency, create adapter (like B). Phase 2: Expand interface for new capabilities. Phase 3: Migrate consumers to SDK types. Phase 4: Remove adapter.

- Pros: Incremental, each phase reviewable independently.
- Cons: 4 PRs for what could be 1. The intermediate adapter is throwaway code. Interface evolves 3 times, more churn than replacing once.

## Decision

**Approach A: Big Bang Replacement.**

The current interface is thin (8 methods + 4 provider methods). The SDK's sub-client pattern is idiomatic Go (like client-go). The 22 consumer files include many build/test files that reference "openshell" as a string literal, not the Client interface. The actual interface consumers are concentrated in `internal/ws/` (workspace layer) and `internal/cmd/build.go`. An adapter layer (B) would be thrown away once we want Watch or InteractiveSession, so it's wasted effort. A phased approach (C) creates the same amount of total diff across 4 PRs with intermediate throwaway code.

## Key Requirements

- **Drop CLI wrapper**: Remove `cliClient`, `execCLI`, `execCLICaptureName`, `parseSandboxName`, `parseSandboxState`, `stripANSI` and all CLI output parsing code from `internal/openshell/client.go`
- **Import SDK**: Add `github.com/rhuss/openshell-sdk-go` to `go.mod` with a `replace` directive to `/Users/rhuss/Work/projects/openshell/openshell-sdk-go` during development
- **Use SDK types directly**: Consumers use `v1.ClientInterface`, `v1.SandboxInterface`, `v1.ProviderInterface`, etc.
- **Keep credentials.go**: Credential detection/resolution logic (`DetectCredentials`, `ResolveCredentials`, `KnownProviderProfiles`) is independent of transport and stays in `internal/openshell/credentials.go`
- **Map GatewayConfig to SDK Config**: The existing `GatewayConfig` and `ResolveGatewayConfig` should produce the SDK's `v1.Config` struct (address, TLS, auth)
- **Leverage fake client**: Unit tests use `fake.NewClient()` from the SDK's `fake` package instead of mocking the CLI
- **Scope boundary**: This migration replaces the transport layer only. New SDK capabilities (Watch, WaitReady, InteractiveSession) are follow-up work, not part of this migration

## Open Questions

- How does `GatewayConfig` (Address, TLS, TLSCertPath, TLSKeyPath, TLSCAPath) map to the SDK's `Config` type? The SDK uses `types.TLSConfig` and `types.AuthProvider`. Does `ResolveGatewayConfig` become a factory for `v1.Config`?
- Should `credentials.go` be refactored to use the SDK's `v1.Provider` and `v1.ProviderSpec` types for its output, or keep the current `ProviderConfig` mapping layer?
- Which of the 22 consumer files actually import the `openshell.Client` interface vs. just referencing "openshell" as a string? The spec phase should audit the exact consumer surface.
- The `replace` directive in `go.mod` is developer-specific (absolute path). Should CI use a published version, or should the SDK be published before this migration ships?
