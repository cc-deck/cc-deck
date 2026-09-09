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

### hook-profile-env-cache

- **Source**: deep-review
- **Date**: 2026-09-09
- **Reference**: 087-harness-profiles (PR #37), `cc-deck/internal/cmd/hook.go`
- **Summary**: `cc-deck hook` loads and parses `config.yaml` on every hook event when `CC_DECK_PROFILE` is set, only to resolve the profile color and icon. Hook events fire on every tool call, so this is disk I/O plus YAML parsing on the hot path. The wrapper already knows both values at generation time.

> Two reviewers (architecture, production) suggested exporting `CC_DECK_PROFILE_COLOR` and `CC_DECK_PROFILE_ICON` from the generated wrapper and reading them in the hook, falling back to the config load only when they are absent. Zero cost per event, and the hook payload contract stays unchanged. Color edits would then take effect on the next `config profile sync` instead of the next hook event, which the guide should say.

### pane-map-write-race

- **Source**: deep-review
- **Date**: 2026-09-09
- **Reference**: pre-existing, surfaced during 087-harness-profiles review, `cc-deck/internal/cmd/hook.go` (`loadPaneMap`, `savePaneMap`)
- **Summary**: The pane-map cache (`pane-map.json`) is read, pruned and rewritten on every hook event with no locking and a non-atomic `os.WriteFile`. Two concurrent hook processes (two sessions firing at once) can clobber each other's entries, and a reader can observe a partially written file.

> The cache self-heals on the next event, so the impact is a brief pane-id miss and a dropped sidebar update. Fix options: write to a temp file in the same directory and `os.Rename` (removes partial reads), plus a lockfile or `flock` for full correctness. If the race is accepted as benign, document it in a comment.

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
