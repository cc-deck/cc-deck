# Brainstorm: Two-Pass Binary Probing for OpenShell Policy

**Date:** 2026-05-24
**Status:** active

## Problem Framing

The current policy binary resolution uses a hardcoded "well-known paths" table to guess where tools are installed in the OpenShell sandbox image. This is fragile: paths vary by base image, install method, and user configuration. Wrong guesses either leave tools unable to reach their package registries (403 from the OpenShell supervisor) or require manual maintenance of the table for every new tool.

Additionally, tools like pip and cargo create new binaries at runtime (virtual environments, toolchain installs) that did not exist when the image was built. The static table cannot account for these.

OpenShell enforces binary paths strictly: if `policy.binaries` is empty, all processes are denied access to that endpoint. If a binary path does not match the requesting process, access is denied. Glob patterns are supported (`/sandbox/**/bin/pip`).

## Approaches Considered

### A: Two-Pass Build with Image Probing (Chosen)

Build the image twice. The first pass produces the image with tools installed but without binary restrictions in the policy. Probe the built image for actual binary paths using `which` (and `find` as fallback). Regenerate the policy with probed paths plus tool-specific globs for runtime-created binaries. Rebuild with the corrected policy (only the policy COPY layer and subsequent layers rebuild from cache).

- Pros: Exact paths from the actual image. No guesswork. Handles any install method. Globs cover runtime-created binaries.
- Cons: Adds one rebuild step (fast due to layer caching). Slightly longer build time.

### B: Keep Well-Known Paths Table

Maintain the static table and expand it as needed.

- Pros: Simple. No build changes.
- Cons: Guesswork. Requires manual updates. Cannot handle runtime-created paths. Already broken for venvs and custom installs.

### C: Probe at ws-new Time

Extract the policy from the image, probe the image for binaries, patch the policy before passing to CreateSandbox.

- Pros: No double build. Policy always matches the running image.
- Cons: The policy baked into the image is incomplete (no binaries). Images are not self-contained. Breaks the OCI extraction optimization from spec 060.

## Decision

Approach A: Two-pass build with image probing.

The double build cost is minimal since all layers except the policy COPY are cached. This produces a self-contained image with a correct, complete policy. The well-known paths table is removed entirely.

This applies only to OpenShell targets. Container and SSH targets do not generate policies and are unaffected.

## Key Requirements

1. **First pass builds the image normally** with a policy that has no `binaries` on any entry (endpoints only). The OpenShell supervisor inside the image is not running during build, so missing binaries do not matter.

2. **Probe step** creates a temporary container from the built image and runs `which <tool>` for each tool in the manifest's `tools` section and each tool referenced in `match.tools` of matched policy components. Falls back to `find / -name <tool> -type f -executable` for tools not found via `which`. Collects the actual filesystem paths.

3. **Tool-specific globs** are added alongside probed paths for tools known to create binaries at runtime:
   - Python: `/sandbox/**/bin/pip`, `/sandbox/**/bin/pip3`, `/sandbox/**/bin/uv`, `/sandbox/**/bin/python`, `/sandbox/**/bin/python3`
   - Rust: `/sandbox/.rustup/toolchains/*/bin/cargo`, `/sandbox/.rustup/toolchains/*/bin/rustc`
   - Node: `/sandbox/**/node_modules/.bin/*` (for npx-installed tools)
   - Go: `/sandbox/go/bin/*` (for go install'd tools)

4. **Policy regeneration** replaces the binaries-less policy with one containing probed paths and globs. Each policy component's binaries field is populated from the probe results for its `match.tools`.

5. **Second pass** rebuilds the image with the updated policy. Only the policy COPY layer and any subsequent layers rebuild (all tool installation layers are cached).

6. **Well-known paths table removed** from `policy_binaries.go`. The `resolveBinaries()` function is replaced by the probe-based approach.

7. **Label stamping** (`oci.StampPolicyLabel`) still runs after the second build, labeling the final image.

## Open Questions

- Should the probe also check for interpreter symlinks (e.g., `python3` -> `python3.12`)? The resolved path may differ from the `which` result if the supervisor checks the real path after symlink resolution.
- Should we cache probe results to speed up subsequent builds when the tool set has not changed?
