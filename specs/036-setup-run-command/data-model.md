# Data Model: cc-deck setup run

**Feature Branch**: `036-setup-run-command`
**Date**: 2026-04-13

## Entities

This feature introduces no new data entities. It reads from the existing `Manifest` struct and executes external tools.

### Manifest (existing, read-only)

| Field | Type | Used By |
|-------|------|---------|
| `targets.container.name` | string | Container build: image name |
| `targets.container.tag` | string | Container build: image tag (default: "latest") |
| `targets.container.registry` | string | Push: registry prefix for push reference |
| `targets.ssh.host` | string | Not directly used (ansible reads inventory.ini) |

### Derived Values

| Value | Source | Formula |
|-------|--------|---------|
| Image reference | Manifest | `name:tag` via `Manifest.ImageRef()` |
| Push reference | Manifest | `registry/name:tag` |
| Container runtime | PATH lookup | `DetectRuntime()` prefers podman, falls back to docker |

## Artifact Detection

Target type is detected by file presence in the setup directory:

| Files Present | Detected Target |
|---------------|----------------|
| `Containerfile` only | container |
| `site.yml` AND `inventory.ini` only | ssh |
| Both sets present | Error: `--target` required |
| Neither present | Error: no artifacts found |

## State

This command is stateless. It reads the manifest once, executes a build tool, and exits with the tool's exit code. No state is written or cached.
