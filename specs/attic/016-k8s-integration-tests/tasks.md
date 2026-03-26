# Tasks: K8s Integration Tests

**Input**: Design documents from `/specs/016-k8s-integration-tests/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: Setup

**Purpose**: Test infrastructure, stub image, and testify dependency

- [ ] T001 (cc-mux-8q7.1) Add testify dependency: `cd cc-deck && go get github.com/stretchr/testify`
- [ ] T002 (cc-mux-8q7.2) [P] Create stub Containerfile at cc-deck/test/Containerfile.stub (Alpine + `CMD ["sleep", "infinity"]`)
- [ ] T003 (cc-mux-8q7.3) [P] Create test helpers in cc-deck/internal/integration/helpers_test.go: testEnv struct, newTestSession(), assertResourceExists(), assertResourceNotExists(), cleanup(). Build tag `//go:build integration`.

**Checkpoint**: testify available, stub image buildable, helper code compiles

---

## Phase 2: Core Lifecycle Tests (US1)

**Goal**: Deploy/list/delete lifecycle verified against kind cluster

**Independent Test**: Create kind cluster, run `go test -tags integration ./internal/integration/`, verify lifecycle tests pass

### Implementation

- [ ] T004 (cc-mux-u47.1) [US1] Create integration_test.go with TestMain: load kubeconfig, connect to kind cluster (skip if unavailable), create test namespace + dummy Secret, initialize testEnv. Build tag `//go:build integration`. File: cc-deck/internal/integration/integration_test.go
- [ ] T005 (cc-mux-u47.2) [P] [US1] TestDeployCreatesResources: deploy a session, assert StatefulSet, Service, ConfigMap, PVC, NetworkPolicy exist with correct labels. Uses t.Parallel(). File: cc-deck/internal/integration/integration_test.go
- [ ] T006 (cc-mux-u47.3) [P] [US1] TestDeployPodReachesRunning: deploy a session, assert Pod reaches Running phase within 60s. Uses t.Parallel(). File: cc-deck/internal/integration/integration_test.go
- [ ] T007 (cc-mux-u47.4) [P] [US1] TestDeployDuplicateNameFails: deploy a session, deploy again with same name, assert ResourceConflictError. Uses t.Parallel(). File: cc-deck/internal/integration/integration_test.go
- [ ] T008 (cc-mux-u47.5) [P] [US1] TestListShowsDeployedSession: deploy a session, call list, assert session appears with correct name and namespace. Uses t.Parallel(). File: cc-deck/internal/integration/integration_test.go
- [ ] T009 (cc-mux-u47.6) [P] [US1] TestDeleteRemovesAllResources: deploy a session, delete it, assert StatefulSet, Service, ConfigMap, NetworkPolicy are gone. Uses t.Parallel(). File: cc-deck/internal/integration/integration_test.go

**Checkpoint**: Core lifecycle tests pass against kind cluster

---

## Phase 3: Resource Validation Tests (US3)

**Goal**: Verify resource configuration matches deploy options

**Independent Test**: Run specific test cases with custom deploy options, assert generated resources match expectations

### Implementation

- [ ] T010 (cc-mux-pfp.1) [P] [US3] TestDeployWithNoNetworkPolicy: deploy with NoNetworkPolicy=true, assert no NetworkPolicy exists. Uses t.Parallel(). File: cc-deck/internal/integration/integration_test.go
- [ ] T011 (cc-mux-pfp.2) [P] [US3] TestDeployCustomStorageSize: deploy with StorageSize="5Gi", assert PVC requests 5Gi. Uses t.Parallel(). File: cc-deck/internal/integration/integration_test.go
- [ ] T012 (cc-mux-pfp.3) [P] [US3] TestNetworkPolicyEgressRules: deploy with Anthropic profile, assert NetworkPolicy egress rules contain api.anthropic.com:443 and DNS:53. Uses t.Parallel(). File: cc-deck/internal/integration/integration_test.go

**Checkpoint**: Resource validation tests pass, 9 total test cases (exceeds SC-005 target of 8)

---

## Phase 4: CI Workflow (US2)

**Goal**: GitHub Actions workflow runs integration tests on push/PR

### Implementation

- [ ] T013 (cc-mux-cdw.1) [US2] Create GitHub Actions workflow at .github/workflows/integration.yaml: checkout, setup-go, kind-action, build stub image, load into kind, create namespace + Secret, run integration tests with 5m timeout

**Checkpoint**: CI workflow runs successfully on push

---

## Phase 5: Validation

**Purpose**: End-to-end verification

- [ ] T014 (cc-mux-5x8.1) Run all integration tests locally against kind cluster, verify all 9 tests pass in parallel within 2 minutes
- [ ] T015 (cc-mux-5x8.2) [P] Verify `go test ./...` (without integration tag) skips integration tests and passes

---

## Dependencies and Execution Order

### Phase Dependencies

- **Phase 1 (Setup)**: No dependencies, can start immediately
- **Phase 2 (Core Tests)**: Depends on T001 (testify), T003 (helpers), T004 (TestMain)
- **Phase 3 (Validation Tests)**: Depends on T003, T004. Can run in parallel with Phase 2.
- **Phase 4 (CI)**: Depends on T002 (Containerfile). Can run in parallel with Phase 2/3.
- **Phase 5 (Validation)**: Depends on all phases complete

### Parallel Opportunities

- T002 and T003 can run in parallel (different files)
- T005-T009 can run in parallel with T010-T012 (different test cases, same file)
- T013 can run in parallel with T005-T012 (CI workflow vs test code)
- T014 and T015 can run in parallel

---

