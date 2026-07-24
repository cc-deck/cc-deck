# Tasks: Workspace-Centric Zellij Session Sharing

All tasks inherit `plan.md` constraints. Tests precede implementation changes.

## Reusable foundation already complete

- [X] T001 Define lifecycle domain types and atomic, secret-free XDG state storage.
- [X] T002 Implement cross-process lifecycle locking and cancellation.
- [X] T003 Implement the provider contract, registry, Cloudflare Quick Tunnel, readiness, status, and idempotent stop.
- [X] T004 Implement URL and shell escaping plus trusted-control and terminal TLS warnings.
- [X] T005 Implement detached guard identity, readiness, process-group validation, and non-recursive teardown dispatch.
- [X] T006 Document sharing configuration and accepted V1 security limitations.

## Workspace-centric implementation

- [X] T007 Align Feature 084 specification, plan, data model, CLI contract, tasks, and quickstart with workspace-centric sharing.
- [ ] T008 Add `SessionManager`, readiness result types, and state-table tests; implement local canonical-session creation with creation-time web sharing.
- [ ] T009 Extract idempotent canonical-session creation from attach for container, compose, SSH, Kubernetes deploy, and OpenShell backends; reject web sharing before backend commands.
- [ ] T010 Replace fixed role labels with workspace identity and multiple named invitation records; add collision-safe memorable label generation and one-invitation construction.
- [ ] T011 Make sharing start workspace-aware, require the canonical session, add independent invite/revoke operations, revoke every active invitation on stop, and stop when the canonical session dies.
- [ ] T012 Integrate readiness and sharing into `ws new`, `ws start`, `ws attach`, `ws invite`, `ws revoke`, `ws unshare`, and `ws stop`; move the hidden guard beneath `ws`.
- [ ] T013 Add reconciled `INFRA`, `SESSION`, and `SHARING` list/status output and remove the standalone public sharing command.
- [ ] T014 Update README, CLI/configuration references, sharing guide, and focused automated evidence for the workspace flows.

## Acceptance and deferred repository verification

- [ ] T032 Execute and record the live acceptance matrix in `quickstart.md`: monotonic timers; 10 repetitions; provider readiness <=10s and measured RTT <=250ms; every start <=15s and normal disconnect <=5s; exactly 2 interactive + 2 observer clients; every observer input attempted; isolation and old-token rejection on every run; reserved-name table; forced signal and uncatchable guard loss; fresh temporary XDG state shared by every command in one operation.
- [ ] T034 Run and repair repository-wide `make test` and `make verify` as tracked by brainstorm 090. Focused feature suites and `make lint` remain required before handoff and must be recorded in `quickstart.md` without marking this task complete.

## Dependencies

T007 precedes implementation. T008-T009 establish workspace readiness. T010-T011 establish the sharing model and service. T012 consumes both, T013 reconciles output and removes the obsolete CLI, and T014 records the implemented behavior. T032 and T034 remain final independent gates.
