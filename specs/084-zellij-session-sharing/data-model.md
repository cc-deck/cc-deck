# Data Model: Zellij Session Sharing

## SharingOperation

Fields: opaque ID, exact session name, provider name, endpoint URL, interactive and observer token labels, provider process metadata, lifecycle state, timestamps, and residual resources. Raw token values are never persisted.

States: `starting`, `active`, `stopping`, `degraded`. Only one operation may exist. Startup failure compensates toward absent; unresolved cleanup becomes degraded; reconciliation probes and removes or retains residual state.

## InvitationSet

Ephemeral output containing interactive and observer browser URLs and terminal commands plus the terminal risk warning. It exists only after both roles and endpoint readiness succeed.

## ProviderStatus

Provider key, `starting|ready|stopped|failed|unknown`, safe diagnostic, public endpoint when ready, and process metadata.

