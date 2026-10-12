# Quickstart: Validating Workspace Sharing via External Endpoint

**Feature**: 087-workspace-sharing-endpoint

How to prove this feature works. Everything below is a `cc-deck` command, per constitution
Principle VI.

## Before you start

**The binary on your PATH is probably not this branch.** `~/bin/cc-deck` is a symlink into the main
checkout and predates this work. Confirm what you are testing before drawing any conclusion from it;
this single mistake consumed hours on 2026-09-06.

```bash
ls -l "$(command -v cc-deck)"
make install
```

**Claude Code runs inside a Zellij session.** Any command that creates a session must be run with the
Zellij environment stripped, or `--layout` silently adds a tab to your live session, exits zero, and
creates nothing:

```bash
env -u ZELLIJ -u ZELLIJ_SESSION_NAME -u ZELLIJ_PANE_ID <command>
```

## Automated validation

```bash
make test          # unit suite, includes the full probe stage matrix
make verify        # tests plus linting, the gate spex finish uses
```

The probe stage matrix in `contracts/endpoint-contract.md` runs offline against `httptest` servers.
The row that matters most serves pages correctly and refuses the WebSocket upgrade, which reproduces
the blank terminal deterministically.

The end to end acceptance test skips when `zellij` is absent and runs in continuous integration where
it is installed.

Two test failures on this branch are pre-existing and unrelated: seven `TestVoiceRelay_*` tests in
`internal/voice`, which appear to read real user configuration rather than fixtures, and the compose
smoke tests, which fail on podman environment issues.

## Manual validation

### 1. Configure an endpoint

```yaml
# ~/.config/cc-deck/config.yaml
sharing:
  endpoint: https://dev.example.com
  verify_timeout: 15s
```

The endpoint must already be serving and must satisfy `contracts/endpoint-contract.md`. cc-deck will
not start one for you, by design.

### 2. Share and verify

```bash
cc-deck ws start web --share
```

Expected: verification passes through all five stages, then invitations print. If any stage fails, no
invitation appears and the message names the stage.

### 3. Confirm the guest path

Open the printed browser invitation. Expected: a live terminal, not a blank page.

A blank page here means an endpoint that serves HTTP but drops WebSocket upgrades, and step 2 should
have caught it. If step 2 passed and the terminal is still blank, the probe has a gap worth a bug
report.

### 4. Confirm listing does not probe

```bash
time cc-deck ws
```

Expected: `shared (verified Nm ago)`, and a duration indistinguishable from an unshared listing.
Zero network requests (SC-008).

### 5. Confirm failure attribution

Stop your endpoint, then:

```bash
cc-deck ws status web
```

Expected: `degraded`, with the failing stage named. Critically, the share must still exist afterwards.
Verify nothing was destroyed:

```bash
cc-deck ws status web    # still shows the share
cc-deck ws               # degraded (stage, Nm ago)
```

Restart the endpoint and run `status` again. Expected: back to `shared` with no intervention. This is
the defect that mattered most: a single listing used to revoke live credentials and kill the tunnel.

### 6. Confirm the workspace survives a failed share

```bash
cc-deck ws start scratch --share --endpoint https://does-not-exist.invalid
```

Expected: sharing fails, and the `scratch` workspace is running and usable locally (FR-031).

### 7. Confirm unshare has no collateral damage

Start a Zellij web server yourself before sharing, then share and unshare. Expected: your web server
is still running afterwards (FR-027), your endpoint is untouched (FR-026), the workspace and its
contents are intact (FR-028), and every invitation is dead (FR-025).

## Known limitation to confirm is documented

cc-deck verifies reachability from its own host only. A guest behind a different DNS filter is
undetectable. Check that the guide says this plainly rather than implying the probe guarantees guest
access (FR-036).

Locally, AdGuard Home at 10.9.11.7 returns NXDOMAIN for every `*.trycloudflare.com` name while
10.9.11.10 and 10.9.11.1 resolve correctly, and `.7` wins. If you test with a quick tunnel and it
fails at the DNS stage, that is the probe working correctly, and it should say so by name.
