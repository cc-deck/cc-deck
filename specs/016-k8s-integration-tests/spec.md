# Feature Specification: K8s Integration Tests

**Feature Branch**: `016-k8s-integration-tests`
**Created**: 2026-03-11
**Status**: Draft
**Input**: User description: "Integration tests for cc-deck Kubernetes CLI using kind clusters and GitHub Actions CI"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Core Lifecycle Tests Run Locally (Priority: P1)

A developer working on cc-deck's Kubernetes code runs integration tests locally against a kind cluster. The tests verify the full deploy/list/delete lifecycle: creating a session deploys the expected K8s resources, listing shows the session, and deleting removes all resources. The developer gets fast feedback on whether their changes break the core K8s interactions without needing a remote cluster.

**Why this priority**: The deploy/list/delete lifecycle is the backbone of the K8s CLI. If these operations break, nothing else works. Local execution is essential for the development feedback loop.

**Independent Test**: Can be tested by creating a kind cluster, running `go test -tags integration ./internal/integration/`, and verifying all tests pass. Delivers immediate confidence that K8s API interactions work correctly.

**Acceptance Scenarios**:

1. **Given** a kind cluster is running and a test namespace exists, **When** the integration test suite runs, **Then** all core lifecycle tests (deploy, list, delete) pass within 5 minutes.
2. **Given** a deploy test runs, **When** the test completes, **Then** a StatefulSet, headless Service, ConfigMap, PVC, and NetworkPolicy all exist in the test namespace with correct labels and configuration.
3. **Given** a deployed session exists, **When** the list function is called, **Then** the session appears in the results with correct name, namespace, and status.
4. **Given** a deployed session exists, **When** the delete function is called, **Then** all associated K8s resources (StatefulSet, Service, ConfigMap, NetworkPolicy) are removed from the namespace.
5. **Given** a session is already deployed with a given name, **When** a second deploy with the same name is attempted, **Then** a duplicate conflict error is returned without modifying existing resources.
6. **Given** a deploy with the no-network-policy option, **When** the test completes, **Then** no NetworkPolicy resource exists for that session.

---

### User Story 2 - Tests Run in GitHub Actions CI (Priority: P1)

A developer pushes a commit or opens a pull request on GitHub. A CI workflow automatically runs the integration tests in a GitHub Actions runner, using a kind cluster. The workflow reports pass/fail status on the PR, catching regressions before merge.

**Why this priority**: CI execution prevents regressions from reaching the main branch. Without automated CI, integration testing depends on developer discipline to run tests locally.

**Independent Test**: Can be tested by pushing a commit and verifying the GitHub Actions workflow completes successfully, with integration test results visible in the PR checks.

**Acceptance Scenarios**:

1. **Given** a push to any branch or a pull request is opened, **When** the CI workflow triggers, **Then** a kind cluster is created, stub image is loaded, test namespace is set up, and integration tests run.
2. **Given** all integration tests pass, **When** the workflow completes, **Then** the CI check shows a green status.
3. **Given** an integration test fails, **When** the workflow completes, **Then** the CI check shows a red status with the specific test failure visible in the logs.
4. **Given** the CI workflow runs, **When** the total execution time is measured, **Then** the entire workflow (cluster creation + image load + tests) completes within 5 minutes.

---

### User Story 3 - Resource Validation Tests (Priority: P2)

A developer changes the resource generation code (StatefulSet spec, NetworkPolicy rules, ConfigMap content). Integration tests verify that the generated resources are applied correctly to the cluster and contain the expected configuration, going beyond unit tests which only check in-memory objects.

**Why this priority**: Unit tests verify resource generation but don't catch issues with K8s API validation, field defaults, or server-side mutation. Integration tests close this gap.

**Independent Test**: Can be tested by running specific test cases that deploy with various configurations and asserting the resulting resources match expectations.

**Acceptance Scenarios**:

1. **Given** a deploy with a custom storage size (e.g., "5Gi"), **When** the PVC is created, **Then** the PVC's requested storage matches "5Gi".
2. **Given** a deploy with an Anthropic profile, **When** the NetworkPolicy is created, **Then** the egress rules allow traffic to the Anthropic API endpoint on port 443 and DNS on port 53.
3. **Given** a deploy with additional egress hosts, **When** the NetworkPolicy is created, **Then** the egress rules include the extra hosts alongside the default backend rules.
4. **Given** a deploy, **When** the Pod starts, **Then** it reaches the Running phase within the configured timeout (60 seconds).

---

### Edge Cases

- What happens when the kind cluster is not running? Tests skip with a clear message indicating no cluster is available.
- What happens when the stub image is not loaded into kind? Pod stays in ImagePullBackOff. The deploy test fails with a timeout, and the error includes Pod events for diagnosis.
- What happens when two tests deploy with the same session name? Each test uses a unique session name to avoid collisions.
- What happens when a test fails mid-lifecycle (after deploy, before delete)? Test cleanup runs automatically to ensure resources are removed even on failure.
- What happens when the test namespace doesn't exist? The test suite creates the namespace before running tests.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The test suite MUST use build tags so integration tests do not run during standard test invocations.
- **FR-002**: The test suite MUST verify that deploy creates all expected K8s resources: StatefulSet, headless Service, ConfigMap, PVC, and NetworkPolicy.
- **FR-003**: The test suite MUST verify that deploying with a duplicate session name returns a conflict error.
- **FR-004**: The test suite MUST verify that delete removes all K8s resources created by deploy (StatefulSet, Service, ConfigMap, NetworkPolicy).
- **FR-005**: The test suite MUST verify that list returns deployed sessions with correct metadata.
- **FR-006**: The test suite MUST verify that deploying with the no-network-policy option skips NetworkPolicy creation.
- **FR-007**: The test suite MUST verify that the deployed Pod reaches Running phase within the configured timeout.
- **FR-008**: The test suite MUST verify that custom storage sizes are reflected in the created PVC.
- **FR-009**: The test suite MUST verify that NetworkPolicy egress rules match the profile's backend configuration.
- **FR-010**: A CI workflow MUST run the integration tests on every push and pull request.
- **FR-011**: The CI workflow MUST create a kind cluster, build and load a stub container image, set up a test namespace with a dummy credential Secret, and run the test suite.
- **FR-012**: Each test MUST use a unique session name to allow parallel-safe execution.
- **FR-013**: Each test MUST register cleanup to remove deployed resources regardless of test outcome.
- **FR-017**: Tests MUST run in parallel (using `t.Parallel()`) to minimize total execution time. Each test is self-contained with its own session name and cleanup, enabling safe concurrent execution against the shared kind cluster.
- **FR-014**: The test suite MUST gracefully skip (not fail) when no kind cluster is available.
- **FR-015**: The stub container image MUST be minimal and run indefinitely so the Pod stays in Running phase.
- **FR-016**: The test suite MUST use a test assertion library for clear, readable assertions.

### Key Entities

- **Test Environment**: Shared state for the test suite including cluster client, namespace, and stub image coordinates. Created once at suite startup and reused across tests.
- **Stub Image**: A minimal container image that satisfies the deploy workflow's Pod Running requirement without needing real application software.
- **Test Session**: A deploy configuration with a unique name, test namespace, and dummy credential profile, created per-test to ensure isolation.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: The core lifecycle tests (deploy, list, delete) all pass against a kind cluster within 2 minutes of test execution time (excluding cluster setup), achieved through parallel test execution.
- **SC-002**: The CI workflow completes end-to-end (cluster creation through test results) within 5 minutes on a standard CI runner.
- **SC-003**: Running the standard test command without the integration tag completes successfully with integration tests automatically skipped.
- **SC-004**: Test failures produce actionable output including the specific resource state and cluster events, enabling diagnosis without manual cluster inspection.
- **SC-005**: At least 8 test cases cover the core deploy/list/delete lifecycle, duplicate detection, network policy variations, and resource validation.

## Assumptions

- CI runners have a container runtime pre-installed and sufficient resources to run a kind cluster.
- kind can create a usable cluster within 60 seconds on standard CI runners.
- kind's default CNI does not enforce NetworkPolicies. Tests verify NetworkPolicy resource creation, not traffic enforcement.
- The stub image does not need to serve HTTP or provide a shell for the core lifecycle tests.
- Local developers may use a different container runtime (podman) than CI (Docker), and the test setup accommodates both.
- OpenShift-specific features (Routes, EgressFirewall) are not testable in kind and are excluded from this test suite.
