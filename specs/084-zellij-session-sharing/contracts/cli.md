# CLI Contract

Sharing is a capability of local workspaces. The public command surface is:

```text
cc-deck ws new NAME [--share | --no-start]
cc-deck ws start NAME [--share]
cc-deck ws attach NAME [--share]
cc-deck ws invite NAME --role interactive|observer [--name LABEL]
cc-deck ws revoke NAME INVITATION_LABEL
cc-deck ws unshare NAME
cc-deck ws list
cc-deck ws status NAME
```

- `ws new` creates a ready private workspace by default. `--no-start` leaves it stopped; it conflicts with `--share`.
- `ws start` and `ws attach` converge missing infrastructure and the canonical session. `--share` is local-only and creates a missing canonical session with web sharing enabled.
- `--share` never replaces or kills an existing private session. The command explains that the session must be killed and restarted explicitly.
- `ws invite` creates one independently revocable credential and prints its raw secret exactly once. `ws revoke` revokes only the named invitation.
- `ws unshare` closes remote access and revokes all active invitations without stopping the canonical session.
- Only one local workspace may be shared per host. A different workspace is rejected while sharing is active.
- `ws list` reports `INFRA`, `SESSION`, and `SHARING`. `ws status` reports safe endpoint, invitation labels and roles, guard health, and residuals; neither output contains raw secrets.
- After canonical-session death, an ordinary `ws start` or `ws attach` recreates a private session. Sharing resumes only with an explicit `--share`.

The detached lifecycle guard is an internal hidden `ws share-guard` command and is not part of the public contract.
