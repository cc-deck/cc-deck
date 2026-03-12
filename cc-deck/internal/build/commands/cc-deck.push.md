---
description: Push the built container image to a registry
---

## User Input

$ARGUMENTS

## Outline

Push the container image built by `/cc-deck.build` to a container registry.

### Step 1: Read the manifest

Read `cc-deck-build.yaml` and extract `image.name` and `image.tag` (default: `latest`).

### Step 2: Verify the image exists

```bash
podman images <image-name>:<tag> --format '{{.ID}}'
```

If the image doesn't exist locally, stop and suggest running `/cc-deck.build` first.

### Step 3: Push the image

```bash
podman push <image-name>:<tag>
```

If the user provided a registry override in the input, tag and push to that registry:

```bash
podman tag <image-name>:<tag> <registry>/<image-name>:<tag>
podman push <registry>/<image-name>:<tag>
```

### Step 4: Handle push failures

Common issues:
- **Auth failure**: Suggest `podman login <registry>` and retry
- **Image not found**: Suggest running `/cc-deck.build` first
- **Network error**: Retry once, then report the error

### Step 5: Report

Show the pushed image reference and registry URL.

### Key Rules

- Detect the container runtime (podman preferred, docker fallback)
- Never push without confirming the image exists locally first
- If auth fails, guide the user through `podman login` rather than asking for credentials
