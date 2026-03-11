# Local Kubernetes Testing with kind

Run cc-deck integration tests locally using kind (Kubernetes in Docker/Podman).

## Prerequisites

- `podman` (container runtime)
- `kind` ([kubernetes-sigs/kind](https://kind.sigs.k8s.io/))
- `kubectl`
- Go 1.22+

## Quick Setup

### 1. Create a kind cluster with podman

```bash
KIND_EXPERIMENTAL_PROVIDER=podman kind create cluster --name cc-deck-test
```

Verify the cluster is running:

```bash
kubectl cluster-info --context kind-cc-deck-test
```

### 2. Create a test namespace and dummy Secret

```bash
kubectl create namespace cc-deck-test
kubectl create secret generic test-api-key \
  --from-literal=api-key="test-key-not-real" \
  -n cc-deck-test
```

### 3. Build a stub container image

The stub image replaces the real Claude Code image for testing. It runs a minimal HTTP server on port 8082 (matching the Zellij web server port) and provides a shell for exec testing.

```bash
# Create a minimal test image
cat <<'EOF' | podman build -t localhost/cc-deck-stub:latest -f - .
FROM docker.io/library/alpine:3.21
RUN apk add --no-cache bash socat
# Minimal HTTP server on port 8082 so the Pod stays healthy
CMD ["sh", "-c", "socat TCP-LISTEN:8082,fork,reuseaddr SYSTEM:'echo HTTP/1.1 200 OK; echo; echo ok' & exec sleep infinity"]
EOF
```

Load the image into the kind cluster:

```bash
KIND_EXPERIMENTAL_PROVIDER=podman kind load docker-image localhost/cc-deck-stub:latest --name cc-deck-test
```

### 4. Create a test config

```bash
mkdir -p /tmp/cc-deck-test
cat > /tmp/cc-deck-test/config.yaml <<EOF
default_profile: test
defaults:
  namespace: cc-deck-test
  image: localhost/cc-deck-stub
  image_tag: latest
profiles:
  test:
    backend: anthropic
    api_key_secret: test-api-key
EOF
```

### 5. Build and test cc-deck

```bash
cd cc-deck
go build -o /tmp/cc-deck-test/cc-deck ./cmd/cc-deck

export CCDECK=/tmp/cc-deck-test/cc-deck
export CCDECK_CONFIG=/tmp/cc-deck-test/config.yaml
export KUBECONFIG=$(KIND_EXPERIMENTAL_PROVIDER=podman kind get kubeconfig-path --name cc-deck-test 2>/dev/null || echo "$HOME/.kube/config")
```

### 6. Run the full lifecycle manually

```bash
# Deploy
$CCDECK deploy test-session --config $CCDECK_CONFIG --namespace cc-deck-test

# Verify resources created
kubectl get statefulset,svc,pvc,netpol -n cc-deck-test

# List sessions
$CCDECK list --config $CCDECK_CONFIG --namespace cc-deck-test

# Check Pod is running
kubectl get pods -n cc-deck-test

# Connect via exec (opens shell in the stub container)
$CCDECK connect test-session --config $CCDECK_CONFIG --namespace cc-deck-test

# Delete
$CCDECK delete test-session --config $CCDECK_CONFIG --namespace cc-deck-test

# Verify cleanup
kubectl get statefulset,svc,pvc,netpol -n cc-deck-test
```

## Cleanup

```bash
KIND_EXPERIMENTAL_PROVIDER=podman kind delete cluster --name cc-deck-test
rm -rf /tmp/cc-deck-test
```

## Integration Test Design

The integration tests use the same kind cluster approach but automated in Go.

### Test structure

```
cc-deck/internal/integration/
    integration_test.go    # Test suite with kind cluster lifecycle
    helpers_test.go        # Shared test utilities
```

### What to test

| Test | Verifies |
|------|----------|
| `TestDeployCreatesResources` | StatefulSet, Service, PVC, ConfigMap, NetworkPolicy all created with correct labels |
| `TestDeployDuplicateNameFails` | Second deploy with same name returns ResourceConflictError |
| `TestDeployWaitForRunning` | Pod reaches Running phase within timeout |
| `TestListShowsSessions` | `list` command output includes deployed session |
| `TestDeleteCleansUp` | All resources removed after delete |
| `TestNetworkPolicyEgress` | NetworkPolicy has correct egress rules for profile backend |
| `TestDeployWithOverlay` | Kustomize overlay merges correctly |
| `TestConnectExec` | Exec attach reaches the container shell |
| `TestSyncPush` | Files appear in Pod after sync |
| `TestSyncPull` | Files copied back from Pod after sync pull |
| `TestProfileValidation` | Deploy fails with missing Secret |

### Build tag

Use a build tag so integration tests don't run during `go test ./...`:

```go
//go:build integration

package integration
```

Run with:

```bash
go test -tags integration -v ./internal/integration/ -count=1
```

### Cluster lifecycle

The test suite creates the kind cluster in `TestMain`, runs all tests, then tears it down:

```go
func TestMain(m *testing.M) {
    // Create kind cluster (or reuse existing)
    // Load stub image
    // Create namespace + test Secret
    code := m.Run()
    // Delete kind cluster (unless KEEP_CLUSTER=1)
    os.Exit(code)
}
```

Set `KEEP_CLUSTER=1` to skip teardown for debugging failed tests.

### Podman socket

kind with podman requires the podman socket. Start it if not running:

```bash
# macOS (podman machine)
podman machine start

# Linux (rootless)
systemctl --user start podman.socket
```

## Notes

- The stub image uses `socat` for a minimal HTTP server, not actual Zellij. This is enough to verify the deploy/connect/delete lifecycle.
- Integration tests are slow (cluster creation takes 30-60s). Run them separately from unit tests.
- The `--no-network-policy` flag is useful in kind since kind's CNI may not enforce NetworkPolicies by default.
- For NetworkPolicy enforcement testing, use kind with Calico: `kind create cluster --config kind-calico.yaml`.
