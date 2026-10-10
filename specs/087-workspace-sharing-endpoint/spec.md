# Feature Specification: Workspace Sharing via External Endpoint

**Feature Branch**: `087-workspace-sharing-endpoint`

**Created**: 2026-09-07

**Status**: Draft

**Input**: User description: "docs/superpowers/specs/2026-09-06-workspace-sharing-endpoint-design.md"

## Clarifications

New requirements added during clarification take the next free identifier rather than being inserted
in document position, so that every identifier already referenced elsewhere stays stable.

### Session 2026-09-07

- Q: The spec calls an invitation a "time bound" grant, but no requirement defines a lifetime and the existing code has no expiry field. Do invitations expire on a timer? → A: No. Remove the "time bound" claim. Access ends only on unshare or on confirmed session death, both of which are already specified. Expiry is recorded as explicitly out of scope rather than silently dropped.
- Q: FR-023 forbids persisting a degraded result, yet the Verification result entity records which layer failed and FR-018 reports the stored result in listings. Does a failed probe record anything? → A: Yes. A failed probe updates the stored verification result with the failing layer and the time, while leaving the sharing state machine untouched. FR-023 governs the state machine, not the observation.
- Q: FR-010 requires verification to use the real credential path without consuming a person's invitation, but does not say what credential it uses instead. Which credential does the probe authenticate with? → A: A short lived observer-role credential minted for each probe and revoked as soon as the probe finishes. Nothing is persisted, preserving the existing invariant that the service stores no secrets.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Share a workspace through an endpoint I already run (Priority: P1)

A developer already runs a public entry point in front of their machine, such as a long lived tunnel managed by their operating system's service manager. They want to hand a colleague a working link to one of their workspaces. They tell cc-deck which workspace to share, cc-deck checks that the entry point genuinely serves that workspace all the way through to a live terminal, and only then does it print an invitation.

**Why this priority**: This is the whole feature. Without it there is nothing to verify, diagnose, or tear down. It is also the slice that fixes the reported defect, because today a user can receive an invitation for an entry point that was never confirmed to work.

**Independent Test**: Configure an entry point that forwards correctly to the local machine, share a workspace, and open the printed invitation in a browser. The terminal fills with live workspace output. Fully testable on its own and delivers the complete sharing value.

**Acceptance Scenarios**:

1. **Given** a configured entry point that correctly forwards to the workspace host, **When** the developer shares a workspace, **Then** cc-deck confirms the entry point serves a live terminal and prints an invitation containing the entry point address.
2. **Given** an entry point that serves pages but silently refuses long lived connections, **When** the developer shares a workspace, **Then** cc-deck refuses to print an invitation and reports that the long lived connection layer is the one at fault.
3. **Given** a guest who opens a printed invitation, **When** the page loads, **Then** the guest reaches a live terminal without any further setup.
4. **Given** no entry point is configured anywhere, **When** the developer asks to share a workspace, **Then** cc-deck refuses with guidance on how to configure one, and does not leave a partially shared workspace behind.
5. **Given** a configured entry point, **When** the developer overrides it with a different address for a single command, **Then** the override is used for that command only and the configured value is unchanged.
6. **Given** a single command that both creates a workspace and shares it, **When** verification fails, **Then** the workspace is running and usable locally, no invitation is printed, no sharing resources remain, and the reported failure is about sharing rather than about the workspace.
7. **Given** the developer shares with verification explicitly skipped, **When** the share is created, **Then** an invitation is printed and the share is reported without a verification age.

---

### User Story 2 - Find out why a share stopped working (Priority: P2)

A share that worked an hour ago no longer does. The developer asks cc-deck for the workspace's status and gets an answer that names the layer at fault, rather than a generic failure, and gets it quickly enough to be usable while debugging.

**Why this priority**: The failure that motivated this work took a full day to diagnose because cc-deck reported success while the guest saw a blank screen. Attribution is what converts an outage into a fix. It depends on P1 existing but is independently valuable and independently testable.

**Independent Test**: Share a workspace successfully, then break one layer of the entry point at a time and ask for status after each. Each break produces a distinct, correctly attributed report.

**Acceptance Scenarios**:

1. **Given** a shared workspace whose entry point host no longer resolves on the local network but does resolve publicly, **When** the developer asks for status, **Then** cc-deck reports the share as degraded and identifies local name resolution as the cause.
2. **Given** a shared workspace whose entry point now returns server errors while its process is still alive and reporting itself healthy, **When** the developer asks for status, **Then** cc-deck reports the share as degraded rather than healthy.
3. **Given** any degraded result, **When** status completes, **Then** no credentials have been revoked, no processes have been stopped, and the share can recover on its own when the entry point returns.
4. **Given** a workspace whose underlying session is confirmed gone, **When** the developer asks for status, **Then** cc-deck revokes access and clears the share.
5. **Given** any status check, **When** the entry point is unreachable and never answers, **Then** the command still returns within its time bound rather than hanging.
6. **Given** a status check that reported a specific layer as broken, **When** the developer subsequently lists workspaces without re-checking, **Then** the listing names that same failing layer with the age of the check, so the failure survives beyond the command that found it.

---

### User Story 3 - Stop sharing without collateral damage (Priority: P2)

The developer stops sharing a workspace. Their own entry point keeps running, any service they started themselves keeps running, and the workspace itself keeps running with all its work intact. Only the access that cc-deck granted goes away.

**Why this priority**: Equal in importance to diagnosis because the current behavior actively destroys user state. Stopping a share kills a service the user was already running, and a single listing command has revoked live credentials. Both are data loss from the user's point of view.

**Independent Test**: Start a supporting service manually, share a workspace, then unshare. The manually started service is still running afterwards, and so is the workspace.

**Acceptance Scenarios**:

1. **Given** the developer started the supporting service themselves before sharing, **When** they unshare, **Then** that service is still running afterwards.
2. **Given** cc-deck started the supporting service as part of sharing, **When** the developer unshares, **Then** cc-deck stops it.
3. **Given** any unshare, **When** it completes, **Then** the workspace session and its contents are untouched and the developer's own entry point is untouched.
4. **Given** an unshare interrupted partway through, **When** the developer runs it again, **Then** it resumes correctly and leaves nothing stranded.
5. **Given** an unshare, **When** it completes, **Then** every invitation previously issued for that workspace stops working.

---

### User Story 4 - Keep listing workspaces fast and honest (Priority: P3)

The developer lists their workspaces many times a day. The listing must stay fast, so it does not check the network, but it must not pretend to know more than it does. It shows the last known sharing result together with how old that result is.

**Why this priority**: A quality of life and honesty requirement rather than a capability. It matters because the alternative, checking every entry point on every listing, reintroduces exactly the per row repetition that made listing slow and lock heavy.

**Independent Test**: Share a workspace, wait, then list workspaces. The listing returns as fast as an unshared listing and reports the sharing result with its age.

**Acceptance Scenarios**:

1. **Given** several shared workspaces, **When** the developer lists them, **Then** no network checks are performed and the command completes as quickly as a listing with no shares.
2. **Given** a workspace verified some minutes ago, **When** the developer lists workspaces, **Then** the listing shows the stored result along with its age rather than implying live knowledge.
3. **Given** a workspace shared with verification skipped, **When** the developer lists workspaces, **Then** the listing shows the same sharing state as any other share but carries no verification age, so the absence of a check is visible without introducing a separate state.
4. **Given** several workspaces sharing state, **When** the developer lists them, **Then** sharing state is resolved once for the whole listing rather than once per workspace.

---

### User Story 5 - Name and switch between entry points (Priority: P3)

A developer with more than one environment, for example one for work and one at home, names each entry point once and selects between them by name.

**Why this priority**: Convenience for repeat users. The single entry point case is fully covered by P1, so this is genuinely optional for a first release.

**Independent Test**: Configure two named entry points and a default, share using the default, then share another workspace naming the other one explicitly. Each uses the correct address.

**Acceptance Scenarios**:

1. **Given** named entry points and a declared default, **When** the developer shares without naming one, **Then** the default is used.
2. **Given** named entry points, **When** the developer names one explicitly, **Then** that one is used.
3. **Given** a name that does not match any configured entry point, **When** the developer shares, **Then** cc-deck refuses with a message listing the names that are configured.

---

### Edge Cases

- The entry point address resolves for the public internet but is blocked by a filter on the developer's own network. cc-deck must distinguish this from an address that does not exist at all, and say which it is.
- The entry point process is alive and reports itself healthy while the address it serves has degraded to returning errors. Process liveness must never be accepted as evidence that sharing works.
- The entry point forwards ordinary page requests correctly but drops long lived connections. This produces a page that loads and a terminal that never fills, and must be caught before an invitation is printed.
- The entry point strips or rewrites the credential the login step issues, so the guest is bounced back to login forever.
- The entry point rewrites request paths, so the wrong workspace, or no workspace, is selected.
- The entry point closes idle connections after a short period, so a terminal disconnects mid use.
- The underlying multiplexer becomes unresponsive: it accepts connections and never answers. Every command that inspects sharing state must remain bounded and must not conclude the workspace is gone.
- The entry point is briefly down while the workspace is alive. This must be reported as degraded and must never revoke credentials, because a restarting entry point is not consent to end access.
- The workspace session ends while sharing is active. Access must end with it, at the next command that establishes sharing state. Because nothing runs in the background, revocation is not immediate, and the documentation must not imply that it is.
- The guest sits behind a different network filter than the host. cc-deck can only verify from its own machine and must not imply otherwise.
- Verification times out or the process is interrupted partway through a probe. The credential it minted must still be revoked, so that a repeatedly failing endpoint does not leave a trail of live credentials behind.
- Verification is skipped explicitly. The resulting share must carry no verification age, so it is never presented as though it had been checked.
- A workspace is created and shared by one command and verification fails. The workspace must survive, because a problem in the exposure layer is not a reason to discard work the user asked for.
- Two workspaces are shared through the same entry point at once. This is permitted, because the entry point belongs to the user and workspaces are selected by address path. Each share keeps its own invitations and its own verification result, and unsharing one must not affect the other.

## Requirements *(mandatory)*

### Functional Requirements

**Entry point ownership**

- **FR-001**: cc-deck MUST NOT start, supervise, restart, or stop any user provided entry point, under any command.
- **FR-002**: cc-deck MUST require that an entry point address can be determined before a workspace can be shared. Whether that address is actually serving is established by verification, not asserted by configuration, and is waived only by the explicit skip in FR-013.
- **FR-003**: cc-deck MUST accept an entry point address from configuration, and MUST allow a single command to override it without changing the configuration.
- **FR-004**: cc-deck MUST support naming multiple entry points and selecting one by name, with a declared default used when no name is given.
- **FR-005**: cc-deck MUST refuse to share, with actionable guidance, when no entry point can be determined.

**Verification**

- **FR-006**: cc-deck MUST verify an entry point before printing any invitation for it.
- **FR-007**: Verification MUST check, in order and stopping at the first failure: that the address resolves, that the secure connection is valid, that the expected client is served, that the credential exchange succeeds and returns a usable credential, and that a long lived connection can be established.
- **FR-008**: Each verification failure MUST identify which of those layers failed, in language a user can act on.
- **FR-009**: When the address fails to resolve locally but resolves through an independent public resolver, cc-deck MUST report the failure as local filtering rather than as a missing address.
- **FR-010**: Verification MUST exercise the real credential path rather than a substitute, so that a credential problem is caught by verification rather than by the guest. The credential it uses MUST NOT count against, consume, or invalidate any invitation issued to a person.
- **FR-044**: Verification MUST mint its own credential for each probe, MUST scope it to the least privileged role available, and MUST revoke it when the probe finishes, whether the probe passed or failed.
- **FR-045**: The probe credential MUST NOT be persisted to disk or recorded in sharing state at any point, preserving the existing guarantee that no secret is stored.
- **FR-046**: A probe that fails or times out MUST still revoke the credential it minted, so that a repeatedly failing endpoint cannot accumulate live credentials.
- **FR-011**: Verification MUST NOT disturb the workspace being shared. It MUST NOT attach to, write to, or otherwise alter the running session.
- **FR-012**: The whole verification MUST complete within a bounded time, configurable, defaulting to fifteen seconds.
- **FR-013**: Users MUST be able to skip verification explicitly for a single command.
- **FR-014**: A share created with verification skipped MUST NOT be reported as verified. It MUST report the same state as a verified share, with the verification age omitted rather than a separate state introduced. No verification age means no verification was performed.

**Where verification runs**

- **FR-015**: Sharing a workspace and issuing an invitation MUST both run full verification as a blocking gate.
- **FR-016**: Asking for a workspace's status MUST re-run verification, and its result MUST determine whether sharing is reported as shared or degraded. These are the only two sharing states; no other value is reported anywhere.
- **FR-017**: Listing workspaces MUST NOT perform any verification or network access.
- **FR-018**: Listing workspaces MUST report the stored verification result together with its age, and MUST omit the age when no verification has been performed, per FR-014. When the stored result is a failure, the listing MUST name the layer that failed alongside its age.
- **FR-019**: Listing workspaces MUST resolve sharing state once for the whole listing, not once per workspace.
- **FR-020**: cc-deck MUST NOT check sharing health in the background. Health is established only when a user runs a command that requires it.

**Evidence and teardown**

- **FR-021**: cc-deck MUST NOT tear down any resource without positive evidence that the underlying workspace session is gone. Absence of a response is not evidence of absence.
- **FR-022**: A failed verification MUST NOT revoke credentials, stop any process, or delete sharing state.
- **FR-023**: A degraded result MUST NOT be persisted as the workspace's stored sharing state, because a persisted degraded operation would be torn down by the next healthy call. This governs the state machine only, not the recorded observation.
- **FR-047**: Every completed verification, whether it passed or failed, MUST update the stored verification result with its outcome, the failing layer when there was one, and the time it was taken. This is an observation, not a state transition, and MUST NOT move the workspace's sharing state or trigger any teardown.
- **FR-024**: When the workspace session is confirmed gone, cc-deck MUST revoke all access granted for it.
- **FR-025**: Stopping sharing MUST revoke every invitation previously issued for that workspace.
- **FR-026**: Stopping sharing MUST NOT stop, signal, or otherwise affect the user's entry point.
- **FR-027**: cc-deck MUST record, at the time it acts, whether it started the supporting service, and MUST stop that service on teardown only when it started it.
- **FR-028**: Stopping sharing MUST NOT end the workspace session or affect its contents.
- **FR-029**: Teardown MUST persist its progress after each step, so that an interrupted teardown resumes correctly rather than stranding resources.
- **FR-030**: When sharing fails to complete, cc-deck MUST roll back every sharing resource it created, leaving no partial share.
- **FR-043**: Invitations MUST NOT expire on a timer. Unsharing a workspace and confirming that its session is gone are the only two events that end access, and together they MUST be sufficient to revoke every invitation ever issued for that workspace.
- **FR-031**: Rollback MUST NOT extend to the workspace session. When a single command both creates a workspace and shares it, and sharing fails, the workspace MUST remain running and usable locally, and only the sharing failure is reported. Sharing is an additive operation on a workspace, never a precondition for its existence.

**Robustness**

- **FR-032**: Every command that inspects sharing state MUST complete within a bounded time even when the underlying multiplexer never responds.
- **FR-033**: cc-deck MUST NOT treat the liveness of any process as evidence that sharing works.

**Documentation**

- **FR-034**: The requirements an entry point must satisfy MUST be documented, including long lived connection passthrough, credential preservation in both directions, unmodified request paths, unbuffered responses, a long idle timeout, and secure transport for anything reachable outside a trusted network.
- **FR-035**: Recipes for producing a conforming entry point with common external tools MUST be documented as user guidance, not implemented as cc-deck functionality.
- **FR-036**: Documentation MUST state plainly that cc-deck verifies reachability only from the host machine, and cannot detect a filter on the guest's network.
- **FR-037**: All acceptance criteria and smoke tests for this feature MUST be expressible entirely through cc-deck commands, with no direct invocation of the underlying multiplexer by the user.
- **FR-038**: The user facing command reference MUST cover every new and changed command and flag introduced by this feature, including the entry point override and the verification skip.
- **FR-039**: The configuration reference MUST cover the redefined sharing schema, including the single address form, the named entry point form, the declared default, and the verification time bound, and MUST record that the previous provider selection key is removed.
- **FR-040**: The project README MUST reflect the changed sharing workflow, specifically that an entry point must exist before a workspace can be shared.
- **FR-041**: A user guide page MUST cover the sharing workflow end to end, and MUST carry the entry point requirements from FR-034, the recipes from FR-035, and the host only verification limitation from FR-036.
- **FR-042**: All documentation produced for this feature MUST follow the project's established voice profile.

### Key Entities

- **Shared workspace**: A workspace with sharing active. Carries the entry point in use, whether the supporting service was started by cc-deck, the last verification result and when it was taken, and the invitations issued for it.
- **Entry point**: A named, user provided public address that forwards to the workspace host. cc-deck resolves it from configuration and verifies it, and has no control over its lifecycle.
- **Verification result**: The outcome of the layered check, recording whether it passed, which layer failed if it did not, and the time it was taken. Every completed check overwrites it, pass or fail. It is what listings report and what status recomputes. It is an observation about the endpoint, deliberately separate from the workspace's sharing state, so recording a failure here never moves the state machine.
- **Invitation**: A grant of access to a shared workspace, optionally labelled and role scoped. It does not expire on a timer. Always created by cc-deck and therefore always revocable by cc-deck.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A developer running a conforming entry point can share a workspace and hand a guest a link that reaches a live terminal on the first attempt, with no manual troubleshooting.
- **SC-002**: The end to end acceptance test drives a real workspace through a real entry point and fails the build for each of the following, independently: an address that does not resolve, an invalid secure connection, an entry point serving something other than the expected client, a credential exchange that returns no usable credential, and an entry point that serves pages correctly while refusing long lived connections. The last of these is the failure that reached a user undetected, and no release passes without it being caught.
- **SC-003**: When exposure is broken, the reported failure names the correct layer in 100% of the layer specific test cases, rather than reporting a generic failure.
- **SC-004**: No cc-deck command hangs indefinitely. Every command that inspects sharing state returns within its stated bound even when the entry point and the multiplexer are both unresponsive.
- **SC-005**: Verification completes within fifteen seconds by default, and the bound is configurable.
- **SC-006**: cc-deck never stops a process it did not start. Verified by tests covering an externally started supporting service, an externally provided entry point, and the workspace session itself.
- **SC-007**: A failing verification never results in revoked access or deleted state. Verified across every failure mode, including timeouts and unresponsive dependencies.
- **SC-008**: Listing workspaces performs zero network requests, and listing a set of shared workspaces takes no more than ten percent longer than listing the same number of unshared workspaces.
- **SC-009**: A user can determine, from the listing alone, both the last known sharing state and how stale that knowledge is.
- **SC-010**: The complete share, verify, invite, diagnose, and unshare journey is demonstrable using only cc-deck commands.
- **SC-011**: No sharing failure, in any test case, results in the loss of a workspace or its contents.
- **SC-012**: Every documentation artifact required by the project's completion rules is updated on the same branch that delivers the change, verified before the feature is considered complete.
- **SC-013**: After any sequence of verification attempts, successful or failed, no credential minted by verification remains valid and none has been written to disk.

## Assumptions

- The user is responsible for running the entry point. Zero configuration sharing was explicitly considered and rejected as a requirement.
- The feature is unreleased, so the configuration schema is redefined rather than migrated, and no backwards compatibility path is required. The previous provider selection setting is removed outright.
- Exposure mechanisms such as managed tunnels, mesh network sharing, and manual reverse tunnels are documentation recipes. None of them is implemented as cc-deck functionality in this release.
- The interface separating cc-deck from the entry point is retained as a seam even though this release has exactly one implementation, so that a future supervised entry point is an addition rather than a restructuring.
- When both a single entry point address and a set of named entry points are configured, the explicitly named or explicitly overridden value takes precedence, then the declared default, then the single address.
- Verification opens a control channel rather than a terminal channel, so it can prove the connection layer works without touching the shared session.
- Verification authenticates with a credential of its own rather than borrowing one issued to a guest, so that repeated status checks never erode a person's access. That credential is minted per probe, scoped to the least privileged role, revoked immediately afterwards, and never written down. The cost is one credential mint and revoke on every status check, accepted in exchange for storing no standing secret.
- Sharing has exactly two reported states, shared and degraded. Skipping verification does not add a third; it only leaves the verification age absent.
- The end to end acceptance test skips when the multiplexer is not installed, following the pattern already used by the existing container smoke tests, and runs in continuous integration where it is installed.
- Reachability is verified from the host only. Guest side network conditions are out of scope and documented as a known limitation.
- Skipping verification does not introduce a new sharing state. The verification age is the only signal, and its absence means no check was performed. This keeps the state model at two values.
- Sharing is additive to a workspace. A failed share never removes a workspace, whether that workspace existed beforehand or was created by the same command.
- Two pre-existing defects are folded into this work because they belong to it: creating a workspace from inside an existing multiplexer session silently appends to that session instead of creating a new one, and stopping sharing stops the supporting service unconditionally.

## Out of Scope

- Supervised entry points, where cc-deck would manage a tunnel's lifecycle. Deferred until real usage demands it.
- Mesh network sharing recipes, including the tailnet only variant. That variant offers a materially different security posture and is worth documenting only once properly tested.
- Detecting whether a guest's own network can reach the entry point.
- Background health monitoring of any kind.
- Invitation expiry. Invitations do not expire on a timer in this release. Adding a lifetime later is an addition to the invitation model rather than a change to the sharing lifecycle, since revocation already exists and would carry it.
