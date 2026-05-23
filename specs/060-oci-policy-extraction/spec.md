# Feature Specification: OCI Policy Extraction

**Feature Branch**: `060-oci-policy-extraction`
**Created**: 2026-05-23
**Status**: Draft
**Input**: User description: "Extract policy files from OCI images at runtime for OpenShell sandbox creation"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Runtime Policy Extraction (Priority: P1)

A developer runs `cc-deck ws new --type openshell` to create an OpenShell sandbox. The system automatically extracts the policy file from the OCI image and passes it to the openshell CLI, without requiring the user to locate or provide the policy file manually.

**Why this priority**: This is the core use case. Without runtime extraction, OpenShell sandbox creation fails entirely when the build directory is not present on the host.

**Independent Test**: Can be fully tested by running `cc-deck ws new --type openshell` with an image that contains a baked-in policy file, and verifying the sandbox starts successfully with the correct policy applied.

**Acceptance Scenarios**:

1. **Given** an OCI image with a policy file at `/etc/openshell/policy.yaml` and a `dev.cc-deck.policy-layer` label, **When** the user runs `cc-deck ws new --type openshell`, **Then** the system extracts the policy from the labeled layer and passes it to the openshell sandbox creation.
2. **Given** an OCI image with a policy file but without the `dev.cc-deck.policy-layer` label, **When** the user runs `cc-deck ws new --type openshell`, **Then** the system falls back to scanning all layers in reverse order, finds the policy file, and passes it to sandbox creation.
3. **Given** an OCI image that does not contain a policy file, **When** the user runs `cc-deck ws new --type openshell`, **Then** the system reports a clear error message suggesting the user provide a policy file manually via `--policy`.

---

### User Story 2 - Build-Time Label Stamping (Priority: P2)

A developer runs `cc-deck build run --target openshell` to build the OpenShell image. After the build completes, the system automatically identifies which image layer contains the policy file and records its digest as a label on the image. This enables fast policy extraction at runtime.

**Why this priority**: Label stamping is an optimization that makes runtime extraction faster by avoiding a full layer scan. The system works without it (via fallback), but it reduces extraction time for labeled images.

**Independent Test**: Can be fully tested by building an openshell image, then inspecting the image labels to verify `dev.cc-deck.policy-layer` is present with a valid layer digest.

**Acceptance Scenarios**:

1. **Given** a successful `podman build` for the openshell target, **When** the build completes, **Then** the system identifies the layer containing `/etc/openshell/policy.yaml` and adds the label `dev.cc-deck.policy-layer` with the layer's diff ID.
2. **Given** a build where the policy file does not exist in the image, **When** the build completes, **Then** the system logs a warning but does not fail the build.

---

### User Story 3 - Backward Compatibility with Unlabeled Images (Priority: P3)

A developer uses an image that was built before the labeling feature was introduced, or built by a third-party tool. The system still extracts the policy file by scanning image layers, ensuring older images continue to work.

**Why this priority**: Ensures a smooth upgrade path. Users should not need to rebuild all existing images after adopting this feature.

**Independent Test**: Can be fully tested by using an image without the `dev.cc-deck.policy-layer` label and verifying that `ws new` still successfully extracts the policy file.

**Acceptance Scenarios**:

1. **Given** an OCI image without the `dev.cc-deck.policy-layer` label but containing `/etc/openshell/policy.yaml`, **When** the user runs `cc-deck ws new --type openshell`, **Then** the system scans layers in reverse order, finds the policy file in the topmost layer that contains it, and uses it for sandbox creation.

---

### Edge Cases

- What happens when the image registry requires authentication? The system uses the standard credential chain (podman/Docker credential helpers) to authenticate automatically.
- What happens when the image is only available locally (not pushed to a registry)? The system accesses the image via the local container daemon.
- What happens when multiple layers contain a file at the same path? The reverse layer scan finds the topmost (most recent) layer, matching how container filesystems resolve overlapping paths.
- What happens when the registry is unreachable? The system reports a clear error suggesting the user provide the policy file manually via `--policy`.
- What happens when the labeled layer digest does not match any layer in the image (stale label)? The system falls back to the full reverse layer scan.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST extract a specified file from an OCI image given an image reference and file path.
- **FR-002**: System MUST first check the image config for a `dev.cc-deck.policy-layer` label and, if present, attempt to extract the file from the labeled layer.
- **FR-003**: System MUST fall back to a reverse layer scan when the label is missing, the labeled layer does not contain the file, or the label digest is invalid.
- **FR-004**: System MUST resolve image references for both local daemon images and remote registry images.
- **FR-005**: System MUST use the standard container credential chain for registry authentication, without requiring additional configuration.
- **FR-006**: System MUST add the `dev.cc-deck.policy-layer` label to the image after a successful openshell build, recording the diff ID of the layer containing the policy file.
- **FR-007**: System MUST NOT create additional image layers when adding the label (config-only metadata change).
- **FR-008**: System MUST write the extracted policy to a temporary file and pass its path to the openshell sandbox creation command.
- **FR-009**: System MUST clean up extracted temporary policy files after sandbox creation completes or fails.
- **FR-010**: System MUST report a clear error when no policy file is found in any layer, suggesting the `--policy` flag as a manual alternative.
- **FR-011**: System MUST remove the existing host-path auto-resolution approach for locating policy files, replacing it with OCI extraction.

### Key Entities

- **OCI Image**: A container image identified by a reference (e.g., `registry/repo:tag` or `sha256:digest`). Contains a manifest, config, and ordered layers.
- **Image Layer**: A tar archive within an OCI image. Layers are stacked in order, with later layers overriding earlier ones for files at the same path.
- **Image Label**: Key-value metadata stored in the image config. Used here to record which layer contains the policy file.
- **Policy File**: A YAML configuration file at `/etc/openshell/policy.yaml` that defines the security policy for an OpenShell sandbox.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users can create OpenShell sandboxes without needing the original build directory on the host.
- **SC-002**: Policy extraction from a labeled image completes within 5 seconds over a typical network connection, by fetching only the necessary layer blob rather than the full image.
- **SC-003**: Images built before this feature (without the label) continue to work for sandbox creation via the fallback layer scan.
- **SC-004**: Build-time label stamping adds less than 2 seconds to the openshell build process.
- **SC-005**: All extraction paths (labeled, fallback, error) are covered by automated tests.

## Assumptions

- The target OCI image follows the standard OCI image specification (manifest v2, schema 2).
- The policy file path within the image is always `/etc/openshell/policy.yaml` and is not configurable in this iteration.
- The container credential chain (podman credential helpers, Docker config.json) is configured correctly on the host for registries that require authentication.
- The `go-containerregistry` library provides sufficient functionality for both local daemon access and remote registry interaction.
- Image label injection via config mutation does not invalidate image signatures (this spec does not cover image signing, which is deferred to a separate supply chain security spec).
- The existing `ws new` command already has access to the image reference for the openshell workspace type.
