# Research: OpenShell gRPC Client

## Proto API Surface

**Decision**: Vendor 3 proto files from OpenShell release tag: `openshell.proto`, `datamodel.proto`, `sandbox.proto`.

**Rationale**: These cover all RPCs cc-deck needs (sandbox CRUD, provider management, exec, SSH sessions). The other proto files (`inference.proto`, `compute_driver.proto`, `test.proto`) are for gateway-internal or testing use.

**Alternatives considered**: Using `buf` for proto management. Rejected because it adds another tool dependency. Simple file copying from a release tarball is sufficient for 3 files.

## Provider Message Structure

**Decision**: Map cc-deck's `credentials map[string]string` parameter to the proto `Provider.credentials` and `Provider.config` fields based on provider type.

**Rationale**: The proto `Provider` message has separate `credentials` (secrets) and `config` (non-secret) maps. For `google-cloud` providers, `project_id` and `region` go into `config`, not `credentials`. This eliminates the CLI flag confusion entirely.

**Implementation**: The `grpcClient.CreateProvider` method builds a `Provider` proto message, placing values in the correct map based on the provider type. The `fromExisting` bool maps to setting appropriate credential discovery flags in the request.

## SSH Tunnel for File Transfer

**Decision**: Implement Go-native HTTP CONNECT tunnel to gateway, then SSH over that tunnel.

**Rationale**: The OpenShell CLI uses exactly this pattern (`ssh.rs:75-140`). The gateway's HTTP CONNECT endpoint accepts a session token and tunnels to the sandbox's SSH server. Go's `net/http` and `golang.org/x/crypto/ssh` provide all the building blocks.

**Key insight from CLI source**: The SSH user is always `sandbox` (hardcoded). The authentication uses the session token from `CreateSshSession`, not SSH keys.

## mTLS vs Insecure Connection

**Decision**: Auto-detect TLS mode based on gateway address and cert availability.

**Rationale**: The CLI does the same. Localhost connections typically run without TLS (dev mode). Remote gateways use mTLS. cc-deck should match this behavior.

**Implementation**: Check if gateway address is localhost/127.0.0.1. If yes and no certs found, use insecure. Otherwise, load certs from standard paths and configure mTLS.

## Build Tag for Legacy Client

**Decision**: Move current `cliClient` to `client_legacy.go` with `//go:build cli_legacy` tag.

**Rationale**: Users can opt into the old behavior with `go build -tags cli_legacy` for one release cycle. After that, remove the file entirely.

**Implementation**: The build tag controls which `NewClient` function is compiled. Both return a `Client` interface, so callers are unaffected.
