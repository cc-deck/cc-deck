# Quickstart: Two-Pass Binary Probing

## What Changed

The `cc-deck build run` command for OpenShell targets now uses a two-pass build process. Instead of looking up binary paths from a hardcoded table, it probes the actual built image to discover where tools are installed.

## User Impact

**No workflow changes required.** The two-pass process is transparent. Running `cc-deck build run` works exactly as before, but binary paths in the generated policy are now discovered automatically from the image.

## For Component Authors

Policy component YAML files gain two optional fields:

```yaml
probe_binaries:
  - pip
  - pip3
runtime_globs:
  - /sandbox/**/bin/pip
  - /sandbox/**/bin/pip3
```

- `probe_binaries`: binary names the probe step searches for via `which`/`find` inside the built image. Falls back to `match.tools` entries if omitted.
- `runtime_globs`: glob patterns for binaries created after the image is built (venvs, toolchains). Merged into the policy alongside probed paths.

Components with explicit `binaries` (like claude-code.yaml) are unchanged and unaffected.

## Build Performance

The two-pass approach adds a container probe step between two `podman build` invocations. The second build reuses all cached layers except the policy COPY layer and later. Expected overhead: under 30 seconds on a warm cache.

## Debugging

If the probe or second-pass build fails, the first-pass image is retained with the tag `<name>:probe-debug`. Inspect it to verify tool installation or run the probe manually:

```bash
podman run --rm <name>:probe-debug which pip3
```
