# CLI Command Contract: cc-deck setup run

## Synopsis

```
cc-deck setup run [dir] [--target container|ssh] [--push]
```

## Purpose

Execute pre-generated build artifacts (Containerfile or Ansible playbooks) directly, without Claude Code involvement.

## Arguments

| Argument | Type | Default | Description |
|----------|------|---------|-------------|
| `dir` | positional, optional | Auto-resolved via `resolveSetupDir()` | Setup directory containing artifacts and manifest |

## Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--target` | string | (auto-detect) | Force target type: `container` or `ssh` |
| `--push` | bool | false | Push container image after successful build (container only) |

## Behavior

### Target Auto-Detection (when `--target` omitted)

1. Check for `Containerfile` in setup directory
2. Check for `site.yml` AND `inventory.ini` in setup directory
3. If only one set found: use that target
4. If both found: error, require `--target`
5. If neither found: error, suggest running `/cc-deck.build`

### Container Target

1. Load manifest from `<dir>/cc-deck-setup.yaml`
2. Detect container runtime via `DetectRuntime()` (podman preferred)
3. Get image reference via `Manifest.ImageRef()` (`name:tag`)
4. Execute: `<runtime> build -t <name>:<tag> -f Containerfile .` from setup directory
5. Stream stdout/stderr to terminal in real time
6. If `--push` and build succeeded:
   a. Validate `targets.container.registry` is set in manifest
   b. Execute: `<runtime> push <registry>/<name>:<tag>`

### SSH Target

1. Verify `ansible-playbook` is on PATH
2. Execute: `ansible-playbook -i inventory.ini site.yml` from setup directory
3. Stream stdout/stderr to terminal in real time

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Build (and push, if requested) succeeded |
| N | Exit code from the build tool (podman, ansible-playbook) is passed through |

## Error Messages

| Condition | Message |
|-----------|---------|
| No container runtime | `neither podman nor docker found in PATH` |
| No ansible-playbook | `ansible-playbook not found in PATH; install with: pip install ansible` |
| No artifacts | `no build artifacts found in <dir>; run /cc-deck.build to generate them` |
| Both artifacts, no --target | `both container and SSH artifacts found; use --target to select one` |
| --push without registry | `targets.container.registry not set in manifest` |
| --push with SSH target | `--push is only valid for container targets` |
| Manifest missing/invalid | Delegated to `LoadManifest()` error handling |
