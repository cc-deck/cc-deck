# Contract: CLI Surface

**Feature**: 087-workspace-sharing-endpoint

Per constitution Principle VI, everything here is a `cc-deck` command. No workflow requires the user
to invoke `zellij` directly.

## Commands

```bash
cc-deck ws start NAME --share                          # uses the configured endpoint
cc-deck ws start NAME --share --endpoint URL           # one-shot address override
cc-deck ws start NAME --share --endpoint-name work     # select a configured endpoint
cc-deck ws start NAME --share --no-verify              # skip the verification gate
cc-deck ws invite NAME --role observer [--name LABEL]
cc-deck ws revoke NAME LABEL
cc-deck ws status NAME
cc-deck ws unshare NAME
cc-deck ws                                             # listing, never probes
```

## Flags

| Flag | Commands | Effect |
|------|----------|--------|
| `--endpoint URL` | `start --share`, `invite` | Overrides the address for this command only. Configuration is unchanged. (FR-003) |
| `--endpoint-name NAME` | `start --share`, `invite` | Selects a configured endpoint by name. Unknown names fail with the configured names listed. (FR-004, FR-005) |
| `--no-verify` | `start --share`, `invite` | Skips the gate. The resulting share reports no verification age. (FR-013, FR-014) |

`--endpoint` and `--endpoint-name` are mutually exclusive.

## Verification gate behaviour

| Command | Probes | On failure |
|---------|--------|------------|
| `ws start --share` | yes, blocking | No invitation printed. Sharing resources rolled back. **The workspace session survives** (FR-030, FR-031). Exit non-zero, message names the failing stage. |
| `ws invite` | yes, blocking | No invitation printed. No credential left behind. Exit non-zero. |
| `ws status` | yes | Reports `degraded`, names the stage, changes nothing. Exit non-zero. (FR-016, FR-022) |
| `ws unshare` | no | Teardown does not depend on endpoint health. |
| `ws` (list) | **never** | Renders the stored result and its age. (FR-017, FR-018, FR-019) |

## Output shapes

Listing, sharing column:

```
shared (verified 12m ago)          # last probe passed
degraded (websocket, 3m ago)       # last probe failed, stage named
shared                             # created with --no-verify, no age
```

Failure message shape, the stage is always named first:

```
Error: endpoint verification failed at the websocket stage
  https://dev.example.com serves the web client but refused the WebSocket upgrade on /ws/control.
  A proxy that forwards HTTP but drops the Upgrade and Connection headers produces a page that
  loads and a terminal that never fills.
  See: cc-deck docs, sharing guide, endpoint requirements.
```

## Removed surface

- The `sharing.provider` configuration key, `Config.SharingProvider()`, and its validation.
- The hardcoded `provider != "cloudflare"` rejection in `ws_share.go:96`.

Because the feature is unreleased, these are removed outright with no deprecation path.

## Exit codes

`0` success. Non-zero on verification failure, on an unresolvable endpoint, and on a degraded status.
A degraded status is a non-zero exit because scripts must be able to detect it, and it remains
non-destructive.
