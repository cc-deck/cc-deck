# Implementation Plan: Workspace Sharing via External Endpoint

**Branch**: `087-workspace-sharing-endpoint` | **Date**: 2026-09-07 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/087-workspace-sharing-endpoint/spec.md`

## Summary

cc-deck stops managing tunnels. The user supplies an endpoint that is already serving, and cc-deck
verifies it through five layers before printing any invitation, records what it found, and never
tears anything down without positive evidence that the workspace session is gone.

Technically this is more deletion than addition. The `Provider` interface, the Cloudflare provider,
the process abstraction, and the PID-based lifecycle all go away, taking with them the entire class
of defect that produced the guard deadlock, the unowned tunnel kill, and the teardown on an
inconclusive probe. What replaces them is a five stage probe with precise failure attribution, plus
two ownership fixes to teardown. The state machine, the lifecycle lock, the step wise persisted
teardown, the invitation logic, and the token lifecycle are all retained; they were not the source of
the failures.

## Technical Context

**Language/Version**: Go 1.25.0. The Rust plugin in `cc-zellij-plugin/` is untouched by this feature.

**Primary Dependencies**: `spf13/cobra` for CLI, `gopkg.in/yaml.v3` for state, `stretchr/testify` for
tests. **No new dependency is added**; the WebSocket stage performs the upgrade handshake by hand
over `net`/`crypto/tls` rather than pulling in a WebSocket library to read one status code (R1).

**Storage**: `~/.local/state/cc-deck/share.yaml`, mode 0600, written atomically through a temp file
and rename by `FileStore`. Mutual exclusion via `flock` on a sibling `.lock` file. Paths resolved
through `internal/xdg`, never `adrg/xdg`.

**Testing**: `go test` through `make test`. Unit tests use `httptest.Server` fakes that can fail at
each probe stage independently. One end to end test starts a real Zellij web server and session, and
skips when `zellij` is absent.

**Target Platform**: macOS and Linux, wherever Zellij 0.44.3 or newer runs.

**Project Type**: CLI tool with an embedded Zellij plugin. This feature touches the Go CLI only.

**Performance Goals**: Probe completes within fifteen seconds, configurable (SC-005). Listing performs
zero network requests and stays within ten percent of an unshared listing (SC-008).

**Constraints**: Every `zellij` invocation stays bounded; a wedged session server accepts connections
and never answers. No secret is ever persisted. No background process, no polling, no supervision.

**Scale/Scope**: One active share per machine, held in a single state file guarded by one lock. Roughly
1,900 lines removed and 900 added across `internal/share`, `internal/cmd`, and `internal/config`.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Evidence |
|-----------|--------|----------|
| I. Tests and documentation ship with the feature | **PASS** | FR-038 through FR-042 name each required artifact: `cli.adoc`, `configuration.adoc`, README, a guide page, and voice compliance. SC-012 makes it a completion condition. This was a gate finding and was fixed before planning. |
| II. Interface implementations satisfy behavioural contracts | **PASS** | `contracts/endpoint-contract.md` Part 2 states nine behavioural requirements for `Endpoint` before any implementation exists, and `provider_contract_test.go` is rewritten against it. |
| III. Build and tool rules | **PASS** | `make test`, `make lint`, `make install` only. Never `go build`. Paths through `internal/xdg`. No container work in this feature. |
| IV. Plugin debug logging | **N/A** | No plugin change. |
| V. Command files are executable code | **N/A** | No `internal/build/commands/*.md` change. |
| VI. Zellij is an implementation detail | **PASS with one noted tension** | Every command in `contracts/cli-contract.md` is a `cc-deck` command, and FR-037 makes it a requirement. See below. |

**Principle VI tension, stated rather than hidden**: this feature requires the user to run their own
reverse proxy before sharing. That is not a Zellij command, so it does not violate the letter of
Principle VI, which is about abstracting the multiplexer. It does mean `ws start --share` is no longer
self sufficient, which the spec records as a deliberately accepted cost. Principle VI.2 says a
workflow needing a raw command is a coverage gap to document; the endpoint recipes in the guide are
that documentation, and supervised endpoints are recorded in Out of Scope as the way to close it if
real usage demands it.

**No violations require justification. Complexity Tracking is omitted.**

## Project Structure

### Documentation (this feature)

```text
specs/087-workspace-sharing-endpoint/
├── plan.md                          # This file
├── spec.md                          # 47 requirements, 13 success criteria
├── research.md                      # Phase 0, eight decisions
├── data-model.md                    # Phase 1
├── quickstart.md                    # Phase 1
├── contracts/
│   ├── endpoint-contract.md         # Proxy contract + Endpoint behavioural contract
│   └── cli-contract.md              # Commands, flags, output shapes, exit codes
├── checklists/
│   └── requirements.md              # 16/16, four validation iterations
└── tasks.md                         # Phase 2, NOT created by /speckit-plan
```

### Source Code (repository root)

```text
cc-deck/internal/share/
├── endpoint.go            ADD     StaticEndpoint, EndpointRef, resolution precedence
├── probe.go               ADD     five stage verifier, stage attribution
├── probe_test.go          ADD     stage matrix against httptest fakes
├── endpoint_test.go       ADD
├── e2e_test.go            ADD     real zellij web + session, skips when absent
├── cloudflare.go          DELETE
├── cloudflare_test.go     DELETE
├── provider.go            REWRITE Provider/Process/ProviderHandle out, Endpoint in
├── provider_contract_test.go REWRITE against Endpoint
├── service.go             EDIT    Start loses process launch; Status loses teardown on
│                                  endpoint failure; teardownLocked gains ownership gate
├── model.go               EDIT    WebServerOwned, LastProbe, EndpointName; drop Provider,
│                                  ProviderHandle, EndpointStopped
├── zellij.go              KEEP    already bounded and correct
├── state.go, lock.go      KEEP
├── invitation.go, labels.go KEEP  invitation and label logic retained
└── service_test.go        EDIT

cc-deck/internal/cmd/
├── ws_share.go            EDIT    drop provider wiring and the hardcoded rejection,
│                                  add --endpoint / --endpoint-name / --no-verify
├── ws.go                  EDIT    listing renders LastProbe with age
└── ws_share_test.go       EDIT

cc-deck/internal/config/
├── config.go              EDIT    SharingConfig replaced, SharingProvider() removed
└── validate.go            EDIT    validate endpoint shape instead of provider name

docs/
├── modules/reference/pages/cli.adoc            EDIT  FR-038
├── modules/reference/pages/configuration.adoc  EDIT  FR-039
└── modules/using/pages/sharing.adoc            EDIT  FR-041, endpoint recipes
README.md                                       EDIT  FR-040
```

**Structure Decision**: The existing package layout is kept. This is a rewrite of the exposure layer
inside `internal/share`, not a restructuring. `endpoint.go` and `probe.go` are new files in that
package rather than a subpackage, because they share the `Zellij` interface and the state types and
splitting them would force those types to be exported for no benefit.

## Implementation Sequencing

Four groups, ordered so each is independently verifiable. Detailed tasks come from `/speckit-tasks`.

1. **Model and configuration.** `ProbeResult`, `WebServerOwned`, `LastProbe`, the new
   `SharingConfig`, and resolution precedence. Pure data, no behaviour, fast to verify.
2. **The probe.** `endpoint.go` and `probe.go` with the full stage matrix from the contract. This is
   the new critical path and carries the most test weight. Test-first: the upgrade-refusing fake
   should exist before the stage that catches it.
3. **Service surgery.** Remove the provider from `Start`, sever the endpoint failure path from
   `reconcileLocked` in `Status` (R5), and gate the web server teardown on ownership (R6). Highest
   regression risk, which is why it follows a proven probe rather than preceding it.
4. **CLI, deletion, and documentation.** Flags, listing output, deleting `cloudflare.go` and the
   `Provider` types, and the five documentation artifacts. Documentation lands here, on this branch,
   not as follow-up work.

## Risks

| Risk | Mitigation |
|------|------------|
| Severing teardown from the endpoint path leaves a genuinely dead share alive | Intended. Session absence remains the only teardown trigger, and it is still checked on every status. `unshare` always works regardless of endpoint health. |
| Minting and revoking a token per probe adds two bounded `zellij` calls to every `status` | Both sit inside the probe deadline. `CreateToken` ignores its label argument (`zellij.go:122`), so the probe must revoke by the name Zellij returns, not one it chose. Called out in R4. |
| The end to end test corrupts the developer's live session | Every session-creating command runs under `env -u ZELLIJ -u ZELLIJ_SESSION_NAME -u ZELLIJ_PANE_ID`. This is the same defect as the unguarded `EnsureSession` folded into this feature. |
| A hand-written WebSocket handshake is subtly wrong | It asserts only `101` plus `Sec-WebSocket-Accept`. The contract test matrix covers both the accepting and the refusing server. |
| Removing `sharing.provider` breaks an existing config | The feature is unreleased. Validation reports unknown keys rather than failing silently. |

## Deferred to implementation

Two defects the spec folds in are handled during group 3 and 4 rather than as separate work: the
missing `ZELLIJ` environment guard in `EnsureSession` (`internal/ws/local.go`), and the unconditional
web server teardown.

## Post-Design Constitution Re-check

Re-evaluated after Phase 1. **All gates still pass.** The design adds no new project, no new
dependency, and no new abstraction beyond the `Endpoint` seam the spec explicitly justifies. Contracts
were written before implementations, satisfying Principle II. The documentation artifacts Principle I
requires are named in the file tree and sequenced into group 4 on this branch.
