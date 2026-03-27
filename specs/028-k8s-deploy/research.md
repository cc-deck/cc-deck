# Research: Kubernetes Deploy Environment

**Feature**: 028-k8s-deploy | **Date**: 2026-03-27

## Research Topics & Findings

### 1. Resource Naming and Labeling

**Decision**: All K8s resources use the `cc-deck-<name>` prefix, consistent with the existing container/compose naming convention (`containerName()` returns `cc-deck-<name>` in container.go).

**Labels** (applied to all generated resources):
```yaml
labels:
  app.kubernetes.io/name: cc-deck
  app.kubernetes.io/instance: <env-name>
  app.kubernetes.io/managed-by: cc-deck
  app.kubernetes.io/component: workspace    # or "mcp-<name>" for sidecars
```

**Rationale**: Kubernetes recommended labels enable standard tooling (kubectl, dashboards) to recognize and filter cc-deck resources. The `managed-by` label distinguishes cc-deck resources from manually created ones, supporting reconciliation.

**Alternatives considered**: Custom `cc-deck.io/` label prefix. Rejected because standard `app.kubernetes.io/` labels are better recognized by tooling and require no CRD.

### 2. Kubeconfig Context Handling

**Decision**: Support both `--kubeconfig` (file path) and `--context` (context name) flags. Default to the current kubeconfig (`~/.kube/config` or `$KUBECONFIG`) and current context.

**Rationale**: This matches kubectl's own behavior. Users with multiple clusters need context selection. The `--kubeconfig` flag is already listed in K8sFields but `--context` is new.

**Implementation**: Use `client-go`'s `clientcmd.NewNonInteractiveDeferredLoadingClientConfig()` with explicit overrides for kubeconfig path and context name. Store the resolved context name in K8sFields for later operations.

**Alternatives considered**: Only supporting `--kubeconfig`. Rejected because many users have a single kubeconfig with multiple contexts, and switching via `--kubeconfig` alone is cumbersome.

### 3. ESO API Version

**Decision**: Use `external-secrets.io/v1` for all ExternalSecret CRD generation.

**Rationale**: ESO v0.17.0 removed `v1beta1` support entirely. The current stable release is v1.3.2, well past the cutoff. All modern ESO installations require `v1`. The schema is identical to `v1beta1` (only the apiVersion field changes).

**Alternatives considered**: Supporting both `v1beta1` and `v1` with auto-detection. Rejected because `v1beta1` is no longer served by current ESO versions and adds unnecessary complexity.

### 4. NetworkPolicy IP Resolution

**Decision**: Resolve domain-to-IP at environment creation time (point-in-time snapshot). No automatic refresh. Users can recreate the NetworkPolicy via `cc-deck env update` (future feature) or delete/recreate the environment.

**Rationale**: This matches the compose environment's proxy whitelist behavior (resolved once at creation). K8s NetworkPolicies require IP CIDR blocks, not domain names. DNS records change, but in practice, major cloud provider API endpoints use stable IPs. The NetworkPolicy serves as a best-effort security layer, not a firewall replacement.

**Implementation**: Use `net.LookupHost()` for each resolved domain. Convert results to `/32` CIDR blocks. For wildcard domains (e.g., `.anthropic.com`), resolve the base domain only (subdomains typically share IP ranges). Document the limitation in the user guide.

**Alternatives considered**:
- DNS-based NetworkPolicy controllers (e.g., Cilium's FQDN policies). Rejected because it couples cc-deck to a specific CNI plugin.
- Periodic refresh via CronJob. Rejected for complexity (YAGNI).

### 5. Git Harvest Implementation

**Decision**: Use git's `ext::` remote helper protocol with `kubectl exec` for bidirectional git operations.

**Format**: `ext::kubectl exec -i <pod-name> -n <namespace> -- %S /workspace`

**Rationale**: The `ext::` protocol is a native git feature that tunnels git-upload-pack and git-receive-pack over any bidirectional command. With `kubectl exec -i`, stdin/stdout are connected to the pod, which is exactly what `ext::` needs. The pod must have `git` installed (which the cc-deck base image includes).

**Push flow**:
1. Add remote: `git remote add k8s "ext::kubectl exec -i cc-deck-<name>-0 -n <ns> -- %S /workspace"`
2. Push: `git push k8s <branch>`

**Harvest flow**:
1. Add remote (same as push)
2. Fetch: `git fetch k8s`
3. Create local branch: `git checkout -b <branch> k8s/<branch>`

**Alternatives considered**:
- `kubectl cp` with tar. Rejected because it loses git history.
- Port-forwarding a git SSH server. Rejected for complexity.

### 6. Image Pull Authentication

**Decision**: Assume cluster has pull secrets pre-configured. This is the responsibility of the cluster administrator, not cc-deck.

**Rationale**: Image pull authentication varies dramatically across cluster setups (ImagePullSecrets on ServiceAccount, namespace-level defaults, node-level credentials, Harbor/Quay robot accounts). Adding cc-deck-level pull secret management would duplicate what cluster admins already handle and introduces security risks.

**Documentation**: The user guide will note that private registry access must be configured at the cluster level before creating k8s-deploy environments.

### 7. OpenShift Route Target

**Decision**: Generate a Route only when a web-accessible port is exposed. For the initial implementation, this targets the Zellij web UI (port 8080 if configured in the pod). Routes are optional and only created when OpenShift API groups are detected AND a web port is specified.

**Rationale**: Not all environments need web access. Generating Routes unconditionally would create unused resources and potential security exposure.

**Implementation**: Detect OpenShift via API discovery (`route.openshift.io/v1`). If detected and `--web-port` is specified (or the manifest defines one), generate a Route targeting that port on the headless Service.

### 8. Stub Image for Integration Tests

**Decision**: Use a minimal Alpine image with `sleep infinity` as the entrypoint. No Zellij installation needed. This matches the existing `cc-deck/test/Containerfile.stub` pattern already used in the CI workflow.

**Rationale**: Integration tests verify K8s resource lifecycle (create/start/stop/delete), not Zellij session behavior. Testing actual Zellij sessions inside K8s is an E2E concern that would require the full cc-deck base image and significantly longer test times.

**Implementation**: The existing `Containerfile.stub` in `cc-deck/test/` creates a fake zellij binary that sleeps forever. This is sufficient for verifying that the StatefulSet creates pods, PVCs persist data, and lifecycle operations work correctly.

### 9. client-go vs kubectl exec

**Decision**: Use `client-go` for K8s API operations (CRUD for StatefulSet, Service, ConfigMap, Secret, NetworkPolicy, PVC). Use `kubectl exec` (via `os/exec`) for interactive operations (attach, exec, push/pull).

**Rationale**:
- `client-go` provides type-safe, programmatic access to the K8s API. It handles authentication, retries, and watches correctly. It avoids shelling out for every API call.
- `kubectl exec` is necessary for interactive TTY attachment (client-go's SPDY exec is complex and fragile for TTY use cases). The existing Container/Compose pattern shells out to `podman exec` for the same reason.

**Dependencies to add**: `k8s.io/client-go`, `k8s.io/api`, `k8s.io/apimachinery`

### 10. Manifest File Name

**Discovery**: The build manifest file is named `cc-deck-image.yaml` (not `cc-deck-build.yaml` as referenced in the spec's `--build-dir` flag description). The spec's FR-016 lists `--build-dir` which is the directory containing the manifest, not the manifest file itself.

**Impact**: The implementation must use `build.LoadManifest()` from `internal/build/manifest.go` to parse the manifest from the build directory.

### 11. MCP Sidecar Image Gap

**Discovery**: The current `MCPEntry` struct in `internal/build/manifest.go` does NOT have an `image` field. It has `Name`, `Transport`, `Port`, `Auth`, and `Description`. For k8s-deploy, MCP sidecars need a container image reference.

**Decision**: Add an optional `Image` field to `MCPEntry` in the manifest schema. When `Image` is set, the MCP server runs as a sidecar container. When `Image` is empty (stdio/local transport), the MCP server is assumed to run inside the main container (no sidecar needed).

**Impact**: This is a backward-compatible schema change (new optional field). Existing manifests without `image` fields continue to work.

### 12. OpenShift EgressFirewall API

**Discovery**: EgressFirewall uses the `k8s.ovn.org/v1` API group (not `network.openshift.io`). Route uses `route.openshift.io/v1`.

**Detection**: Use the Kubernetes discovery client to check for these API groups before generating resources.

**Implementation**:
```go
// Check for OpenShift Route support
_, err := discoveryClient.ServerResourcesForGroupVersion("route.openshift.io/v1")
hasRoutes := err == nil

// Check for EgressFirewall support
_, err = discoveryClient.ServerResourcesForGroupVersion("k8s.ovn.org/v1")
hasEgressFirewall := err == nil
```

### 13. Existing Codebase Patterns Summary

**Key patterns from lifecycle research**:
- Create: validate name > check tool > check conflict > resolve definition > create resources > record definition > record instance
- Attach: nested Zellij check > auto-start > timestamp > session detect/create
- Delete: running check > best-effort cleanup of ALL resources > remove instance > remove definition
- Status: reconcile stored state against actual runtime state
- State store: `AddInstance()`/`UpdateInstance()`/`RemoveInstance()` for v2 instances
- Definition store: parallel persistence for user-editable configuration
- Credential handling: file-based (bind mount read-only) vs plain (env var/secret)
- Error handling: fast-fail validations, best-effort cleanup, atomic store writes

**Key patterns from CLI research**:
- Flags registered as local flags using `cmd.Flags()`
- Type-specific field injection via type assertion after factory creation
- Precedence: CLI flag > project definition > config default > hardcoded default
- `cmd.Flags().Changed()` detects explicit CLI flag usage

**Key patterns from network research**:
- `network.Resolver` is the single source of truth for domain expansion
- User config at `$XDG_CONFIG_HOME/cc-deck/domains.yaml`
- Override syntax: `+group` (add), `-group` (remove), `group` (replace), `all` (disable)
- Compose integration: resolve domains, pass to generator, generate proxy config

**Key patterns from testing research**:
- Table-driven tests with testify assert/require
- Test isolation via `t.TempDir()`, `t.Setenv()`
- External tools stubbed with minimal shell scripts in PATH
- Integration tests gated by `//go:build integration` tag
- CI uses `helm/kind-action@v1` for kind cluster setup
- Existing `Containerfile.stub` and `integration.yaml` workflow ready for extension
