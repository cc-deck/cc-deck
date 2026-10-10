# Research: Zellij Session Sharing

## Decisions

### Zellij lifecycle

Wrap the validated Zellij 0.44.3 web CLI for status, token creation/revocation, web-server lifecycle, and per-session sharing. Probe capabilities before mutation. A custom server would duplicate Zellij and remote backends are deferred.

### Provider boundary

Define a small provider lifecycle contract and ship Cloudflare Quick Tunnel plus a deterministic test provider. This preserves the chosen extension point without expanding V1 to multiple production backends.

### Transaction and process seams

Use an injected command runner and reverse-order compensations. External commands partially fail; explicit compensations make startup rollback, idempotent stop, and stale reconciliation testable.

### State and secrets

Follow `internal/ws/state.go`: atomic XDG state with 0700 directories and 0600 files. Persist token labels and process metadata, never token values. Config storage is unsuitable for ephemeral secrets.

### Accepted V1 risks

Keep terminal attach experimental with the validated certificate bypass and warning. Crash cleanup is best effort and reconciled by later lifecycle actions. Brainstorm 089 owns hardening; browser-only and blocking delivery were declined.

### Verification

Use unit and contract fakes for network-free CI, plus a manual real Zellij/cloudflared quickstart. Run `make test`, `make lint`, `make verify`, and the prose voice-profile check.

