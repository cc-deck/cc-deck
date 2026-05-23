# Research: OCI Policy Extraction

## R1: go-containerregistry for Local Daemon + Registry Access

**Decision**: Use `github.com/google/go-containerregistry` for all OCI image operations.

**Rationale**: Pure Go library with no runtime dependency on podman/docker binaries. Supports both local daemon images (via `daemon` package) and remote registries (via `remote` package). Handles auth via standard credential helpers automatically. The `mutate` package supports config-only label injection without creating new layers.

**Alternatives considered**:
- `podman inspect` via exec.Command: Would require podman installed at runtime, cannot fetch individual layers from remote registries, and adds process overhead. Rejected.
- `containers/image`: Heavier library designed for full image transport (pull/push), more than needed for single-file extraction. Rejected.
- `crane` CLI wrapper: Adds binary dependency and process overhead. The Go library is more direct. Rejected.

## R2: Layer Scanning Strategy

**Decision**: Walk layers in reverse order (topmost first) to find the file, matching OCI overlay filesystem semantics.

**Rationale**: OCI images stack layers where later layers override earlier ones. A `COPY` in a later Dockerfile stage overwrites the same path from an earlier stage. Reverse scanning finds the "winning" file first, consistent with how the container runtime resolves file paths.

**Alternatives considered**:
- Forward scan (bottom-up): Would find the oldest version of the file, potentially incorrect if overridden by a later COPY. Rejected.
- Only check the last layer: Too brittle. The file might be in any layer depending on Containerfile structure. Rejected.

## R3: Label Injection Approach

**Decision**: Use `mutate.Config` to add labels to the image config without creating new layers. Write the mutated image back to the local daemon via `daemon.Write`.

**Rationale**: Labels are stored in the image config JSON, not in layers. Mutating the config changes only the config blob and manifest (which references it). No new layer is created, so the image size is unaffected. The `daemon.Write` function handles writing back to the local podman/docker daemon.

**Alternatives considered**:
- `podman label` command: Does not exist as a direct command. Would need to use `buildah config --label` which adds a dependency on buildah. Rejected.
- Store label in a separate metadata file: Breaks the self-contained nature of the image. Rejected.

## R4: Local Daemon Compatibility with Podman

**Decision**: Use `daemon.Image` and `daemon.Write` which connect via the Docker-compatible API socket. Podman exposes this socket at the user's XDG runtime directory.

**Rationale**: `go-containerregistry`'s daemon package uses the Docker API (via `DOCKER_HOST` env var or default socket). Podman provides Docker-compatible API access via `podman system service` or the always-running socket at `$XDG_RUNTIME_DIR/podman/podman.sock`. The project already uses podman exclusively.

**Alternatives considered**:
- Direct podman socket path: Hardcoding the socket path is fragile. Using the DOCKER_HOST env var or letting the library auto-detect is more portable. Accepted as fallback.

## R5: Temp File Management for Extracted Policy

**Decision**: Use `os.CreateTemp` to write the extracted policy bytes, then pass the path to `CreateSandbox`. Clean up via `defer os.Remove` in the calling function.

**Rationale**: The openshell `CreateSandbox` function expects a file path for the `--policy` flag. Writing to a temp file and cleaning up after sandbox creation (success or failure) is the simplest approach. The policy file is small (typically <10KB), so temp file overhead is negligible.

**Alternatives considered**:
- Named pipe / stdin: The openshell CLI expects a file path, not stdin. Would require changes to the openshell binary. Rejected.
- Write to XDG cache: Adds complexity for cache invalidation. Not worth it for a small, cheap-to-extract file. Rejected.

## R6: Existing Code to Modify

**Decision**: Modify `internal/ws/openshell.go` (`resolveSandboxConfig`) to call the new OCI extraction when no explicit policy path is set. Modify `internal/cmd/build.go` (`runOpenShellBuild`) to add label stamping after successful build.

**Key files**:
- `cc-deck/internal/ws/openshell.go:104-127` - `resolveSandboxConfig()` currently reads `def.Policy` directly
- `cc-deck/internal/cmd/build.go:337-391` - `runOpenShellBuild()` handles build and push
- `cc-deck/internal/cmd/build.go:833-859` - `refreshOpenShellPolicy()` assembles policy
- `cc-deck/internal/openshell/client.go:95` - `CreateSandbox()` expects policy file path
