# Quickstart: Validate Zellij Session Sharing

Run `make test`, `make lint`, and `make verify` from the repository root.

For manual acceptance, use supported Zellij, cloudflared, one running local session, and four client contexts. Start sharing; join twice interactively and twice as observers; verify full-session interaction versus rejected observer input; verify other sessions are unavailable; verify status hides tokens; stop and confirm disconnection and rejected old invitations. Repeat with reserved characters in the session name and forced provider/controller termination to validate reconciliation.

Terminal attachment is experimental in V1 and includes an explicit certificate-validation bypass. Perform it only after acknowledging the documented interception risk.

## Acceptance record: 2026-07-24

The acceptance run used the feature worktree on macOS. Zellij 0.44.3 and cloudflared 2026.7.2 were present. No live public tunnel was opened and no real Zellij session was mutated during this unattended run.

| Requirement | Result | Evidence or blocker |
|---|---|---|
| Deterministic unit and provider-contract cases | Implemented, execution blocked | The test suite contains transactional start/rollback, role-specific invitations, state permissions, lifecycle guard, observer-input matrix, provider contract, and CLI cases. `make test` could not complete because six unrelated existing tests in `internal/ws/openshell_test.go` expect two return values from `selectCredentialMode`, which now returns three. |
| Ten timed starts with readiness <=10 s, RTT <=250 ms, and start <=15 s | Not executed | The required repository test target did not compile. The current deterministic tests also do not supply the specified ten-repetition monotonic timing/RTT harness. |
| Ten normal stops with disconnect <=5 s | Not executed | Requires four real clients or a purpose-built measured acceptance harness; neither was safely available after the repository test target failed. |
| Exactly two interactive and two observer clients | Not executed live | The automated observer harness models two clients sharing one read-only credential, and service tests model invitation reuse. It does not establish four real Zellij clients. |
| Observer keyboard, mouse, paste, resize, tab focus, pane focus, and terminal-control rejection | Implemented, not executed through real clients | `observer_acceptance_test.go` enumerates all seven input classes for two modeled observers. Live browser and terminal input injection was not run. |
| Selected-session isolation with multiple local sessions | Not executed live | Adapter and service tests cover selected-session ownership, but no additional real host sessions were created for this run. |
| Old-token rejection after stop | Not executed live | Service teardown tests verify both token revocations. Reconnection with real expired invitations was not attempted. |
| Reserved session names | Implemented, execution blocked | Invitation tests cover URL escaping and shell quoting. The full test target was blocked before a successful suite result could be recorded. |
| Supported signal and provider-exit guard cleanup | Implemented, execution blocked | Guard tests cover signaling, identity validation, provider exit, lock release before watch, and teardown dispatch. The full test target did not compile. |
| Uncatchable guard loss and later reconciliation | Not executed live | This accepted V1 limitation requires externally killing a real guard, inspecting residual resources, and invoking a later lifecycle command. |
| Fresh temporary XDG first-time flow | Not executed end-to-end | State-store tests use temporary directories, but a real CLI/provider/Zellij flow in a fresh XDG environment was not run. |
| Prose profile | Unavailable | The repository requires `/prose:check` with `.style/voice.yaml`, but exposes no Make target, executable, script, or package command for that plugin in this environment. Documentation was reviewed manually only. |
| `make test` | Blocked | Initial sandbox run could not access the Go build cache. The approved rerun reached the unrelated `internal/ws/openshell_test.go` assignment-mismatch errors and was stopped after the failure was established. |
| `make lint` | Failed | `go vet ./...` reported the same six assignment-mismatch errors in `internal/ws/openshell_test.go`. |
| `make verify` | Blocked | It entered `make test` and reported the same unrelated compile errors; it could not reach a successful lint phase. |

T032, T033, and T034 remain incomplete. A follow-up acceptance run must first restore repository-wide compilation, then execute the ten-repetition measured matrix with real Zellij/browser/terminal clients and a live provider, and run the external prose plugin.
