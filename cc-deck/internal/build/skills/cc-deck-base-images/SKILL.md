---
name: cc-deck.base-images
description: Check known sources for base image updates and maintain base-images.yaml
---

# Base Image Discovery

Check upstream sources for new or updated base images and help maintain `base-images.yaml`.

## Invocation

- `/cc-deck.base-images` -- check all entries for updates, report findings
- `/cc-deck.base-images update` -- check and offer to apply updates to `base-images.yaml`

## Steps

### 1. Load current state

Read `base-images.yaml` from the repo root. If `.base-images-digests.json` exists, load the last-known digests.

### 2. Check registry digests

For each entry in `base-images.yaml`, run:

```bash
skopeo inspect --raw docker://<ref> 2>/dev/null | jq -r '.digest // .config.digest // "unknown"'
```

If `skopeo` is not available, fall back to:

```bash
podman inspect --format '{{.Digest}}' docker://<ref> 2>/dev/null
```

Compare against stored digests. Report changes:
- "nvidia-upstream: digest changed (sha256:old... -> sha256:new...)"
- "nvidia-upstream: unchanged"
- "rh-ubi-openshell: image not found (may have been renamed or removed)"

### 3. Check upstream repos for new images

Check these GitHub sources for new base image references:

```bash
# OpenShell upstream releases
gh api repos/NVIDIA/OpenShell-Community/releases/latest --jq '.tag_name'

# Red Hat agentic starter kits -- look for Containerfile FROM lines
gh api repos/red-hat-data-services/agentic-starter-kits/contents/ --jq '.[].name' 2>/dev/null
```

### 4. Scan known registries

Check for new tags in known registries:

```bash
# NVIDIA OpenShell sandbox images
skopeo list-tags docker://ghcr.io/nvidia/openshell-community/sandboxes/base 2>/dev/null | jq -r '.Tags[]'

# AIPCC images (if registry exists)
skopeo list-tags docker://quay.io/aipcc/openshell-base 2>/dev/null | jq -r '.Tags[]'
```

Report any tags not currently tracked in `base-images.yaml`.

### 5. Report findings

Present a summary:
- **Digest changes** for tracked entries
- **New images** found in upstream repos or registries
- **Stale entries** that could not be found

### 6. Apply updates (if `update` argument)

If the user invoked with `update`:
1. Show proposed changes to `base-images.yaml`
2. Ask for confirmation before writing
3. Update `.base-images-digests.json` with current digests
4. Suggest running `make test-images` to validate the changes
