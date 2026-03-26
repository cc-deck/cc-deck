# Research: K8s Integration Tests

## R1: Integration Test Infrastructure for Go + K8s

**Decision**: Use kind (Kubernetes in Docker/Podman) for full cluster lifecycle testing.

**Rationale**: kind provides a real K8s API server with kubelet, enabling testing of Pod scheduling, phase transitions, and resource creation. envtest (API server + etcd only) cannot test Pod Running phase or scheduling, which is central to the deploy workflow.

**Alternatives considered**:
- **envtest** (`sigs.k8s.io/controller-runtime/pkg/envtest`): Faster startup (~5s) but cannot verify Pod lifecycle. Only tests API-level CRUD. Rejected because `WaitForPodRunning` is a core workflow step.
- **Minikube**: Heavier than kind, slower to create/destroy. No advantage for CI.
- **k3d**: Similar to kind but uses k3s. Less mature GitHub Actions support.

## R2: CI Runtime for kind

**Decision**: Docker on GitHub Actions, podman locally via `KIND_EXPERIMENTAL_PROVIDER=podman`.

**Rationale**: `ubuntu-latest` runners have Docker pre-installed. `helm/kind-action@v1` GitHub Action handles cluster creation, image loading, and cleanup. Adding podman to CI would require extra installation steps with no benefit.

**Alternatives considered**:
- **Podman in CI**: Requires manual installation on ubuntu runners. Adds 30-60s and complexity. No functional benefit since kind abstracts the runtime.

## R3: Stub Container Image

**Decision**: Minimal Alpine image with `sleep infinity`. Built in CI from a Containerfile.

**Rationale**: The deploy workflow's `WaitForPodRunning` checks Pod phase, not readiness probes or HTTP endpoints. The simplest image that reaches Running phase is sufficient. Building in CI (vs. pulling from a registry) avoids maintaining a published image and keeps the test self-contained.

**Alternatives considered**:
- **Pre-built image on ghcr.io**: Simpler CI setup but requires image maintenance and publishing pipeline.
- **`busybox`/`alpine` directly**: Could work with command override, but the StatefulSet hardcodes `command: ["zellij"]`. The stub Containerfile overrides CMD so it works with the existing resource builder.

## R4: Test Assertion Library

**Decision**: `github.com/stretchr/testify` (assert + require packages).

**Rationale**: testify is the de facto Go testing companion. `require` stops on first failure (for setup steps), `assert` continues (for validation steps). Clean diff output on assertion failures.

**Alternatives considered**:
- **stdlib only**: More verbose, harder to read assertion failures.
- **gomega/ginkgo**: BDD style is overkill for integration tests.

## R5: Parallel Test Execution

**Decision**: All tests use `t.Parallel()` with unique session names per test.

**Rationale**: Integration tests are I/O-bound (waiting for Pod scheduling). Running them concurrently against the same kind cluster reduces total wall time from ~8 minutes (sequential, 8 tests x ~60s each) to ~2 minutes (parallel, limited by slowest test + scheduling contention).

**Design**:
- Each test generates a unique session name: `fmt.Sprintf("t-%s-%s", testShortName, randomSuffix)`
- All tests share the same namespace, K8s client, and stub image reference via the `testEnv` struct
- `t.Cleanup()` ensures resources are deleted even on failure
- No test depends on another test's state

## R6: Existing Code Entry Points

**Decision**: Tests call the `session.Deploy()`, `session.Delete()`, `session.List()` Go functions directly, not the CLI binary.

**Rationale**: Testing at the function level is faster, avoids binary rebuilds, and gives direct access to error types (e.g., `ResourceConflictError`). The CLI layer (cobra commands) is a thin wrapper that doesn't need integration testing.

**Key functions**:
- `session.Deploy(ctx, DeployOptions)` - creates all resources, waits for Pod Running
- `session.Delete(ctx, DeleteOptions)` - removes all resources
- `session.List(ctx, ListOptions)` - queries cluster for sessions
- `k8s.NewClient(ClientOptions)` - creates K8s client from kubeconfig
- `k8s.DetectCapabilities(discovery)` - detects cluster features

## R7: Test Skip Mechanism

**Decision**: `TestMain` attempts to connect to a kind cluster. If it fails, all tests are skipped with `t.Skip("no kind cluster available")`.

**Rationale**: Developers running `go test -tags integration ./...` without a kind cluster should see skips, not failures. This also means CI must set up the cluster before running tests.

**Detection**: Try `kubernetes.NewForConfig(restConfig)` with the kubeconfig. If the cluster is unreachable, skip.
