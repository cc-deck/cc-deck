# Quickstart: Validate Zellij Session Sharing

Run `make test`, `make lint`, and `make verify` from the repository root.

For manual acceptance, use supported Zellij, cloudflared, one local workspace, and four client contexts. Exercise the workspace-centric flow:

```bash
cc-deck ws new demo --share
cc-deck ws invite demo --role interactive --name alice
cc-deck ws invite demo --role observer
cc-deck ws status demo
cc-deck ws revoke demo alice
cc-deck ws unshare demo
```

The equivalent sharing entry points are `cc-deck ws start demo --share` and `cc-deck ws attach demo --share`. Join twice interactively and twice as observers; verify complete-session interaction versus rejected observer input; verify other sessions are unavailable; verify list/status hide tokens; unshare and confirm disconnection and rejected old invitations. Repeat with reserved characters, forced provider/controller termination, and canonical-session death. Confirm that a plain restart after session death is private.

Terminal attachment is experimental in V1 and includes an explicit certificate-validation bypass. Perform it only after acknowledging the documented interception risk.

## Acceptance record: 2026-07-24

Focused workspace-centric evidence:

* `go test ./internal/ws -run 'TestEnsureReady|TestLocalWorkspace' -count=1` passed.
* `go test ./internal/ws -count=1` passed after all backends implemented canonical-session readiness.
* `go test ./internal/share -count=1` passed for named invitations, workspace ownership, revoke/stop, session health, and guard behavior.
* `go test ./internal/cmd -run 'TestWs|TestPromoted|TestRunWsStatus|TestRunWsList' -count=1` and `go test ./cmd/cc-deck -count=1` passed.
* The complete command suite reaches Podman smoke tests that cannot access the sandboxed Podman socket; this does not complete deferred T034.
* Final verification reran `internal/share` and `internal/ws` successfully. The aggregate command again entered the known external-smoke path and was stopped; brainstorm 090 covers this exact default-tier defect.
* `make lint` passed: `go vet ./...` and `cargo clippy -- -D warnings` both exited successfully outside the sandbox.
* `make install` succeeded. Help for `ws new`, `ws start`, `ws attach`, `ws invite`, `ws revoke`, and `ws unshare` matched the approved flags and arguments; the standalone `cc-deck share` command returned unknown-command status.

The acceptance run used the feature worktree on macOS. Zellij 0.44.3 and cloudflared 2026.7.2 were present. No live public tunnel was opened and no real Zellij session was mutated during this unattended run.

| Requirement | Result | Evidence or blocker |
|---|---|---|
| Deterministic unit and provider-contract cases | Focused suites pass | The test suite contains transactional start/rollback, role-specific invitations, state permissions, lifecycle guard, observer-input matrix, provider contract, and CLI cases. The six stale `internal/ws/openshell_test.go` call sites were updated for the current three-value API, and `go test ./internal/ws` passes. |
| Ten timed starts with readiness <=10 s, RTT <=250 ms, and start <=15 s | Not executed | The required repository test target did not compile. The current deterministic tests also do not supply the specified ten-repetition monotonic timing/RTT harness. |
| Ten normal stops with disconnect <=5 s | Not executed | Requires four real clients or a purpose-built measured acceptance harness; neither was safely available after the repository test target failed. |
| Exactly two interactive and two observer clients | Not executed live | The automated observer harness models two clients sharing one read-only credential, and service tests model invitation reuse. It does not establish four real Zellij clients. |
| Observer keyboard, mouse, paste, resize, tab focus, pane focus, and terminal-control rejection | Implemented, not executed through real clients | `observer_acceptance_test.go` enumerates all seven input classes for two modeled observers. Live browser and terminal input injection was not run. |
| Selected-session isolation with multiple local sessions | Not executed live | Adapter and service tests cover selected-session ownership, but no additional real host sessions were created for this run. |
| Old-token rejection after stop | Not executed live | Service teardown tests verify both token revocations. Reconnection with real expired invitations was not attempted. |
| Reserved session names | Focused tests pass | Invitation tests cover URL escaping and shell quoting. |
| Supported signal and provider-exit guard cleanup | Focused tests pass | Guard tests cover signaling, identity validation, provider exit, lock release before watch, and teardown dispatch. |
| Uncatchable guard loss and later reconciliation | Not executed live | This accepted V1 limitation requires externally killing a real guard, inspecting residual resources, and invoking a later lifecycle command. |
| Fresh temporary XDG first-time flow | Not executed end-to-end | State-store tests use temporary directories, but a real CLI/provider/Zellij flow in a fresh XDG environment was not run. |
| Prose validation | Passed (manual) | Reviewed the README sharing section, sharing guide, CLI reference, configuration reference, and navigation for grammar, spelling, terminology consistency, AsciiDoc structure, and agreement with the specification. Corrected an awkward readiness sentence and normalized the phrase "role availability." No local prose checker (`vale`, `typos`, `codespell`, or `asciidoctor`) was available. |
| `make test` | Incomplete | The compile errors were corrected. An approved out-of-sandbox rerun passed the packages that reported results but made no further progress and was interrupted; the target did not complete. |
| `make lint` | Passed | `go vet ./...` and `cargo clippy -- -D warnings` completed successfully outside the sandbox. |
| `make verify` | Incomplete | Its test phase likewise made no further progress after early packages passed and was interrupted, so the aggregate target did not complete. |

T032 and T034 remain incomplete. Repository-wide compilation and lint are restored, but the full test target must be diagnosed to completion. A follow-up acceptance run must also execute the ten-repetition measured matrix with real Zellij/browser/terminal clients and a live provider.
