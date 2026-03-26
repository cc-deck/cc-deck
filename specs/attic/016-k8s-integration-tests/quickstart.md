# Quickstart: Running K8s Integration Tests

## Local Development

### One-time setup

```bash
# Create kind cluster (podman)
KIND_EXPERIMENTAL_PROVIDER=podman kind create cluster --name cc-deck-test

# Build and load stub image
podman build -t localhost/cc-deck-stub:latest -f cc-deck/test/Containerfile.stub .
KIND_EXPERIMENTAL_PROVIDER=podman kind load docker-image localhost/cc-deck-stub:latest --name cc-deck-test

# Create test namespace and dummy Secret
kubectl create namespace cc-deck-test
kubectl create secret generic test-api-key --from-literal=api-key="test-key" -n cc-deck-test
```

### Run tests

```bash
cd cc-deck
go test -tags integration -v -timeout 5m ./internal/integration/
```

### Reuse cluster across runs

```bash
# Keep cluster alive (skip teardown)
KEEP_CLUSTER=1 go test -tags integration -v ./internal/integration/
```

### Cleanup

```bash
KIND_EXPERIMENTAL_PROVIDER=podman kind delete cluster --name cc-deck-test
```

## CI (GitHub Actions)

Tests run automatically on push and PR via `.github/workflows/integration.yaml`. The workflow creates a kind cluster, builds the stub image, and runs the test suite.
