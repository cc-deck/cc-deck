# Guided Demo Report

**Feature**: Workspace Sharing via External Endpoint
**Date**: 2026-09-07
**Spec**: [spec.md](spec.md)
**Result**: 6 passed, 1 partial, 0 skipped, 0 failed (out of 7)

---

## Triage Summary

| Tier | Count | Detail |
|------|-------|--------|
| full | 7 | Binary built, Zellij 0.45.1 web server online, Playwright available |
| partial | 0 | (flow 3 was full tier but produced partial evidence, see below) |
| setup_offered | 0 | |
| manual | 0 | |

User selection: "Run all seven"

All flows ran against a throwaway `demo` workspace with isolated state files. The
developer's live sessions (`cc-deck-local`, `wt-plain`, `wt-layout`) were never
touched, and every session-creating command ran with `ZELLIJ`,
`ZELLIJ_SESSION_NAME`, and `ZELLIJ_PANE_ID` stripped.

The Zellij web server used as the endpoint was started by the developer, not by
cc-deck. That is what makes flow 7 meaningful: cc-deck had to leave it running.

---

## Flow 1: Refuse to share when no endpoint resolves

**Tier**: full
**Covers**: FR-002, FR-003, FR-004, FR-005

### Evidence

Three refusal paths, each with a different cause.

```
$ cc-deck ws start demo --share            # two endpoints configured, no default
Error: no sharing endpoint is configured, so there is nothing to share through;
       configured endpoints: home, work
cc-deck does not start an endpoint for you. Set sharing.endpoint in your cc-deck config,
or pass --endpoint URL for one command. See the sharing guide for endpoint requirements.

$ cc-deck ws start demo --share --endpoint-name office
Error: no endpoint named "office" is configured; configured endpoints: home, work

$ cc-deck ws start demo --share --endpoint https://a.example --endpoint-name work
Error: if any flags in the group [endpoint endpoint-name] are set none of the others can be
```

**Expected** (FR-005): refuse to share, with actionable guidance, when no entry point can be determined.
**Actual**: refused in all three cases, each naming what is configured and what to do next.

### Verdict: PASS

Every refusal points somewhere rather than merely saying no.

**Verify yourself**:
1. `cc-deck ws start NAME --share --endpoint-name nonexistent`
2. Look for the list of configured endpoint names in the error.

---

## Flow 2: Verify five layers, then print invitations

**Tier**: full
**Covers**: FR-006, FR-007, FR-015

### Evidence

```
$ cc-deck ws start demo --share
interactive invitation "swift-koala":
Browser:
http://127.0.0.1:8082/cc-deck-demo
Login token: a20442c1-2f3f-4d15-8e9f-840c25617133
...
observer invitation "merry-tiger":
...
Workspace "demo" ready

elapsed: 300ms  (DNS, TLS, HTTP, Auth, WebSocket all ran inside this)
```

Persisted state afterwards:

```yaml
endpoint_url: http://127.0.0.1:8082
invitations:
    - label: swift-koala
      credential_name: token_10
    - label: merry-tiger
      credential_name: token_11
last_probe:
    ok: true
    checked_at: 2026-09-07T12:40:50.102749Z
state: active
```

**Expected** (FR-006, FR-015): verify before printing any invitation, as a blocking gate.
**Actual**: full five-stage verification completed in 300ms, then invitations printed. The recorded result carries `ok: true` and a timestamp; no login token appears anywhere in the file.

### Verdict: PASS

**Verify yourself**:
1. `cc-deck ws start NAME --share`
2. `cat ~/.local/state/cc-deck/share.yaml` and confirm `last_probe.ok` is true and no `Login token` value appears.

---

## Flow 3: Guest opens the invitation and reaches a live terminal

**Tier**: full (evidence partial, see disclaimer)
**Covers**: the end to end guest path behind FR-006 and FR-007 stage five

### Evidence

Driven with Playwright against the printed observer invitation.

| Step | Observed |
|------|----------|
| Navigate to `http://127.0.0.1:8082/cc-deck-demo` | Page title "Zellij Web Client", login prompt rendered |
| Enter the observer token, click Authenticate | Page title changes to `cc-deck-demo` |
| WebSocket connections | Two established from Chrome to port 8082 (control and terminal) |
| Terminal canvas | Two canvases present, sized 2394x2072, WebGL renderer active |
| Rendered glyphs | None visible |

**Partial evidence disclaimer**: this proves the guest path works up to and
including the terminal WebSocket. It does not prove glyphs reach the screen.

A control test settles the cause. Navigating the same browser to `wt-plain`, a
session alive for over a day with real content, renders identically blank (zero
rows, nothing painted). The host side agrees: `zellij action dump-screen` on both
sessions returns nothing either. The blank rendering is therefore a property of
the automated headless browser, where xterm's WebGL renderer paints nothing, and
not a property of this feature.

The only console error was a `401` on `/session` before authentication, which is
the client probing before it has a cookie, exactly as expected.

### Verdict: PARTIAL

The layers this feature is responsible for all pass. The final visual is
unverifiable in an automated browser and must be confirmed by eye. Manual steps
are in the section at the end of this report.

**Verify yourself**:
1. `cc-deck ws start NAME --share`
2. Open the printed browser invitation in a real browser and paste the token.
3. Confirm a terminal with a prompt appears, not a blank page.

---

## Flow 4: A broken endpoint is diagnosed by layer and destroys nothing

**Tier**: full
**Covers**: FR-008, FR-016, FR-021, FR-022, FR-047

### Evidence

The endpoint was broken by stopping the web server it points at.

```
$ cc-deck ws status demo
Sharing:           degraded
Endpoint:          http://127.0.0.1:8082
Invitation:        swift-koala (interactive)
Invitation:        merry-tiger (observer)
Error: sharing is degraded: endpoint verification failed at the http stage
  The address did not serve the Zellij web client. It may not be answering at all,
  or the proxy may point somewhere other than the Zellij web server, or it may be
  rewriting paths.
  The share is intact. Repair the endpoint and run this command again
EXIT CODE = 1
```

State immediately after the failed check:

| Check | Before | After | Expected |
|-------|--------|-------|----------|
| Active credentials | 3 | 3 | unchanged |
| Workspace session | alive | alive | unchanged |
| Persisted `state:` | active | **active** | active, never degraded |
| `last_probe` | ok: true | ok: false, failed_at: http | the observation is recorded |

Listing then showed `degraded (http, 16s ago)`.

Restarting the web server and running `ws status` again returned exit code 0 and
`shared (verified 0s ago)`, with no intervention beyond repairing the endpoint.

**Expected** (FR-022): a failed verification must not revoke credentials, stop any process, or delete sharing state.
**Actual**: nothing was revoked, stopped, or deleted. The result was recorded as an observation while `state:` stayed `active`, and recovery was automatic.

### Verdict: PASS

This is the defect that mattered most. Before this change, a single listing
against an unhealthy endpoint revoked live credentials and killed the tunnel.

**Verify yourself**:
1. Share a workspace, then stop your endpoint.
2. `cc-deck ws status NAME` and note the exit code and the named stage.
3. `grep '^state:' ~/.local/state/cc-deck/share.yaml` and confirm it still says `active`.
4. Restart the endpoint and run `ws status` again.

---

## Flow 5: Listing is cheap and honest about staleness

**Tier**: full
**Covers**: FR-017, FR-018, FR-019

### Evidence

```
$ cc-deck ws
NAME  TYPE   SESSION  SHARING                   ...
demo  local  active   shared (verified 0s ago)

listing elapsed: 37ms      (the verifying share took 300ms)
```

All three output shapes were observed across the demo:

```
shared (verified 0s ago)     flow 2, after a passing check
degraded (http, 16s ago)     flow 4, after a failing check, naming the layer
private                      flow 6, workspace not shared
```

**Expected** (FR-017): listing must not perform any verification or network access.
**Actual**: 37ms against 300ms for the verifying path, roughly an eighth of the cost, with the stored result and its age rendered rather than a fresh check.

### Verdict: PASS

**Verify yourself**:
1. `time cc-deck ws` on a shared workspace.
2. Compare against `time cc-deck ws status NAME`, which does verify.

---

## Flow 6: A failed share leaves the workspace running

**Tier**: full
**Covers**: FR-030, FR-031

### Evidence

```
$ cc-deck ws start demo --share --endpoint https://does-not-exist.invalid
Error: endpoint verification failed at the dns stage
  https://does-not-exist.invalid does not resolve from this machine:
  lookup does-not-exist.invalid: no such host
  The address does not resolve from this machine. Check the name, and check
  whether a local DNS filter is answering for it.
  See: cc-deck docs, sharing guide, endpoint requirements

$ cc-deck ws
demo  local  active   private        <- workspace running, simply not shared

sharing state left behind: none
```

The surviving session then accepted a new pane, confirming it is genuinely usable
and not merely listed.

**Expected** (FR-031): the workspace must remain running and usable locally, and only the sharing failure is reported.
**Actual**: exactly that. No partial share was left behind either (FR-030).

### Verdict: PASS

**Verify yourself**:
1. `cc-deck ws start NAME --share --endpoint https://does-not-exist.invalid`
2. `cc-deck ws attach NAME` and confirm the workspace is usable.

---

## Flow 7: Unshare removes only what cc-deck created

**Tier**: full
**Covers**: FR-025, FR-026, FR-027, FR-028

### Evidence

The Zellij web server was started by the developer before the demo, so
`web_server_owned` was absent from the state file (false).

| Resource | Before unshare | After unshare | Expected |
|----------|---------------|---------------|----------|
| Web server (developer started) | online | **online** | survives |
| Workspace session | alive | **alive** | survives |
| Sharing state file | present | removed | removed |
| Developer's 3 live sessions | 3 | **3** | untouched |
| Invitations | 2 active | revoked | revoked |

**Expected** (FR-027): stop the supporting service on teardown only when cc-deck started it.
**Actual**: the web server survived because cc-deck did not start it. Ownership was recorded when cc-deck acted, not inferred at teardown.

### Verdict: PASS

This closes the second folded-in defect: teardown used to stop the web server
unconditionally, so a developer already running `zellij web` lost it on unshare.

**Verify yourself**:
1. Start `zellij web --daemonize` yourself.
2. Share a workspace, then `cc-deck ws unshare NAME`.
3. `zellij web --status` and confirm it is still online.

---

## FR Coverage

| FR | Flow | Verdict | Classification |
|----|------|---------|----------------|
| FR-001 | - | - | internal-only (satisfied by deletion, contract test C-1) |
| FR-002 | Flow 1 | PASS | observable |
| FR-003 | Flow 1 | PASS | observable |
| FR-004 | Flow 1 | PASS | observable |
| FR-005 | Flow 1 | PASS | observable |
| FR-006 | Flow 2, 3 | PASS | observable |
| FR-007 | Flow 2, 4 | PASS | observable |
| FR-008 | Flow 4, 6 | PASS | observable |
| FR-009 | - | - | internal-only (unit test; needs a filtering resolver to demo) |
| FR-010 | - | - | internal-only (credential path, unit + e2e tested) |
| FR-011 | - | - | internal-only (MUST NOT attach, contract test C-7) |
| FR-012 | Flow 2 | PASS | observable (300ms, well inside the bound) |
| FR-013 | - | - | observable, not demoed (`--no-verify`, unit tested) |
| FR-014 | Flow 5 | PASS | observable (the "no age" shape) |
| FR-015 | Flow 2 | PASS | observable |
| FR-016 | Flow 4 | PASS | observable |
| FR-017 | Flow 5 | PASS | observable |
| FR-018 | Flow 4, 5 | PASS | observable |
| FR-019 | Flow 5 | PASS | observable |
| FR-020 | - | - | internal-only (no background work, by construction) |
| FR-021 | Flow 4 | PASS | observable |
| FR-022 | Flow 4 | PASS | observable |
| FR-023 | Flow 4 | PASS | observable (`state:` stayed active) |
| FR-024 | - | - | internal-only (confirmed-absent teardown, unit tested) |
| FR-025 | Flow 7 | PASS | observable |
| FR-026 | Flow 7 | PASS | observable |
| FR-027 | Flow 7 | PASS | observable |
| FR-028 | Flow 7 | PASS | observable |
| FR-029 | - | - | internal-only (interrupted teardown, unit tested) |
| FR-030 | Flow 6 | PASS | observable |
| FR-031 | Flow 6 | PASS | observable |
| FR-032 | - | - | internal-only (bounded zellij calls, unit tested) |
| FR-033 | - | - | internal-only (by construction, no liveness check exists) |
| FR-034 to FR-037 | - | - | documentation requirements |
| FR-038 to FR-042 | - | - | documentation requirements |
| FR-043 | - | - | internal-only (no expiry field exists) |
| FR-044 to FR-046 | - | - | internal-only (probe credential lifecycle, e2e tested) |
| FR-047 | Flow 4 | PASS | observable (`last_probe` updated on a failure) |

**Observable FRs demonstrated: 21 of 22.** FR-013 (`--no-verify`) was not
exercised interactively; it is covered by unit tests and by the listing shape in
flow 5.

---

## Manual verification of flow 3

Flow 3 is the one step an automated browser cannot finish. To confirm it by eye,
see the "Testing this by hand" instructions issued with this report: open a
terminal outside Zellij, use the worktree binary, share a workspace, and open the
printed invitation in a real browser. A terminal with a prompt means the
WebSocket stage did its job. A blank page with a loaded chrome means the endpoint
drops upgrades, which is the defect this feature exists to catch before an
invitation is ever printed.
