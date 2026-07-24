# Tasks: Zellij Session Sharing

All tasks inherit `plan.md` Global Constraints and Interfaces.

## Phase 1: Foundational Contracts

- [X] T001 Define domain types and state-transition tests in `cc-deck/internal/share/model.go` and `cc-deck/internal/share/model_test.go`
- [X] T002 Define all `plan.md` interfaces and concrete boundary types (`Process`, handles, requests, statuses, operations, invitations) plus fakes in `cc-deck/internal/share/provider.go`, `cc-deck/internal/share/model.go`, and `cc-deck/internal/share/test_fakes_test.go`
- [X] T003 [P] Write atomic permission/state-store tests before implementing the store in `cc-deck/internal/share/state_test.go`
- [X] T004 Implement `Store.Load`, `Store.Save`, and `Store.Remove` with 0700/0600 atomic XDG state in `cc-deck/internal/share/state.go`
- [X] T005 [P] Write contention and cancellation tests before implementing cross-process serialization in `cc-deck/internal/share/lock_test.go`
- [X] T006 Implement `Store.WithLock` in `cc-deck/internal/share/lock.go`
- [X] T007 [P] Add sharing schema/defaults and validation tests in `cc-deck/internal/config/config_test.go` and `cc-deck/internal/config/validate_test.go`
- [X] T008 Implement sharing schema/defaults and provider validation in `cc-deck/internal/config/config.go` and `cc-deck/internal/config/validate.go`

## Phase 2: Runtime Adapters

- [X] T009 [P] Write Zellij 0.44.3 capability, session isolation, web lifecycle, and both-token-role tests in `cc-deck/internal/share/zellij_test.go`
- [X] T010 Implement the `Zellij` adapter in `cc-deck/internal/share/zellij.go`
- [X] T011 [P] Write the provider contract suite and Cloudflare failure/readiness cases in `cc-deck/internal/share/provider_contract_test.go` and `cc-deck/internal/share/cloudflare_test.go`
- [X] T012 Implement Cloudflare validation, encrypted endpoint startup, bounded readiness parsing, status, and idempotent stop in `cc-deck/internal/share/cloudflare.go`

## Phase 3: User Story 1 - Complete Interactive Sharing MVP

**Independent test**: Start one session and obtain all four invitations; two interactive clients can control the complete selected session while other sessions remain unavailable.

- [X] T013 [P] [US1] Write URL/shell escaping, role labeling, secret suppression, and warning tests in `cc-deck/internal/share/invitation_test.go`
- [X] T014 [US1] Implement session-specific interactive and observer browser/terminal invitations in `cc-deck/internal/share/invitation.go`
- [X] T015 [US1] Write start-saga tests covering both credentials before reveal, isolation, one-operation enforcement, readiness, all rollback points, and two interactive clients in `cc-deck/internal/share/service_test.go`
- [X] T016 [US1] Implement `Service.Start` with both role credentials and reverse-order compensation in `cc-deck/internal/share/service.go`
- [X] T017 [US1] Write Cobra tests for safe selection, four-part output, trusted-control warning, experimental TLS warning, and errors in `cc-deck/internal/cmd/share_test.go`
- [X] T018 [US1] Implement and register `cc-deck share start` in `cc-deck/internal/cmd/share.go` and `cc-deck/cmd/cc-deck/main.go`

## Phase 4: User Story 2 - Read-Only Observers

**Independent test**: Two observers reuse the observer invitation and 100% of keyboard, mouse, paste, resize, focus, and terminal-control attempts fail to change shared state.

- [X] T019 [US2] Add the complete observer input-attempt matrix and two-observer acceptance harness in `cc-deck/internal/share/observer_acceptance_test.go`
- [X] T020 [US2] Enforce observer token selection and prohibit generation of interactive observer commands in `cc-deck/internal/share/zellij.go` and `cc-deck/internal/share/invitation.go`

## Phase 5: User Story 3 - Stop, Status, and Recovery

**Independent test**: Normal stop disconnects clients and revokes old invitations; every partial failure reports residuals; forced controller loss triggers an automatic attempt and later reconciliation.

- [X] T021 [US3] Write stop/status tests covering all-step teardown, idempotency, secret suppression, residuals, stale state, forced guard loss, validation-lock release before watch, normal-stop disarm ordering, and absence of recursive lock acquisition in `cc-deck/internal/share/service_test.go` and `cc-deck/internal/share/guard_test.go`
- [X] T022 [US3] Implement `Service.Stop`, `Service.Status`, and stale reconciliation in `cc-deck/internal/share/service.go`
- [X] T023 [US3] Implement detached guard launch, bounded readiness handshake, persisted identity validation under a short released lock, unlocked provider-exit/signal watch, normal disarm ordering, and teardown through the once-locking service in `cc-deck/internal/share/guard.go` and hidden CLI wiring in `cc-deck/internal/cmd/share.go`
- [X] T024 [US3] Write CLI status/stop/degraded output and exit-code tests in `cc-deck/internal/cmd/share_test.go`
- [X] T025 [US3] Implement CLI status/stop/degraded behavior in `cc-deck/internal/cmd/share.go`

## Phase 6: User Story 4 - Provider Selection

**Independent test**: Cloudflare and the isolated fake pass identical start/readiness/status/rollback/idempotent-stop contract cases.

- [X] T026 [US4] Run one shared suite covering start, readiness, status, invitation integration, startup rollback, idempotent stop, and partial failure against Cloudflare-with-fake-runner and isolated fake provider in `cc-deck/internal/share/provider_contract_test.go`
- [X] T027 [US4] Add provider registry, selection, completion, and unknown-provider tests in `cc-deck/internal/share/provider_contract_test.go` and `cc-deck/internal/cmd/share_test.go`
- [X] T028 [US4] Implement provider registry and `--provider` selection in `cc-deck/internal/share/provider.go` and `cc-deck/internal/cmd/share.go`

## Phase 7: Documentation and Measured Acceptance

- [X] T029 [P] Document overview and accepted security risks in `README.md`
- [X] T030 [P] Document CLI/configuration in `docs/modules/reference/pages/cli.adoc` and `docs/modules/reference/pages/configuration.adoc`
- [X] T031 [P] Add the Antora guide/navigation in `docs/modules/using/pages/sharing.adoc` and `docs/modules/using/nav.adoc`
- [ ] T032 Execute and record the acceptance matrix in `specs/084-zellij-session-sharing/quickstart.md`: monotonic timers; 10 repetitions; injected provider readiness <=10s and measured RTT <=250ms; every start <=15s and normal disconnect <=5s; exactly 2 interactive + 2 observer clients; every listed observer input attempted; isolation and old-token rejection on every run; reserved-name table; forced signal and uncatchable guard loss; fresh temporary XDG environment for first-time flow
- [ ] T033 Run prose-profile validation and correct `README.md` and `docs/`
- [ ] T034 Run `make test`, `make lint`, and `make verify` and correct failures

## Dependencies and Parallel Work

T001-T008 establish contracts. T009-T012 may then run in parallel. US1 is the MVP and blocks US2/US3; US4 can proceed after provider contracts. Documentation files are parallel after CLI stabilization. Tests precede each corresponding implementation.
