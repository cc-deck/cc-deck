# Quickstart: Base Image Probe

## What This Feature Does

The base image probe inspects a container image before building to discover what is already installed. This lets the build skip redundant tool installations and use the correct package manager, so switching base images (Fedora, UBI, OpenShell/Debian) works without manual Containerfile editing.

## How It Works

```
cc-deck build init --target container
# edit build.yaml: change base image to UBI 9
cc-deck build run .cc-deck/setup --target container
```

The build now automatically:
1. Probes the UBI 9 base image (OS, package manager, installed tools)
2. Caches probe results in `probe-cache.json`
3. Generates a Containerfile that uses `dnf` and skips pre-installed tools
4. On repeat builds, reuses cached probe results (sub-second)

## Standalone Probe

```bash
cc-deck build probe registry.fedoraproject.org/fedora:41
cc-deck build probe registry.access.redhat.com/ubi9/ubi:latest --format json
cc-deck build probe <image> --no-cache   # force re-probe
```

## Key Files

| File | Purpose |
|------|---------|
| `cc-deck/internal/build/imageprobe/` | Go package: probe script generation, parsing, caching |
| `cc-deck/cmd/build_probe.go` | CLI subcommand entry point |
| `<setup-dir>/probe-cache.json` | Cached probe results per base image |
