# Brainstorm: Share as a verb

**Date:** 2026-09-08
**Status:** active

## Problem Framing

Sharing is currently reached through a `--share` flag on `new`, `start` and `attach`.
In all three the flag means the same narrow thing: "if you are about to create the session, create it shared".
It reads like "share this workspace", which is what anyone assumes on first contact.

The gap between those two readings produced a run of failures during a live test session:

- `ws new second` starts a private session, so the obvious follow-up `ws start second --share` can never succeed.
- The refusal tells the user to kill and recreate the session, destroying whatever is running in it.
- `unshare` already exists as a verb, so the pair is lopsided: there is a way to stop sharing but no way to start it.
- `ws update --endpoint` was added the same day to move an active share, putting a third spelling of "sharing" on a fourth command.

The restriction that forces the kill-and-recreate also looks unnecessary.
`web_sharing "on"` is set globally in the Zellij config, and cc-deck already enforces that through `EnsureZellijWebSharing`.
Two sessions created without any sharing flag, `cc-deck-local` and a throwaway `diag-plain2`, both served correctly over the web client during testing.
The guard infers "this session cannot be shared" from how the session was created, when what actually matters is whether the web server can serve it.

A separate defect compounds the confusion.
A session with no attached terminal client renders nothing, so a guest opening a valid invitation sees a blank screen.
Sharing reports success and the guest sees nothing, with no indication anywhere that the two facts are connected.

## Approaches Considered

### A: Verification is the judge

Remove the creation-history guard.
`ws share` turns sharing on and runs the existing five-layer endpoint probe against that session's URL.
A probe that passes is itself proof the session is servable, because it fetched the web client, exchanged a token for a session cookie, and upgraded a WebSocket.

- Pros: no new concept and no new mechanism; reuses machinery that is already built, tested and well-worded; a single failure path whose staged diagnostics already exist.
- Cons: a failure attributes the problem to the endpoint even when the session is at fault. The stages soften this, since DNS, TLS and HTTP failures are endpoint-shaped while a failure only on the session path is session-shaped.

### B: Explicit shareability check

Add a distinct pre-flight that asks Zellij whether the named session is web-shareable, run before endpoint verification.

- Pros: precise attribution, so "your session cannot be shared" and "your endpoint is broken" are never confused.
- Cons: a new mechanism to build and keep correct. Zellij does not expose this cleanly; `list-clients` cannot distinguish a web client from a terminal client, so any check would be inferred rather than authoritative.

### C: Guarantee by configuration

Treat every session as shareable by construction, on the grounds that cc-deck already forces global `web_sharing "on"`, and check nothing beyond the endpoint.

- Pros: the simplest possible design.
- Cons: silently wrong whenever the guarantee does not hold, which is the same class of failure that made this problem expensive to diagnose in the first place.

## Decision

**Approach A.** Verification decides, and the creation-history guard goes away.

The command surface becomes symmetric around a single verb:

```
cc-deck ws share NAME --endpoint URL      start sharing, in place
cc-deck ws share NAME --endpoint OTHER    move an active share
cc-deck ws unshare NAME                   stop
cc-deck ws invite NAME                    add an invitation
cc-deck ws revoke NAME LABEL              remove one
```

`ws share` acts on the session as it is.
Nothing is killed, nothing is recreated, and whatever is running keeps running.

The `--share` flag is removed from `new`, `start` and `attach`.
The feature is unreleased, so there is no compatibility cost, and one way to do a thing is worth more than two spellings of it.
`ws update --endpoint`, added earlier the same day, folds into the verb.

## Key Requirements

- `ws share NAME` shares a workspace whose session is already running and private, without restarting it.
- Re-running `ws share` with the same endpoint is idempotent and reports the live share rather than doing nothing quietly.
- Re-running it with a different endpoint moves the share. The move revokes existing invitations and says so plainly, because every distributed link stops working and that cost cannot be hidden.
- Endpoint verification remains a blocking gate: no invitation is printed before the endpoint passes all five layers.
- A failed share leaves the workspace running and the previous share intact.
- `ws share` reports when no terminal client is attached, because guests would otherwise open a valid invitation onto a blank screen. Sharing still succeeds; the output states that someone must attach.
- The `--share` flag is removed from `new`, `start` and `attach`.
- `ws update --endpoint` is removed; the verb owns moving.
- Sharing stays under the workspace group as `cc-deck ws share`, alongside `unshare`, `invite` and `revoke`.

### Folded in from the idea inbox

- **endpoint-name-dual-meaning**: `StaticEndpoint.Name()` returns the implementation kind while `EndpointRef.Name` carries the user's configured endpoint name, so "name" means two things in one package. Rename the interface method to `Kind()`. The verb makes sharing the primary consumer of the endpoint API, which exposes the collision further.
- **dead-invitation-set-api**: `BuildInvitations` and `InvitationSet` have no production caller and are kept alive only by their own tests. The dead surface grew when the terminal invitation was dropped from the CLI output, orphaning `Invitation.Terminal` and `terminalInvitation()` as well. Remove all of it.

## Open Questions

- How should the output distinguish a probe failure caused by the session from one caused by the endpoint, given that approach A deliberately uses one gate for both?
- Should `ws share` on a workspace with no session at all create one, or refuse and point at `ws start`? The symmetric reading suggests refusing, since `share` is not a creation verb, but that makes the common first-time path two commands.
- Does anything else need to know that a session is shared before it renders, now that the blank-screen condition is reported? The sidebar shows a sharing state today, but it is not connected to whether a client is attached.
- Is the "no terminal client attached" condition detectable reliably enough to report without false positives, given that `list-clients` counts web clients too?
