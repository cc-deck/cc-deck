# OCI Policy Extraction

## Problem

When creating an OpenShell sandbox with `cc-deck ws new --type openshell`, the openshell CLI needs the policy file on the host to pass via `--policy`. Currently this fails because there is no mechanism to locate the policy file at runtime.

The policy is baked into the image at `/etc/openshell/policy.yaml` during `podman build` (via `COPY` in the Containerfile). But `ws new` cannot rely on the host build directory existing, because:

- The image may have been built on a different machine
- The image may have been pulled from a registry on a Kubernetes node
- The build directory is a build-time concern, not a runtime concern

## Proposed Solution

Use `go-containerregistry` to extract the policy file directly from the OCI image, without pulling the full image. OCI registries serve layers as individual blobs, so we only need to download the specific layer containing `policy.yaml`.

### Two-phase approach

**Phase 1: Build-time labeling**

After `podman build`, use go-containerregistry to:
1. Load the built image from the local daemon
2. Walk layers in reverse to find the one containing `etc/openshell/policy.yaml`
3. Record its digest as a label: `dev.cc-deck.policy-layer=sha256:...`
4. Write the labeled image back to the local daemon

This happens in `cc-deck build run --target openshell`, after the podman build succeeds but before any push. Labels are config-only metadata, so no extra layer is created.

**Phase 2: Runtime extraction**

At `ws new --type openshell` time:
1. Parse the image reference
2. Fetch the manifest and config from the registry (just JSON, a few KB)
3. Read the `dev.cc-deck.policy-layer` label
4. If label exists: fetch that single layer blob, extract `etc/openshell/policy.yaml`
5. If label missing or file not in that layer: fall back to reverse layer scan (iterate all layers until found)
6. Write the extracted policy to a temp file, pass to `openshell sandbox create --policy`

The fallback ensures compatibility with images built before this feature or by other tools.

### Library choice

`github.com/google/go-containerregistry` (the library behind the `crane` CLI):
- Pure Go, no dependency on podman/docker at runtime
- Works with local daemon images and remote registries
- Handles auth via Docker/podman credential helpers automatically
- `mutate` package for label injection without creating extra layers
- Already widely used in the Kubernetes ecosystem (ko, Tekton, Knative)

## Implementation scope

### New package: `internal/oci/`

- `ExtractFileFromImage(imageRef, filePath string) ([]byte, error)`: extracts a file from an image, using label hint then fallback
- `FindLayerContaining(image v1.Image, filePath string) (v1.Hash, error)`: walks layers to find the one with the file
- `AddLabel(imageRef, key, value string) error`: adds a label to a local image

### Changes to existing code

- `cmd/build.go` (`build run`): after successful podman build for openshell target, call `FindLayerContaining` + `AddLabel` to stamp the policy layer digest
- `cmd/ws.go` (`ws new`): for openshell workspaces, call `ExtractFileFromImage` to get the policy, write to temp file, pass to `CreateSandbox`
- `go.mod`: add `github.com/google/go-containerregistry`
- Remove the host-path auto-resolution added earlier (the `filepath.Join(projectRoot, ".cc-deck", "setup", ...)` approach)

### Label convention

- Key: `dev.cc-deck.policy-layer`
- Value: layer diff ID as `sha256:<hex>` (matches the format from `go-containerregistry`)
- Namespace follows OCI reverse-DNS convention

## Edge cases

- Image on a registry that requires auth: go-containerregistry uses the standard Docker/podman credential chain
- Image only available locally (not pushed): works via `daemon.Image()` for local access
- Image built without the label (older cc-deck or third-party build): fallback layer scan handles this
- Multiple files named `policy.yaml` in different layers: reverse scan finds the topmost (most recent COPY wins)
- Registry unreachable: error with clear message suggesting `--policy` flag as manual override

## Not in scope

- Signing or verifying the policy file (deferred to supply chain security spec)
- Caching extracted policies locally (the file is small, extraction is fast)
- Supporting non-OCI image formats
