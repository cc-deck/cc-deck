# Brainstorm: K8s Integration Tests

**Date**: 2026-03-11
**Status**: active
**Feature**: cc-deck (Kubernetes CLI)
**Affects**: cc-deck/internal/integration/, .github/workflows/, Containerfile.stub

## Problem

The cc-deck K8s CLI has ~5,400 lines of Go code implementing deploy, connect, list, delete, logs, sync, and profile commands. Existing tests only cover resource generation (unit tests in `resources_test.go`, `network_test.go`, `sync_test.go`). There are no tests that verify the actual K8s API interactions: creating StatefulSets, waiting for Pods, cleaning up resources, or detecting duplicate sessions.

Without integration tests, regressions in the deploy/delete lifecycle go undetected until manual testing.

## Decisions

- **Test infrastructure**: kind (Kubernetes in Docker/Podman) for full cluster lifecycle
- **CI runtime**: Docker on GitHub Actions (`ubuntu-latest`), podman locally via `KIND_EXPERIMENTAL_PROVIDER=podman`
- **Container image**: Build a stub image in CI from a minimal Containerfile (Alpine + sleep)
- **Assertion library**: testify (`github.com/stretchr/testify/assert`, `require`)
- **Build tag**: `//go:build integration` to separate from unit tests

## Phase 1: Core Lifecycle (Spec Target)

Test the fundamental deploy/list/delete lifecycle against a kind cluster.

### Test Matrix

| Test | What it verifies |
|------|-----------------|
| `TestDeployCreatesResources` | StatefulSet, Service, PVC, ConfigMap, NetworkPolicy all exist with correct labels after deploy |
| `TestDeployPodReachesRunning` | Pod reaches Running phase within timeout |
| `TestDeployDuplicateNameFails` | Second deploy with same name returns `ResourceConflictError` |
| `TestListShowsDeployedSession` | `session.List()` returns the deployed session with correct metadata |
| `TestDeleteRemovesAllResources` | All resources (StatefulSet, Service, NetworkPolicy, ConfigMap) gone after delete |
| `TestDeployWithNoNetworkPolicy` | `--no-network-policy` flag skips NetworkPolicy creation |
| `TestNetworkPolicyEgressRules` | NetworkPolicy contains correct egress rules for Anthropic backend |
| `TestDeployWithCustomStorage` | PVC uses the specified storage size |

### Stub Container Image

Minimal Containerfile that satisfies the deploy requirements:

```dockerfile
FROM docker.io/library/alpine:3.21
CMD ["sleep", "infinity"]
```

The Pod just needs to reach Running phase. No HTTP server needed for Phase 1 since `WaitForPodRunning` checks Pod phase, not readiness probes.

### Test Infrastructure

```
cc-deck/
  test/
    Containerfile.stub          # Stub image for integration tests
  internal/
    integration/
      integration_test.go       # TestMain + core lifecycle tests
      helpers_test.go           # Shared utilities (create client, wait, cleanup)
```

**TestMain lifecycle:**
1. Check for existing kind cluster (`cc-deck-test`) or create one
2. Build and load stub image
3. Create test namespace + dummy Secret
4. Run tests
5. Delete kind cluster (skip if `KEEP_CLUSTER=1`)

**Parallel safety:** Each test uses a unique session name (e.g., `test-deploy-<random>`) to allow parallel execution within the same namespace.

### GitHub Actions Workflow

```yaml
name: Integration Tests
on: [push, pull_request]
jobs:
  integration:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: cc-deck/go.mod
      - uses: helm/kind-action@v1
        with:
          cluster_name: cc-deck-test
      - name: Build stub image
        run: docker build -t localhost/cc-deck-stub:latest -f cc-deck/test/Containerfile.stub .
      - name: Load image into kind
        run: kind load docker-image localhost/cc-deck-stub:latest --name cc-deck-test
      - name: Setup test namespace
        run: |
          kubectl create namespace cc-deck-test
          kubectl create secret generic test-api-key \
            --from-literal=api-key="test-key" -n cc-deck-test
      - name: Run integration tests
        run: |
          cd cc-deck
          go test -tags integration -v -timeout 5m ./internal/integration/
```

### Test Helper Design

```go
// helpers_test.go

// testEnv holds shared state for the test suite
type testEnv struct {
    clientset  kubernetes.Interface
    restConfig *rest.Config
    namespace  string
    image      string
    imageTag   string
}

// newTestSession creates deploy options with a unique name
func (e *testEnv) newTestSession(t *testing.T) session.DeployOptions

// assertResourceExists checks a K8s resource exists by GVR + name
func (e *testEnv) assertResourceExists(t *testing.T, gvr schema.GroupVersionResource, name string)

// assertResourceNotExists checks a K8s resource does not exist
func (e *testEnv) assertResourceNotExists(t *testing.T, gvr schema.GroupVersionResource, name string)

// cleanup deletes all resources for a session name (best-effort)
func (e *testEnv) cleanup(t *testing.T, sessionName string)
```

## Phase 2: Extended Tests (Future)

### Connect Tests

| Test | Challenge | Approach |
|------|-----------|----------|
| `TestConnectExec` | Needs TTY, interactive | Use `remotecommand` API directly, run `echo hello` and check output |
| `TestConnectPortForward` | Needs HTTP server in Pod | Upgrade stub to include socat, verify HTTP response on forwarded port |
| `TestConnectAutoDetect` | No Route/Ingress in kind | Defaults to exec, verify method selection |

### Sync Tests

| Test | What it verifies |
|------|-----------------|
| `TestSyncPushFiles` | Files from local dir appear in Pod at `/workspace` |
| `TestSyncPullFiles` | Files created in Pod are pulled back locally |
| `TestSyncExcludes` | Excluded patterns are not synced |

Sync tests need the Pod to be running, so they depend on a successful deploy. The stub image needs `tar` (included in Alpine).

### Logs Tests

| Test | What it verifies |
|------|-----------------|
| `TestLogsStreams` | `logs` returns Pod stdout content |
| `TestLogsFollow` | `--follow` streams in real-time (with timeout) |

### NetworkPolicy Enforcement

Testing actual egress blocking requires a CNI that enforces NetworkPolicies. kind's default CNI (kindnet) does not enforce them. Options:
- Use kind with Calico CNI (adds ~30s setup time)
- Accept that NP creation is tested (resource exists) but enforcement is not

**Recommendation**: Test creation only in CI. Enforcement testing is a manual/staging concern.

### OpenShift-Specific Tests

Route creation and EgressFirewall require an OpenShift cluster. These can't run in kind. Options:
- Mock the discovery client to report OpenShift capabilities
- Test resource generation (unit test level) rather than application
- Run on a real OpenShift cluster in a separate CI job (needs ROSA/OCP access)

**Recommendation**: Unit test resource generation for OpenShift resources. Integration test only the vanilla K8s path.

## Phase 3: CI Pipeline (Future)

### Full Workflow

```
on push/PR:
  1. Unit tests (go test ./...) - fast, no cluster
  2. Clippy + cargo test (Rust plugin) - fast
  3. Integration tests (kind) - slow, deploy/list/delete lifecycle
  4. Build release binaries (optional, on tag)
```

### Caching

- Go module cache: `actions/setup-go` handles this
- kind image cache: Not worth it (kind creates fresh clusters fast)
- Stub image: Build is <5s, not worth caching

### Timeouts

- Individual test timeout: 2 minutes (deploy + wait for running + delete)
- Suite timeout: 5 minutes total
- kind cluster creation: ~60s on GitHub runners

## Local Development

```bash
# Create cluster (one-time, reuse across test runs)
KIND_EXPERIMENTAL_PROVIDER=podman kind create cluster --name cc-deck-test

# Build and load stub image
podman build -t localhost/cc-deck-stub:latest -f cc-deck/test/Containerfile.stub .
KIND_EXPERIMENTAL_PROVIDER=podman kind load docker-image localhost/cc-deck-stub:latest --name cc-deck-test

# Setup namespace (one-time)
kubectl create namespace cc-deck-test
kubectl create secret generic test-api-key --from-literal=api-key="test" -n cc-deck-test

# Run tests (reuses existing cluster)
cd cc-deck
KEEP_CLUSTER=1 go test -tags integration -v -timeout 5m ./internal/integration/

# Cleanup when done
KIND_EXPERIMENTAL_PROVIDER=podman kind delete cluster --name cc-deck-test
```

## Dependencies

- `github.com/stretchr/testify` (assert, require) for test assertions
- `sigs.k8s.io/kind` is NOT a Go dependency, it's a CLI tool used externally
- No new Go dependencies needed beyond testify

## Out of Scope

- OpenShift Route/EgressFirewall integration testing (needs real OCP cluster)
- NetworkPolicy enforcement testing (needs Calico CNI)
- Container image build pipeline (separate project)
- Load/stress testing
- E2E tests with real Claude Code image
