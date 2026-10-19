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

## ~~multi-file-credential-support~~ (resolved)

- **Status**: Resolved 2026-09-09
- **Resolution**: Harness profiles (087, PR #37) made `InjectSSH` and `InjectOpenShell` upload every entry of `FileCredentials` with per-file destinations (T041).
- **Source**: deep-review
- **Date**: 2026-07-08
- **Reference**: 079-credential-transport
- **Summary**: `MergeCredentials` collects multiple file credentials but `InjectSSH` only processes the first one. Current agents use at most one file credential, but multi-agent scenarios could surface this limitation.

> Pre-existing design in the credential package. `FileCredentials` (plural) is collected during merge, but transport functions consume `FileCredential` (singular, `files[0]`). If a future agent declares multiple file credentials, only the first would be transported.

### openshell-policy-tempfile-cleanup

- **Source**: deep-review
- **Date**: 2026-09-09
- **Reference**: pre-existing, surfaced during 087-harness-profiles review, `cc-deck/internal/ws/openshell.go` (`resolveSandboxConfig`, `Create`)
- **Summary**: The temp policy file extracted from an OCI image (`cc-deck-policy-*.yaml`) is removed by a `defer` in `Create` that sits after other error checks. Any future early return between `resolveSandboxConfig` and that `defer`, or a new caller of `resolveSandboxConfig`, leaks the file.

> Move the cleanup responsibility into `resolveSandboxConfig` (return bytes instead of a path, or register the removal immediately after creation) so the lifetime does not depend on the caller's control flow.

### profile-wrappers-podman-backends

- **Source**: manual
- **Date**: 2026-09-09
- **Reference**: 087-harness-profiles (PR #37), `cc-deck/internal/ws/container.go`, `cc-deck/internal/ws/compose.go`, `cc-deck/internal/profile/sync.go` (`Provision`, `Target`)
- **Summary**: Profile wrappers are provisioned for local, SSH and OpenShell workspaces but not for the podman container and compose backends. Images built by `cc-deck build` already put `~/.local/share/cc-deck/bin` on `PATH` (`05-shell-finalize.tmpl`), so only the delivery step is missing.

> Implement a `profile.Target` adapter over `podman cp` (Upload) and `podman exec` (Run, Home, Agents) and call `profile.Provision` from `ContainerWorkspace.Create` and `ComposeWorkspace.Create` after credential injection, mirroring `ws/profile_target.go` for SSH and OpenShell. Credential files land under `~/.config/cc-deck/profiles/<name>/` inside the container; env-sourced keys are already carried by `InjectContainer` via podman secrets, so the wrapper's `${NAME:?}` check works unchanged. Add `sync --workspace` support for container workspaces and a provision test with the in-memory target.

### profile-wrappers-kubernetes-backend

- **Source**: manual
- **Date**: 2026-09-09
- **Reference**: 087-harness-profiles (PR #37), `cc-deck/internal/ws/k8s_deploy.go`, `cc-deck/internal/profile/claude.go` (`Render` rejects `secret` sources)
- **Summary**: The Kubernetes deploy backend gets no profile wrappers. Two gaps: no delivery step (`kubectl cp` and `kubectl exec` target), and profiles for this backend use `{secret: <k8s-secret>}` sources, which `Render` refuses because the value is not a host env var or file.

> Map a `secret` source to the mounted path or env var the pod exposes (`InjectK8s` already produces Secret data and volume mounts), so the translator can render `${NAME:?}` or a file check against the in-pod location. Then add a `Target` over `kubectl cp`/`kubectl exec` and call `profile.Provision` from `K8sDeployWorkspace.Create`. Until then Kubernetes keeps the single workspace-level profile selected by `default_profile`.
### endpoint-name-dual-meaning

- **Source**: deep-review
- **Date**: 2026-09-07
- **Reference**: 087-workspace-sharing-endpoint
- **Summary**: `StaticEndpoint.Name()` returns the implementation kind while `EndpointRef.Name` carries the configured endpoint name, so "name" means two things in one package.

> When the Endpoint interface gains a second implementation, implementors will need to know that Name() identifies the implementation, not the user's endpoint. Consider renaming to Kind().

### dead-invitation-set-api

- **Source**: deep-review
- **Date**: 2026-09-07
- **Reference**: 087-workspace-sharing-endpoint
- **Summary**: `BuildInvitations` and `InvitationSet` have no production caller and are kept alive only by their own tests.

> Tests covering dead code are worse than no tests because they provide false coverage confidence. Pre-existing, not introduced by this feature.

### observation-save-errors-discarded

- **Source**: deep-review
- **Date**: 2026-09-07
- **Reference**: 087-workspace-sharing-endpoint
- **Summary**: recordProbe and reconcileLocked both discard the store.Save error, so a disk failure silently loses a probe observation.

> Deliberate: an observation must not become a state change, and cleanup must not be blocked by persistence failure. But it means FR-047's "MUST update" is best-effort under disk error, with no signal to the user.

### status-cost-two-subprocess-spawns

- **Source**: deep-review
- **Date**: 2026-09-07
- **Reference**: 087-workspace-sharing-endpoint
- **Summary**: Every `ws status` mints and revokes a Zellij token, so status will never be sub-second even on a healthy system.

> An accepted trade for storing no standing secret. Worth revisiting if status latency becomes a complaint, for example by reusing a short-lived credential within one command.

### e2e-coverage-lost-without-zellij

- **Source**: deep-review
- **Date**: 2026-09-07
- **Reference**: 087-workspace-sharing-endpoint
- **Summary**: The three end to end tests skip when zellij is absent, and they are the only coverage of the real network stack and real binary.

> These caught the web_client_id disagreement that every offline fake had missed. CI without Zellij installed silently loses that class of coverage.

### probe-credential-in-memory-lifetime

- **Source**: deep-review
- **Date**: 2026-09-07
- **Reference**: 087-workspace-sharing-endpoint
- **Summary**: The login body holding the probe credential stays reachable through bytes.NewReader longer than strictly necessary.

> Acceptable for a short-lived CLI. Redaction coverage on diagnostics was confirmed complete; this is about in-memory residency, not disclosure.

### configurable-public-dns-resolver

- **Source**: deep-review
- **Date**: 2026-09-07
- **Reference**: 087-workspace-sharing-endpoint
- **Summary**: The DNS second opinion is hardcoded to Cloudflare at 1.1.1.1 with no way to change or disable it.

> Now documented, but on a restrictive or audited network a user may want to choose their own second-opinion resolver or turn the query off entirely.

### per-profile-remote-secret-scope

- **Source**: deep-review
- **Date**: 2026-10-12
- **Reference**: 087-harness-profiles, `cc-deck/internal/profile/sync.go` EnvCredentials, `cc-deck/internal/ws/ssh.go`
- **Summary**: On SSH workspaces every env-sourced profile secret lands in the shared `~/.config/cc-deck/credentials.env`, so each profiled session (and its subprocesses and MCP servers) sees every other profile's token, and the tokens stay at rest on the remote host.

> Writing each env-sourced value to `~/.config/cc-deck/profiles/<name>/<field>` (0600) and rendering the remote wrapper with the file form would scope each secret to its own wrapper. Related hardening: create remote credential files under `umask 077` and send content over stdin instead of argv, and single-quote values in `credentials.env` instead of Go `%q`.

### env-override-warning-in-validate

- **Source**: deep-review
- **Date**: 2026-10-12
- **Reference**: 087-harness-profiles, `cc-deck/internal/profile/sync.go` envOverrideWarnings
- **Summary**: FR-007a's warning for an `env` key that overrides another auth kind is only emitted by `sync` and `Provision`, not by `cc-deck config validate`, because the config package cannot import the translators.

> A validation hook that the profile package registers with config (or an `AuthKindOf(backend, auth)` helper plus an exported auth variable table) would let `config validate` report it too. Either that, or reword FR-007a to say sync warns.

### derived-color-palette-shift

- **Source**: deep-review
- **Date**: 2026-10-12
- **Reference**: 087-harness-profiles, `cc-deck/internal/profile/color.go`
- **Summary**: Replacing the palette changes the derived color of every profile without an explicit `color:`; only profiles added afterwards get a stored, stable color.

> The feature is unreleased, so this only affects users of the branch. If it ships after any release with the old palette, a release note or a `sync` hint listing `color:` values that keep current colors would avoid a surprise recolor.

### anthropic-auth-token-precedence

- **Source**: deep-review
- **Date**: 2026-10-12
- **Reference**: 087-harness-profiles, `cc-deck/internal/profile/claude.go` AuthVars
- **Summary**: `ANTHROPIC_AUTH_TOKEN` is grouped with `ANTHROPIC_API_KEY` under the api_key kind (research R16), so an api_key profile does not clear an inherited `ANTHROPIC_AUTH_TOKEN`, which Claude Code prefers over the API key.

> Computing the unset list from the profile's actual source variable, or giving `ANTHROPIC_AUTH_TOKEN` its own kind owned only by a profile that sources it, would close the gap for gateway tokens exported in the calling shell.

### codex-client-env-trust

- **Source**: deep-review
- **Date**: 2026-10-15
- **Reference**: 089-codex-daemon-pane-attribution, `cc-deck/internal/cmd/codex_attribution.go` clientEnvTarget
- **Summary**: A parseable `client_env.ZELLIJ_PANE_ID` wins over everything, without checking that a live Codex client of this user sits in that pane.

> Once Codex forwards client environment (openai/codex#44902), cross-check client_env against procinfo (a live, bindable client with that pane and session) before trusting it.

### codex-non-shell-daemon-clients

- **Source**: deep-review
- **Date**: 2026-10-15
- **Reference**: 089-codex-daemon-pane-attribution, discovery in `resolveCodex`
- **Summary**: Discovery assumes every daemon client is a visible local `codex` process; a desktop app or socket client that is not would let its events bind to the only TUI in the same directory.

> Document the assumption and revisit when Codex exposes other daemon clients.

### codex-same-dir-resume-row

- **Source**: deep-review
- **Date**: 2026-10-15
- **Reference**: 089-codex-daemon-pane-attribution, SessionStart fallback
- **Summary**: Resuming a session in a second client in the same directory while the original client runs keeps the original row; a SessionStart `source` field (resume vs compact) in the payload would let the hook tell the cases apart.

> Parsing the SessionStart source would close the last documented wrong-row window without weakening US2-2.

### pane-map-lock-latency

- **Source**: deep-review
- **Date**: 2026-10-15
- **Reference**: 089-codex-daemon-pane-attribution, `withPaneMapLock`
- **Summary**: The 2 s lock wait plus F_FULLFSYNC on Darwin sits in the blocking hook path; delivering before recording, or a shorter timeout, would cut tail latency.

> Not a defect at the measured load (about 10 ms per serialized write), but invisible today.

### hook-autosave-and-darwin-seams

- **Source**: deep-review
- **Date**: 2026-10-15
- **Reference**: 089-codex-daemon-pane-attribution, `deliverHookPayload`, `procinfo_darwin.go`
- **Summary**: The autoSave gating and the Darwin ListUser and Ancestor error branches have no test seam.

> A `hookAutoSave` variable and injectable sysctl readers would make them testable.

### background-cleanup-lifecycle-gc

- **Source**: deep-review
- **Date**: 2026-10-19
- **Reference**: 090-claude-bg-pane-attribution, `bg-job-panes.json` and per-job lock files
- **Summary**: Cleanup ownership must not expire by age while sidebar rows can remain visible. Jobs killed without SessionEnd can therefore leave ledger entries and empty lock files behind.

> Add lifecycle-aware garbage collection that verifies worker retirement and conditionally releases any owned row before forgetting it. Lock-file cleanup must avoid replacing an inode still used by waiting hooks. Do not restore a TTL that silently loses active row ownership.
