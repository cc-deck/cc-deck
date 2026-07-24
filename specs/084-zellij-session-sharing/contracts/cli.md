# CLI Contract

- `cc-deck share start [session] [--provider cloudflare]`: prints the four role/method invitations once after complete readiness. Experimental terminal commands have a prominent warning. Failure prints no usable invitation and identifies rollback residuals.
- `cc-deck share status`: reports inactive, active, or degraded; session, provider, endpoint, role availability, and residual resources; never token values.
- `cc-deck share stop`: idempotently attempts endpoint shutdown, both token revocations, session unsharing, and state cleanup; nonzero exit identifies residual exposure.

