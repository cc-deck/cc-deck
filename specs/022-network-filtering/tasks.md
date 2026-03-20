# Tasks: Network Security and Domain Filtering

**Input**: Design documents from `/specs/022-network-filtering/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/cli.md

**Tests**: Tests included where they validate core logic (domain expansion, dedup, cycle detection). Not included for CLI commands or generated output formatting.

**Organization**: Tasks grouped by user story to enable independent implementation and testing.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2)
- Include exact file paths in descriptions

## Phase 1: Setup

**Purpose**: Create new packages and shared types

- [x] T001 Create `cc-deck/internal/network/` package directory
- [x] T002 [P] Create `cc-deck/internal/compose/` package directory
- [x] T003 [P] Create `cc-deck/testdata/domains/` test fixtures directory

---

## Phase 2: Foundational (Domain Group System)

**Purpose**: Core domain expansion engine that ALL user stories depend on

**CRITICAL**: No user story work can begin until this phase is complete

- [x] T004 Define built-in domain groups in `cc-deck/internal/network/builtin.go` (python, nodejs, rust, golang, github, gitlab, docker, quay, anthropic, vertexai)
- [x] T005 Implement domain config loading from `~/.config/cc-deck/domains.yaml` in `cc-deck/internal/network/config.go` (use adrg/xdg, gopkg.in/yaml.v3, non-fatal if missing)
- [x] T006 Implement core expansion logic in `cc-deck/internal/network/domains.go`: `ExpandGroup()`, `ExpandAll()`, `WildcardDedup()` with recursive includes, cycle detection, and group-vs-domain disambiguation (no dot = group, has dot = domain)
- [x] T007 Write unit tests for domain expansion in `cc-deck/internal/network/domains_test.go`: built-in expansion, user override, user extension (extends: builtin), recursive includes, cycle detection error, wildcard dedup, unknown group error, mixed groups and literal domains
- [x] T008 Create test fixtures in `cc-deck/testdata/domains/user-override.yaml` and `cc-deck/testdata/domains/user-extend.yaml` and `cc-deck/testdata/domains/circular.yaml`

**Checkpoint**: `go test ./internal/network/...` passes with all expansion, dedup, and error cases covered

---

## Phase 3: User Story 1 - Deploy with Default Network Filtering (Priority: P1) MVP

**Goal**: `cc-deck deploy --compose` generates compose.yaml with proxy sidecar and internal network when manifest has `network` section

**Independent Test**: Deploy a session, verify allowed domains succeed and blocked domains fail

### Implementation for User Story 1

- [x] T009 [US1] Add `NetworkConfig` struct to `cc-deck/internal/build/manifest.go` with `AllowedDomains []string` field, add `Network *NetworkConfig` to Manifest struct
- [x] T010 [US1] Write unit test for manifest loading with network section in `cc-deck/internal/build/manifest_test.go`
- [x] T011 [US1] Implement tinyproxy config generation in `cc-deck/internal/compose/proxy.go`: generate `tinyproxy.conf` (FilterDefaultDeny, fnmatch) and `whitelist` file from expanded domain list, handle wildcard conversion (`.example.com` to `*.example.com`)
- [x] T012 [US1] Write unit tests for proxy config generation in `cc-deck/internal/compose/proxy_test.go`
- [x] T013 [US1] Implement compose.yaml generation in `cc-deck/internal/compose/generate.go`: session container on internal network, proxy container on internal + default, HTTP_PROXY/HTTPS_PROXY env vars, volume mounts for proxy config and logs, .env.example generation
- [x] T014 [US1] Write unit tests for compose generation in `cc-deck/internal/compose/generate_test.go` (verify YAML structure, network topology, env vars)
- [x] T015 [US1] Add `--compose` flag to deploy command in `cc-deck/internal/cmd/deploy.go`: read manifest, expand domain groups, call compose generator, write output files
- [x] T016 [US1] Register `--allowed-domains` string flag on deploy command in `cc-deck/internal/cmd/deploy.go` (flag only, pass raw value through to deploy flow; parsing logic deferred to T025 in Phase 7)
- [x] T016b [US1] Inject backend-specific domain group (`anthropic` or `vertexai`) into the expanded domain list automatically based on manifest or profile backend setting, in the compose generation path in `cc-deck/internal/cmd/deploy.go` (FR-002)

**Checkpoint**: `cc-deck deploy --compose <build-dir>` generates valid compose.yaml with proxy sidecar. Backend domains are included automatically without user specification. Manual test with `podman compose up` verifies filtering works.

---

## Phase 4: User Story 2 - Configure Domain Groups in Build Manifest (Priority: P1)

**Goal**: `/cc-deck.extract` auto-detects project ecosystems and populates `network.allowedDomains` in manifest

**Independent Test**: Run extract on project with go.mod, verify `golang` appears in manifest

### Implementation for User Story 2

- [x] T017 [US2] Update `/cc-deck.extract` AI command template in `cc-deck/internal/build/commands/cc-deck.extract.md` to detect ecosystem files (go.mod, pyproject.toml, package.json, Cargo.toml) and add corresponding domain groups to manifest `network.allowed_domains`

**Checkpoint**: `/cc-deck.extract` on a Go+Python project populates `network.allowed_domains: [golang, python]`

---

## Phase 5: User Story 3 - User-Defined Domain Groups (Priority: P2)

**Goal**: Users can define custom groups and extend built-in groups in `~/.config/cc-deck/domains.yaml`

**Independent Test**: Create a domains.yaml with custom group, reference in manifest, verify expanded list is correct

### Implementation for User Story 3

- [x] T018 [US3] Add support for `extends: builtin` merge logic in `cc-deck/internal/network/domains.go` (merge user domains with built-in group of same name)
- [x] T019 [US3] Add support for `includes` cross-group references in `cc-deck/internal/network/domains.go` (recursive expansion with visited-set cycle detection)
- [x] T020 [US3] Write unit tests for extends and includes in `cc-deck/internal/network/domains_test.go`

**Checkpoint**: Domain expansion correctly merges built-in + user config with extends, includes, and cycle detection

Note: Most of this logic is implemented in Phase 2 (T006). These tasks cover the user-facing integration and edge case testing.

---

## Phase 6: User Story 4 - Seed and Explore Domain Definitions (Priority: P2)

**Goal**: `cc-deck domains init/list/show` commands for discoverability

**Independent Test**: Run `cc-deck domains init`, verify file created. Run `cc-deck domains show python`, verify domain list.

### Implementation for User Story 4

- [x] T021 [US4] Create `cc-deck/internal/cmd/domains.go` with `cc-deck domains` parent command and `init` subcommand (seed domains.yaml with commented built-in definitions, preserve existing user modifications)
- [x] T022 [P] [US4] Add `list` subcommand to `cc-deck/internal/cmd/domains.go` (show all groups with source: builtin, user, extended)
- [x] T023 [P] [US4] Add `show <group>` subcommand to `cc-deck/internal/cmd/domains.go` (display expanded domains with source annotation per entry)
- [x] T024 [US4] Register `domains` command in `cc-deck/cmd/cc-deck/main.go`

**Checkpoint**: `cc-deck domains list` shows built-in groups, `cc-deck domains show python` shows domain list

---

## Phase 7: User Story 5 - Deploy-Time Domain Overrides (Priority: P2)

**Goal**: `--allowed-domains +group` / `-group` / `group,group` syntax at deploy time

**Independent Test**: Deploy with `--allowed-domains +rust`, verify rust domains in proxy config alongside manifest defaults

### Implementation for User Story 5

- [x] T025 [US5] Implement `--allowed-domains` parsing logic in `cc-deck/internal/network/override.go`: parse `+` (add to manifest defaults), `-` (remove from defaults), bare (replace entirely), `all` (disable with warning to stderr). This implements the parsing for the flag registered in T016.
- [x] T026 [US5] Write unit tests for override parsing in `cc-deck/internal/network/override_test.go`
- [x] T027 [US5] Wire override parsing into deploy command flow in `cc-deck/internal/cmd/deploy.go` (call override parser from T025, apply after loading manifest network config, before domain expansion)

**Checkpoint**: Deploy with `--allowed-domains +rust,-python` correctly adds rust and removes python from manifest defaults

---

## Phase 8: User Story 6 - Kubernetes NetworkPolicy Generation (Priority: P3)

**Goal**: Refactor K8s NetworkPolicy to use domain group expansion instead of hardcoded backend hosts

**Independent Test**: Generate K8s manifests with domain groups, verify NetworkPolicy egress rules match expanded domains

### Implementation for User Story 6

- [x] T028 [US6] Refactor `backendEgressRules()` in `cc-deck/internal/k8s/network.go` to accept expanded domain list from domain group system instead of using hardcoded backend switch
- [x] T029 [US6] Refactor `backendDNSNames()` in `cc-deck/internal/k8s/network.go` to use domain group expansion (load `anthropic` or `vertexai` built-in group)
- [x] T030 [US6] Update `BuildNetworkPolicy()` call sites in `cc-deck/internal/session/deploy.go` to expand domain groups before passing AllowedEgress
- [x] T031 [US6] Update existing network tests in `cc-deck/internal/k8s/network_test.go` to verify domain group integration (preserve existing test behavior)

**Checkpoint**: Existing `go test ./internal/k8s/...` passes with no functional change. Domain groups now feed the existing builders.

---

## Phase 9: User Story 7 - OpenShift EgressFirewall Generation (Priority: P3)

**Goal**: Refactor EgressFirewall to use domain group expansion

**Independent Test**: Generate OpenShift manifests with domain groups, verify EgressFirewall FQDN rules

### Implementation for User Story 7

- [x] T032 [US7] Update `BuildEgressFirewall()` call sites in `cc-deck/internal/session/deploy.go` to expand domain groups before passing AllowedEgress
- [x] T033 [US7] Update EgressFirewall tests in `cc-deck/internal/k8s/network_test.go` to verify domain group integration

**Checkpoint**: Existing EgressFirewall tests pass. FQDN rules now come from domain group expansion.

---

## Phase 10: User Story 8 - Audit Blocked Requests (Priority: P3)

**Goal**: `cc-deck domains blocked <session>` shows denied requests from proxy logs

**Independent Test**: Make request to blocked domain, run `cc-deck domains blocked`, verify domain appears

### Implementation for User Story 8

- [x] T034 [US8] Add `blocked <session>` subcommand to `cc-deck/internal/cmd/domains.go` (locate proxy container via compose project name, parse tinyproxy access log for denied entries, display with timestamps)
- [x] T035 [US8] Add `add <session> <domain>` and `remove <session> <domain>` subcommands to `cc-deck/internal/cmd/domains.go` (regenerate proxy whitelist, restart proxy container via `podman compose restart proxy`)

**Checkpoint**: `cc-deck domains blocked my-session` displays blocked requests. `cc-deck domains add` reconfigures proxy without session restart.

---

## Phase 11: Polish and Cross-Cutting Concerns

**Purpose**: Flag migration, documentation, backward compatibility

- [x] T036 Add `--allow-egress` as deprecated alias for `--allowed-domains` in `cc-deck/internal/cmd/deploy.go` (preserve backward compatibility during transition)
- [ ] T037 Run quickstart.md validation (manual end-to-end test of the documented workflow)

---

## Dependencies and Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies, start immediately
- **Foundational (Phase 2)**: Depends on Phase 1. BLOCKS all user stories.
- **US1 (Phase 3)**: Depends on Phase 2. MVP target.
- **US2 (Phase 4)**: Depends on Phase 2. Can run in parallel with US1.
- **US3 (Phase 5)**: Depends on Phase 2. Most logic already in Phase 2.
- **US4 (Phase 6)**: Depends on Phase 2. Can run in parallel with US1.
- **US5 (Phase 7)**: Depends on Phase 3 (deploy command exists).
- **US6 (Phase 8)**: Depends on Phase 2. Can run in parallel with US1.
- **US7 (Phase 9)**: Depends on Phase 8 (shared refactor).
- **US8 (Phase 10)**: Depends on Phase 3 (compose generation exists).
- **Polish (Phase 11)**: Depends on all desired stories complete.

### User Story Dependencies

- **US1 (P1)**: Independent after Foundational
- **US2 (P1)**: Independent after Foundational
- **US3 (P2)**: Independent after Foundational (extends Phase 2 logic)
- **US4 (P2)**: Independent after Foundational
- **US5 (P2)**: Depends on US1 (deploy command with --compose)
- **US6 (P3)**: Independent after Foundational
- **US7 (P3)**: Depends on US6 (shared refactor pattern)
- **US8 (P3)**: Depends on US1 (compose/proxy infrastructure)

### Parallel Opportunities

After Phase 2 completes, these can run in parallel:
- US1 (compose generation) + US4 (domains CLI) + US6 (K8s refactor) + US2 (extract)
- US3 extends Phase 2 logic, minimal additional work
- US5 and US8 wait for US1

---

## Parallel Example: After Foundational Phase

```
Agent A: US1 - Compose generation (T009-T016)
Agent B: US4 - Domains CLI commands (T021-T024)
Agent C: US6 - K8s NetworkPolicy refactor (T028-T031)
Agent D: US2 - Extract command update (T017)
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (T001-T003)
2. Complete Phase 2: Foundational domain system (T004-T008)
3. Complete Phase 3: User Story 1 compose generation (T009-T016)
4. **STOP and VALIDATE**: Test with `podman compose up`, verify filtering works
5. Deploy/demo if ready

### Incremental Delivery

1. Setup + Foundational -> Domain expansion works
2. Add US1 -> Compose with proxy sidecar works (MVP!)
3. Add US4 -> Users can explore domain groups
4. Add US2 -> Extract auto-populates domains
5. Add US5 -> Deploy-time overrides
6. Add US6+US7 -> K8s/OpenShift use domain groups
7. Add US8 -> Audit blocked requests
8. Polish -> Backward compat, docs
