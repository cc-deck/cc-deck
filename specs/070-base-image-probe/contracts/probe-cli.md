# Contract: cc-deck build probe CLI

## Command

```
cc-deck build probe <image-ref> [--setup-dir <path>] [--format json|table] [--no-cache] [--timeout <seconds>]
```

## Arguments

| Argument | Required | Default | Description |
|----------|----------|---------|-------------|
| `<image-ref>` | yes | - | Container image reference (e.g., `registry.fedoraproject.org/fedora:41`) |
| `--setup-dir` | no | `.cc-deck/setup` | Path to setup directory (for probe cache and manifest) |
| `--format` | no | `table` | Output format: `json` for machine-readable, `table` for human-readable summary |
| `--no-cache` | no | false | Force a fresh probe, ignoring cached results |
| `--timeout` | no | 30 | Probe timeout in seconds |

## Behavior

1. Resolve image digest via `podman inspect` (pull if not local)
2. Check `<setup-dir>/probe-cache.json` for a cache hit (matching ref + digest)
3. If cache hit and `--no-cache` not set: return cached results
4. Otherwise: run probe script via `podman run --rm --entrypoint /bin/sh`
5. Parse results, store in cache, return

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Probe succeeded (from cache or fresh) |
| 1 | Probe failed (image pull failure, runtime error, timeout) |

## JSON Output Schema

```json
{
  "image_ref": "registry.fedoraproject.org/fedora:41",
  "image_digest": "sha256:abc123...",
  "cached": false,
  "os": {
    "id": "fedora",
    "id_like": "",
    "name": "Fedora Linux 41",
    "version": "41"
  },
  "package_manager": "dnf",
  "tools": {
    "git": {"name": "git", "path": "/usr/bin/git", "version": "2.43.0", "present": true},
    "python3": {"name": "python3", "path": "/usr/bin/python3", "version": "3.12.4", "present": true},
    "go": {"name": "go", "path": "", "version": "", "present": false}
  },
  "user": {"name": "root", "uid": 0, "home": "/root", "shell": "/bin/bash"},
  "shells": ["bash", "sh"],
  "duration_ms": 4200
}
```

## Table Output Format

```
Base Image Probe: registry.fedoraproject.org/fedora:41
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

  OS:              Fedora Linux 41
  Package Manager: dnf
  User:            root (uid=0, /root)
  Shells:          bash, sh

  Tools:
  ✓ git          2.43.0    /usr/bin/git
  ✓ python3      3.12.4    /usr/bin/python3
  ✗ go           -         not found
  ✓ curl         8.6.0     /usr/bin/curl
  ...

  Probed in 4.2s (fresh)
```

## Diff Output (when manifest is available)

When `--setup-dir` contains a `build.yaml`, the table output appends a diff section:

```
  Tool Diff (vs manifest requirements):
  ✓ python3      3.12.4 ≥ 3.12 (required)    → skip
  ✗ go           -      (1.25 required)        → install via dnf
  ~ nodejs       18.0.0 < 22 (required)        → shadow in /usr/local/bin
```
