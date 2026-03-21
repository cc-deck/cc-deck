# Contract: CLI Extensions

**Feature**: 024-container-env | **Date**: 2026-03-20

## Modified Commands

### `cc-deck env create`

New flags for container environments:

```
--type, -t     Environment type (local, container)          [default: local]
--image        Container image reference                     [container only]
--port         Port mapping host:container (repeatable)      [container only]
--all-ports    Expose all declared ports                     [container only]
--storage      Storage type: named-volume, host-path         [default: named-volume]
--path         Host path for bind mount                      [requires --storage host-path]
--credential   KEY=VALUE credential injection (repeatable)   [container only]
```

**Type help text update**:
```
Environment types:
  local       Zellij session on the host machine (default)
  container   Container via Podman
  k8s-deploy  Kubernetes StatefulSet (not yet implemented)
  k8s-sandbox Ephemeral Kubernetes Pod (not yet implemented)
```

**Validation**:
- `--image` is ignored for `--type local`
- `--port` and `--all-ports` are mutually exclusive concepts but both can be provided
- `--path` requires `--storage host-path`
- `--credential` format is validated as `KEY=VALUE`
- Unknown flags for local type produce a warning, not an error

### `cc-deck env delete`

New flag:

```
--keep-volumes    Preserve named volumes on delete    [container only]
```

### `cc-deck env list`

Updated type filter to include `container`:

```
--type, -t    Filter by type (local, container)
```

Reconciliation now includes container environments (via `podman inspect`).

### `cc-deck env exec`

Implemented for container environments:

```
cc-deck env exec <name> -- <command...>
```

Runs `podman exec cc-deck-<name> <command...>`.

### `cc-deck env push`

Implemented for container environments:

```
cc-deck env push <name> [local-path]
```

- `local-path` defaults to current directory
- Copies to `/workspace/<basename>` inside container

### `cc-deck env pull`

Implemented for container environments:

```
cc-deck env pull <name> <remote-path> [local-path]
```

- `remote-path` is required (path inside container)
- `local-path` defaults to current directory

## Config Defaults

Extension to `config.yaml`:

```yaml
defaults:
  container:
    image: quay.io/cc-deck/cc-deck-demo:latest
    storage: named-volume
```

Resolved by the CLI: flag → config default → hardcoded fallback (demo image with warning).
