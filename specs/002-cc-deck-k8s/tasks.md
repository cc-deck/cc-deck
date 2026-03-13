# Tasks: cc-deck (Kubernetes CLI)

**Input**: Design documents from `/specs/002-cc-deck-k8s/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md, contracts/

**Tests**: Not explicitly requested in spec. Test tasks omitted.

**Organization**: Tasks grouped by user story to enable independent implementation and testing.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Go project initialization and dependency setup

- [ ] T001 (cc-mux-35y.1) Initialize Go module with `go mod init` and add dependencies (cobra, viper, client-go, adrg/xdg, yaml.v3) in cc-deck/go.mod
- [ ] T002 (cc-mux-35y.2) [P] Create CLI entry point with root cobra command, global flags (--kubeconfig, --namespace, --profile, --config, --verbose, --output) in cc-deck/cmd/cc-deck/main.go
- [ ] T003 (cc-mux-35y.3) [P] Create .gitignore for Go project (cc-deck/cc-deck binary, vendor/) in cc-deck/.gitignore
- [ ] T004 (cc-mux-35y.4) Verify build pipeline: `cd cc-deck && go build ./cmd/cc-deck` produces working binary

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core infrastructure that MUST be complete before any user story

**CRITICAL**: No user story work can begin until this phase is complete

- [ ] T005 (cc-mux-9o2.1) Implement Config struct with YAML serialization, XDG path resolution, load/save operations in cc-deck/internal/config/config.go
- [ ] T006 (cc-mux-9o2.2) [P] Implement Profile struct with Anthropic and Vertex AI backend types, validation, CRUD operations in cc-deck/internal/config/profile.go
- [ ] T007 (cc-mux-9o2.3) [P] Implement K8s client creation with kubeconfig loading (flag, env var, default path fallback), namespace resolution in cc-deck/internal/k8s/client.go
- [ ] T008 (cc-mux-9o2.4) [P] Implement OpenShift detection via discovery API (check for route.openshift.io/v1 and k8s.ovn.org/v1) in cc-deck/internal/k8s/discovery.go
- [ ] T009 (cc-mux-9o2.5) Implement K8s resource builders: StatefulSet, headless Service, PVC volumeClaimTemplate with labels and configurable storage size in cc-deck/internal/k8s/resources.go
- [ ] T010 (cc-mux-9o2.6) [P] Implement Server-Side Apply helpers for creating/updating K8s resources idempotently in cc-deck/internal/k8s/apply.go

**Checkpoint**: Foundation ready. Config loads, K8s client connects, resources can be built.

---

## Phase 3: User Story 1 - Deploy a Claude Code Session (Priority: P1) MVP

**Goal**: `cc-deck deploy <name>` creates a StatefulSet with Claude Code + Zellij on the cluster

**Independent Test**: Run `cc-deck deploy test-session`, verify Pod is Running with Claude Code accessible

### Implementation for User Story 1

- [ ] T011 (cc-mux-qf5.1) [US1] Implement deploy workflow: validate inputs, load profile, build resources, apply to cluster, wait for Pod Running, update local config in cc-deck/internal/session/deploy.go
- [ ] T012 (cc-mux-qf5.2) [US1] Implement credential mounting logic: Anthropic API key as env var from Secret, Vertex AI SA key mount + env vars in cc-deck/internal/session/deploy.go
- [ ] T013 (cc-mux-qf5.3) [US1] Implement ConfigMap generation for Zellij config.kdl (web server enabled, 0.0.0.0 binding) in cc-deck/internal/k8s/resources.go
- [ ] T014 (cc-mux-qf5.4) [US1] Implement Pod readiness wait with event streaming for scheduling failures (Pending > 60s shows events) in cc-deck/internal/session/deploy.go
- [ ] T015 (cc-mux-qf5.5) [US1] Implement session tracking: add deployed session to local config file with name, namespace, profile, pod_name, timestamp in cc-deck/internal/config/config.go
- [ ] T016 (cc-mux-qf5.6) [US1] Implement cobra `deploy` command with flags (--profile, --storage, --image, --sync-dir, --allow-egress, --no-network-policy) in cc-deck/internal/cmd/deploy.go
- [ ] T017 (cc-mux-qf5.7) [US1] Handle duplicate session name error: check if StatefulSet already exists before creating in cc-deck/internal/session/deploy.go

**Checkpoint**: Can deploy a Claude Code session to K8s. MVP is functional.

---

## Phase 4: User Story 2 - Connect to a Session (Priority: P1)

**Goal**: `cc-deck connect <name>` attaches to a running Zellij session in the Pod

**Independent Test**: Deploy a session, run `cc-deck connect myproject`, verify interactive terminal

### Implementation for User Story 2

- [ ] T018 (cc-mux-498.1) [US2] Implement exec connection method: kubectl exec into Pod + zellij attach, with interactive TTY passthrough in cc-deck/internal/session/connect.go
- [ ] T019 (cc-mux-498.2) [US2] Implement web connection method: port-forward Pod port 8082, open browser with Zellij web URL in cc-deck/internal/session/connect.go
- [ ] T020 (cc-mux-498.3) [US2] Implement Route/Ingress URL discovery for direct web access (OpenShift Route, K8s Ingress) in cc-deck/internal/session/connect.go
- [ ] T021 (cc-mux-498.4) [US2] Implement auto-detection of connection method: Route available -> web URL, otherwise -> exec in cc-deck/internal/session/connect.go
- [ ] T022 (cc-mux-498.5) [US2] Update session config with connection details after first connect in cc-deck/internal/config/config.go
- [ ] T023 (cc-mux-498.6) [US2] Implement cobra `connect` command with flags (--method, --web, --port) in cc-deck/internal/cmd/connect.go

**Checkpoint**: Can deploy and connect to sessions. Core workflow complete.

---

## Phase 5: User Story 3 - Manage Credential Profiles (Priority: P1)

**Goal**: Create, list, switch between Anthropic and Vertex AI credential profiles

**Independent Test**: Add two profiles (anthropic + vertex), deploy with each, verify correct backend

### Implementation for User Story 3

- [ ] T024 (cc-mux-dr9.1) [US3] Implement interactive profile creation flow: prompt for backend type, credential details, model, save to config in cc-deck/internal/config/profile.go
- [ ] T025 (cc-mux-dr9.2) [US3] Implement profile validation: check referenced K8s Secrets exist on cluster, report missing with creation instructions in cc-deck/internal/config/profile.go
- [ ] T026 (cc-mux-dr9.3) [US3] Implement cobra `profile add`, `profile list`, `profile use`, `profile show` subcommands in cc-deck/internal/cmd/profile.go

**Checkpoint**: Full credential profile management. Can switch between Anthropic and Vertex AI.

---

## Phase 6: User Story 4 - Secure Egress with Network Policies (Priority: P2)

**Goal**: Default-deny egress NetworkPolicy with backend-aware allowlisting

**Independent Test**: Deploy a session, curl an unapproved site from Pod, verify blocked

### Implementation for User Story 4

- [ ] T027 (cc-mux-qjg.1) [US4] Implement NetworkPolicy builder: default-deny egress, DNS exception (scoped to kube-dns), backend-specific CIDR allowlist in cc-deck/internal/k8s/network.go
- [ ] T028 (cc-mux-qjg.2) [US4] Implement EgressFirewall builder for OpenShift: FQDN-based allowlist for AI backend and user hosts in cc-deck/internal/k8s/network.go
- [ ] T029 (cc-mux-qjg.3) [US4] Implement backend-specific egress rules: api.anthropic.com for Anthropic, *.googleapis.com for Vertex, plus user --allow-egress hosts in cc-deck/internal/k8s/network.go
- [ ] T030 (cc-mux-qjg.4) [US4] Integrate NetworkPolicy and EgressFirewall creation into deploy workflow (skip if --no-network-policy) in cc-deck/internal/session/deploy.go

**Checkpoint**: Deployed sessions have locked-down egress by default.

---

## Phase 7: User Story 5 - Git Repository Sync (Priority: P2)

**Goal**: Bidirectional file sync between local directory and Pod PVC

**Independent Test**: Sync a local repo to Pod, make changes, sync back, verify files

### Implementation for User Story 5

- [ ] T031 (cc-mux-ga2.1) [US5] Implement push sync: tar local directory, pipe to Pod via kubectl exec tar extract into /workspace in cc-deck/internal/sync/sync.go
- [ ] T032 (cc-mux-ga2.2) [US5] Implement pull sync: tar from Pod /workspace, pipe to local directory via kubectl exec in cc-deck/internal/sync/sync.go
- [ ] T033 (cc-mux-ga2.3) [US5] Implement exclude patterns for sync (--exclude flag, default: .git, node_modules, target, __pycache__) in cc-deck/internal/sync/sync.go
- [ ] T034 (cc-mux-ga2.4) [US5] Implement git credential mounting: SSH key Secret or token Secret mounted into Pod for git push/pull in cc-deck/internal/k8s/resources.go
- [ ] T035 (cc-mux-ga2.5) [US5] Implement cobra `sync` command with flags (--pull, --dir, --exclude) in cc-deck/internal/cmd/sync.go
- [ ] T036 (cc-mux-ga2.6) [US5] Integrate initial sync into deploy workflow when --sync-dir is provided in cc-deck/internal/session/deploy.go

**Checkpoint**: Can sync code in and out of sessions. Git push/pull works from within the Pod.

---

## Phase 8: User Story 6 - Session Lifecycle Management (Priority: P3)

**Goal**: List, delete, and view logs for sessions

**Independent Test**: Deploy a session, list it, view logs, delete it, verify cleanup

### Implementation for User Story 6

- [ ] T037 (cc-mux-o8v.1) [US6] Implement list workflow: read local config, reconcile with live cluster state (check Pod status), display table in cc-deck/internal/session/list.go
- [ ] T038 (cc-mux-o8v.2) [US6] Implement delete workflow: delete StatefulSet, Service, PVC, NetworkPolicy, EgressFirewall, Route/Ingress, remove from local config in cc-deck/internal/session/delete.go
- [ ] T039 (cc-mux-o8v.3) [US6] Implement logs workflow: stream Pod logs to terminal via client-go in cc-deck/internal/session/logs.go
- [ ] T040 (cc-mux-o8v.4) [US6] Implement stale session reconciliation: mark sessions as "deleted" if resources no longer exist on cluster in cc-deck/internal/session/list.go
- [ ] T041 (cc-mux-o8v.5) [US6] Implement cobra `list`, `delete`, `logs` commands in cc-deck/internal/cmd/list.go, cc-deck/internal/cmd/delete.go, cc-deck/internal/cmd/logs.go
- [ ] T042 (cc-mux-o8v.6) [US6] Implement cobra `version` command in cc-deck/internal/cmd/version.go

**Checkpoint**: Full session lifecycle management. Can list, inspect, and clean up sessions.

---

## Phase 9: Polish & Cross-Cutting Concerns

**Purpose**: Error handling, UX refinements, documentation

- [ ] T043 (cc-mux-3gg.1) [P] Implement structured error messages with troubleshooting guidance for common failures (cluster unreachable, Secret missing, PVC quota, image pull) in cc-deck/internal/k8s/errors.go
- [ ] T044 (cc-mux-3gg.2) [P] Implement JSON and YAML output formatting for `list` and `profile list` commands (--output flag) in cc-deck/internal/cmd/list.go and cc-deck/internal/cmd/profile.go
- [ ] T045 (cc-mux-3gg.3) [P] Add shell completion support (bash, zsh, fish) via cobra's built-in completion generator in cc-deck/cmd/cc-deck/main.go
- [ ] T046 (cc-mux-3gg.4) [P] Implement user-provided kustomize overlay support: accept --overlay flag pointing to a kustomize directory, merge with generated resources before apply in cc-deck/internal/k8s/apply.go
- [ ] T047 (cc-mux-3gg.5) Validate end-to-end workflow: deploy, connect, sync, list, delete on a real cluster
- [ ] T048 (cc-mux-3gg.6) Run quickstart.md validation: full setup-to-usage flow

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies, start immediately
- **Foundational (Phase 2)**: Depends on Setup. BLOCKS all user stories.
- **US1 (Phase 3)**: Depends on Phase 2. This is the MVP.
- **US2 (Phase 4)**: Depends on Phase 2. Requires deploy from US1 for testing but is independently implementable.
- **US3 (Phase 5)**: Depends on Phase 2. Profile management is standalone but integrates with deploy.
- **US4 (Phase 6)**: Depends on Phase 2. Enhances deploy with security.
- **US5 (Phase 7)**: Depends on Phase 2. Enhances deploy with file sync.
- **US6 (Phase 8)**: Depends on Phase 2. Lifecycle management for deployed sessions.
- **Polish (Phase 9)**: Depends on US1-US6 being complete.

### User Story Dependencies

- **US1 (P1)**: After Phase 2. No dependencies on other stories. **This is the MVP.**
- **US2 (P1)**: After Phase 2. Integrates with deployed sessions from US1 but independently testable.
- **US3 (P1)**: After Phase 2. Profiles used by deploy but CRUD is standalone.
- **US4 (P2)**: After Phase 2. Integrates into deploy workflow.
- **US5 (P2)**: After Phase 2. Integrates into deploy workflow.
- **US6 (P3)**: After Phase 2. Operates on deployed sessions.

### Parallel Opportunities

- T002 + T003 (CLI entry + gitignore)
- T005 + T006 + T007 + T008 + T010 (config, profile, client, discovery, apply)
- US3, US4, US5 can proceed in parallel (different concerns)
- All T043-T046 (polish tasks touch different files)

---

## Implementation Strategy

### MVP First (User Story 1 + 2 + 3)

1. Complete Phase 1: Setup (T001-T004)
2. Complete Phase 2: Foundational (T005-T010)
3. Complete Phase 3: User Story 1 - Deploy (T011-T017)
4. Complete Phase 4: User Story 2 - Connect (T018-T023)
5. Complete Phase 5: User Story 3 - Profiles (T024-T026)
6. **STOP and VALIDATE**: Can deploy, connect, and manage profiles

### Incremental Delivery

1. Setup + Foundational -> CLI scaffold connects to cluster
2. Add US1 -> Deploy sessions (MVP!)
3. Add US2 -> Connect to sessions
4. Add US3 -> Profile management
5. Add US4 -> Egress security
6. Add US5 -> Git sync
7. Add US6 -> Lifecycle management
8. Each story adds value without breaking previous stories

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story is independently completable and testable
- Commit after each task or logical group
- Stop at any checkpoint to validate story independently

