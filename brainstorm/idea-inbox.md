# Idea Inbox

Ideas captured from code reviews for future brainstorming.

## ~~sdk-api-gaps~~ (resolved)

- **Status**: Resolved 2026-06-30
- **Resolution**: Command is not a creation-time concept (closed [#10](https://github.com/rhuss/openshell-sdk-go/issues/10)). Policy fixed in SDK v0.2.1 (closed [#11](https://github.com/rhuss/openshell-sdk-go/issues/11)). FromExisting is a cc-deck client concept, not an SDK concern (closed [#12](https://github.com/rhuss/openshell-sdk-go/issues/12)). SandboxSuspended never existed in the proto (closed [#13](https://github.com/rhuss/openshell-sdk-go/issues/13)).

## remote-gateway-security

- **Source**: triage
- **Date**: 2026-06-30
- **Reference**: PR #7 (075-openshell-sdk-migration)
- **Summary**: Three independent bot reviewers (CodeRabbit, Copilot, Devin) flagged that non-localhost gateway connections default to `NoAuth` with no TLS, protected only by a log warning. This is a security concern for production deployments where the gateway runs on a remote host.

> - **CodeRabbit**: `client.go:49` "Fail closed for remote gateways without TLS. Provider, exec, and file-transfer traffic can be exposed in transit."
> - **Copilot**: `client.go:49` "Non-localhost gRPC connections default to no authentication."
> - **Devin**: `client.go:49` "Non-localhost gRPC connections default to no authentication."
>
> Possible approaches: require TLS for non-localhost, add an explicit `--insecure` opt-in flag, or add auth configuration to `GatewayConfig`.

## waitready-timeout

- **Source**: triage
- **Date**: 2026-06-30
- **Reference**: PR #7 (075-openshell-sdk-migration)
- **Summary**: Two bot reviewers (Devin, Copilot) flagged that `WaitReady` inherits the caller's context deadline, which may have no timeout. The old polling loop enforced a 60s hardcoded timeout. If the caller passes `context.Background()`, sandbox creation could hang indefinitely on a stuck provisioning.

> - **Devin**: `openshell.go:335` "WaitReady replaces custom polling loop with different timeout semantics."
> - **Copilot**: `openshell.go:335` "No longer a default create timeout."
>
> Fix: wrap the WaitReady call in a `context.WithTimeout(ctx, 60*time.Second)` to preserve the old behavior.
