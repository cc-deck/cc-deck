# Tasks: Kubernetes Deploy Environment

**Input**: Design documents from `/specs/028-k8s-deploy/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md, contracts/

**Tests**: Integration tests with kind are MANDATORY (User Story 7). Unit tests for resource generation and credential management.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Add K8s dependencies and create base structure

- [ ] T001 Add k8s.io/client-go, k8s.io/api, and k8s.io/apimachinery dependencies to cc-deck/go.mod
- [ ] T002 Create K8sDeployEnvironment struct with fields from data-model.md in cc-deck/internal/env/k8s_deploy.go
- [ ] T003 Register K8sDeployEnvironment in factory switch in cc-deck/internal/env/factory.go

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: K8s client infrastructure and CLI flag wiring that ALL user stories depend on

**CRITICAL**: No user story work can begin until this phase is complete

- [ ] T004 Create K8s client helper (kubeconfig loading, context selection, discovery client) in cc-deck/internal/env/k8s_client.go
- [ ] T005 [P] Add k8s-deploy-specific CLI flags (--namespace, --kubeconfig, --context, --storage-size, --storage-class, --credential, --existing-secret, --secret-store, --secret-store-ref, --secret-path, --build-dir, --no-network-policy, --allow-domain, --allow-group, --keep-volumes, --timeout) to createFlags struct and flag registration in cc-deck/internal/cmd/env.go
- [ ] T006 [P] Add type assertion block for K8sDeployEnvironment field injection in runEnvCreate() in cc-deck/internal/cmd/env.go
- [ ] T007 [P] Add k8s-deploy-specific fields (Namespace, Kubeconfig, Context, StorageSize, StorageClass, etc.) to EnvironmentDefinition struct in cc-deck/internal/env/definition.go
- [ ] T008 Add precedence resolution for k8s-deploy definition values in runEnvCreate() in cc-deck/internal/cmd/env.go

**Checkpoint**: Foundation ready, K8s client can connect, CLI accepts k8s-deploy flags

---

## Phase 3: User Story 1 - Deploy a Persistent K8s Environment (Priority: P1) MVP

**Goal**: Create, attach to, stop, start, and delete a k8s-deploy environment through standard `cc-deck env` commands

**Independent Test**: Create a k8s-deploy environment against a kind cluster, attach, create a file, detach, re-attach, verify file persists

### Tests for User Story 1

- [ ] T009 [P] [US1] Unit tests for K8s resource generation (StatefulSet, Service, ConfigMap specs) in cc-deck/internal/env/k8s_resources_test.go
- [ ] T010 [P] [US1] Unit tests for Create, Start, Stop, Delete lifecycle methods in cc-deck/internal/env/k8s_deploy_test.go

### Implementation for User Story 1

- [ ] T011 [P] [US1] Implement K8s resource generation: GenerateResources() producing StatefulSet, headless Service, and ConfigMap per contracts/k8s-resource-generation.md in cc-deck/internal/env/k8s_resources.go
- [ ] T012 [US1] Implement Create method: validate name, check kubectl, check conflict, generate resources, apply to cluster via client-go, record definition and instance state in cc-deck/internal/env/k8s_deploy.go
- [ ] T013 [US1] Implement cleanup on Create failure: delete any partially created resources (Service, ConfigMap, StatefulSet) before returning error in cc-deck/internal/env/k8s_deploy.go
- [ ] T014 [US1] Implement Attach method: nested Zellij detection, auto-start if stopped, timestamp update, kubectl exec into Pod with Zellij session creation in cc-deck/internal/env/k8s_deploy.go
- [ ] T015 [P] [US1] Implement Start method (scale StatefulSet to replicas=1, wait for Pod readiness, update state) and Stop method (scale to replicas=0, update state) in cc-deck/internal/env/k8s_deploy.go
- [ ] T016 [US1] Implement Delete method: running check, best-effort cleanup of StatefulSet/Service/ConfigMap/PVC (unless --keep-volumes), remove instance and definition from state stores in cc-deck/internal/env/k8s_deploy.go
- [ ] T017 [US1] Implement Type(), Name(), and stub methods (Exec, Push, Pull, Harvest returning ErrNotSupported) in cc-deck/internal/env/k8s_deploy.go

**Checkpoint**: Core lifecycle (create/attach/stop/start/delete) works against a K8s cluster

---

## Phase 4: User Story 2 - Credential Management (Priority: P1)

**Goal**: Securely provide credentials via inline key-value pairs, existing Secrets, or External Secrets Operator

**Independent Test**: Create environment with inline credentials, verify Secret exists with correct data; create with --existing-secret, verify no new Secret created but mount works

### Tests for User Story 2

- [ ] T018 [P] [US2] Unit tests for credential Secret generation (inline, existing, ESO) per contracts/credential-management.md in cc-deck/internal/env/k8s_credentials_test.go

### Implementation for User Story 2

- [ ] T019 [US2] Implement inline credential handling: create K8s Secret with key-value pairs, add volume mount at /run/secrets/cc-deck/ in cc-deck/internal/env/k8s_credentials.go
- [ ] T020 [US2] Implement existing Secret reference: skip Secret creation, add volume mount for user-managed Secret in cc-deck/internal/env/k8s_credentials.go
- [ ] T021 [US2] Implement ESO integration: check for ESO CRDs via discovery, generate ExternalSecret CR with external-secrets.io/v1 in cc-deck/internal/env/k8s_credentials.go
- [ ] T022 [US2] Integrate credential handling into Create method: call credential functions before resource application, merge credential volumes into StatefulSet spec in cc-deck/internal/env/k8s_deploy.go
- [ ] T023 [US2] Update Delete method: cleanup inline Secrets (cc-deck-managed), preserve existing Secrets (user-managed), delete ExternalSecret CRs in cc-deck/internal/env/k8s_deploy.go

**Checkpoint**: Credentials volume-mounted at /run/secrets/cc-deck/ via inline, existing, or ESO sources

---

## Phase 5: User Story 3 - Network Egress Filtering (Priority: P2)

**Goal**: Generate deny-all egress NetworkPolicy with allowlisted domains, consistent UX across environment types

**Independent Test**: Create environment, verify NetworkPolicy exists with resolved IP rules for AI backend domains

### Tests for User Story 3

- [ ] T024 [P] [US3] Unit tests for NetworkPolicy generation from resolved domains in cc-deck/internal/env/k8s_resources_test.go

### Implementation for User Story 3

- [ ] T025 [US3] Implement NetworkPolicy generation: deny-all egress base, DNS always allowed, domain-to-IP resolution via net.LookupHost(), egress rules per resolved IP on port 443 in cc-deck/internal/env/k8s_resources.go
- [ ] T026 [US3] Integrate domain resolution: use network.Resolver to expand --allow-domain and --allow-group flags (same as compose pattern), pass resolved domains to resource generation in cc-deck/internal/env/k8s_deploy.go
- [ ] T027 [US3] Wire --no-network-policy flag to skip NetworkPolicy creation in Create method in cc-deck/internal/env/k8s_deploy.go
- [ ] T028 [US3] Update Delete method to clean up NetworkPolicy resource in cc-deck/internal/env/k8s_deploy.go

**Checkpoint**: NetworkPolicy created with correct egress rules; --no-network-policy skips creation; flags match compose UX

---

## Phase 6: User Story 4 - MCP Sidecar Containers (Priority: P2)

**Goal**: Generate sidecar containers from build manifest MCP entries sharing Pod network namespace

**Independent Test**: Create environment from build directory with MCP definitions, verify Pod spec contains sidecar containers with correct image/ports

### Tests for User Story 4

- [ ] T029 [P] [US4] Unit tests for MCP sidecar container generation from manifest MCPEntry in cc-deck/internal/env/k8s_resources_test.go

### Implementation for User Story 4

- [ ] T030 [US4] Add optional Image field to MCPEntry struct in cc-deck/internal/build/manifest.go
- [ ] T031 [US4] Implement MCP sidecar generation: for each MCPEntry with Image, generate corev1.Container with name, image, ports, and env var references from Secret in cc-deck/internal/env/k8s_resources.go
- [ ] T032 [US4] Integrate build manifest loading into Create: when --build-dir specified, load cc-deck-image.yaml via build.LoadManifest(), extract MCP entries, pass to resource generation in cc-deck/internal/env/k8s_deploy.go
- [ ] T033 [US4] Handle MCP credential injection: add MCP env var names to the credential Secret, set container env to reference Secret keys in cc-deck/internal/env/k8s_credentials.go

**Checkpoint**: MCP sidecars appear in Pod spec, share network namespace, credentials mounted

---

## Phase 7: User Story 5 - OpenShift Compatibility (Priority: P2)

**Goal**: Auto-detect OpenShift and generate Route and EgressFirewall resources

**Independent Test**: Mock API discovery to report OpenShift capabilities, verify generated resources include Route and EgressFirewall

### Tests for User Story 5

- [ ] T034 [P] [US5] Unit tests for OpenShift detection and resource generation (Route, EgressFirewall) with mock discovery client in cc-deck/internal/env/k8s_openshift_test.go

### Implementation for User Story 5

- [ ] T035 [US5] Implement OpenShift API detection: check for route.openshift.io/v1 and k8s.ovn.org/v1 via discovery client in cc-deck/internal/env/k8s_openshift.go
- [ ] T036 [US5] Implement Route generation for web access port targeting headless Service in cc-deck/internal/env/k8s_openshift.go
- [ ] T037 [US5] Implement EgressFirewall generation with rules consistent with NetworkPolicy egress in cc-deck/internal/env/k8s_openshift.go
- [ ] T038 [US5] Integrate OpenShift resources into Create: detect platform, generate extra resources, apply; integrate into Delete: clean up OpenShift resources in cc-deck/internal/env/k8s_deploy.go

**Checkpoint**: OpenShift Route and EgressFirewall auto-generated when platform detected

---

## Phase 8: User Story 6 - File Synchronization (Priority: P2)

**Goal**: Push/pull files via tar-over-exec and harvest git commits via ext::kubectl exec

**Independent Test**: Push local directory into environment, verify files appear at remote path; pull back, verify content matches

### Tests for User Story 6

- [ ] T039 [P] [US6] Unit tests for sync option parsing and command construction in cc-deck/internal/env/k8s_sync_test.go

### Implementation for User Story 6

- [ ] T040 [US6] Implement Push method: tar local files, pipe via kubectl exec into Pod, extract at /workspace in cc-deck/internal/env/k8s_sync.go
- [ ] T041 [US6] Implement Pull method: tar remote files via kubectl exec, extract locally in cc-deck/internal/env/k8s_sync.go
- [ ] T042 [US6] Implement Harvest method: configure ext::kubectl exec remote helper, git fetch from Pod, create local branch, optional --pr via gh in cc-deck/internal/env/k8s_sync.go
- [ ] T043 [US6] Implement Exec method: run arbitrary command via kubectl exec in the Pod in cc-deck/internal/env/k8s_deploy.go
- [ ] T044 [US6] Wire Push/Pull/Harvest/Exec into K8sDeployEnvironment (replace ErrNotSupported stubs) in cc-deck/internal/env/k8s_deploy.go

**Checkpoint**: Files can be pushed/pulled, git commits harvested from remote environment

---

## Phase 9: User Story 7 - Integration Tests with kind (Priority: P2)

**Goal**: Integration tests covering full lifecycle on kind cluster, runnable locally and in CI

**Independent Test**: Run `go test -tags integration ./internal/integration/` against kind cluster, all tests pass under 5 minutes

### Implementation for User Story 7

- [ ] T045 [US7] Create integration test file with //go:build integration tag covering create, start, stop, delete lifecycle in cc-deck/internal/integration/k8s_deploy_test.go
- [ ] T046 [US7] Add resource verification tests: after create, verify StatefulSet, Service, ConfigMap, PVC, NetworkPolicy exist with correct labels in cc-deck/internal/integration/k8s_deploy_test.go
- [ ] T047 [US7] Add duplicate conflict test: verify second create with same name returns error without modifying existing resources in cc-deck/internal/integration/k8s_deploy_test.go
- [ ] T048 [US7] Update CI workflow: extend .github/workflows/integration.yaml with k8s-deploy integration test job using helm/kind-action@v1, stub image loading, test namespace creation

**Checkpoint**: Integration tests pass locally against kind and in GitHub Actions CI

---

## Phase 10: User Story 8 - Environment Status and Listing (Priority: P3)

**Goal**: Show k8s-deploy environments in unified list with detailed status from K8s API

**Independent Test**: Create k8s-deploy environment, run cc-deck env list, verify it appears with correct columns

### Tests for User Story 8

- [ ] T049 [P] [US8] Unit tests for Status reconciliation logic in cc-deck/internal/env/k8s_deploy_test.go

### Implementation for User Story 8

- [ ] T050 [US8] Implement Status method: query K8s API for StatefulSet/Pod status, reconcile with stored state, return EnvironmentStatus with Pod readiness and PVC info in cc-deck/internal/env/k8s_deploy.go
- [ ] T051 [US8] Implement ReconcileK8sDeployEnvs function: batch reconciliation of all k8s-deploy instances against K8s API in cc-deck/internal/env/k8s_deploy.go
- [ ] T052 [US8] Wire reconciliation into env list command: call ReconcileK8sDeployEnvs alongside existing reconciliation functions in cc-deck/internal/cmd/env.go

**Checkpoint**: k8s-deploy environments appear in unified list with accurate state from K8s API

---

## Phase 11: Polish and Cross-Cutting Concerns

**Purpose**: Documentation, cleanup, and validation

- [ ] T053 [P] Update README.md with k8s-deploy feature description, usage examples, and Feature Specifications table entry
- [ ] T054 [P] Update CLI reference (docs/modules/reference/pages/cli.adoc) with all new commands, flags, and usage examples for k8s-deploy
- [ ] T055 [P] Create user guide page (docs/modules/running/pages/k8s-deploy.adoc) covering overview, quick start, credentials, network filtering, MCP sidecars, and OpenShift
- [ ] T056 [P] Add landing page feature card for k8s-deploy to cc-deck.github.io features section (per Constitution IX)
- [ ] T057 Run quickstart.md validation: verify all example commands from specs/028-k8s-deploy/quickstart.md work end-to-end
- [ ] T058 Run make lint and make test to verify no regressions

---

## Dependencies and Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies, can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion, BLOCKS all user stories
- **US1 (Phase 3)**: Depends on Foundational. Core lifecycle, MVP.
- **US2 (Phase 4)**: Depends on US1 (integrates into Create/Delete methods)
- **US3 (Phase 5)**: Depends on US1 (integrates into Create/Delete)
- **US4 (Phase 6)**: Depends on US1 + US2 (needs resource generation + credential handling)
- **US5 (Phase 7)**: Depends on US1 + US3 (needs resource generation + network policy)
- **US6 (Phase 8)**: Depends on US1 (needs running environment to sync into)
- **US7 (Phase 9)**: Depends on US1 + US2 (tests lifecycle with credentials)
- **US8 (Phase 10)**: Depends on US1 (needs environments to list/status)
- **Polish (Phase 11)**: Depends on all desired user stories being complete

### User Story Dependencies

```
Phase 1: Setup
    │
Phase 2: Foundational
    │
Phase 3: US1 (Core Lifecycle) ── MVP CHECKPOINT
    │
    ├── Phase 4: US2 (Credentials)
    │       │
    │       ├── Phase 6: US4 (MCP Sidecars)
    │       │
    │       └── Phase 9: US7 (Integration Tests)
    │
    ├── Phase 5: US3 (Network Filtering)
    │       │
    │       └── Phase 7: US5 (OpenShift)
    │
    ├── Phase 8: US6 (File Sync)
    │
    └── Phase 10: US8 (Status/Listing)
            │
    Phase 11: Polish
```

### Parallel Opportunities

Within each phase, tasks marked [P] can run in parallel:
- **Phase 2**: T005, T006, T007 are independent files
- **Phase 3**: T009 + T010 (tests), T011 + T015 (different methods)
- **Phase 4-8**: Test tasks can run parallel to each other
- **Phase 11**: T053, T054, T055 (different doc files)

After Foundational, independent stories can run in parallel:
- US3 (Network) and US6 (File Sync) have no mutual dependency
- US8 (Status) has no dependency on US2-US7

---

## Parallel Example: User Story 1

```bash
# Launch tests in parallel:
Task: "Unit tests for K8s resource generation in k8s_resources_test.go"
Task: "Unit tests for lifecycle methods in k8s_deploy_test.go"

# Launch independent implementations in parallel:
Task: "Implement resource generation in k8s_resources.go"
Task: "Implement Start/Stop methods in k8s_deploy.go"
```

---

## Implementation Strategy

### MVP First (User Stories 1 + 2)

1. Complete Phase 1: Setup (add dependencies)
2. Complete Phase 2: Foundational (CLI flags, client helper)
3. Complete Phase 3: US1 Core Lifecycle (create/attach/stop/start/delete)
4. Complete Phase 4: US2 Credentials (inline + existing Secret)
5. **STOP and VALIDATE**: Test full lifecycle with credentials on kind cluster
6. This delivers: persistent K8s development environments with credential management

### Incremental Delivery

1. MVP (US1 + US2) delivers core value
2. Add US3 (Network) + US5 (OpenShift) for security and platform support
3. Add US4 (MCP) for enhanced AI agent capabilities
4. Add US6 (File Sync) for developer workflow
5. Add US7 (Integration Tests) for CI confidence
6. Add US8 (Status) for observability
7. Polish: Documentation across all features

### Single Developer Strategy

Follow phases sequentially in priority order (P1 first, then P2, then P3). Within each phase, implement tasks in order. Commit after each logical group.
