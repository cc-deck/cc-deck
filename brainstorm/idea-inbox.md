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

### openshell-error-messages

- **Source**: manual testing
- **Date**: 2026-07-27
- **Reference**: `ws new` and `ws delete` error paths
- **Summary**: OpenShell error messages expose raw gRPC transport errors without identifying the root cause. When the gateway tunnel is not running, the user sees `Unavailable: connection error: desc = "transport: Error while dialing: dial tcp 127.0.0.1:17670: connect: connection refused"` instead of a clear message like "OpenShell gateway is not reachable. Is the tunnel running?" The SDK client connection errors should be caught and translated to actionable messages that name the likely cause and suggest a fix.

> Two concrete cases:
> - `ws new` fails during credential provider creation with a raw gRPC error. Should say "Gateway not reachable" and suggest checking the tunnel.
> - `ws delete` fails with the same raw error. Already improved with `--force` hint, but the root cause message is still opaque.
>
> The fix belongs in `internal/openshell/client.go` (or a wrapper) where `ensureClient` connects. Catch `connection refused` on localhost:17670 and wrap with a human-readable explanation.

### openshell-policy-extraction-local-image

- **Source**: manual testing
- **Date**: 2026-07-27
- **Reference**: `ws new --type openshell --image openshell-test:latest`
- **Summary**: Policy extraction fails with a confusing error when the image name has no registry prefix. The message `Get "https:///v2/": http: no Host in request URL` comes from trying to query a remote registry with an empty host. The image `openshell-test:latest` is a local-only image (no registry), so remote lookup makes no sense. The extraction code should recognize registry-less image names and skip the remote fallback, or produce a clearer error like "Image 'openshell-test:latest' not found locally and has no registry to query remotely. Push it to a registry or use --policy to provide the policy file."

> The OCI extraction code in `internal/oci/` tries local podman first, then falls back to a remote registry. The remote fallback constructs a URL without a host when the image reference has no registry component, causing the `no Host in request URL` error.

### multi-file-credential-support

- **Source**: deep-review
- **Date**: 2026-07-08
- **Reference**: 079-credential-transport
- **Summary**: `MergeCredentials` collects multiple file credentials but `InjectSSH` only processes the first one. Current agents use at most one file credential, but multi-agent scenarios could surface this limitation.

> Pre-existing design in the credential package. `FileCredentials` (plural) is collected during merge, but transport functions consume `FileCredential` (singular, `files[0]`). If a future agent declares multiple file credentials, only the first would be transported.
